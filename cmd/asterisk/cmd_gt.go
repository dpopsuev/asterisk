package main

import (
	"asterisk/internal/calibrate"
	"asterisk/internal/calibrate/scenarios"
	"asterisk/internal/dataset"
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
		store := dataset.NewFileStore(gtDataDir)
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

		results := dataset.CheckScenario(scenario)

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

var gtImportCmd = &cobra.Command{
	Use:   "import <scenario>",
	Short: "Export a Go scenario to JSON in the datasets directory",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		scenario, err := lookupGoScenario(args[0])
		if err != nil {
			return err
		}

		store := dataset.NewFileStore(gtDataDir)
		if err := store.Save(context.Background(), scenario); err != nil {
			return fmt.Errorf("save dataset: %w", err)
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Exported %d cases (%d candidates) to %s/%s.json\n",
			len(scenario.Cases), len(scenario.Candidates), gtDataDir, scenario.Name)
		return nil
	},
}

var gtExportCmd = &cobra.Command{
	Use:   "export <scenario>",
	Short: "Load a JSON dataset for calibration use",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		store := dataset.NewFileStore(gtDataDir)
		scenario, err := store.Load(context.Background(), args[0])
		if err != nil {
			return err
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Loaded %q: %d cases, %d candidates, %d RCAs, %d symptoms\n",
			scenario.Name, len(scenario.Cases), len(scenario.Candidates), len(scenario.RCAs), len(scenario.Symptoms))
		return nil
	},
}

func lookupGoScenario(name string) (*calibrate.Scenario, error) {
	switch name {
	case "ptp-mock":
		return scenarios.PTPMockScenario(), nil
	case "daemon-mock":
		return scenarios.DaemonMockScenario(), nil
	case "ptp-real":
		return scenarios.PTPRealScenario(), nil
	case "ptp-real-ingest":
		return scenarios.PTPRealIngestScenario(), nil
	default:
		return nil, fmt.Errorf("unknown scenario: %s (available: ptp-mock, daemon-mock, ptp-real, ptp-real-ingest)", name)
	}
}

func init() {
	gtCmd.PersistentFlags().StringVar(&gtDataDir, "data-dir", "datasets", "Directory for ground truth JSON files")
	gtCmd.AddCommand(gtStatusCmd)
	gtCmd.AddCommand(gtImportCmd)
	gtCmd.AddCommand(gtExportCmd)
	rootCmd.AddCommand(gtCmd)
}
