package calibrate

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"time"
)

// discardLogger returns a logger that discards all output.
func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// Dispatcher abstracts how a prompt is delivered to an external agent
// and how the resulting artifact is collected back.
// This is the Strategy pattern: CursorAdapter holds one Dispatcher and
// delegates transport to it. Different dispatchers handle different
// communication mechanisms (stdin, file polling, HTTP, etc.).
type Dispatcher interface {
	// Dispatch delivers the prompt at PromptPath to the external agent and
	// blocks until the artifact appears at ArtifactPath.
	// Returns the raw artifact bytes or an error (e.g. timeout).
	Dispatch(ctx DispatchContext) ([]byte, error)
}

// DispatchContext carries all the metadata a dispatcher needs to deliver
// a prompt and collect an artifact.
type DispatchContext struct {
	CaseID       string // ground-truth case ID, e.g. "C1"
	Step         string // pipeline step name, e.g. "F0_RECALL"
	PromptPath   string // absolute path to the filled prompt file
	ArtifactPath string // absolute path where artifact JSON should appear
}

// --- StdinDispatcher (interactive, extracts current CursorAdapter behavior) ---

// StdinDispatcher delivers prompts by printing a banner to stdout and
// blocking on stdin until the user presses Enter. This is the default
// dispatcher — identical to the original CursorAdapter interactive flow.
type StdinDispatcher struct {
	reader *bufio.Reader
}

// NewStdinDispatcher creates a dispatcher that reads from os.Stdin.
func NewStdinDispatcher() *StdinDispatcher {
	return &StdinDispatcher{reader: bufio.NewReader(os.Stdin)}
}

// Dispatch prints a banner with case/step/paths, blocks on stdin, then reads
// and validates the artifact file.
func (d *StdinDispatcher) Dispatch(ctx DispatchContext) ([]byte, error) {
	fmt.Println()
	fmt.Println("================================================================")
	fmt.Printf("  Case: %-6s  Step: %s\n", ctx.CaseID, ctx.Step)
	fmt.Println("================================================================")
	fmt.Printf("  Prompt:   %s\n", ctx.PromptPath)
	fmt.Printf("  Artifact: %s\n", ctx.ArtifactPath)
	fmt.Println("----------------------------------------------------------------")
	fmt.Println("  1. Open the prompt file and paste it into Cursor")
	fmt.Println("  2. Save Cursor's JSON response to the artifact path above")
	fmt.Println("  3. Press Enter to continue")
	fmt.Println("================================================================")
	fmt.Print("  > ")
	_, _ = d.reader.ReadString('\n')

	data, err := os.ReadFile(ctx.ArtifactPath)
	if err != nil {
		return nil, fmt.Errorf("artifact not found at %s: %w", ctx.ArtifactPath, err)
	}

	var raw json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("invalid JSON in %s: %w", ctx.ArtifactPath, err)
	}

	fmt.Printf("  Read artifact (%d bytes)\n", len(data))
	return raw, nil
}

// --- FileDispatcher (auto mode, signal file polling) ---

// FileDispatcherConfig configures the FileDispatcher behavior.
type FileDispatcherConfig struct {
	PollInterval    time.Duration // how often to check for the artifact; default 500ms
	Timeout         time.Duration // max time to wait for the artifact; default 10min
	MaxStaleRejects int           // consecutive stale dispatch_id reads before aborting; default 3
	SignalDir       string        // directory for signal.json; defaults to artifact dir
	Logger          *slog.Logger  // structured logger; nil = discard
}

// DefaultFileDispatcherConfig returns sensible defaults.
func DefaultFileDispatcherConfig() FileDispatcherConfig {
	return FileDispatcherConfig{
		PollInterval:    500 * time.Millisecond,
		Timeout:         10 * time.Minute,
		MaxStaleRejects: 10,
	}
}

// signalFile is the JSON written next to the prompt to inform the external
// agent that a prompt is waiting.
type signalFile struct {
	Status       string `json:"status"`        // waiting, processing, done, error
	DispatchID   int64  `json:"dispatch_id"`   // monotonic ID; agent must echo in artifact wrapper
	CaseID       string `json:"case_id"`
	Step         string `json:"step"`
	PromptPath   string `json:"prompt_path"`
	ArtifactPath string `json:"artifact_path"`
	Timestamp    string `json:"timestamp"`
	Error        string `json:"error,omitempty"`
}

// artifactWrapper is a thin envelope the responder writes.  The dispatcher
// accepts the artifact only when dispatch_id matches the current signal.
// The "data" field carries the actual step payload.
type artifactWrapper struct {
	DispatchID int64           `json:"dispatch_id"`
	Data       json.RawMessage `json:"data"`
}

