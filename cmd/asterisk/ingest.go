package main

import (
	"asterisk/adapters/rca"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	framework "github.com/dpopsuev/origami"
)

// IngestConfig provides configuration for the ingestion pipeline,
// injected via walker context at key "config".
type IngestConfig struct {
	RPProject    string
	LookbackDays int
	ScenarioPath string
	DatasetDir   string
	CandidateDir string
}

// LaunchInfo summarizes an RP launch for the pipeline.
type LaunchInfo struct {
	ID          int       `json:"id"`
	UUID        string    `json:"uuid"`
	Name        string    `json:"name"`
	Number      int       `json:"number"`
	Status      string    `json:"status"`
	StartTime   time.Time `json:"start_time"`
	FailedCount int       `json:"failed_count"`
}

// FailureInfo represents a parsed test failure from an RP launch.
type FailureInfo struct {
	LaunchID     int    `json:"launch_id"`
	LaunchName   string `json:"launch_name"`
	ItemID       int    `json:"item_id"`
	ItemUUID     string `json:"item_uuid"`
	TestName     string `json:"test_name"`
	Status       string `json:"status"`
	ErrorMessage string `json:"error_message"`
	IssueType    string `json:"issue_type,omitempty"`
	AutoAnalyzed bool   `json:"auto_analyzed,omitempty"`
}

// DedupKey generates the deduplication key for a failure.
func (f *FailureInfo) DedupKey(project string) string {
	return fmt.Sprintf("%s:%d:%d", project, f.LaunchID, f.ItemID)
}

// SymptomMatch holds the result of matching a failure against the symptom catalog.
type SymptomMatch struct {
	FailureInfo
	SymptomID   string  `json:"symptom_id,omitempty"`
	SymptomName string  `json:"symptom_name,omitempty"`
	Confidence  float64 `json:"confidence"`
	Matched     bool    `json:"matched"`
}

// CandidateCase is a candidate case ready for human review.
type CandidateCase struct {
	ID           string    `json:"id"`
	LaunchID     int       `json:"launch_id"`
	ItemID       int       `json:"item_id"`
	TestName     string    `json:"test_name"`
	ErrorMessage string    `json:"error_message"`
	SymptomID    string    `json:"symptom_id,omitempty"`
	SymptomName  string    `json:"symptom_name,omitempty"`
	Status       string    `json:"status"` // "candidate" or "verified"
	CreatedAt    time.Time `json:"created_at"`
	DedupKey     string    `json:"dedup_key"`
}

// IngestSummary is the output of the notify node.
type IngestSummary struct {
	LaunchesFetched   int `json:"launches_fetched"`
	FailuresParsed    int `json:"failures_parsed"`
	SymptomsMatched   int `json:"symptoms_matched"`
	Deduplicated      int `json:"deduplicated"`
	CandidatesCreated int `json:"candidates_created"`
}

// LaunchFetcher abstracts the RP API for listing launches.
type LaunchFetcher interface {
	FetchLaunches(project string, since time.Time) ([]LaunchInfo, error)
	FetchFailures(launchID int) ([]FailureInfo, error)
}

type ingestArtifact struct {
	typ  string
	data any
	conf float64
}

func (a *ingestArtifact) Type() string       { return a.typ }
func (a *ingestArtifact) Confidence() float64 { return a.conf }
func (a *ingestArtifact) Raw() any            { return a.data }

// --- FetchLaunchesNode ---

// FetchLaunchesNode calls the RP API to list recent launches.
type FetchLaunchesNode struct {
	Fetcher LaunchFetcher
}

func (n *FetchLaunchesNode) Name() string                       { return "fetch_launches" }
func (n *FetchLaunchesNode) ElementAffinity() framework.Element { return framework.ElementWater }

