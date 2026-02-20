package framework

// KnownModels is the registry of LLM models that have been observed
// behind adapters. The wet identity probe test fails on unknown models,
// forcing explicit acknowledgment of every new ghost that enters the system.
//
// Add entries here as they are discovered via live probes. The key is the
// model_name returned by the probe prompt.
var KnownModels = map[string]ModelIdentity{
	// Local adapters (no LLM)
	"stub":            {ModelName: "stub", Provider: "asterisk"},
	"basic-heuristic": {ModelName: "basic-heuristic", Provider: "asterisk"},

	// Cursor IDE agents -- Cursor's Task subagent layer reports this identity.
	// The underlying foundation model is abstracted away by Cursor.
	// First seen: 2026-02-20, wet probe via 3 parallel subagents (all identical).
	"composer": {ModelName: "composer", Provider: "Cursor"},
}

// IsKnownModel checks whether a probed ModelIdentity matches an entry
// in the registry. Matches on ModelName only (provider is informational).
func IsKnownModel(mi ModelIdentity) bool {
	_, ok := KnownModels[mi.ModelName]
	return ok
}
