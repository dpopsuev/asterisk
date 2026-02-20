package framework

import (
	"context"
	"errors"
	"testing"
)

// --- test helpers ---

type stubNode struct {
	name    string
	element Element
	artifact Artifact
	err     error
}

func (n *stubNode) Name() string              { return n.name }
func (n *stubNode) ElementAffinity() Element   { return n.element }
func (n *stubNode) Process(ctx context.Context, nc NodeContext) (Artifact, error) {
	return n.artifact, n.err
}

type stubArtifact struct {
	typ        string
	confidence float64
	raw        any
}

func (a *stubArtifact) Type() string       { return a.typ }
func (a *stubArtifact) Confidence() float64 { return a.confidence }
func (a *stubArtifact) Raw() any           { return a.raw }

type stubEdge struct {
	id, from, to string
	shortcut     bool
	loop         bool
	evalFn       func(Artifact, *WalkerState) *Transition
}

func (e *stubEdge) ID() string       { return e.id }
func (e *stubEdge) From() string     { return e.from }
func (e *stubEdge) To() string       { return e.to }
func (e *stubEdge) IsShortcut() bool { return e.shortcut }
func (e *stubEdge) IsLoop() bool     { return e.loop }
func (e *stubEdge) Evaluate(a Artifact, s *WalkerState) *Transition {
	if e.evalFn != nil {
		return e.evalFn(a, s)
	}
	return &Transition{NextNode: e.to, Explanation: e.id + " matched"}
}

type stubWalker struct {
	identity AgentIdentity
	state    *WalkerState
	visited  []string
}

func (w *stubWalker) Identity() AgentIdentity { return w.identity }
func (w *stubWalker) State() *WalkerState     { return w.state }
func (w *stubWalker) Handle(ctx context.Context, node Node, nc NodeContext) (Artifact, error) {
	w.visited = append(w.visited, node.Name())
	return node.Process(ctx, nc)
}

// --- tests ---

func TestGraph_LinearWalk(t *testing.T) {
	nodeA := &stubNode{name: "A", artifact: &stubArtifact{typ: "a", confidence: 1.0}}
	nodeB := &stubNode{name: "B", artifact: &stubArtifact{typ: "b", confidence: 1.0}}
	nodeC := &stubNode{name: "C", artifact: &stubArtifact{typ: "c", confidence: 1.0}}

	edges := []Edge{
		&stubEdge{id: "E1", from: "A", to: "B"},
		&stubEdge{id: "E2", from: "B", to: "C"},
		&stubEdge{id: "E3", from: "C", to: "_done"},
	}

	g, err := NewGraph("test", []Node{nodeA, nodeB, nodeC}, edges, nil)
	if err != nil {
		t.Fatal(err)
	}

	w := &stubWalker{state: NewWalkerState("case-1")}
	if err := g.Walk(context.Background(), w, "A"); err != nil {
		t.Fatalf("Walk failed: %v", err)
	}

	if w.state.Status != "done" {
		t.Errorf("expected status done, got %s", w.state.Status)
	}
	if len(w.visited) != 3 {
		t.Errorf("expected 3 visited nodes, got %d: %v", len(w.visited), w.visited)
	}
	want := []string{"A", "B", "C"}
	for i, v := range want {
		if w.visited[i] != v {
			t.Errorf("visited[%d] = %s, want %s", i, w.visited[i], v)
		}
	}
	if len(w.state.History) != 3 {
		t.Errorf("expected 3 history entries, got %d", len(w.state.History))
	}
}

