package main

import (
	"asterisk/internal/calibrate/scenarios"
	"asterisk/internal/ingest"
	"context"
	"fmt"
	"os"
	"time"

	framework "github.com/dpopsuev/origami"
	"github.com/spf13/cobra"
)

var (
	consumeProject      string
	consumeLookbackDays int
	consumeCandidateDir string
	consumeDatasetDir   string
	consumeDryRun       bool
)

var consumeCmd = &cobra.Command{
	Use:   "consume",
	Short: "Data ingestion commands",
	Long:  "Discover new CI failures and create candidate cases for dataset growth.",
}

var consumeRunCmd = &cobra.Command{
	Use:   "run",
	Short: "Walk the ingestion pipeline to discover new failures",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		scenario := scenarios.PTPRealIngestScenario()
		symptoms := scenario.Symptoms

		dedupIdx, err := ingest.LoadDedupIndex(consumeDatasetDir, consumeCandidateDir)
		if err != nil {
			return fmt.Errorf("load dedup index: %w", err)
		}

		fetcher := &stubFetcher{}
		if consumeDryRun {
			fmt.Fprintln(cmd.OutOrStdout(), "[dry-run] Using stub fetcher (no RP API calls)")
		}

		nodeReg := ingest.IngestNodeRegistry(
			fetcher, symptoms, consumeProject, dedupIdx, consumeCandidateDir,
		)

		pipelineData, err := os.ReadFile("pipelines/asterisk-rp.yaml")
		if err != nil {
			return fmt.Errorf("read pipeline: %w", err)
		}

		def, err := framework.LoadPipeline(pipelineData)
		if err != nil {
			return fmt.Errorf("parse pipeline: %w", err)
		}

		edgeIDs := make([]string, len(def.Edges))
		for i, ed := range def.Edges {
			edgeIDs[i] = ed.ID
		}
		edgeFactory := make(framework.EdgeFactory, len(edgeIDs))
		for _, id := range edgeIDs {
			edgeFactory[id] = func(ed framework.EdgeDef) framework.Edge {
				return &consumeForwardEdge{def: ed}
			}
		}

		reg := framework.GraphRegistries{
			Nodes: nodeReg,
			Edges: edgeFactory,
		}

		graph, err := def.BuildGraph(reg)
		if err != nil {
			return fmt.Errorf("build graph: %w", err)
		}

		walker := framework.NewProcessWalker("consume")
		walker.State().Context["config"] = &ingest.IngestConfig{
			RPProject:    consumeProject,
			LookbackDays: consumeLookbackDays,
			DatasetDir:   consumeDatasetDir,
			CandidateDir: consumeCandidateDir,
		}

		if err := graph.Walk(ctx, walker, def.Start); err != nil {
			return fmt.Errorf("walk pipeline: %w", err)
		}

		if summary, ok := walker.State().Context["summary"]; ok {
			if s, ok := summary.(ingest.IngestSummary); ok {
				fmt.Fprintf(cmd.OutOrStdout(),
					"Fetched %d launches, found %d failures, matched %d symptoms, "+
						"created %d candidates (%d deduplicated)\n",
					s.LaunchesFetched, s.FailuresParsed, s.SymptomsMatched,
					s.CandidatesCreated, s.Deduplicated)
			}
		}

		return nil
	},
}

type consumeForwardEdge struct {
	def framework.EdgeDef
}

func (e *consumeForwardEdge) ID() string         { return e.def.ID }
func (e *consumeForwardEdge) From() string       { return e.def.From }
func (e *consumeForwardEdge) To() string         { return e.def.To }
func (e *consumeForwardEdge) IsShortcut() bool   { return e.def.Shortcut }
func (e *consumeForwardEdge) IsLoop() bool       { return e.def.Loop }
func (e *consumeForwardEdge) Evaluate(_ framework.Artifact, _ *framework.WalkerState) *framework.Transition {
	return &framework.Transition{NextNode: e.def.To}
}

type stubFetcher struct{}

func (f *stubFetcher) FetchLaunches(_ string, _ time.Time) ([]ingest.LaunchInfo, error) {
	return nil, nil
}
func (f *stubFetcher) FetchFailures(_ int) ([]ingest.FailureInfo, error) {
	return nil, nil
}

func init() {
	consumeRunCmd.Flags().StringVar(&consumeProject, "project", "", "RP project name")
	consumeRunCmd.Flags().IntVar(&consumeLookbackDays, "lookback", 7, "Days to look back for launches")
	consumeRunCmd.Flags().StringVar(&consumeCandidateDir, "candidate-dir", "candidates", "Directory for candidate case files")
	consumeRunCmd.Flags().StringVar(&consumeDatasetDir, "dataset-dir", "datasets", "Directory for verified dataset files")
	consumeRunCmd.Flags().BoolVar(&consumeDryRun, "dry-run", false, "Use stub fetcher (no RP API calls)")

	consumeCmd.AddCommand(consumeRunCmd)
}
