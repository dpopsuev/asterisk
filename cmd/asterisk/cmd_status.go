package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"asterisk/adapters/rca"
)

var statusFlags struct {
	caseID  int64
	suiteID int64
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show investigation state for a case",
	RunE:  runStatus,
}

func init() {
	f := statusCmd.Flags()
	f.Int64Var(&statusFlags.caseID, "case-id", 0, "Case DB ID (required)")
	f.Int64Var(&statusFlags.suiteID, "suite-id", 0, "Suite DB ID (required)")

	_ = statusCmd.MarkFlagRequired("case-id")
	_ = statusCmd.MarkFlagRequired("suite-id")
}

func runStatus(cmd *cobra.Command, _ []string) error {
	caseDir := rca.CaseDir(rca.DefaultBasePath, statusFlags.suiteID, statusFlags.caseID)
	state, err := rca.LoadCheckpointState(caseDir, statusFlags.caseID)
	if err != nil {
		return fmt.Errorf("load state: %w", err)
	}
	out := cmd.OutOrStdout()
	if state == nil {
		fmt.Fprintf(out, "No investigation state for case #%d in suite #%d\n", statusFlags.caseID, statusFlags.suiteID)
		fmt.Fprintf(out, "Run 'asterisk cursor' to start the investigation.\n")
		return nil
	}

	fmt.Fprintf(out, "Case:    #%d\n", statusFlags.caseID)
	fmt.Fprintf(out, "Suite:   #%d\n", statusFlags.suiteID)
	fmt.Fprintf(out, "Step:    %s\n", vocabNameWithCode(state.CurrentNode))
	fmt.Fprintf(out, "Status:  %s\n", state.Status)
	if len(state.LoopCounts) > 0 {
		fmt.Fprintf(out, "Loops:\n")
		for name, count := range state.LoopCounts {
			fmt.Fprintf(out, "  %s: %d\n", name, count)
		}
	}
	if len(state.History) > 0 {
		fmt.Fprintf(out, "History: (%d steps)\n", len(state.History))
		for _, h := range state.History {
			fmt.Fprintf(out, "  %s -> %s [%s]\n", vocabName(h.Node), h.Outcome, vocabNameWithCode(h.EdgeID))
		}
	}
	return nil
}
