// asterisk is the main CLI: analyze, push, cursor (orchestrated prompts), save, status.
//
// Usage:
//
//	asterisk analyze --launch=<path|id> [--workspace=<path>] -o <artifact>
//	asterisk push -f <artifact-path>
//	asterisk cursor --launch=<path|id> [--workspace=<path>] [--case-id=<id>]
//	asterisk save -f <artifact-path> --case-id=<id> --suite-id=<id>
//	asterisk status --case-id=<id> --suite-id=<id>
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"log/slog"

	"asterisk/internal/calibrate"
	"asterisk/internal/calibrate/scenarios"
	"asterisk/internal/investigate"
	"asterisk/internal/orchestrate"
	"asterisk/internal/postinvest"
	"asterisk/internal/preinvest"
	"asterisk/internal/rp"
	"asterisk/internal/store"
	"asterisk/internal/workspace"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}
	sub := os.Args[1]
	args := os.Args[2:]
	switch sub {
	case "analyze":
		runAnalyze(args)
	case "push":
		runPush(args)
	case "cursor":
		runCursor(args)
	case "save":
		runSave(args)
	case "status":
		runStatus(args)
	case "calibrate":
		runCalibrate(args)
	default:
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Fprintf(os.Stderr, "usage: asterisk <analyze|push|cursor|save|status|calibrate> [options]\n")
	fmt.Fprintf(os.Stderr, "  asterisk analyze    --launch=<path|id> [--workspace=<path>] [--adapter=basic] -o <artifact>\n")
	fmt.Fprintf(os.Stderr, "  asterisk push       -f <artifact-path>\n")
	fmt.Fprintf(os.Stderr, "  asterisk cursor     --launch=<path|id> [--workspace=<path>] [--case-id=<id>]\n")
	fmt.Fprintf(os.Stderr, "  asterisk save       -f <artifact-path> --case-id=<id> --suite-id=<id>\n")
	fmt.Fprintf(os.Stderr, "  asterisk status     --case-id=<id> --suite-id=<id>\n")
	fmt.Fprintf(os.Stderr, "  asterisk calibrate  --scenario=<name> [--runs=N] [--adapter=stub|cursor] [--dispatch=stdin|file] [--agent-debug]\n")
}

func runAnalyze(args []string) {
	fs := flag.NewFlagSet("analyze", flag.ExitOnError)
	launch := fs.String("launch", "", "Path to envelope JSON or launch ID (required)")
	workspacePath := fs.String("workspace", "", "Path to context workspace file (YAML/JSON)")
	artifactPath := fs.String("o", "", "Output artifact path (required)")
	dbPath := fs.String("db", store.DefaultDBPath, "Store DB path")
	adapterName := fs.String("adapter", "basic", "Adapter: basic (heuristic, default)")
	rpBase := fs.String("rp-base-url", "", "RP base URL (optional; for fetch by launch ID)")
	rpKeyPath := fs.String("rp-api-key", ".rp-api-key", "Path to RP API key file")
	_ = fs.Parse(args)

	if *launch == "" || *artifactPath == "" {
		fs.PrintDefaults()
		os.Exit(1)
	}

	// Load envelope
	env := loadEnvelopeForAnalyze(*launch, *dbPath, *rpBase, *rpKeyPath)
	if env == nil {
		fmt.Fprintf(os.Stderr, "could not load envelope for launch %q\n", *launch)
		os.Exit(1)
	}
	if len(env.FailureList) == 0 {
		fmt.Fprintf(os.Stderr, "envelope has no failures\n")
		os.Exit(1)
	}

	// Open store for v2 pipeline
	st, err := store.Open(*dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "open store: %v\n", err)
		os.Exit(1)
	}
	defer st.Close()

	// Load workspace
	var repoNames []string
	if *workspacePath != "" {
		ws, err := workspace.LoadFromPath(*workspacePath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "load workspace: %v\n", err)
			os.Exit(1)
		}
		for _, r := range ws.Repos {
			repoNames = append(repoNames, r.Name)
		}
	}

	// Create v2 store scaffolding for all failures
	suiteID, cases := createAnalysisScaffolding(st, env)

	// Create adapter
	var adapter calibrate.ModelAdapter
	switch *adapterName {
	case "basic":
		ba := calibrate.NewBasicAdapter(st, repoNames)
		for i, c := range cases {
			label := fmt.Sprintf("A%d", i+1)
			ba.RegisterCase(label, &calibrate.BasicCaseInfo{
				Name:         c.Name,
				ErrorMessage: c.ErrorMessage,
				LogSnippet:   c.LogSnippet,
				StoreCaseID:  c.ID,
			})
		}
		adapter = ba
	default:
		fmt.Fprintf(os.Stderr, "unknown adapter: %s (supported: basic)\n", *adapterName)
		os.Exit(1)
	}

	// Run analysis through the v2 pipeline
	cfg := calibrate.AnalysisConfig{
		Adapter:    adapter,
		Thresholds: orchestrate.DefaultThresholds(),
	}
	report, err := calibrate.RunAnalysis(st, cases, suiteID, cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "analyze: %v\n", err)
		os.Exit(1)
	}
	report.LaunchName = env.Name

	// Write JSON report to artifact path
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "marshal report: %v\n", err)
		os.Exit(1)
	}
	if err := os.WriteFile(*artifactPath, data, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "write report: %v\n", err)
		os.Exit(1)
	}

	// Print human-readable report to stdout
	fmt.Print(calibrate.FormatAnalysisReport(report))
	fmt.Printf("\nReport written to: %s\n", *artifactPath)
}