func (n *FetchLaunchesNode) Process(ctx context.Context, nc framework.NodeContext) (framework.Artifact, error) {
	cfg := extractConfig(nc)
	if cfg == nil {
		return nil, fmt.Errorf("fetch_launches: no IngestConfig in context")
	}

	lookback := time.Duration(cfg.LookbackDays) * 24 * time.Hour
	since := time.Now().Add(-lookback)

	launches, err := n.Fetcher.FetchLaunches(cfg.RPProject, since)
	if err != nil {
		return nil, fmt.Errorf("fetch_launches: %w", err)
	}

	nc.WalkerState.Context["launches"] = launches
	nc.WalkerState.MergeContext(map[string]any{
		"vars": map[string]any{"launches": launches},
	})

	return &ingestArtifact{typ: "launches", data: launches, conf: 1.0}, nil
}

// --- ParseFailuresNode ---

// ParseFailuresNode extracts failed test items from launches.
type ParseFailuresNode struct {
	Fetcher LaunchFetcher
}

func (n *ParseFailuresNode) Name() string                       { return "parse_failures" }
func (n *ParseFailuresNode) ElementAffinity() framework.Element { return framework.ElementFire }

func (n *ParseFailuresNode) Process(ctx context.Context, nc framework.NodeContext) (framework.Artifact, error) {
	launchesRaw, ok := nc.WalkerState.Context["launches"]
	if !ok {
		return nil, fmt.Errorf("parse_failures: no launches in context")
	}
	launches, ok := launchesRaw.([]LaunchInfo)
	if !ok {
		return nil, fmt.Errorf("parse_failures: launches type %T", launchesRaw)
	}

	var failures []FailureInfo
	for _, l := range launches {
		items, err := n.Fetcher.FetchFailures(l.ID)
		if err != nil {
			continue
		}
		failures = append(failures, items...)
	}

	nc.WalkerState.Context["failures"] = failures
	nc.WalkerState.MergeContext(map[string]any{
		"vars": map[string]any{"failures": failures},
	})

	return &ingestArtifact{typ: "failures", data: failures, conf: 1.0}, nil
}

// --- MatchSymptomsNode ---

// MatchSymptomsNode matches failures against the symptom catalog.
type MatchSymptomsNode struct {
	Symptoms []rca.GroundTruthSymptom
}

func (n *MatchSymptomsNode) Name() string                       { return "match_symptoms" }
func (n *MatchSymptomsNode) ElementAffinity() framework.Element { return framework.ElementEarth }

func (n *MatchSymptomsNode) Process(_ context.Context, nc framework.NodeContext) (framework.Artifact, error) {
	failuresRaw, ok := nc.WalkerState.Context["failures"]
	if !ok {
		return nil, fmt.Errorf("match_symptoms: no failures in context")
	}
	failures, ok := failuresRaw.([]FailureInfo)
	if !ok {
		return nil, fmt.Errorf("match_symptoms: failures type %T", failuresRaw)
	}

	var matches []SymptomMatch
	for _, f := range failures {
		match := SymptomMatch{FailureInfo: f}
		for _, s := range n.Symptoms {
			if matchesPattern(f.ErrorMessage, s.ErrorPattern) || matchesPattern(f.TestName, s.ErrorPattern) {
				match.SymptomID = s.ID
				match.SymptomName = s.Name
				match.Matched = true
				match.Confidence = 0.8
				break
			}
		}
		if !match.Matched {
			match.Confidence = 0.3
		}
		matches = append(matches, match)
	}

	nc.WalkerState.Context["matches"] = matches
	return &ingestArtifact{typ: "symptom_matches", data: matches, conf: 1.0}, nil
}

func matchesPattern(text, pattern string) bool {
	if pattern == "" {
		return false
	}
	re, err := regexp.Compile("(?i)" + pattern)
	if err != nil {
		return strings.Contains(strings.ToLower(text), strings.ToLower(pattern))
	}
	return re.MatchString(text)
}

// --- DeduplicateNode ---

// DeduplicateNode filters out failures already in the dataset.
type DeduplicateNode struct {
	Project string
	Index   *DedupIndex
}

func (n *DeduplicateNode) Name() string                       { return "deduplicate" }
func (n *DeduplicateNode) ElementAffinity() framework.Element { return framework.ElementEarth }

