package rca_test

import (
	"fmt"
	"testing"

	"asterisk/adapters/rca/adapt"
	"asterisk/adapters/store"
	"asterisk/adapters/rca"
)

func TestRunAnalysis_BasicAdapter(t *testing.T) {
	tmpDir := t.TempDir()
	st := store.NewMemStore()

	suite := &store.InvestigationSuite{Name: "test", Status: "active"}
	suiteID, err := st.CreateSuite(suite)
	if err != nil {
		t.Fatalf("create suite: %v", err)
	}

	v := &store.Version{Label: "4.20"}
	vid, err := st.CreateVersion(v)
	if err != nil {
		t.Fatalf("create version: %v", err)
	}

	pipe := &store.Pipeline{SuiteID: suiteID, VersionID: vid, Name: "CI", Status: "complete"}
	pipeID, err := st.CreatePipeline(pipe)
	if err != nil {
		t.Fatalf("create pipeline: %v", err)
	}

	launch := &store.Launch{PipelineID: pipeID, Name: "Launch", Status: "complete"}
	launchID, err := st.CreateLaunch(launch)
	if err != nil {
		t.Fatalf("create launch: %v", err)
	}

	job := &store.Job{LaunchID: launchID, Name: "[T-TSC]", Status: "complete"}
	jobID, err := st.CreateJob(job)
	if err != nil {
		t.Fatalf("create job: %v", err)
	}

	// Create test cases
	caseInfos := []struct {
		name string
		err  string
		log  string
	}{
		{"PTP Recovery Test", "ptp4l clock offset exceeded", "phc2sys sync failed"},
		{"Cloud Event Test", "cloud event subscription lost", "events proxy error"},
		{"Automation Test", "automation: test setup failed", "ginkgo internal error"},
	}

	adapter := adapt.NewBasicAdapter(st, []string{"linuxptp-daemon", "cloud-event-proxy"})
	var storeCases []*store.Case

	for i, ci := range caseInfos {
		c := &store.Case{
			JobID:        jobID,
			LaunchID:     launchID,
			Name:         ci.name,
			Status:       "open",
			ErrorMessage: ci.err,
			LogSnippet:   ci.log,
		}
		caseID, err := st.CreateCaseV2(c)
		if err != nil {
			t.Fatalf("create case %d: %v", i, err)
		}
		c.ID = caseID
		storeCases = append(storeCases, c)

		// Register with the adapter using the label RunAnalysis will use
		label := fmt.Sprintf("A%d", i+1)
		adapter.RegisterCase(label, &adapt.BasicCaseInfo{
			Name:         ci.name,
			ErrorMessage: ci.err,
			LogSnippet:   ci.log,
			StoreCaseID:  caseID,
		})
	}

	cfg := rca.AnalysisConfig{
		Adapter:    adapter,
		Thresholds: rca.DefaultThresholds(),
		BasePath:   tmpDir,
	}

	report, err := rca.RunAnalysis(st, storeCases, suiteID, cfg)
	if err != nil {
		t.Fatalf("RunAnalysis: %v", err)
	}

	if report.TotalCases != 3 {
		t.Errorf("expected 3 total cases, got %d", report.TotalCases)
	}
	if len(report.CaseResults) != 3 {
		t.Errorf("expected 3 case results, got %d", len(report.CaseResults))
	}

	// Each case should have a non-empty path
	for _, cr := range report.CaseResults {
		if len(cr.Path) == 0 {
			t.Errorf("case %s has empty path", cr.CaseLabel)
		}
	}

	// The third case (automation) should be skipped
	if len(report.CaseResults) >= 3 {
		cr := report.CaseResults[2]
		if !cr.Skip {
			t.Errorf("case A3 (automation) should be skipped")
		}
	}
}

func TestRunAnalysis_EmptyCases(t *testing.T) {
	tmpDir := t.TempDir()
	st := store.NewMemStore()
	adapter := adapt.NewBasicAdapter(st, nil)

	cfg := rca.AnalysisConfig{
		Adapter:    adapter,
		Thresholds: rca.DefaultThresholds(),
		BasePath:   tmpDir,
	}

	report, err := rca.RunAnalysis(st, nil, 1, cfg)
	if err != nil {
		t.Fatalf("RunAnalysis with empty cases: %v", err)
	}
	if report.TotalCases != 0 {
		t.Errorf("expected 0 total cases, got %d", report.TotalCases)
	}
	if len(report.CaseResults) != 0 {
		t.Errorf("expected 0 case results, got %d", len(report.CaseResults))
	}
}

func TestFormatAnalysisReport_Structure(t *testing.T) {
	report := &rca.AnalysisReport{
		LaunchName: "test-launch",
		Adapter:    "stub",
		TotalCases: 2,
		CaseResults: []rca.AnalysisCaseResult{
			{
				CaseLabel:   "A1",
				TestName:    "Test PTP Recovery",
				DefectType:  "pb001",
				Category:    "product",
				Path:        []string{"F0", "F1", "F2", "F3", "F5", "F6"},
				RecallHit:   true,
				Convergence: 0.85,
				RCAMessage:  "PTP clock offset exceeded threshold",
			},
			{
				CaseLabel: "A2",
				TestName:  "Test Automation Skip",
				Category:  "automation",
				Path:      []string{"F0", "F1"},
				Skip:      true,
			},
		},
	}

	output := rca.FormatAnalysisReport(report)
	if len(output) == 0 {
		t.Fatal("FormatAnalysisReport returned empty string")
	}

	checks := []string{
		"Asterisk Analysis Report",
		"test-launch",
		"stub",
		"Recall hits:",
		"Skipped:",
		"Per-case breakdown",
		"A1",
		"A2",
		"PTP clock offset exceeded threshold",
	}
	for _, check := range checks {
		if !containsStr(output, check) {
			t.Errorf("report missing expected text: %q", check)
		}
	}
}
