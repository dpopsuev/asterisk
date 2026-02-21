package framework

import "sync"

// Vocabulary translates machine codes to human-readable names.
// Implementations must be safe for concurrent use.
type Vocabulary interface {
	Name(code string) string
}

// VocabularyFunc adapts a plain function to the Vocabulary interface.
type VocabularyFunc func(string) string

func (f VocabularyFunc) Name(code string) string { return f(code) }

// MapVocabulary is a thread-safe, register-based vocabulary.
// Unknown codes are returned as-is (pass-through default).
type MapVocabulary struct {
	mu      sync.RWMutex
	entries map[string]string
}

// NewMapVocabulary returns an empty vocabulary ready for registration.
func NewMapVocabulary() *MapVocabulary {
	return &MapVocabulary{entries: make(map[string]string)}
}

// Register adds a single code → name mapping. Returns the receiver for chaining.
func (v *MapVocabulary) Register(code, name string) *MapVocabulary {
	v.mu.Lock()
	v.entries[code] = name
	v.mu.Unlock()
	return v
}

// RegisterAll adds all entries from the map. Returns the receiver for chaining.
func (v *MapVocabulary) RegisterAll(entries map[string]string) *MapVocabulary {
	v.mu.Lock()
	for code, name := range entries {
		v.entries[code] = name
	}
	v.mu.Unlock()
	return v
}

// Name returns the human-readable name for code, or code itself if unregistered.
func (v *MapVocabulary) Name(code string) string {
	v.mu.RLock()
	name, ok := v.entries[code]
	v.mu.RUnlock()
	if ok {
		return name
	}
	return code
}

// NameWithCode formats as "Human Name (code)" for dual-audience contexts.
// If the vocabulary returns the code unchanged, only the code is returned.
func NameWithCode(v Vocabulary, code string) string {
	name := v.Name(code)
	if name == code {
		return code
	}
	return name + " (" + code + ")"
}

// ChainVocabulary tries multiple vocabularies in order. The first one that
// returns a value different from the input code wins. If none translates
// the code, the code is returned as-is.
type ChainVocabulary []Vocabulary

func (c ChainVocabulary) Name(code string) string {
	for _, v := range c {
		if name := v.Name(code); name != code {
			return name
		}
	}
	return code
}