func (n *DeduplicateNode) Process(_ context.Context, nc framework.NodeContext) (framework.Artifact, error) {
	matchesRaw, ok := nc.WalkerState.Context["matches"]
	if !ok {
		return nil, fmt.Errorf("deduplicate: no matches in context")
	}
	matches, ok := matchesRaw.([]SymptomMatch)
	if !ok {
		return nil, fmt.Errorf("deduplicate: matches type %T", matchesRaw)
	}

	newCases, dupes := n.Index.Filter(n.Project, matches)

	nc.WalkerState.Context["new_cases"] = newCases
	nc.WalkerState.Context["dupes"] = dupes
	nc.WalkerState.MergeContext(map[string]any{
		"vars": map[string]any{"new_cases": newCases},
	})

	return &ingestArtifact{
		typ:  "dedup_result",
		data: map[string]any{"new_cases": newCases, "duplicates": dupes},
		conf: 1.0,
	}, nil
}

// --- CreateCandidatesNode ---

// CreateCandidatesNode writes candidate case JSON files to the output directory.
type CreateCandidatesNode struct {
	OutputDir string
	Project   string
}

func (n *CreateCandidatesNode) Name() string                       { return "create_candidates" }
func (n *CreateCandidatesNode) ElementAffinity() framework.Element { return framework.ElementFire }

func (n *CreateCandidatesNode) Process(_ context.Context, nc framework.NodeContext) (framework.Artifact, error) {
	casesRaw, ok := nc.WalkerState.Context["new_cases"]
	if !ok {
		return nil, fmt.Errorf("create_candidates: no new_cases in context")
	}
	newCases, ok := casesRaw.([]SymptomMatch)
	if !ok {
		return nil, fmt.Errorf("create_candidates: new_cases type %T", casesRaw)
	}

	if err := os.MkdirAll(n.OutputDir, 0o755); err != nil {
		return nil, fmt.Errorf("create_candidates: mkdir: %w", err)
	}

	var candidates []CandidateCase
	now := time.Now()

	for i, m := range newCases {
		c := CandidateCase{
			ID:           fmt.Sprintf("CAND-%d-%d", now.Unix(), i+1),
			LaunchID:     m.LaunchID,
			ItemID:       m.ItemID,
			TestName:     m.TestName,
			ErrorMessage: m.ErrorMessage,
			SymptomID:    m.SymptomID,
			SymptomName:  m.SymptomName,
			Status:       "candidate",
			CreatedAt:    now,
			DedupKey:     m.DedupKey(n.Project),
		}

		data, err := json.MarshalIndent(c, "", "  ")
		if err != nil {
			return nil, fmt.Errorf("create_candidates: marshal %s: %w", c.ID, err)
		}

		path := filepath.Join(n.OutputDir, c.ID+".json")
		if err := os.WriteFile(path, data, 0o644); err != nil {
			return nil, fmt.Errorf("create_candidates: write %s: %w", path, err)
		}

		candidates = append(candidates, c)
	}

	nc.WalkerState.Context["candidates"] = candidates
	return &ingestArtifact{typ: "candidates", data: candidates, conf: 1.0}, nil
}

// --- NotifyReviewNode ---

// NotifyReviewNode produces an IngestSummary and stores it in walker context.
type NotifyReviewNode struct{}

func (n *NotifyReviewNode) Name() string                       { return "notify_review" }
func (n *NotifyReviewNode) ElementAffinity() framework.Element { return framework.ElementAir }

