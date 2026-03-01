package rca

import (
	"testing"

	"asterisk/adapters/store"
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
	storeHooks := []string{"store.recall", "store.triage", "store.investigate", "store.correlate", "store.review"}
	for _, name := range storeHooks {
		if _, ok := a.Hooks[name]; !ok {
			t.Errorf("missing hook %q", name)
		}
	}
	expectedTotal := 8 + len(storeHooks)
	if len(a.Hooks) != expectedTotal {
		t.Errorf("expected %d total hooks, got %d", expectedTotal, len(a.Hooks))
	}
}

func TestAdapter_Hooks_NilStore(t *testing.T) {
	a := Adapter(AdapterConfig{})
	injectCount := 8
	if len(a.Hooks) != injectCount {
		t.Errorf("expected %d inject hooks with nil store, got %d", injectCount, len(a.Hooks))
	}
	for _, name := range []string{"inject.envelope", "inject.failure", "inject.workspace", "inject.sources", "inject.prior", "inject.taxonomy", "inject.history", "inject.recall-digest"} {
		if _, ok := a.Hooks[name]; !ok {
			t.Errorf("missing inject hook %q", name)
		}
	}
}