func TestGraph_ShortcutEdge(t *testing.T) {
	nodeA := &stubNode{name: "A", artifact: &stubArtifact{typ: "a", confidence: 0.95}}
	nodeB := &stubNode{name: "B", artifact: &stubArtifact{typ: "b", confidence: 1.0}}
	nodeC := &stubNode{name: "C", artifact: &stubArtifact{typ: "c", confidence: 1.0}}

	edges := []Edge{
		&stubEdge{
			id: "shortcut", from: "A", to: "C", shortcut: true,
			evalFn: func(a Artifact, _ *WalkerState) *Transition {
				if a.Confidence() >= 0.9 {
					return &Transition{NextNode: "C", Explanation: "high confidence shortcut"}
				}
				return nil
			},
		},
		&stubEdge{id: "normal", from: "A", to: "B"},
		&stubEdge{id: "B-to-C", from: "B", to: "C"},
		&stubEdge{id: "C-done", from: "C", to: "_done"},
	}

	g, err := NewGraph("test", []Node{nodeA, nodeB, nodeC}, edges, nil)
	if err != nil {
		t.Fatal(err)
	}

	w := &stubWalker{state: NewWalkerState("case-2")}
	if err := g.Walk(context.Background(), w, "A"); err != nil {
		t.Fatalf("Walk failed: %v", err)
	}

	if len(w.visited) != 2 {
		t.Errorf("expected 2 visited nodes (A, C), got %d: %v", len(w.visited), w.visited)
	}
	if w.visited[0] != "A" || w.visited[1] != "C" {
		t.Errorf("expected [A, C], got %v", w.visited)
	}
}

func TestGraph_LoopEdge(t *testing.T) {
	callCount := 0
	nodeA := &stubNode{name: "A", artifact: &stubArtifact{typ: "a", confidence: 1.0}}
	nodeB := &stubNode{
		name: "B",
		artifact: &stubArtifact{typ: "b", confidence: 0.5},
	}
	nodeB.artifact = &stubArtifact{typ: "b", confidence: 0.5}

	maxLoops := 2
	edges := []Edge{
		&stubEdge{id: "A-B", from: "A", to: "B"},
		&stubEdge{
			id: "B-loop", from: "B", to: "B", loop: true,
			evalFn: func(a Artifact, s *WalkerState) *Transition {
				callCount++
				if s.LoopCounts["B-loop"] < maxLoops {
					s.IncrementLoop("B-loop")
					return &Transition{NextNode: "B", Explanation: "loop again"}
				}
				return nil
			},
		},
		&stubEdge{id: "B-done", from: "B", to: "_done"},
	}

	g, err := NewGraph("test", []Node{nodeA, nodeB}, edges, nil)
	if err != nil {
		t.Fatal(err)
	}

	w := &stubWalker{state: NewWalkerState("case-3")}
	if err := g.Walk(context.Background(), w, "A"); err != nil {
		t.Fatalf("Walk failed: %v", err)
	}

	// A visited once, B visited 1 (initial) + 2 (loops) = 3
	if len(w.visited) != 4 {
		t.Errorf("expected 4 visits (A + B*3), got %d: %v", len(w.visited), w.visited)
	}
	if w.state.LoopCounts["B-loop"] != maxLoops {
		t.Errorf("expected loop count %d, got %d", maxLoops, w.state.LoopCounts["B-loop"])
	}
}

