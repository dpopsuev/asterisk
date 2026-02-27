package adapt

import (
	"encoding/json"
	"testing"

	"asterisk/internal/orchestrate"
	"asterisk/adapters/store"
)

func TestBasicAdapter_Name(t *testing.T) {
	st := store.NewMemStore()
	a := NewBasicAdapter(st, []string{"repo1"})
	if a.Name() != "basic" {
		t.Errorf("expected name 'basic', got %q", a.Name())
	}
}

func TestBasicAdapter_UnknownCase(t *testing.T) {
	st := store.NewMemStore()
	a := NewBasicAdapter(st, nil)

	_, err := a.SendPrompt("nonexistent", string(orchestrate.StepF0Recall), "")
	if err == nil {
		t.Error("expected error for unknown case")
	}
}

func TestBasicAdapter_SendPrompt_AllSteps(t *testing.T) {
	st := store.NewMemStore()
	a := NewBasicAdapter(st, []string{"linuxptp-daemon", "cnf-features-deploy"})
	a.RegisterCase("C1", &BasicCaseInfo{
		Name:         "[T-TSC] PTP Recovery test",
		ErrorMessage: "ptp4l clock offset exceeded",
		LogSnippet:   "phc2sys sync failed",
		StoreCaseID:  1,
	})

	steps := []orchestrate.PipelineStep{
		orchestrate.StepF0Recall,
		orchestrate.StepF1Triage,
		orchestrate.StepF2Resolve,
		orchestrate.StepF3Invest,
		orchestrate.StepF4Correlate,
		orchestrate.StepF5Review,
		orchestrate.StepF6Report,
	}

	for _, step := range steps {
		t.Run(string(step), func(t *testing.T) {
			data, err := a.SendPrompt("C1", string(step), "")
			if err != nil {
				t.Fatalf("SendPrompt(%s): %v", step, err)
			}
			if len(data) == 0 {
				t.Error("expected non-empty response")
			}
			// Verify it's valid JSON
			var raw json.RawMessage
			if err := json.Unmarshal(data, &raw); err != nil {
				t.Errorf("response is not valid JSON: %v", err)
			}
		})
	}
}

func TestBasicAdapter_InvalidStep(t *testing.T) {
	st := store.NewMemStore()
	a := NewBasicAdapter(st, nil)
	a.RegisterCase("C1", &BasicCaseInfo{Name: "test"})

	_, err := a.SendPrompt("C1", "invalid", "")
	if err == nil {
		t.Error("expected error for invalid step")
	}
}

func TestBasicAdapter_Triage_PTPDomain(t *testing.T) {
	st := store.NewMemStore()
	a := NewBasicAdapter(st, []string{"linuxptp-daemon"})

	tests := []struct {
		name         string
		info         *BasicCaseInfo
		wantCat      string
		wantHyp      string
		wantSkip     bool
	}{
		{
			"product bug with PTP keywords",
			&BasicCaseInfo{
				Name:         "PTP Recovery Test",
				ErrorMessage: "ptp4l clock sync failure",
			},
			"product", "pb001", false,
		},
		{
			"automation skip",
			&BasicCaseInfo{
				Name:         "Automation: setup check",
				ErrorMessage: "automation: version mismatch",
			},
			"automation", "au001", true,
		},
		{
			"infra timeout",
			&BasicCaseInfo{
				Name:         "Connection test",
				ErrorMessage: "connection refused after timeout",
			},
			"infra", "ti001", true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a.RegisterCase("C1", tt.info)
			data, err := a.SendPrompt("C1", string(orchestrate.StepF1Triage), "")
			if err != nil {
				t.Fatalf("SendPrompt: %v", err)
			}
			var result orchestrate.TriageResult
			if err := json.Unmarshal(data, &result); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if result.SymptomCategory != tt.wantCat {
				t.Errorf("category = %q, want %q", result.SymptomCategory, tt.wantCat)
			}
			if result.DefectTypeHypothesis != tt.wantHyp {
				t.Errorf("hypothesis = %q, want %q", result.DefectTypeHypothesis, tt.wantHyp)
			}
			if result.SkipInvestigation != tt.wantSkip {
				t.Errorf("skip = %v, want %v", result.SkipInvestigation, tt.wantSkip)
			}
		})
	}
}

