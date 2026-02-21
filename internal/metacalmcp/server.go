package metacalmcp

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"

	"asterisk/pkg/framework"
	fwmcp "asterisk/pkg/framework/mcp"
	"asterisk/pkg/framework/metacal"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// Server wraps the MCP SDK server and manages discovery sessions.
type Server struct {
	MCPServer *sdkmcp.Server
	RunsDir   string

	mu        sync.Mutex
	session   *Session
	persisted bool
	log       *slog.Logger
}

// NewServer creates a metacal MCP server with discovery tools registered.
func NewServer(runsDir string) *Server {
	fw := fwmcp.NewServer("metacal", "dev")
	s := &Server{
		MCPServer: fw.MCPServer,
		RunsDir:   runsDir,
		log:       slog.Default().With("component", "metacal-mcp"),
	}
	s.registerTools()
	return s
}

func (s *Server) registerTools() {
	sdkmcp.AddTool(s.MCPServer, &sdkmcp.Tool{
		Name:        "start_discovery",
		Description: "Start a model discovery session. Returns session ID and config.",
	}, s.handleStartDiscovery)

	sdkmcp.AddTool(s.MCPServer, &sdkmcp.Tool{
		Name:        "get_discovery_prompt",
		Description: "Get the next discovery prompt for the current iteration. Returns done=true when discovery is complete.",
	}, s.handleGetDiscoveryPrompt)

	sdkmcp.AddTool(s.MCPServer, &sdkmcp.Tool{
		Name:        "submit_discovery_response",
		Description: "Submit a subagent's raw response text. Server parses identity, scores probe, updates seen map.",
	}, s.handleSubmitDiscoveryResponse)

	sdkmcp.AddTool(s.MCPServer, &sdkmcp.Tool{
		Name:        "get_discovery_report",
		Description: "Get the final discovery report with all discovered models and probe scores.",
	}, s.handleGetDiscoveryReport)

	sdkmcp.AddTool(s.MCPServer, &sdkmcp.Tool{
		Name:        "emit_signal",
		Description: "Emit a signal to the agent message bus for observability.",
	}, s.handleEmitSignal)

	sdkmcp.AddTool(s.MCPServer, &sdkmcp.Tool{
		Name:        "get_signals",
		Description: "Read signals from the agent message bus. Returns all signals, or signals since a given index.",
	}, s.handleGetSignals)
}

// --- Tool input/output types ---

type startDiscoveryInput struct {
	MaxIterations     int    `json:"max_iterations,omitempty" jsonschema:"max discovery iterations (default 15)"`
	ProbeID           string `json:"probe_id,omitempty" jsonschema:"probe identifier (default refactor-v1)"`
	TerminateOnRepeat *bool  `json:"terminate_on_repeat,omitempty" jsonschema:"stop when a model repeats (default true)"`
}

type startDiscoveryOutput struct {
	SessionID     string `json:"session_id"`
	MaxIterations int    `json:"max_iterations"`
	ProbeID       string `json:"probe_id"`
	Status        string `json:"status"`
}

type getDiscoveryPromptInput struct {
	SessionID string `json:"session_id" jsonschema:"session ID from start_discovery"`
}

type getDiscoveryPromptOutput struct {
	Done      bool   `json:"done"`
	Prompt    string `json:"prompt,omitempty"`
	Iteration int    `json:"iteration"`
}

type submitDiscoveryResponseInput struct {
	SessionID string `json:"session_id" jsonschema:"session ID from start_discovery"`
	Response  string `json:"response" jsonschema:"raw text response from the subagent"`
}

type submitDiscoveryResponseOutput struct {
	ModelName string           `json:"model_name"`
	Provider  string           `json:"provider"`
	Key       string           `json:"key"`
	Score     metacal.ProbeScore `json:"score"`
	Repeated  bool             `json:"repeated"`
	Known     bool             `json:"known"`
	Iteration int              `json:"iteration"`
	Done      bool             `json:"done"`
}

type getDiscoveryReportInput struct {
	SessionID string `json:"session_id" jsonschema:"session ID from start_discovery"`
}

type getDiscoveryReportOutput struct {
	Status       string                    `json:"status"`
	UniqueModels int                       `json:"unique_models"`
	TermReason   string                    `json:"term_reason,omitempty"`
	Report       *metacal.RunReport        `json:"report,omitempty"`
	ModelNames   []string                  `json:"model_names,omitempty"`
	Error        string                    `json:"error,omitempty"`
}

type emitSignalInput struct {
	SessionID string            `json:"session_id" jsonschema:"session ID from start_discovery"`
	Event     string            `json:"event" jsonschema:"signal event"`
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
	SessionID string `json:"session_id" jsonschema:"session ID from start_discovery"`
	Since     int    `json:"since,omitempty" jsonschema:"return signals from this index onward (0-based)"`
}

type getSignalsOutput struct {
	Signals []fwmcp.Signal `json:"signals"`
	Total   int            `json:"total"`
}

// --- Tool handlers ---

func (s *Server) handleStartDiscovery(_ context.Context, _ *sdkmcp.CallToolRequest, input startDiscoveryInput) (*sdkmcp.CallToolResult, startDiscoveryOutput, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.session != nil && s.session.GetState() == StateRunning {
		return nil, startDiscoveryOutput{}, fmt.Errorf("a discovery session is already running (id=%s)", s.session.ID)
	}

	config := metacal.DefaultConfig()
	if input.MaxIterations > 0 {
		config.MaxIterations = input.MaxIterations
	}
	if input.ProbeID != "" {
		config.ProbeID = input.ProbeID
	}
	if input.TerminateOnRepeat != nil {
		config.TerminateOnRepeat = *input.TerminateOnRepeat
	}

	sess := NewSession(config)
	s.session = sess
	s.persisted = false
	s.log.Info("discovery session started", "id", sess.ID, "max_iterations", config.MaxIterations)

	return nil, startDiscoveryOutput{
		SessionID:     sess.ID,
		MaxIterations: config.MaxIterations,
		ProbeID:       config.ProbeID,
		Status:        string(StateRunning),
	}, nil
}

