package framework

import "testing"

var (
	_ Artifact = (*Indictment)(nil)
	_ Artifact = (*DefenseBrief)(nil)
	_ Artifact = (*HearingRecord)(nil)
	_ Artifact = (*Verdict)(nil)
)

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
		ConfidenceScore:   0.8,
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
		Challenges:      []EvidenceChallenge{{EvidenceIndex: 0, Challenge: "weak", Severity: "high"}},
		PleaDeal:        false,
		ConfidenceScore: 0.7,
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
		ConfidenceScore:     0.9,
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

func TestCourtEvidenceGap(t *testing.T) {
	gap := CourtEvidenceGap{
		EvidenceGap: EvidenceGap{
			Description:     "missing network metrics during failure window",
			Source:          "infrastructure_telemetry",
			Severity:        GapSeverityHigh,
			SuggestedAction: "collect pod-level network stats from prometheus",
		},
		CourtPhase: "D3",
	}
	if gap.Description == "" {
		t.Error("Description should not be empty")
	}
	if gap.SuggestedAction == "" {
		t.Error("SuggestedAction should not be empty")
	}
	if gap.CourtPhase != "D3" {
		t.Errorf("CourtPhase = %q, want D3", gap.CourtPhase)
	}
}

func TestBuildCourtEdgeFactory_AllKeysPresent(t *testing.T) {
	cfg := DefaultCourtConfig()
	factory := BuildCourtEdgeFactory(cfg)
	expectedKeys := []string{
		"HD1", "HD2", "HD3", "HD4", "HD5", "HD6",
		"HD7", "HD8", "HD9", "HD10", "HD11", "HD12",
	}
	for _, k := range expectedKeys {
		if _, ok := factory[k]; !ok {
			t.Errorf("missing edge factory key %q", k)
		}
	}
	if len(factory) != len(expectedKeys) {
		t.Errorf("factory has %d keys, want %d", len(factory), len(expectedKeys))
	}
}

func TestBuildCourtEdgeFactory_EdgesImplementInterface(t *testing.T) {
	cfg := DefaultCourtConfig()
	factory := BuildCourtEdgeFactory(cfg)
	for id, fn := range factory {
		edge := fn(EdgeDef{ID: id, From: "a", To: "b"})
		if edge.ID() != id {
			t.Errorf("edge %s: ID() = %q, want %q", id, edge.ID(), id)
		}
		if edge.From() != "a" {
			t.Errorf("edge %s: From() = %q, want %q", id, edge.From(), "a")
		}
	}
}

func TestCourtEdge_HD1_FastTrack(t *testing.T) {
	cfg := DefaultCourtConfig()
	factory := BuildCourtEdgeFactory(cfg)
	edge := factory["HD1"](EdgeDef{ID: "HD1", From: "indict", To: "defend"})

	high := &Indictment{ConfidenceScore: 0.96}
	tr := edge.Evaluate(high, &WalkerState{})
	if tr == nil {
		t.Fatal("HD1 should trigger for confidence >= 0.95")
	}
	if tr.NextNode != "defend" {
		t.Errorf("NextNode = %q, want defend", tr.NextNode)
	}

	low := &Indictment{ConfidenceScore: 0.80}
	tr = edge.Evaluate(low, &WalkerState{})
	if tr != nil {
		t.Error("HD1 should not trigger for confidence < 0.95")
	}
}

func TestCourtEdge_HD2_PleaDeal(t *testing.T) {
	cfg := DefaultCourtConfig()
	factory := BuildCourtEdgeFactory(cfg)
	edge := factory["HD2"](EdgeDef{ID: "HD2", From: "defend", To: "verdict"})

	plea := &DefenseBrief{PleaDeal: true, ConfidenceScore: 0.5}
	tr := edge.Evaluate(plea, &WalkerState{})
	if tr == nil {
		t.Fatal("HD2 should trigger on plea deal")
	}

	noPlea := &DefenseBrief{PleaDeal: false, ConfidenceScore: 0.5}
	tr = edge.Evaluate(noPlea, &WalkerState{})
	if tr != nil {
		t.Error("HD2 should not trigger without plea deal")
	}
}

