package rca

import (
	"os"
	"path/filepath"
	"testing"

	"asterisk/adapters/rp"
	"asterisk/adapters/store"
	"github.com/dpopsuev/origami/knowledge"
)

// TestBuildParams_RecallWithPriorSymptom verifies that when a prior symptom+RCA
// exists in the store for the same test name, BuildParams at F0_RECALL populates
// History so the recall prompt can render "Known symptom" and "Prior RCAs".
//
// RED scenario: C1 was fully investigated, creating a Symptom and RCA.
// C2 arrives with the same test name — its recall prompt should show C1's data.
func TestBuildParams_RecallWithPriorSymptom(t *testing.T) {
	st := store.NewMemStore()

	// C1 was investigated: symptom and RCA exist in the store
	symID, err := st.CreateSymptom(&store.Symptom{
		Name:            "[T-TSC] PTP sync stability test",
		Fingerprint:     "fp-ptp-sync-product",
		ErrorPattern:    "ptp4l[12345.678]: master offset -999999 s2 freq",
		Component:       "product",
		Status:          "active",
		OccurrenceCount: 1,
	})
	if err != nil {
		t.Fatalf("CreateSymptom: %v", err)
	}

	rcaID, err := st.SaveRCAV2(&store.RCA{
		Title:            "linuxptp-daemon: time jump during switchover",
		DefectType:       "pb001",
		Status:           "open",
		AffectedVersions: `["4.21"]`,
	})
	if err != nil {
		t.Fatalf("SaveRCAV2: %v", err)
	}

	if _, err := st.LinkSymptomToRCA(&store.SymptomRCA{
		SymptomID:  symID,
		RCAID:      rcaID,
		Confidence: 0.9,
	}); err != nil {
		t.Fatalf("LinkSymptomToRCA: %v", err)
	}

	// C2: new case, same test name, SymptomID == 0 (not yet triaged)
	caseC2 := &store.Case{
		ID:           99,
		Name:         "[T-TSC] PTP sync stability test",
		ErrorMessage: "ptp4l[12345.678]: master offset -999999 s2 freq",
		Status:       "open",
		SymptomID:    0, // not yet triaged
	}

	params := BuildParams(st, caseC2, nil, nil, StepF0Recall, "")

	// The recall prompt must have History populated with C1's symptom and RCA
	if params.History == nil {
		t.Fatal("expected History to be populated for recall with prior symptom, got nil")
	}
	if params.History.SymptomInfo == nil {
		t.Fatal("expected History.SymptomInfo to be populated, got nil")
	}
	if params.History.SymptomInfo.Name != "[T-TSC] PTP sync stability test" {
		t.Errorf("SymptomInfo.Name = %q, want %q",
			params.History.SymptomInfo.Name, "[T-TSC] PTP sync stability test")
	}
	if params.History.SymptomInfo.OccurrenceCount != 1 {
		t.Errorf("SymptomInfo.OccurrenceCount = %d, want 1",
			params.History.SymptomInfo.OccurrenceCount)
	}
	if len(params.History.PriorRCAs) == 0 {
		t.Fatal("expected at least one PriorRCA, got 0")
	}
	if params.History.PriorRCAs[0].Title != "linuxptp-daemon: time jump during switchover" {
		t.Errorf("PriorRCA.Title = %q, want %q",
			params.History.PriorRCAs[0].Title, "linuxptp-daemon: time jump during switchover")
	}
}

// TestBuildParams_RecallWithoutPriorSymptom verifies that when no symptom exists
// for a given test name, History remains nil at F0_RECALL (normal first-discovery).
func TestBuildParams_RecallWithoutPriorSymptom(t *testing.T) {
	st := store.NewMemStore()

	caseData := &store.Case{
		ID:           1,
		Name:         "[T-TSC] Some brand new test",
		ErrorMessage: "timeout waiting for convergence",
		Status:       "open",
		SymptomID:    0,
	}

	params := BuildParams(st, caseData, nil, nil, StepF0Recall, "")

	if params.History != nil && params.History.SymptomInfo != nil {
		t.Errorf("expected History to be nil for first-discovery, got SymptomInfo: %+v",
			params.History.SymptomInfo)
	}
}

