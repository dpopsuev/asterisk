package calibrate_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"asterisk/internal/calibrate"
	"asterisk/internal/calibrate/adapt"
	"asterisk/internal/calibrate/scenarios"
	"asterisk/internal/orchestrate"
)

// TestWeaveTranscripts_StubAdapter runs a full calibration with the stub adapter
// and verifies the weaver produces transcripts grouped by RCA.
func TestWeaveTranscripts_StubAdapter(t *testing.T) {
	scenario := scenarios.PTPMockScenario()
	adapter := adapt.NewStubAdapter(scenario)

	tmpDir := t.TempDir()

	cfg := calibrate.RunConfig{
		Scenario:   scenario,
		Adapter:    adapter,
		Runs:       1,
		PromptDir:  ".cursor/prompts",
		Thresholds: orchestrate.DefaultThresholds(),
		BasePath:   tmpDir,
	}

	report, err := calibrate.RunCalibration(context.Background(), cfg)
	if err != nil {
		t.Fatalf("RunCalibration: %v", err)
	}

	transcripts, err := calibrate.WeaveTranscripts(report)
	if err != nil {
		t.Fatalf("WeaveTranscripts: %v", err)
	}

	if len(transcripts) == 0 {
		t.Fatal("expected at least one RCA transcript")
	}

	// Each transcript must have a primary case with entries.
	for _, tr := range transcripts {
		if tr.Primary == nil {
			t.Errorf("RCA %d: no primary case", tr.RCAID)
			continue
		}
		if len(tr.Primary.Entries) == 0 {
			t.Errorf("RCA %d (case %s): no entries in primary transcript",
				tr.RCAID, tr.Primary.CaseID)
		}
		if tr.Primary.TestName == "" {
			t.Errorf("RCA %d: primary case has empty test name", tr.RCAID)
		}
	}
}

// TestWeaveTranscripts_NilReport returns nil when report is nil.
func TestWeaveTranscripts_NilReport(t *testing.T) {
	result, err := calibrate.WeaveTranscripts(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Errorf("expected nil, got %d transcripts", len(result))
	}
}

// TestWeaveTranscripts_EmptyCaseResults returns nil when no case results.
func TestWeaveTranscripts_EmptyCaseResults(t *testing.T) {
	report := &calibrate.CalibrationReport{CaseResults: nil}
	result, err := calibrate.WeaveTranscripts(report)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Errorf("expected nil, got %d transcripts", len(result))
	}
}

// TestWeaveTranscripts_GroupsByRCA verifies cases with the same ActualRCAID
// appear in the same transcript.
func TestWeaveTranscripts_GroupsByRCA(t *testing.T) {
	scenario := scenarios.PTPMockScenario()
	adapter := adapt.NewStubAdapter(scenario)

	tmpDir := t.TempDir()

	cfg := calibrate.RunConfig{
		Scenario:   scenario,
		Adapter:    adapter,
		Runs:       1,
		PromptDir:  ".cursor/prompts",
		Thresholds: orchestrate.DefaultThresholds(),
		BasePath:   tmpDir,
	}

	report, err := calibrate.RunCalibration(context.Background(), cfg)
	if err != nil {
		t.Fatalf("RunCalibration: %v", err)
	}

	// Count how many distinct non-zero RCA IDs exist.
	rcaIDs := make(map[int64]int)
	orphans := 0
	for _, cr := range report.CaseResults {
		if cr.ActualRCAID == 0 {
			orphans++
		} else {
			rcaIDs[cr.ActualRCAID]++
		}
	}

	transcripts, err := calibrate.WeaveTranscripts(report)
	if err != nil {
		t.Fatalf("WeaveTranscripts: %v", err)
	}

	expectedGroups := len(rcaIDs) + orphans
	if len(transcripts) != expectedGroups {
		t.Errorf("expected %d transcript groups, got %d", expectedGroups, len(transcripts))
	}

	// Any RCA with >1 case should produce correlated entries.
	for _, tr := range transcripts {
		if count, ok := rcaIDs[tr.RCAID]; ok && count > 1 {
			expectedCorrelated := count - 1
			if len(tr.Correlated) != expectedCorrelated {
				t.Errorf("RCA %d: expected %d correlated, got %d",
					tr.RCAID, expectedCorrelated, len(tr.Correlated))
			}
		}
	}
}

