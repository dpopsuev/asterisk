package main

import (
	"github.com/dpopsuev/origami/adapters/rp"
	"asterisk/adapters/calibration/scenarios"
	"asterisk/adapters/ingest"
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
	consumeRPBase       string
	consumeRPKeyPath    string
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

		var fetcher ingest.LaunchFetcher
		if consumeDryRun {
			fmt.Fprintln(cmd.OutOrStdout(), "[dry-run] Using stub fetcher (no RP API calls)")
			fetcher = &stubFetcher{}
		} else {
			rpBase := consumeRPBase
			if rpBase == "" {
				rpBase = os.Getenv("ASTERISK_RP_URL")
			}
			if rpBase == "" {
				return fmt.Errorf("RP base URL required: set --rp-base-url or $ASTERISK_RP_URL")
			}
			key, err := rp.ReadAPIKey(consumeRPKeyPath)
			if err != nil {
				return fmt.Errorf("read RP API key: %w", err)
			}
			client, err := rp.New(rpBase, key, rp.WithTimeout(30*time.Second))
			if err != nil {
				return fmt.Errorf("create RP client: %w", err)
			}
			fetcher = &rpLaunchFetcher{client: client, project: consumeProject}
		}

		nodeReg := ingest.IngestNodeRegistry(
			fetcher, symptoms, consumeProject, dedupIdx, consumeCandidateDir,
		)

		pipelineData, err := os.ReadFile("pipelines/asterisk-ingest.yaml")
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

type rpLaunchFetcher struct {
	client  *rp.Client
	project string
}

func (f *rpLaunchFetcher) FetchLaunches(project string, since time.Time) ([]ingest.LaunchInfo, error) {
	ctx := context.Background()
	paged, err := f.client.Project(project).Launches().List(ctx,
		rp.WithPageSize(100),
		rp.WithSort("startTime,desc"),
	)
	if err != nil {
		return nil, fmt.Errorf("list launches: %w", err)
	}

	var launches []ingest.LaunchInfo
	for _, l := range paged.Content {
		var startTime time.Time
		if l.StartTime != nil {
			startTime = l.StartTime.Time()
		}
		if !since.IsZero() && startTime.Before(since) {
			continue
		}
		failed := 0
		if l.Statistics != nil {
			if execs, ok := l.Statistics.Executions["failed"]; ok {
				failed = execs
			}
		}
		launches = append(launches, ingest.LaunchInfo{
			ID:          l.ID,
			UUID:        l.UUID,
			Name:        l.Name,
			Number:      l.Number,
			Status:      l.Status,
			StartTime:   startTime,
			FailedCount: failed,
		})
	}
	return launches, nil
}

func (f *rpLaunchFetcher) FetchFailures(launchID int) ([]ingest.FailureInfo, error) {
	ctx := context.Background()
	items, err := f.client.Project(f.project).Items().ListAll(ctx,
		rp.WithLaunchID(launchID),
		rp.WithStatus("FAILED"),
	)
	if err != nil {
		return nil, fmt.Errorf("list failed items: %w", err)
	}

	var failures []ingest.FailureInfo
	for _, item := range items {
		fi := ingest.FailureInfo{
			LaunchID: launchID,
			ItemID:   item.ID,
			ItemUUID: item.UUID,
			TestName: item.Name,
			Status:   item.Status,
		}
		if item.Issue != nil {
			fi.IssueType = item.Issue.IssueType
			fi.AutoAnalyzed = item.Issue.AutoAnalyzed
		}
		failures = append(failures, fi)
	}
	return failures, nil
}

func init() {
	consumeRunCmd.Flags().StringVar(&consumeProject, "project", "", "RP project name")
	consumeRunCmd.Flags().IntVar(&consumeLookbackDays, "lookback", 7, "Days to look back for launches")
	consumeRunCmd.Flags().StringVar(&consumeCandidateDir, "candidate-dir", "candidates", "Directory for candidate case files")
	consumeRunCmd.Flags().StringVar(&consumeDatasetDir, "dataset-dir", "datasets", "Directory for verified dataset files")
	consumeRunCmd.Flags().BoolVar(&consumeDryRun, "dry-run", false, "Use stub fetcher (no RP API calls)")
	consumeRunCmd.Flags().StringVar(&consumeRPBase, "rp-base-url", "", "RP base URL (default: $ASTERISK_RP_URL)")
	consumeRunCmd.Flags().StringVar(&consumeRPKeyPath, "rp-api-key", ".rp-api-key", "Path to RP API key file")

	consumeCmd.AddCommand(consumeRunCmd)
}
