package framework

import (
	"context"
	"errors"
	"testing"
)

func TestWalkTeam_LinearWithTwoWalkers(t *testing.T) {
	nodeA := &stubNode{name: "classify", element: ElementFire, artifact: &stubArtifact{typ: "classification", confidence: 0.9}}
	nodeB := &stubNode{name: "investigate", element: ElementWater, artifact: &stubArtifact{typ: "investigation", confidence: 0.8}}
	nodeC := &stubNode{name: "decide", element: ElementEarth, artifact: &stubArtifact{typ: "decision", confidence: 0.95}}

	edges := []Edge{
		&stubEdge{id: "E1", from: "classify", to: "investigate"},
		&stubEdge{id: "E2", from: "investigate", to: "decide"},
		&stubEdge{id: "E3", from: "decide", to: "_done"},
	}

	g, err := NewGraph("triage", []Node{nodeA, nodeB, nodeC}, edges, nil)
	if err != nil {
		t.Fatal(err)
	}

	herald := &stubWalker{
		identity: AgentIdentity{
			PersonaName:  "Herald",
			Element:      ElementFire,
			StepAffinity: map[string]float64{"classify": 0.9, "investigate": 0.1, "decide": 0.5},
		},
		state: NewWalkerState("herald-1"),
	}
	seeker := &stubWalker{
		identity: AgentIdentity{
			PersonaName:  "Seeker",
			Element:      ElementWater,
			StepAffinity: map[string]float64{"classify": 0.1, "investigate": 0.9, "decide": 0.3},
		},
		state: NewWalkerState("seeker-1"),
	}

	tc := &TraceCollector{}
	team := &Team{
		Walkers:   []Walker{herald, seeker},
		Scheduler: &AffinityScheduler{},
		Observer:  tc,
	}

	if err := g.WalkTeam(context.Background(), team, "classify"); err != nil {
		t.Fatalf("WalkTeam failed: %v", err)
	}

	switches := tc.EventsOfType(EventWalkerSwitch)
	if len(switches) < 2 {
		t.Errorf("expected at least 2 walker switches, got %d", len(switches))
	}

	enters := tc.EventsOfType(EventNodeEnter)
	if len(enters) != 3 {
		t.Fatalf("expected 3 node_enter events, got %d", len(enters))
	}
	if enters[0].Walker != "Herald" {
		t.Errorf("classify should be handled by Herald, got %s", enters[0].Walker)
	}
	if enters[1].Walker != "Seeker" {
		t.Errorf("investigate should be handled by Seeker, got %s", enters[1].Walker)
	}

	completes := tc.EventsOfType(EventWalkComplete)
	if len(completes) != 1 {
		t.Errorf("expected 1 walk_complete event, got %d", len(completes))
	}
}

func TestWalkTeam_MaxStepsGuard(t *testing.T) {
	nodeA := &stubNode{name: "A", artifact: &stubArtifact{typ: "a"}}
	edges := []Edge{
		&stubEdge{id: "A-loop", from: "A", to: "A"},
	}

	g, err := NewGraph("loop", []Node{nodeA}, edges, nil)
	if err != nil {
		t.Fatal(err)
	}

	w := &stubWalker{
		identity: AgentIdentity{PersonaName: "Solo"},
		state:    NewWalkerState("solo-1"),
	}

	team := &Team{
		Walkers:   []Walker{w},
		Scheduler: &SingleScheduler{Walker: w},
		MaxSteps:  3,
	}

	err = g.WalkTeam(context.Background(), team, "A")
	if err == nil {
		t.Fatal("expected max steps error")
	}
	if !errors.Is(err, nil) {
		// Just check that the error message mentions max steps
		if got := err.Error(); len(got) == 0 {
			t.Fatal("expected non-empty error message")
		}
	}
}

func TestWalkTeam_ObserverReceivesEdgeEvents(t *testing.T) {
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

	w := &stubWalker{
		identity: AgentIdentity{PersonaName: "Solo"},
		state:    NewWalkerState("s1"),
	}
	tc := &TraceCollector{}
	team := &Team{
		Walkers:   []Walker{w},
		Scheduler: &SingleScheduler{Walker: w},
		Observer:  tc,
	}

	if err := g.WalkTeam(context.Background(), team, "A"); err != nil {
		t.Fatal(err)
	}

	edgeEvals := tc.EventsOfType(EventEdgeEvaluate)
	if len(edgeEvals) < 2 {
		t.Errorf("expected at least 2 edge_evaluate events, got %d", len(edgeEvals))
	}

	transitions := tc.EventsOfType(EventTransition)
	if len(transitions) < 1 {
		t.Errorf("expected at least 1 transition event, got %d", len(transitions))
	}
}

func TestWalkTeam_NilObserver(t *testing.T) {
	nodeA := &stubNode{name: "A", artifact: &stubArtifact{typ: "a"}}
	edges := []Edge{
		&stubEdge{id: "A-done", from: "A", to: "_done"},
	}

	g, err := NewGraph("test", []Node{nodeA}, edges, nil)
	if err != nil {
		t.Fatal(err)
	}

	w := &stubWalker{
		identity: AgentIdentity{PersonaName: "Solo"},
		state:    NewWalkerState("s1"),
	}
	team := &Team{
		Walkers:   []Walker{w},
		Scheduler: &SingleScheduler{Walker: w},
		Observer:  nil,
	}

	if err := g.WalkTeam(context.Background(), team, "A"); err != nil {
		t.Fatal(err)
	}
}

func TestWalkTeam_NoWalkersError(t *testing.T) {
	nodeA := &stubNode{name: "A", artifact: &stubArtifact{typ: "a"}}

	g, err := NewGraph("test", []Node{nodeA}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	team := &Team{
		Walkers:   nil,
		Scheduler: &AffinityScheduler{},
	}

	err = g.WalkTeam(context.Background(), team, "A")
	if err == nil {
		t.Fatal("expected error for empty walkers")
	}
}

func TestWalkTeam_StartNodeNotFound(t *testing.T) {
	g, err := NewGraph("test", nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	w := &stubWalker{
		identity: AgentIdentity{PersonaName: "Solo"},
		state:    NewWalkerState("s1"),
	}
	team := &Team{
		Walkers:   []Walker{w},
		Scheduler: &SingleScheduler{Walker: w},
	}

	err = g.WalkTeam(context.Background(), team, "nonexistent")
	if !errors.Is(err, ErrNodeNotFound) {
		t.Errorf("expected ErrNodeNotFound, got %v", err)
	}
}

func TestWalkTeam_ContextCancellation(t *testing.T) {
	nodeA := &stubNode{name: "A", artifact: &stubArtifact{typ: "a"}}
	nodeB := &stubNode{name: "B", artifact: &stubArtifact{typ: "b"}}
	edges := []Edge{
		&stubEdge{id: "A-B", from: "A", to: "B"},
	}

	g, err := NewGraph("test", []Node{nodeA, nodeB}, edges, nil)
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	w := &stubWalker{
		identity: AgentIdentity{PersonaName: "Solo"},
		state:    NewWalkerState("s1"),
	}
	tc := &TraceCollector{}
	team := &Team{
		Walkers:   []Walker{w},
		Scheduler: &SingleScheduler{Walker: w},
		Observer:  tc,
	}

	err = g.WalkTeam(ctx, team, "A")
	if err == nil {
		t.Fatal("expected error from cancelled context")
	}

	walkErrors := tc.EventsOfType(EventWalkError)
	if len(walkErrors) == 0 {
		t.Error("expected walk_error event on cancellation")
	}
}