// TestRenderRCATranscript_ReverseOrder verifies that the rendered Markdown
// places later pipeline steps before earlier ones.
func TestRenderRCATranscript_ReverseOrder(t *testing.T) {
	tr := &calibrate.RCATranscript{
		RCAID:      1,
		Component:  "ptp-operator",
		DefectType: "pb001",
		RCAMessage: "test rca",
		Primary: &calibrate.CaseTranscript{
			CaseID:   "C1",
			TestName: "test case 1",
			Version:  "4.20",
			Job:      "[T-TSC]",
			Path:     []string{"F0", "F1", "F3"},
			Entries: []calibrate.TranscriptEntry{
				{Step: "F0_RECALL", StepName: "Recall", Response: `{"match":false}`, HeuristicID: "H2", Decision: "recall miss", Timestamp: "2026-01-01T00:00:00Z"},
				{Step: "F1_TRIAGE", StepName: "Triage", Response: `{"symptom_category":"product"}`, HeuristicID: "H7", Decision: "single repo", Timestamp: "2026-01-01T00:00:01Z"},
				{Step: "F3_INVESTIGATE", StepName: "Investigate", Response: `{"defect_type":"pb001"}`, HeuristicID: "H9", Decision: "converged", Timestamp: "2026-01-01T00:00:02Z"},
			},
		},
	}

	md := calibrate.RenderRCATranscript(tr)

	// F3 must appear before F1, and F1 before F0 in the output.
	posF3 := strings.Index(md, "F3_INVESTIGATE")
	posF1 := strings.Index(md, "F1_TRIAGE")
	posF0 := strings.Index(md, "F0_RECALL")

	if posF3 < 0 || posF1 < 0 || posF0 < 0 {
		t.Fatalf("missing step references in rendered transcript")
	}
	if posF3 > posF1 {
		t.Errorf("F3 should appear before F1 in reverse order (posF3=%d > posF1=%d)", posF3, posF1)
	}
	if posF1 > posF0 {
		t.Errorf("F1 should appear before F0 in reverse order (posF1=%d > posF0=%d)", posF1, posF0)
	}
}

// TestRenderRCATranscript_IncludesPromptWhenAvailable verifies that prompt content
// appears in the rendered output when present.
func TestRenderRCATranscript_IncludesPromptWhenAvailable(t *testing.T) {
	tr := &calibrate.RCATranscript{
		RCAID:      1,
		Component:  "test-comp",
		DefectType: "pb001",
		Primary: &calibrate.CaseTranscript{
			CaseID:   "C1",
			TestName: "test",
			Path:     []string{"F0"},
			Entries: []calibrate.TranscriptEntry{
				{
					Step:        "F0_RECALL",
					StepName:    "Recall",
					Prompt:      "# F0 Recall Prompt\nDetermine similarity...",
					Response:    `{"match": false}`,
					HeuristicID: "H2",
					Decision:    "recall miss",
					Timestamp:   "2026-01-01T00:00:00Z",
				},
			},
		},
	}

	md := calibrate.RenderRCATranscript(tr)

	if !strings.Contains(md, "#### Prompt") {
		t.Error("expected Prompt section in output")
	}
	if !strings.Contains(md, "Determine similarity") {
		t.Error("expected prompt content in output")
	}
}

