package mcp

import (
	"sync"
	"time"
)

// Signal represents a single event on the agent message bus.
type Signal struct {
	Timestamp string            `json:"ts"`
	Event     string            `json:"event"`
	Agent     string            `json:"agent"`
	CaseID    string            `json:"case_id,omitempty"`
	Step      string            `json:"step,omitempty"`
	Meta      map[string]string `json:"meta,omitempty"`
}

// SignalBus is a thread-safe, append-only signal log for agent coordination.
type SignalBus struct {
	mu      sync.Mutex
	signals []Signal
}

// NewSignalBus returns a new SignalBus.
func NewSignalBus() *SignalBus {
	return &SignalBus{}
}

// Emit appends a signal with the given event, agent, caseID, step, and meta.
func (b *SignalBus) Emit(event, agent, caseID, step string, meta map[string]string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.signals = append(b.signals, Signal{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Event:     event,
		Agent:     agent,
		CaseID:    caseID,
		Step:      step,
		Meta:      meta,
	})
}

// Since returns a copy of signals from index idx onward. If idx is negative it is clamped to 0.
// If idx >= len(signals), returns nil.
func (b *SignalBus) Since(idx int) []Signal {
	b.mu.Lock()
	defer b.mu.Unlock()
	if idx < 0 {
		idx = 0
	}
	if idx >= len(b.signals) {
		return nil
	}
	out := make([]Signal, len(b.signals)-idx)
	copy(out, b.signals[idx:])
	return out
}

// Len returns the number of signals in the bus.
func (b *SignalBus) Len() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.signals)
}
