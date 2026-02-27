package rca

import (
	"context"
	"testing"

	"asterisk/adapters/rp"
	"asterisk/adapters/store"

	framework "github.com/dpopsuev/origami"
)

func TestAdapter_NamespaceAndProvides(t *testing.T) {
	a := Adapter(AdapterConfig{})
	if a.Namespace != "rca" {
		t.Errorf("Namespace = %q, want rca", a.Namespace)
	}
	if a.Name != "asterisk-rca" {
		t.Errorf("Name = %q, want asterisk-rca", a.Name)
	}
}

func TestAdapter_Transformers(t *testing.T) {
	a := Adapter(AdapterConfig{
		PromptDir: ".cursor/prompts",
	})
	for _, name := range []string{"context-builder", "prompt-filler"} {
		if _, ok := a.Transformers[name]; !ok {
			t.Errorf("missing transformer %q", name)
		}
	}
}

func TestAdapter_Extractors(t *testing.T) {
	a := Adapter(AdapterConfig{})
	expected := []string{"recall", "triage", "resolve", "investigate", "correlate", "review", "report"}
	for _, name := range expected {
		if _, ok := a.Extractors[name]; !ok {
			t.Errorf("missing extractor %q", name)
		}
	}
}

func TestAdapter_Hooks_WithStore(t *testing.T) {
	ms := store.NewMemStore()
	c := &store.Case{ID: 1}
	a := Adapter(AdapterConfig{Store: ms, CaseData: c})
	expected := []string{"store.recall", "store.triage", "store.investigate", "store.correlate", "store.review"}
	for _, name := range expected {
		if _, ok := a.Hooks[name]; !ok {
			t.Errorf("missing hook %q", name)
		}
	}
}

func TestAdapter_Hooks_NilStore(t *testing.T) {
	a := Adapter(AdapterConfig{})
	if len(a.Hooks) != 0 {
		t.Errorf("expected 0 hooks with nil store, got %d", len(a.Hooks))
	}
}

func TestContextBuilder_Name(t *testing.T) {
	cb := NewContextBuilder(nil, nil, nil, nil, "")
	if cb.Name() != "context-builder" {
		t.Errorf("Name() = %q, want context-builder", cb.Name())
	}
}

func TestPromptFiller_Name(t *testing.T) {
	pf := NewPromptFiller("")
	if pf.Name() != "prompt-filler" {
		t.Errorf("Name() = %q, want prompt-filler", pf.Name())
	}
}

func TestContextBuilder_KnownStep(t *testing.T) {
	ms := store.NewMemStore()
	c := &store.Case{ID: 1, Name: "test-case"}
	env := &rp.Envelope{RunID: "123", Name: "test-launch"}
	cb := NewContextBuilder(ms, c, env, nil, "")

	tc := &framework.TransformerContext{
		NodeName: "recall",
		Meta:     map[string]any{"step": "recall"},
	}
	result, err := cb.Transform(context.Background(), tc)
	if err != nil {
		t.Fatalf("Transform: %v", err)
	}
	params, ok := result.(*TemplateParams)
	if !ok {
		t.Fatalf("result type = %T, want *TemplateParams", result)
	}
	if params.CaseID != 1 {
		t.Errorf("CaseID = %d, want 1", params.CaseID)
	}
	if params.LaunchID != "123" {
		t.Errorf("LaunchID = %q, want 123", params.LaunchID)
	}
}

func TestContextBuilder_UnknownStep(t *testing.T) {
	ms := store.NewMemStore()
	c := &store.Case{ID: 1, Name: "test"}
	env := &rp.Envelope{RunID: "123"}
	cb := NewContextBuilder(ms, c, env, nil, "")

	tc := &framework.TransformerContext{
		NodeName: "unknown_step_xyz",
		Meta:     map[string]any{},
	}
	_, err := cb.Transform(context.Background(), tc)
	if err == nil {
		t.Fatal("expected error for unknown step")
	}
}
