package calibrate

import (
	"github.com/dpopsuev/origami"
	"asterisk/internal/orchestrate"
	"encoding/json"
	"strings"
	"testing"
)

type dialecticHearingMockAdapter struct {
	roundsBeforeConverge int
	callCount            int
}

func (a *dialecticHearingMockAdapter) Name() string { return "dialectic-hearing-mock" }
func (a *dialecticHearingMockAdapter) SendPrompt(caseID string, step orchestrate.PipelineStep, prompt string) (json.RawMessage, error) {
	a.callCount++
	resp := dialecticRoundResponse{
		ThesisArgument:     "the evidence is strong",
		AntithesisRebuttal: "the evidence is circumstantial",
		ArbiterNotes:       "weighing arguments",
		Converged:          a.callCount >= a.roundsBeforeConverge,
	}
	b, _ := json.Marshal(resp)
	return b, nil
}

func TestDialecticLoop_ConvergesEarly(t *testing.T) {
	adapter := &dialecticHearingMockAdapter{roundsBeforeConverge: 2}
	thesis := &framework.ThesisChallenge{ChargedDefectType: "product_bug", ConfidenceScore: 0.80}
	antithesis := &framework.AntithesisResponse{AlternativeHypothesis: "automation_bug"}

	record, err := DialecticLoop(adapter, "C01", thesis, antithesis, 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !record.Converged {
		t.Error("expected dialectic to converge")
	}
	if len(record.Rounds) != 2 {
		t.Errorf("expected 2 rounds, got %d", len(record.Rounds))
	}
	if adapter.callCount != 2 {
		t.Errorf("adapter called %d times, want 2", adapter.callCount)
	}
}

func TestDialecticLoop_HitsMaxRounds(t *testing.T) {
	adapter := &dialecticHearingMockAdapter{roundsBeforeConverge: 100}

	record, err := DialecticLoop(adapter, "C02", nil, nil, 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if record.Converged {
		t.Error("should not converge when max rounds < convergence threshold")
	}
	if len(record.Rounds) != 3 {
		t.Errorf("expected 3 rounds, got %d", len(record.Rounds))
	}
}

func TestDialecticLoop_RoundNumbering(t *testing.T) {
	adapter := &dialecticHearingMockAdapter{roundsBeforeConverge: 3}

	record, err := DialecticLoop(adapter, "C03", nil, nil, 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for i, r := range record.Rounds {
		if r.Round != i+1 {
			t.Errorf("round[%d].Round = %d, want %d", i, r.Round, i+1)
		}
	}
}

func TestBuildNegationInjection_NilSynthesis(t *testing.T) {
	inj := BuildNegationInjection("C01", nil, nil, "product_bug")
	if inj != nil {
		t.Error("expected nil for nil synthesis")
	}
}

func TestBuildNegationInjection_NotRemand(t *testing.T) {
	s := &framework.Synthesis{Decision: framework.SynthesisAffirm}
	inj := BuildNegationInjection("C01", s, nil, "product_bug")
	if inj != nil {
		t.Error("expected nil for non-remand synthesis")
	}
}

func TestBuildNegationInjection_Remand(t *testing.T) {
	s := &framework.Synthesis{
		Decision: framework.SynthesisRemand,
		NegationFeedback: &framework.NegationFeedback{
			ChallengedEvidence: []int{0, 2},
			SpecificQuestions:  []string{"Was the pod restarted during the test?"},
		},
	}
	antithesis := &framework.AntithesisResponse{AlternativeHypothesis: "infrastructure_issue"}
	inj := BuildNegationInjection("C05", s, antithesis, "product_bug")

	if inj == nil {
		t.Fatal("expected non-nil injection")
	}
	if inj.CaseID != "C05" {
		t.Errorf("CaseID = %q, want C05", inj.CaseID)
	}
	if inj.OriginalDefect != "product_bug" {
		t.Errorf("OriginalDefect = %q, want product_bug", inj.OriginalDefect)
	}
	if inj.AlternativeHypothesis != "infrastructure_issue" {
		t.Errorf("AlternativeHypothesis = %q, want infrastructure_issue", inj.AlternativeHypothesis)
	}
	if len(inj.ChallengedEvidenceIdx) != 2 {
		t.Errorf("ChallengedEvidenceIdx = %d items, want 2", len(inj.ChallengedEvidenceIdx))
	}
	if len(inj.SpecificQuestions) != 1 {
		t.Errorf("SpecificQuestions = %d items, want 1", len(inj.SpecificQuestions))
	}
}

func TestInjectNegationContext_Nil(t *testing.T) {
	result := InjectNegationContext("base prompt", nil)
	if result != "base prompt" {
		t.Error("nil injection should return base prompt unchanged")
	}
}

func TestInjectNegationContext_Full(t *testing.T) {
	inj := &NegationFeedbackInjection{
		CaseID:                "C05",
		OriginalDefect:        "product_bug",
		AlternativeHypothesis: "infrastructure_issue",
		ChallengedEvidenceIdx: []int{0, 2},
		SpecificQuestions:     []string{"Was the pod restarted?"},
	}
	result := InjectNegationContext("Investigate the following case:", inj)

	if !strings.Contains(result, "DIALECTIC REMAND FEEDBACK") {
		t.Error("missing remand header")
	}
	if !strings.Contains(result, "C05") {
		t.Error("missing case ID")
	}
	if !strings.Contains(result, "product_bug") {
		t.Error("missing original defect")
	}
	if !strings.Contains(result, "infrastructure_issue") {
		t.Error("missing alternative hypothesis")
	}
	if !strings.Contains(result, "Evidence item #0") {
		t.Error("missing challenged evidence index")
	}
	if !strings.Contains(result, "Was the pod restarted?") {
		t.Error("missing specific question")
	}
	if !strings.Contains(result, "END DIALECTIC FEEDBACK") {
		t.Error("missing footer")
	}
}