func TestGraph_ErrNodeNotFound_StartNode(t *testing.T) {
	g, err := NewGraph("test", nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	w := &stubWalker{state: NewWalkerState("case-4")}
	err = g.Walk(context.Background(), w, "nonexistent")
	if !errors.Is(err, ErrNodeNotFound) {
		t.Errorf("expected ErrNodeNotFound, got %v", err)
	}
}

func TestGraph_ErrNodeNotFound_EdgeTarget(t *testing.T) {
	nodeA := &stubNode{name: "A", artifact: &stubArtifact{typ: "a"}}
	edges := []Edge{
		&stubEdge{id: "bad", from: "A", to: "Z"},
	}

	_, err := NewGraph("test", []Node{nodeA}, edges, nil)
	if !errors.Is(err, ErrNodeNotFound) {
		t.Errorf("expected ErrNodeNotFound during construction, got %v", err)
	}
}

func TestGraph_ErrNoEdge(t *testing.T) {
	nodeA := &stubNode{name: "A", artifact: &stubArtifact{typ: "a"}}
	nodeB := &stubNode{name: "B", artifact: &stubArtifact{typ: "b"}}

	edges := []Edge{
		&stubEdge{id: "A-B", from: "A", to: "B"},
		&stubEdge{
			id: "B-never", from: "B", to: "_done",
			evalFn: func(Artifact, *WalkerState) *Transition { return nil },
		},
	}

	g, err := NewGraph("test", []Node{nodeA, nodeB}, edges, nil)
	if err != nil {
		t.Fatal(err)
	}

	w := &stubWalker{state: NewWalkerState("case-5")}
	err = g.Walk(context.Background(), w, "A")
	if !errors.Is(err, ErrNoEdge) {
		t.Errorf("expected ErrNoEdge, got %v", err)
	}
	if w.state.Status != "error" {
		t.Errorf("expected status error, got %s", w.state.Status)
	}
}

func TestGraph_TerminalNodeNoEdges(t *testing.T) {
	nodeA := &stubNode{name: "A", artifact: &stubArtifact{typ: "a"}}

	g, err := NewGraph("test", []Node{nodeA}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	w := &stubWalker{state: NewWalkerState("case-6")}
	err = g.Walk(context.Background(), w, "A")
	if err != nil {
		t.Fatalf("expected nil error for terminal node, got %v", err)
	}
	if w.state.Status != "done" {
		t.Errorf("expected status done, got %s", w.state.Status)
	}
}

func TestGraph_ContextCancellation(t *testing.T) {
	nodeA := &stubNode{name: "A", artifact: &stubArtifact{typ: "a"}}
	nodeB := &stubNode{name: "B", artifact: &stubArtifact{typ: "b"}}
	edges := []Edge{
		&stubEdge{id: "A-B", from: "A", to: "B"},
		&stubEdge{id: "B-done", from: "B", to: "_done"},
	}

	g, err := NewGraph("test", []Node{nodeA, nodeB}, edges, nil)
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	w := &stubWalker{state: NewWalkerState("case-7")}
	err = g.Walk(ctx, w, "A")
	if err == nil {
		t.Fatal("expected error from cancelled context")
	}
	if w.state.Status != "error" {
		t.Errorf("expected status error, got %s", w.state.Status)
	}
}

func TestGraph_Zones(t *testing.T) {
	nodeA := &stubNode{name: "A"}
	nodeB := &stubNode{name: "B"}
	nodeC := &stubNode{name: "C"}

	zones := []Zone{
		{Name: "front", NodeNames: []string{"A", "B"}, ElementAffinity: "fire", Stickiness: 0},
		{Name: "back", NodeNames: []string{"C"}, ElementAffinity: "water", Stickiness: 3},
	}

	g, err := NewGraph("test", []Node{nodeA, nodeB, nodeC}, nil, zones)
	if err != nil {
		t.Fatal(err)
	}

	if len(g.Zones()) != 2 {
		t.Errorf("expected 2 zones, got %d", len(g.Zones()))
	}
	if g.Zones()[0].Name != "front" {
		t.Errorf("expected zone 'front', got %q", g.Zones()[0].Name)
	}
}

func TestGraph_ContextAdditions(t *testing.T) {
	nodeA := &stubNode{name: "A", artifact: &stubArtifact{typ: "a"}}
	nodeB := &stubNode{name: "B", artifact: &stubArtifact{typ: "b"}}

	edges := []Edge{
		&stubEdge{
			id: "A-B", from: "A", to: "B",
			evalFn: func(Artifact, *WalkerState) *Transition {
				return &Transition{
					NextNode:         "B",
					ContextAdditions: map[string]any{"key": "value"},
					Explanation:      "with context",
				}
			},
		},
		&stubEdge{id: "B-done", from: "B", to: "_done"},
	}

	g, err := NewGraph("test", []Node{nodeA, nodeB}, edges, nil)
	if err != nil {
		t.Fatal(err)
	}

	w := &stubWalker{state: NewWalkerState("case-8")}
	if err := g.Walk(context.Background(), w, "A"); err != nil {
		t.Fatal(err)
	}

	if v, ok := w.state.Context["key"]; !ok || v != "value" {
		t.Errorf("expected context key=value, got %v", w.state.Context)
	}
}
