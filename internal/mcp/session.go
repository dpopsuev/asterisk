package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"asterisk/internal/calibrate"
	"asterisk/internal/calibrate/adapt"
	"github.com/dpopsuev/origami/dispatch"
	"asterisk/internal/calibrate/scenarios"
	"github.com/dpopsuev/origami/logging"
	"asterisk/internal/orchestrate"
	"asterisk/internal/preinvest"
	"asterisk/internal/rp"
	"asterisk/internal/store"
	fwmcp "github.com/dpopsuev/origami/mcp"
)

// SessionState tracks the lifecycle of a calibration session.
type SessionState string

const (
	StateRunning  SessionState = "running"
	StateDone     SessionState = "done"
	StateError    SessionState = "error"
)

// Session holds the state for a single calibration run driven by MCP tool calls.
type Session struct {
	ID              string
	TotalCases      int
	Scenario        string
	DesiredCapacity int
	Bus             *fwmcp.SignalBus

	log        *slog.Logger
	state      SessionState
	dispatcher *dispatch.MuxDispatcher
	report     *calibrate.CalibrationReport
	err        error
	doneCh     chan struct{}
	cancel     context.CancelFunc

	ttl            time.Duration
	lastActivityAt time.Time

	// Agent-side concurrency tracking.
	agentInFlight int
	// batchPeak is the maximum agentInFlight reached in the current batch.
	// Reset to 0 when agentInFlight drops to 0 (batch complete).
	batchPeak int
	// sessionPeakInFlight is the all-time max agentInFlight seen in this session.
	// Unlike batchPeak, this never resets. Once it reaches desiredCapacity,
	// the capacity gate stays open permanently.
	sessionPeakInFlight int
	// concurrentPullers tracks how many get_next_step calls are blocked
	// right now waiting for a step. If >= desiredCapacity, the agent has
	// proven concurrency (independent worker model) and the gate opens.
	concurrentPullers int
	// peakPullers is the max concurrentPullers seen in this session.
	// Once it reaches desiredCapacity, the gate stays open permanently
	// (the agent has proven it runs enough concurrent workers).
	peakPullers int
	// gateExempt is set when get_next_step returns done or unavailable,
	// signaling the pipeline can't fill capacity. Resets each batch.
	gateExempt bool

	mu sync.Mutex
}

// GetState returns the current session state in a thread-safe manner.
func (s *Session) GetState() SessionState {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.state
}

// PullerEnter is called when a get_next_step call starts blocking for a step.
// Tracks concurrent callers to detect the independent-worker pattern.
func (s *Session) PullerEnter() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.concurrentPullers++
	if s.concurrentPullers > s.peakPullers {
		s.peakPullers = s.concurrentPullers
	}
}

// PullerExit is called when a get_next_step call delivers a step or returns.
func (s *Session) PullerExit() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.concurrentPullers > 0 {
		s.concurrentPullers--
	}
}

// AgentPull increments the agent in-flight counter (called on get_next_step delivery).
func (s *Session) AgentPull() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.agentInFlight++
	if s.agentInFlight > s.batchPeak {
		s.batchPeak = s.agentInFlight
	}
	if s.agentInFlight > s.sessionPeakInFlight {
		s.sessionPeakInFlight = s.agentInFlight
	}
	return s.agentInFlight
}

// AgentSubmit decrements the agent in-flight counter (called on submit_artifact).
func (s *Session) AgentSubmit() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.agentInFlight > 0 {
		s.agentInFlight--
	}
	if s.agentInFlight == 0 {
		s.batchPeak = 0
		s.gateExempt = false
	}
	return s.agentInFlight
}

// SetGateExempt marks the current batch as exempt from the capacity gate.
// Called when get_next_step returns done/unavailable (pipeline can't fill capacity).
func (s *Session) SetGateExempt() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.gateExempt = true
}

// CheckCapacityGate returns an error if the agent hasn't proven it runs
// enough concurrent workers. The gate opens when ANY of:
//   - desiredCapacity <= 1 (serial mode is legitimate)
//   - gateExempt (pipeline draining, fewer steps than capacity)
//   - batchPeak >= desiredCapacity (batch pattern: pulled N before submitting)
//   - sessionPeakInFlight >= desiredCapacity (session-wide proof of parallelism)
//   - peakPullers >= desiredCapacity (worker pattern: N concurrent get_next_step callers observed)
func (s *Session) CheckCapacityGate() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.DesiredCapacity <= 1 {
		return nil
	}
	if s.gateExempt {
		return nil
	}
	if s.batchPeak >= s.DesiredCapacity {
		return nil
	}
	if s.sessionPeakInFlight >= s.DesiredCapacity {
		return nil
	}
	if s.peakPullers >= s.DesiredCapacity {
		return nil
	}

	return fmt.Errorf(
		"CAPACITY GATE ADVISORY: you pulled %d/%d steps this batch (session peak: %d, peak concurrent callers: %d/%d). "+
			"Pull %d more steps via get_next_step before submitting, or run %d concurrent workers. "+
			"If you don't bring more workers, the TTL watchdog will terminate this session",
		s.batchPeak, s.DesiredCapacity, s.sessionPeakInFlight, s.peakPullers, s.DesiredCapacity,
		s.DesiredCapacity-s.batchPeak, s.DesiredCapacity)
}

// AgentInFlight returns how many steps the agent has pulled but not yet submitted.
func (s *Session) AgentInFlight() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.agentInFlight
}

