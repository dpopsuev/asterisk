package main

import "github.com/dpopsuev/origami/kami"

// PoliceStationTheme implements kami.Theme with Asterisk's police
// station personality — each agent is a detective archetype, each
// circuit node is an investigative procedure.
type PoliceStationTheme struct{}

var _ kami.Theme = (*PoliceStationTheme)(nil)

func (PoliceStationTheme) Name() string { return "Asterisk Police Station" }

func (PoliceStationTheme) AgentIntros() []kami.AgentIntro {
	return []kami.AgentIntro{
		{
			PersonaName: "Herald",
			Element:     "Fire",
			Role:        "Lead Detective / Fast-path Classifier",
			Catchphrase: "I saw the error. I already know what happened. You're welcome.",
		},
		{
			PersonaName: "Seeker",
			Element:     "Water",
			Role:        "Forensic Analyst / Deep Evidence Gatherer",
			Catchphrase: "Let's not jump to conclusions. I'd like to examine all 47 log files first.",
		},
		{
			PersonaName: "Sentinel",
			Element:     "Earth",
			Role:        "Desk Sergeant / Infrastructure Specialist",
			Catchphrase: "I've filed this under 'infrastructure.' Next case.",
		},
		{
			PersonaName: "Weaver",
			Element:     "Air",
			Role:        "Undercover Agent / Cross-repo Correlator",
			Catchphrase: "What if the bug isn't in the code? What if it's in the *process*?",
		},
		{
			PersonaName: "Arbiter",
			Element:     "Diamond",
			Role:        "Internal Affairs / Adversarial Reviewer",
			Catchphrase: "The evidence is inconclusive. I'm reopening the investigation.",
		},
		{
			PersonaName: "Catalyst",
			Element:     "Lightning",
			Role:        "Dispatch / Circuit Orchestrator",
			Catchphrase: "New failure incoming! All units respond!",
		},
	}
}

func (PoliceStationTheme) NodeDescriptions() map[string]string {
	return map[string]string{
		"recall":      "Witness Interview / Historical Failure Lookup — checking if we've seen this crime before",
		"triage":      "Case Classification / Defect Type Classification — felony, misdemeanor, or false alarm?",
		"resolve":     "Jurisdiction Check / Repository Selection — which precinct handles this repo?",
		"investigate": "Crime Scene Analysis / Evidence Gathering — logs, commits, and circuits",
		"correlate":   "Cross-Reference / Failure Pattern Correlation — matching against the open case board",
		"review":      "Evidence Review / Confidence Scoring — does the case hold up under scrutiny?",
		"report":      "Case Report / RCA Verdict — filing the final analysis with evidence chain",
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