// loadEnvelopeForAnalyze resolves the envelope from a file path or launch ID.
func loadEnvelopeForAnalyze(launch, dbPath, rpBase, rpKeyPath string) *preinvest.Envelope {
	if _, err := os.Stat(launch); err == nil {
		data, err := os.ReadFile(launch)
		if err != nil {
			return nil
		}
		var env preinvest.Envelope
		if err := json.Unmarshal(data, &env); err != nil {
			return nil
		}
		return &env
	}

	launchID, err := strconv.Atoi(launch)
	if err != nil || launchID <= 0 {
		return nil
	}
	st, err := store.Open(dbPath)
	if err != nil {
		return nil
	}
	defer st.Close()

	env, _ := st.GetEnvelope(launchID)
	if env == nil && rpBase != "" {
		key, _ := rp.ReadAPIKey(rpKeyPath)
		client, err := rp.New(rpBase, key)
		if err != nil {
			return nil
		}
		fetcher := rp.NewFetcher(client, "ecosystem-qe")
		adapter := &store.PreinvestStoreAdapter{Store: st}
		if err := preinvest.FetchAndSave(fetcher, adapter, launchID); err != nil {
			return nil
		}
		env, _ = adapter.Get(launchID)
	}
	return env
}

// createAnalysisScaffolding creates v2 store entities (suite, version, pipeline,
// launch, job, cases) for all failures in the envelope.
func createAnalysisScaffolding(st store.Store, env *preinvest.Envelope) (int64, []*store.Case) {
	rpLaunchID, _ := strconv.Atoi(env.RunID)

	suiteID, _ := st.CreateSuite(&store.InvestigationSuite{
		Name:        fmt.Sprintf("Analysis %s", env.Name),
		Description: fmt.Sprintf("Automated analysis for launch %s", env.RunID),
		Status:      "active",
	})

	vID, _ := st.CreateVersion(&store.Version{Label: "unknown"})
	if vID == 0 {
		v, _ := st.GetVersionByLabel("unknown")
		if v != nil {
			vID = v.ID
		}
	}

	pID, _ := st.CreatePipeline(&store.Pipeline{
		SuiteID:    suiteID,
		VersionID:  vID,
		Name:       env.Name,
		RPLaunchID: rpLaunchID,
		Status:     "complete",
	})

	lID, _ := st.CreateLaunch(&store.Launch{
		PipelineID: pID,
		RPLaunchID: rpLaunchID,
		Name:       env.Name,
		Status:     "complete",
	})

	jID, _ := st.CreateJob(&store.Job{
		LaunchID: lID,
		Name:     env.Name,
		Status:   "complete",
	})

	var cases []*store.Case
	for _, f := range env.FailureList {
		caseID, _ := st.CreateCaseV2(&store.Case{
			JobID:    jID,
			LaunchID: lID,
			RPItemID: f.ID,
			Name:     f.Name,
			Status:   "open",
		})
		c, _ := st.GetCaseV2(caseID)
		if c != nil {
			cases = append(cases, c)
		}
	}

	return suiteID, cases
}

