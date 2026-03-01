package rca

import (
	"testing"

	"asterisk/adapters/store"
)

// --- ApplyStoreEffects tests ---

func TestApplyStoreEffects_F0Recall_NoMatch(t *testing.T) {
	st := store.NewMemStore()
	caseData := createTestCase(t, st)

	artifact := &RecallResult{Match: false, Confidence: 0.1}
	err := ApplyStoreEffects(st, caseData, StepF0Recall, artifact)
	if err != nil {
		t.Fatalf("ApplyStoreEffects: %v", err)
	}

	// No links should be created
	updated, err := st.GetCaseV2(caseData.ID)
	if err != nil {
		t.Fatalf("GetCaseV2: %v", err)
	}
	if updated.SymptomID != 0 {
		t.Errorf("expected no symptom link, got %d", updated.SymptomID)
	}
	if updated.RCAID != 0 {
		t.Errorf("expected no RCA link, got %d", updated.RCAID)
	}
}

func TestApplyStoreEffects_F0Recall_Match(t *testing.T) {
	st := store.NewMemStore()
	caseData := createTestCase(t, st)

	// Create a symptom and RCA to link to
	sym := &store.Symptom{Name: "test", Fingerprint: "fp123", Status: "active", OccurrenceCount: 1}
	symID, err := st.CreateSymptom(sym)
	if err != nil {
		t.Fatalf("create symptom: %v", err)
	}

	rca := &store.RCA{Title: "test rca", Status: "open"}
	rcaID, err := st.SaveRCAV2(rca)
	if err != nil {
		t.Fatalf("save rca: %v", err)
	}

	artifact := &RecallResult{Match: true, PriorRCAID: rcaID, SymptomID: symID, Confidence: 0.85}
	err = ApplyStoreEffects(st, caseData, StepF0Recall, artifact)
	if err != nil {
		t.Fatalf("ApplyStoreEffects: %v", err)
	}

	if caseData.SymptomID != symID {
		t.Errorf("case SymptomID = %d, want %d", caseData.SymptomID, symID)
	}
	if caseData.RCAID != rcaID {
		t.Errorf("case RCAID = %d, want %d", caseData.RCAID, rcaID)
	}
}

func TestApplyStoreEffects_F1Triage(t *testing.T) {
	st := store.NewMemStore()
	caseData := createTestCase(t, st)

	artifact := &TriageResult{
		SymptomCategory:      "product",
		Severity:             "high",
		DefectTypeHypothesis: "pb001",
		CandidateRepos:       []string{"linuxptp-daemon"},
	}
	err := ApplyStoreEffects(st, caseData, StepF1Triage, artifact)
	if err != nil {
		t.Fatalf("ApplyStoreEffects: %v", err)
	}

	// Case should be triaged
	if caseData.Status != "triaged" {
		t.Errorf("status = %q, want 'triaged'", caseData.Status)
	}

	// Symptom should be created and linked
	if caseData.SymptomID == 0 {
		t.Error("expected symptom to be created and linked")
	}
}

func TestApplyStoreEffects_F3Investigate(t *testing.T) {
	st := store.NewMemStore()
	caseData := createTestCase(t, st)
	// Pre-set symptom for the symptom->RCA link test
	sym := &store.Symptom{Name: "test", Fingerprint: "fp1", Status: "active", OccurrenceCount: 1}
	symID, _ := st.CreateSymptom(sym)
	caseData.SymptomID = symID

	artifact := &InvestigateArtifact{
		RCAMessage:       "PTP clock offset exceeded threshold",
		DefectType:       "pb001",
		Component:        "linuxptp-daemon",
		ConvergenceScore: 0.85,
		EvidenceRefs:     []string{"src/ptp.c"},
	}
	err := ApplyStoreEffects(st, caseData, StepF3Invest, artifact)
	if err != nil {
		t.Fatalf("ApplyStoreEffects: %v", err)
	}

	// RCA should be created
	if caseData.RCAID == 0 {
		t.Fatal("expected RCA to be created")
	}

	// Case should be investigated
	if caseData.Status != "investigated" {
		t.Errorf("status = %q, want 'investigated'", caseData.Status)
	}

	// RCA fields should be correct
	rca, err := st.GetRCAV2(caseData.RCAID)
	if err != nil {
		t.Fatalf("get rca: %v", err)
	}
	if rca.DefectType != "pb001" {
		t.Errorf("rca defect = %q, want 'pb001'", rca.DefectType)
	}
	if rca.Component != "linuxptp-daemon" {
		t.Errorf("rca component = %q, want 'linuxptp-daemon'", rca.Component)
	}
	if rca.ConvergenceScore != 0.85 {
		t.Errorf("rca convergence = %f, want 0.85", rca.ConvergenceScore)
	}
}

func TestApplyStoreEffects_F4Correlate_Duplicate(t *testing.T) {
	st := store.NewMemStore()
	caseData := createTestCase(t, st)

	// Create a pre-existing RCA
	rca := &store.RCA{Title: "existing rca", Status: "open"}
	rcaID, _ := st.SaveRCAV2(rca)

	artifact := &CorrelateResult{
		IsDuplicate: true,
		LinkedRCAID: rcaID,
		Confidence:  0.90,
	}
	err := ApplyStoreEffects(st, caseData, StepF4Correlate, artifact)
	if err != nil {
		t.Fatalf("ApplyStoreEffects: %v", err)
	}

	if caseData.RCAID != rcaID {
		t.Errorf("case RCAID = %d, want %d", caseData.RCAID, rcaID)
	}
}

