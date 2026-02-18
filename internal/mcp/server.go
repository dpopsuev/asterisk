package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sync"

	"asterisk/internal/calibrate"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// Server wraps the MCP SDK server and manages calibration sessions.
type Server struct {
	MCPServer   *sdkmcp.Server
	ProjectRoot string

	mu      sync.Mutex
	session *Session
}

// NewServer creates an MCP server with calibration and signal bus tools.
// It captures the current working directory as the project root so relative
// paths (prompt templates, artifact dirs) resolve correctly.
func NewServer() *Server {
	cwd, _ := os.Getwd()
	s := &Server{ProjectRoot: cwd}
	s.MCPServer = sdkmcp.NewServer(
		&sdkmcp.Implementation{Name: "asterisk", Version: "dev"},
		nil,
	)
	s.registerTools()
	return s
}

func (s *Server) registerTools() {
	sdkmcp.AddTool(s.MCPServer, &sdkmcp.Tool{
		Name:        "start_calibration",
		Description: "Start a calibration run. Spawns the runner goroutine and returns a session ID.",
	}, s.handleStartCalibration)

	sdkmcp.AddTool(s.MCPServer, &sdkmcp.Tool{
		Name:        "get_next_step",
		Description: "Get the next pipeline step prompt. Blocks until the runner is ready. Returns done=true when all cases are complete.",
	}, s.handleGetNextStep)

	sdkmcp.AddTool(s.MCPServer, &sdkmcp.Tool{
		Name:        "submit_artifact",
		Description: "Submit a JSON artifact for the current pipeline step. The runner scores it and advances.",
	}, s.handleSubmitArtifact)

	sdkmcp.AddTool(s.MCPServer, &sdkmcp.Tool{
		Name:        "get_report",
		Description: "Get the final calibration report with M1-M20+M14b scorecard and per-case results.",
	}, s.handleGetReport)

	sdkmcp.AddTool(s.MCPServer, &sdkmcp.Tool{
		Name:        "emit_signal",
		Description: "Emit a signal to the agent message bus for observability. Use to announce dispatch, start, done, error events.",
	}, s.handleEmitSignal)

	sdkmcp.AddTool(s.MCPServer, &sdkmcp.Tool{
		Name:        "get_signals",
		Description: "Read signals from the agent message bus. Returns all signals, or signals since a given index.",
	}, s.handleGetSignals)
}

// --- Tool input/output types ---

type startCalibrationInput struct {
	Scenario  string `json:"scenario" jsonschema:"scenario name (ptp-mock, daemon-mock, ptp-real, ptp-real-ingest)"`
	Adapter   string `json:"adapter" jsonschema:"model adapter (stub, basic, cursor)"`
	Grade     string `json:"grade,omitempty" jsonschema:"filter by evidence grade (A, B, C, or comma-separated)"`
	RPBaseURL string `json:"rp_base_url,omitempty" jsonschema:"ReportPortal base URL for RP-sourced cases"`
	RPProject string `json:"rp_project,omitempty" jsonschema:"ReportPortal project name"`
	Parallel  int    `json:"parallel,omitempty" jsonschema:"number of parallel workers (default 1 = serial)"`
}

type startCalibrationOutput struct {
	SessionID  string `json:"session_id"`
	TotalCases int    `json:"total_cases"`
	Scenario   string `json:"scenario"`
	Status     string `json:"status"`
}

type getNextStepInput struct {
	SessionID string `json:"session_id" jsonschema:"session ID from start_calibration"`
}

type getNextStepOutput struct {
	Done         bool   `json:"done"`
	CaseID       string `json:"case_id,omitempty"`
	Step         string `json:"step,omitempty"`
	PromptPath   string `json:"prompt_path,omitempty"`
	ArtifactPath string `json:"artifact_path,omitempty"`
	DispatchID   int64  `json:"dispatch_id,omitempty"`
}

type submitArtifactInput struct {
	SessionID    string `json:"session_id" jsonschema:"session ID from start_calibration"`
	ArtifactJSON string `json:"artifact_json" jsonschema:"JSON artifact string for this pipeline step"`
	DispatchID   int64  `json:"dispatch_id,omitempty" jsonschema:"dispatch ID from get_next_step for artifact routing"`
}

