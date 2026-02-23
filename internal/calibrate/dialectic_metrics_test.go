package calibrate

import (
	"math"
	"testing"
)

func approxEqual(a, b float64) bool {
	return math.Abs(a-b) < 0.001
}

func TestComputeDialecticMetrics_Empty(t *testing.T) {
	m := ComputeDialecticMetrics(nil, nil)
	if m.CasesActivated != 0 {
		t.Errorf("CasesActivated = %d, want 0", m.CasesActivated)
	}
	if m.SynthesisFlipRate != 0 {
		t.Errorf("SynthesisFlipRate = %f, want 0", m.SynthesisFlipRate)
	}
}

func TestComputeDialecticMetrics_NoDialectic(t *testing.T) {
	results := []CaseResult{
		{CaseID: "C01", DialecticActivated: false},
		{CaseID: "C02", DialecticActivated: false},
	}
	m := ComputeDialecticMetrics(results, nil)
	if m.CasesActivated != 0 {
		t.Errorf("CasesActivated = %d, want 0", m.CasesActivated)
	}
}

func TestComputeDialecticMetrics_MixedSyntheses(t *testing.T) {
	results := []CaseResult{
		{CaseID: "C01", DialecticActivated: true, DialecticSynthesis: "affirm", DialecticFlipped: false},
		{CaseID: "C02", DialecticActivated: true, DialecticSynthesis: "amend", DialecticFlipped: true, DialecticFinalDefect: "automation_bug"},
		{CaseID: "C03", DialecticActivated: true, DialecticSynthesis: "acquit", DialecticFlipped: true},
		{CaseID: "C04", DialecticActivated: false},
		{CaseID: "C05", DialecticActivated: true, DialecticSynthesis: "unresolved"},
	}
	gt := []GroundTruthCase{
		{ID: "C01", ExpectedSynthesis: "affirm"},
		{ID: "C02", ExpectedSynthesis: "amend"},
		{ID: "C03", ExpectedSynthesis: "affirm"},
		{ID: "C05"},
	}

	m := ComputeDialecticMetrics(results, gt)

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
	if m.UnresolvedCount != 1 {
		t.Errorf("UnresolvedCount = %d, want 1", m.UnresolvedCount)
	}
	if !approxEqual(m.SynthesisFlipRate, 0.50) {
		t.Errorf("SynthesisFlipRate = %f, want 0.50", m.SynthesisFlipRate)
	}
	if !approxEqual(m.SynthesisAccuracy, 2.0/3.0) {
		t.Errorf("SynthesisAccuracy = %f, want %f", m.SynthesisAccuracy, 2.0/3.0)
	}
}

func TestComputeDialecticMetrics_NegationEffectiveness(t *testing.T) {
	results := []CaseResult{
		{CaseID: "C01", DialecticActivated: true, DialecticSynthesis: "amend", DialecticFlipped: true,
			DialecticFinalDefect: "automation_bug", DialecticNegations: 1},
		{CaseID: "C02", DialecticActivated: true, DialecticSynthesis: "affirm", DialecticFlipped: false,
			DialecticNegations: 1},
	}
	gt := []GroundTruthCase{
		{ID: "C01", ExpectedSynthesis: "amend"},
		{ID: "C02", ExpectedSynthesis: "affirm"},
	}

	m := ComputeDialecticMetrics(results, gt)

	if m.NegationCount != 0 {
		t.Errorf("NegationCount = %d, want 0 (amend and affirm, not remand)", m.NegationCount)
	}
	if !approxEqual(m.NegationEffectiveness, 0.50) {
		t.Errorf("NegationEffectiveness = %f, want 0.50", m.NegationEffectiveness)
	}
}

func TestComputeDialecticMetrics_AllAffirm(t *testing.T) {
	results := []CaseResult{
		{CaseID: "C01", DialecticActivated: true, DialecticSynthesis: "affirm"},
		{CaseID: "C02", DialecticActivated: true, DialecticSynthesis: "affirm"},
	}
	m := ComputeDialecticMetrics(results, nil)
	if m.SynthesisFlipRate != 0 {
		t.Errorf("SynthesisFlipRate = %f, want 0", m.SynthesisFlipRate)
	}
	if m.AffirmCount != 2 {
		t.Errorf("AffirmCount = %d, want 2", m.AffirmCount)
	}
}
