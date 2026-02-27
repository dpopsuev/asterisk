package mcpconfig

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"asterisk/internal/calibrate"
	"asterisk/internal/calibrate/adapt"
	"asterisk/internal/calibrate/scenarios"
	"asterisk/internal/orchestrate"
	"asterisk/adapters/rp"
	"asterisk/adapters/store"
	"github.com/dpopsuev/origami/dispatch"
	fwmcp "github.com/dpopsuev/origami/mcp"
)

var (
	DefaultGetNextStepTimeout = 10 * time.Second
	DefaultSessionTTL         = 5 * time.Minute
)

// Server wraps the generic PipelineServer with Asterisk-specific domain hooks.
type Server struct {
	*fwmcp.PipelineServer
	ProjectRoot string
}

// NewServer creates an Asterisk MCP server by configuring the generic
// PipelineServer with Asterisk domain hooks (scenarios, adapters, RP wiring).
func NewServer() *Server {
	cwd, _ := os.Getwd()
	s := &Server{ProjectRoot: cwd}
	s.PipelineServer = fwmcp.NewPipelineServer(s.buildConfig())
	return s
}

func (s *Server) buildConfig() fwmcp.PipelineConfig {
	return fwmcp.PipelineConfig{
		Name:        "asterisk",
		Version:     "dev",
		StepSchemas: asteriskStepSchemas(),
		WorkerPreamble: "You are an Asterisk calibration worker.",
		DefaultGetNextStepTimeout: int(DefaultGetNextStepTimeout / time.Millisecond),
		DefaultSessionTTL:         int(DefaultSessionTTL / time.Millisecond),
		CreateSession: func(ctx context.Context, params fwmcp.StartParams, disp *dispatch.MuxDispatcher, bus *dispatch.SignalBus) (fwmcp.RunFunc, fwmcp.SessionMeta, error) {
			return s.createSession(ctx, params, disp)
		},
		FormatReport: func(result any) (string, any, error) {
			report, ok := result.(*calibrate.CalibrationReport)
			if !ok {
				return "", nil, fmt.Errorf("unexpected result type: %T", result)
			}
			formatted := calibrate.FormatReport(report)
			return formatted, report, nil
		},
	}
}