func TestApplyStoreEffects_F4Correlate_NotDuplicate(t *testing.T) {
	st := store.NewMemStore()
	caseData := createTestCase(t, st)

	artifact := &CorrelateResult{IsDuplicate: false}
	err := ApplyStoreEffects(st, caseData, StepF4Correlate, artifact)
	if err != nil {
		t.Fatalf("ApplyStoreEffects: %v", err)
	}

	if caseData.RCAID != 0 {
		t.Errorf("expected no RCA link for non-duplicate, got %d", caseData.RCAID)
	}
}

func TestApplyStoreEffects_F5Review_Approve(t *testing.T) {
	st := store.NewMemStore()
	caseData := createTestCase(t, st)

	artifact := &ReviewDecision{Decision: "approve"}
	err := ApplyStoreEffects(st, caseData, StepF5Review, artifact)
	if err != nil {
		t.Fatalf("ApplyStoreEffects: %v", err)
	}

	if caseData.Status != "reviewed" {
		t.Errorf("status = %q, want 'reviewed'", caseData.Status)
	}
}

func TestApplyStoreEffects_F5Review_Overturn(t *testing.T) {
	st := store.NewMemStore()
	caseData := createTestCase(t, st)

	// Create an RCA to overturn
	rca := &store.RCA{Title: "original", Description: "original rca", DefectType: "pb001", Status: "open"}
	rcaID, _ := st.SaveRCAV2(rca)
	caseData.RCAID = rcaID

	artifact := &ReviewDecision{
		Decision: "overturn",
		HumanOverride: &HumanOverride{
			DefectType: "au001",
			RCAMessage: "human corrected: automation issue",
		},
	}
	err := ApplyStoreEffects(st, caseData, StepF5Review, artifact)
	if err != nil {
		t.Fatalf("ApplyStoreEffects: %v", err)
	}

	if caseData.Status != "reviewed" {
		t.Errorf("status = %q, want 'reviewed'", caseData.Status)
	}

	// RCA should be updated with human correction
	updated, err := st.GetRCAV2(rcaID)
	if err != nil {
		t.Fatalf("get rca: %v", err)
	}
	if updated.DefectType != "au001" {
		t.Errorf("rca defect after overturn = %q, want 'au001'", updated.DefectType)
	}
	if updated.Description != "human corrected: automation issue" {
		t.Errorf("rca description = %q", updated.Description)
	}
}

func TestApplyStoreEffects_F2Resolve_NoOp(t *testing.T) {
	st := store.NewMemStore()
	caseData := createTestCase(t, st)

	artifact := &ResolveResult{SelectedRepos: []RepoSelection{{Name: "repo"}}}
	err := ApplyStoreEffects(st, caseData, StepF2Resolve, artifact)
	if err != nil {
		t.Fatalf("ApplyStoreEffects: %v", err)
	}
	// F2 has no store effects, this just verifies no error
}

func TestApplyStoreEffects_NilArtifact(t *testing.T) {
	st := store.NewMemStore()
	caseData := createTestCase(t, st)

	// nil artifact should be safely handled
	err := ApplyStoreEffects(st, caseData, StepF0Recall, nil)
	if err != nil {
		t.Fatalf("expected nil artifact to be safe, got: %v", err)
	}
}

func TestApplyStoreEffects_WrongType(t *testing.T) {
	st := store.NewMemStore()
	caseData := createTestCase(t, st)

	// Wrong artifact type should be silently ignored
	err := ApplyStoreEffects(st, caseData, StepF0Recall, "not a recall result")
	if err != nil {
		t.Fatalf("expected wrong type to be safe, got: %v", err)
	}
}

// --- ComputeFingerprint ---

func TestComputeFingerprint_Effects(t *testing.T) {
	fp1 := ComputeFingerprint("test1", "error1", "comp1")
	fp2 := ComputeFingerprint("test1", "error1", "comp1")
	fp3 := ComputeFingerprint("test2", "error1", "comp1")

	if fp1 != fp2 {
		t.Error("identical inputs should produce identical fingerprints")
	}
	if fp1 == fp3 {
		t.Error("different inputs should produce different fingerprints")
	}
	if len(fp1) != 16 {
		t.Errorf("expected 16-char hex fingerprint, got %d chars", len(fp1))
	}
}

// --- Helpers ---

func createTestCase(t *testing.T, st store.Store) *store.Case {
	t.Helper()
	suite := &store.InvestigationSuite{Name: "test", Status: "active"}
	suiteID, err := st.CreateSuite(suite)
	if err != nil {
		t.Fatalf("create suite: %v", err)
	}
	v := &store.Version{Label: "1.0"}
	vid, err := st.CreateVersion(v)
	if err != nil {
		t.Fatalf("create version: %v", err)
	}
	pipe := &store.Circuit{SuiteID: suiteID, VersionID: vid, Name: "CI", Status: "complete"}
	pipeID, err := st.CreateCircuit(pipe)
	if err != nil {
		t.Fatalf("create circuit: %v", err)
	}
	launch := &store.Launch{CircuitID: pipeID, Name: "Launch", Status: "complete"}
	launchID, err := st.CreateLaunch(launch)
	if err != nil {
		t.Fatalf("create launch: %v", err)
	}
	job := &store.Job{LaunchID: launchID, Name: "Job", Status: "complete"}
	jobID, err := st.CreateJob(job)
	if err != nil {
		t.Fatalf("create job: %v", err)
	}
	c := &store.Case{
		JobID:        jobID,
		LaunchID:     launchID,
		Name:         "test case",
		Status:       "open",
		ErrorMessage: "test error",
	}
	caseID, err := st.CreateCaseV2(c)
	if err != nil {
		t.Fatalf("create case: %v", err)
	}
	c.ID = caseID
	return c
}
