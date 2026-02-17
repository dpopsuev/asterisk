package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"asterisk/internal/calibrate"
	"asterisk/internal/calibrate/adapt"
	"asterisk/internal/orchestrate"
	"asterisk/internal/store"
	"asterisk/internal/workspace"
)

var analyzeFlags struct {
	launch        string
	workspacePath string
	artifactPath  string
	dbPath        string
	adapterName   string
	rpBase        string
	rpKeyPath     string
}

var analyzeCmd = &cobra.Command{
	Use:   "analyze",
	Short: "Run evidence-based RCA on a ReportPortal launch",
	Long:  "Analyze failures from a ReportPortal launch envelope and produce\nan RCA artifact with defect classifications and confidence scores.",
	RunE:  runAnalyze,
}

func init() {
	f := analyzeCmd.Flags()
	f.StringVar(&analyzeFlags.launch, "launch", "", "Path to envelope JSON or launch ID (required)")
	f.StringVar(&analyzeFlags.workspacePath, "workspace", "", "Path to context workspace file (YAML/JSON)")
	f.StringVarP(&analyzeFlags.artifactPath, "output", "o", "", "Output artifact path (required)")
	f.StringVar(&analyzeFlags.dbPath, "db", store.DefaultDBPath, "Store DB path")
	f.StringVar(&analyzeFlags.adapterName, "adapter", "basic", "Adapter: basic (heuristic, default)")
	f.StringVar(&analyzeFlags.rpBase, "rp-base-url", "", "RP base URL (optional; for fetch by launch ID)")
	f.StringVar(&analyzeFlags.rpKeyPath, "rp-api-key", ".rp-api-key", "Path to RP API key file")

	_ = analyzeCmd.MarkFlagRequired("launch")
	_ = analyzeCmd.MarkFlagRequired("output")
}

func runAnalyze(cmd *cobra.Command, _ []string) error {
	env := loadEnvelopeForAnalyze(analyzeFlags.launch, analyzeFlags.dbPath, analyzeFlags.rpBase, analyzeFlags.rpKeyPath)
	if env == nil {
		return fmt.Errorf("could not load envelope for launch %q", analyzeFlags.launch)
	}
	if len(env.FailureList) == 0 {
		return fmt.Errorf("envelope has no failures")
	}

	st, err := store.Open(analyzeFlags.dbPath)
	if err != nil {
		return fmt.Errorf("open store: %w", err)
	}
	defer st.Close()

	var repoNames []string
	if analyzeFlags.workspacePath != "" {
		ws, err := workspace.LoadFromPath(analyzeFlags.workspacePath)
		if err != nil {
			return fmt.Errorf("load workspace: %w", err)
		}
		for _, r := range ws.Repos {
			repoNames = append(repoNames, r.Name)
		}
	}

	suiteID, cases := createAnalysisScaffolding(st, env)

	var adapter calibrate.ModelAdapter
	switch analyzeFlags.adapterName {
	case "basic":
		ba := adapt.NewBasicAdapter(st, repoNames)
		for i, c := range cases {
			label := fmt.Sprintf("A%d", i+1)
			ba.RegisterCase(label, &adapt.BasicCaseInfo{
				Name:         c.Name,
				ErrorMessage: c.ErrorMessage,
				LogSnippet:   c.LogSnippet,
				StoreCaseID:  c.ID,
			})
		}
		adapter = ba
	default:
		return fmt.Errorf("unknown adapter: %s (supported: basic)", analyzeFlags.adapterName)
	}

	cfg := calibrate.AnalysisConfig{
		Adapter:    adapter,
		Thresholds: orchestrate.DefaultThresholds(),
	}
	report, err := calibrate.RunAnalysis(st, cases, suiteID, cfg)
	if err != nil {
		return fmt.Errorf("analyze: %w", err)
	}
	report.LaunchName = env.Name

	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal report: %w", err)
	}
	if err := os.WriteFile(analyzeFlags.artifactPath, data, 0644); err != nil {
		return fmt.Errorf("write report: %w", err)
	}

	fmt.Fprint(cmd.OutOrStdout(), calibrate.FormatAnalysisReport(report))
	fmt.Fprintf(cmd.OutOrStdout(), "\nReport written to: %s\n", analyzeFlags.artifactPath)
	return nil
}
