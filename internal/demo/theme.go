// Package demo provides the Asterisk presentation theme for Kami.
//
// The Police Station theme maps Asterisk's RCA pipeline to a crime
// investigation metaphor. Agents are detectives with distinct
// personalities investigating "crimes against CI."
package demo

import "github.com/dpopsuev/origami/kami"

// PoliceStationTheme implements kami.Theme with Asterisk's police
// station personality — each agent is a detective archetype, each
// pipeline node is an investigative procedure.
type PoliceStationTheme struct{}

var _ kami.Theme = (*PoliceStationTheme)(nil)

func (PoliceStationTheme) Name() string { return "Asterisk Police Station" }

func (PoliceStationTheme) AgentIntros() []kami.AgentIntro {
	return []kami.AgentIntro{
		{
			PersonaName: "Herald",
			Element:     "Fire",
			Role:        "Lead Detective",
			Catchphrase: "I saw the error. I already know what happened. You're welcome.",
		},
		{
			PersonaName: "Seeker",
			Element:     "Water",
			Role:        "Forensic Analyst",
			Catchphrase: "Let's not jump to conclusions. I'd like to examine all 47 log files first.",
		},
		{
			PersonaName: "Sentinel",
			Element:     "Earth",
			Role:        "Desk Sergeant",
			Catchphrase: "I've filed this under 'infrastructure.' Next case.",
		},
		{
			PersonaName: "Weaver",
			Element:     "Air",
			Role:        "Undercover Agent",
			Catchphrase: "What if the bug isn't in the code? What if it's in the *process*?",
		},
		{
			PersonaName: "Arbiter",
			Element:     "Diamond",
			Role:        "Internal Affairs",
			Catchphrase: "The evidence is inconclusive. I'm reopening the investigation.",
		},
		{
			PersonaName: "Catalyst",
			Element:     "Lightning",
			Role:        "Dispatch",
			Catchphrase: "New failure incoming! All units respond!",
		},
	}
}

func (PoliceStationTheme) NodeDescriptions() map[string]string {
	return map[string]string{
		"recall":      "Witness Interview — checking if we've seen this crime before",
		"triage":      "Case Classification — is this a felony, misdemeanor, or false alarm?",
		"resolve":     "Jurisdiction Check — which precinct handles this repo?",
		"investigate": "Crime Scene Analysis — gathering evidence from logs, commits, and pipelines",
		"correlate":   "Cross-Reference — matching this case against the open case board",
		"review":      "Evidence Review — does the case file hold up under scrutiny?",
		"report":      "Case Report — filing the final RCA with evidence and confidence",
	}
}

func (PoliceStationTheme) CostumeAssets() map[string]string {
	return map[string]string{
		"hat":   "police-hat",
		"badge": "detective-badge",
		"icon":  "magnifying-glass",
	}
}

func (PoliceStationTheme) CooperationDialogs() []kami.Dialog {
	return []kami.Dialog{
		{From: "Herald", To: "Seeker", Message: "I already solved it. The test is flaky."},
		{From: "Seeker", To: "Herald", Message: "You haven't even read the logs yet."},
		{From: "Sentinel", To: "Weaver", Message: "Just file it under infrastructure and move on."},
		{From: "Weaver", To: "Sentinel", Message: "But what if the infra failure is *caused* by a code change?"},
		{From: "Arbiter", To: "Herald", Message: "Your confidence score is 0.42. That's not a conviction, that's a hunch."},
		{From: "Herald", To: "Arbiter", Message: "My hunches have a better track record than your spreadsheets."},
		{From: "Catalyst", To: "Sentinel", Message: "Three new failures just came in. All PTP operator."},
		{From: "Sentinel", To: "Catalyst", Message: "Same commit range. Batch them."},
	}
}
