package scenarios_test

import (
	"testing"

	"asterisk/adapters/calibration/scenarios"

	"github.com/google/go-cmp/cmp"
)

func TestLoadScenario_AllValid(t *testing.T) {
	for _, name := range scenarios.ListScenarios() {
		t.Run(name, func(t *testing.T) {
			s, err := scenarios.LoadScenario(name)
			if err != nil {
				t.Fatalf("LoadScenario(%q): %v", name, err)
			}
			if s.Name != name {
				t.Errorf("Name = %q, want %q", s.Name, name)
			}
			if len(s.Cases) == 0 {
				t.Error("expected at least one case")
			}
			if len(s.RCAs) == 0 {
				t.Error("expected at least one RCA")
			}
		})
	}
}

func TestListScenarios(t *testing.T) {
	names := scenarios.ListScenarios()
	if len(names) != 4 {
		t.Fatalf("expected 4 scenarios, got %d: %v", len(names), names)
	}
	want := []string{"daemon-mock", "ptp-mock", "ptp-real", "ptp-real-ingest"}
	if diff := cmp.Diff(want, names); diff != "" {
		t.Errorf("ListScenarios mismatch:\n%s", diff)
	}
}

func TestLoadScenario_NotFound(t *testing.T) {
	_, err := scenarios.LoadScenario("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent scenario")
	}
}
