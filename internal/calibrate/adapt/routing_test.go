package adapt

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"asterisk/internal/calibrate"
	"asterisk/internal/orchestrate"
	"asterisk/adapters/store"
	"github.com/dpopsuev/origami/knowledge"
)

// fakeAdapter is a minimal ModelAdapter for testing the recorder.
type fakeAdapter struct {
	name    string
	calls   []fakeCall
	mu      sync.Mutex
	storeOK bool
	idOK    bool
}

type fakeCall struct {
	CaseID string
	Step   string
}

func (f *fakeAdapter) Name() string { return f.name }

func (f *fakeAdapter) SendPrompt(caseID string, step string, _ string) (json.RawMessage, error) {
	f.mu.Lock()
	f.calls = append(f.calls, fakeCall{caseID, step})
	f.mu.Unlock()
	return json.RawMessage(`{"ok":true}`), nil
}

// fakeStoreAwareAdapter implements both ModelAdapter and StoreAware.
type fakeStoreAwareAdapter struct {
	fakeAdapter
	storeCalls   int
	suiteCalls   int
	wsCalls      int
	regCalls     int
	mu2          sync.Mutex
}

func (f *fakeStoreAwareAdapter) SetStore(_ store.Store) {
	f.mu2.Lock()
	f.storeCalls++
	f.mu2.Unlock()
}
func (f *fakeStoreAwareAdapter) SetSuiteID(_ int64) {
	f.mu2.Lock()
	f.suiteCalls++
	f.mu2.Unlock()
}
func (f *fakeStoreAwareAdapter) SetCatalog(_ *knowledge.KnowledgeSourceCatalog) {
	f.mu2.Lock()
	f.wsCalls++
	f.mu2.Unlock()
}
func (f *fakeStoreAwareAdapter) RegisterCase(_ string, _ *store.Case) {
	f.mu2.Lock()
	f.regCalls++
	f.mu2.Unlock()
}

// fakeIDMappableAdapter implements both ModelAdapter and IDMappable.
type fakeIDMappableAdapter struct {
	fakeAdapter
	rcaCalls     int
	symptomCalls int
	mu2          sync.Mutex
}

func (f *fakeIDMappableAdapter) SetRCAID(_ string, _ int64) {
	f.mu2.Lock()
	f.rcaCalls++
	f.mu2.Unlock()
}
func (f *fakeIDMappableAdapter) SetSymptomID(_ string, _ int64) {
	f.mu2.Lock()
	f.symptomCalls++
	f.mu2.Unlock()
}

func TestRoutingRecorder_Records(t *testing.T) {
	inner := &fakeAdapter{name: "test-adapter"}
	rec := NewRoutingRecorder(inner, "crimson")

	resp, err := rec.SendPrompt("C1", string(orchestrate.StepF1Triage), "prompt")
	if err != nil {
		t.Fatal(err)
	}
	if string(resp) != `{"ok":true}` {
		t.Errorf("response = %s, want {\"ok\":true}", resp)
	}

	log := rec.Log()
	if log.Len() != 1 {
		t.Fatalf("log.Len() = %d, want 1", log.Len())
	}
	e := log[0]
	if e.CaseID != "C1" {
		t.Errorf("CaseID = %q, want C1", e.CaseID)
	}
	if e.Step != string(orchestrate.StepF1Triage) {
		t.Errorf("Step = %q, want %s", e.Step, orchestrate.StepF1Triage)
	}
	if e.AdapterColor != "crimson" {
		t.Errorf("AdapterColor = %q, want crimson", e.AdapterColor)
	}
	if e.DispatchID != 1 {
		t.Errorf("DispatchID = %d, want 1", e.DispatchID)
	}
	if e.Timestamp.IsZero() {
		t.Error("Timestamp is zero")
	}
}

func TestRoutingRecorder_DelegatesName(t *testing.T) {
	inner := &fakeAdapter{name: "basic"}
	rec := NewRoutingRecorder(inner, "crimson")
	if rec.Name() != "basic" {
		t.Errorf("Name() = %q, want basic", rec.Name())
	}
}

