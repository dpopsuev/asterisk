package calibrate_test

import (
	"context"
	"testing"

	"asterisk/internal/calibrate"
	"asterisk/internal/calibrate/adapt"
	"asterisk/internal/calibrate/scenarios"
	"asterisk/internal/orchestrate"
)

func TestStubCalibration_AllMetricsPass(t *testing.T) {
	// Override the orchestrate base path to a temp dir
	tmpDir := t.TempDir()
	// basePath is passed via RunConfig.BasePath below

	scenario := scenarios.PTPMockScenario()
	adapter := adapt.NewStubAdapter(scenario)
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

	// All 20 metrics should pass
	passed, total := report.Metrics.PassCount()
	if passed != total {
		t.Errorf("metrics: %d/%d passed; expected all to pass", passed, total)
		for _, m := range report.Metrics.AllMetrics() {
			if !m.Pass {
				t.Errorf("  FAIL: %s (%s) = %.2f (threshold %.2f) detail=%s",
					m.ID, m.Name, m.Value, m.Threshold, m.Detail)
			}
		}
	}

	// Verify case count
	if len(report.CaseResults) != 12 {
		t.Errorf("expected 12 case results, got %d", len(report.CaseResults))
	}

	// All 12 paths should be correct
	correctPaths := 0
	for _, cr := range report.CaseResults {
		if cr.PathCorrect {
			correctPaths++
		}
	}
	if correctPaths != 12 {
		t.Errorf("expected 12 correct paths, got %d", correctPaths)
		for _, cr := range report.CaseResults {
			if !cr.PathCorrect {
				t.Logf("  %s: actual=%v", cr.CaseID, cr.ActualPath)
			}
		}
	}

	// Spot-check serial killer detection (R1 across versions)
	r1Cases := map[string]bool{"C1": true, "C2": true, "C3": true, "C6": true, "C9": true, "C10": true}
	var r1RCAIDs []int64
	for _, cr := range report.CaseResults {
		if r1Cases[cr.CaseID] {
			if cr.ActualRCAID == 0 {
				t.Errorf("case %s should have an RCA link", cr.CaseID)
			}
			r1RCAIDs = append(r1RCAIDs, cr.ActualRCAID)
		}
	}
	if len(r1RCAIDs) > 1 {
		first := r1RCAIDs[0]
		for _, id := range r1RCAIDs[1:] {
			if id != first {
				t.Errorf("serial killer: not all R1 cases linked to same RCA (%v)", r1RCAIDs)
				break
			}
		}
	}
}

func TestStubCalibration_MultiRun(t *testing.T) {
	tmpDir := t.TempDir()
	// basePath is passed via RunConfig.BasePath below

	scenario := scenarios.PTPMockScenario()
	adapter := adapt.NewStubAdapter(scenario)
	cfg := calibrate.RunConfig{
		Scenario:   scenario,
		Adapter:    adapter,
		Runs:       3,
		PromptDir:  ".cursor/prompts",
		Thresholds: orchestrate.DefaultThresholds(),
		BasePath:   tmpDir,
	}

	report, err := calibrate.RunCalibration(context.Background(), cfg)
	if err != nil {
		t.Fatalf("RunCalibration: %v", err)
	}

	// Variance (M20) should be 0 for deterministic stub
	for _, m := range report.Metrics.AllMetrics() {
		if m.ID == "M20" && m.Value != 0 {
			t.Errorf("M20 run_variance should be 0 for deterministic stub, got %f", m.Value)
		}
	}

	// All metrics should still pass
	passed, total := report.Metrics.PassCount()
	if passed != total {
		t.Errorf("multi-run: %d/%d passed", passed, total)
	}
}

func TestFormatReport(t *testing.T) {
	tmpDir := t.TempDir()
	// basePath is passed via RunConfig.BasePath below

	scenario := scenarios.PTPMockScenario()
	adapter := adapt.NewStubAdapter(scenario)
	cfg := calibrate.DefaultRunConfig(scenario, adapter)
	cfg.Thresholds = orchestrate.DefaultThresholds()
	cfg.BasePath = tmpDir

	report, err := calibrate.RunCalibration(context.Background(), cfg)
	if err != nil {
		t.Fatalf("RunCalibration: %v", err)
	}

	output := calibrate.FormatReport(report)
	if len(output) == 0 {
		t.Fatal("FormatReport returned empty string")
	}

	// Verify key sections are present (auto-generated from Tier)
	checks := []string{
		"Asterisk Calibration Report",
		"ptp-mock",
		"stub",
		"Outcome",
		"Investigation",
		"Detection",
		"Efficiency",
		"Meta",
		"RESULT: PASS",
		"Per-case breakdown",
	}
	for _, check := range checks {
		if !containsStr(output, check) {
			t.Errorf("report missing expected text: %q", check)
		}
	}
}

