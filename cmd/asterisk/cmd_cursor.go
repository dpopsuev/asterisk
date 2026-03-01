package main

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"asterisk/adapters/rca"
	"asterisk/adapters/store"
	"github.com/dpopsuev/origami/knowledge"
)

var cursorFlags struct {
	launch        string
	workspacePath string
	itemID        int
	promptDir     string
	dbPath        string
}

var cursorCmd = &cobra.Command{
	Use:   "cursor",
	Short: "Run interactive Cursor-based RCA for a single failure",
	Long: `Generate a prompt for the next circuit step and print it.
Paste the prompt into Cursor, save the artifact, then run again to advance.`,
	RunE: runCursor,
}

func init() {
	f := cursorCmd.Flags()
	f.StringVar(&cursorFlags.launch, "launch", "", "Path to envelope JSON or launch ID (required)")
	f.StringVar(&cursorFlags.workspacePath, "workspace", "", "Path to context workspace file (YAML/JSON)")
	f.IntVar(&cursorFlags.itemID, "case-id", 0, "Failure (test item) RP ID; default first from envelope")
	f.StringVar(&cursorFlags.promptDir, "prompt-dir", ".cursor/prompts", "Directory containing prompt templates")
	f.StringVar(&cursorFlags.dbPath, "db", store.DefaultDBPath, "Store DB path")

	_ = cursorCmd.MarkFlagRequired("launch")
}

func runCursor(cmd *cobra.Command, _ []string) error {
	env, rpLaunchID := loadEnvelopeForCursor(cursorFlags.launch, cursorFlags.dbPath)
	if env == nil {
		return fmt.Errorf("could not load envelope for launch %q", cursorFlags.launch)
	}
	if len(env.FailureList) == 0 {
		return fmt.Errorf("envelope has no failures")
	}

	item := env.FailureList[0]
	for _, f := range env.FailureList {
		if cursorFlags.itemID == 0 || f.ID == cursorFlags.itemID {
			item = f
			break
		}
	}
	if cursorFlags.itemID != 0 && item.ID != cursorFlags.itemID {
		return fmt.Errorf("case-id %d not in envelope", cursorFlags.itemID)
	}

	st, err := store.Open(cursorFlags.dbPath)
	if err != nil {
		return fmt.Errorf("open store: %w", err)
	}
	defer st.Close()

	caseData := ensureCaseInStore(st, env, rpLaunchID, item)

	var catalog *knowledge.KnowledgeSourceCatalog
	if cursorFlags.workspacePath != "" {
		cat, err := knowledge.LoadFromPath(cursorFlags.workspacePath)
		if err != nil {
			return fmt.Errorf("load catalog: %w", err)
		}
		catalog = cat
	}

	suiteID := int64(1)
	caseDir, err := rca.EnsureCaseDir(rca.DefaultBasePath, suiteID, caseData.ID)
	if err != nil {
		return fmt.Errorf("ensure case dir: %w", err)
	}

	result, err := rca.RunHITLStep(context.Background(), rca.HITLConfig{
		Store:     st,
		CaseData:  caseData,
		Envelope:  env,
		Catalog:   catalog,
		PromptDir: cursorFlags.promptDir,
		CaseDir:   caseDir,
	})
	if err != nil {
		return fmt.Errorf("orchestrate: %w", err)
	}

	out := cmd.OutOrStdout()
	if result.IsDone {
		fmt.Fprintf(out, "Circuit complete for case #%d. %s\n", caseData.ID, result.Explanation)
		return nil
	}

	fmt.Fprintf(out, "Step: %s\n", vocabNameWithCode(result.CurrentStep))
	fmt.Fprintf(out, "Prompt: %s\n", result.PromptPath)
	fmt.Fprintf(out, "\nPaste the prompt into Cursor, then save the artifact to the case directory.\n")
	fmt.Fprintf(out, "Run 'asterisk save' to ingest the artifact and advance.\n")
	return nil
}
