package adapt

import (
	"fmt"
	"testing"

	"asterisk/adapters/rca"
	"github.com/dpopsuev/origami/dispatch"
	"github.com/dpopsuev/origami"
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
	scenario := &rca.Scenario{Name: "test"}
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

func TestLLMAdapter_Identify_ValidResponse(t *testing.T) {
	mock := &mockProbeDispatcher{
		response: []byte(`{"model_name":"claude-4-sonnet","provider":"Anthropic","version":"20250514","wrapper":"Cursor"}`),
	}
	adapter := NewLLMAdapter("", WithDispatcher(mock))

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
	if mi.Version != "20250514" {
		t.Errorf("Version = %q, want %q", mi.Version, "20250514")
	}
	if mi.Wrapper != "Cursor" {
		t.Errorf("Wrapper = %q, want %q", mi.Wrapper, "Cursor")
	}
	if mi.Raw == "" {
		t.Error("Raw should contain the original response")
	}
}

func TestLLMAdapter_Identify_NoVersion(t *testing.T) {
	mock := &mockProbeDispatcher{
		response: []byte(`{"model_name":"claude-4-sonnet","provider":"Anthropic"}`),
	}
	adapter := NewLLMAdapter("", WithDispatcher(mock))

	mi, err := adapter.Identify()
	if err != nil {
		t.Fatal(err)
	}
	if mi.Version != "" {
		t.Errorf("Version = %q, want empty (version is optional)", mi.Version)
	}
}

func TestLLMAdapter_Identify_WrapperAsModelName(t *testing.T) {
	mock := &mockProbeDispatcher{
		response: []byte(`{"model_name":"composer","provider":"Cursor"}`),
	}
	adapter := NewLLMAdapter("", WithDispatcher(mock))

	_, err := adapter.Identify()
	if err == nil {
		t.Fatal("expected error when model_name is a known wrapper")
	}
	t.Logf("correctly rejected wrapper identity: %v", err)
}

func TestLLMAdapter_Identify_GarbageResponse(t *testing.T) {
	mock := &mockProbeDispatcher{
		response: []byte(`I am Claude, made by Anthropic!`),
	}
	adapter := NewLLMAdapter("", WithDispatcher(mock))

	_, err := adapter.Identify()
	if err == nil {
		t.Fatal("expected error for garbage response")
	}
}

func TestLLMAdapter_Identify_EmptyModelName(t *testing.T) {
	mock := &mockProbeDispatcher{
		response: []byte(`{"model_name":"","provider":"Anthropic"}`),
	}
	adapter := NewLLMAdapter("", WithDispatcher(mock))

	_, err := adapter.Identify()
	if err == nil {
		t.Fatal("expected error for empty model_name")
	}
}

func TestLLMAdapter_Identify_EmptyProvider(t *testing.T) {
	mock := &mockProbeDispatcher{
		response: []byte(`{"model_name":"claude","provider":""}`),
	}
	adapter := NewLLMAdapter("", WithDispatcher(mock))

	_, err := adapter.Identify()
	if err == nil {
		t.Fatal("expected error for empty provider")
	}
}

func TestLLMAdapter_Identify_DispatchError(t *testing.T) {
	mock := &mockProbeDispatcher{
		err: fmt.Errorf("connection refused"),
	}
	adapter := NewLLMAdapter("", WithDispatcher(mock))

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

func TestParseModelIdentity_WithVersion(t *testing.T) {
	mi, err := ParseModelIdentity([]byte(`{"model_name":"gpt-4o","provider":"OpenAI","version":"2024-08-06"}`))
	if err != nil {
		t.Fatal(err)
	}
	if mi.Version != "2024-08-06" {
		t.Errorf("Version = %q, want %q", mi.Version, "2024-08-06")
	}
	want := "gpt-4o@2024-08-06/OpenAI"
	if got := mi.String(); got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}

func TestParseModelIdentity_WithWrapper(t *testing.T) {
	mi, err := ParseModelIdentity([]byte(`{"model_name":"claude-sonnet-4","provider":"Anthropic","version":"20250514","wrapper":"Cursor"}`))
	if err != nil {
		t.Fatal(err)
	}
	if mi.Wrapper != "Cursor" {
		t.Errorf("Wrapper = %q, want %q", mi.Wrapper, "Cursor")
	}
	want := "claude-sonnet-4@20250514/Anthropic (via Cursor)"
	if got := mi.String(); got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}

func TestParseModelIdentity_RejectsWrapperAsModel(t *testing.T) {
	wrappers := []string{"composer", "copilot", "cursor", "azure"}
	for _, w := range wrappers {
		data := []byte(fmt.Sprintf(`{"model_name":%q,"provider":"SomeProvider"}`, w))
		mi, err := ParseModelIdentity(data)
		if err == nil {
			t.Errorf("ParseModelIdentity accepted wrapper %q as model_name: %+v", w, mi)
		}
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
	// Compile-time check that all adapters satisfy rca.Identifiable.
	var _ interface {
		Identify() (framework.ModelIdentity, error)
	} = (*StubAdapter)(nil)

	var _ interface {
		Identify() (framework.ModelIdentity, error)
	} = (*BasicAdapter)(nil)

	var _ interface {
		Identify() (framework.ModelIdentity, error)
	} = (*LLMAdapter)(nil)
}

func TestModelIdentity_Conciseness(t *testing.T) {
	identities := []framework.ModelIdentity{
		{ModelName: "stub", Provider: "asterisk"},
		{ModelName: "basic-heuristic", Provider: "asterisk"},
		{ModelName: "claude-4-sonnet", Provider: "Anthropic"},
		{ModelName: "claude-4-sonnet", Provider: "Anthropic", Version: "20250514"},
		{ModelName: "gpt-4o", Provider: "OpenAI", Version: "2024-08-06"},
		{ModelName: "claude-sonnet-4", Provider: "Anthropic", Version: "20250514", Wrapper: "Cursor"},
	}
	for _, mi := range identities {
		s := mi.String()
		if len(s) > 60 {
			t.Errorf("String() too long (%d): %q", len(s), s)
		}
		tag := mi.Tag()
		if len(tag) > 24 {
			t.Errorf("Tag() too long (%d): %q", len(tag), tag)
		}
	}
}
