package framework

import "context"

// Walker is an agent traversing a graph. It combines identity
// (who the agent is) with processing capability (how it handles nodes).
type Walker interface {
	Identity() AgentIdentity
	State() *WalkerState
	Handle(ctx context.Context, node Node, nc NodeContext) (Artifact, error)
}

// WalkerState tracks a walker's progress through a graph.
// It mirrors orchestrate.CaseState with string-based node names
// instead of typed PipelineStep.
type WalkerState struct {
	ID          string         `json:"id"`
	CurrentNode string         `json:"current_node"`
	LoopCounts  map[string]int `json:"loop_counts"`
	Status      string         `json:"status"` // running, paused, done, error
	History     []StepRecord   `json:"history"`
	Context     map[string]any `json:"context"`
}

// NewWalkerState creates a WalkerState with initialized maps.
func NewWalkerState(id string) *WalkerState {
	return &WalkerState{
		ID:         id,
		Status:     "running",
		LoopCounts: make(map[string]int),
		Context:    make(map[string]any),
	}
}

// RecordStep appends a step to the history and updates the current node.
func (ws *WalkerState) RecordStep(node, outcome, edgeID, timestamp string) {
	ws.History = append(ws.History, StepRecord{
		Node:      node,
		Outcome:   outcome,
		EdgeID:    edgeID,
		Timestamp: timestamp,
	})
	ws.CurrentNode = node
}

// IncrementLoop increments the loop counter for an edge and returns the new count.
func (ws *WalkerState) IncrementLoop(edgeID string) int {
	ws.LoopCounts[edgeID]++
	return ws.LoopCounts[edgeID]
}

// MergeContext merges additions into the walker's accumulated context.
func (ws *WalkerState) MergeContext(additions map[string]any) {
	if additions == nil {
		return
	}
	for k, v := range additions {
		ws.Context[k] = v
	}
}

// StepRecord logs a completed node visit.
type StepRecord struct {
	Node      string `json:"node"`
	Outcome   string `json:"outcome"`
	EdgeID    string `json:"edge_id"`
	Timestamp string `json:"timestamp"`
}