func TestCourtEdge_HD5_HearingComplete(t *testing.T) {
	cfg := DefaultCourtConfig()
	factory := BuildCourtEdgeFactory(cfg)
	edge := factory["HD5"](EdgeDef{ID: "HD5", From: "hearing", To: "verdict"})

	converged := &HearingRecord{Converged: true, MaxRounds: 3, Rounds: []HearingRound{{Round: 1}}}
	tr := edge.Evaluate(converged, &WalkerState{})
	if tr == nil {
		t.Fatal("HD5 should trigger when converged")
	}

	maxRounds := &HearingRecord{Converged: false, MaxRounds: 2, Rounds: []HearingRound{{Round: 1}, {Round: 2}}}
	tr = edge.Evaluate(maxRounds, &WalkerState{})
	if tr == nil {
		t.Fatal("HD5 should trigger when max rounds reached")
	}

	inProgress := &HearingRecord{Converged: false, MaxRounds: 5, Rounds: []HearingRound{{Round: 1}}}
	tr = edge.Evaluate(inProgress, &WalkerState{})
	if tr != nil {
		t.Error("HD5 should not trigger mid-hearing")
	}
}

func TestCourtEdge_HD6_Affirm(t *testing.T) {
	cfg := DefaultCourtConfig()
	factory := BuildCourtEdgeFactory(cfg)
	edge := factory["HD6"](EdgeDef{ID: "HD6", From: "verdict", To: "_done"})

	v := &Verdict{Decision: VerdictAffirm}
	tr := edge.Evaluate(v, &WalkerState{})
	if tr == nil || tr.NextNode != "_done" {
		t.Fatal("HD6 should route affirm to _done")
	}

	v2 := &Verdict{Decision: VerdictAmend}
	tr = edge.Evaluate(v2, &WalkerState{})
	if tr != nil {
		t.Error("HD6 should not trigger for amend")
	}
}

func TestCourtEdge_HD8_Remand_WithLimit(t *testing.T) {
	cfg := CourtConfig{MaxRemands: 2}
	factory := BuildCourtEdgeFactory(cfg)
	edge := factory["HD8"](EdgeDef{ID: "HD8", From: "verdict", To: "indict", Loop: true})

	state := &WalkerState{LoopCounts: map[string]int{"verdict": 0}}
	v := &Verdict{Decision: VerdictRemand}
	tr := edge.Evaluate(v, state)
	if tr == nil {
		t.Fatal("HD8 should allow remand when under limit")
	}
	if tr.NextNode != "indict" {
		t.Errorf("NextNode = %q, want indict", tr.NextNode)
	}

	state.LoopCounts["verdict"] = 2
	tr = edge.Evaluate(v, state)
	if tr != nil {
		t.Error("HD8 should not remand when at limit")
	}
}

func TestCourtEdge_HD9_Acquit(t *testing.T) {
	cfg := DefaultCourtConfig()
	factory := BuildCourtEdgeFactory(cfg)
	edge := factory["HD9"](EdgeDef{ID: "HD9", From: "verdict", To: "_done"})

	v := &Verdict{Decision: VerdictAcquit}
	tr := edge.Evaluate(v, &WalkerState{})
	if tr == nil || tr.NextNode != "_done" {
		t.Fatal("HD9 should route acquit to _done")
	}
}

func TestCourtEdge_HD12_Mistrial(t *testing.T) {
	cfg := DefaultCourtConfig()
	factory := BuildCourtEdgeFactory(cfg)
	edge := factory["HD12"](EdgeDef{ID: "HD12", From: "verdict", To: "_done"})

	v := &Verdict{Decision: VerdictMistrial}
	tr := edge.Evaluate(v, &WalkerState{})
	if tr == nil || tr.NextNode != "_done" {
		t.Fatal("HD12 should route mistrial to _done")
	}
}

func TestCourtEdge_NilArtifact(t *testing.T) {
	cfg := DefaultCourtConfig()
	factory := BuildCourtEdgeFactory(cfg)
	for id, fn := range factory {
		edge := fn(EdgeDef{ID: id})
		tr := edge.Evaluate(nil, &WalkerState{LoopCounts: map[string]int{}})
		if tr != nil {
			t.Errorf("edge %s should return nil for nil artifact", id)
		}
	}
}