func TestBasicAdapter_ComponentIdentification(t *testing.T) {
	st := store.NewMemStore()
	a := NewBasicAdapter(st, []string{"linuxptp-daemon", "cnf-features-deploy", "cloud-event-proxy", "cnf-gotests"})

	tests := []struct {
		name      string
		info      *BasicCaseInfo
		wantComp  string
	}{
		{
			"linuxptp-daemon via phc2sys",
			&BasicCaseInfo{ErrorMessage: "phc2sys sync failed"},
			"linuxptp-daemon",
		},
		{
			"cloud-event-proxy via cloud event",
			&BasicCaseInfo{ErrorMessage: "cloud event subscription lost"},
			"cloud-event-proxy",
		},
		{
			"cnf-features-deploy via losing subscription",
			&BasicCaseInfo{ErrorMessage: "losing subscription to events"},
			"cnf-features-deploy",
		},
		{
			"cnf-gotests via tracking issue",
			&BasicCaseInfo{ErrorMessage: "tracking issue for failures in ntpfailover-specific tests"},
			"cnf-gotests",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a.RegisterCase("C1", tt.info)
			data, err := a.SendPrompt("C1", string(orchestrate.StepF3Invest), "")
			if err != nil {
				t.Fatalf("SendPrompt: %v", err)
			}
			var result orchestrate.InvestigateArtifact
			if err := json.Unmarshal(data, &result); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if result.Component != tt.wantComp {
				t.Errorf("component = %q, want %q", result.Component, tt.wantComp)
			}
		})
	}
}

func TestBasicAdapter_Recall_NoMatch(t *testing.T) {
	st := store.NewMemStore()
	a := NewBasicAdapter(st, nil)
	a.RegisterCase("C1", &BasicCaseInfo{Name: "test", ErrorMessage: "error"})

	data, err := a.SendPrompt("C1", string(orchestrate.StepF0Recall), "")
	if err != nil {
		t.Fatalf("SendPrompt: %v", err)
	}
	var result orchestrate.RecallResult
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if result.Match {
		t.Error("expected no match for empty store")
	}
}

func TestBasicAdapter_Recall_SymptomMatch(t *testing.T) {
	st := store.NewMemStore()
	a := NewBasicAdapter(st, nil)

	// Pre-populate store with a symptom
	fp := orchestrate.ComputeFingerprint("test", "error", "")
	sym := &store.Symptom{
		Name:            "test",
		Fingerprint:     fp,
		ErrorPattern:    "error",
		Status:          "active",
		OccurrenceCount: 1,
	}
	symID, err := st.CreateSymptom(sym)
	if err != nil {
		t.Fatalf("create symptom: %v", err)
	}

	a.RegisterCase("C1", &BasicCaseInfo{Name: "test", ErrorMessage: "error"})
	data, err := a.SendPrompt("C1", string(orchestrate.StepF0Recall), "")
	if err != nil {
		t.Fatalf("SendPrompt: %v", err)
	}
	var result orchestrate.RecallResult
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !result.Match {
		t.Error("expected match with existing symptom")
	}
	if result.SymptomID != symID {
		t.Errorf("symptom ID = %d, want %d", result.SymptomID, symID)
	}
}

func TestBasicAdapter_Correlate_NoExistingRCA(t *testing.T) {
	st := store.NewMemStore()
	a := NewBasicAdapter(st, nil)
	a.RegisterCase("C1", &BasicCaseInfo{ErrorMessage: "some error"})

	data, err := a.SendPrompt("C1", string(orchestrate.StepF4Correlate), "")
	if err != nil {
		t.Fatalf("SendPrompt: %v", err)
	}
	var result orchestrate.CorrelateResult
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if result.IsDuplicate {
		t.Error("expected no duplicate for empty store")
	}
}

