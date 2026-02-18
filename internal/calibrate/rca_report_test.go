package calibrate

import (
	"strings"
	"testing"
	"time"
)

var testTime = time.Date(2026, 2, 18, 14, 30, 0, 0, time.UTC)

func TestRenderRCAReport_EmptyReport(t *testing.T) {
	got := RenderRCAReport(nil, testTime)
	if !strings.Contains(got, "No failures analyzed") {
		t.Errorf("expected empty-report message, got:\n%s", got)
	}

	got = RenderRCAReport(&AnalysisReport{}, testTime)
	if !strings.Contains(got, "No failures analyzed") {
		t.Errorf("expected empty-report message for zero cases, got:\n%s", got)
	}
}

func TestRenderRCAReport_SingleCase(t *testing.T) {
	report := &AnalysisReport{
		LaunchName: "test-launch-4.20",
		Adapter:    "basic",
		TotalCases: 1,
		CaseResults: []AnalysisCaseResult{
			{
				CaseLabel:      "A1",
				TestName:       "should have ptp4l in UP state",
				DefectType:     "pb001",
				Category:       "product",
				RCAMessage:     "Suspected component: linuxptp-daemon",
				Component:      "linuxptp-daemon",
				Path:           []string{"F0", "F1", "F3", "F4", "F5", "F6"},
				EvidenceRefs:   []string{"linuxptp-daemon:relevant_source_file"},
				SelectedRepos:  []string{"linuxptp-daemon"},
				Convergence:    0.80,
				RCAID:          1,
				RPIssueType:    "ti_abc123",
				RPAutoAnalyzed: false,
			},
		},
	}

	got := RenderRCAReport(report, testTime)

	checks := []string{
		"# RCA Report — test-launch-4.20",
		"2026-02-18 14:30 UTC",
		"basic",
		"Product Bug (pb001)",
		"linuxptp-daemon",
		"80%",
		"[human]",
		"Recall",
		"Suspected component: linuxptp-daemon",
		"linuxptp-daemon:relevant_source_file",
	}
	for _, want := range checks {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in report:\n%s", want, got)
		}
	}
}

func TestRenderRCAReport_MultipleComponentsGrouped(t *testing.T) {
	report := &AnalysisReport{
		LaunchName: "test-launch",
		Adapter:    "basic",
		TotalCases: 3,
		CaseResults: []AnalysisCaseResult{
			{CaseLabel: "A1", TestName: "test-1", DefectType: "pb001", Component: "comp-a", Convergence: 0.70, RCAID: 1},
			{CaseLabel: "A2", TestName: "test-2", DefectType: "pb001", Component: "comp-b", Convergence: 0.80, RCAID: 2},
			{CaseLabel: "A3", TestName: "test-3", DefectType: "au001", Component: "comp-a", Convergence: 0.75, RCAID: 3},
		},
	}

	got := RenderRCAReport(report, testTime)

	if !strings.Contains(got, "comp-a (2 failures)") {
		t.Errorf("expected comp-a grouped with 2 failures, got:\n%s", got)
	}
	if !strings.Contains(got, "comp-b (1 failure)") {
		t.Errorf("expected comp-b grouped with 1 failure, got:\n%s", got)
	}

	compAIdx := strings.Index(got, "comp-a (2 failures)")
	compBIdx := strings.Index(got, "comp-b (1 failure)")
	if compAIdx > compBIdx {
		t.Error("expected comp-a before comp-b (alphabetical)")
	}
}