// TestBuildParams_RecallPicksBestCandidate verifies that when multiple symptoms
// exist, the candidate matching the test name is chosen.
func TestBuildParams_RecallPicksBestCandidate(t *testing.T) {
	st := store.NewMemStore()

	// Symptom A: different test
	if _, err := st.CreateSymptom(&store.Symptom{
		Name:         "[T-TSC] Other test",
		Fingerprint:  "fp-other",
		ErrorPattern: "some error",
		Component:    "product",
		Status:       "active",
	}); err != nil {
		t.Fatalf("CreateSymptom A: %v", err)
	}

	// Symptom B: matching test
	symBID, err := st.CreateSymptom(&store.Symptom{
		Name:            "[T-TSC] PTP Recovery test",
		Fingerprint:     "fp-ptp-recovery",
		ErrorPattern:    "Expected 0 to equal 1",
		Component:       "product",
		Status:          "active",
		OccurrenceCount: 3,
	})
	if err != nil {
		t.Fatalf("CreateSymptom B: %v", err)
	}

	rcaID, _ := st.SaveRCAV2(&store.RCA{
		Title:      "PTP recovery regression",
		DefectType: "pb001",
		Status:     "open",
	})
	st.LinkSymptomToRCA(&store.SymptomRCA{SymptomID: symBID, RCAID: rcaID, Confidence: 0.85})

	caseData := &store.Case{
		ID:           42,
		Name:         "[T-TSC] PTP Recovery test",
		ErrorMessage: "Expected 0 to equal 1",
		Status:       "open",
		SymptomID:    0,
	}

	params := BuildParams(st, caseData, nil, nil, StepF0Recall, "")

	if params.History == nil || params.History.SymptomInfo == nil {
		t.Fatal("expected History from matching symptom B")
	}
	if params.History.SymptomInfo.Name != "[T-TSC] PTP Recovery test" {
		t.Errorf("picked wrong symptom: %q", params.History.SymptomInfo.Name)
	}
	if params.History.SymptomInfo.OccurrenceCount != 3 {
		t.Errorf("OccurrenceCount = %d, want 3", params.History.SymptomInfo.OccurrenceCount)
	}
	if len(params.History.PriorRCAs) != 1 {
		t.Fatalf("expected 1 PriorRCA, got %d", len(params.History.PriorRCAs))
	}
}

// TestBuildParams_NonRecallStep_NoCandiateSearch verifies that for non-F0 steps,
// the existing behavior is unchanged (only loads History via SymptomID).
func TestBuildParams_NonRecallStep_NoCandiateSearch(t *testing.T) {
	st := store.NewMemStore()

	// Symptom exists but case doesn't have it linked
	st.CreateSymptom(&store.Symptom{
		Name:         "[T-TSC] PTP Recovery test",
		Fingerprint:  "fp-ptp",
		ErrorPattern: "Expected 0 to equal 1",
		Component:    "product",
		Status:       "active",
	})

	caseData := &store.Case{
		ID:           1,
		Name:         "[T-TSC] PTP Recovery test",
		ErrorMessage: "Expected 0 to equal 1",
		Status:       "triaged",
		SymptomID:    0, // not linked
	}

	// F1_TRIAGE should NOT trigger candidate search
	params := BuildParams(st, caseData, nil, nil, StepF1Triage, "")
	if params.History != nil && params.History.SymptomInfo != nil {
		t.Errorf("F1_TRIAGE should not trigger candidate search, got History: %+v",
			params.History.SymptomInfo)
	}
}

