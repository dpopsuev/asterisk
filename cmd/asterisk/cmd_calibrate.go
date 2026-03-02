package main

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	framework "github.com/dpopsuev/origami"
	cal "github.com/dpopsuev/origami/calibrate"
	"github.com/dpopsuev/origami/dispatch"

	"asterisk/adapters/rca"
	"asterisk/adapters/calibration/scenarios"
	astrp "asterisk/adapters/rp"
	"asterisk/adapters/store"
	"github.com/dpopsuev/origami/adapters/rp"
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
	rpProject     string
	routingLog    string
	scorecard     string
}

var calibrateCmd = &cobra.Command{
	Use:   "calibrate",
	Short: "Run calibration against a scenario to measure circuit accuracy",
	Long: `Calibrate runs the full F0-F6 circuit against a predefined scenario
with ground-truth expectations, computing accuracy metrics (M1-M20).`,
	RunE: runCalibrate,
}

func init() {
	f := calibrateCmd.Flags()
	f.StringVar(&calibrateFlags.scenario, "scenario", "ptp-mock", "Scenario name (ptp-mock, daemon-mock, ptp-real, ptp-real-ingest)")
	f.StringVar(&calibrateFlags.adapter, "adapter", "stub", "Model adapter (stub, basic, llm)")
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
	f.StringVar(&calibrateFlags.rpProject, "rp-project", "", "RP project name (default: $ASTERISK_RP_PROJECT)")
	f.StringVar(&calibrateFlags.routingLog, "routing-log", "", "Write adapter routing log to path (JSON); empty = disabled")
	f.StringVar(&calibrateFlags.scorecard, "scorecard", "scorecards/asterisk-rca.yaml", "Path to scorecard YAML for metric definitions")
}