func TestExtractEvidenceRefs(t *testing.T) {
	tests := []struct {
		name      string
		errMsg    string
		component string
		wantLen   int
	}{
		{
			"component and jira",
			"OCPBUGS-12345 in linuxptp-daemon",
			"linuxptp-daemon",
			2, // component:relevant_source_file + OCPBUGS-12345
		},
		{
			"component only",
			"no jira here",
			"linuxptp-daemon",
			1,
		},
		{
			"unknown component",
			"OCPBUGS-99999",
			"unknown",
			1, // just jira
		},
		{
			"no evidence",
			"plain error",
			"unknown",
			0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			refs := extractEvidenceRefs(tt.errMsg, tt.component)
			if len(refs) != tt.wantLen {
				t.Errorf("got %d refs %v, want %d", len(refs), refs, tt.wantLen)
			}
		})
	}
}

func TestBasicAdapter_Correlate_MatchingRCA(t *testing.T) {
	st := store.NewMemStore()
	a := NewBasicAdapter(st, nil)

	// Use v1 SaveRCA since buildCorrelate uses ListRCAs (v1)
	rca := &store.RCA{
		Title:       "PTP sync error",
		Description: "clock sync failure detected",
		DefectType:  "pb001",
		Status:      "open",
	}
	_, err := st.SaveRCA(rca)
	if err != nil {
		t.Fatalf("save rca: %v", err)
	}

	// Register a case with a matching error
	a.RegisterCase("C1", &BasicCaseInfo{
		ErrorMessage: "clock sync failure detected",
	})

	data, err := a.SendPrompt("C1", string(orchestrate.StepF4Correlate), "")
	if err != nil {
		t.Fatalf("SendPrompt: %v", err)
	}
	var result orchestrate.CorrelateResult
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !result.IsDuplicate {
		t.Error("expected duplicate detection for matching RCA")
	}
	if result.LinkedRCAID == 0 {
		t.Error("expected linked RCA ID")
	}
	if result.Confidence <= 0 {
		t.Error("expected positive confidence")
	}
}

func TestBasicAdapter_Correlate_EmptyErrorMessage(t *testing.T) {
	st := store.NewMemStore()
	a := NewBasicAdapter(st, nil)

	// Use v1 SaveRCA since buildCorrelate uses ListRCAs (v1)
	rca := &store.RCA{Title: "something", Description: "description", Status: "open"}
	st.SaveRCA(rca)

	a.RegisterCase("C1", &BasicCaseInfo{ErrorMessage: ""})

	data, err := a.SendPrompt("C1", string(orchestrate.StepF4Correlate), "")
	if err != nil {
		t.Fatalf("SendPrompt: %v", err)
	}
	var result orchestrate.CorrelateResult
	json.Unmarshal(data, &result)
	if result.IsDuplicate {
		t.Error("empty error should not match")
	}
}

