package calibrate

import (
	"github.com/dpopsuev/origami"
	"asterisk/internal/orchestrate"
	"encoding/json"
	"testing"
)

type dialecticMockAdapter struct {
	responses map[orchestrate.PipelineStep]json.RawMessage
	calls     []orchestrate.PipelineStep
}

func (a *dialecticMockAdapter) Name() string { return "dialectic-mock" }
func (a *dialecticMockAdapter) SendPrompt(caseID string, step orchestrate.PipelineStep, prompt string) (json.RawMessage, error) {
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

func TestRunDialectic_Disabled(t *testing.T) {
	cfg := RunConfig{DialecticConfig: framework.DialecticConfig{Enabled: false}}
	result := RunDialectic(cfg, "C01", 0.60, "product_bug", nil)
	if result.Activated {
		t.Error("dialectic should not activate when disabled")
	}
}

func TestRunDialectic_HighConfidence_NoActivation(t *testing.T) {
	cfg := RunConfig{DialecticConfig: framework.DialecticConfig{Enabled: true, ContradictionThreshold: 0.85}}
	result := RunDialectic(cfg, "C01", 0.90, "product_bug", nil)
	if result.Activated {
		t.Error("dialectic should not activate when confidence is high")
	}
}

func TestRunDialectic_Affirm(t *testing.T) {
	adapter := &dialecticMockAdapter{
		responses: map[orchestrate.PipelineStep]json.RawMessage{
			orchestrate.StepD0Indict: mustJSON(framework.ThesisChallenge{
				ChargedDefectType: "product_bug",
				ConfidenceScore:   0.96,
			}),
			orchestrate.StepD2Defend: mustJSON(framework.AntithesisResponse{
				Concession:      true,
				ConfidenceScore: 0.3,
			}),
			orchestrate.StepD4Verdict: mustJSON(framework.Synthesis{
				Decision:            framework.SynthesisAffirm,
				FinalClassification: "product_bug",
				ConfidenceScore:     0.95,
			}),
		},
	}

	cfg := RunConfig{DialecticConfig: framework.DialecticConfig{
		Enabled:                true,
		ContradictionThreshold: 0.85,
		MaxTurns:               10,
		MaxNegations:           2,
	}}

	result := RunDialectic(cfg, "C01", 0.70, "product_bug", adapter)
	if !result.Activated {
		t.Fatal("dialectic should activate")
	}
	if result.SynthesisDecision != framework.SynthesisAffirm {
		t.Errorf("SynthesisDecision = %q, want affirm", result.SynthesisDecision)
	}
	if result.Flipped {
		t.Error("should not flip when affirming same defect type")
	}
}

func TestRunDialectic_Amend(t *testing.T) {
	adapter := &dialecticMockAdapter{
		responses: map[orchestrate.PipelineStep]json.RawMessage{
			orchestrate.StepD0Indict: mustJSON(framework.ThesisChallenge{
				ChargedDefectType: "product_bug",
				ConfidenceScore:   0.80,
			}),
			orchestrate.StepD2Defend: mustJSON(framework.AntithesisResponse{
				AlternativeHypothesis: "automation_bug",
				Challenges:            []framework.EvidenceChallenge{{EvidenceIndex: 0, Challenge: "weak", Severity: "high"}},
				ConfidenceScore:       0.6,
			}),
			orchestrate.StepD3Hearing: mustJSON(framework.DialecticRecord{
				Converged: true,
				MaxRounds: 3,
				Rounds:    []framework.DialecticRound{{Round: 1}},
			}),
			orchestrate.StepD4Verdict: mustJSON(framework.Synthesis{
				Decision:            framework.SynthesisAmend,
				FinalClassification: "automation_bug",
				ConfidenceScore:     0.85,
			}),
		},
	}

	cfg := RunConfig{DialecticConfig: framework.DialecticConfig{
		Enabled:                true,
		ContradictionThreshold: 0.85,
		MaxTurns:               10,
		MaxNegations:           2,
	}}

	result := RunDialectic(cfg, "C02", 0.65, "product_bug", adapter)
	if !result.Activated {
		t.Fatal("dialectic should activate")
	}
	if result.SynthesisDecision != framework.SynthesisAmend {
		t.Errorf("SynthesisDecision = %q, want amend", result.SynthesisDecision)
	}
	if !result.Flipped {
		t.Error("should flip when amending to different defect type")
	}
	if result.FinalDefect != "automation_bug" {
		t.Errorf("FinalDefect = %q, want automation_bug", result.FinalDefect)
	}
}

func TestRunDialectic_MaxTurns(t *testing.T) {
	adapter := &dialecticMockAdapter{
		responses: map[orchestrate.PipelineStep]json.RawMessage{},
	}

	cfg := RunConfig{DialecticConfig: framework.DialecticConfig{
		Enabled:                true,
		ContradictionThreshold: 0.85,
		MaxTurns:               3,
		MaxNegations:           2,
	}}

	result := RunDialectic(cfg, "C03", 0.60, "product_bug", adapter)
	if !result.Activated {
		t.Fatal("dialectic should activate")
	}
	if result.Rounds > 3 {
		t.Errorf("Rounds = %d, should not exceed MaxTurns=3", result.Rounds)
	}
}

func TestStepToDialecticNode(t *testing.T) {
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
		got := stepToDialecticNode(tc.step)
		if got != tc.want {
			t.Errorf("stepToDialecticNode(%s) = %q, want %q", tc.step, got, tc.want)
		}
	}
}

func TestDialecticNodeToStep(t *testing.T) {
	cases := []struct {
		node string
		want orchestrate.PipelineStep
	}{
		{"indict", orchestrate.StepD0Indict},
		{"defend", orchestrate.StepD2Defend},
		{"_done", orchestrate.StepDialecticDone},
		{"unknown", orchestrate.StepDialecticDone},
	}
	for _, tc := range cases {
		got := dialecticNodeToStep(tc.node)
		if got != tc.want {
			t.Errorf("dialecticNodeToStep(%q) = %q, want %q", tc.node, got, tc.want)
		}
	}
}

func TestIsDialecticStep(t *testing.T) {
	dialecticSteps := []orchestrate.PipelineStep{
		orchestrate.StepD0Indict, orchestrate.StepD1Discover,
		orchestrate.StepD2Defend, orchestrate.StepD3Hearing, orchestrate.StepD4Verdict,
	}
	for _, s := range dialecticSteps {
		if !s.IsDialecticStep() {
			t.Errorf("%s.IsDialecticStep() = false, want true", s)
		}
	}

	lightSteps := []orchestrate.PipelineStep{
		orchestrate.StepF0Recall, orchestrate.StepF1Triage, orchestrate.StepDone,
	}
	for _, s := range lightSteps {
		if s.IsDialecticStep() {
			t.Errorf("%s.IsDialecticStep() = true, want false", s)
		}
	}
}