func TestRoutingRecorder_DelegatesStoreAware(t *testing.T) {
	inner := &fakeStoreAwareAdapter{fakeAdapter: fakeAdapter{name: "llm"}}
	rec := NewRoutingRecorder(inner, "cerulean")

	rec.SetStore(nil)
	rec.SetSuiteID(42)
	rec.SetCatalog(nil)
	rec.RegisterCase("C1", nil)

	if inner.storeCalls != 1 {
		t.Errorf("SetStore calls = %d, want 1", inner.storeCalls)
	}
	if inner.suiteCalls != 1 {
		t.Errorf("SetSuiteID calls = %d, want 1", inner.suiteCalls)
	}
	if inner.wsCalls != 1 {
		t.Errorf("SetCatalog calls = %d, want 1", inner.wsCalls)
	}
	if inner.regCalls != 1 {
		t.Errorf("RegisterCase calls = %d, want 1", inner.regCalls)
	}
}

func TestRoutingRecorder_DelegatesIDMappable(t *testing.T) {
	inner := &fakeIDMappableAdapter{fakeAdapter: fakeAdapter{name: "stub"}}
	rec := NewRoutingRecorder(inner, "stub")

	rec.SetRCAID("rca-1", 100)
	rec.SetSymptomID("sym-1", 200)

	if inner.rcaCalls != 1 {
		t.Errorf("SetRCAID calls = %d, want 1", inner.rcaCalls)
	}
	if inner.symptomCalls != 1 {
		t.Errorf("SetSymptomID calls = %d, want 1", inner.symptomCalls)
	}
}

func TestRoutingRecorder_StoreAwareNoOpOnPlainAdapter(t *testing.T) {
	inner := &fakeAdapter{name: "plain"}
	rec := NewRoutingRecorder(inner, "amber")

	// These should not panic on a non-StoreAware adapter.
	rec.SetStore(nil)
	rec.SetSuiteID(1)
	rec.SetCatalog(nil)
	rec.RegisterCase("C1", nil)
	rec.SetRCAID("r1", 1)
	rec.SetSymptomID("s1", 1)
}

func TestRoutingRecorder_ConcurrentSafety(t *testing.T) {
	inner := &fakeAdapter{name: "test"}
	rec := NewRoutingRecorder(inner, "crimson")

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			_, _ = rec.SendPrompt("C1", string(orchestrate.StepF0Recall), "")
		}(i)
	}
	wg.Wait()

	log := rec.Log()
	if log.Len() != 50 {
		t.Errorf("log.Len() = %d, want 50", log.Len())
	}
}

func TestRoutingLog_ForCase(t *testing.T) {
	log := RoutingLog{
		{CaseID: "C1", Step: "F0", AdapterColor: "crimson"},
		{CaseID: "C2", Step: "F0", AdapterColor: "cerulean"},
		{CaseID: "C1", Step: "F1", AdapterColor: "crimson"},
	}
	filtered := log.ForCase("C1")
	if filtered.Len() != 2 {
		t.Errorf("ForCase(C1).Len() = %d, want 2", filtered.Len())
	}
	for _, e := range filtered {
		if e.CaseID != "C1" {
			t.Errorf("unexpected CaseID %q in filtered result", e.CaseID)
		}
	}
}

func TestRoutingLog_ForStep(t *testing.T) {
	log := RoutingLog{
		{CaseID: "C1", Step: "F0", AdapterColor: "crimson"},
		{CaseID: "C2", Step: "F0", AdapterColor: "cerulean"},
		{CaseID: "C1", Step: "F1", AdapterColor: "crimson"},
	}
	filtered := log.ForStep("F0")
	if filtered.Len() != 2 {
		t.Errorf("ForStep(F0).Len() = %d, want 2", filtered.Len())
	}
}

func TestRoutingLog_ForCaseForStep(t *testing.T) {
	log := RoutingLog{
		{CaseID: "C1", Step: "F0", AdapterColor: "crimson"},
		{CaseID: "C1", Step: "F1", AdapterColor: "cerulean"},
		{CaseID: "C2", Step: "F0", AdapterColor: "amber"},
	}
	filtered := log.ForCase("C1").ForStep("F1")
	if filtered.Len() != 1 {
		t.Fatalf("chained filter: Len() = %d, want 1", filtered.Len())
	}
	if filtered[0].AdapterColor != "cerulean" {
		t.Errorf("color = %q, want cerulean", filtered[0].AdapterColor)
	}
}

func TestRoutingLog_EmptyFilters(t *testing.T) {
	log := RoutingLog{}
	if log.ForCase("C1").Len() != 0 {
		t.Error("ForCase on empty log should return empty")
	}
	if log.ForStep("F0").Len() != 0 {
		t.Error("ForStep on empty log should return empty")
	}
}

