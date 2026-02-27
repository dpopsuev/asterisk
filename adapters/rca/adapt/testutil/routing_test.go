package testutil

import (
	"os"
	"path/filepath"
	"testing"

	"asterisk/adapters/rca/adapt"
)

func TestAssertRouting_Pass(t *testing.T) {
	log := adapt.RoutingLog{
		{CaseID: "C1", Step: "F0_RECALL", AdapterColor: "crimson"},
		{CaseID: "C1", Step: "F1_TRIAGE", AdapterColor: "cerulean"},
	}
	AssertRouting(t, log, "C1", "F0_RECALL", "crimson")
	AssertRouting(t, log, "C1", "F1_TRIAGE", "cerulean")
}

func TestAssertRouting_MissingEntry(t *testing.T) {
	log := adapt.RoutingLog{
		{CaseID: "C1", Step: "F0_RECALL", AdapterColor: "crimson"},
	}
	ft := &fakeTB{}
	AssertRouting(ft, log, "C1", "F3_INVEST", "cerulean")
	if !ft.errored {
		t.Error("expected AssertRouting to fail for missing entry")
	}
}

func TestAssertRouting_ColorMismatch(t *testing.T) {
	log := adapt.RoutingLog{
		{CaseID: "C1", Step: "F0_RECALL", AdapterColor: "crimson"},
	}
	ft := &fakeTB{}
	AssertRouting(ft, log, "C1", "F0_RECALL", "cerulean")
	if !ft.errored {
		t.Error("expected AssertRouting to fail for color mismatch")
	}
}

func TestAssertAllRouted_Pass(t *testing.T) {
	log := adapt.RoutingLog{
		{CaseID: "C1", Step: "F0", AdapterColor: "crimson"},
		{CaseID: "C2", Step: "F0", AdapterColor: "cerulean"},
		{CaseID: "C1", Step: "F1", AdapterColor: "crimson"},
	}
	AssertAllRouted(t, log, []string{"C1", "C2"})
}

func TestAssertAllRouted_MissingCase(t *testing.T) {
	log := adapt.RoutingLog{
		{CaseID: "C1", Step: "F0", AdapterColor: "crimson"},
	}
	ft := &fakeTB{}
	AssertAllRouted(ft, log, []string{"C1", "C2"})
	if !ft.errored {
		t.Error("expected AssertAllRouted to fail for missing case")
	}
}

func TestLoadRoutingReplay_Success(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "replay.json")
	original := adapt.RoutingLog{
		{CaseID: "C1", Step: "F0", AdapterColor: "crimson"},
	}
	if err := adapt.SaveRoutingLog(path, original); err != nil {
		t.Fatal(err)
	}
	loaded := LoadRoutingReplay(t, path)
	if loaded.Len() != 1 {
		t.Errorf("loaded %d entries, want 1", loaded.Len())
	}
}

func TestLoadRoutingReplay_BadPath(t *testing.T) {
	ft := &fakeTB{}
	LoadRoutingReplay(ft, "/nonexistent/file.json")
	if !ft.fataled {
		t.Error("expected LoadRoutingReplay to fatal on bad path")
	}
}

// fakeTB captures test failures without aborting the real test.
type fakeTB struct {
	testing.TB
	errored bool
	fataled bool
}

func (f *fakeTB) Helper()                       {}
func (f *fakeTB) Errorf(string, ...interface{})  { f.errored = true }
func (f *fakeTB) Fatalf(string, ...interface{})  { f.fataled = true }

// Ensure fakeTB suppresses log output.
func (f *fakeTB) Log(...interface{})             {}
func (f *fakeTB) Logf(string, ...interface{})    {}

func TestAssertRouting_EmptyLog(t *testing.T) {
	ft := &fakeTB{}
	AssertRouting(ft, adapt.RoutingLog{}, "C1", "F0", "crimson")
	if !ft.errored {
		t.Error("expected error for empty log")
	}
}

func TestAssertAllRouted_EmptyLog(t *testing.T) {
	ft := &fakeTB{}
	AssertAllRouted(ft, adapt.RoutingLog{}, []string{"C1"})
	if !ft.errored {
		t.Error("expected error for empty log with cases")
	}
}

func TestAssertAllRouted_EmptyCases(t *testing.T) {
	log := adapt.RoutingLog{
		{CaseID: "C1", Step: "F0", AdapterColor: "crimson"},
	}
	AssertAllRouted(t, log, []string{})
}

func TestLoadRoutingReplay_BadJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")
	os.WriteFile(path, []byte("not json"), 0o644)

	ft := &fakeTB{}
	LoadRoutingReplay(ft, path)
	if !ft.fataled {
		t.Error("expected fatal on bad JSON")
	}
}
