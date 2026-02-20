package framework

import "testing"

func TestModelIdentity_String_Determinism(t *testing.T) {
	mi := ModelIdentity{ModelName: "claude-4-sonnet", Provider: "Anthropic"}
	first := mi.String()
	for i := 0; i < 100; i++ {
		if got := mi.String(); got != first {
			t.Fatalf("iteration %d: String() = %q, want %q", i, got, first)
		}
	}
}

func TestModelIdentity_Tag_Determinism(t *testing.T) {
	mi := ModelIdentity{ModelName: "gpt-4o", Provider: "OpenAI"}
	first := mi.Tag()
	for i := 0; i < 100; i++ {
		if got := mi.Tag(); got != first {
			t.Fatalf("iteration %d: Tag() = %q, want %q", i, got, first)
		}
	}
}

func TestModelIdentity_String_Conciseness(t *testing.T) {
	cases := []ModelIdentity{
		{ModelName: "claude-4-sonnet", Provider: "Anthropic"},
		{ModelName: "gpt-4o", Provider: "OpenAI"},
		{ModelName: "stub", Provider: "asterisk"},
		{ModelName: "basic-heuristic", Provider: "asterisk"},
	}
	for _, mi := range cases {
		s := mi.String()
		if len(s) > 40 {
			t.Errorf("String() too long (%d chars): %q", len(s), s)
		}
		if len(s) == 0 {
			t.Error("String() is empty")
		}
	}
}

func TestModelIdentity_Tag_Conciseness(t *testing.T) {
	cases := []ModelIdentity{
		{ModelName: "claude-4-sonnet", Provider: "Anthropic"},
		{ModelName: "a-very-long-model-name-that-exceeds-twenty-chars", Provider: "Corp"},
	}
	for _, mi := range cases {
		tag := mi.Tag()
		if len(tag) > 24 {
			t.Errorf("Tag() too long (%d chars): %q", len(tag), tag)
		}
		if len(tag) < 3 {
			t.Errorf("Tag() too short (%d chars): %q", len(tag), tag)
		}
	}
}

func TestModelIdentity_NonEmpty(t *testing.T) {
	mi := ModelIdentity{ModelName: "test-model", Provider: "test-corp"}
	if mi.String() == "" {
		t.Error("String() should not be empty for populated identity")
	}
	if mi.Tag() == "" {
		t.Error("Tag() should not be empty for populated identity")
	}
}

func TestModelIdentity_ZeroValue(t *testing.T) {
	var mi ModelIdentity
	s := mi.String()
	if s == "" {
		t.Error("String() should not be empty for zero-value identity")
	}
	if s != "unknown/unknown" {
		t.Errorf("String() = %q, want %q", s, "unknown/unknown")
	}
	tag := mi.Tag()
	if tag != "[unknown]" {
		t.Errorf("Tag() = %q, want %q", tag, "[unknown]")
	}
}

func TestModelIdentity_String_Format(t *testing.T) {
	mi := ModelIdentity{ModelName: "claude-4-sonnet", Provider: "Anthropic"}
	want := "claude-4-sonnet/Anthropic"
	if got := mi.String(); got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}

func TestModelIdentity_Tag_Format(t *testing.T) {
	mi := ModelIdentity{ModelName: "claude-4-sonnet", Provider: "Anthropic"}
	want := "[claude-4-sonnet]"
	if got := mi.Tag(); got != want {
		t.Errorf("Tag() = %q, want %q", got, want)
	}
}

func TestModelIdentity_Tag_Truncation(t *testing.T) {
	mi := ModelIdentity{ModelName: "a-really-long-model-name-that-exceeds-limit"}
	tag := mi.Tag()
	if len(tag) > 24 {
		t.Errorf("Tag() should truncate to 24 chars max, got %d: %q", len(tag), tag)
	}
	if tag[0] != '[' || tag[len(tag)-1] != ']' {
		t.Errorf("Tag() should be bracket-wrapped, got %q", tag)
	}
}
