package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"asterisk/internal/calibrate"
	"asterisk/internal/calibrate/adapt"
	"asterisk/internal/calibrate/dispatch"
	"asterisk/internal/calibrate/scenarios"
	"asterisk/internal/orchestrate"
	"asterisk/internal/preinvest"
	"asterisk/internal/rp"
	"asterisk/internal/store"
)

// SessionState tracks the lifecycle of a calibration session.
type SessionState string

const (
	StateRunning  SessionState = "running"
	StateDone     SessionState = "done"
	StateError    SessionState = "error"
)

// Signal represents a single event on the agent message bus.
type Signal struct {
	Timestamp string            `json:"ts"`
	Event     string            `json:"event"`
	Agent     string            `json:"agent"`
	CaseID    string            `json:"case_id,omitempty"`
	Step      string            `json:"step,omitempty"`
	Meta      map[string]string `json:"meta,omitempty"`
}

// SignalBus is a thread-safe, append-only signal log for agent coordination.
type SignalBus struct {
	mu      sync.Mutex
	signals []Signal
}

func (b *SignalBus) Emit(event, agent, caseID, step string, meta map[string]string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.signals = append(b.signals, Signal{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Event:     event,
		Agent:     agent,
		CaseID:    caseID,
		Step:      step,
		Meta:      meta,
	})
}

func (b *SignalBus) Since(idx int) []Signal {
	b.mu.Lock()
	defer b.mu.Unlock()
	if idx < 0 {
		log.Printf("[signal-bus] WARN: Since called with negative idx=%d, clamping to 0", idx)
		idx = 0
	}
	if idx >= len(b.signals) {
		return nil
	}
	out := make([]Signal, len(b.signals)-idx)
	copy(out, b.signals[idx:])
	return out
}

func (b *SignalBus) Len() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.signals)
}

// Session holds the state for a single calibration run driven by MCP tool calls.
type Session struct {
	ID         string
	TotalCases int
	Scenario   string
	Bus        *SignalBus

	state      SessionState
	dispatcher *dispatch.MuxDispatcher
	report     *calibrate.CalibrationReport
	err        error
	doneCh     chan struct{}
	cancel     context.CancelFunc

	mu sync.Mutex
}

// GetState returns the current session state in a thread-safe manner.
func (s *Session) GetState() SessionState {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.state
}

// StartCalibrationInput mirrors the tool arguments for start_calibration.
type StartCalibrationInput struct {
	Scenario    string `json:"scenario"`
	Adapter     string `json:"adapter"`
	Grade       string `json:"grade,omitempty"`
	RPBaseURL   string `json:"rp_base_url,omitempty"`
	RPProject   string `json:"rp_project,omitempty"`
	RPKeyPath   string `json:"rp_key_path,omitempty"`
	Parallel    int    `json:"parallel,omitempty"`
	ProjectRoot string `json:"-"`
}

// NewSession creates a calibration session, resolves the scenario, spawns
// the calibration runner goroutine, and returns immediately.
func NewSession(ctx context.Context, input StartCalibrationInput) (*Session, error) {
	scenario, err := loadScenario(input.Scenario)
	if err != nil {
		return nil, err
	}

	var rpFetcher preinvest.Fetcher
	if input.RPBaseURL != "" {
		rpProject := input.RPProject
		if rpProject == "" {
			rpProject = os.Getenv("ASTERISK_RP_PROJECT")
		}
		if rpProject == "" {
			return nil, fmt.Errorf("rp_project is required when rp_base_url is set")
		}
		keyPath := input.RPKeyPath
		if keyPath == "" {
			keyPath = ".rp-api-key"
		}
		key, err := rp.ReadAPIKey(keyPath)
		if err != nil {
			return nil, fmt.Errorf("read RP API key from %s: %w", keyPath, err)
		}
		client, err := rp.New(input.RPBaseURL, key, rp.WithTimeout(30*time.Second))
		if err != nil {
			return nil, fmt.Errorf("create RP client: %w", err)
		}
		rpFetcher = rp.NewFetcher(client, rpProject)
		if err := calibrate.ResolveRPCases(rpFetcher, scenario); err != nil {
			return nil, fmt.Errorf("resolve RP-sourced cases: %w", err)
		}
	}

	if input.Grade != "" {
		scenario = calibrate.FilterByGrade(scenario, input.Grade)
		if len(scenario.Cases) == 0 {
			return nil, fmt.Errorf("no cases match grade=%s", input.Grade)
		}
	}

	// Resolve relative paths against project root so templates and artifacts
	// are found regardless of the process's current working directory.
	root := input.ProjectRoot
	promptDir := filepath.Join(root, ".cursor/prompts")
	basePath := filepath.Join(root, ".asterisk/calibrate")

	runCtx, runCancel := context.WithCancel(context.Background())

	mcpDisp := dispatch.NewMuxDispatcher(runCtx)
	tokenTracker := dispatch.NewTokenTracker()
	tracked := dispatch.NewTokenTrackingDispatcher(mcpDisp, tokenTracker)

	var adapter calibrate.ModelAdapter
	switch input.Adapter {
	case "stub":
		adapter = adapt.NewStubAdapter(scenario)
	case "basic":
		basicSt, err := store.Open(":memory:")
		if err != nil {
			runCancel()
			return nil, fmt.Errorf("basic adapter: open store: %w", err)
		}
		var repoNames []string
		for _, r := range scenario.Workspace.Repos {
			repoNames = append(repoNames, r.Name)
		}
		ba := adapt.NewBasicAdapter(basicSt, repoNames)
		for _, c := range scenario.Cases {
			ba.RegisterCase(c.ID, &adapt.BasicCaseInfo{
				Name:         c.TestName,
				ErrorMessage: c.ErrorMessage,
			})
		}
		adapter = ba
	default:
		adapter = adapt.NewCursorAdapter(
			promptDir,
			adapt.WithDispatcher(tracked),
			adapt.WithBasePath(basePath),
		)
	}

	parallel := input.Parallel
	if parallel < 1 {
		parallel = 1
	}

	cfg := calibrate.RunConfig{
		Scenario:     scenario,
		Adapter:      adapter,
		Runs:         1,
		PromptDir:    promptDir,
		Thresholds:   orchestrate.DefaultThresholds(),
		TokenTracker: tokenTracker,
		Parallel:     parallel,
		TokenBudget:  parallel,
		BasePath:     basePath,
		RPFetcher:    rpFetcher,
	}

	bus := &SignalBus{}

	sess := &Session{
		ID:         fmt.Sprintf("s-%d", time.Now().UnixMilli()),
		state:      StateRunning,
		TotalCases: len(scenario.Cases),
		Scenario:   scenario.Name,
		Bus:        bus,
		dispatcher: mcpDisp,
		doneCh:     make(chan struct{}),
		cancel:     runCancel,
	}

	bus.Emit("session_started", "server", "", "", map[string]string{
		"scenario":    scenario.Name,
		"adapter":     input.Adapter,
		"total_cases": fmt.Sprintf("%d", len(scenario.Cases)),
	})

	// Ensure calibration artifact directory exists
	if err := os.MkdirAll(basePath, 0755); err != nil {
		runCancel()
		return nil, fmt.Errorf("create calibrate dir: %w", err)
	}

	go sess.run(runCtx, cfg)

	return sess, nil
}

