package demo

import (
	"testing"

	"github.com/dpopsuev/origami/kami"
)

func TestPoliceStationKabuki_ImplementsInterface(t *testing.T) {
	var _ kami.KabukiConfig = PoliceStationKabuki{}
}

func TestPoliceStationKabuki_Hero(t *testing.T) {
	k := PoliceStationKabuki{}
	h := k.Hero()
	if h == nil {
		t.Fatal("Hero() returned nil")
	}
	if h.Title == "" {
		t.Error("Hero.Title is empty")
	}
	if h.Subtitle == "" {
		t.Error("Hero.Subtitle is empty")
	}
}

func TestPoliceStationKabuki_Problem(t *testing.T) {
	k := PoliceStationKabuki{}
	p := k.Problem()
	if p == nil {
		t.Fatal("Problem() returned nil")
	}
	if p.Title == "" {
		t.Error("Problem.Title is empty")
	}
	if len(p.BulletPoints) == 0 {
		t.Error("Problem.BulletPoints is empty")
	}
}

func TestPoliceStationKabuki_Results(t *testing.T) {
	k := PoliceStationKabuki{}
	r := k.Results()
	if r == nil {
		t.Fatal("Results() returned nil")
	}
	if len(r.Metrics) == 0 {
		t.Error("Results.Metrics is empty")
	}
	for i, m := range r.Metrics {
		if m.Label == "" {
			t.Errorf("Metrics[%d].Label is empty", i)
		}
		if m.Value < 0 || m.Value > 1 {
			t.Errorf("Metrics[%d].Value = %f, want [0,1]", i, m.Value)
		}
	}
}

func TestPoliceStationKabuki_Competitive(t *testing.T) {
	k := PoliceStationKabuki{}
	c := k.Competitive()
	if len(c) < 2 {
		t.Fatalf("got %d competitors, want at least 2", len(c))
	}

	highlighted := 0
	for _, comp := range c {
		if comp.Highlight {
			highlighted++
		}
		if comp.Name == "" {
			t.Error("competitor has empty Name")
		}
	}
	if highlighted != 1 {
		t.Errorf("want exactly 1 highlighted competitor, got %d", highlighted)
	}
}

func TestPoliceStationKabuki_Architecture(t *testing.T) {
	k := PoliceStationKabuki{}
	a := k.Architecture()
	if a == nil {
		t.Fatal("Architecture() returned nil")
	}
	if len(a.Components) != 7 {
		t.Errorf("got %d components, want 7 (one per pipeline node)", len(a.Components))
	}
}

func TestPoliceStationKabuki_Roadmap(t *testing.T) {
	k := PoliceStationKabuki{}
	r := k.Roadmap()
	if len(r) == 0 {
		t.Fatal("Roadmap() returned empty")
	}

	hasCurrent := false
	for _, m := range r {
		if m.Status == "current" {
			hasCurrent = true
		}
	}
	if !hasCurrent {
		t.Error("Roadmap has no milestone with status 'current'")
	}
}

func TestPoliceStationKabuki_Closing(t *testing.T) {
	k := PoliceStationKabuki{}
	c := k.Closing()
	if c == nil {
		t.Fatal("Closing() returned nil")
	}
	if c.Headline == "" {
		t.Error("Closing.Headline is empty")
	}
}

func TestPoliceStationKabuki_TransitionLine(t *testing.T) {
	k := PoliceStationKabuki{}
	if line := k.TransitionLine(); line == "" {
		t.Error("TransitionLine() returned empty")
	}
}
