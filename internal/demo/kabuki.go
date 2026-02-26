package demo

import "github.com/dpopsuev/origami/kami"

// PoliceStationKabuki implements kami.KabukiConfig with Asterisk's
// Police Station content — each section tells the story of AI-driven
// root-cause analysis through a crime investigation metaphor.
type PoliceStationKabuki struct{}

var _ kami.KabukiConfig = (*PoliceStationKabuki)(nil)

func (PoliceStationKabuki) Hero() *kami.HeroSection {
	return &kami.HeroSection{
		Title:     "Asterisk",
		Subtitle:  "AI-Driven Root-Cause Analysis for CI Failures",
		Presenter: "Asterisk Police Department",
		Framework: "Origami",
	}
}

func (PoliceStationKabuki) Problem() *kami.ProblemSection {
	return &kami.ProblemSection{
		Title:     "Crimes Against CI",
		Narrative: "Every failed CI pipeline is a crime scene. Manual triage burns hours, root causes hide in logs nobody reads, and the same failures haunt teams for weeks. The culprits? Flaky tests, silent infrastructure regressions, and code changes that break things three repos away.",
		BulletPoints: []string{
			"Manual RCA takes 30-90 minutes per failure",
			"70% of CI failures share root causes with previous incidents",
			"Cross-repo correlations are invisible to single-repo investigators",
			"Evidence degrades as logs rotate and pipelines are re-triggered",
		},
		Stat:      "83%",
		StatLabel: "accuracy on blind evaluation (M19, 18 verified cases)",
	}
}

func (PoliceStationKabuki) Results() *kami.ResultsSection {
	return &kami.ResultsSection{
		Title:       "Case Closed: Calibration Results",
		Description: "Blind evaluation against Jira-verified ground truth (ptp-real-ingest scenario, 18 cases)",
		Metrics: []kami.Metric{
			{Label: "M19 Accuracy", Value: 0.83, Color: "#ee0000"},
			{Label: "M1 Recall", Value: 1.00, Color: "#06c"},
			{Label: "M2 Classification", Value: 1.00, Color: "#06c"},
			{Label: "M15 Evidence Quality", Value: 0.72, Color: "#f0ab00"},
			{Label: "M9 Repo Selection", Value: 1.00, Color: "#06c"},
			{Label: "M10 Component ID", Value: 1.00, Color: "#06c"},
		},
		Summary: []kami.SummaryCard{
			{Value: "19/21", Label: "Metrics passing", Color: "#3e8635"},
			{Value: "0.83", Label: "M19 (BasicAdapter)", Color: "#ee0000"},
			{Value: "18", Label: "Verified cases", Color: "#06c"},
		},
	}
}

func (PoliceStationKabuki) Competitive() []kami.Competitor {
	return []kami.Competitor{
		{
			Name: "Asterisk + Origami",
			Fields: map[string]string{
				"Architecture":  "Graph-based agentic pipeline",
				"Orchestration": "Declarative YAML DSL",
				"Agents":        "Persona + Element identity system",
				"Introspection": "Adversarial Dialectic (D0-D4)",
				"Debugger":      "Kami live debugger + Kabuki presentation",
				"Calibration":   "30-case blind eval, 21 metrics",
			},
			Highlight: true,
		},
		{
			Name: "CrewAI",
			Fields: map[string]string{
				"Architecture":  "Crew/Flow pattern",
				"Orchestration": "Python decorators",
				"Agents":        "Role + Goal strings",
				"Introspection": "None",
				"Debugger":      "AgentOps (external)",
				"Calibration":   "No built-in",
			},
		},
		{
			Name: "LangGraph",
			Fields: map[string]string{
				"Architecture":  "State machine graph",
				"Orchestration": "Python API",
				"Agents":        "Flat tool-calling agents",
				"Introspection": "None",
				"Debugger":      "LangSmith (external SaaS)",
				"Calibration":   "No built-in",
			},
		},
	}
}

func (PoliceStationKabuki) Architecture() *kami.ArchitectureSection {
	return &kami.ArchitectureSection{
		Title: "Precinct Architecture",
		Components: []kami.ArchComponent{
			{Name: "Recall", Description: "Check case history — have we seen this crime before?", Color: "#06c"},
			{Name: "Triage", Description: "Classify the incident — felony, misdemeanor, or false alarm", Color: "#06c"},
			{Name: "Resolve", Description: "Assign jurisdiction — which repo precinct handles this?", Color: "#f0ab00"},
			{Name: "Investigate", Description: "Crime scene analysis — logs, commits, pipelines", Color: "#ee0000"},
			{Name: "Correlate", Description: "Cross-reference the open case board", Color: "#f0ab00"},
			{Name: "Review", Description: "Evidence review — does the case hold up?", Color: "#3e8635"},
			{Name: "Report", Description: "File the final RCA with confidence score", Color: "#3e8635"},
		},
		Footer: "7 nodes • 3 zones (Backcourt, Frontcourt, Paint) • 17 edges with expression-driven routing",
	}
}

func (PoliceStationKabuki) Roadmap() []kami.Milestone {
	return []kami.Milestone{
		{ID: "S1", Label: "Foundation — consumer ergonomics, walker experience", Status: "done"},
		{ID: "S2", Label: "Ouroboros — seed pipelines, meta-calibration", Status: "done"},
		{ID: "S3", Label: "Kami — live agentic debugger (MCP + SSE + WS)", Status: "done"},
		{ID: "S4", Label: "Kabuki — presentation engine", Status: "done"},
		{ID: "S5", Label: "Demo — Police Station showcase (you are here)", Status: "current"},
		{ID: "S6", Label: "LSP — Language Server for pipeline YAML", Status: "future"},
	}
}

func (PoliceStationKabuki) Closing() *kami.ClosingSection {
	return &kami.ClosingSection{
		Headline: "Case Closed.",
		Tagline:  "Asterisk: because CI failures deserve a real investigation.",
		Lines: []string{
			"Graph-based agentic pipeline for root-cause analysis",
			"Powered by Origami — the engine under the hood",
			"83% accuracy on blind evaluation, 19/21 metrics passing",
		},
	}
}

func (PoliceStationKabuki) TransitionLine() string {
	return "Time to investigate some crimes against CI."
}
