package main

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"asterisk/internal/calibrate"
	"asterisk/internal/calibrate/adapt"
	"asterisk/internal/calibrate/dispatch"
	"asterisk/internal/calibrate/scenarios"
	"asterisk/internal/orchestrate"
	"asterisk/internal/preinvest"
	"asterisk/internal/rp"
	"asterisk/internal/store"
)

var calibrateFlags struct {
	scenario      string
	adapter       string
	dispatchMode  string
	agentDebug    bool
	runs          int
	promptDir     string
	clean         bool
	costReport    bool
	parallel      int
	tokenBudget   int
	batchSize     int
	transcript    bool
	rpBase        string
	rpKeyPath     string
}

var calibrateCmd = &cobra.Command{
	Use:   "calibrate",
	Short: "Run calibration against a scenario to measure pipeline accuracy",
	Long: `Calibrate runs the full F0-F6 pipeline against a predefined scenario
with ground-truth expectations, computing accuracy metrics (M1-M20).`,
	RunE: runCalibrate,
}

func init() {
	f := calibrateCmd.Flags()
	f.StringVar(&calibrateFlags.scenario, "scenario", "ptp-mock", "Scenario name (ptp-mock, daemon-mock, ptp-real, ptp-real-ingest)")
	f.StringVar(&calibrateFlags.adapter, "adapter", "stub", "Model adapter (stub, basic, cursor)")
	f.StringVar(&calibrateFlags.dispatchMode, "dispatch", "stdin", "Dispatch mode for cursor adapter (stdin, file, batch-file)")
	f.BoolVar(&calibrateFlags.agentDebug, "agent-debug", false, "Enable verbose debug logging for dispatcher/agent communication")
	f.IntVar(&calibrateFlags.runs, "runs", 1, "Number of calibration runs")
	f.StringVar(&calibrateFlags.promptDir, "prompt-dir", ".cursor/prompts", "Prompt template directory")
	f.BoolVar(&calibrateFlags.clean, "clean", true, "Remove .asterisk/calibrate/ before starting (cursor adapter only)")
	f.BoolVar(&calibrateFlags.costReport, "cost-report", false, "Write token-report.json with per-case token/cost breakdown")
	f.IntVar(&calibrateFlags.parallel, "parallel", 1, "Number of parallel workers for triage/investigation (1 = serial)")
	f.IntVar(&calibrateFlags.tokenBudget, "token-budget", 0, "Max concurrent dispatches (0 = same as --parallel)")
	f.IntVar(&calibrateFlags.batchSize, "batch-size", 4, "Max signals per batch for batch-file dispatch mode")
	f.BoolVar(&calibrateFlags.transcript, "transcript", false, "Write per-RCA transcript files after calibration")
	f.StringVar(&calibrateFlags.rpBase, "rp-base-url", "", "RP base URL for RP-sourced scenario cases")
	f.StringVar(&calibrateFlags.rpKeyPath, "rp-api-key", ".rp-api-key", "Path to RP API key file")
}

