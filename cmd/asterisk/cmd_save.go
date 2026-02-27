package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/dpopsuev/origami/adapters/rp"
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
	Short: "Save an artifact to the store and advance pipeline state",
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
	if saveFlags.caseID != 0 && saveFlags.suiteID != 0 {
		return runSaveOrchestrated(cmd)
	}
	return runSaveLegacy(cmd)
}

func runSaveLegacy(cmd *cobra.Command) error {
	data, err := os.ReadFile(saveFlags.artifactPath)
	if err != nil {
		return fmt.Errorf("read artifact: %w", err)
	}
	var a rp.Artifact
	if err := json.Unmarshal(data, &a); err != nil {
		return fmt.Errorf("parse artifact: %w", err)
	}
	launchID, _ := strconv.Atoi(a.LaunchID)
	if launchID == 0 {
		return fmt.Errorf("invalid launch_id in artifact")
	}

	st, err := store.Open(saveFlags.dbPath)
	if err != nil {
		return fmt.Errorf("open store: %w", err)
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
		return fmt.Errorf("save RCA: %w", err)
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
	fmt.Fprintf(cmd.OutOrStdout(), "Saved RCA (id=%d); linked to %d case(s)\n", rcaID, len(a.CaseIDs))
	return nil
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
	state, err := rca.LoadState(caseDir)
	if err != nil || state == nil {
		return fmt.Errorf("no state found for case #%d in suite #%d", saveFlags.caseID, saveFlags.suiteID)
	}

	artifactFilename := rca.ArtifactFilename(state.CurrentStep)
	if artifactFilename == "" {
		return fmt.Errorf("no artifact expected for step %s", vocabNameWithCode(string(state.CurrentStep)))
	}

	data, err := os.ReadFile(saveFlags.artifactPath)
	if err != nil {
		return fmt.Errorf("read artifact: %w", err)
	}
	destPath := caseDir + "/" + artifactFilename
	if err := os.WriteFile(destPath, data, 0600); err != nil {
		return fmt.Errorf("write artifact to case dir: %w", err)
	}

	cfg := rca.RunnerConfig{
		PromptDir:  ".cursor/prompts",
		Thresholds: rca.DefaultThresholds(),
	}
	result, err := rca.SaveArtifactAndAdvance(st, caseData, caseDir, cfg)
	if err != nil {
		return fmt.Errorf("advance: %w", err)
	}

	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "Saved artifact for %s\n", vocabNameWithCode(string(state.CurrentStep)))
	if result.IsDone {
		fmt.Fprintf(out, "Pipeline complete! %s\n", result.Explanation)
	} else {
		fmt.Fprintf(out, "Next step: %s (%s)\n", vocabNameWithCode(string(result.NextStep)), result.Explanation)
		fmt.Fprintf(out, "Run 'asterisk cursor' to generate the next prompt.\n")
	}
	return nil
}