func TestRenderRCAReport_RPTags(t *testing.T) {
	report := &AnalysisReport{
		LaunchName: "rp-test",
		Adapter:    "basic",
		TotalCases: 2,
		CaseResults: []AnalysisCaseResult{
			{CaseLabel: "A1", TestName: "t1", DefectType: "pb001", Component: "comp",
				RPIssueType: "ti_human", RPAutoAnalyzed: false, Convergence: 0.80, RCAID: 1},
			{CaseLabel: "A2", TestName: "t2", DefectType: "au001", Component: "comp",
				RPIssueType: "ti_auto", RPAutoAnalyzed: true, Convergence: 0.70, RCAID: 2},
		},
	}

	got := RenderRCAReport(report, testTime)

	if !strings.Contains(got, "[human]") {
		t.Errorf("expected [human] tag, got:\n%s", got)
	}
	if !strings.Contains(got, "[auto]") {
		t.Errorf("expected [auto] tag, got:\n%s", got)
	}
}

func TestRenderRCAReport_Flags(t *testing.T) {
	report := &AnalysisReport{
		LaunchName: "flags-test",
		Adapter:    "basic",
		TotalCases: 3,
		CaseResults: []AnalysisCaseResult{
			{CaseLabel: "A1", TestName: "t1", DefectType: "pb001", Component: "c", RecallHit: true, RCAID: 1},
			{CaseLabel: "A2", TestName: "t2", DefectType: "au001", Component: "c", Skip: true},
			{CaseLabel: "A3", TestName: "t3", DefectType: "pb001", Component: "c", Cascade: true, RCAID: 3},
		},
	}

	got := RenderRCAReport(report, testTime)

	if !strings.Contains(got, "recall-hit") {
		t.Errorf("expected recall-hit flag, got:\n%s", got)
	}
	if !strings.Contains(got, "skipped") {
		t.Errorf("expected skipped flag, got:\n%s", got)
	}
	if !strings.Contains(got, "cascade") {
		t.Errorf("expected cascade flag, got:\n%s", got)
	}
}

func TestRenderRCAReport_ConvergenceRounding(t *testing.T) {
	report := &AnalysisReport{
		LaunchName: "conv-test",
		Adapter:    "basic",
		TotalCases: 1,
		CaseResults: []AnalysisCaseResult{
			{CaseLabel: "A1", TestName: "t1", DefectType: "pb001", Component: "c",
				Convergence: 0.7999999999999999, RCAID: 1},
		},
	}

	got := RenderRCAReport(report, testTime)

	if !strings.Contains(got, "80%") {
		t.Errorf("expected convergence rounded to 80%%, got:\n%s", got)
	}
	if strings.Contains(got, "79%") || strings.Contains(got, "0.79") {
		t.Errorf("convergence not properly rounded, got:\n%s", got)
	}
}

func TestRenderRCAReport_UnknownComponent(t *testing.T) {
	report := &AnalysisReport{
		LaunchName: "unknown-test",
		Adapter:    "basic",
		TotalCases: 1,
		CaseResults: []AnalysisCaseResult{
			{CaseLabel: "A1", TestName: "t1", DefectType: "en001", Component: "", Skip: true},
		},
	}

	got := RenderRCAReport(report, testTime)

	if !strings.Contains(got, "unknown") {
		t.Errorf("expected 'unknown' for empty component, got:\n%s", got)
	}
}

func TestRenderRCAReport_EvidenceDeduplication(t *testing.T) {
	report := &AnalysisReport{
		LaunchName: "evidence-test",
		Adapter:    "basic",
		TotalCases: 2,
		CaseResults: []AnalysisCaseResult{
			{CaseLabel: "A1", TestName: "t1", DefectType: "pb001", Component: "comp",
				EvidenceRefs: []string{"comp:file_a", "comp:file_b"}, RCAID: 1},
			{CaseLabel: "A2", TestName: "t2", DefectType: "pb001", Component: "comp",
				EvidenceRefs: []string{"comp:file_a", "comp:file_c"}, RCAID: 2},
		},
	}

	got := RenderRCAReport(report, testTime)

	count := strings.Count(got, "comp:file_a")
	// Once in the component section, twice in per-case details
	if count < 2 {
		t.Errorf("expected comp:file_a to appear in component section and case details, count=%d", count)
	}
}
