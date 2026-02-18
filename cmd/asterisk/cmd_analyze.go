package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

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
	rpProject     string
}

var analyzeCmd = &cobra.Command{
	Use:   "analyze [launch-id]",
	Short: "Run evidence-based RCA on a ReportPortal launch",
	Long: `Analyze failures from a ReportPortal launch and produce an RCA artifact
with defect classifications and confidence scores.

Usage:
  asterisk analyze 33195                    # Launch ID as positional arg
  asterisk analyze --launch=33195           # Launch ID as flag
  asterisk analyze path/to/envelope.json    # Local envelope file

The RP base URL is read from the ASTERISK_RP_URL environment variable,
or can be set with --rp-base-url. If neither is set, the tool will
prompt you to configure it.

The RP API token is read from .rp-api-key (first line). If the file
does not exist, the tool will show you how to get and save the token.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runAnalyze,
}

func init() {
	f := analyzeCmd.Flags()
	f.StringVar(&analyzeFlags.launch, "launch", "", "Path to envelope JSON or launch ID")
	f.StringVar(&analyzeFlags.workspacePath, "workspace", "", "Path to context workspace file (YAML/JSON)")
	f.StringVarP(&analyzeFlags.artifactPath, "output", "o", "", "Output artifact path (default: .asterisk/output/rca-<launch>.json)")
	f.StringVar(&analyzeFlags.dbPath, "db", store.DefaultDBPath, "Store DB path")
	f.StringVar(&analyzeFlags.adapterName, "adapter", "basic", "Adapter: basic (heuristic, default)")
	f.StringVar(&analyzeFlags.rpBase, "rp-base-url", "", "RP base URL (default: $ASTERISK_RP_URL)")
	f.StringVar(&analyzeFlags.rpKeyPath, "rp-api-key", ".rp-api-key", "Path to RP API key file")
	f.StringVar(&analyzeFlags.rpProject, "rp-project", "", "RP project name (default: $ASTERISK_RP_PROJECT)")
}

func runAnalyze(cmd *cobra.Command, args []string) error {
	launch := analyzeFlags.launch
	if launch == "" && len(args) > 0 {
		launch = args[0]
	}
	if launch == "" {
		return fmt.Errorf("launch ID or envelope path is required\n\nUsage: asterisk analyze <launch-id>\n       asterisk analyze path/to/envelope.json")
	}

	rpBase := analyzeFlags.rpBase
	if rpBase == "" {
		rpBase = os.Getenv("ASTERISK_RP_URL")
	}

	if _, err := strconv.Atoi(launch); err == nil && rpBase == "" {
		return fmt.Errorf("RP base URL is required when using a launch ID\n\nSet it via environment variable:\n  export ASTERISK_RP_URL=https://your-rp-instance.example.com\n\nOr use the --rp-base-url flag:\n  asterisk analyze %s --rp-base-url https://your-rp-instance.example.com", launch)
	}

	if rpBase != "" {
		if err := checkTokenFile(analyzeFlags.rpKeyPath); err != nil {
			return err
		}
	}

	artifactPath := analyzeFlags.artifactPath
	if artifactPath == "" {
		safeName := launch
		if id, err := strconv.Atoi(launch); err == nil {
			safeName = strconv.Itoa(id)
		} else {
			safeName = filepath.Base(launch)
		}
		outputDir := filepath.Join(".asterisk", "output")
		if err := os.MkdirAll(outputDir, 0700); err != nil {
			return fmt.Errorf("create output dir: %w", err)
		}
		artifactPath = filepath.Join(outputDir, fmt.Sprintf("rca-%s.json", safeName))
	}

	rpProject := resolveRPProject(analyzeFlags.rpProject)
	if rpBase != "" && rpProject == "" {
		return fmt.Errorf("RP project name is required when using RP API\n\nSet it via environment variable:\n  export ASTERISK_RP_PROJECT=your-project-name\n\nOr use the --rp-project flag:\n  asterisk analyze %s --rp-project your-project-name", launch)
	}

	env := loadEnvelopeForAnalyze(launch, analyzeFlags.dbPath, rpBase, analyzeFlags.rpKeyPath, rpProject)
	if env == nil {
		return fmt.Errorf("could not load envelope for launch %q", launch)
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
	} else {
		repoNames = defaultWorkspaceRepos()
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

	for i := range report.CaseResults {
		if i < len(env.FailureList) {
			report.CaseResults[i].RPIssueType = env.FailureList[i].IssueType
			report.CaseResults[i].RPAutoAnalyzed = env.FailureList[i].AutoAnalyzed
		}
	}

	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal report: %w", err)
	}
	if err := os.WriteFile(artifactPath, data, 0600); err != nil {
		return fmt.Errorf("write report: %w", err)
	}

	fmt.Fprint(cmd.OutOrStdout(), calibrate.FormatAnalysisReport(report))
	fmt.Fprintf(cmd.OutOrStdout(), "\nReport written to: %s\n", artifactPath)
	return nil
}
