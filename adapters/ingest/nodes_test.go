package ingest

import (
	"asterisk/adapters/rca"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	framework "github.com/dpopsuev/origami"
)

// --- Mock fetcher ---

type mockFetcher struct {
	launches []LaunchInfo
	failures map[int][]FailureInfo
}

func (m *mockFetcher) FetchLaunches(_ string, _ time.Time) ([]LaunchInfo, error) {
	return m.launches, nil
}

func (m *mockFetcher) FetchFailures(launchID int) ([]FailureInfo, error) {
	return m.failures[launchID], nil
}

func newMockFetcher() *mockFetcher {
	return &mockFetcher{
		launches: []LaunchInfo{
			{ID: 100, Name: "run-100", Status: "FAILED", FailedCount: 2},
			{ID: 101, Name: "run-101", Status: "FAILED", FailedCount: 1},
		},
		failures: map[int][]FailureInfo{
			100: {
				{LaunchID: 100, LaunchName: "run-100", ItemID: 1001, TestName: "TestPTP_Sync", ErrorMessage: "ptp4l sync timeout", Status: "FAILED"},
				{LaunchID: 100, LaunchName: "run-100", ItemID: 1002, TestName: "TestPTP_Config", ErrorMessage: "config mismatch error", Status: "FAILED"},
			},
			101: {
				{LaunchID: 101, LaunchName: "run-101", ItemID: 2001, TestName: "TestPTP_Sync", ErrorMessage: "ptp4l sync timeout", Status: "FAILED"},
			},
		},
	}
}

func testSymptoms() []rca.GroundTruthSymptom {
	return []rca.GroundTruthSymptom{
		{ID: "S1", Name: "PTP Sync Timeout", ErrorPattern: "ptp4l sync timeout", Component: "ptp-operator", MapsToRCA: "R1"},
		{ID: "S2", Name: "Config Mismatch", ErrorPattern: "config mismatch", Component: "ptp-operator", MapsToRCA: "R2"},
	}
}

func TestFetchLaunchesNode(t *testing.T) {
	fetcher := newMockFetcher()
	node := &FetchLaunchesNode{Fetcher: fetcher}

	ws := framework.NewWalkerState("test")
	ws.Context["config"] = &IngestConfig{
		RPProject:    "ptp-ci",
		LookbackDays: 7,
	}

	art, err := node.Process(context.Background(), framework.NodeContext{WalkerState: ws})
	if err != nil {
		t.Fatalf("Process: %v", err)
	}

	launches := art.Raw().([]LaunchInfo)
	if len(launches) != 2 {
		t.Errorf("launches = %d, want 2", len(launches))
	}
}

func TestParseFailuresNode(t *testing.T) {
	fetcher := newMockFetcher()
	node := &ParseFailuresNode{Fetcher: fetcher}

	ws := framework.NewWalkerState("test")
	ws.Context["launches"] = fetcher.launches

	art, err := node.Process(context.Background(), framework.NodeContext{WalkerState: ws})
	if err != nil {
		t.Fatalf("Process: %v", err)
	}

	failures := art.Raw().([]FailureInfo)
	if len(failures) != 3 {
		t.Errorf("failures = %d, want 3", len(failures))
	}
}

func TestMatchSymptomsNode(t *testing.T) {
	node := &MatchSymptomsNode{Symptoms: testSymptoms()}

	ws := framework.NewWalkerState("test")
	ws.Context["failures"] = []FailureInfo{
		{LaunchID: 100, ItemID: 1001, TestName: "TestPTP_Sync", ErrorMessage: "ptp4l sync timeout"},
		{LaunchID: 100, ItemID: 1002, TestName: "TestPTP_Config", ErrorMessage: "config mismatch error"},
		{LaunchID: 100, ItemID: 1003, TestName: "TestUnknown", ErrorMessage: "something else entirely"},
	}

	art, err := node.Process(context.Background(), framework.NodeContext{WalkerState: ws})
	if err != nil {
		t.Fatalf("Process: %v", err)
	}

	matches := art.Raw().([]SymptomMatch)
	if len(matches) != 3 {
		t.Errorf("matches = %d, want 3", len(matches))
	}

	matchedCount := 0
	for _, m := range matches {
		if m.Matched {
			matchedCount++
		}
	}
	if matchedCount != 2 {
		t.Errorf("matched = %d, want 2", matchedCount)
	}
}