func runPush(args []string) {
	fs := flag.NewFlagSet("push", flag.ExitOnError)
	artifactPath := fs.String("f", "", "Artifact file path (required)")
	rpBase := fs.String("rp-base-url", "", "RP base URL (optional)")
	rpKeyPath := fs.String("rp-api-key", ".rp-api-key", "Path to RP API key file")
	_ = fs.Parse(args)

	if *artifactPath == "" {
		fs.PrintDefaults()
		os.Exit(1)
	}

	pushStore := postinvest.NewMemPushStore()
	var pusher postinvest.Pusher = postinvest.DefaultPusher{}
	if *rpBase != "" {
		key, err := rp.ReadAPIKey(*rpKeyPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "read API key: %v\n", err)
			os.Exit(1)
		}
		client, err := rp.New(*rpBase, key)
		if err != nil {
			fmt.Fprintf(os.Stderr, "create RP client: %v\n", err)
			os.Exit(1)
		}
		pusher = rp.NewPusher(client, "ecosystem-qe")
	}
	if err := pusher.Push(*artifactPath, pushStore, "", ""); err != nil {
		fmt.Fprintf(os.Stderr, "push: %v\n", err)
		os.Exit(1)
	}
	rec := pushStore.LastPushed()
	if rec != nil {
		fmt.Printf("Pushed: launch=%s defect_type=%s\n", rec.LaunchID, rec.DefectType)
	}
}

func runCursor(args []string) {
	fs := flag.NewFlagSet("cursor", flag.ExitOnError)
	launch := fs.String("launch", "", "Path to envelope JSON or launch ID (required)")
	workspacePath := fs.String("workspace", "", "Path to context workspace file (YAML/JSON)")
	itemID := fs.Int("case-id", 0, "Failure (test item) RP ID; default first from envelope")
	promptDir := fs.String("prompt-dir", ".cursor/prompts", "Directory containing prompt templates")
	dbPath := fs.String("db", store.DefaultDBPath, "Store DB path")
	_ = fs.Parse(args)

	if *launch == "" {
		fs.PrintDefaults()
		os.Exit(1)
	}

	// Load envelope
	env, rpLaunchID := loadEnvelopeForCursor(*launch, *dbPath)
	if env == nil {
		fmt.Fprintf(os.Stderr, "could not load envelope for launch %q\n", *launch)
		os.Exit(1)
	}
	if len(env.FailureList) == 0 {
		fmt.Fprintf(os.Stderr, "envelope has no failures\n")
		os.Exit(1)
	}

	// Select failure item
	item := env.FailureList[0]
	for _, f := range env.FailureList {
		if *itemID == 0 || f.ID == *itemID {
			item = f
			break
		}
	}
	if *itemID != 0 && item.ID != *itemID {
		fmt.Fprintf(os.Stderr, "case-id %d not in envelope\n", *itemID)
		os.Exit(1)
	}

	// Open store
	st, err := store.Open(*dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "open store: %v\n", err)
		os.Exit(1)
	}
	defer st.Close()

	// Ensure case exists in the store (find or create scaffolding)
	caseData := ensureCaseInStore(st, env, rpLaunchID, item)

	// Load workspace
	var ws *workspace.Workspace
	if *workspacePath != "" {
		w, err := workspace.LoadFromPath(*workspacePath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "load workspace: %v\n", err)
			os.Exit(1)
		}
		ws = w
	}

	// Run the orchestrator
	cfg := orchestrate.RunnerConfig{
		PromptDir:  *promptDir,
		Thresholds: orchestrate.DefaultThresholds(),
	}
	result, err := orchestrate.RunStep(st, caseData, env, ws, cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "orchestrate: %v\n", err)
		os.Exit(1)
	}

	if result.IsDone {
		fmt.Printf("Pipeline complete for case #%d. %s\n", caseData.ID, result.Explanation)
		return
	}

	fmt.Printf("Step: %s\n", result.NextStep)
	fmt.Printf("Prompt: %s\n", result.PromptPath)
	fmt.Printf("\nPaste the prompt into Cursor, then save the artifact to the case directory.\n")
	fmt.Printf("Run 'asterisk cursor' again to advance to the next step.\n")
}

