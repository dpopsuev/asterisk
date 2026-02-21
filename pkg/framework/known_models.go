package framework

import "strings"

// KnownModels is the registry of foundation LLM models that have been
// observed behind adapters. The wet identity probe test fails on unknown
// models, forcing explicit acknowledgment of every new ghost.
//
// Keys are lowercase model_name. Only foundation models belong here;
// wrappers (Cursor, Copilot, Azure) go in KnownWrappers.
var KnownModels = map[string]ModelIdentity{
	// Local adapters (no LLM)
	"stub":            {ModelName: "stub", Provider: "asterisk"},
	"basic-heuristic": {ModelName: "basic-heuristic", Provider: "asterisk"},

	// Foundation models discovered via live probes
	// First seen: 2026-02-20, wet probe via 3 parallel Cursor subagents (2/3 hit).
	"claude-sonnet-4-20250514": {ModelName: "claude-sonnet-4-20250514", Provider: "Anthropic", Version: "20250514"},
}

// KnownWrappers lists hosting environments that sit between the caller
// and the foundation model. If a probe's model_name matches a wrapper
// name, the probe failed to reach the foundation layer.
var KnownWrappers = map[string]bool{
	"auto":     true, // Cursor auto-select layer; foundation model is behind it
	"composer": true,
	"copilot":  true,
	"cursor":   true,
	"azure":    true,
}

// IsKnownModel checks whether a probed ModelIdentity matches a foundation
// model in the registry. Matches on ModelName (case-insensitive).
func IsKnownModel(mi ModelIdentity) bool {
	_, ok := KnownModels[strings.ToLower(mi.ModelName)]
	return ok
}

// IsWrapperName returns true if the given name is a known wrapper/IDE
// rather than a foundation model. Case-insensitive.
func IsWrapperName(name string) bool {
	return KnownWrappers[strings.ToLower(name)]
}

// LookupModel returns the registered identity for a foundation model name.
func LookupModel(modelName string) (ModelIdentity, bool) {
	mi, ok := KnownModels[strings.ToLower(modelName)]
	return mi, ok
}
