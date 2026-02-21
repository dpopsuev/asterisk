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
	WalkTeam(ctx context.Context, team *Team, startNode string) error
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
	observer  WalkObserver      // graph-level observer, used by Walk and composed with team observer in WalkTeam
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

// WithObserver attaches a graph-level observer that receives walk events
// from both Walk() and WalkTeam(). In WalkTeam(), this observer is composed
// with the team's observer via MultiObserver.
func WithObserver(obs WalkObserver) GraphOption {
	return func(g *DefaultGraph) {
		g.observer = obs
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
//
// If a graph-level observer is set via WithObserver, walk events are emitted
// at the same points as WalkTeam (node enter/exit, transitions, completion, errors).
func (g *DefaultGraph) Walk(ctx context.Context, walker Walker, startNode string) error {
	obs := g.observer
	walkerName := walker.Identity().PersonaName

	node, ok := g.nodeIndex[startNode]
	if !ok {
		err := fmt.Errorf("%w: start node %q", ErrNodeNotFound, startNode)
		emitEvent(obs, WalkEvent{Type: EventWalkError, Node: startNode, Error: err})
		return err
	}

	state := walker.State()
	state.CurrentNode = startNode
	var priorArtifact Artifact

	for {
		if err := ctx.Err(); err != nil {
			state.Status = "error"
			emitEvent(obs, WalkEvent{Type: EventWalkError, Error: err})
			return err
		}

		emitEvent(obs, WalkEvent{Type: EventNodeEnter, Node: node.Name(), Walker: walkerName})
		nodeStart := time.Now()

		nc := NodeContext{
			WalkerState:   state,
			PriorArtifact: priorArtifact,
			Meta:          make(map[string]any),
		}

		artifact, err := walker.Handle(ctx, node, nc)
		nodeElapsed := time.Since(nodeStart)

		if err != nil {
			state.Status = "error"
			emitEvent(obs, WalkEvent{Type: EventNodeExit, Node: node.Name(), Walker: walkerName, Elapsed: nodeElapsed, Error: err})
			emitEvent(obs, WalkEvent{Type: EventWalkError, Node: node.Name(), Error: err})
			return fmt.Errorf("node %s: %w", node.Name(), err)
		}

		emitEvent(obs, WalkEvent{Type: EventNodeExit, Node: node.Name(), Walker: walkerName, Artifact: artifact, Elapsed: nodeElapsed})

		edges := g.EdgesFrom(node.Name())
		if len(edges) == 0 {
			state.Status = "done"
			emitEvent(obs, WalkEvent{Type: EventWalkComplete, Node: node.Name(), Walker: walkerName})
			return nil
		}

		var matched *Transition
		var matchedEdge Edge
		for _, e := range edges {
			emitEvent(obs, WalkEvent{Type: EventEdgeEvaluate, Node: node.Name(), Edge: e.ID()})
			t := e.Evaluate(artifact, state)
			if t != nil {
				matched = t
				matchedEdge = e
				break
			}
		}

		if matched == nil {
			state.Status = "error"
			err := fmt.Errorf("%w: node %q, artifact type %q", ErrNoEdge, node.Name(), artifact.Type())
			emitEvent(obs, WalkEvent{Type: EventWalkError, Node: node.Name(), Error: err})
			return err
		}

		emitEvent(obs, WalkEvent{Type: EventTransition, Node: node.Name(), Edge: matchedEdge.ID()})

		state.RecordStep(node.Name(), matchedEdge.ID(), matchedEdge.ID(), time.Now().UTC().Format(time.RFC3339))
		state.MergeContext(matched.ContextAdditions)

		if matched.NextNode == g.doneNode {
			state.Status = "done"
			emitEvent(obs, WalkEvent{Type: EventWalkComplete, Walker: walkerName})
			return nil
		}

		nextNode, ok := g.nodeIndex[matched.NextNode]
		if !ok {
			state.Status = "error"
			err := fmt.Errorf("%w: transition target %q from edge %s", ErrNodeNotFound, matched.NextNode, matchedEdge.ID())
			emitEvent(obs, WalkEvent{Type: EventWalkError, Error: err})
			return err
		}

		priorArtifact = artifact
		node = nextNode
		state.CurrentNode = matched.NextNode
	}
}

// WalkTeam traverses the graph with multiple walkers coordinated by a
// scheduler. Before each node, the scheduler picks the walker. The
// observer (if non-nil) receives events for the full walk lifecycle.
// MaxSteps > 0 provides defense-in-depth against infinite loops.
//
// When both a graph-level observer (WithObserver) and team.Observer are set,
// events are fanned out to both via MultiObserver.
func (g *DefaultGraph) WalkTeam(ctx context.Context, team *Team, startNode string) error {
	obs := composeObservers(g.observer, team.Observer)

	node, ok := g.nodeIndex[startNode]
	if !ok {
		emitEvent(obs, WalkEvent{Type: EventWalkError, Node: startNode, Error: fmt.Errorf("%w: start node %q", ErrNodeNotFound, startNode)})
		return fmt.Errorf("%w: start node %q", ErrNodeNotFound, startNode)
	}

	if len(team.Walkers) == 0 {
		return fmt.Errorf("team has no walkers")
	}

	var priorWalker Walker
	var priorArtifact Artifact
	steps := 0

	for {
		if err := ctx.Err(); err != nil {
			emitEvent(obs, WalkEvent{Type: EventWalkError, Error: err})
			return err
		}

		if team.MaxSteps > 0 && steps >= team.MaxSteps {
			err := fmt.Errorf("max steps (%d) exceeded at node %q", team.MaxSteps, node.Name())
			emitEvent(obs, WalkEvent{Type: EventWalkError, Node: node.Name(), Error: err})
			return err
		}

		zone := zoneForNode(node.Name(), g.zones)
		walker := team.Scheduler.Select(SchedulerContext{
			Node:        node,
			Zone:        zone,
			Walkers:     team.Walkers,
			PriorWalker: priorWalker,
		})

		if priorWalker == nil || walker.Identity().PersonaName != priorWalker.Identity().PersonaName {
			emitEvent(obs, WalkEvent{
				Type:   EventWalkerSwitch,
				Node:   node.Name(),
				Walker: walker.Identity().PersonaName,
			})
		}

		emitEvent(obs, WalkEvent{Type: EventNodeEnter, Node: node.Name(), Walker: walker.Identity().PersonaName})
		nodeStart := time.Now()

		state := walker.State()
		state.CurrentNode = node.Name()

		nc := NodeContext{
			WalkerState:   state,
			PriorArtifact: priorArtifact,
			Meta:          make(map[string]any),
		}

		artifact, err := walker.Handle(ctx, node, nc)
		nodeElapsed := time.Since(nodeStart)

		if err != nil {
			state.Status = "error"
			emitEvent(obs, WalkEvent{
				Type:    EventNodeExit,
				Node:    node.Name(),
				Walker:  walker.Identity().PersonaName,
				Elapsed: nodeElapsed,
				Error:   err,
			})
			emitEvent(obs, WalkEvent{Type: EventWalkError, Node: node.Name(), Error: err})
			return fmt.Errorf("node %s: %w", node.Name(), err)
		}

		emitEvent(obs, WalkEvent{
			Type:     EventNodeExit,
			Node:     node.Name(),
			Walker:   walker.Identity().PersonaName,
			Artifact: artifact,
			Elapsed:  nodeElapsed,
		})

		edges := g.EdgesFrom(node.Name())
		if len(edges) == 0 {
			state.Status = "done"
			emitEvent(obs, WalkEvent{Type: EventWalkComplete, Node: node.Name(), Walker: walker.Identity().PersonaName})
			return nil
		}

		var matched *Transition
		var matchedEdge Edge
		for _, e := range edges {
			emitEvent(obs, WalkEvent{Type: EventEdgeEvaluate, Node: node.Name(), Edge: e.ID()})
			t := e.Evaluate(artifact, state)
			if t != nil {
				matched = t
				matchedEdge = e
				break
			}
		}

		if matched == nil {
			state.Status = "error"
			err := fmt.Errorf("%w: node %q, artifact type %q", ErrNoEdge, node.Name(), artifact.Type())
			emitEvent(obs, WalkEvent{Type: EventWalkError, Node: node.Name(), Error: err})
			return err
		}

		emitEvent(obs, WalkEvent{Type: EventTransition, Node: node.Name(), Edge: matchedEdge.ID()})

		state.RecordStep(node.Name(), matchedEdge.ID(), matchedEdge.ID(), time.Now().UTC().Format(time.RFC3339))
		state.MergeContext(matched.ContextAdditions)

		if matched.NextNode == g.doneNode {
			state.Status = "done"
			emitEvent(obs, WalkEvent{Type: EventWalkComplete, Walker: walker.Identity().PersonaName})
			return nil
		}

		nextNode, ok := g.nodeIndex[matched.NextNode]
		if !ok {
			state.Status = "error"
			err := fmt.Errorf("%w: transition target %q from edge %s", ErrNodeNotFound, matched.NextNode, matchedEdge.ID())
			emitEvent(obs, WalkEvent{Type: EventWalkError, Error: err})
			return err
		}

		priorArtifact = artifact
		priorWalker = walker
		node = nextNode
		steps++
	}
}

// composeObservers returns a single observer from two possibly-nil observers.
func composeObservers(a, b WalkObserver) WalkObserver {
	if a == nil {
		return b
	}
	if b == nil {
		return a
	}
	return MultiObserver{a, b}
}
