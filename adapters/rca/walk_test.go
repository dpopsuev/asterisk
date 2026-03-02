package rca

import (
	"context"
	"testing"

	"asterisk/adapters/store"

	framework "github.com/dpopsuev/origami"
)

// fullCircuitTransformer returns deterministic typed results for all steps,
// driving the circuit through recall-hit → review-approve → report-done.
type fullCircuitTransformer struct{}

func (f *fullCircuitTransformer) Name() string { return "test-full" }
func (f *fullCircuitTransformer) Transform(_ context.Context, tc *framework.TransformerContext) (any, error) {
	step := NodeNameToStep(tc.NodeName)
	switch step {
	case StepF0Recall:
		return &RecallResult{Match: true, Confidence: 0.95, Reasoning: "known failure"}, nil
	case StepF5Review:
		return &ReviewDecision{Decision: "approve"}, nil
	case StepF6Report:
		return map[string]any{"summary": "done"}, nil
	default:
		return map[string]any{}, nil
	}
}

func TestWalkCase_RecallHitPath(t *testing.T) {
	ms := store.NewMemStore()
	c := &store.Case{ID: 1, Name: "test-case"}

	storeAdapter := &framework.Adapter{
		Namespace: "store", Name: "test-store",
		Hooks: StoreHooks(ms, c),
	}
	transAdapter := TransformerAdapter(&fullCircuitTransformer{})

	result, err := WalkCase(context.Background(), WalkConfig{
		Store:    ms,
		CaseData: c,
		CaseLabel: "T1",
		Adapters: []*framework.Adapter{transAdapter, storeAdapter},
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

	if result.State == nil {
		t.Fatal("expected non-nil State in WalkResult")
	}
}

// triageInvestigateTransformer drives: recall-miss → triage → investigate → correlate → review → report
type triageInvestigateTransformer struct{}

func (f *triageInvestigateTransformer) Name() string { return "test-triage" }
func (f *triageInvestigateTransformer) Transform(_ context.Context, tc *framework.TransformerContext) (any, error) {
	step := NodeNameToStep(tc.NodeName)
	switch step {
	case StepF0Recall:
		return &RecallResult{Match: false, Confidence: 0.1}, nil
	case StepF1Triage:
		return &TriageResult{SymptomCategory: "product_bug", CandidateRepos: []string{"repo-a"}}, nil
	case StepF3Invest:
		return &InvestigateArtifact{ConvergenceScore: 0.8, EvidenceRefs: []string{"commit-abc"}, DefectType: "product_bug"}, nil
	case StepF4Correlate:
		return &CorrelateResult{IsDuplicate: false, Confidence: 0.3}, nil
	case StepF5Review:
		return &ReviewDecision{Decision: "approve"}, nil
	case StepF6Report:
		return map[string]any{"summary": "done"}, nil
	default:
		return map[string]any{}, nil
	}
}

func TestWalkCase_TriageInvestigatePath(t *testing.T) {
	ms := store.NewMemStore()
	c := &store.Case{ID: 2, Name: "test-deep"}

	storeAdapter := &framework.Adapter{
		Namespace: "store", Name: "test-store",
		Hooks: StoreHooks(ms, c),
	}
	transAdapter := TransformerAdapter(&triageInvestigateTransformer{})

	result, err := WalkCase(context.Background(), WalkConfig{
		Store:    ms,
		CaseData: c,
		CaseLabel: "T2",
		Adapters: []*framework.Adapter{transAdapter, storeAdapter},
	})
	if err != nil {
		t.Fatalf("WalkCase: %v", err)
	}

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

func TestWalkCase_HITL_Fallback(t *testing.T) {
	hitlAdapter := HITLAdapter()
	th := DefaultThresholds()
	runner, err := BuildRunner(th, hitlAdapter)
	if err != nil {
		t.Fatalf("BuildRunner: %v", err)
	}

	walker := framework.NewProcessWalker("test")

	def, err := AsteriskCircuitDef(th)
	if err != nil {
		t.Fatalf("AsteriskCircuitDef: %v", err)
	}

	err = runner.Walk(context.Background(), walker, def.Start)
	if err == nil {
		t.Fatal("expected error for HITL fallback (no prompt dir)")
	}
}
