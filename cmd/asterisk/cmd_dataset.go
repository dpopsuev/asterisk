package main

import (
	"asterisk/internal/ingest"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
)

var (
	datasetCandidateDir string
	datasetDir          string
)

var datasetCmd = &cobra.Command{
	Use:   "dataset",
	Short: "Dataset management — review and promote candidate cases",
	Long:  "Manage the ground truth dataset: review ingested candidates, promote to verified.",
}

var datasetReviewCmd = &cobra.Command{
	Use:   "review",
	Short: "List all candidate cases pending review",
	RunE: func(cmd *cobra.Command, args []string) error {
		entries, err := os.ReadDir(datasetCandidateDir)
		if err != nil {
			if os.IsNotExist(err) {
				fmt.Fprintln(cmd.OutOrStdout(), "No candidates directory found. Run 'asterisk consume run' first.")
				return nil
			}
			return fmt.Errorf("read candidates: %w", err)
		}

		w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 4, 2, ' ', 0)
		fmt.Fprintf(w, "ID\tTest\tSymptom\tCreated\tStatus\n")
		fmt.Fprintf(w, "--\t----\t-------\t-------\t------\n")

		count := 0
		for _, e := range entries {
			if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
				continue
			}
			data, err := os.ReadFile(filepath.Join(datasetCandidateDir, e.Name()))
			if err != nil {
				continue
			}
			var c ingest.CandidateCase
			if json.Unmarshal(data, &c) != nil {
				continue
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
				c.ID,
				truncate(c.TestName, 40),
				c.SymptomName,
				c.CreatedAt.Format("2006-01-02"),
				c.Status,
			)
			count++
		}
		w.Flush()

		if count == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "No candidates found.")
		} else {
			fmt.Fprintf(cmd.OutOrStdout(), "\nTotal: %d candidates\n", count)
		}
		return nil
	},
}

var datasetPromoteCmd = &cobra.Command{
	Use:   "promote <candidate-id>",
	Short: "Promote a candidate case to verified status",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		candidateID := args[0]

		srcPath := filepath.Join(datasetCandidateDir, candidateID+".json")
		data, err := os.ReadFile(srcPath)
		if err != nil {
			return fmt.Errorf("candidate %q not found: %w", candidateID, err)
		}

		var c ingest.CandidateCase
		if err := json.Unmarshal(data, &c); err != nil {
			return fmt.Errorf("parse candidate: %w", err)
		}

		c.Status = "verified"

		verifiedID, err := nextVerifiedID(datasetDir)
		if err != nil {
			return fmt.Errorf("assign ID: %w", err)
		}

		c.ID = verifiedID

		if err := os.MkdirAll(datasetDir, 0o755); err != nil {
			return fmt.Errorf("create dataset dir: %w", err)
		}

		out, err := json.MarshalIndent(c, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal: %w", err)
		}

		dstPath := filepath.Join(datasetDir, verifiedID+".json")
		if err := os.WriteFile(dstPath, out, 0o644); err != nil {
			return fmt.Errorf("write: %w", err)
		}

		if err := os.Remove(srcPath); err != nil {
			return fmt.Errorf("remove candidate: %w", err)
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Promoted %s → %s (verified)\n", candidateID, verifiedID)
		return nil
	},
}

var datasetStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show dataset and ingestion status",
	RunE: func(cmd *cobra.Command, args []string) error {
		verified := countJSONFiles(datasetDir)
		candidates := countJSONFiles(datasetCandidateDir)

		fmt.Fprintf(cmd.OutOrStdout(), "Dataset:    %d verified cases (%s)\n", verified, datasetDir)
		fmt.Fprintf(cmd.OutOrStdout(), "Candidates: %d pending review (%s)\n", candidates, datasetCandidateDir)
		return nil
	},
}

func nextVerifiedID(dir string) (string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}

	maxNum := 0
	for _, e := range entries {
		name := strings.TrimSuffix(e.Name(), ".json")
		var num int
		if _, err := fmt.Sscanf(name, "V%d", &num); err == nil && num > maxNum {
			maxNum = num
		}
	}

	return fmt.Sprintf("V%03d", maxNum+1), nil
}

func countJSONFiles(dir string) int {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}
	n := 0
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".json" {
			n++
		}
	}
	return n
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}

func init() {
	_ = time.Now // ensure time is imported for future use

	datasetCmd.PersistentFlags().StringVar(&datasetCandidateDir, "candidate-dir", "candidates", "Directory for candidate case files")
	datasetCmd.PersistentFlags().StringVar(&datasetDir, "dataset-dir", "datasets", "Directory for verified dataset files")

	datasetCmd.AddCommand(datasetReviewCmd)
	datasetCmd.AddCommand(datasetPromoteCmd)
	datasetCmd.AddCommand(datasetStatusCmd)
	rootCmd.AddCommand(datasetCmd)
}
