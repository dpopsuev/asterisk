package framework

import "context"

// Node is a processing stage in a pipeline graph.
// Implementations are domain-specific (e.g. recall, triage, investigate).
type Node interface {
	Name() string
	ElementAffinity() Element
	Process(ctx context.Context, nc NodeContext) (Artifact, error)
}

// Artifact is the output of a Node's processing.
// The framework treats it as opaque; typed artifacts are domain-specific.
type Artifact interface {
	Type() string
	Confidence() float64
	Raw() any
}

// NodeContext is the input to a Node's Process method: the accumulated
// context for this walker at this node.
type NodeContext struct {
	WalkerState   *WalkerState
	PriorArtifact Artifact
	Meta          map[string]any
}
