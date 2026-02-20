package framework

import "testing"

func TestDefaultCourtConfig(t *testing.T) {
	cfg := DefaultCourtConfig()
	if cfg.Enabled {
		t.Error("default court should be disabled")
	}
	if cfg.MaxRemands != 2 {
		t.Errorf("MaxRemands = %d, want 2", cfg.MaxRemands)
	}
	if cfg.MaxHandoffs != 6 {
		t.Errorf("MaxHandoffs = %d, want 6", cfg.MaxHandoffs)
	}
	if cfg.ActivationThreshold != 0.85 {
		t.Errorf("ActivationThreshold = %f, want 0.85", cfg.ActivationThreshold)
	}
}

func TestCourtConfig_ShouldActivate(t *testing.T) {
	cfg := CourtConfig{Enabled: true, ActivationThreshold: 0.85}

	cases := []struct {
		confidence float64
		want       bool
	}{
		{0.90, false},
		{0.85, false},
		{0.84, true},
		{0.65, true},
		{0.50, true},
		{0.49, false},
		{0.30, false},
		{1.00, false},
	}
	for _, tc := range cases {
		got := cfg.ShouldActivate(tc.confidence)
		if got != tc.want {
			t.Errorf("ShouldActivate(%f) = %v, want %v", tc.confidence, got, tc.want)
		}
	}
}

func TestCourtConfig_ShouldActivate_Disabled(t *testing.T) {
	cfg := CourtConfig{Enabled: false, ActivationThreshold: 0.85}
	if cfg.ShouldActivate(0.65) {
		t.Error("disabled court should never activate")
	}
}

func TestIndictment_ArtifactInterface(t *testing.T) {
	ind := &Indictment{
		ChargedDefectType: "product_bug",
		Confidence:        0.8,
		Evidence:          []EvidenceItem{{Description: "test", Source: "log", Weight: 0.9}},
	}
	if ind.Type() != "indictment" {
		t.Errorf("Type() = %q, want %q", ind.Type(), "indictment")
	}
	if ind.Raw() != ind {
		t.Error("Raw() should return self")
	}
}

func TestDefenseBrief_ArtifactInterface(t *testing.T) {
	brief := &DefenseBrief{
		Challenges: []EvidenceChallenge{{EvidenceIndex: 0, Challenge: "weak", Severity: "high"}},
		PleaDeal:   false,
		Confidence: 0.7,
	}
	if brief.Type() != "defense_brief" {
		t.Errorf("Type() = %q, want %q", brief.Type(), "defense_brief")
	}
}

func TestHearingRecord_ArtifactInterface(t *testing.T) {
	record := &HearingRecord{
		Rounds:    []HearingRound{{Round: 1, ProsecutionArgument: "p", DefenseRebuttal: "d", JudgeNotes: "j"}},
		MaxRounds: 3,
		Converged: false,
	}
	if record.Type() != "hearing_record" {
		t.Errorf("Type() = %q, want %q", record.Type(), "hearing_record")
	}
}

func TestVerdict_ArtifactInterface(t *testing.T) {
	v := &Verdict{
		Decision:            VerdictAffirm,
		FinalClassification: "product_bug",
		Confidence:          0.9,
		Reasoning:           "confirmed",
	}
	if v.Type() != "verdict" {
		t.Errorf("Type() = %q, want %q", v.Type(), "verdict")
	}
}

func TestVerdict_Remand(t *testing.T) {
	v := &Verdict{
		Decision: VerdictRemand,
		RemandFeedback: &RemandFeedback{
			ChallengedEvidence: []int{0, 2},
			AlternativeHyp:     "could be flaky",
			SpecificQuestions:   []string{"Was network stable?"},
		},
	}
	if v.Decision != VerdictRemand {
		t.Errorf("Decision = %q, want remand", v.Decision)
	}
	if v.RemandFeedback == nil {
		t.Fatal("RemandFeedback should not be nil for remand")
	}
	if len(v.RemandFeedback.ChallengedEvidence) != 2 {
		t.Errorf("ChallengedEvidence count = %d, want 2", len(v.RemandFeedback.ChallengedEvidence))
	}
}

func TestVerdictDecision_Constants(t *testing.T) {
	decisions := []VerdictDecision{VerdictAffirm, VerdictAmend, VerdictAcquit, VerdictRemand, VerdictMistrial}
	if len(decisions) != 5 {
		t.Errorf("expected 5 verdict decisions, got %d", len(decisions))
	}
	seen := make(map[VerdictDecision]bool)
	for _, d := range decisions {
		if seen[d] {
			t.Errorf("duplicate decision: %s", d)
		}
		seen[d] = true
	}
}