type submitArtifactOutput struct {
	OK string `json:"ok"`
}

type getReportInput struct {
	SessionID string `json:"session_id" jsonschema:"session ID from start_calibration"`
}

type getReportOutput struct {
	Status      string                       `json:"status"`
	Report      string                       `json:"report,omitempty"`
	Metrics     *calibrate.MetricSet         `json:"metrics,omitempty"`
	CaseResults []calibrate.CaseResult       `json:"case_results,omitempty"`
	Error       string                       `json:"error,omitempty"`
}

type emitSignalInput struct {
	SessionID string            `json:"session_id" jsonschema:"session ID from start_calibration"`
	Event     string            `json:"event" jsonschema:"signal event (dispatch, start, done, error, loop)"`
	Agent     string            `json:"agent" jsonschema:"agent type (main, sub, server)"`
	CaseID    string            `json:"case_id,omitempty" jsonschema:"case ID if applicable"`
	Step      string            `json:"step,omitempty" jsonschema:"pipeline step if applicable"`
	Meta      map[string]string `json:"meta,omitempty" jsonschema:"optional key-value metadata"`
}

type emitSignalOutput struct {
	OK    string `json:"ok"`
	Index int    `json:"index"`
}

type getSignalsInput struct {
	SessionID string `json:"session_id" jsonschema:"session ID from start_calibration"`
	Since     int    `json:"since,omitempty" jsonschema:"return signals from this index onward (0-based)"`
}

type getSignalsOutput struct {
	Signals []Signal `json:"signals"`
	Total   int      `json:"total"`
}

// --- Tool handlers ---

func (s *Server) handleStartCalibration(ctx context.Context, _ *sdkmcp.CallToolRequest, input startCalibrationInput) (*sdkmcp.CallToolResult, startCalibrationOutput, error) {
	s.mu.Lock()
	if s.session != nil {
		select {
		case <-s.session.Done():
			s.session.Cancel()
		default:
			s.mu.Unlock()
			return nil, startCalibrationOutput{}, fmt.Errorf("a calibration session is already running (id=%s)", s.session.ID)
		}
	}
	s.session = nil
	s.mu.Unlock()

	sess, err := NewSession(ctx, StartCalibrationInput{
		Scenario:    input.Scenario,
		Adapter:     input.Adapter,
		Grade:       input.Grade,
		RPBaseURL:   input.RPBaseURL,
		RPProject:   input.RPProject,
		Parallel:    input.Parallel,
		ProjectRoot: s.ProjectRoot,
	})
	if err != nil {
		return nil, startCalibrationOutput{}, fmt.Errorf("start calibration: %w", err)
	}

	s.mu.Lock()
	s.session = sess
	s.mu.Unlock()

	return nil, startCalibrationOutput{
		SessionID:  sess.ID,
		TotalCases: sess.TotalCases,
		Scenario:   sess.Scenario,
		Status:     string(StateRunning),
	}, nil
}

func (s *Server) handleGetNextStep(ctx context.Context, _ *sdkmcp.CallToolRequest, input getNextStepInput) (*sdkmcp.CallToolResult, getNextStepOutput, error) {
	sess, err := s.getSession(input.SessionID)
	if err != nil {
		return nil, getNextStepOutput{}, err
	}

	dc, done, err := sess.GetNextStep(ctx)
	if err != nil {
		return nil, getNextStepOutput{}, fmt.Errorf("get_next_step: %w", err)
	}

	if done {
		sess.Bus.Emit("pipeline_done", "server", "", "", nil)
		return nil, getNextStepOutput{Done: true}, nil
	}

	sess.Bus.Emit("step_ready", "server", dc.CaseID, dc.Step, map[string]string{
		"prompt_path": dc.PromptPath,
	})

	return nil, getNextStepOutput{
		Done:         false,
		CaseID:       dc.CaseID,
		Step:         dc.Step,
		PromptPath:   dc.PromptPath,
		ArtifactPath: dc.ArtifactPath,
		DispatchID:   dc.DispatchID,
	}, nil
}

