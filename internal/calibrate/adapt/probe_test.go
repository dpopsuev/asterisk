package adapt

import (
	"fmt"
	"testing"

	"asterisk/internal/calibrate"
	"asterisk/internal/calibrate/dispatch"
	"asterisk/internal/framework"
)

// mockProbeDispatcher returns a fixed response for any dispatch.
type mockProbeDispatcher struct {
	response []byte
	err      error
}

func (d *mockProbeDispatcher) Dispatch(_ dispatch.DispatchContext) ([]byte, error) {
	return d.response, d.err
}

func TestStubAdapter_Identify_Determinism(t *testing.T) {
	scenario := &calibrate.Scenario{Name: "test"}
	adapter := NewStubAdapter(scenario)

	first, err := adapter.Identify()
	if err != nil {
		t.Fatal(err)
	}
	if first.ModelName == "" || first.Provider == "" {
		t.Fatalf("identity fields must not be empty: %+v", first)
	}

	for i := 0; i < 100; i++ {
		got, err := adapter.Identify()
		if err != nil {
			t.Fatalf("iteration %d: %v", i, err)
		}
		if got != first {
			t.Fatalf("iteration %d: got %+v, want %+v", i, got, first)
		}
	}
}

func TestBasicAdapter_Identify_Determinism(t *testing.T) {
	adapter := NewBasicAdapter(nil, nil)

	first, err := adapter.Identify()
	if err != nil {
		t.Fatal(err)
	}
	if first.ModelName == "" || first.Provider == "" {
		t.Fatalf("identity fields must not be empty: %+v", first)
	}

	for i := 0; i < 100; i++ {
		got, err := adapter.Identify()
		if err != nil {
			t.Fatalf("iteration %d: %v", i, err)
		}
		if got != first {
			t.Fatalf("iteration %d: got %+v, want %+v", i, got, first)
		}
	}
}

func TestCursorAdapter_Identify_ValidResponse(t *testing.T) {
	mock := &mockProbeDispatcher{
		response: []byte(`{"model_name":"claude-4-sonnet","provider":"Anthropic"}`),
	}
	adapter := NewCursorAdapter("", WithDispatcher(mock))

	mi, err := adapter.Identify()
	if err != nil {
		t.Fatal(err)
	}
	if mi.ModelName != "claude-4-sonnet" {
		t.Errorf("ModelName = %q, want %q", mi.ModelName, "claude-4-sonnet")
	}
	if mi.Provider != "Anthropic" {
		t.Errorf("Provider = %q, want %q", mi.Provider, "Anthropic")
	}
	if mi.Raw == "" {
		t.Error("Raw should contain the original response")
	}
}

func TestCursorAdapter_Identify_GarbageResponse(t *testing.T) {
	mock := &mockProbeDispatcher{
		response: []byte(`I am Claude, made by Anthropic!`),
	}
	adapter := NewCursorAdapter("", WithDispatcher(mock))

	_, err := adapter.Identify()
	if err == nil {
		t.Fatal("expected error for garbage response")
	}
}

func TestCursorAdapter_Identify_EmptyModelName(t *testing.T) {
	mock := &mockProbeDispatcher{
		response: []byte(`{"model_name":"","provider":"Anthropic"}`),
	}
	adapter := NewCursorAdapter("", WithDispatcher(mock))

	_, err := adapter.Identify()
	if err == nil {
		t.Fatal("expected error for empty model_name")
	}
}

func TestCursorAdapter_Identify_EmptyProvider(t *testing.T) {
	mock := &mockProbeDispatcher{
		response: []byte(`{"model_name":"claude","provider":""}`),
	}
	adapter := NewCursorAdapter("", WithDispatcher(mock))

	_, err := adapter.Identify()
	if err == nil {
		t.Fatal("expected error for empty provider")
	}
}

func TestCursorAdapter_Identify_DispatchError(t *testing.T) {
	mock := &mockProbeDispatcher{
		err: fmt.Errorf("connection refused"),
	}
	adapter := NewCursorAdapter("", WithDispatcher(mock))

	_, err := adapter.Identify()
	if err == nil {
		t.Fatal("expected error when dispatcher fails")
	}
}

func TestParseModelIdentity_Valid(t *testing.T) {
	mi, err := ParseModelIdentity([]byte(`{"model_name":"gpt-4o","provider":"OpenAI"}`))
	if err != nil {
		t.Fatal(err)
	}
	if mi.ModelName != "gpt-4o" || mi.Provider != "OpenAI" {
		t.Errorf("unexpected identity: %+v", mi)
	}
}

func TestParseModelIdentity_WithWhitespace(t *testing.T) {
	mi, err := ParseModelIdentity([]byte(`  {"model_name":"gpt-4o","provider":"OpenAI"}  `))
	if err != nil {
		t.Fatal(err)
	}
	if mi.ModelName != "gpt-4o" {
		t.Errorf("ModelName = %q, want %q", mi.ModelName, "gpt-4o")
	}
}

func TestParseModelIdentity_Invalid(t *testing.T) {
	_, err := ParseModelIdentity([]byte(`not json`))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestAdapters_ImplementIdentifiable(t *testing.T) {
	// Compile-time check that all adapters satisfy calibrate.Identifiable.
	var _ interface {
		Identify() (framework.ModelIdentity, error)
	} = (*StubAdapter)(nil)

	var _ interface {
		Identify() (framework.ModelIdentity, error)
	} = (*BasicAdapter)(nil)

	var _ interface {
		Identify() (framework.ModelIdentity, error)
	} = (*CursorAdapter)(nil)
}

func TestModelIdentity_Conciseness(t *testing.T) {
	identities := []framework.ModelIdentity{
		{ModelName: "stub", Provider: "asterisk"},
		{ModelName: "basic-heuristic", Provider: "asterisk"},
		{ModelName: "claude-4-sonnet", Provider: "Anthropic"},
		{ModelName: "gpt-4o", Provider: "OpenAI"},
	}
	for _, mi := range identities {
		s := mi.String()
		if len(s) > 40 {
			t.Errorf("String() too long (%d): %q", len(s), s)
		}
		tag := mi.Tag()
		if len(tag) > 24 {
			t.Errorf("Tag() too long (%d): %q", len(tag), tag)
		}
	}
}
