package calibrate

import (
	"asterisk/internal/framework"
	"asterisk/internal/orchestrate"
	"encoding/json"
	"testing"
)

type courtMockAdapter struct {
	responses map[orchestrate.PipelineStep]json.RawMessage
	calls     []orchestrate.PipelineStep
}

func (a *courtMockAdapter) Name() string { return "court-mock" }
func (a *courtMockAdapter) SendPrompt(caseID string, step orchestrate.PipelineStep, prompt string) (json.RawMessage, error) {
	a.calls = append(a.calls, step)
	if r, ok := a.responses[step]; ok {
		return r, nil
	}
	return json.RawMessage(`{}`), nil
}

func mustJSON(v any) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}

func TestRunCourt_Disabled(t *testing.T) {
	cfg := RunConfig{CourtConfig: framework.CourtConfig{Enabled: false}}
	result := RunCourt(cfg, "C01", 0.60, "product_bug", nil)
	if result.Activated {
		t.Error("court should not activate when disabled")
	}
}

func TestRunCourt_HighConfidence_NoActivation(t *testing.T) {
	cfg := RunConfig{CourtConfig: framework.CourtConfig{Enabled: true, ActivationThreshold: 0.85}}
	result := RunCourt(cfg, "C01", 0.90, "product_bug", nil)
	if result.Activated {
		t.Error("court should not activate when confidence is high")
	}
}

func TestRunCourt_Affirm(t *testing.T) {
	adapter := &courtMockAdapter{
		responses: map[orchestrate.PipelineStep]json.RawMessage{
			orchestrate.StepD0Indict: mustJSON(framework.Indictment{
				ChargedDefectType: "product_bug",
				ConfidenceScore:   0.96,
			}),
			orchestrate.StepD2Defend: mustJSON(framework.DefenseBrief{
				PleaDeal:        true,
				ConfidenceScore: 0.3,
			}),
			orchestrate.StepD4Verdict: mustJSON(framework.Verdict{
				Decision:            framework.VerdictAffirm,
				FinalClassification: "product_bug",
				ConfidenceScore:     0.95,
			}),
		},
	}

	cfg := RunConfig{CourtConfig: framework.CourtConfig{
		Enabled:             true,
		ActivationThreshold: 0.85,
		MaxHandoffs:         10,
		MaxRemands:          2,
	}}

	result := RunCourt(cfg, "C01", 0.70, "product_bug", adapter)
	if !result.Activated {
		t.Fatal("court should activate")
	}
	if result.VerdictDecision != framework.VerdictAffirm {
		t.Errorf("VerdictDecision = %q, want affirm", result.VerdictDecision)
	}
	if result.Flipped {
		t.Error("should not flip when affirming same defect type")
	}
}

func TestRunCourt_Amend(t *testing.T) {
	adapter := &courtMockAdapter{
		responses: map[orchestrate.PipelineStep]json.RawMessage{
			orchestrate.StepD0Indict: mustJSON(framework.Indictment{
				ChargedDefectType: "product_bug",
				ConfidenceScore:   0.80,
			}),
			orchestrate.StepD2Defend: mustJSON(framework.DefenseBrief{
				AlternativeHypothesis: "automation_bug",
				Challenges:            []framework.EvidenceChallenge{{EvidenceIndex: 0, Challenge: "weak", Severity: "high"}},
				ConfidenceScore:       0.6,
			}),
			orchestrate.StepD3Hearing: mustJSON(framework.HearingRecord{
				Converged: true,
				MaxRounds: 3,
				Rounds:    []framework.HearingRound{{Round: 1}},
			}),
			orchestrate.StepD4Verdict: mustJSON(framework.Verdict{
				Decision:            framework.VerdictAmend,
				FinalClassification: "automation_bug",
				ConfidenceScore:     0.85,
			}),
		},
	}

	cfg := RunConfig{CourtConfig: framework.CourtConfig{
		Enabled:             true,
		ActivationThreshold: 0.85,
		MaxHandoffs:         10,
		MaxRemands:          2,
	}}

	result := RunCourt(cfg, "C02", 0.65, "product_bug", adapter)
	if !result.Activated {
		t.Fatal("court should activate")
	}
	if result.VerdictDecision != framework.VerdictAmend {
		t.Errorf("VerdictDecision = %q, want amend", result.VerdictDecision)
	}
	if !result.Flipped {
		t.Error("should flip when amending to different defect type")
	}
	if result.FinalDefect != "automation_bug" {
		t.Errorf("FinalDefect = %q, want automation_bug", result.FinalDefect)
	}
}

func TestRunCourt_MaxHandoffs(t *testing.T) {
	adapter := &courtMockAdapter{
		responses: map[orchestrate.PipelineStep]json.RawMessage{},
	}

	cfg := RunConfig{CourtConfig: framework.CourtConfig{
		Enabled:             true,
		ActivationThreshold: 0.85,
		MaxHandoffs:         3,
		MaxRemands:          2,
	}}

	result := RunCourt(cfg, "C03", 0.60, "product_bug", adapter)
	if !result.Activated {
		t.Fatal("court should activate")
	}
	if result.Rounds > 3 {
		t.Errorf("Rounds = %d, should not exceed MaxHandoffs=3", result.Rounds)
	}
}

func TestStepToCourtNode(t *testing.T) {
	cases := []struct {
		step orchestrate.PipelineStep
		want string
	}{
		{orchestrate.StepD0Indict, "indict"},
		{orchestrate.StepD1Discover, "discover"},
		{orchestrate.StepD2Defend, "defend"},
		{orchestrate.StepD3Hearing, "hearing"},
		{orchestrate.StepD4Verdict, "verdict"},
		{orchestrate.StepF0Recall, ""},
	}
	for _, tc := range cases {
		got := stepToCourtNode(tc.step)
		if got != tc.want {
			t.Errorf("stepToCourtNode(%s) = %q, want %q", tc.step, got, tc.want)
		}
	}
}

func TestCourtNodeToStep(t *testing.T) {
	cases := []struct {
		node string
		want orchestrate.PipelineStep
	}{
		{"indict", orchestrate.StepD0Indict},
		{"defend", orchestrate.StepD2Defend},
		{"_done", orchestrate.StepCourtDone},
		{"unknown", orchestrate.StepCourtDone},
	}
	for _, tc := range cases {
		got := courtNodeToStep(tc.node)
		if got != tc.want {
			t.Errorf("courtNodeToStep(%q) = %q, want %q", tc.node, got, tc.want)
		}
	}
}

func TestIsCourtStep(t *testing.T) {
	courtSteps := []orchestrate.PipelineStep{
		orchestrate.StepD0Indict, orchestrate.StepD1Discover,
		orchestrate.StepD2Defend, orchestrate.StepD3Hearing, orchestrate.StepD4Verdict,
	}
	for _, s := range courtSteps {
		if !s.IsCourtStep() {
			t.Errorf("%s.IsCourtStep() = false, want true", s)
		}
	}

	lightSteps := []orchestrate.PipelineStep{
		orchestrate.StepF0Recall, orchestrate.StepF1Triage, orchestrate.StepDone,
	}
	for _, s := range lightSteps {
		if s.IsCourtStep() {
			t.Errorf("%s.IsCourtStep() = true, want false", s)
		}
	}
}
