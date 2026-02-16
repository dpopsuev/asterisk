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
	"strconv"

	"asterisk/internal/investigate"
	"asterisk/internal/orchestrate"
	"asterisk/internal/postinvest"
	"asterisk/internal/preinvest"
	"asterisk/internal/rpfetch"
	"asterisk/internal/rppush"
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
	default:
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Fprintf(os.Stderr, "usage: asterisk <analyze|push|cursor|save|status> [options]\n")
	fmt.Fprintf(os.Stderr, "  asterisk analyze  --launch=<path|id> [--workspace=<path>] -o <artifact>\n")
	fmt.Fprintf(os.Stderr, "  asterisk push     -f <artifact-path>\n")
	fmt.Fprintf(os.Stderr, "  asterisk cursor   --launch=<path|id> [--workspace=<path>] [--case-id=<id>]\n")
	fmt.Fprintf(os.Stderr, "  asterisk save     -f <artifact-path> --case-id=<id> --suite-id=<id>\n")
	fmt.Fprintf(os.Stderr, "  asterisk status   --case-id=<id> --suite-id=<id>\n")
}

func runAnalyze(args []string) {
	fs := flag.NewFlagSet("analyze", flag.ExitOnError)
	launch := fs.String("launch", "", "Path to envelope JSON or launch ID (required)")
	workspacePath := fs.String("workspace", "", "Path to context workspace file (YAML/JSON)")
	artifactPath := fs.String("o", "", "Output artifact path (required)")
	dbPath := fs.String("db", store.DefaultDBPath, "Store DB path")
	rpBase := fs.String("rp-base-url", "", "RP base URL (optional; for fetch by launch ID)")
	rpKeyPath := fs.String("rp-api-key", ".rp-api-key", "Path to RP API key file")
	_ = fs.Parse(args)

	if *launch == "" || *artifactPath == "" {
		fs.PrintDefaults()
		os.Exit(1)
	}

	// Resolve envelope source and launch ID
	var envSrc investigate.EnvelopeSource
	var launchID int
	if _, err := os.Stat(*launch); err == nil {
		// Treat as file path
		data, err := os.ReadFile(*launch)
		if err != nil {
			fmt.Fprintf(os.Stderr, "read envelope: %v\n", err)
			os.Exit(1)
		}
		var env preinvest.Envelope
		if err := json.Unmarshal(data, &env); err != nil {
			fmt.Fprintf(os.Stderr, "parse envelope: %v\n", err)
			os.Exit(1)
		}
		launchID, _ = strconv.Atoi(env.RunID)
		if launchID == 0 {
			launchID = 1
		}
		envSrc = &singleEnvelopeSource{env: &env, launchID: launchID}
	} else {
		// Treat as launch ID; need store and optionally fetch from RP
		var err error
		launchID, err = strconv.Atoi(*launch)
		if err != nil || launchID <= 0 {
			fmt.Fprintf(os.Stderr, "launch must be path to envelope JSON or positive launch ID\n")
			os.Exit(1)
		}
		st, err := store.Open(*dbPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "open store: %v\n", err)
			os.Exit(1)
		}
		defer st.Close()
		adapter := &store.PreinvestStoreAdapter{Store: st}
		env, _ := adapter.Get(launchID)
		if env == nil && *rpBase != "" {
			key, _ := rpfetch.ReadAPIKey(*rpKeyPath)
			client := rpfetch.NewClient(rpfetch.Config{BaseURL: *rpBase, APIKey: key, Project: "ecosystem-qe"})
			fetcher := rpfetch.NewFetcher(client)
			if err := preinvest.FetchAndSave(fetcher, adapter, launchID); err != nil {
				fmt.Fprintf(os.Stderr, "fetch: %v\n", err)
				os.Exit(1)
			}
			env, _ = adapter.Get(launchID)
		}
		if env == nil {
			fmt.Fprintf(os.Stderr, "no envelope for launch %d (fetch from RP with --rp-base-url or provide envelope file)\n", launchID)
			os.Exit(1)
		}
		envSrc = adapter
	}

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

	if err := investigate.AnalyzeWithWorkspace(envSrc, launchID, *artifactPath, ws); err != nil {
		fmt.Fprintf(os.Stderr, "analyze: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Artifact: %s\n", *artifactPath)
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
		key, err := rpfetch.ReadAPIKey(*rpKeyPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "read API key: %v\n", err)
			os.Exit(1)
		}
		client := rppush.NewClient(rppush.Config{BaseURL: *rpBase, APIKey: key, Project: "ecosystem-qe"})
		pusher = rppush.NewRPPusher(client)
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

// singleEnvelopeSource returns a fixed envelope for one launch ID (for file-based envelope).
// Implements investigate.EnvelopeSource.
type singleEnvelopeSource struct {
	env      *preinvest.Envelope
	launchID int
}

func (s *singleEnvelopeSource) Get(launchID int) (*preinvest.Envelope, error) {
	if launchID == s.launchID {
		return s.env, nil
	}
	return nil, nil
}
