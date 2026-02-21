package calibrate

import (
	"math"
	"testing"
)

func approxEqual(a, b float64) bool {
	return math.Abs(a-b) < 0.001
}

func TestComputeCourtMetrics_Empty(t *testing.T) {
	m := ComputeCourtMetrics(nil, nil)
	if m.CasesActivated != 0 {
		t.Errorf("CasesActivated = %d, want 0", m.CasesActivated)
	}
	if m.VerdictFlipRate != 0 {
		t.Errorf("VerdictFlipRate = %f, want 0", m.VerdictFlipRate)
	}
}

func TestComputeCourtMetrics_NoCourt(t *testing.T) {
	results := []CaseResult{
		{CaseID: "C01", CourtActivated: false},
		{CaseID: "C02", CourtActivated: false},
	}
	m := ComputeCourtMetrics(results, nil)
	if m.CasesActivated != 0 {
		t.Errorf("CasesActivated = %d, want 0", m.CasesActivated)
	}
}

func TestComputeCourtMetrics_MixedVerdicts(t *testing.T) {
	results := []CaseResult{
		{CaseID: "C01", CourtActivated: true, CourtVerdict: "affirm", CourtFlipped: false},
		{CaseID: "C02", CourtActivated: true, CourtVerdict: "amend", CourtFlipped: true, CourtFinalDefect: "automation_bug"},
		{CaseID: "C03", CourtActivated: true, CourtVerdict: "acquit", CourtFlipped: true},
		{CaseID: "C04", CourtActivated: false},
		{CaseID: "C05", CourtActivated: true, CourtVerdict: "mistrial"},
	}
	gt := []GroundTruthCase{
		{ID: "C01", ExpectedVerdict: "affirm"},
		{ID: "C02", ExpectedVerdict: "amend"},
		{ID: "C03", ExpectedVerdict: "affirm"},
		{ID: "C05"},
	}

	m := ComputeCourtMetrics(results, gt)

	if m.CasesActivated != 4 {
		t.Errorf("CasesActivated = %d, want 4", m.CasesActivated)
	}
	if m.AffirmCount != 1 {
		t.Errorf("AffirmCount = %d, want 1", m.AffirmCount)
	}
	if m.AmendCount != 1 {
		t.Errorf("AmendCount = %d, want 1", m.AmendCount)
	}
	if m.AcquitCount != 1 {
		t.Errorf("AcquitCount = %d, want 1", m.AcquitCount)
	}
	if m.MistrialCount != 1 {
		t.Errorf("MistrialCount = %d, want 1", m.MistrialCount)
	}
	// 2 flips out of 4 activated = 0.50
	if !approxEqual(m.VerdictFlipRate, 0.50) {
		t.Errorf("VerdictFlipRate = %f, want 0.50", m.VerdictFlipRate)
	}
	// 2 correct (C01=affirm, C02=amend) out of 3 with expected (C01,C02,C03)
	if !approxEqual(m.VerdictAccuracy, 2.0/3.0) {
		t.Errorf("VerdictAccuracy = %f, want %f", m.VerdictAccuracy, 2.0/3.0)
	}
}

func TestComputeCourtMetrics_RemandEffectiveness(t *testing.T) {
	results := []CaseResult{
		{CaseID: "C01", CourtActivated: true, CourtVerdict: "amend", CourtFlipped: true,
			CourtFinalDefect: "automation_bug", CourtRemands: 1},
		{CaseID: "C02", CourtActivated: true, CourtVerdict: "affirm", CourtFlipped: false,
			CourtRemands: 1},
	}
	gt := []GroundTruthCase{
		{ID: "C01", ExpectedVerdict: "amend"},
		{ID: "C02", ExpectedVerdict: "affirm"},
	}

	m := ComputeCourtMetrics(results, gt)

	if m.RemandCount != 0 {
		t.Errorf("RemandCount = %d, want 0 (amend and affirm, not remand)", m.RemandCount)
	}
	// 1 improved remand (C01 flipped + final defect + matches expected) out of 2 total
	if !approxEqual(m.RemandEffectiveness, 0.50) {
		t.Errorf("RemandEffectiveness = %f, want 0.50", m.RemandEffectiveness)
	}
}

func TestComputeCourtMetrics_AllAffirm(t *testing.T) {
	results := []CaseResult{
		{CaseID: "C01", CourtActivated: true, CourtVerdict: "affirm"},
		{CaseID: "C02", CourtActivated: true, CourtVerdict: "affirm"},
	}
	m := ComputeCourtMetrics(results, nil)
	if m.VerdictFlipRate != 0 {
		t.Errorf("VerdictFlipRate = %f, want 0", m.VerdictFlipRate)
	}
	if m.AffirmCount != 2 {
		t.Errorf("AffirmCount = %d, want 2", m.AffirmCount)
	}
}