func (s *Server) handleGetDiscoveryPrompt(_ context.Context, _ *sdkmcp.CallToolRequest, input getDiscoveryPromptInput) (*sdkmcp.CallToolResult, getDiscoveryPromptOutput, error) {
	sess, err := s.getSession(input.SessionID)
	if err != nil {
		return nil, getDiscoveryPromptOutput{}, err
	}

	prompt, done := sess.NextPrompt()
	return nil, getDiscoveryPromptOutput{
		Done:      done,
		Prompt:    prompt,
		Iteration: sess.iteration,
	}, nil
}

func (s *Server) handleSubmitDiscoveryResponse(_ context.Context, _ *sdkmcp.CallToolRequest, input submitDiscoveryResponseInput) (*sdkmcp.CallToolResult, submitDiscoveryResponseOutput, error) {
	sess, err := s.getSession(input.SessionID)
	if err != nil {
		return nil, submitDiscoveryResponseOutput{}, err
	}

	if input.Response == "" {
		return nil, submitDiscoveryResponseOutput{}, fmt.Errorf("response is required")
	}

	result, repeated, err := sess.SubmitResponse(input.Response)
	if err != nil {
		return nil, submitDiscoveryResponseOutput{}, fmt.Errorf("submit_discovery_response: %w", err)
	}

	done := sess.GetState() != StateRunning

	return nil, submitDiscoveryResponseOutput{
		ModelName: result.Model.ModelName,
		Provider:  result.Model.Provider,
		Key:       metacal.ModelKey(result.Model),
		Score:     result.Probe.Score,
		Repeated:  repeated,
		Known:     framework.IsKnownModel(result.Model),
		Iteration: result.Iteration,
		Done:      done,
	}, nil
}

func (s *Server) handleGetDiscoveryReport(_ context.Context, _ *sdkmcp.CallToolRequest, input getDiscoveryReportInput) (*sdkmcp.CallToolResult, getDiscoveryReportOutput, error) {
	sess, err := s.getSession(input.SessionID)
	if err != nil {
		return nil, getDiscoveryReportOutput{}, err
	}

	state := sess.GetState()
	if state == StateRunning {
		sess.Finalize("report_requested")
	}

	report := sess.GetReport()
	if report == nil {
		return nil, getDiscoveryReportOutput{Status: "no_report"}, nil
	}

	s.mu.Lock()
	alreadyPersisted := s.persisted
	s.mu.Unlock()

	if s.RunsDir != "" && !alreadyPersisted {
		store, storeErr := metacal.NewFileRunStore(s.RunsDir)
		if storeErr == nil {
			if saveErr := store.SaveRun(*report); saveErr != nil {
				s.log.Warn("failed to persist run report", "error", saveErr)
			} else {
				s.mu.Lock()
				s.persisted = true
				s.mu.Unlock()
				s.log.Info("run report persisted", "run_id", report.RunID, "dir", s.RunsDir)
			}
		}
	}

	out := getDiscoveryReportOutput{
		Status:       string(StateDone),
		UniqueModels: len(report.UniqueModels),
		TermReason:   report.TermReason,
		Report:       report,
		ModelNames:   report.ModelNames(),
	}

	reportJSON, err := json.MarshalIndent(report, "", "  ")
	if err == nil {
		s.log.Info("discovery report", "report_size", len(reportJSON))
	}

	return nil, out, nil
}

func (s *Server) handleEmitSignal(_ context.Context, _ *sdkmcp.CallToolRequest, input emitSignalInput) (*sdkmcp.CallToolResult, emitSignalOutput, error) {
	if input.Event == "" {
		return nil, emitSignalOutput{}, fmt.Errorf("event is required")
	}
	if input.Agent == "" {
		return nil, emitSignalOutput{}, fmt.Errorf("agent is required")
	}

	sess, err := s.getSession(input.SessionID)
	if err != nil {
		return nil, emitSignalOutput{}, err
	}

	sess.Bus.Emit(input.Event, input.Agent, input.CaseID, input.Step, input.Meta)
	idx := sess.Bus.Len()

	return nil, emitSignalOutput{
		OK:    "signal emitted",
		Index: idx,
	}, nil
}

func (s *Server) handleGetSignals(_ context.Context, _ *sdkmcp.CallToolRequest, input getSignalsInput) (*sdkmcp.CallToolResult, getSignalsOutput, error) {
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

// Shutdown cleans up any active session.
func (s *Server) Shutdown() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.session != nil && s.session.GetState() == StateRunning {
		s.session.Finalize("shutdown")
	}
	s.session = nil
}

// SessionID returns the current session's ID, or empty string if none.
func (s *Server) SessionID() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.session != nil {
		return s.session.ID
	}
	return ""
}

func (s *Server) getSession(id string) (*Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.session == nil {
		return nil, fmt.Errorf("no active session (call start_discovery first)")
	}
	if s.session.ID != id {
		return nil, fmt.Errorf("session_id mismatch: have %s, got %s", s.session.ID, id)
	}
	return s.session, nil
}
