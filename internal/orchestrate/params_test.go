package orchestrate

import (
	"testing"

	"asterisk/internal/store"
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