func (s *Server) createSession(ctx context.Context, params fwmcp.StartParams, disp *dispatch.MuxDispatcher) (fwmcp.RunFunc, fwmcp.SessionMeta, error) {
	extra := params.Extra

	scenarioName, _ := extra["scenario"].(string)
	adapterName, _ := extra["adapter"].(string)
	rpBaseURL, _ := extra["rp_base_url"].(string)
	rpProject, _ := extra["rp_project"].(string)

	scenario, err := loadScenario(scenarioName)
	if err != nil {
		return nil, fwmcp.SessionMeta{}, err
	}

	var rpFetcher rp.EnvelopeFetcher
	if rpBaseURL != "" {
		if rpProject == "" {
			rpProject = os.Getenv("ASTERISK_RP_PROJECT")
		}
		if rpProject == "" {
			return nil, fwmcp.SessionMeta{}, fmt.Errorf("rp_project is required when rp_base_url is set")
		}
		key, err := rp.ReadAPIKey(".rp-api-key")
		if err != nil {
			return nil, fwmcp.SessionMeta{}, fmt.Errorf("read RP API key: %w", err)
		}
		client, err := rp.New(rpBaseURL, key, rp.WithTimeout(30*time.Second))
		if err != nil {
			return nil, fwmcp.SessionMeta{}, fmt.Errorf("create RP client: %w", err)
		}
		rpFetcher = rp.NewFetcher(client, rpProject)
		if err := calibrate.ResolveRPCases(rpFetcher, scenario); err != nil {
			return nil, fwmcp.SessionMeta{}, fmt.Errorf("resolve RP-sourced cases: %w", err)
		}
	}

	root := s.ProjectRoot
	promptDir := filepath.Join(root, ".cursor/prompts")
	basePath := filepath.Join(root, ".asterisk/calibrate")

	tokenTracker := dispatch.NewTokenTracker()
	tracked := dispatch.NewTokenTrackingDispatcher(disp, tokenTracker)

	var adapter calibrate.ModelAdapter
	switch adapterName {
	case "stub":
		adapter = adapt.NewStubAdapter(scenario)
	case "basic":
		basicSt, err := store.Open(":memory:")
		if err != nil {
			return nil, fwmcp.SessionMeta{}, fmt.Errorf("basic adapter: open store: %w", err)
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
		adapter = adapt.NewLLMAdapter(
			promptDir,
			adapt.WithDispatcher(tracked),
			adapt.WithBasePath(basePath),
		)
	}

	parallel := params.Parallel
	if parallel < 1 {
		parallel = 1
	}

	if err := os.MkdirAll(basePath, 0755); err != nil {
		return nil, fwmcp.SessionMeta{}, fmt.Errorf("create calibrate dir: %w", err)
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

	runFn := func(ctx context.Context) (any, error) {
		return calibrate.RunCalibration(ctx, cfg)
	}

	meta := fwmcp.SessionMeta{
		TotalCases: len(scenario.Cases),
		Scenario:   scenario.Name,
	}

	return runFn, meta, nil
}

// asteriskStepSchemas returns the F0-F6 step schemas for Asterisk calibration.
func asteriskStepSchemas() []fwmcp.StepSchema {
	return []fwmcp.StepSchema{
		{
			Name:   "F0_RECALL",
			Fields: map[string]string{"match": "bool", "confidence": "float", "reasoning": "string"},
			Defs: []fwmcp.FieldDef{
				{Name: "match", Type: "bool", Required: true},
				{Name: "confidence", Type: "float", Required: true},
				{Name: "reasoning", Type: "string", Required: true},
			},
		},
		{
			Name: "F1_TRIAGE",
			Fields: map[string]string{
				"symptom_category": "string", "severity": "string",
				"defect_type_hypothesis": "string", "candidate_repos[]": "string[]",
				"skip_investigation": "bool", "cascade_suspected": "bool",
			},
			Defs: []fwmcp.FieldDef{
				{Name: "symptom_category", Type: "string", Required: true},
				{Name: "severity", Type: "string", Required: true},
				{Name: "defect_type_hypothesis", Type: "string", Required: true},
				{Name: "candidate_repos", Type: "array", Required: false},
				{Name: "skip_investigation", Type: "bool", Required: false},
				{Name: "cascade_suspected", Type: "bool", Required: false},
			},
		},
		{
			Name:   "F2_RESOLVE",
			Fields: map[string]string{"selected_repos[]": "{name, reason}"},
			Defs: []fwmcp.FieldDef{
				{Name: "selected_repos", Type: "array", Required: true},
			},
		},
		{
			Name: "F3_INVESTIGATE",
			Fields: map[string]string{
				"rca_message": "string", "defect_type": "string", "component": "string",
				"convergence_score": "float", "evidence_refs[]": "string[]",
			},
			Defs: []fwmcp.FieldDef{
				{Name: "rca_message", Type: "string", Required: true},
				{Name: "defect_type", Type: "string", Required: true},
				{Name: "component", Type: "string", Required: true},
				{Name: "convergence_score", Type: "float", Required: false},
				{Name: "evidence_refs", Type: "array", Required: false},
			},
		},
		{
			Name:   "F4_CORRELATE",
			Fields: map[string]string{"is_duplicate": "bool", "confidence": "float"},
			Defs: []fwmcp.FieldDef{
				{Name: "is_duplicate", Type: "bool", Required: true},
				{Name: "confidence", Type: "float", Required: true},
			},
		},
		{
			Name:   "F5_REVIEW",
			Fields: map[string]string{"decision": "approve|reassess|overturn"},
			Defs: []fwmcp.FieldDef{
				{Name: "decision", Type: "string", Required: true},
			},
		},
		{
			Name:   "F6_REPORT",
			Fields: map[string]string{"defect_type": "string", "case_id": "string", "summary": "string"},
			Defs: []fwmcp.FieldDef{
				{Name: "defect_type", Type: "string", Required: true},
				{Name: "case_id", Type: "string", Required: true},
				{Name: "summary", Type: "string", Required: true},
			},
		},
	}
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