func (s *Server) handleSubmitArtifact(ctx context.Context, _ *sdkmcp.CallToolRequest, input submitArtifactInput) (*sdkmcp.CallToolResult, submitArtifactOutput, error) {
	sess, err := s.getSession(input.SessionID)
	if err != nil {
		return nil, submitArtifactOutput{}, err
	}

	data := []byte(input.ArtifactJSON)
	if !json.Valid(data) {
		return nil, submitArtifactOutput{}, fmt.Errorf("artifact_json is not valid JSON")
	}

	if err := sess.SubmitArtifact(ctx, input.DispatchID, data); err != nil {
		return nil, submitArtifactOutput{}, fmt.Errorf("submit_artifact: %w", err)
	}

	sess.Bus.Emit("artifact_submitted", "server", "", "", map[string]string{
		"bytes": fmt.Sprintf("%d", len(data)),
	})

	return nil, submitArtifactOutput{OK: "artifact accepted"}, nil
}

func (s *Server) handleGetReport(ctx context.Context, _ *sdkmcp.CallToolRequest, input getReportInput) (*sdkmcp.CallToolResult, getReportOutput, error) {
	sess, err := s.getSession(input.SessionID)
	if err != nil {
		return nil, getReportOutput{}, err
	}

	select {
	case <-sess.Done():
	case <-ctx.Done():
		return nil, getReportOutput{}, ctx.Err()
	}

	if sessErr := sess.Err(); sessErr != nil {
		return nil, getReportOutput{
			Status: string(StateError),
			Error:  sessErr.Error(),
		}, nil
	}

	report := sess.Report()
	if report == nil {
		return nil, getReportOutput{Status: "no_report"}, nil
	}

	formatted := calibrate.FormatReport(report)

	return nil, getReportOutput{
		Status:      string(StateDone),
		Report:      formatted,
		Metrics:     &report.Metrics,
		CaseResults: report.CaseResults,
	}, nil
}

func (s *Server) handleEmitSignal(ctx context.Context, _ *sdkmcp.CallToolRequest, input emitSignalInput) (*sdkmcp.CallToolResult, emitSignalOutput, error) {
	if input.Event == "" {
		log.Printf("[signal-bus] WARN: emit_signal rejected: empty event field")
		return nil, emitSignalOutput{}, fmt.Errorf("event is required")
	}
	if input.Agent == "" {
		log.Printf("[signal-bus] WARN: emit_signal rejected: empty agent field")
		return nil, emitSignalOutput{}, fmt.Errorf("agent is required")
	}

	sess, err := s.getSession(input.SessionID)
	if err != nil {
		return nil, emitSignalOutput{}, err
	}

	sess.Bus.Emit(input.Event, input.Agent, input.CaseID, input.Step, input.Meta)
	idx := sess.Bus.Len()
	log.Printf("[signal-bus] signal #%d: event=%s agent=%s case=%s step=%s", idx, input.Event, input.Agent, input.CaseID, input.Step)

	return nil, emitSignalOutput{
		OK:    "signal emitted",
		Index: idx,
	}, nil
}

func (s *Server) handleGetSignals(ctx context.Context, _ *sdkmcp.CallToolRequest, input getSignalsInput) (*sdkmcp.CallToolResult, getSignalsOutput, error) {
	sess, err := s.getSession(input.SessionID)
	if err != nil {
		return nil, getSignalsOutput{}, err
	}

	signals := sess.Bus.Since(input.Since)
	return nil, getSignalsOutput{
		Signals: signals,
		Total:   sess.Bus.Len(),
	}, nil
}

// Shutdown cancels any active session, releasing runner goroutines.
func (s *Server) Shutdown() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.session != nil {
		s.session.Cancel()
		s.session = nil
	}
}

func (s *Server) getSession(id string) (*Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.session == nil {
		return nil, fmt.Errorf("no active session (call start_calibration first)")
	}
	if s.session.ID != id {
		return nil, fmt.Errorf("session_id mismatch: have %s, got %s", s.session.ID, id)
	}
	return s.session, nil
}
