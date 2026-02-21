package main

import (
	"asterisk/internal/origami"
	"context"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

var gtDataDir string

var gtCmd = &cobra.Command{
	Use:   "gt",
	Short: "Ground truth dataset management",
	Long:  "Manage ground truth datasets: status, import, export.",
}

var gtStatusCmd = &cobra.Command{
	Use:   "status [scenario]",
	Short: "Show dataset completeness overview",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		store := origami.NewFileStore(gtDataDir)
		ctx := context.Background()

		if len(args) == 0 {
			names, err := store.List(ctx)
			if err != nil {
				return err
			}
			if len(names) == 0 {
				fmt.Println("No datasets found in", gtDataDir)
				return nil
			}
			fmt.Printf("Datasets in %s:\n", gtDataDir)
			for _, n := range names {
				fmt.Printf("  %s\n", n)
			}
			return nil
		}

		scenario, err := store.Load(ctx, args[0])
		if err != nil {
			return err
		}

		results := origami.CheckScenario(scenario)

		w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
		fmt.Fprintf(w, "Case\tRCA\tScore\tPromotable\tMissing\n")
		fmt.Fprintf(w, "----\t---\t-----\t----------\t-------\n")

		promotable := 0
		for _, r := range results {
			status := "no"
			if r.Promotable {
				status = "YES"
				promotable++
			}
			missing := ""
			if len(r.Missing) > 0 && len(r.Missing) <= 3 {
				missing = fmt.Sprintf("%v", r.Missing)
			} else if len(r.Missing) > 3 {
				missing = fmt.Sprintf("%d fields", len(r.Missing))
			}
			fmt.Fprintf(w, "%s\t%s\t%.0f%%\t%s\t%s\n",
				r.CaseID, r.RCAID, r.Score*100, status, missing)
		}
		w.Flush()

		fmt.Printf("\nTotal: %d cases, %d promotable, %d need work\n",
			len(results), promotable, len(results)-promotable)

		return nil
	},
}

func init() {
	gtCmd.PersistentFlags().StringVar(&gtDataDir, "data-dir", "datasets", "Directory for ground truth JSON files")
	gtCmd.AddCommand(gtStatusCmd)
	rootCmd.AddCommand(gtCmd)
}