// FileDispatcher writes a signal.json file and polls for the artifact file
// to appear on disk. Designed for automated/semi-automated calibration where
// an external agent (Cursor skill, harness, script) watches for the signal.
type FileDispatcher struct {
	cfg        FileDispatcherConfig
	log        *slog.Logger
	dispatchID int64 // monotonic counter
}

// NewFileDispatcher creates a file-based dispatcher with the given config.
func NewFileDispatcher(cfg FileDispatcherConfig) *FileDispatcher {
	if cfg.PollInterval <= 0 {
		cfg.PollInterval = 500 * time.Millisecond
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 10 * time.Minute
	}
	if cfg.MaxStaleRejects <= 0 {
		cfg.MaxStaleRejects = 10
	}
	l := cfg.Logger
	if l == nil {
		l = discardLogger()
	}
	return &FileDispatcher{cfg: cfg, log: l}
}

// Dispatch writes signal.json with a monotonic dispatch_id, polls for an
// artifact whose wrapper echoes the same dispatch_id, validates JSON, and
// returns the inner "data" bytes.  Stale artifacts from previous dispatches
// are deterministically rejected by ID mismatch — no timing fences needed.
func (d *FileDispatcher) Dispatch(ctx DispatchContext) ([]byte, error) {
	signalDir := d.cfg.SignalDir
	if signalDir == "" {
		signalDir = filepath.Dir(ctx.ArtifactPath)
	}
	signalPath := filepath.Join(signalDir, "signal.json")

	d.dispatchID++
	did := d.dispatchID

	dl := d.log.With("case", ctx.CaseID, "step", ctx.Step, "dispatch_id", did)
	dl.Debug("dispatch begin", "artifact_path", ctx.ArtifactPath, "signal_path", signalPath)

	// Remove any existing artifact file before writing the signal.
	// Without this, the polling loop may immediately find a stale artifact
	// from a previous dispatch or calibrate runner's writeback, hitting the
	// MaxStaleRejects limit before the responder can write the new one.
	if _, err := os.Stat(ctx.ArtifactPath); err == nil {
		dl.Debug("removing stale artifact before dispatch", "path", ctx.ArtifactPath)
		_ = os.Remove(ctx.ArtifactPath)
	}

	// Write signal: status=waiting with the new dispatch_id.
	sig := signalFile{
		Status:       "waiting",
		DispatchID:   did,
		CaseID:       ctx.CaseID,
		Step:         ctx.Step,
		PromptPath:   ctx.PromptPath,
		ArtifactPath: ctx.ArtifactPath,
		Timestamp:    time.Now().UTC().Format(time.RFC3339),
	}
	if err := writeSignal(signalPath, &sig); err != nil {
		return nil, fmt.Errorf("write signal: %w", err)
	}
	dl.Debug("signal written", "status", "waiting")

	fmt.Printf("[file-dispatch] signal.json written: case=%s step=%s dispatch_id=%d\n", ctx.CaseID, ctx.Step, did)
	fmt.Printf("[file-dispatch] waiting for artifact at %s (timeout %s)\n", ctx.ArtifactPath, d.cfg.Timeout)

	// Poll for artifact file with matching dispatch_id
	deadline := time.Now().Add(d.cfg.Timeout)
	pollCount := 0
	staleCount := 0 // consecutive stale dispatch_id mismatches
	for {
		if time.Now().After(deadline) {
			dl.Debug("timeout reached", "polls", pollCount)
			sig.Status = "error"
			sig.Error = "timeout waiting for artifact"
			_ = writeSignal(signalPath, &sig)
			return nil, fmt.Errorf("timeout after %s waiting for artifact at %s", d.cfg.Timeout, ctx.ArtifactPath)
		}

		// Check if the responder reported an error via signal.json
		if sigData, readErr := os.ReadFile(signalPath); readErr == nil {
			var liveSig signalFile
			if json.Unmarshal(sigData, &liveSig) == nil && liveSig.DispatchID == did && liveSig.Status == "error" {
				dl.Debug("responder reported error via signal", "error", liveSig.Error)
				return nil, fmt.Errorf("responder error: %s", liveSig.Error)
			}
		}

		pollCount++
		data, err := os.ReadFile(ctx.ArtifactPath)
		if err != nil {
			if pollCount <= 3 || pollCount%20 == 0 {
				dl.Debug("poll: artifact not found", "poll", pollCount, "err", err)
			}
			staleCount = 0 // file absent = responder hasn't written yet; reset stale streak
			time.Sleep(d.cfg.PollInterval)
			continue
		}

		dl.Debug("poll: artifact file found", "poll", pollCount, "bytes", len(data))

		// Parse the wrapper to check dispatch_id
		var wrapper artifactWrapper
		if err := json.Unmarshal(data, &wrapper); err != nil {
			dl.Debug("poll: invalid JSON on first read, retrying", "poll", pollCount, "err", err)
			time.Sleep(d.cfg.PollInterval)
			data, err = os.ReadFile(ctx.ArtifactPath)
			if err != nil {
				dl.Debug("poll: artifact disappeared on retry", "poll", pollCount, "err", err)
				continue
			}
			if err := json.Unmarshal(data, &wrapper); err != nil {
				dl.Debug("poll: invalid JSON on retry — failing", "poll", pollCount, "err", err)
				sig.Status = "error"
				sig.Error = fmt.Sprintf("invalid JSON in artifact: %v", err)
				_ = writeSignal(signalPath, &sig)
				return nil, fmt.Errorf("invalid JSON in %s: %w", ctx.ArtifactPath, err)
			}
		}

		// Reject stale artifacts deterministically by dispatch_id
		if wrapper.DispatchID != did {
			staleCount++
			dl.Debug("poll: stale artifact (dispatch_id mismatch)",
				"poll", pollCount, "want", did, "got", wrapper.DispatchID,
				"stale_streak", staleCount, "max", d.cfg.MaxStaleRejects)
			if staleCount >= d.cfg.MaxStaleRejects {
				sig.Status = "error"
				sig.Error = fmt.Sprintf("exceeded stale tolerance: %d consecutive artifacts with wrong dispatch_id (want %d, last got %d)",
					staleCount, did, wrapper.DispatchID)
				_ = writeSignal(signalPath, &sig)
				return nil, fmt.Errorf("stale artifact tolerance exceeded: %d consecutive dispatch_id mismatches (want %d, got %d) at %s",
					staleCount, did, wrapper.DispatchID, ctx.ArtifactPath)
			}
			time.Sleep(d.cfg.PollInterval)
			continue
		}
		staleCount = 0 // reset on any valid read (including no-file polls)

		// dispatch_id matches — this is our artifact
		if len(wrapper.Data) == 0 {
			sig.Status = "error"
			sig.Error = "artifact wrapper has empty 'data' field"
			_ = writeSignal(signalPath, &sig)
			return nil, fmt.Errorf("artifact at %s has matching dispatch_id but empty 'data'", ctx.ArtifactPath)
		}

		dl.Debug("artifact validated", "poll", pollCount, "bytes", len(wrapper.Data))

		// Update signal: status=processing
		sig.Status = "processing"
		sig.Error = ""
		_ = writeSignal(signalPath, &sig)

		fmt.Printf("[file-dispatch] artifact found (%d bytes, dispatch_id=%d)\n", len(wrapper.Data), did)
		return wrapper.Data, nil
	}
}

