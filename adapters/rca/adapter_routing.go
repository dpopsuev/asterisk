package rca

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"asterisk/adapters/store"
	"github.com/dpopsuev/origami/knowledge"
	"github.com/dpopsuev/origami/logging"
)

// RoutingEntry records a single adapter routing decision.
type RoutingEntry struct {
	CaseID       string    `json:"case_id"`
	Step         string    `json:"step"`
	AdapterColor string    `json:"adapter_color"`
	Timestamp    time.Time `json:"timestamp"`
	DispatchID   int64     `json:"dispatch_id,omitempty"`
}

// RoutingLog is an ordered sequence of routing decisions.
type RoutingLog []RoutingEntry

func (l RoutingLog) ForCase(id string) RoutingLog {
	var out RoutingLog
	for _, e := range l {
		if e.CaseID == id {
			out = append(out, e)
		}
	}
	return out
}

func (l RoutingLog) ForStep(step string) RoutingLog {
	var out RoutingLog
	for _, e := range l {
		if e.Step == step {
			out = append(out, e)
		}
	}
	return out
}

func (l RoutingLog) Len() int { return len(l) }

// RoutingRecorder wraps a ModelAdapter, recording every SendPrompt call.
// Thread-safe for parallel calibration.
type RoutingRecorder struct {
	inner ModelAdapter
	color string
	mu    sync.Mutex
	log   RoutingLog
	seq   int64
}

func NewRoutingRecorder(inner ModelAdapter, color string) *RoutingRecorder {
	return &RoutingRecorder{inner: inner, color: color}
}

func (r *RoutingRecorder) Name() string { return r.inner.Name() }

func (r *RoutingRecorder) SendPrompt(caseID string, step string, prompt string) (json.RawMessage, error) {
	r.mu.Lock()
	r.seq++
	entry := RoutingEntry{
		CaseID: caseID, Step: step, AdapterColor: r.color,
		Timestamp: time.Now(), DispatchID: r.seq,
	}
	r.log = append(r.log, entry)
	r.mu.Unlock()

	logging.New("routing").Info("dispatch", "color", r.color, "case_id", caseID, "step", step)
	return r.inner.SendPrompt(caseID, step, prompt)
}

func (r *RoutingRecorder) Log() RoutingLog {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make(RoutingLog, len(r.log))
	copy(out, r.log)
	return out
}

// StoreAware delegation
func (r *RoutingRecorder) SetStore(st store.Store) {
	if sa, ok := r.inner.(StoreAware); ok {
		sa.SetStore(st)
	}
}

func (r *RoutingRecorder) SetSuiteID(id int64) {
	if sa, ok := r.inner.(StoreAware); ok {
		sa.SetSuiteID(id)
	}
}

func (r *RoutingRecorder) SetCatalog(cat *knowledge.KnowledgeSourceCatalog) {
	if sa, ok := r.inner.(StoreAware); ok {
		sa.SetCatalog(cat)
	}
}

func (r *RoutingRecorder) RegisterCase(gtCaseID string, storeCase *store.Case) {
	if sa, ok := r.inner.(StoreAware); ok {
		sa.RegisterCase(gtCaseID, storeCase)
	}
}

// IDMappable delegation
func (r *RoutingRecorder) SetRCAID(gtID string, storeID int64) {
	if im, ok := r.inner.(IDMappable); ok {
		im.SetRCAID(gtID, storeID)
	}
}

func (r *RoutingRecorder) SetSymptomID(gtID string, storeID int64) {
	if im, ok := r.inner.(IDMappable); ok {
		im.SetSymptomID(gtID, storeID)
	}
}

func SaveRoutingLog(path string, log RoutingLog) error {
	data, err := json.MarshalIndent(log, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal routing log: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write routing log to %s: %w", path, err)
	}
	logging.New("routing").Info("routing log saved", "path", path, "entries", len(log))
	return nil
}

func LoadRoutingLog(path string) (RoutingLog, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read routing log from %s: %w", path, err)
	}
	var log RoutingLog
	if err := json.Unmarshal(data, &log); err != nil {
		return nil, fmt.Errorf("unmarshal routing log: %w", err)
	}
	return log, nil
}

type RoutingDiff struct {
	CaseID   string `json:"case_id"`
	Step     string `json:"step"`
	Expected string `json:"expected"`
	Actual   string `json:"actual"`
}

func CompareRoutingLogs(expected, actual RoutingLog) []RoutingDiff {
	type key struct{ CaseID, Step string }
	actualMap := make(map[key]string, len(actual))
	for _, e := range actual {
		actualMap[key{e.CaseID, e.Step}] = e.AdapterColor
	}
	expectedMap := make(map[key]string, len(expected))

	var diffs []RoutingDiff
	for _, e := range expected {
		k := key{e.CaseID, e.Step}
		expectedMap[k] = e.AdapterColor
		ac, ok := actualMap[k]
		if !ok {
			diffs = append(diffs, RoutingDiff{CaseID: e.CaseID, Step: e.Step, Expected: e.AdapterColor, Actual: "<missing>"})
		} else if ac != e.AdapterColor {
			diffs = append(diffs, RoutingDiff{CaseID: e.CaseID, Step: e.Step, Expected: e.AdapterColor, Actual: ac})
		}
	}
	for _, e := range actual {
		k := key{e.CaseID, e.Step}
		if _, ok := expectedMap[k]; !ok {
			diffs = append(diffs, RoutingDiff{CaseID: e.CaseID, Step: e.Step, Expected: "<missing>", Actual: e.AdapterColor})
		}
	}
	return diffs
}
