//go:build wet

package framework

import (
	"encoding/json"
	"os"
	"testing"
)

// TestWetIdentityProbe_CheckRegistry validates that a live probe result
// matches the KnownModels registry.
//
// Run with: go test -tags wet -run TestWetIdentityProbe ./internal/framework/...
//
// The probe result is supplied via:
//   - IDENTITY_PROBE_JSON env var (raw JSON string), OR
//   - testdata/probe_result.json fixture file
//
// When the test fails, it prints the exact identity to add to known_models.go.
func TestWetIdentityProbe_CheckRegistry(t *testing.T) {
	raw := os.Getenv("IDENTITY_PROBE_JSON")
	if raw == "" {
		data, err := os.ReadFile("testdata/probe_result.json")
		if err != nil {
			t.Skip("no probe result available: set IDENTITY_PROBE_JSON or create testdata/probe_result.json")
		}
		raw = string(data)
	}

	var mi ModelIdentity
	if err := json.Unmarshal([]byte(raw), &mi); err != nil {
		t.Fatalf("failed to parse probe result: %v\nraw: %s", err, raw)
	}

	t.Logf("Probed identity: model_name=%q provider=%q", mi.ModelName, mi.Provider)
	t.Logf("String: %s", mi.String())
	t.Logf("Tag:    %s", mi.Tag())

	if !IsKnownModel(mi) {
		t.Fatalf("UNKNOWN MODEL detected: %s\n\n"+
			"Add this entry to known_models.go:\n\n"+
			"\t%q: {ModelName: %q, Provider: %q},\n",
			mi.String(), mi.ModelName, mi.ModelName, mi.Provider)
	}

	known := KnownModels[mi.ModelName]
	if known.Provider != mi.Provider {
		t.Errorf("provider mismatch: registry has %q, probe returned %q",
			known.Provider, mi.Provider)
	}

	t.Logf("Model %s is registered and verified", mi.String())
}

// TestWetIdentityProbe_RegistryNotEmpty ensures someone has populated the
// registry. If it's empty, the wet test is meaningless.
func TestWetIdentityProbe_RegistryNotEmpty(t *testing.T) {
	if len(KnownModels) == 0 {
		t.Fatal("KnownModels registry is empty -- run a live probe first and populate it")
	}
}