// TestBuildParams_RecallDormantReactivation verifies the IsDormantReactivation flag
// is set when a dormant symptom is found as candidate.
func TestBuildParams_RecallDormantReactivation(t *testing.T) {
	st := store.NewMemStore()

	symID, _ := st.CreateSymptom(&store.Symptom{
		Name:         "[T-TSC] PTP sync test",
		Fingerprint:  "fp-dormant",
		ErrorPattern: "timeout",
		Component:    "product",
		Status:       "dormant",
	})

	rcaID, _ := st.SaveRCAV2(&store.RCA{
		Title:      "Old resolved RCA",
		DefectType: "pb001",
		Status:     "resolved",
	})
	st.LinkSymptomToRCA(&store.SymptomRCA{SymptomID: symID, RCAID: rcaID, Confidence: 0.9})

	caseData := &store.Case{
		ID:           10,
		Name:         "[T-TSC] PTP sync test",
		ErrorMessage: "timeout",
		Status:       "open",
		SymptomID:    0,
	}

	params := BuildParams(st, caseData, nil, nil, StepF0Recall, "")

	if params.History == nil || params.History.SymptomInfo == nil {
		t.Fatal("expected History for dormant symptom candidate")
	}
	if !params.History.SymptomInfo.IsDormantReactivation {
		t.Error("expected IsDormantReactivation=true for dormant symptom")
	}
}

func TestBuildParams_WorkspaceLaunchAttributes(t *testing.T) {
	st := store.NewMemStore()
	caseData := &store.Case{ID: 1, Name: "test", Status: "open"}
	env := &rp.Envelope{
		RunID: "100",
		Name:  "launch-100",
		LaunchAttributes: []rp.Attribute{
			{Key: "ocp_version", Value: "4.21"},
			{Key: "operator_version", Value: "4.21.0-202402"},
			{Key: "cluster", Value: "cnf-lab-01", System: true},
		},
	}

	params := BuildParams(st, caseData, env, nil, StepF1Triage, "")

	if params.Workspace == nil {
		t.Fatal("expected Workspace to be populated")
	}
	if params.Workspace.AttrsStatus != Resolved {
		t.Errorf("AttrsStatus = %q, want %q", params.Workspace.AttrsStatus, Resolved)
	}
	if len(params.Workspace.LaunchAttributes) != 3 {
		t.Fatalf("LaunchAttributes len = %d, want 3", len(params.Workspace.LaunchAttributes))
	}
	if params.Workspace.LaunchAttributes[0].Key != "ocp_version" {
		t.Errorf("first attr key = %q, want %q", params.Workspace.LaunchAttributes[0].Key, "ocp_version")
	}
	if params.Workspace.LaunchAttributes[0].Value != "4.21" {
		t.Errorf("first attr value = %q, want %q", params.Workspace.LaunchAttributes[0].Value, "4.21")
	}
}

func TestBuildParams_WorkspaceJiraLinks(t *testing.T) {
	st := store.NewMemStore()
	caseData := &store.Case{ID: 1, Name: "test", Status: "open"}
	env := &rp.Envelope{
		RunID: "100",
		Name:  "launch-100",
		FailureList: []rp.FailureItem{
			{
				ID: 1, Name: "test-1", Status: "FAILED",
				ExternalIssues: []rp.ExternalIssue{
					{TicketID: "OCPBUGS-70233", URL: "https://issues.redhat.com/browse/OCPBUGS-70233"},
				},
			},
			{
				ID: 2, Name: "test-2", Status: "FAILED",
				ExternalIssues: []rp.ExternalIssue{
					{TicketID: "OCPBUGS-70233", URL: "https://issues.redhat.com/browse/OCPBUGS-70233"},
					{TicketID: "OCPBUGS-71000", URL: "https://issues.redhat.com/browse/OCPBUGS-71000"},
				},
			},
		},
	}

	params := BuildParams(st, caseData, env, nil, StepF1Triage, "")

	if params.Workspace == nil {
		t.Fatal("expected Workspace to be populated")
	}
	if params.Workspace.JiraStatus != Resolved {
		t.Errorf("JiraStatus = %q, want %q", params.Workspace.JiraStatus, Resolved)
	}
	if len(params.Workspace.JiraLinks) != 2 {
		t.Fatalf("JiraLinks len = %d, want 2 (deduplicated)", len(params.Workspace.JiraLinks))
	}
}

