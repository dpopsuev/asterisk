// Package testutil provides test helpers for asserting adapter routing decisions.
package testutil

import (
	"testing"

	"asterisk/adapters/rca/adapt"
)

// AssertRouting finds the entry for (caseID, step) and asserts the adapter color matches.
// Fails with a clear diff on mismatch or missing entry.
func AssertRouting(t testing.TB, log adapt.RoutingLog, caseID, step, expectedColor string) {
	t.Helper()
	entries := log.ForCase(caseID).ForStep(step)
	if entries.Len() == 0 {
		t.Errorf("AssertRouting: no entry for case=%q step=%q; log has %d entries", caseID, step, log.Len())
		return
	}
	actual := entries[0].AdapterColor
	if actual != expectedColor {
		t.Errorf("AssertRouting: case=%q step=%q: got color=%q, want %q", caseID, step, actual, expectedColor)
	}
}

// AssertAllRouted verifies that every case ID in cases appears at least once in the log.
func AssertAllRouted(t testing.TB, log adapt.RoutingLog, cases []string) {
	t.Helper()
	seen := make(map[string]bool, log.Len())
	for _, e := range log {
		seen[e.CaseID] = true
	}
	for _, id := range cases {
		if !seen[id] {
			t.Errorf("AssertAllRouted: case %q has no routing entries", id)
		}
	}
}

// LoadRoutingReplay loads a routing log from path, calling t.Fatal on error.
func LoadRoutingReplay(t testing.TB, path string) adapt.RoutingLog {
	t.Helper()
	log, err := adapt.LoadRoutingLog(path)
	if err != nil {
		t.Fatalf("LoadRoutingReplay: %v", err)
	}
	return log
}