// Cancel terminates the runner goroutine and releases resources.
func (s *Session) Cancel() {
	if s.cancel != nil {
		s.cancel()
	}
}

// run executes RunCalibration in a goroutine and captures the result.
func (s *Session) run(ctx context.Context, cfg calibrate.RunConfig) {
	defer close(s.doneCh)
	defer s.cancel()

	report, err := calibrate.RunCalibration(ctx, cfg)

	s.mu.Lock()
	defer s.mu.Unlock()

	if err != nil {
		s.state = StateError
		s.err = err
		s.Bus.Emit("session_error", "server", "", "", map[string]string{"error": err.Error()})
		log.Printf("[mcp-session] calibration error: %v", err)
		return
	}
	s.state = StateDone
	s.report = report
	s.Bus.Emit("session_done", "server", "", "", map[string]string{
		"case_results": fmt.Sprintf("%d", len(report.CaseResults)),
	})
	log.Printf("[mcp-session] calibration complete: %d case results", len(report.CaseResults))
}

// GetNextStep blocks until the runner produces the next prompt, or returns
// done=true if the run has completed.
func (s *Session) GetNextStep(ctx context.Context) (dc dispatch.DispatchContext, done bool, err error) {
	select {
	case <-s.doneCh:
		return dispatch.DispatchContext{}, true, nil
	default:
	}

	// Try both: runner may finish between the default check and the select.
	select {
	case <-ctx.Done():
		return dispatch.DispatchContext{}, false, ctx.Err()
	case <-s.doneCh:
		return dispatch.DispatchContext{}, true, nil
	case dc, ok := <-s.dispatcher.PromptCh():
		if !ok {
			return dispatch.DispatchContext{}, true, nil
		}
		return dc, false, nil
	}
}

// SubmitArtifact routes the agent's artifact to the correct Dispatch caller.
// If dispatchID is 0, falls back to legacy unrouted submit (serial mode only).
func (s *Session) SubmitArtifact(ctx context.Context, dispatchID int64, data []byte) error {
	if !json.Valid(data) {
		return fmt.Errorf("invalid JSON in artifact")
	}
	return s.dispatcher.SubmitArtifact(ctx, dispatchID, data)
}

// Report returns the calibration report, or nil if not yet done.
func (s *Session) Report() *calibrate.CalibrationReport {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.report
}

// Err returns any error from the calibration run.
func (s *Session) Err() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.err
}

// Done returns a channel that closes when the calibration completes.
func (s *Session) Done() <-chan struct{} {
	return s.doneCh
}

func loadScenario(name string) (*calibrate.Scenario, error) {
	switch name {
	case "ptp-mock":
		return scenarios.PTPMockScenario(), nil
	case "daemon-mock":
		return scenarios.DaemonMockScenario(), nil
	case "ptp-real":
		return scenarios.PTPRealScenario(), nil
	case "ptp-real-ingest":
		return scenarios.PTPRealIngestScenario(), nil
	default:
		return nil, fmt.Errorf("unknown scenario: %s (available: ptp-mock, daemon-mock, ptp-real, ptp-real-ingest)", name)
	}
}
