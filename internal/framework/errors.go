package framework

import "errors"

var (
	// ErrNodeNotFound is returned when a referenced node does not exist in the graph.
	ErrNodeNotFound = errors.New("framework: node not found")

	// ErrNoEdge is returned when no edge matches from the current node,
	// indicating the walk has reached a terminal state or a graph definition gap.
	ErrNoEdge = errors.New("framework: no matching edge from node")

	// ErrMaxLoops is returned when a loop edge's counter exceeds the configured maximum.
	ErrMaxLoops = errors.New("framework: max loop iterations exceeded")
)
