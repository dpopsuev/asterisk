package framework

import "strings"

// KnownModels is the registry of LLM models that have been observed
// behind adapters. The wet identity probe test fails on unknown models,
// forcing explicit acknowledgment of every new ghost that enters the system.
//
// Add entries here as they are discovered via live probes. The key is the
// model_name (lowercase) returned by the probe prompt.
var KnownModels = map[string]ModelIdentity{
	// Local adapters (no LLM)
	"stub":            {ModelName: "stub", Provider: "asterisk"},
	"basic-heuristic": {ModelName: "basic-heuristic", Provider: "asterisk"},

	// Cursor IDE agents -- Cursor's Task subagent layer reports this identity.
	// The underlying foundation model is abstracted away by Cursor.
	// Version returns "unknown" -- Cursor doesn't expose it.
	// Casing is inconsistent ("Composer" vs "composer"); IsKnownModel normalizes.
	// First seen: 2026-02-20, wet probe via 3 parallel subagents.
	"composer": {ModelName: "composer", Provider: "Cursor"},
}

// IsKnownModel checks whether a probed ModelIdentity matches an entry
// in the registry. Matches on ModelName (case-insensitive).
func IsKnownModel(mi ModelIdentity) bool {
	_, ok := KnownModels[strings.ToLower(mi.ModelName)]
	return ok
}

// LookupModel returns the registered identity for a model name.
func LookupModel(modelName string) (ModelIdentity, bool) {
	mi, ok := KnownModels[strings.ToLower(modelName)]
	return mi, ok
}