func TestBuildParams_WorkspaceReposPaths(t *testing.T) {
	st := store.NewMemStore()
	caseData := &store.Case{ID: 1, Name: "test", Status: "open"}
	catalog := &knowledge.KnowledgeSourceCatalog{
		Sources: []knowledge.Source{
			{Name: "ptp-operator", URI: "/home/user/repos/ptp-operator", Purpose: "SUT", Branch: "release-4.21", Kind: knowledge.SourceKindRepo},
			{Name: "cnf-gotests", URI: "/home/user/repos/cnf-gotests", Purpose: "Test framework", Kind: knowledge.SourceKindRepo},
		},
	}

	params := BuildParams(st, caseData, nil, catalog, StepF2Resolve, "")

	if params.Workspace == nil {
		t.Fatal("expected Workspace to be populated")
	}
	if params.Workspace.ReposStatus != Resolved {
		t.Errorf("ReposStatus = %q, want %q", params.Workspace.ReposStatus, Resolved)
	}
	if len(params.Workspace.Repos) != 2 {
		t.Fatalf("Repos len = %d, want 2", len(params.Workspace.Repos))
	}
	if params.Workspace.Repos[0].Path != "/home/user/repos/ptp-operator" {
		t.Errorf("repo URI = %q, want /home/user/repos/ptp-operator", params.Workspace.Repos[0].Path)
	}
}

func TestBuildParams_WorkspaceUnavailable(t *testing.T) {
	st := store.NewMemStore()
	caseData := &store.Case{ID: 1, Name: "test", Status: "open"}

	params := BuildParams(st, caseData, nil, nil, StepF1Triage, "")

	if params.Workspace == nil {
		t.Fatal("expected Workspace to always be populated (with statuses)")
	}
	if params.Workspace.AttrsStatus != Unavailable {
		t.Errorf("AttrsStatus = %q, want %q", params.Workspace.AttrsStatus, Unavailable)
	}
	if params.Workspace.JiraStatus != Unavailable {
		t.Errorf("JiraStatus = %q, want %q", params.Workspace.JiraStatus, Unavailable)
	}
	if params.Workspace.ReposStatus != Unavailable {
		t.Errorf("ReposStatus = %q, want %q", params.Workspace.ReposStatus, Unavailable)
	}
}

func TestBuildParams_WorkspaceFullContext(t *testing.T) {
	st := store.NewMemStore()
	caseData := &store.Case{ID: 1, Name: "test", Status: "open"}
	env := &rp.Envelope{
		RunID: "100",
		Name:  "launch-100",
		LaunchAttributes: []rp.Attribute{
			{Key: "ocp_version", Value: "4.21"},
		},
		FailureList: []rp.FailureItem{
			{
				ID: 1, Name: "test-1", Status: "FAILED",
				ExternalIssues: []rp.ExternalIssue{
					{TicketID: "OCPBUGS-70233", URL: "https://issues.redhat.com/browse/OCPBUGS-70233"},
				},
			},
		},
	}
	catalog := &knowledge.KnowledgeSourceCatalog{
		Sources: []knowledge.Source{
			{Name: "ptp-operator", URI: "/repos/ptp-operator", Purpose: "SUT", Kind: knowledge.SourceKindRepo},
		},
	}

	params := BuildParams(st, caseData, env, catalog, StepF3Invest, "")

	if params.Workspace == nil {
		t.Fatal("expected Workspace")
	}
	if params.Workspace.AttrsStatus != Resolved {
		t.Errorf("AttrsStatus = %q, want %q", params.Workspace.AttrsStatus, Resolved)
	}
	if params.Workspace.JiraStatus != Resolved {
		t.Errorf("JiraStatus = %q, want %q", params.Workspace.JiraStatus, Resolved)
	}
	if params.Workspace.ReposStatus != Resolved {
		t.Errorf("ReposStatus = %q, want %q", params.Workspace.ReposStatus, Resolved)
	}
	if len(params.Workspace.LaunchAttributes) != 1 {
		t.Errorf("LaunchAttributes len = %d, want 1", len(params.Workspace.LaunchAttributes))
	}
	if len(params.Workspace.JiraLinks) != 1 {
		t.Errorf("JiraLinks len = %d, want 1", len(params.Workspace.JiraLinks))
	}
	if len(params.Workspace.Repos) != 1 {
		t.Errorf("Repos len = %d, want 1", len(params.Workspace.Repos))
	}
}

