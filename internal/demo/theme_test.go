package demo

import (
	"testing"

	"github.com/dpopsuev/origami/kami"
)

func TestPoliceStationTheme_ImplementsInterface(t *testing.T) {
	var _ kami.Theme = PoliceStationTheme{}
}

func TestPoliceStationTheme_AllNodesHaveDescriptions(t *testing.T) {
	theme := PoliceStationTheme{}
	descs := theme.NodeDescriptions()

	pipelineNodes := []string{"recall", "triage", "resolve", "investigate", "correlate", "review", "report"}
	for _, node := range pipelineNodes {
		if _, ok := descs[node]; !ok {
			t.Errorf("missing description for node %q", node)
		}
	}
}

func TestPoliceStationTheme_AllPersonasHaveIntros(t *testing.T) {
	theme := PoliceStationTheme{}
	intros := theme.AgentIntros()

	if len(intros) < 6 {
		t.Errorf("got %d intros, want at least 6", len(intros))
	}

	for _, intro := range intros {
		if intro.PersonaName == "" {
			t.Error("intro has empty PersonaName")
		}
		if intro.Catchphrase == "" {
			t.Errorf("persona %q has empty Catchphrase", intro.PersonaName)
		}
	}
}

func TestPoliceStationTheme_CooperationDialogs(t *testing.T) {
	theme := PoliceStationTheme{}
	dialogs := theme.CooperationDialogs()

	if len(dialogs) < 4 {
		t.Errorf("got %d dialogs, want at least 4", len(dialogs))
	}

	for i, d := range dialogs {
		if d.From == "" || d.To == "" || d.Message == "" {
			t.Errorf("dialog[%d] has empty field: from=%q, to=%q, message=%q", i, d.From, d.To, d.Message)
		}
	}
}

func TestPoliceStationTheme_Name(t *testing.T) {
	theme := PoliceStationTheme{}
	if name := theme.Name(); name == "" {
		t.Error("Name() returned empty string")
	}
}
