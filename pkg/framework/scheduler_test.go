package framework

import (
	"context"
	"testing"
)

type affinityWalker struct {
	identity AgentIdentity
	state    *WalkerState
}

func (w *affinityWalker) Identity() AgentIdentity { return w.identity }
func (w *affinityWalker) State() *WalkerState     { return w.state }
func (w *affinityWalker) Handle(_ context.Context, node Node, nc NodeContext) (Artifact, error) {
	return node.Process(context.Background(), nc)
}

func TestSingleScheduler(t *testing.T) {
	w := &affinityWalker{identity: AgentIdentity{PersonaName: "solo"}, state: NewWalkerState("s1")}
	sched := &SingleScheduler{Walker: w}

	node := &stubNode{name: "classify", element: ElementFire}
	got := sched.Select(SchedulerContext{Node: node, Walkers: []Walker{w}})
	if got.Identity().PersonaName != "solo" {
		t.Errorf("expected solo, got %s", got.Identity().PersonaName)
	}
}

func TestAffinityScheduler_PicksHighestAffinity(t *testing.T) {
	herald := &affinityWalker{
		identity: AgentIdentity{
			PersonaName:  "Herald",
			Element:      ElementFire,
			StepAffinity: map[string]float64{"classify": 0.9, "investigate": 0.2},
		},
		state: NewWalkerState("h1"),
	}
	seeker := &affinityWalker{
		identity: AgentIdentity{
			PersonaName:  "Seeker",
			Element:      ElementWater,
			StepAffinity: map[string]float64{"classify": 0.2, "investigate": 0.9},
		},
		state: NewWalkerState("s1"),
	}

	sched := &AffinityScheduler{}
	walkers := []Walker{herald, seeker}

	classifyNode := &stubNode{name: "classify", element: ElementFire}
	got := sched.Select(SchedulerContext{Node: classifyNode, Walkers: walkers})
	if got.Identity().PersonaName != "Herald" {
		t.Errorf("classify: expected Herald, got %s", got.Identity().PersonaName)
	}

	investigateNode := &stubNode{name: "investigate", element: ElementWater}
	got = sched.Select(SchedulerContext{Node: investigateNode, Walkers: walkers})
	if got.Identity().PersonaName != "Seeker" {
		t.Errorf("investigate: expected Seeker, got %s", got.Identity().PersonaName)
	}
}

func TestAffinityScheduler_TieBreakByElement(t *testing.T) {
	fire := &affinityWalker{
		identity: AgentIdentity{
			PersonaName:  "Fire",
			Element:      ElementFire,
			StepAffinity: map[string]float64{"node": 0.5},
		},
		state: NewWalkerState("f1"),
	}
	water := &affinityWalker{
		identity: AgentIdentity{
			PersonaName:  "Water",
			Element:      ElementWater,
			StepAffinity: map[string]float64{"node": 0.5},
		},
		state: NewWalkerState("w1"),
	}

	sched := &AffinityScheduler{}
	fireNode := &stubNode{name: "node", element: ElementFire}

	got := sched.Select(SchedulerContext{Node: fireNode, Walkers: []Walker{water, fire}})
	if got.Identity().PersonaName != "Fire" {
		t.Errorf("expected Fire (element tiebreak), got %s", got.Identity().PersonaName)
	}
}

func TestAffinityScheduler_FallbackToFirst(t *testing.T) {
	w1 := &affinityWalker{
		identity: AgentIdentity{PersonaName: "First"},
		state:    NewWalkerState("1"),
	}
	w2 := &affinityWalker{
		identity: AgentIdentity{PersonaName: "Second"},
		state:    NewWalkerState("2"),
	}

	sched := &AffinityScheduler{}
	node := &stubNode{name: "unknown"}

	got := sched.Select(SchedulerContext{Node: node, Walkers: []Walker{w1, w2}})
	if got.Identity().PersonaName != "First" {
		t.Errorf("expected First (fallback), got %s", got.Identity().PersonaName)
	}
}

func TestAffinityScheduler_SingleWalker(t *testing.T) {
	w := &affinityWalker{
		identity: AgentIdentity{PersonaName: "Only"},
		state:    NewWalkerState("o1"),
	}

	sched := &AffinityScheduler{}
	got := sched.Select(SchedulerContext{Node: &stubNode{name: "x"}, Walkers: []Walker{w}})
	if got.Identity().PersonaName != "Only" {
		t.Errorf("expected Only, got %s", got.Identity().PersonaName)
	}
}

func TestAffinityScheduler_EmptyWalkers(t *testing.T) {
	sched := &AffinityScheduler{}
	got := sched.Select(SchedulerContext{Node: &stubNode{name: "x"}, Walkers: nil})
	if got != nil {
		t.Errorf("expected nil for empty walkers, got %v", got)
	}
}

func TestZoneForNode(t *testing.T) {
	zones := []Zone{
		{Name: "front", NodeNames: []string{"A", "B"}},
		{Name: "back", NodeNames: []string{"C"}},
	}

	z := zoneForNode("B", zones)
	if z == nil || z.Name != "front" {
		t.Errorf("expected zone 'front' for node B, got %v", z)
	}

	z = zoneForNode("C", zones)
	if z == nil || z.Name != "back" {
		t.Errorf("expected zone 'back' for node C, got %v", z)
	}

	z = zoneForNode("Z", zones)
	if z != nil {
		t.Errorf("expected nil for unknown node Z, got %v", z)
	}
}