func runCalibrate(cmd *cobra.Command, _ []string) error {
	scenario, err := scenarios.LoadScenario(calibrateFlags.scenario)
	if err != nil {
		return err
	}

	// Resolve RP-sourced cases before adapter creation so adapters see real data.
	var rpFetcher rp.EnvelopeFetcher
	if calibrateFlags.rpBase != "" {
		rpProject := resolveRPProject(calibrateFlags.rpProject)
		if rpProject == "" {
			return fmt.Errorf("RP project name is required when using RP API\n\nSet it via environment variable:\n  export ASTERISK_RP_PROJECT=your-project-name\n\nOr use the --rp-project flag:\n  asterisk calibrate --rp-base-url ... --rp-project your-project-name")
		}
		key, err := rp.ReadAPIKey(calibrateFlags.rpKeyPath)
		if err != nil {
			return fmt.Errorf("read RP API key from %s: %w", calibrateFlags.rpKeyPath, err)
		}
		client, err := rp.New(calibrateFlags.rpBase, key, rp.WithTimeout(30*time.Second))
		if err != nil {
			return fmt.Errorf("create RP client: %w", err)
		}
		rpFetcher = rp.NewFetcher(client, rpProject)
		if err := astrp.ResolveRPCases(rpFetcher, scenario); err != nil {
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

	var adapters []*framework.Adapter
	var transformerLabel string
	var idMapper rca.IDMappable
	var routingRecorder *rca.RoutingRecorder
	switch calibrateFlags.adapter {
	case "stub":
		stub := rca.NewStubTransformer(scenario)
		var t framework.Transformer = stub
		if calibrateFlags.routingLog != "" {
			routingRecorder = rca.NewRoutingRecorder(t, adapterColor(calibrateFlags.adapter))
			t = routingRecorder
		}
		adapters = []*framework.Adapter{rca.TransformerAdapter(t)}
		transformerLabel = "stub"
		idMapper = stub
	case "basic":
		basicSt, err := store.Open(":memory:")
		if err != nil {
			return fmt.Errorf("basic transformer: open store: %w", err)
		}
		var repoNames []string
		for _, r := range scenario.Workspace.Repos {
			repoNames = append(repoNames, r.Name)
		}
		adapters = []*framework.Adapter{rca.HeuristicAdapter(basicSt, repoNames)}
		transformerLabel = "basic"
	case "llm":
		dispatcher, err := buildDispatcher(DispatchOpts{
			Mode:      calibrateFlags.dispatchMode,
			Logger:    debugLogger,
			SuiteDir:  filepath.Join(".asterisk", "calibrate", "batch"),
			BatchSize: calibrateFlags.batchSize,
		})
		if err != nil {
			return err
		}
		trackedDispatcher := dispatch.NewTokenTrackingDispatcher(dispatcher, tokenTracker)
		var t framework.Transformer = rca.NewRCATransformer(trackedDispatcher, calibrateFlags.promptDir, rca.WithRCABasePath(calibDir))
		if calibrateFlags.routingLog != "" {
			routingRecorder = rca.NewRoutingRecorder(t, adapterColor(calibrateFlags.adapter))
			t = routingRecorder
		}
		adapters = []*framework.Adapter{rca.TransformerAdapter(t)}
		transformerLabel = "llm"
	default:
		return fmt.Errorf("unknown adapter: %s (available: stub, basic, llm)", calibrateFlags.adapter)
	}

	var basePath string
	if calibrateFlags.adapter == "llm" {
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
		fmt.Fprintf(out, "Adapter: llm (dispatch=%s, clean=%v)\n", calibrateFlags.dispatchMode, calibrateFlags.clean)
		fmt.Fprintf(out, "Scenario: %s (%d cases)\n\n", scenario.Name, len(scenario.Cases))
	} else {
		tmpDir, err := os.MkdirTemp("", "asterisk-calibrate-*")
		if err != nil {
			return fmt.Errorf("create temp dir: %w", err)
		}
		defer os.RemoveAll(tmpDir)
		basePath = tmpDir
	}

	if calibrateFlags.adapter == "llm" && calibrateFlags.runs > 1 {
		return fmt.Errorf("llm adapter only supports --runs=1 (interactive mode)")
	}

	if calibrateFlags.adapter == "llm" && calibrateFlags.dispatchMode == "file" {
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

	sc, err := cal.LoadScoreCard(calibrateFlags.scorecard)
	if err != nil {
		return fmt.Errorf("load scorecard: %w", err)
	}

	cfg := rca.RunConfig{
		Scenario:     scenario,
		Adapters:     adapters,
		TransformerName: transformerLabel,
		IDMapper:     idMapper,
		Runs:         calibrateFlags.runs,
		PromptDir:    calibrateFlags.promptDir,
		Thresholds:   rca.DefaultThresholds(),
		TokenTracker: tokenTracker,
		Parallel:     parallelN,
		TokenBudget:  budgetN,
		BasePath:     basePath,
		RPFetcher:    rpFetcher,
		ScoreCard:    sc,
	}

	report, err := rca.RunCalibration(cmd.Context(), cfg)

	if calibrateFlags.adapter == "llm" && calibrateFlags.dispatchMode == "file" {
		dispatch.FinalizeSignals(calibDir)
	}

	if err != nil {
		return fmt.Errorf("calibration failed: %w", err)
	}

	out := cmd.OutOrStdout()
	rendered, err := rca.RenderCalibrationReport(report)
	if err != nil {
		return fmt.Errorf("render calibration report: %w", err)
	}
	fmt.Fprint(out, rendered)

	bill := rca.BuildCostBill(report)
	if bill != nil {
		md := dispatch.FormatCostBill(bill)
		fmt.Fprint(out, md)

		tokiPath := calibDir + "/tokimeter.md"
		if err := os.WriteFile(tokiPath, []byte(md), 0600); err != nil {
			fmt.Fprintf(os.Stderr, "write tokimeter bill: %v\n", err)
		} else {
			fmt.Fprintf(out, "\nTokiMeter bill: %s\n", tokiPath)
		}
	}

	if calibrateFlags.costReport && report.Tokens != nil {
		tokenReportPath := calibDir + "/token-report.json"
		data, err := json.MarshalIndent(report.Tokens, "", "  ")
		if err == nil {
			if err := os.WriteFile(tokenReportPath, data, 0600); err != nil {
				fmt.Fprintf(os.Stderr, "write token report: %v\n", err)
			} else {
				fmt.Fprintf(out, "\nToken report: %s\n", tokenReportPath)
			}
		}
	}

	if calibrateFlags.transcript {
		transcripts, err := rca.WeaveTranscripts(report)
		if err != nil {
			fmt.Fprintf(os.Stderr, "weave transcripts: %v\n", err)
		} else if len(transcripts) > 0 {
			transcriptDir := filepath.Join(basePath, "transcripts")
			if err := os.MkdirAll(transcriptDir, 0755); err != nil {
				fmt.Fprintf(os.Stderr, "create transcript dir: %v\n", err)
			} else {
				for i := range transcripts {
					slug := rca.TranscriptSlug(&transcripts[i])
					md, renderErr := rca.RenderTranscript(&transcripts[i])
					if renderErr != nil {
						fmt.Fprintf(os.Stderr, "render transcript %s: %v\n", slug, renderErr)
						continue
					}
					tPath := filepath.Join(transcriptDir, slug+".md")
					if err := os.WriteFile(tPath, []byte(md), 0600); err != nil {
						fmt.Fprintf(os.Stderr, "write transcript %s: %v\n", slug, err)
					}
				}
				fmt.Fprintf(out, "\n[transcript] wrote %d RCA transcript(s) to %s/\n", len(transcripts), transcriptDir)
			}
		} else {
			fmt.Fprintln(out, "\n[transcript] no transcripts produced (no case results)")
		}
	}

	if routingRecorder != nil {
		if err := rca.SaveRoutingLog(calibrateFlags.routingLog, routingRecorder.Log()); err != nil {
			fmt.Fprintf(os.Stderr, "save routing log: %v\n", err)
		} else {
			fmt.Fprintf(out, "\nRouting log: %s (%d entries)\n", calibrateFlags.routingLog, routingRecorder.Log().Len())
		}
	}

	passed, total := report.Metrics.PassCount()
	if passed < total {
		return fmt.Errorf("calibration: %d/%d metrics passed", passed, total)
	}
	return nil
}

// adapterColor maps adapter name to color identity for routing log tagging.
func adapterColor(name string) string {
	switch name {
	case "basic":
		return "crimson"
	case "llm":
		return "cerulean"
	default:
		return name
	}
}