func runCalibrate(cmd *cobra.Command, _ []string) error {
	var scenario *calibrate.Scenario
	switch calibrateFlags.scenario {
	case "ptp-mock":
		scenario = scenarios.PTPMockScenario()
	case "daemon-mock":
		scenario = scenarios.DaemonMockScenario()
	case "ptp-real":
		scenario = scenarios.PTPRealScenario()
	case "ptp-real-ingest":
		scenario = scenarios.PTPRealIngestScenario()
	default:
		return fmt.Errorf("unknown scenario: %s (available: ptp-mock, daemon-mock, ptp-real, ptp-real-ingest)", calibrateFlags.scenario)
	}

	// Resolve RP-sourced cases before adapter creation so adapters see real data.
	var rpFetcher preinvest.Fetcher
	if calibrateFlags.rpBase != "" {
		key, err := rp.ReadAPIKey(calibrateFlags.rpKeyPath)
		if err != nil {
			return fmt.Errorf("read RP API key from %s: %w", calibrateFlags.rpKeyPath, err)
		}
		client, err := rp.New(calibrateFlags.rpBase, key)
		if err != nil {
			return fmt.Errorf("create RP client: %w", err)
		}
		rpFetcher = rp.NewFetcher(client, "ecosystem-qe")
		if err := calibrate.ResolveRPCases(rpFetcher, scenario); err != nil {
			return fmt.Errorf("resolve RP-sourced cases: %w", err)
		}
	}

	calibDir := ".asterisk/calibrate"
	tokenTracker := dispatch.NewTokenTracker()

	var debugLogger *slog.Logger
	if calibrateFlags.agentDebug {
		debugLogger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
		debugLogger.Info("agent-debug enabled: dispatcher and adapter operations will be traced to stderr")
	}

	var adapter calibrate.ModelAdapter
	switch calibrateFlags.adapter {
	case "stub":
		adapter = adapt.NewStubAdapter(scenario)
	case "basic":
		basicSt, err := store.Open(":memory:")
		if err != nil {
			return fmt.Errorf("basic adapter: open store: %w", err)
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
	case "cursor":
		var dispatcher dispatch.Dispatcher
		switch calibrateFlags.dispatchMode {
		case "stdin":
			dispatcher = dispatch.NewStdinDispatcher()
		case "file":
			cfg := dispatch.DefaultFileDispatcherConfig()
			cfg.Logger = debugLogger
			dispatcher = dispatch.NewFileDispatcher(cfg)
		case "batch-file":
			cfg := dispatch.BatchFileDispatcherConfig{
				FileConfig: dispatch.FileDispatcherConfig{
					Logger: debugLogger,
				},
				SuiteDir:  filepath.Join(".asterisk", "calibrate", "batch"),
				BatchSize: calibrateFlags.batchSize,
				Logger:    debugLogger,
			}
			dispatcher = dispatch.NewBatchFileDispatcher(cfg)
		default:
			return fmt.Errorf("unknown dispatch mode: %s (available: stdin, file, batch-file)", calibrateFlags.dispatchMode)
		}
		trackedDispatcher := dispatch.NewTokenTrackingDispatcher(dispatcher, tokenTracker)
		adapter = adapt.NewCursorAdapter(calibrateFlags.promptDir, adapt.WithDispatcher(trackedDispatcher), adapt.WithBasePath(calibDir))
	default:
		return fmt.Errorf("unknown adapter: %s (available: stub, basic, cursor)", calibrateFlags.adapter)
	}

	var basePath string
	if calibrateFlags.adapter == "cursor" {
		if calibrateFlags.clean {
			if info, err := os.Stat(calibDir); err == nil && info.IsDir() {
				fmt.Fprintf(cmd.OutOrStdout(), "[cleanup] removing previous calibration artifacts: %s/\n", calibDir)
				if err := os.RemoveAll(calibDir); err != nil {
					return fmt.Errorf("clean calibrate dir: %w", err)
				}
			}
			dbPath := store.DefaultDBPath
			if _, err := os.Stat(dbPath); err == nil {
				fmt.Fprintf(cmd.OutOrStdout(), "[cleanup] removing previous DB: %s\n", dbPath)
				_ = os.Remove(dbPath)
				_ = os.Remove(dbPath + "-journal")
			}
		}
		if err := os.MkdirAll(calibDir, 0755); err != nil {
			return fmt.Errorf("create calibrate dir: %w", err)
		}
		basePath = calibDir
		out := cmd.OutOrStdout()
		fmt.Fprintf(out, "Calibration artifacts: %s/\n", calibDir)
		fmt.Fprintf(out, "Adapter: cursor (dispatch=%s, clean=%v)\n", calibrateFlags.dispatchMode, calibrateFlags.clean)
		fmt.Fprintf(out, "Scenario: %s (%d cases)\n\n", scenario.Name, len(scenario.Cases))
	} else {
		tmpDir, err := os.MkdirTemp("", "asterisk-calibrate-*")
		if err != nil {
			return fmt.Errorf("create temp dir: %w", err)
		}
		defer os.RemoveAll(tmpDir)
		basePath = tmpDir
	}

	if calibrateFlags.adapter == "cursor" && calibrateFlags.runs > 1 {
		return fmt.Errorf("cursor adapter only supports --runs=1 (interactive mode)")
	}

	if calibrateFlags.adapter == "cursor" && calibrateFlags.dispatchMode == "file" {
		fmt.Fprintln(cmd.OutOrStdout(), "[lifecycle] dispatch=file: ensure Cursor agent or MCP server is responding to signals")
	}

	parallelN := calibrateFlags.parallel
	if parallelN < 1 {
		parallelN = 1
	}
	budgetN := calibrateFlags.tokenBudget
	if budgetN <= 0 {
		budgetN = parallelN
	}

	cfg := calibrate.RunConfig{
		Scenario:     scenario,
		Adapter:      adapter,
		Runs:         calibrateFlags.runs,
		PromptDir:    calibrateFlags.promptDir,
		Thresholds:   orchestrate.DefaultThresholds(),
		TokenTracker: tokenTracker,
		Parallel:     parallelN,
		TokenBudget:  budgetN,
		BasePath:     basePath,
		RPFetcher:    rpFetcher,
	}

	report, err := calibrate.RunCalibration(cmd.Context(), cfg)

	if calibrateFlags.adapter == "cursor" && calibrateFlags.dispatchMode == "file" {
		dispatch.FinalizeSignals(calibDir)
	}

	if err != nil {
		return fmt.Errorf("calibration failed: %w", err)
	}

	out := cmd.OutOrStdout()
	fmt.Fprint(out, calibrate.FormatReport(report))

	bill := calibrate.BuildTokiMeterBill(report)
	if bill != nil {
		md := calibrate.FormatTokiMeter(bill)
		fmt.Fprint(out, md)

		tokiPath := calibDir + "/tokimeter.md"
		if err := os.WriteFile(tokiPath, []byte(md), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "write tokimeter bill: %v\n", err)
		} else {
			fmt.Fprintf(out, "\nTokiMeter bill: %s\n", tokiPath)
		}
	}

	if calibrateFlags.costReport && report.Tokens != nil {
		tokenReportPath := calibDir + "/token-report.json"
		data, err := json.MarshalIndent(report.Tokens, "", "  ")
		if err == nil {
			if err := os.WriteFile(tokenReportPath, data, 0644); err != nil {
				fmt.Fprintf(os.Stderr, "write token report: %v\n", err)
			} else {
				fmt.Fprintf(out, "\nToken report: %s\n", tokenReportPath)
			}
		}
	}

	if calibrateFlags.transcript {
		transcripts, err := calibrate.WeaveTranscripts(report)
		if err != nil {
			fmt.Fprintf(os.Stderr, "weave transcripts: %v\n", err)
		} else if len(transcripts) > 0 {
			transcriptDir := filepath.Join(basePath, "transcripts")
			if err := os.MkdirAll(transcriptDir, 0755); err != nil {
				fmt.Fprintf(os.Stderr, "create transcript dir: %v\n", err)
			} else {
				for i := range transcripts {
					slug := calibrate.TranscriptSlug(&transcripts[i])
					md := calibrate.RenderRCATranscript(&transcripts[i])
					tPath := filepath.Join(transcriptDir, slug+".md")
					if err := os.WriteFile(tPath, []byte(md), 0644); err != nil {
						fmt.Fprintf(os.Stderr, "write transcript %s: %v\n", slug, err)
					}
				}
				fmt.Fprintf(out, "\n[transcript] wrote %d RCA transcript(s) to %s/\n", len(transcripts), transcriptDir)
			}
		} else {
			fmt.Fprintln(out, "\n[transcript] no transcripts produced (no case results)")
		}
	}

	passed, total := report.Metrics.PassCount()
	if passed < total {
		return fmt.Errorf("calibration: %d/%d metrics passed", passed, total)
	}
	return nil
}