// ensureCaseInStore finds or creates the full v2 scaffolding for a failure item.
func ensureCaseInStore(st store.Store, env *preinvest.Envelope, rpLaunchID int, item preinvest.FailureItem) *store.Case {
	// Try to find existing case by RP item ID
	suites, _ := st.ListSuites()
	for _, suite := range suites {
		if suite.Status != "open" {
			continue
		}
		pipelines, _ := st.ListPipelinesBySuite(suite.ID)
		for _, p := range pipelines {
			launches, _ := st.ListLaunchesByPipeline(p.ID)
			for _, l := range launches {
				jobs, _ := st.ListJobsByLaunch(l.ID)
				for _, j := range jobs {
					cases, _ := st.ListCasesByJob(j.ID)
					for _, c := range cases {
						if c.RPItemID == item.ID {
							return c
						}
					}
				}
			}
		}
	}

	// Not found — create scaffolding
	suiteID, _ := st.CreateSuite(&store.InvestigationSuite{
		Name:        fmt.Sprintf("Investigation %s", env.Name),
		Description: fmt.Sprintf("Auto-created for launch %s", env.RunID),
	})

	vID, _ := st.CreateVersion(&store.Version{Label: "unknown"})
	if vID == 0 {
		v, _ := st.GetVersionByLabel("unknown")
		if v != nil {
			vID = v.ID
		}
	}

	pID, _ := st.CreatePipeline(&store.Pipeline{
		SuiteID:    suiteID,
		VersionID:  vID,
		Name:       env.Name,
		RPLaunchID: rpLaunchID,
	})

	lID, _ := st.CreateLaunch(&store.Launch{
		PipelineID: pID,
		RPLaunchID: rpLaunchID,
		Name:       env.Name,
	})

	jID, _ := st.CreateJob(&store.Job{
		LaunchID: lID,
		RPItemID: item.ID,
		Name:     item.Name,
	})

	caseID, _ := st.CreateCaseV2(&store.Case{
		JobID:    jID,
		LaunchID: lID,
		RPItemID: item.ID,
		Name:     item.Name,
		Status:   "open",
	})

	caseData, _ := st.GetCaseV2(caseID)
	if caseData == nil {
		fmt.Fprintf(os.Stderr, "failed to create case in store\n")
		os.Exit(1)
	}
	return caseData
}

func loadEnvelopeForCursor(launch string, dbPath string) (*preinvest.Envelope, int) {
	if _, err := os.Stat(launch); err == nil {
		data, err := os.ReadFile(launch)
		if err != nil {
			return nil, 0
		}
		var env preinvest.Envelope
		if err := json.Unmarshal(data, &env); err != nil {
			return nil, 0
		}
		launchID, _ := strconv.Atoi(env.RunID)
		if launchID == 0 {
			launchID = 1
		}
		return &env, launchID
	}
	launchID, err := strconv.Atoi(launch)
	if err != nil || launchID <= 0 {
		return nil, 0
	}
	st, err := store.Open(dbPath)
	if err != nil {
		return nil, 0
	}
	defer st.Close()
	adapter := &store.PreinvestStoreAdapter{Store: st}
	env, _ := adapter.Get(launchID)
	if env == nil {
		return nil, 0
	}
	return env, launchID
}