// TestRenderRCATranscript_OmitsPromptWhenEmpty verifies that the Prompt section
// is skipped when no prompt content is available (e.g. stub/basic adapter).
func TestRenderRCATranscript_OmitsPromptWhenEmpty(t *testing.T) {
	tr := &calibrate.RCATranscript{
		RCAID:      1,
		Component:  "test-comp",
		DefectType: "pb001",
		Primary: &calibrate.CaseTranscript{
			CaseID:   "C1",
			TestName: "test",
			Path:     []string{"F0"},
			Entries: []calibrate.TranscriptEntry{
				{Step: "F0_RECALL", StepName: "Recall", Prompt: "", Response: `{"match": false}`, HeuristicID: "H2", Decision: "recall miss", Timestamp: "2026-01-01T00:00:00Z"},
			},
		},
	}

	md := calibrate.RenderRCATranscript(tr)

	if strings.Contains(md, "#### Prompt") {
		t.Error("Prompt section should be omitted when prompt is empty")
	}
}

// TestTranscriptSlug verifies filesystem-safe slug generation.
func TestTranscriptSlug(t *testing.T) {
	tests := []struct {
		component  string
		defectType string
		want       string
	}{
		{"ptp-operator", "pb001", "rca-transcript-ptp-operator-pb001"},
		{"", "", "rca-transcript-unknown-unknown"},
		{"My Component", "AB001", "rca-transcript-my-component-ab001"},
	}

	for _, tc := range tests {
		tr := &calibrate.RCATranscript{Component: tc.component, DefectType: tc.defectType}
		got := calibrate.TranscriptSlug(tr)
		if got != tc.want {
			t.Errorf("TranscriptSlug(%q, %q) = %q, want %q", tc.component, tc.defectType, got, tc.want)
		}
	}
}

// TestWeaveTranscripts_WritesToDisk verifies end-to-end: run calibration,
// weave, render, and write transcript files to disk.
func TestWeaveTranscripts_WritesToDisk(t *testing.T) {
	scenario := scenarios.PTPMockScenario()
	adapter := adapt.NewStubAdapter(scenario)

	tmpDir := t.TempDir()

	cfg := calibrate.RunConfig{
		Scenario:   scenario,
		Adapter:    adapter,
		Runs:       1,
		PromptDir:  ".cursor/prompts",
		Thresholds: orchestrate.DefaultThresholds(),
		BasePath:   tmpDir,
	}

	report, err := calibrate.RunCalibration(context.Background(), cfg)
	if err != nil {
		t.Fatalf("RunCalibration: %v", err)
	}

	transcripts, err := calibrate.WeaveTranscripts(report)
	if err != nil {
		t.Fatalf("WeaveTranscripts: %v", err)
	}

	transcriptDir := filepath.Join(tmpDir, "transcripts")
	if err := os.MkdirAll(transcriptDir, 0755); err != nil {
		t.Fatalf("create transcript dir: %v", err)
	}

	for i := range transcripts {
		slug := calibrate.TranscriptSlug(&transcripts[i])
		md := calibrate.RenderRCATranscript(&transcripts[i])
		tPath := filepath.Join(transcriptDir, slug+".md")
		if err := os.WriteFile(tPath, []byte(md), 0644); err != nil {
			t.Fatalf("write transcript: %v", err)
		}
	}

	// Verify files exist.
	entries, err := os.ReadDir(transcriptDir)
	if err != nil {
		t.Fatalf("read transcript dir: %v", err)
	}

	if len(entries) == 0 {
		t.Fatal("no transcript files written")
	}

	// Verify each file is non-empty and contains expected markers.
	for _, entry := range entries {
		data, err := os.ReadFile(filepath.Join(transcriptDir, entry.Name()))
		if err != nil {
			t.Errorf("read %s: %v", entry.Name(), err)
			continue
		}
		content := string(data)
		if !strings.Contains(content, "# RCA Transcript") {
			t.Errorf("%s: missing RCA Transcript header", entry.Name())
		}
		if !strings.Contains(content, "#### Response") {
			t.Errorf("%s: missing Response section", entry.Name())
		}
		if !strings.Contains(content, "#### Decision") {
			t.Errorf("%s: missing Decision section", entry.Name())
		}
	}

	t.Logf("wrote %d transcript files to %s", len(entries), transcriptDir)
}