func (n *NotifyReviewNode) Process(_ context.Context, nc framework.NodeContext) (framework.Artifact, error) {
	summary := IngestSummary{}

	if v, ok := nc.WalkerState.Context["launches"]; ok {
		if l, ok := v.([]LaunchInfo); ok {
			summary.LaunchesFetched = len(l)
		}
	}
	if v, ok := nc.WalkerState.Context["failures"]; ok {
		if f, ok := v.([]FailureInfo); ok {
			summary.FailuresParsed = len(f)
		}
	}
	if v, ok := nc.WalkerState.Context["matches"]; ok {
		if m, ok := v.([]SymptomMatch); ok {
			for _, match := range m {
				if match.Matched {
					summary.SymptomsMatched++
				}
			}
		}
	}
	if v, ok := nc.WalkerState.Context["dupes"]; ok {
		if d, ok := v.(int); ok {
			summary.Deduplicated = d
		}
	}
	if v, ok := nc.WalkerState.Context["candidates"]; ok {
		if c, ok := v.([]CandidateCase); ok {
			summary.CandidatesCreated = len(c)
		}
	}

	nc.WalkerState.Context["summary"] = summary
	return &ingestArtifact{typ: "ingest_summary", data: summary, conf: 1.0}, nil
}

// IngestNodeRegistry returns a NodeRegistry with all ingestion pipeline nodes.
func IngestNodeRegistry(fetcher LaunchFetcher, symptoms []rca.GroundTruthSymptom, project string, dedupIdx *DedupIndex, candidateDir string) framework.NodeRegistry {
	return framework.NodeRegistry{
		"ingest.fetch":     func(_ framework.NodeDef) framework.Node { return &FetchLaunchesNode{Fetcher: fetcher} },
		"ingest.parse":     func(_ framework.NodeDef) framework.Node { return &ParseFailuresNode{Fetcher: fetcher} },
		"ingest.match":     func(_ framework.NodeDef) framework.Node { return &MatchSymptomsNode{Symptoms: symptoms} },
		"ingest.dedup":     func(_ framework.NodeDef) framework.Node { return &DeduplicateNode{Project: project, Index: dedupIdx} },
		"ingest.candidate": func(_ framework.NodeDef) framework.Node { return &CreateCandidatesNode{OutputDir: candidateDir, Project: project} },
		"ingest.notify":    func(_ framework.NodeDef) framework.Node { return &NotifyReviewNode{} },
	}
}

func extractConfig(nc framework.NodeContext) *IngestConfig {
	raw, ok := nc.WalkerState.Context["config"]
	if !ok {
		return nil
	}
	cfg, ok := raw.(*IngestConfig)
	if !ok {
		return nil
	}
	return cfg
}

// --- DedupIndex ---

// DedupIndex tracks known dedup keys to prevent duplicate ingestion.
// Keys follow the format: {rp_project}:{launch_id}:{test_item_id}
type DedupIndex struct {
	known map[string]bool
}

// NewDedupIndex creates an empty dedup index.
func NewDedupIndex() *DedupIndex {
	return &DedupIndex{known: make(map[string]bool)}
}

// LoadDedupIndex scans a dataset directory and candidate directory for
// existing dedup keys. It reads JSON files looking for "dedup_key" fields.
func LoadDedupIndex(dirs ...string) (*DedupIndex, error) {
	idx := NewDedupIndex()
	for _, dir := range dirs {
		if err := idx.scanDir(dir); err != nil {
			return nil, err
		}
	}
	return idx, nil
}

func (d *DedupIndex) scanDir(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		var doc struct {
			DedupKey string `json:"dedup_key"`
		}
		if json.Unmarshal(data, &doc) == nil && doc.DedupKey != "" {
			d.known[doc.DedupKey] = true
		}
	}
	return nil
}

// Contains returns true if the key is already known.
func (d *DedupIndex) Contains(key string) bool {
	return d.known[key]
}

// Add marks a key as known.
func (d *DedupIndex) Add(key string) {
	d.known[key] = true
}

// Size returns the number of known keys.
func (d *DedupIndex) Size() int {
	return len(d.known)
}

// Filter returns only the matches whose dedup key is NOT in the index.
func (d *DedupIndex) Filter(project string, matches []SymptomMatch) (newCases []SymptomMatch, dupes int) {
	for _, m := range matches {
		key := m.DedupKey(project)
		if d.Contains(key) {
			dupes++
			continue
		}
		d.Add(key)
		newCases = append(newCases, m)
	}
	return
}
