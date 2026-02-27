package rca

import (
	"context"
	"encoding/json"
	"testing"

	"asterisk/internal/orchestrate"

	framework "github.com/dpopsuev/origami"
)

type stubAdapter struct {
	responses map[string]json.RawMessage
}

func (s *stubAdapter) Name() string { return "stub" }
func (s *stubAdapter) SendPrompt(_ string, step string, _ string) (json.RawMessage, error) {
	return s.responses[step], nil
}

func TestRCANode_Process_Recall(t *testing.T) {
	reg := NodeRegistry()
	factory := reg["recall"]
	node := factory(framework.NodeDef{Name: "recall"})

	adapter := &stubAdapter{
		responses: map[string]json.RawMessage{
			"F0_RECALL": json.RawMessage(`{"match":true,"confidence":0.9,"reasoning":"match"}`),
		},
	}

	ws := framework.NewWalkerState("test")
	ws.Context[KeyAdapter] = adapter
	ws.Context[KeyCaseLabel] = "C1"

	nc := framework.NodeContext{
		WalkerState: ws,
	}

	art, err := node.Process(context.Background(), nc)
	if err != nil {
		t.Fatalf("Process: %v", err)
	}
	if art.Type() != string(orchestrate.StepF0Recall) {
		t.Errorf("Type = %q, want %q", art.Type(), orchestrate.StepF0Recall)
	}

	result, ok := art.Raw().(*orchestrate.RecallResult)
	if !ok {
		t.Fatalf("Raw type = %T, want *RecallResult", art.Raw())
	}
	if !result.Match || result.Confidence != 0.9 {
		t.Errorf("RecallResult = %+v", result)
	}
}

func TestRCANode_Process_Triage(t *testing.T) {
	reg := NodeRegistry()
	factory := reg["triage"]
	node := factory(framework.NodeDef{Name: "triage"})

	adapter := &stubAdapter{
		responses: map[string]json.RawMessage{
			"F1_TRIAGE": json.RawMessage(`{"symptom_category":"product_bug","candidate_repos":["repo-a"]}`),
		},
	}

	ws := framework.NewWalkerState("test")
	ws.Context[KeyAdapter] = adapter

	nc := framework.NodeContext{WalkerState: ws}
	art, err := node.Process(context.Background(), nc)
	if err != nil {
		t.Fatalf("Process: %v", err)
	}

	result, ok := art.Raw().(*orchestrate.TriageResult)
	if !ok {
		t.Fatalf("Raw type = %T, want *TriageResult", art.Raw())
	}
	if result.SymptomCategory != "product_bug" {
		t.Errorf("SymptomCategory = %q", result.SymptomCategory)
	}
}

func TestRCANode_Process_MissingAdapter(t *testing.T) {
	reg := NodeRegistry()
	factory := reg["recall"]
	node := factory(framework.NodeDef{Name: "recall"})

	ws := framework.NewWalkerState("test")
	nc := framework.NodeContext{WalkerState: ws}

	_, err := node.Process(context.Background(), nc)
	if err == nil {
		t.Fatal("expected error for missing adapter")
	}
}

func TestRCANode_AllSteps(t *testing.T) {
	reg := NodeRegistry()
	expected := []string{"recall", "triage", "resolve", "investigate", "correlate", "review", "report"}
	for _, name := range expected {
		if _, ok := reg[name]; !ok {
			t.Errorf("missing node factory for %q", name)
		}
	}
}

func TestNodeRegistry_FactoryProducesCorrectName(t *testing.T) {
	reg := NodeRegistry()
	for name, factory := range reg {
		node := factory(framework.NodeDef{Name: name})
		if node.Name() != name {
			t.Errorf("factory %q produced node named %q", name, node.Name())
		}
	}
}

func TestMarbleRegistry_AllSteps(t *testing.T) {
	reg := MarbleRegistry()
	expected := []string{"rca.recall", "rca.triage", "rca.resolve", "rca.investigate", "rca.correlate", "rca.review", "rca.report"}
	for _, name := range expected {
		if _, ok := reg[name]; !ok {
			t.Errorf("missing marble for %q", name)
		}
	}
}

func TestMarbleRegistry_Atomic(t *testing.T) {
	reg := MarbleRegistry()
	for name, factory := range reg {
		marble := factory(framework.NodeDef{Name: name})
		if marble.IsComposite() {
			t.Errorf("marble %q should be atomic", name)
		}
		if marble.PipelineDef() != nil {
			t.Errorf("marble %q should have nil PipelineDef", name)
		}
	}
}