// StartCalibrationInput mirrors the tool arguments for start_calibration.
type StartCalibrationInput struct {
	Scenario    string `json:"scenario"`
	Adapter     string `json:"adapter"`
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

	bus := fwmcp.NewSignalBus()

	sess := &Session{
		ID:              fmt.Sprintf("s-%d", time.Now().UnixMilli()),
		log:             logging.New("mcp-session"),
		state:           StateRunning,
		TotalCases:      len(scenario.Cases),
		Scenario:        scenario.Name,
		DesiredCapacity: parallel,
		Bus:             bus,
		dispatcher:      mcpDisp,
		doneCh:          make(chan struct{}),
		cancel:          runCancel,
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

// SetTTL configures the session inactivity TTL. When no submit_artifact
// arrives for this duration, the session aborts itself. Can be called
// after session creation (e.g. from test hooks).
func (s *Session) SetTTL(ttl time.Duration) {
	s.mu.Lock()
	s.ttl = ttl
	s.lastActivityAt = time.Now()
	s.mu.Unlock()

	go s.watchdog()
}

// touchActivity updates the last-activity timestamp (called on each submit).
func (s *Session) touchActivity() {
	s.mu.Lock()
	prev := s.lastActivityAt
	s.lastActivityAt = time.Now()
	ttl := s.ttl
	s.mu.Unlock()

	if ttl > 0 && !prev.IsZero() {
		s.log.Debug("activity reset", "gap", time.Since(prev), "ttl", ttl)
	}
}

// watchdog monitors session inactivity. If no submit_artifact arrives for
// the configured TTL, the session is aborted. This prevents indefinite
// hangs when the agent side is stuck or disconnected.
func (s *Session) watchdog() {
	s.mu.Lock()
	ttl := s.ttl
	s.mu.Unlock()

	if ttl <= 0 {
		return
	}

	ticker := time.NewTicker(ttl / 5)
	defer ticker.Stop()

	for {
		select {
		case <-s.doneCh:
			return
		case <-ticker.C:
			s.mu.Lock()
			stale := time.Since(s.lastActivityAt)
			currentTTL := s.ttl
			s.mu.Unlock()

			if currentTTL <= 0 {
				return
			}

			if stale > currentTTL {
				s.log.Warn("TTL watchdog triggered, aborting session",
					"stale", stale, "ttl", currentTTL, "session_id", s.ID)
				s.Bus.Emit("session_error", "server", "", "", map[string]string{
					"error": fmt.Sprintf("session TTL expired: no activity for %v", stale),
				})
				s.dispatcher.Abort(fmt.Errorf("session TTL expired: no activity for %v", stale))
				s.mu.Lock()
				s.state = StateError
				s.err = fmt.Errorf("session TTL expired: no activity for %v", stale)
				s.mu.Unlock()
				s.cancel()
				return
			}
		}
	}
}

// Cancel terminates the runner goroutine and releases resources.
func (s *Session) Cancel() {
	if s.cancel != nil {
		s.cancel()
	}
}

// Done returns a channel that closes when the runner goroutine exits.
func (s *Session) Done() <-chan struct{} {
	return s.doneCh
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
		s.log.Error("calibration failed", "error", err)
		return
	}
	s.state = StateDone
	s.report = report
	s.Bus.Emit("session_done", "server", "", "", map[string]string{
		"case_results": fmt.Sprintf("%d", len(report.CaseResults)),
	})
	s.log.Info("calibration complete", "case_results", len(report.CaseResults))
}

// GetNextStep blocks until the runner produces the next prompt, the run
// completes, or the timeout expires. When timeout is 0 it blocks forever
// (backward-compatible). Returns available=false on timeout.
func (s *Session) GetNextStep(ctx context.Context, timeout time.Duration) (dc dispatch.DispatchContext, done bool, available bool, err error) {
	select {
	case <-s.doneCh:
		return dispatch.DispatchContext{}, true, false, nil
	default:
	}

	var timer <-chan time.Time
	if timeout > 0 {
		timer = time.After(timeout)
	}

	start := time.Now()

	select {
	case <-ctx.Done():
		return dispatch.DispatchContext{}, false, false, ctx.Err()
	case <-s.doneCh:
		return dispatch.DispatchContext{}, true, false, nil
	case dc, ok := <-s.dispatcher.PromptCh():
		if !ok {
			return dispatch.DispatchContext{}, true, false, nil
		}
		s.log.Debug("step delivered",
			"case_id", dc.CaseID, "step", dc.Step, "dispatch_id", dc.DispatchID, "wait", time.Since(start))
		return dc, false, true, nil
	case <-timer:
		s.log.Debug("get_next_step timed out, no step available", "timeout", timeout)
		return dispatch.DispatchContext{}, false, false, nil
	}
}

// SubmitArtifact routes the agent's artifact to the correct Dispatch caller.
// If dispatchID is 0, falls back to legacy unrouted submit (serial mode only).
// Strips markdown code fences from LLM responses before validation.
func (s *Session) SubmitArtifact(ctx context.Context, dispatchID int64, data []byte) error {
	data = cleanArtifactJSON(data)
	if !json.Valid(data) {
		return fmt.Errorf("invalid JSON in artifact")
	}
	s.touchActivity()
	return s.dispatcher.SubmitArtifact(ctx, dispatchID, data)
}

// cleanArtifactJSON strips markdown code fences that LLMs often wrap around
// JSON output (e.g. ```json\n{...}\n```).
func cleanArtifactJSON(data []byte) []byte {
	s := bytes.TrimSpace(data)
	if len(s) == 0 {
		return s
	}
	if bytes.HasPrefix(s, []byte("```")) {
		if idx := bytes.IndexByte(s, '\n'); idx >= 0 {
			s = s[idx+1:]
		}
		if bytes.HasSuffix(s, []byte("```")) {
			s = s[:len(s)-3]
		}
		s = bytes.TrimSpace(s)
	}
	return s
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
