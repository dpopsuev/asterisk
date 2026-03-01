package rca

import (
	"context"
	"encoding/json"
	"testing"

	"asterisk/adapters/store"

	framework "github.com/dpopsuev/origami"
)

// fullCircuitAdapter returns deterministic responses for all RCA steps,
// driving the circuit through recall-hit → review-approve → report-done.
type fullCircuitAdapter struct{}

func (a *fullCircuitAdapter) Name() string { return "test-full" }
func (a *fullCircuitAdapter) SendPrompt(_ string, step string, _ string) (json.RawMessage, error) {
	switch CircuitStep(step) {
	case StepF0Recall:
		return json.Marshal(RecallResult{
			Match: true, Confidence: 0.95, Reasoning: "known failure",
		})
	case StepF5Review:
		return json.Marshal(ReviewDecision{Decision: "approve"})
	case StepF6Report:
		return json.Marshal(map[string]any{"summary": "done"})
	default:
		return json.Marshal(map[string]any{})
	}
}

func TestWalkCase_RecallHitPath(t *testing.T) {
	ms := store.NewMemStore()
	c := &store.Case{ID: 1, Name: "test-case"}

	hooks := StoreHooks(ms, c)

	result, err := WalkCase(context.Background(), WalkConfig{
		Store:     ms,
		CaseData:  c,
		Adapter:   &fullCircuitAdapter{},
		CaseLabel: "T1",
		Hooks:     hooks,
	})
	if err != nil {
		t.Fatalf("WalkCase: %v", err)
	}

	if len(result.Path) == 0 {
		t.Fatal("expected non-empty path")
	}
	if result.Path[0] != "recall" {
		t.Errorf("first step = %q, want recall", result.Path[0])
	}

	// recall-hit shortcut goes to review, then report, then DONE
	expectedPath := []string{"recall", "review", "report"}
	if len(result.Path) != len(expectedPath) {
		t.Errorf("path = %v, want %v", result.Path, expectedPath)
	} else {
		for i, step := range expectedPath {
			if result.Path[i] != step {
				t.Errorf("path[%d] = %q, want %q", i, result.Path[i], step)
			}
		}
	}
}

// triageInvestigateAdapter drives: recall-miss → triage → investigate → correlate → review → report
type triageInvestigateAdapter struct{}

func (a *triageInvestigateAdapter) Name() string { return "test-triage" }
func (a *triageInvestigateAdapter) SendPrompt(_ string, step string, _ string) (json.RawMessage, error) {
	switch CircuitStep(step) {
	case StepF0Recall:
		return json.Marshal(RecallResult{Match: false, Confidence: 0.1})
	case StepF1Triage:
		return json.Marshal(TriageResult{
			SymptomCategory: "product_bug",
			CandidateRepos:  []string{"repo-a"},
		})
	case StepF3Invest:
		return json.Marshal(InvestigateArtifact{
			ConvergenceScore: 0.8,
			EvidenceRefs:     []string{"commit-abc"},
			DefectType:       "product_bug",
		})
	case StepF4Correlate:
		return json.Marshal(CorrelateResult{
			IsDuplicate: false, Confidence: 0.3,
		})
	case StepF5Review:
		return json.Marshal(ReviewDecision{Decision: "approve"})
	case StepF6Report:
		return json.Marshal(map[string]any{"summary": "done"})
	default:
		return json.Marshal(map[string]any{})
	}
}

func TestWalkCase_TriageInvestigatePath(t *testing.T) {
	ms := store.NewMemStore()
	c := &store.Case{ID: 2, Name: "test-deep"}
	hooks := StoreHooks(ms, c)

	result, err := WalkCase(context.Background(), WalkConfig{
		Store:     ms,
		CaseData:  c,
		Adapter:   &triageInvestigateAdapter{},
		CaseLabel: "T2",
		Hooks:     hooks,
	})
	if err != nil {
		t.Fatalf("WalkCase: %v", err)
	}

	// recall-miss → triage → single-repo shortcut to investigate → correlate → review → report
	if len(result.Path) < 4 {
		t.Errorf("expected at least 4 steps, got %d: %v", len(result.Path), result.Path)
	}
	if result.Path[0] != "recall" {
		t.Errorf("first step = %q, want recall", result.Path[0])
	}
	if result.Path[1] != "triage" {
		t.Errorf("second step = %q, want triage", result.Path[1])
	}
}

func TestWalkCase_MissingAdapter(t *testing.T) {
	nodes := NodeRegistry()
	th := DefaultThresholds()
	runner, err := BuildRunnerWith(th, nodes)
	if err != nil {
		t.Fatalf("BuildRunnerWith: %v", err)
	}
	_ = runner

	walker := framework.NewProcessWalker("test")

	def, err := AsteriskCircuitDef(th)
	if err != nil {
		t.Fatalf("AsteriskCircuitDef: %v", err)
	}

	err = runner.Walk(context.Background(), walker, def.Start)
	if err == nil {
		t.Fatal("expected error for missing adapter in walker context")
	}
}
