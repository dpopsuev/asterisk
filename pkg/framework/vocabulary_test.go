package framework

import "testing"

func TestMapVocabulary_ZeroConfig(t *testing.T) {
	v := NewMapVocabulary()
	if got := v.Name("F0"); got != "F0" {
		t.Errorf("empty vocabulary: Name(F0) = %q, want F0", got)
	}
}

func TestMapVocabulary_Register(t *testing.T) {
	v := NewMapVocabulary().
		Register("F0", "Recall").
		Register("F1", "Triage")

	tests := []struct {
		code, want string
	}{
		{"F0", "Recall"},
		{"F1", "Triage"},
		{"F2", "F2"},
	}
	for _, tt := range tests {
		if got := v.Name(tt.code); got != tt.want {
			t.Errorf("Name(%q) = %q, want %q", tt.code, got, tt.want)
		}
	}
}

func TestMapVocabulary_RegisterAll(t *testing.T) {
	v := NewMapVocabulary().RegisterAll(map[string]string{
		"pb001": "Product Bug",
		"ab001": "Automation Bug",
	})
	if got := v.Name("pb001"); got != "Product Bug" {
		t.Errorf("Name(pb001) = %q, want Product Bug", got)
	}
}

func TestNameWithCode(t *testing.T) {
	v := NewMapVocabulary().Register("F0", "Recall")

	if got := NameWithCode(v, "F0"); got != "Recall (F0)" {
		t.Errorf("NameWithCode(F0) = %q, want %q", got, "Recall (F0)")
	}
	if got := NameWithCode(v, "F9"); got != "F9" {
		t.Errorf("NameWithCode(F9) = %q, want F9 (unknown passthrough)", got)
	}
}

func TestVocabularyFunc(t *testing.T) {
	upper := VocabularyFunc(func(code string) string {
		if code == "x" {
			return "X-RAY"
		}
		return code
	})
	if got := upper.Name("x"); got != "X-RAY" {
		t.Errorf("VocabularyFunc(x) = %q, want X-RAY", got)
	}
	if got := upper.Name("y"); got != "y" {
		t.Errorf("VocabularyFunc(y) = %q, want y (passthrough)", got)
	}
}

func TestChainVocabulary(t *testing.T) {
	stages := NewMapVocabulary().Register("F0", "Recall")
	defects := NewMapVocabulary().Register("pb001", "Product Bug")

	chain := ChainVocabulary{stages, defects}

	tests := []struct {
		code, want string
	}{
		{"F0", "Recall"},
		{"pb001", "Product Bug"},
		{"unknown", "unknown"},
	}
	for _, tt := range tests {
		if got := chain.Name(tt.code); got != tt.want {
			t.Errorf("ChainVocabulary.Name(%q) = %q, want %q", tt.code, got, tt.want)
		}
	}
}

func TestChainVocabulary_FirstWins(t *testing.T) {
	first := NewMapVocabulary().Register("X", "First")
	second := NewMapVocabulary().Register("X", "Second")

	chain := ChainVocabulary{first, second}
	if got := chain.Name("X"); got != "First" {
		t.Errorf("chain should pick first match: got %q, want First", got)
	}
}