// MarkDone updates the signal file to "done" after the caller has processed the artifact.
// Called by CursorAdapter after successfully parsing the artifact.
func (d *FileDispatcher) MarkDone(artifactPath string) {
	signalDir := d.cfg.SignalDir
	if signalDir == "" {
		signalDir = filepath.Dir(artifactPath)
	}
	signalPath := filepath.Join(signalDir, "signal.json")

	data, err := os.ReadFile(signalPath)
	if err != nil {
		d.log.Debug("mark-done: cannot read signal", "path", signalPath, "err", err)
		return
	}
	var sig signalFile
	if err := json.Unmarshal(data, &sig); err != nil {
		d.log.Debug("mark-done: cannot parse signal", "path", signalPath, "err", err)
		return
	}
	d.log.Debug("mark-done", "prev_status", sig.Status, "case", sig.CaseID, "step", sig.Step, "dispatch_id", sig.DispatchID)
	sig.Status = "done"
	_ = writeSignal(signalPath, &sig)
}

// CurrentDispatchID returns the latest dispatch_id. Useful for tests.
func (d *FileDispatcher) CurrentDispatchID() int64 {
	return d.dispatchID
}

// writeSignal atomically writes a signal file.
func writeSignal(path string, sig *signalFile) error {
	data, err := json.MarshalIndent(sig, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal signal: %w", err)
	}
	// Write to temp file then rename for atomicity
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return fmt.Errorf("write signal tmp: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		// Fallback: direct write (rename may fail on some FS).
		// Clean up the orphaned .tmp file since rename didn't consume it.
		defer os.Remove(tmp)
		return os.WriteFile(path, data, 0644)
	}
	return nil
}
