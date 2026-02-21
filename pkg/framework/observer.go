package framework

import (
	"log/slog"
	"sync"
	"time"
)

// WalkEventType classifies walk events for filtering and routing.
type WalkEventType string

const (
	EventNodeEnter    WalkEventType = "node_enter"
	EventNodeExit     WalkEventType = "node_exit"
	EventEdgeEvaluate WalkEventType = "edge_evaluate"
	EventTransition   WalkEventType = "transition"
	EventWalkerSwitch WalkEventType = "walker_switch"
	EventWalkComplete WalkEventType = "walk_complete"
	EventWalkError    WalkEventType = "walk_error"
)

// WalkEvent is a single observation from a graph walk. The Metadata map
// is the forward-compatible extension point — new fields go there
// without breaking the struct.
type WalkEvent struct {
	Type     WalkEventType
	Node     string
	Walker   string
	Edge     string
	Artifact Artifact
	Elapsed  time.Duration
	Error    error
	Metadata map[string]any
}

// WalkObserver receives events during a graph walk. Single-method
// design (like http.Handler) so adding new event types never breaks
// existing observers.
type WalkObserver interface {
	OnEvent(WalkEvent)
}

// WalkObserverFunc adapts a plain function to the WalkObserver interface.
type WalkObserverFunc func(WalkEvent)

func (f WalkObserverFunc) OnEvent(e WalkEvent) { f(e) }

// MultiObserver fans out events to multiple observers.
type MultiObserver []WalkObserver

func (m MultiObserver) OnEvent(e WalkEvent) {
	for _, obs := range m {
		obs.OnEvent(e)
	}
}

// LogObserver writes walk events as structured slog lines.
type LogObserver struct {
	Logger *slog.Logger
}

func (o *LogObserver) OnEvent(e WalkEvent) {
	logger := o.Logger
	if logger == nil {
		logger = slog.Default()
	}

	attrs := []slog.Attr{
		slog.String("event", string(e.Type)),
	}
	if e.Node != "" {
		attrs = append(attrs, slog.String("node", e.Node))
	}
	if e.Walker != "" {
		attrs = append(attrs, slog.String("walker", e.Walker))
	}
	if e.Edge != "" {
		attrs = append(attrs, slog.String("edge", e.Edge))
	}
	if e.Elapsed > 0 {
		attrs = append(attrs, slog.Duration("elapsed", e.Elapsed))
	}
	if e.Error != nil {
		attrs = append(attrs, slog.String("error", e.Error.Error()))
	}

	args := make([]any, len(attrs))
	for i, a := range attrs {
		args[i] = a
	}

	if e.Error != nil {
		logger.LogAttrs(nil, slog.LevelWarn, "walk", attrs...)
	} else {
		logger.LogAttrs(nil, slog.LevelInfo, "walk", attrs...)
	}
}

// TraceCollector accumulates walk events in memory for post-walk analysis.
// Safe for concurrent use.
type TraceCollector struct {
	mu     sync.Mutex
	events []WalkEvent
}

func (t *TraceCollector) OnEvent(e WalkEvent) {
	t.mu.Lock()
	t.events = append(t.events, e)
	t.mu.Unlock()
}

// Events returns a copy of all collected events.
func (t *TraceCollector) Events() []WalkEvent {
	t.mu.Lock()
	defer t.mu.Unlock()
	out := make([]WalkEvent, len(t.events))
	copy(out, t.events)
	return out
}

// Reset clears collected events.
func (t *TraceCollector) Reset() {
	t.mu.Lock()
	t.events = nil
	t.mu.Unlock()
}

// EventsOfType returns only events matching the given type.
func (t *TraceCollector) EventsOfType(typ WalkEventType) []WalkEvent {
	t.mu.Lock()
	defer t.mu.Unlock()
	var out []WalkEvent
	for _, e := range t.events {
		if e.Type == typ {
			out = append(out, e)
		}
	}
	return out
}

// emitEvent is a helper to safely emit an event to a possibly-nil observer.
func emitEvent(obs WalkObserver, e WalkEvent) {
	if obs != nil {
		obs.OnEvent(e)
	}
}
