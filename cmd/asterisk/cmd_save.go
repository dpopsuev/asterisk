package main

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"asterisk/adapters/rca"
	"asterisk/adapters/store"
)

var saveFlags struct {
	artifactPath string
	dbPath       string
	caseID       int64
	suiteID      int64
}

var saveCmd = &cobra.Command{
	Use:   "save",
	Short: "Save an artifact to the store and advance circuit state",
	RunE:  runSave,
}

func init() {
	f := saveCmd.Flags()
	f.StringVarP(&saveFlags.artifactPath, "file", "f", "", "Artifact file path (required)")
	f.StringVar(&saveFlags.dbPath, "db", store.DefaultDBPath, "Store DB path")
	f.Int64Var(&saveFlags.caseID, "case-id", 0, "Case DB ID (for orchestrated flow)")
	f.Int64Var(&saveFlags.suiteID, "suite-id", 0, "Suite DB ID (for orchestrated flow)")

	_ = saveCmd.MarkFlagRequired("file")
}

func runSave(cmd *cobra.Command, _ []string) error {
	if saveFlags.caseID == 0 || saveFlags.suiteID == 0 {
		return fmt.Errorf("--case-id and --suite-id are required")
	}
	return runSaveOrchestrated(cmd)
}

func runSaveOrchestrated(cmd *cobra.Command) error {
	st, err := store.Open(saveFlags.dbPath)
	if err != nil {
		return fmt.Errorf("open store: %w", err)
	}
	defer st.Close()

	caseData, err := st.GetCaseV2(saveFlags.caseID)
	if err != nil || caseData == nil {
		return fmt.Errorf("case #%d not found", saveFlags.caseID)
	}

	caseDir := rca.CaseDir(rca.DefaultBasePath, saveFlags.suiteID, saveFlags.caseID)

	artifactData, err := os.ReadFile(saveFlags.artifactPath)
	if err != nil {
		return fmt.Errorf("read artifact: %w", err)
	}

	result, err := rca.ResumeHITLStep(context.Background(), rca.HITLConfig{
		Store:    st,
		CaseData: caseData,
		CaseDir:  caseDir,
	}, artifactData)
	if err != nil {
		return fmt.Errorf("advance: %w", err)
	}

	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "Saved artifact and advanced circuit\n")
	if result.IsDone {
		fmt.Fprintf(out, "Circuit complete! %s\n", result.Explanation)
	} else {
		fmt.Fprintf(out, "Next step: %s (%s)\n", vocabNameWithCode(result.CurrentStep), result.Explanation)
		fmt.Fprintf(out, "Run 'asterisk cursor' to generate the next prompt.\n")
	}
	return nil
}
