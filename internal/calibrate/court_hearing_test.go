package calibrate

import (
	"asterisk/pkg/framework"
	"asterisk/internal/orchestrate"
	"encoding/json"
	"strings"
	"testing"
)

type hearingMockAdapter struct {
	roundsBeforeConverge int
	callCount            int
}

func (a *hearingMockAdapter) Name() string { return "hearing-mock" }
func (a *hearingMockAdapter) SendPrompt(caseID string, step orchestrate.PipelineStep, prompt string) (json.RawMessage, error) {
	a.callCount++
	resp := hearingRoundResponse{
		ProsecutionArgument: "the evidence is strong",
		DefenseRebuttal:     "the evidence is circumstantial",
		JudgeNotes:          "weighing arguments",
		Converged:           a.callCount >= a.roundsBeforeConverge,
	}
	b, _ := json.Marshal(resp)
	return b, nil
}

func TestHearingLoop_ConvergesEarly(t *testing.T) {
	adapter := &hearingMockAdapter{roundsBeforeConverge: 2}
	ind := &framework.Indictment{ChargedDefectType: "product_bug", ConfidenceScore: 0.80}
	def := &framework.DefenseBrief{AlternativeHypothesis: "automation_bug"}

	record, err := HearingLoop(adapter, "C01", ind, def, 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !record.Converged {
		t.Error("expected hearing to converge")
	}
	if len(record.Rounds) != 2 {
		t.Errorf("expected 2 rounds, got %d", len(record.Rounds))
	}
	if adapter.callCount != 2 {
		t.Errorf("adapter called %d times, want 2", adapter.callCount)
	}
}

func TestHearingLoop_HitsMaxRounds(t *testing.T) {
	adapter := &hearingMockAdapter{roundsBeforeConverge: 100}

	record, err := HearingLoop(adapter, "C02", nil, nil, 3)
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

func TestHearingLoop_RoundNumbering(t *testing.T) {
	adapter := &hearingMockAdapter{roundsBeforeConverge: 3}

	record, err := HearingLoop(adapter, "C03", nil, nil, 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for i, r := range record.Rounds {
		if r.Round != i+1 {
			t.Errorf("round[%d].Round = %d, want %d", i, r.Round, i+1)
		}
	}
}

func TestBuildRemandInjection_NilVerdict(t *testing.T) {
	inj := BuildRemandInjection("C01", nil, nil, "product_bug")
	if inj != nil {
		t.Error("expected nil for nil verdict")
	}
}

func TestBuildRemandInjection_NotRemand(t *testing.T) {
	v := &framework.Verdict{Decision: framework.VerdictAffirm}
	inj := BuildRemandInjection("C01", v, nil, "product_bug")
	if inj != nil {
		t.Error("expected nil for non-remand verdict")
	}
}

func TestBuildRemandInjection_Remand(t *testing.T) {
	v := &framework.Verdict{
		Decision: framework.VerdictRemand,
		RemandFeedback: &framework.RemandFeedback{
			ChallengedEvidence: []int{0, 2},
			SpecificQuestions:  []string{"Was the pod restarted during the test?"},
		},
	}
	def := &framework.DefenseBrief{AlternativeHypothesis: "infrastructure_issue"}
	inj := BuildRemandInjection("C05", v, def, "product_bug")

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

func TestInjectRemandContext_Nil(t *testing.T) {
	result := InjectRemandContext("base prompt", nil)
	if result != "base prompt" {
		t.Error("nil injection should return base prompt unchanged")
	}
}

func TestInjectRemandContext_Full(t *testing.T) {
	inj := &RemandFeedbackInjection{
		CaseID:                "C05",
		OriginalDefect:        "product_bug",
		AlternativeHypothesis: "infrastructure_issue",
		ChallengedEvidenceIdx: []int{0, 2},
		SpecificQuestions:     []string{"Was the pod restarted?"},
	}
	result := InjectRemandContext("Investigate the following case:", inj)

	if !strings.Contains(result, "COURT REMAND FEEDBACK") {
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
	if !strings.Contains(result, "END COURT FEEDBACK") {
		t.Error("missing footer")
	}
}
