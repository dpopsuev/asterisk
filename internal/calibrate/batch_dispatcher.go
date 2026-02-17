package calibrate

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// BatchFileDispatcher writes N signals concurrently, generates a batch
// manifest and briefing file, then polls all N artifact paths in parallel.
// It wraps N FileDispatcher instances — one per signal slot.
type BatchFileDispatcher struct {
	cfg         FileDispatcherConfig
	log         *slog.Logger
	suiteDir    string // .asterisk/calibrate/{suiteID}
	batchID     int64  // monotonic batch counter
	batchSize   int    // max signals per batch (default 4)
	tokenBudget int    // total token budget; 0 = unlimited
	tokenUsed   int    // cumulative tokens used across batches
}

// BatchFileDispatcherConfig configures the BatchFileDispatcher.
type BatchFileDispatcherConfig struct {
	FileConfig  FileDispatcherConfig
	SuiteDir    string
	BatchSize   int
	TokenBudget int // total token budget; 0 = unlimited
	Logger      *slog.Logger
}

// NewBatchFileDispatcher creates a batch dispatcher.
func NewBatchFileDispatcher(cfg BatchFileDispatcherConfig) *BatchFileDispatcher {
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = 4
	}
	l := cfg.Logger
	if l == nil {
		l = discardLogger()
	}
	return &BatchFileDispatcher{
		cfg:         cfg.FileConfig,
		log:         l,
		suiteDir:    cfg.SuiteDir,
		batchSize:   cfg.BatchSize,
		tokenBudget: cfg.TokenBudget,
	}
}

// BatchResult holds the result of dispatching one signal in a batch.
type BatchResult struct {
	Index int
	Data  []byte
	Err   error
}

// DispatchBatch writes N signals, generates a manifest and briefing path,
// then polls all N artifact paths concurrently using one goroutine per signal.
// Returns results in the same order as the input contexts.
func (d *BatchFileDispatcher) DispatchBatch(ctxs []DispatchContext, phase string, briefingPath string) ([][]byte, []error) {
	n := len(ctxs)
	if n == 0 {
		return nil, nil
	}

	d.batchID++
	bid := d.batchID

	d.log.Debug("batch dispatch begin",
		"batch_id", bid, "signals", n, "phase", phase)

	// Build signal entries for the manifest
	signals := make([]BatchSignalEntry, n)
	for i, ctx := range ctxs {
		sigDir := filepath.Dir(ctx.ArtifactPath)
		signals[i] = BatchSignalEntry{
			CaseID:     ctx.CaseID,
			SignalPath: filepath.Join(sigDir, "signal.json"),
			Status:     "pending",
		}
	}

	// Write manifest
	manifest := NewBatchManifest(bid, phase, briefingPath, signals)
	manifestPath := filepath.Join(d.suiteDir, "batch-manifest.json")
	if err := os.MkdirAll(d.suiteDir, 0755); err != nil {
		errs := make([]error, n)
		for i := range errs {
			errs[i] = fmt.Errorf("mkdir suite dir: %w", err)
		}
		return make([][]byte, n), errs
	}
	if err := WriteManifest(manifestPath, manifest); err != nil {
		errs := make([]error, n)
		for i := range errs {
			errs[i] = fmt.Errorf("write manifest: %w", err)
		}
		return make([][]byte, n), errs
	}

	d.log.Debug("manifest written",
		"batch_id", bid, "path", manifestPath, "signals", n)

	// Update manifest to in_progress
	manifest.Status = "in_progress"
	_ = WriteManifest(manifestPath, manifest)

	// Dispatch all signals concurrently using individual FileDispatchers
	results := make([]BatchResult, n)
	var wg sync.WaitGroup

	for i, ctx := range ctxs {
		wg.Add(1)
		go func(idx int, dctx DispatchContext) {
			defer wg.Done()
			fd := NewFileDispatcher(d.cfg)
			data, err := fd.Dispatch(dctx)
			results[idx] = BatchResult{
				Index: idx,
				Data:  data,
				Err:   err,
			}
			// Update per-signal status in manifest
			if err != nil {
				signals[idx].Status = "error"
			} else {
				signals[idx].Status = "done"
			}
		}(i, ctx)
	}

	wg.Wait()

	// Determine batch status
	allDone := true
	allError := true
	for _, r := range results {
		if r.Err != nil {
			allDone = false
		} else {
			allError = false
		}
	}

	if allError {
		manifest.Status = "error"
	} else if allDone {
		manifest.Status = "done"
	} else {
		manifest.Status = "done" // partial success is still "done"
	}
	manifest.Signals = signals
	_ = WriteManifest(manifestPath, manifest)

	// Write budget status if a token budget is configured
	if d.tokenBudget > 0 {
		budgetPath := filepath.Join(d.suiteDir, "budget-status.json")
		if err := WriteBudgetStatus(budgetPath, d.tokenBudget, d.tokenUsed); err != nil {
			d.log.Warn("failed to write budget status", "error", err)
		}
	}

	d.log.Debug("batch dispatch complete",
		"batch_id", bid, "status", manifest.Status)

	// Collect results
	data := make([][]byte, n)
	errs := make([]error, n)
	for _, r := range results {
		data[r.Index] = r.Data
		errs[r.Index] = r.Err
	}

	return data, errs
}

// Dispatch implements the Dispatcher interface for single-signal compatibility.
// It delegates to a one-element DispatchBatch.
func (d *BatchFileDispatcher) Dispatch(ctx DispatchContext) ([]byte, error) {
	data, errs := d.DispatchBatch([]DispatchContext{ctx}, "single", "")
	if len(errs) > 0 && errs[0] != nil {
		return nil, errs[0]
	}
	if len(data) > 0 {
		return data[0], nil
	}
	return nil, fmt.Errorf("batch dispatch returned no results")
}

// SuiteDir returns the configured suite directory.
func (d *BatchFileDispatcher) SuiteDir() string {
	return d.suiteDir
}

// BatchSize returns the configured maximum batch size.
func (d *BatchFileDispatcher) BatchSize() int {
	return d.batchSize
}

// ManifestPath returns the path to the batch manifest for the current suite.
func (d *BatchFileDispatcher) ManifestPath() string {
	return filepath.Join(d.suiteDir, "batch-manifest.json")
}

// WriteBriefing writes a briefing file to the suite directory and returns its path.
func (d *BatchFileDispatcher) WriteBriefing(content string) (string, error) {
	path := filepath.Join(d.suiteDir, "briefing.md")
	if err := os.MkdirAll(d.suiteDir, 0755); err != nil {
		return "", fmt.Errorf("mkdir for briefing: %w", err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return "", fmt.Errorf("write briefing: %w", err)
	}
	return path, nil
}

// LastBatchID returns the latest batch ID (useful for testing).
func (d *BatchFileDispatcher) LastBatchID() int64 {
	return d.batchID
}

// UpdateTokenUsage sets the cumulative token usage (called by the runner
// after reading token tracker data). This drives the budget-status.json.
func (d *BatchFileDispatcher) UpdateTokenUsage(used int) {
	d.tokenUsed = used
}

// TokenBudget returns the configured token budget.
func (d *BatchFileDispatcher) TokenBudget() int {
	return d.tokenBudget
}

// now is a helper for timestamps (allows testing override).
func now() string {
	return time.Now().UTC().Format(time.RFC3339)
}