func TestDeduplicateNode(t *testing.T) {
	idx := NewDedupIndex()
	idx.Add("ptp-ci:100:1001")

	node := &DeduplicateNode{Project: "ptp-ci", Index: idx}

	ws := framework.NewWalkerState("test")
	ws.Context["matches"] = []SymptomMatch{
		{FailureInfo: FailureInfo{LaunchID: 100, ItemID: 1001, TestName: "TestPTP_Sync"}, Matched: true},
		{FailureInfo: FailureInfo{LaunchID: 100, ItemID: 1002, TestName: "TestPTP_Config"}, Matched: true},
		{FailureInfo: FailureInfo{LaunchID: 101, ItemID: 2001, TestName: "TestPTP_Sync"}, Matched: true},
	}

	art, err := node.Process(context.Background(), framework.NodeContext{WalkerState: ws})
	if err != nil {
		t.Fatalf("Process: %v", err)
	}

	result := art.Raw().(map[string]any)
	newCases := result["new_cases"].([]SymptomMatch)
	dupes := result["duplicates"].(int)

	if len(newCases) != 2 {
		t.Errorf("new_cases = %d, want 2", len(newCases))
	}
	if dupes != 1 {
		t.Errorf("dupes = %d, want 1", dupes)
	}
}

func TestCreateCandidatesNode(t *testing.T) {
	dir := t.TempDir()
	node := &CreateCandidatesNode{OutputDir: dir, Project: "ptp-ci"}

	ws := framework.NewWalkerState("test")
	ws.Context["new_cases"] = []SymptomMatch{
		{FailureInfo: FailureInfo{LaunchID: 100, ItemID: 1002, TestName: "TestPTP_Config", ErrorMessage: "config mismatch"}, SymptomID: "S2", SymptomName: "Config Mismatch", Matched: true},
	}

	art, err := node.Process(context.Background(), framework.NodeContext{WalkerState: ws})
	if err != nil {
		t.Fatalf("Process: %v", err)
	}

	candidates := art.Raw().([]CandidateCase)
	if len(candidates) != 1 {
		t.Fatalf("candidates = %d, want 1", len(candidates))
	}

	if candidates[0].Status != "candidate" {
		t.Errorf("status = %q, want candidate", candidates[0].Status)
	}

	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Errorf("files = %d, want 1", len(entries))
	}

	data, _ := os.ReadFile(filepath.Join(dir, entries[0].Name()))
	var saved CandidateCase
	if err := json.Unmarshal(data, &saved); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if saved.DedupKey != "ptp-ci:100:1002" {
		t.Errorf("dedup_key = %q, want ptp-ci:100:1002", saved.DedupKey)
	}
}

func TestNotifyReviewNode(t *testing.T) {
	node := &NotifyReviewNode{}

	ws := framework.NewWalkerState("test")
	ws.Context["launches"] = []LaunchInfo{{ID: 1}, {ID: 2}}
	ws.Context["failures"] = []FailureInfo{{ItemID: 1}, {ItemID: 2}, {ItemID: 3}}
	ws.Context["matches"] = []SymptomMatch{
		{Matched: true},
		{Matched: false},
		{Matched: true},
	}
	ws.Context["dupes"] = 1
	ws.Context["candidates"] = []CandidateCase{{ID: "C1"}, {ID: "C2"}}

	art, err := node.Process(context.Background(), framework.NodeContext{WalkerState: ws})
	if err != nil {
		t.Fatalf("Process: %v", err)
	}

	summary := art.Raw().(IngestSummary)
	if summary.LaunchesFetched != 2 {
		t.Errorf("launches = %d, want 2", summary.LaunchesFetched)
	}
	if summary.FailuresParsed != 3 {
		t.Errorf("failures = %d, want 3", summary.FailuresParsed)
	}
	if summary.SymptomsMatched != 2 {
		t.Errorf("matched = %d, want 2", summary.SymptomsMatched)
	}
	if summary.Deduplicated != 1 {
		t.Errorf("deduped = %d, want 1", summary.Deduplicated)
	}
	if summary.CandidatesCreated != 2 {
		t.Errorf("candidates = %d, want 2", summary.CandidatesCreated)
	}
}

func TestDedupIndex_SameLaunchTwice(t *testing.T) {
	idx := NewDedupIndex()

	matches := []SymptomMatch{
		{FailureInfo: FailureInfo{LaunchID: 100, ItemID: 1001}, Matched: true},
	}

	// First run
	newCases1, dupes1 := idx.Filter("proj", matches)
	if len(newCases1) != 1 || dupes1 != 0 {
		t.Errorf("first run: new=%d dupes=%d, want 1, 0", len(newCases1), dupes1)
	}

	// Second run with same failure
	newCases2, dupes2 := idx.Filter("proj", matches)
	if len(newCases2) != 0 || dupes2 != 1 {
		t.Errorf("second run: new=%d dupes=%d, want 0, 1", len(newCases2), dupes2)
	}
}

func TestDedupIndex_LoadFromDir(t *testing.T) {
	dir := t.TempDir()

	c := CandidateCase{ID: "C1", DedupKey: "proj:100:1001"}
	data, _ := json.Marshal(c)
	os.WriteFile(filepath.Join(dir, "C1.json"), data, 0o644)

	idx, err := LoadDedupIndex(dir)
	if err != nil {
		t.Fatalf("LoadDedupIndex: %v", err)
	}
	if !idx.Contains("proj:100:1001") {
		t.Error("expected key to be loaded")
	}
	if idx.Contains("proj:200:2001") {
		t.Error("unexpected key")
	}
}