func TestStubCalibration_DaemonMock(t *testing.T) {
	tmpDir := t.TempDir()
	// basePath is passed via RunConfig.BasePath below

	scenario := scenarios.DaemonMockScenario()
	adapter := adapt.NewStubAdapter(scenario)
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

	passed, total := report.Metrics.PassCount()
	if passed != total {
		t.Errorf("daemon-mock metrics: %d/%d passed", passed, total)
		for _, m := range report.Metrics.AllMetrics() {
			if !m.Pass {
				t.Errorf("  FAIL: %s (%s) = %.2f (threshold %.2f) detail=%s",
					m.ID, m.Name, m.Value, m.Threshold, m.Detail)
			}
		}
	}

	if len(report.CaseResults) != 8 {
		t.Errorf("expected 8 case results, got %d", len(report.CaseResults))
	}

	// All paths should be correct
	for _, cr := range report.CaseResults {
		if !cr.PathCorrect {
			t.Errorf("case %s path incorrect: actual=%v", cr.CaseID, cr.ActualPath)
		}
	}

	// Verify cascade detected for C6
	for _, cr := range report.CaseResults {
		if cr.CaseID == "C6" && !cr.ActualCascade {
			t.Error("C6 should have cascade detected")
		}
	}
}

func TestStubCalibration_PTPReal(t *testing.T) {
	tmpDir := t.TempDir()
	// basePath is passed via RunConfig.BasePath below

	scenario := scenarios.PTPRealScenario()
	adapter := adapt.NewStubAdapter(scenario)
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

	passed, total := report.Metrics.PassCount()
	if passed != total {
		t.Errorf("ptp-real metrics: %d/%d passed", passed, total)
		for _, m := range report.Metrics.AllMetrics() {
			if !m.Pass {
				t.Errorf("  FAIL: %s (%s) = %.2f (threshold %.2f) detail=%s",
					m.ID, m.Name, m.Value, m.Threshold, m.Detail)
			}
		}
	}

	if len(report.CaseResults) != 8 {
		t.Errorf("expected 8 case results, got %d", len(report.CaseResults))
	}

	// Verify PANIC case (C2) handled correctly
	for _, cr := range report.CaseResults {
		if cr.CaseID == "C2" {
			if !cr.ActualRecallHit {
				t.Error("C2 (PANICKED) should have recall hit")
			}
			if cr.ActualDefectType != "pb001" {
				t.Errorf("C2 defect type: got %s, want pb001", cr.ActualDefectType)
			}
		}
	}

	// Verify cascade detection for C6
	for _, cr := range report.CaseResults {
		if cr.CaseID == "C6" && !cr.ActualCascade {
			t.Error("C6 (BeforeEach cascade) should have cascade detected")
		}
	}

	// Verify R1 serial killer: C4, C5, C6, C7, C8 should all link to same RCA
	r1Cases := map[string]bool{"C4": true, "C5": true, "C6": true, "C7": true, "C8": true}
	var r1IDs []int64
	for _, cr := range report.CaseResults {
		if r1Cases[cr.CaseID] && cr.ActualRCAID != 0 {
			r1IDs = append(r1IDs, cr.ActualRCAID)
		}
	}
	if len(r1IDs) > 1 {
		first := r1IDs[0]
		for _, id := range r1IDs[1:] {
			if id != first {
				t.Errorf("R1 serial killer: not all cases linked to same RCA (%v)", r1IDs)
				break
			}
		}
	}
}

func TestScenarioCoverage(t *testing.T) {
	testCases := []struct {
		name     string
		scenario *calibrate.Scenario
		rcas     int
		symptoms int
		cases    int
		repos    int
	}{
		{"ptp-mock", scenarios.PTPMockScenario(), 3, 4, 12, 5},
		{"daemon-mock", scenarios.DaemonMockScenario(), 2, 3, 8, 5},
		{"ptp-real", scenarios.PTPRealScenario(), 2, 3, 8, 5},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			s := tc.scenario

			if len(s.RCAs) != tc.rcas {
				t.Errorf("expected %d RCAs, got %d", tc.rcas, len(s.RCAs))
			}
			if len(s.Symptoms) != tc.symptoms {
				t.Errorf("expected %d symptoms, got %d", tc.symptoms, len(s.Symptoms))
			}
			if len(s.Cases) != tc.cases {
				t.Errorf("expected %d cases, got %d", tc.cases, len(s.Cases))
			}
			if len(s.Workspace.Repos) != tc.repos {
				t.Errorf("expected %d workspace repos, got %d", tc.repos, len(s.Workspace.Repos))
			}

			// Check red herring exists
			hasRedHerring := false
			for _, r := range s.Workspace.Repos {
				if r.IsRedHerring {
					hasRedHerring = true
				}
			}
			if !hasRedHerring {
				t.Error("workspace should have at least one red herring repo")
			}

			// Check all cases reference valid RCAs and symptoms
			rcaSet := make(map[string]bool)
			for _, r := range s.RCAs {
				rcaSet[r.ID] = true
			}
			symSet := make(map[string]bool)
			for _, sym := range s.Symptoms {
				symSet[sym.ID] = true
			}
			for _, c := range s.Cases {
				if c.RCAID != "" && !rcaSet[c.RCAID] {
					t.Errorf("case %s references unknown RCA %q", c.ID, c.RCAID)
				}
				if c.SymptomID != "" && !symSet[c.SymptomID] {
					t.Errorf("case %s references unknown symptom %q", c.ID, c.SymptomID)
				}
			}
		})
	}
}

func containsStr(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && len(s) >= len(substr) &&
		(s == substr || findSubstring(s, substr))
}

func findSubstring(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