func runSave(args []string) {
	fs := flag.NewFlagSet("save", flag.ExitOnError)
	artifactPath := fs.String("f", "", "Artifact file path (required)")
	dbPath := fs.String("db", store.DefaultDBPath, "Store DB path")
	caseIDFlag := fs.Int64("case-id", 0, "Case DB ID (for orchestrated flow)")
	suiteIDFlag := fs.Int64("suite-id", 0, "Suite DB ID (for orchestrated flow)")
	_ = fs.Parse(args)

	if *artifactPath == "" {
		fs.PrintDefaults()
		os.Exit(1)
	}

	// If case-id and suite-id are provided, use orchestrated save
	if *caseIDFlag != 0 && *suiteIDFlag != 0 {
		runSaveOrchestrated(*artifactPath, *caseIDFlag, *suiteIDFlag, *dbPath)
		return
	}

	// Legacy save: ingest artifact and create/link RCA (v1 style)
	data, err := os.ReadFile(*artifactPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read artifact: %v\n", err)
		os.Exit(1)
	}
	var a investigate.Artifact
	if err := json.Unmarshal(data, &a); err != nil {
		fmt.Fprintf(os.Stderr, "parse artifact: %v\n", err)
		os.Exit(1)
	}
	launchID, _ := strconv.Atoi(a.LaunchID)
	if launchID == 0 {
		fmt.Fprintf(os.Stderr, "invalid launch_id in artifact\n")
		os.Exit(1)
	}

	st, err := store.Open(*dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "open store: %v\n", err)
		os.Exit(1)
	}
	defer st.Close()

	title := a.RCAMessage
	if len(title) > 80 {
		title = title[:80] + "..."
	}
	if title == "" {
		title = "RCA"
	}
	rcaID, err := st.SaveRCA(&store.RCA{
		Title:       title,
		Description: a.RCAMessage,
		DefectType:  a.DefectType,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "save RCA: %v\n", err)
		os.Exit(1)
	}
	cases, _ := st.ListCasesByLaunch(launchID)
	for _, c := range cases {
		for _, itemID := range a.CaseIDs {
			if c.RPItemID == itemID {
				_ = st.LinkCaseToRCA(c.ID, rcaID)
				break
			}
		}
	}
	if len(cases) == 0 {
		for _, itemID := range a.CaseIDs {
			caseID, err := st.CreateCase(launchID, itemID)
			if err != nil {
				continue
			}
			_ = st.LinkCaseToRCA(caseID, rcaID)
		}
	}
	fmt.Printf("Saved RCA (id=%d); linked to %d case(s)\n", rcaID, len(a.CaseIDs))
}

