package framework

import (
	"context"
	"fmt"
	"time"
)

// Graph is a directed graph of Nodes connected by Edges, partitioned into Zones.
type Graph interface {
	Name() string
	Nodes() []Node
	Edges() []Edge
	Zones() []Zone
	NodeByName(name string) (Node, bool)
	EdgesFrom(nodeName string) []Edge
	Walk(ctx context.Context, walker Walker, startNode string) error
}

// Zone is a meta-phase grouping of Nodes with shared characteristics.
type Zone struct {
	Name            string
	NodeNames       []string
	ElementAffinity Element
	Stickiness      int // 0-3 stickiness value for agents in this zone
}

// DefaultGraph is the reference Graph implementation. It stores nodes and
// edges in maps for O(1) lookup while preserving edge definition order
// for deterministic first-match evaluation.
type DefaultGraph struct {
	name      string
	nodes     []Node
	edges     []Edge
	zones     []Zone
	nodeIndex map[string]Node
	edgeIndex map[string][]Edge // from-node -> edges in definition order
	doneNode  string            // terminal pseudo-node name (walk stops here)
}

// GraphOption configures a DefaultGraph during construction.
type GraphOption func(*DefaultGraph)

// WithDoneNode sets the terminal pseudo-node name. When a transition targets
// this node, the walk completes successfully. Defaults to "_done".
func WithDoneNode(name string) GraphOption {
	return func(g *DefaultGraph) {
		g.doneNode = name
	}
}

// NewGraph constructs a DefaultGraph from the provided nodes, edges, and zones.
// Returns an error if referential integrity checks fail (e.g. an edge
// references a nonexistent node).
func NewGraph(name string, nodes []Node, edges []Edge, zones []Zone, opts ...GraphOption) (*DefaultGraph, error) {
	g := &DefaultGraph{
		name:      name,
		nodes:     nodes,
		edges:     edges,
		zones:     zones,
		nodeIndex: make(map[string]Node, len(nodes)),
		edgeIndex: make(map[string][]Edge),
		doneNode:  "_done",
	}
	for _, opt := range opts {
		opt(g)
	}

	for _, n := range nodes {
		g.nodeIndex[n.Name()] = n
	}
	for _, e := range edges {
		if _, ok := g.nodeIndex[e.From()]; !ok {
			return nil, fmt.Errorf("%w: edge %s references source %q", ErrNodeNotFound, e.ID(), e.From())
		}
		to := e.To()
		if to != g.doneNode {
			if _, ok := g.nodeIndex[to]; !ok {
				return nil, fmt.Errorf("%w: edge %s references target %q", ErrNodeNotFound, e.ID(), to)
			}
		}
		g.edgeIndex[e.From()] = append(g.edgeIndex[e.From()], e)
	}

	return g, nil
}

func (g *DefaultGraph) Name() string    { return g.name }
func (g *DefaultGraph) Nodes() []Node   { return g.nodes }
func (g *DefaultGraph) Edges() []Edge   { return g.edges }
func (g *DefaultGraph) Zones() []Zone   { return g.zones }

func (g *DefaultGraph) NodeByName(name string) (Node, bool) {
	n, ok := g.nodeIndex[name]
	return n, ok
}

func (g *DefaultGraph) EdgesFrom(nodeName string) []Edge {
	return g.edgeIndex[nodeName]
}

// Walk traverses the graph starting at startNode using the provided walker.
// At each node, the walker processes the node to produce an artifact, then
// edges from that node are evaluated in definition order (first match wins).
// The walk completes when a transition targets the done node, or returns an
// error if no edge matches or a node is not found.
func (g *DefaultGraph) Walk(ctx context.Context, walker Walker, startNode string) error {
	node, ok := g.nodeIndex[startNode]
	if !ok {
		return fmt.Errorf("%w: start node %q", ErrNodeNotFound, startNode)
	}

	state := walker.State()
	state.CurrentNode = startNode
	var priorArtifact Artifact

	for {
		if err := ctx.Err(); err != nil {
			state.Status = "error"
			return err
		}

		nc := NodeContext{
			WalkerState:   state,
			PriorArtifact: priorArtifact,
			Meta:          make(map[string]string),
		}

		artifact, err := walker.Handle(ctx, node, nc)
		if err != nil {
			state.Status = "error"
			return fmt.Errorf("node %s: %w", node.Name(), err)
		}

		edges := g.EdgesFrom(node.Name())
		if len(edges) == 0 {
			state.Status = "done"
			return nil
		}

		var matched *Transition
		var matchedEdge Edge
		for _, e := range edges {
			t := e.Evaluate(artifact, state)
			if t != nil {
				matched = t
				matchedEdge = e
				break
			}
		}

		if matched == nil {
			state.Status = "error"
			return fmt.Errorf("%w: node %q, artifact type %q", ErrNoEdge, node.Name(), artifact.Type())
		}

		state.RecordStep(node.Name(), matchedEdge.ID(), matchedEdge.ID(), time.Now().UTC().Format(time.RFC3339))
		state.MergeContext(matched.ContextAdditions)

		if matched.NextNode == g.doneNode {
			state.Status = "done"
			return nil
		}

		nextNode, ok := g.nodeIndex[matched.NextNode]
		if !ok {
			state.Status = "error"
			return fmt.Errorf("%w: transition target %q from edge %s", ErrNodeNotFound, matched.NextNode, matchedEdge.ID())
		}

		priorArtifact = artifact
		node = nextNode
		state.CurrentNode = matched.NextNode
	}
}