func TestBasicAdapter_ComponentIdentification_Extended(t *testing.T) {
	st := store.NewMemStore()
	a := NewBasicAdapter(st, []string{"linuxptp-daemon"})

	tests := []struct {
		name     string
		info     *BasicCaseInfo
		wantComp string
	}{
		{
			"ptp4l direct match",
			&BasicCaseInfo{ErrorMessage: "ptp4l process crashed"},
			"linuxptp-daemon",
		},
		{
			"clock state locked",
			&BasicCaseInfo{ErrorMessage: "clock state not locked after timeout"},
			"linuxptp-daemon",
		},
		{
			"offset threshold",
			&BasicCaseInfo{ErrorMessage: "offset threshold exceeded"},
			"linuxptp-daemon",
		},
		{
			"ptp_recovery.go file",
			&BasicCaseInfo{ErrorMessage: "error at ptp_recovery.go:100"},
			"linuxptp-daemon",
		},
		{
			"configmap update",
			&BasicCaseInfo{ErrorMessage: "configmap update failed for ptp"},
			"cloud-event-proxy",
		},
		{
			"sidecar container",
			&BasicCaseInfo{ErrorMessage: "sidecar container not ready"},
			"cloud-event-proxy",
		},
		{
			"gnss sync state",
			&BasicCaseInfo{ErrorMessage: "gnss sync state changed"},
			"cloud-event-proxy",
		},
		{
			"ocpbugs-49372",
			&BasicCaseInfo{ErrorMessage: "regression OCPBUGS-49372"},
			"cnf-features-deploy",
		},
		{
			"ocpbugs-54967 jira hint",
			&BasicCaseInfo{ErrorMessage: "OCPBUGS-54967 clock issue"},
			"linuxptp-daemon",
		},
		{
			"workload partitioning default",
			&BasicCaseInfo{ErrorMessage: "workload partitioning test failure"},
			"linuxptp-daemon",
		},
		{
			"unknown component",
			&BasicCaseInfo{ErrorMessage: "totally unrelated error"},
			"unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a.RegisterCase("C1", tt.info)
			data, err := a.SendPrompt("C1", string(orchestrate.StepF3Invest), "")
			if err != nil {
				t.Fatalf("SendPrompt: %v", err)
			}
			var result orchestrate.InvestigateArtifact
			json.Unmarshal(data, &result)
			if result.Component != tt.wantComp {
				t.Errorf("component = %q, want %q", result.Component, tt.wantComp)
			}
		})
	}
}

func TestBasicAdapter_Resolve_UnknownComponent(t *testing.T) {
	st := store.NewMemStore()
	repos := []string{"repo1", "repo2"}
	a := NewBasicAdapter(st, repos)
	a.RegisterCase("C1", &BasicCaseInfo{
		Name:         "generic test",
		ErrorMessage: "completely unrecognized issue",
	})

	data, err := a.SendPrompt("C1", string(orchestrate.StepF2Resolve), "")
	if err != nil {
		t.Fatalf("SendPrompt: %v", err)
	}
	var result orchestrate.ResolveResult
	json.Unmarshal(data, &result)

	// When component is unknown, all workspace repos should be selected
	if len(result.SelectedRepos) != 2 {
		t.Errorf("expected 2 repos for unknown component, got %d", len(result.SelectedRepos))
	}
}

func TestBasicAdapter_Investigate_UnknownComponent(t *testing.T) {
	st := store.NewMemStore()
	a := NewBasicAdapter(st, nil)
	a.RegisterCase("C1", &BasicCaseInfo{
		Name:         "generic test",
		ErrorMessage: "completely unrecognized issue",
	})

	data, err := a.SendPrompt("C1", string(orchestrate.StepF3Invest), "")
	if err != nil {
		t.Fatalf("SendPrompt: %v", err)
	}
	var result orchestrate.InvestigateArtifact
	json.Unmarshal(data, &result)

	if result.ConvergenceScore != 0.70 {
		t.Errorf("convergence = %f, want 0.70 for unknown component (BasicAdapter signals done)", result.ConvergenceScore)
	}
}

func TestBasicMatchCount(t *testing.T) {
	tests := []struct {
		text     string
		keywords []string
		want     int
	}{
		{"ptp clock offset", []string{"ptp", "clock"}, 2},
		{"nothing here", []string{"ptp", "clock"}, 0},
		{"ptp ptp ptp", []string{"ptp"}, 1},
	}
	for _, tt := range tests {
		got := basicMatchCount(tt.text, tt.keywords)
		if got != tt.want {
			t.Errorf("basicMatchCount(%q, %v) = %d, want %d", tt.text, tt.keywords, got, tt.want)
		}
	}
}