// runSaveOrchestrated copies the artifact to the per-case directory and advances state.
func runSaveOrchestrated(artifactPath string, caseID, suiteID int64, dbPath string) {
	st, err := store.Open(dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "open store: %v\n", err)
		os.Exit(1)
	}
	defer st.Close()

	caseData, err := st.GetCaseV2(caseID)
	if err != nil || caseData == nil {
		fmt.Fprintf(os.Stderr, "case #%d not found\n", caseID)
		os.Exit(1)
	}

	caseDir := orchestrate.CaseDir(suiteID, caseID)
	state, err := orchestrate.LoadState(caseDir)
	if err != nil || state == nil {
		fmt.Fprintf(os.Stderr, "no state found for case #%d in suite #%d\n", caseID, suiteID)
		os.Exit(1)
	}

	// Copy artifact to case directory with the correct filename
	artifactFilename := orchestrate.ArtifactFilename(state.CurrentStep)
	if artifactFilename == "" {
		fmt.Fprintf(os.Stderr, "no artifact expected for step %s\n", state.CurrentStep)
		os.Exit(1)
	}

	data, err := os.ReadFile(artifactPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read artifact: %v\n", err)
		os.Exit(1)
	}
	destPath := caseDir + "/" + artifactFilename
	if err := os.WriteFile(destPath, data, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "write artifact to case dir: %v\n", err)
		os.Exit(1)
	}

	// Advance state
	cfg := orchestrate.RunnerConfig{
		PromptDir:  ".cursor/prompts",
		Thresholds: orchestrate.DefaultThresholds(),
	}
	result, err := orchestrate.SaveArtifactAndAdvance(st, caseData, caseDir, cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "advance: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Saved artifact for %s\n", state.CurrentStep)
	if result.IsDone {
		fmt.Printf("Pipeline complete! %s\n", result.Explanation)
	} else {
		fmt.Printf("Next step: %s (%s)\n", result.NextStep, result.Explanation)
		fmt.Printf("Run 'asterisk cursor' to generate the next prompt.\n")
	}
}

func runStatus(args []string) {
	fs := flag.NewFlagSet("status", flag.ExitOnError)
	caseIDFlag := fs.Int64("case-id", 0, "Case DB ID (required)")
	suiteIDFlag := fs.Int64("suite-id", 0, "Suite DB ID (required)")
	_ = fs.Parse(args)

	if *caseIDFlag == 0 || *suiteIDFlag == 0 {
		fs.PrintDefaults()
		os.Exit(1)
	}

	caseDir := orchestrate.CaseDir(*suiteIDFlag, *caseIDFlag)
	state, err := orchestrate.LoadState(caseDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load state: %v\n", err)
		os.Exit(1)
	}
	if state == nil {
		fmt.Printf("No investigation state for case #%d in suite #%d\n", *caseIDFlag, *suiteIDFlag)
		fmt.Printf("Run 'asterisk cursor' to start the investigation.\n")
		return
	}

	fmt.Printf("Case:    #%d\n", state.CaseID)
	fmt.Printf("Suite:   #%d\n", state.SuiteID)
	fmt.Printf("Step:    %s\n", state.CurrentStep)
	fmt.Printf("Status:  %s\n", state.Status)
	if len(state.LoopCounts) > 0 {
		fmt.Printf("Loops:\n")
		for name, count := range state.LoopCounts {
			fmt.Printf("  %s: %d\n", name, count)
		}
	}
	if len(state.History) > 0 {
		fmt.Printf("History: (%d steps)\n", len(state.History))
		for _, h := range state.History {
			fmt.Printf("  %s -> %s [%s] %s\n", h.Step, h.Outcome, h.HeuristicID, h.Timestamp)
		}
	}
}