func TestSaveLoadRoutingLog_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "routing.json")

	original := RoutingLog{
		{CaseID: "C1", Step: "F0", AdapterColor: "crimson", Timestamp: time.Date(2026, 2, 19, 12, 0, 0, 0, time.UTC), DispatchID: 1},
		{CaseID: "C1", Step: "F1", AdapterColor: "cerulean", Timestamp: time.Date(2026, 2, 19, 12, 1, 0, 0, time.UTC), DispatchID: 2},
	}

	if err := SaveRoutingLog(path, original); err != nil {
		t.Fatal(err)
	}

	loaded, err := LoadRoutingLog(path)
	if err != nil {
		t.Fatal(err)
	}

	if len(loaded) != len(original) {
		t.Fatalf("loaded %d entries, want %d", len(loaded), len(original))
	}
	for i, e := range loaded {
		o := original[i]
		if e.CaseID != o.CaseID || e.Step != o.Step || e.AdapterColor != o.AdapterColor || e.DispatchID != o.DispatchID {
			t.Errorf("entry[%d]: got %+v, want %+v", i, e, o)
		}
	}
}

func TestSaveRoutingLog_BadPath(t *testing.T) {
	err := SaveRoutingLog("/nonexistent/dir/routing.json", RoutingLog{})
	if err == nil {
		t.Error("expected error for bad path")
	}
}

func TestLoadRoutingLog_BadPath(t *testing.T) {
	_, err := LoadRoutingLog("/nonexistent/routing.json")
	if err == nil {
		t.Error("expected error for bad path")
	}
}

func TestLoadRoutingLog_BadJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")
	os.WriteFile(path, []byte("not json"), 0o644)

	_, err := LoadRoutingLog(path)
	if err == nil {
		t.Error("expected error for bad JSON")
	}
}

func TestCompareRoutingLogs_Identical(t *testing.T) {
	a := RoutingLog{
		{CaseID: "C1", Step: "F0", AdapterColor: "crimson"},
		{CaseID: "C1", Step: "F1", AdapterColor: "cerulean"},
	}
	diffs := CompareRoutingLogs(a, a)
	if len(diffs) != 0 {
		t.Errorf("identical logs produced %d diffs", len(diffs))
	}
}

func TestCompareRoutingLogs_ColorMismatch(t *testing.T) {
	expected := RoutingLog{
		{CaseID: "C1", Step: "F0", AdapterColor: "crimson"},
	}
	actual := RoutingLog{
		{CaseID: "C1", Step: "F0", AdapterColor: "cerulean"},
	}
	diffs := CompareRoutingLogs(expected, actual)
	if len(diffs) != 1 {
		t.Fatalf("got %d diffs, want 1", len(diffs))
	}
	d := diffs[0]
	if d.Expected != "crimson" || d.Actual != "cerulean" {
		t.Errorf("diff = %+v, want crimson->cerulean", d)
	}
}

func TestCompareRoutingLogs_MissingInActual(t *testing.T) {
	expected := RoutingLog{
		{CaseID: "C1", Step: "F0", AdapterColor: "crimson"},
		{CaseID: "C2", Step: "F0", AdapterColor: "amber"},
	}
	actual := RoutingLog{
		{CaseID: "C1", Step: "F0", AdapterColor: "crimson"},
	}
	diffs := CompareRoutingLogs(expected, actual)
	if len(diffs) != 1 {
		t.Fatalf("got %d diffs, want 1", len(diffs))
	}
	if diffs[0].Actual != "<missing>" {
		t.Errorf("expected <missing>, got %q", diffs[0].Actual)
	}
}

func TestCompareRoutingLogs_ExtraInActual(t *testing.T) {
	expected := RoutingLog{
		{CaseID: "C1", Step: "F0", AdapterColor: "crimson"},
	}
	actual := RoutingLog{
		{CaseID: "C1", Step: "F0", AdapterColor: "crimson"},
		{CaseID: "C3", Step: "F0", AdapterColor: "amber"},
	}
	diffs := CompareRoutingLogs(expected, actual)
	if len(diffs) != 1 {
		t.Fatalf("got %d diffs, want 1", len(diffs))
	}
	if diffs[0].Expected != "<missing>" {
		t.Errorf("expected <missing>, got %q", diffs[0].Expected)
	}
}

// Verify interface compliance at compile time.
var (
	_ calibrate.ModelAdapter = (*RoutingRecorder)(nil)
	_ calibrate.StoreAware   = (*RoutingRecorder)(nil)
	_ calibrate.IDMappable   = (*RoutingRecorder)(nil)
)