func TestLoadAlwaysReadSources_HappyPath(t *testing.T) {
	dir := t.TempDir()
	docPath := filepath.Join(dir, "architecture.md")
	os.WriteFile(docPath, []byte("# PTP Architecture\nlinuxptp-daemon is a pod."), 0644)

	cat := &knowledge.KnowledgeSourceCatalog{
		Sources: []knowledge.Source{
			{
				Name:       "ptp-architecture",
				Kind:       knowledge.SourceKindDoc,
				Purpose:    "Disambiguation doc",
				ReadPolicy: knowledge.ReadAlways,
				LocalPath:  docPath,
			},
		},
	}

	result := loadAlwaysReadSources(cat)
	if len(result) != 1 {
		t.Fatalf("got %d sources, want 1", len(result))
	}
	if result[0].Name != "ptp-architecture" {
		t.Errorf("Name = %q, want %q", result[0].Name, "ptp-architecture")
	}
	if result[0].Purpose != "Disambiguation doc" {
		t.Errorf("Purpose = %q, want %q", result[0].Purpose, "Disambiguation doc")
	}
	if result[0].Content != "# PTP Architecture\nlinuxptp-daemon is a pod." {
		t.Errorf("Content = %q", result[0].Content)
	}
}

func TestLoadAlwaysReadSources_ConditionalOnly(t *testing.T) {
	cat := &knowledge.KnowledgeSourceCatalog{
		Sources: []knowledge.Source{
			{
				Name:       "repo-a",
				Kind:       knowledge.SourceKindRepo,
				ReadPolicy: knowledge.ReadConditional,
			},
		},
	}

	result := loadAlwaysReadSources(cat)
	if result != nil {
		t.Errorf("expected nil for conditional-only catalog, got %d sources", len(result))
	}
}

func TestLoadAlwaysReadSources_MissingLocalPath(t *testing.T) {
	cat := &knowledge.KnowledgeSourceCatalog{
		Sources: []knowledge.Source{
			{
				Name:       "no-path-doc",
				Kind:       knowledge.SourceKindDoc,
				ReadPolicy: knowledge.ReadAlways,
			},
		},
	}

	result := loadAlwaysReadSources(cat)
	if len(result) != 0 {
		t.Errorf("expected 0 sources for missing LocalPath, got %d", len(result))
	}
}

func TestLoadAlwaysReadSources_NonexistentFile(t *testing.T) {
	cat := &knowledge.KnowledgeSourceCatalog{
		Sources: []knowledge.Source{
			{
				Name:       "ghost-doc",
				Kind:       knowledge.SourceKindDoc,
				ReadPolicy: knowledge.ReadAlways,
				LocalPath:  "/tmp/nonexistent-doc-12345.md",
			},
		},
	}

	result := loadAlwaysReadSources(cat)
	if len(result) != 0 {
		t.Errorf("expected 0 sources for nonexistent file, got %d", len(result))
	}
}

func TestLoadAlwaysReadSources_NilCatalog(t *testing.T) {
	result := loadAlwaysReadSources(nil)
	if result != nil {
		t.Errorf("expected nil for nil catalog, got %d sources", len(result))
	}
}
