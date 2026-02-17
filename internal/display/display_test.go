package display

import "testing"

func TestDefectType(t *testing.T) {
	cases := []struct {
		code, want string
	}{
		{"pb001", "Product Bug"},
		{"au001", "Automation Bug"},
		{"ab001", "Automation Bug"},
		{"en001", "Environment Issue"},
		{"fw001", "Firmware Issue"},
		{"ti001", "To Investigate"},
		{"si001", "System Issue"},
		{"nd001", "No Defect"},
		{"ib003", "Infrastructure Bug"},
		{"unknown", "unknown"},
		{"", ""},
	}
	for _, tc := range cases {
		if got := DefectType(tc.code); got != tc.want {
			t.Errorf("DefectType(%q) = %q, want %q", tc.code, got, tc.want)
		}
	}
}

func TestDefectTypeWithCode(t *testing.T) {
	if got := DefectTypeWithCode("pb001"); got != "Product Bug (pb001)" {
		t.Errorf("got %q", got)
	}
	if got := DefectTypeWithCode("unknown"); got != "unknown" {
		t.Errorf("got %q", got)
	}
}

func TestStage(t *testing.T) {
	cases := []struct {
		code, want string
	}{
		{"F0", "Recall"},
		{"F1", "Triage"},
		{"F2", "Resolve"},
		{"F3", "Investigate"},
		{"F4", "Correlate"},
		{"F5", "Review"},
		{"F6", "Report"},
		{"F0_RECALL", "Recall"},
		{"F1_TRIAGE", "Triage"},
		{"F3_INVESTIGATE", "Investigate"},
		{"INIT", "Init"},
		{"DONE", "Done"},
		{"UNKNOWN_STEP", "UNKNOWN_STEP"},
	}
	for _, tc := range cases {
		if got := Stage(tc.code); got != tc.want {
			t.Errorf("Stage(%q) = %q, want %q", tc.code, got, tc.want)
		}
	}
}

func TestStageWithCode(t *testing.T) {
	if got := StageWithCode("F0_RECALL"); got != "Recall (F0)" {
		t.Errorf("got %q", got)
	}
	if got := StageWithCode("F3"); got != "Investigate (F3)" {
		t.Errorf("got %q", got)
	}
	if got := StageWithCode("INIT"); got != "Init" {
		t.Errorf("got %q, want %q", got, "Init")
	}
}

func TestStagePath(t *testing.T) {
	got := StagePath([]string{"F0", "F1", "F2", "F3"})
	want := "Recall \u2192 Triage \u2192 Resolve \u2192 Investigate"
	if got != want {
		t.Errorf("StagePath = %q, want %q", got, want)
	}
	if got := StagePath(nil); got != "" {
		t.Errorf("StagePath(nil) = %q, want empty", got)
	}
}

func TestMetric(t *testing.T) {
	if got := Metric("M1"); got != "Defect Type Accuracy" {
		t.Errorf("got %q", got)
	}
	if got := Metric("M19"); got != "Overall Accuracy" {
		t.Errorf("got %q", got)
	}
	if got := Metric("M99"); got != "M99" {
		t.Errorf("got %q", got)
	}
}

func TestMetricWithCode(t *testing.T) {
	if got := MetricWithCode("M1"); got != "Defect Type Accuracy (M1)" {
		t.Errorf("got %q", got)
	}
}

func TestHeuristic(t *testing.T) {
	if got := Heuristic("H1"); got != "Recall Hit" {
		t.Errorf("got %q", got)
	}
	if got := Heuristic("H15"); got != "Review Reassess" {
		t.Errorf("got %q", got)
	}
	if got := Heuristic("H99"); got != "H99" {
		t.Errorf("got %q", got)
	}
}

func TestHeuristicWithCode(t *testing.T) {
	if got := HeuristicWithCode("H1"); got != "Recall Hit (H1)" {
		t.Errorf("got %q", got)
	}
}

func TestClusterKey(t *testing.T) {
	got := ClusterKey("product|ptp4l|pb001")
	want := "product / ptp4l / Product Bug"
	if got != want {
		t.Errorf("ClusterKey = %q, want %q", got, want)
	}
}