func runCalibrate(args []string) {
	fs := flag.NewFlagSet("calibrate", flag.ExitOnError)
	scenarioName := fs.String("scenario", "ptp-mock", "Scenario name (ptp-mock, daemon-mock, ptp-real, ptp-real-ingest)")
	adapterName := fs.String("adapter", "stub", "Model adapter (stub, cursor)")
	dispatchMode := fs.String("dispatch", "stdin", "Dispatch mode for cursor adapter (stdin, file, batch-file)")
	agentDebug := fs.Bool("agent-debug", false, "Enable verbose debug logging for dispatcher/agent communication")
	runs := fs.Int("runs", 1, "Number of calibration runs")
	promptDir := fs.String("prompt-dir", ".cursor/prompts", "Prompt template directory")
	clean := fs.Bool("clean", true, "Remove .asterisk/calibrate/ before starting (cursor adapter only)")
	responderMode := fs.String("responder", "auto", "Responder lifecycle: auto (spawn/kill), external (user manages), none")
	costReport := fs.Bool("cost-report", false, "Write token-report.json with per-case token/cost breakdown")
	parallel := fs.Int("parallel", 1, "Number of parallel workers for triage/investigation (1 = serial)")
	tokenBudget := fs.Int("token-budget", 0, "Max concurrent dispatches (0 = same as --parallel)")
	batchSize := fs.Int("batch-size", 4, "Max signals per batch for batch-file dispatch mode")
	_ = fs.Parse(args)

	// Load scenario
	var scenario *calibrate.Scenario
	switch *scenarioName {
	case "ptp-mock":
		scenario = scenarios.PTPMockScenario()
	case "daemon-mock":
		scenario = scenarios.DaemonMockScenario()
	case "ptp-real":
		scenario = scenarios.PTPRealScenario()
	case "ptp-real-ingest":
		scenario = scenarios.PTPRealIngestScenario()
	default:
		fmt.Fprintf(os.Stderr, "unknown scenario: %s (available: ptp-mock, daemon-mock, ptp-real, ptp-real-ingest)\n", *scenarioName)
		os.Exit(1)
	}

	// Token tracker — always enabled; near-zero overhead, enables real M18 data.
	tokenTracker := calibrate.NewTokenTracker()

	// Build debug logger for agent communication
	var debugLogger *slog.Logger
	if *agentDebug {
		debugLogger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
		debugLogger.Info("agent-debug enabled: dispatcher and adapter operations will be traced to stderr")
	}

	// Create adapter
	var adapter calibrate.ModelAdapter
	switch *adapterName {
	case "stub":
		adapter = calibrate.NewStubAdapter(scenario)
	case "cursor":
		// Build dispatcher based on --dispatch flag
		var dispatcher calibrate.Dispatcher
		switch *dispatchMode {
		case "stdin":
			dispatcher = calibrate.NewStdinDispatcher()
		case "file":
			cfg := calibrate.DefaultFileDispatcherConfig()
			cfg.Logger = debugLogger
			dispatcher = calibrate.NewFileDispatcher(cfg)
		case "batch-file":
			cfg := calibrate.BatchFileDispatcherConfig{
				FileConfig: calibrate.FileDispatcherConfig{
					Logger: debugLogger,
				},
				SuiteDir:  filepath.Join(".asterisk", "calibrate", "batch"),
				BatchSize: *batchSize,
				Logger:    debugLogger,
			}
			dispatcher = calibrate.NewBatchFileDispatcher(cfg)
		default:
			fmt.Fprintf(os.Stderr, "unknown dispatch mode: %s (available: stdin, file, batch-file)\n", *dispatchMode)
			os.Exit(1)
		}
		// Wrap the dispatcher with token tracking
		trackedDispatcher := calibrate.NewTokenTrackingDispatcher(dispatcher, tokenTracker)
		adapter = calibrate.NewCursorAdapter(*promptDir, calibrate.WithDispatcher(trackedDispatcher))
	default:
		fmt.Fprintf(os.Stderr, "unknown adapter: %s (available: stub, cursor)\n", *adapterName)
		os.Exit(1)
	}

	// Set up the investigation artifacts directory.
	// For stub: temp dir (auto-cleaned). For cursor: persistent dir (user needs access).
	calibDir := ".asterisk/calibrate"
	if *adapterName == "cursor" {
		// Pre-run cleanup: remove stale artifacts and DB from previous runs.
		if *clean {
			if info, err := os.Stat(calibDir); err == nil && info.IsDir() {
				fmt.Printf("[cleanup] removing previous calibration artifacts: %s/\n", calibDir)
				if err := os.RemoveAll(calibDir); err != nil {
					fmt.Fprintf(os.Stderr, "clean calibrate dir: %v\n", err)
					os.Exit(1)
				}
			}
			// Also remove the DB to prevent symptom accumulation across runs
			dbPath := store.DefaultDBPath
			if _, err := os.Stat(dbPath); err == nil {
				fmt.Printf("[cleanup] removing previous DB: %s\n", dbPath)
				_ = os.Remove(dbPath)
				_ = os.Remove(dbPath + "-journal")
			}
		}
		if err := os.MkdirAll(calibDir, 0755); err != nil {
			fmt.Fprintf(os.Stderr, "create calibrate dir: %v\n", err)
			os.Exit(1)
		}
		orchestrate.BasePath = calibDir
		fmt.Printf("Calibration artifacts: %s/\n", calibDir)
		fmt.Printf("Adapter: cursor (dispatch=%s, responder=%s, clean=%v)\n", *dispatchMode, *responderMode, *clean)
		fmt.Printf("Scenario: %s (%d cases)\n\n", scenario.Name, len(scenario.Cases))
	} else {
		tmpDir, err := os.MkdirTemp("", "asterisk-calibrate-*")
		if err != nil {
			fmt.Fprintf(os.Stderr, "create temp dir: %v\n", err)
			os.Exit(1)
		}
		defer os.RemoveAll(tmpDir)
		orchestrate.BasePath = tmpDir
	}

	// Cursor mode is interactive — only 1 run makes sense
	if *adapterName == "cursor" && *runs > 1 {
		fmt.Fprintf(os.Stderr, "cursor adapter only supports --runs=1 (interactive mode)\n")
		os.Exit(1)
	}

	// Spawn mock-calibration-agent subprocess when --responder=auto and --dispatch=file.
	if *adapterName == "cursor" && *dispatchMode == "file" && *responderMode == "auto" {
		responderProc, err := calibrate.SpawnResponder(calibDir, *agentDebug)
		if err != nil {
			fmt.Fprintf(os.Stderr, "spawn mock-calibration-agent: %v\n", err)
			os.Exit(1)
		}
		defer calibrate.StopResponder(responderProc)

		// Forward SIGINT/SIGTERM to ensure the mock agent is killed on interrupt.
		calibrate.ForwardSignals(responderProc)
	} else if *adapterName == "cursor" && *dispatchMode == "file" && *responderMode == "external" {
		fmt.Println("[lifecycle] responder=external: ensure mock-calibration-agent is running separately")
	}

	parallelN := *parallel
	if parallelN < 1 {
		parallelN = 1
	}
	budgetN := *tokenBudget
	if budgetN <= 0 {
		budgetN = parallelN
	}

	cfg := calibrate.RunConfig{
		Scenario:     scenario,
		Adapter:      adapter,
		Runs:         *runs,
		PromptDir:    *promptDir,
		Thresholds:   orchestrate.DefaultThresholds(),
		TokenTracker: tokenTracker,
		Parallel:     parallelN,
		TokenBudget:  budgetN,
	}

	report, err := calibrate.RunCalibration(cfg)

	// Post-run: finalize all signal.json files regardless of success/failure.
	if *adapterName == "cursor" && *dispatchMode == "file" {
		calibrate.FinalizeSignals(calibDir)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "calibration failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Print(calibrate.FormatReport(report))

	// TokiMeter — markdown cost bill (always printed when tokens are tracked)
	bill := calibrate.BuildTokiMeterBill(report)
	if bill != nil {
		md := calibrate.FormatTokiMeter(bill)
		fmt.Print(md)

		// Write tokimeter.md alongside JSON report
		tokiPath := calibDir + "/tokimeter.md"
		if err := os.WriteFile(tokiPath, []byte(md), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "write tokimeter bill: %v\n", err)
		} else {
			fmt.Printf("\nTokiMeter bill: %s\n", tokiPath)
		}
	}

	// Write token-report.json when --cost-report is set
	if *costReport && report.Tokens != nil {
		tokenReportPath := calibDir + "/token-report.json"
		data, err := json.MarshalIndent(report.Tokens, "", "  ")
		if err == nil {
			if err := os.WriteFile(tokenReportPath, data, 0644); err != nil {
				fmt.Fprintf(os.Stderr, "write token report: %v\n", err)
			} else {
				fmt.Printf("\nToken report: %s\n", tokenReportPath)
			}
		}
	}

	passed, total := report.Metrics.PassCount()
	if passed < total {
		os.Exit(1)
	}
}
