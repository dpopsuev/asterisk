package main

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
		Title:     "Crimes Against CI / Automated Root-Cause Analysis for CI Failures",
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
		Title: "Precinct Architecture / RCA Pipeline",
		Components: []kami.ArchComponent{
			{Name: "Recall", Description: "Witness Interview / Historical Failure Lookup", Color: "#06c"},
			{Name: "Triage", Description: "Case Classification / Defect Type Classification", Color: "#06c"},
			{Name: "Resolve", Description: "Jurisdiction Check / Repository Selection", Color: "#f0ab00"},
			{Name: "Investigate", Description: "Crime Scene Analysis / Log, Commit & Pipeline Evidence Gathering", Color: "#ee0000"},
			{Name: "Correlate", Description: "Cross-Reference / Failure Pattern Correlation", Color: "#f0ab00"},
			{Name: "Review", Description: "Evidence Review / Confidence Scoring & Adversarial Verification", Color: "#3e8635"},
			{Name: "Report", Description: "Case Report / Final RCA Verdict with Evidence Chain", Color: "#3e8635"},
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

func (PoliceStationKabuki) CodeShowcases() []kami.CodeShowcase {
	return []kami.CodeShowcase{
		{
			ID:    "act2-dsl",
			Title: "Pipeline DSL / Declarative Graph Definition",
			Blocks: []kami.CodeBlock{
				{
					Language:   "yaml",
					Annotation: "7 nodes, 17 edges with expression-driven routing",
					Code: `pipeline: asterisk-rca
description: "Root-cause analysis pipeline (F0 Recall through F6 Report)"
vars:
  recall_hit: 0.80
  recall_uncertain: 0.40
  convergence_sufficient: 0.50
  max_investigate_loops: 1

nodes:
  - name: recall
    family: recall
    after: [store.recall]
  - name: triage
    family: triage
    after: [store.triage]
  - name: resolve
    family: resolve
  - name: investigate
    family: investigate
    after: [store.investigate]
  - name: correlate
    family: correlate
  - name: review
    family: review
  - name: report
    family: report`,
				},
				{
					Language:   "yaml",
					Annotation: "Edges with CEL expressions, shortcuts, and loop control",
					Code: `edges:
  - id: H1
    name: recall-hit
    from: recall
    to: review
    shortcut: true
    when: "output.match == true && output.confidence >= config.recall_hit"
  - id: H10
    name: investigate-low
    from: investigate
    to: resolve
    loop: true
    when: "output.convergence_score < config.convergence_sufficient &&
           state.loops.investigate < config.max_investigate_loops"
  - id: H12
    name: review-approve
    from: review
    to: report
    when: 'output.decision == "approve"'`,
				},
			},
		},
		{
			ID:    "act3-papercup",
			Title: "Papercup Protocol / V2 Choreography",
			Blocks: []kami.CodeBlock{
				{
					Language:   "go",
					Annotation: "Server generates worker prompts, workers loop independently",
					Code: `// Worker loop — each subagent runs independently
for {
    step := getNextStep(sessionID, preferredCaseID)
    if step.Done { break }

    // Worker produces artifact for this step
    artifact := processStep(step.Prompt, step.Schema)

    submitStep(sessionID, step.DispatchID, step.Name, artifact)
}`,
				},
				{
					Language:   "yaml",
					Annotation: "Zone stickiness routes steps to the right worker",
					Code: `# Zone stickiness levels:
# any(0)       — no preference, work stealing OK
# slight(1)    — prefer same zone, steal if idle
# strong(2)    — strongly prefer same zone
# exclusive(3) — only this zone's worker handles it`,
				},
			},
		},
		{
			ID:    "act3-skills",
			Title: "Cursor Skills / Agent-Driven Execution",
			Blocks: []kami.CodeBlock{
				{
					Language:   "yaml",
					Annotation: "asterisk-calibrate: supervisor + 4 parallel workers",
					Code: `# SKILL.md drives the agent:
# 1. Parent reads SKILL.md, starts pipeline session
# 2. Parent launches N worker subagents via Task tool
# 3. Each worker owns its get_next_step / submit_step loop
# 4. Workers self-terminate when done=true
# 5. Parent monitors via get_signals, presents report`,
				},
				{
					Language:   "yaml",
					Annotation: "asterisk-analyze: agent IS the reasoning engine",
					Code: `# Single-agent flow:
# 1. Agent reads SKILL.md
# 2. Launches asterisk binary (CLI dispatcher)
# 3. Produces F0-F6 artifacts via signal.json
# 4. Each artifact = one pipeline step result
# 5. Agent reasoning replaces LLM inference`,
				},
			},
		},
		{
			ID:    "act3-rtfm",
			Title: "ReadPolicy / Smart Knowledge Routing",
			Blocks: []kami.CodeBlock{
				{
					Language:   "yaml",
					Annotation: "Sources declare their own read policy",
					Code: `sources:
  - kind: doc
    name: ptp-operator-architecture
    read_policy: always         # injected into every prompt
    read_when: ""
    local_path: datasets/docs/ptp/architecture.md
  - kind: repo
    name: ptp-operator
    read_policy: conditional    # follows tag-based routing rules
    read_when: "investigating code changes"`,
				},
				{
					Language:   "go",
					Annotation: "Prompt injection replaces a dedicated graph node",
					Code: `// No context node needed — params.go loads always-read content directly
sources := catalog.AlwaysReadSources()
for _, s := range sources {
    content, _ := os.ReadFile(s.LocalPath)
    params.AlwaysReadSources = append(params.AlwaysReadSources, AlwaysReadSource{
        Name: s.Name, Purpose: s.Purpose, Content: string(content),
    })
}`,
				},
			},
		},
	}
}

func (PoliceStationKabuki) Concepts() []kami.ConceptGroup {
	return []kami.ConceptGroup{
		{
			ID:       "act2-colors",
			Title:    "Element Color System / Identity Through Color",
			Subtitle: "Each element maps to a distinct color, giving agents visual identity across the UI",
			Cards: []kami.ConceptCard{
				{Name: "Fire", Icon: "\U0001F525", Description: "Fast-path classifier. Confident, decisive, sometimes premature.", Color: "#ee0000"},
				{Name: "Water", Icon: "\U0001F30A", Description: "Deep evidence gatherer. Thorough, methodical, occasionally slow.", Color: "#37a3a3"},
				{Name: "Earth", Icon: "\U0001F30D", Description: "Infrastructure specialist. Pragmatic, categorical, risk of oversimplification.", Color: "#5e40be"},
				{Name: "Air", Icon: "\U0001F4A8", Description: "Cross-repo correlator. Creative, lateral thinker, sometimes tangential.", Color: "#f0ab00"},
				{Name: "Diamond", Icon: "\U0001F48E", Description: "Adversarial reviewer. Skeptical, evidence-demanding, the quality gate.", Color: "#0066cc"},
				{Name: "Lightning", Icon: "\u26A1", Description: "Pipeline orchestrator. Dispatches work, monitors progress, manages queues.", Color: "#ee0000"},
			},
		},
		{
			ID:       "act2-architecture",
			Title:    "Origami Architecture / Graph-Based Pipeline",
			Subtitle: "Declarative YAML defines the graph; typed Go interfaces execute it",
			Cards: []kami.ConceptCard{
				{Name: "Node", Description: "Processing unit. Each node has a family, optional after-hooks, and produces typed output."},
				{Name: "Edge", Description: "Conditional transition. CEL expressions evaluate node output to choose the next path."},
				{Name: "Graph", Description: "Complete pipeline topology. Nodes + edges + vars + start/done markers."},
				{Name: "Walker", Description: "Traversal engine. Enters nodes, evaluates edges, manages state and loop counters."},
				{Name: "Zone", Description: "Logical grouping of nodes. Maps to worker affinity and zone stickiness."},
				{Name: "Extractor", Description: "AI-powered data transformer. Converts unstructured data to typed artifacts."},
			},
		},
		{
			ID:       "act2-kami",
			Title:    "Kami: The Demiurge Pattern / Triple-Homed Server",
			Subtitle: "One EventBridge, three transports — MCP, HTTP/SSE, WebSocket",
			Cards: []kami.ConceptCard{
				{Name: "MCP (stdio)", Description: "Agent tools: pause, resume, breakpoints, highlight, inspect. The Cursor agent drives the debugger through MCP."},
				{Name: "HTTP/SSE", Description: "Event stream: real-time pipeline events flow to the browser. Node enters, exits, transitions, signals — all streamed live."},
				{Name: "WebSocket", Description: "Command channel: browser sends pause/resume, the server relays to the walker. Bidirectional control."},
				{Name: "EventBridge", Description: "The hub. All three transports share one bridge. Events from any source are broadcast to all listeners."},
			},
		},
		{
			ID:       "act2-mcp-servers",
			Title:    "Three MCP Servers / Tool Registry",
			Subtitle: "Each server exposes a focused tool set for its domain",
			Cards: []kami.ConceptCard{
				{Name: "pipeline-marshaller", Description: "8 tools: start_pipeline, get_next_step, submit_step, get_report, emit_signal, get_signals, get_worker_health, submit_artifact"},
				{Name: "kami-debugger", Description: "14 tools: pause, resume, advance_node, set/clear breakpoint, highlight_nodes/zone, zoom_to_zone, place_marker, clear_all, set_speed, get_pipeline_state, get_snapshot, get_assertions, get_selection"},
				{Name: "ouroboros-metacalibration", Description: "9 tools: all 8 marshaller tools + assemble_profiles for ModelProfile aggregation from discovery runs"},
			},
		},
		{
			ID:       "act2-dialectic",
			Title:    "Adversarial Dialectic / Confidence Through Challenge",
			Subtitle: "Escalation ladder: shadow personas challenge conclusions at increasing intensity",
			Cards: []kami.ConceptCard{
				{Name: "D0 — Baseline", Description: "No challenge. Agent produces output, confidence is self-reported. Fast but unreliable."},
				{Name: "D1 — Soft Probe", Description: "Light questioning: 'Are you sure about the repo selection?' Catches obvious errors."},
				{Name: "D2 — Devil's Advocate", Description: "Structured opposition: antithesis persona argues the opposite conclusion with evidence."},
				{Name: "D3 — Prosecution", Description: "Full adversarial: prosecution vs defense. Each side presents evidence. Arbiter decides."},
				{Name: "D4 — Red Team", Description: "Maximum scrutiny: multiple shadow personas attack from different angles simultaneously."},
			},
		},
		{
			ID:       "act2-masks",
			Title:    "Masks / Detachable Behavioral Middleware",
			Subtitle: "Modify agent behavior without changing identity — composable, stackable, reversible",
			Cards: []kami.ConceptCard{
				{Name: "Concept", Description: "A Mask wraps an agent's behavior. Think HTTP middleware for AI agents. Intercept input, transform output, add constraints."},
				{Name: "Composable", Description: "Stack multiple masks: Verbose + Conservative + Audit. Each layer adds behavior without modifying the agent's core identity."},
				{Name: "Reversible", Description: "Remove a mask, behavior reverts. No permanent state change. Useful for temporary adjustments during specific pipeline phases."},
				{Name: "Use Cases", Description: "Increase verbosity for debugging. Add conservatism for high-stakes decisions. Enforce audit logging for compliance."},
			},
		},
		{
			ID:       "act2-knowledge",
			Title:    "Knowledge Sources / Read Policy Labels",
			Subtitle: "Framework building block: tell agents which knowledge is mandatory vs conditional",
			Cards: []kami.ConceptCard{
				{Name: "Source Types", Description: "SourceKindRepo (Git repos), SourceKindDoc (documentation), SourceKindAPI (external APIs). Each source has metadata and access patterns."},
				{Name: "Read: Always", Description: "Mandatory prerequisite knowledge injected into every prompt. Architecture docs, disambiguation guides. No dedicated graph node needed."},
				{Name: "Read: When...", Description: "Conditional reading. 'Read when investigating code changes' — only relevant for code-related triage categories."},
				{Name: "Router", Description: "KnowledgeSourceRouter selects sources by tags, case context, and read policy. Prevents irrelevant knowledge from polluting the prompt."},
			},
		},
		{
			ID:       "act2-adapters",
			Title:    "Adapters & Marbles / Coming Next",
			Subtitle: "Reusable building blocks for pipeline composition",
			Cards: []kami.ConceptCard{
				{Name: "Adapters", Description: "Helper bundles: transformers, extractors, hooks. Package common patterns (JSON parsing, YAML validation, log filtering) as reusable units."},
				{Name: "Marbles", Description: "Reusable graph nodes: atomic (single node) or composite (subgraphs). Snap together like marbles on a track."},
				{Name: "FQCN Resolution", Description: "Fully qualified component names. Adapters and marbles use FQCN for unambiguous resolution across pipelines."},
				{Name: "Status", Description: "Conceptual design complete. DSL surface and runtime support shipping in the next milestone."},
			},
		},
		{
			ID:       "act3-ouroboros",
			Title:    "Ouroboros / Meta-Calibration",
			Subtitle: "The snake that eats its own tail: calibrate the calibrator",
			Cards: []kami.ConceptCard{
				{Name: "Seed Pipeline", Description: "3-node graph: Generator (creates test cases), Subject (the system under test), Judge (evaluates results)."},
				{Name: "PersonaSheet", Description: "Quantified persona profiles: behavioral dimensions, affinity scores, element mapping. Generated from discovery runs."},
				{Name: "Auto-Routing", Description: "Given a pipeline and PersonaSheet, Ouroboros automatically assigns personas to nodes based on dimension affinity."},
				{Name: "Discovery → Tuning", Description: "Two phases: Discovery probes model behavior across dimensions. Tuning optimizes persona-node assignments for the target pipeline."},
			},
		},
		{
			ID:       "act3-personas",
			Title:    "Persona System / Eight Archetypes",
			Subtitle: "Four primary personas and four antithesis counterparts, each with element identity",
			Cards: []kami.ConceptCard{
				{Name: "Herald (Fire)", Icon: "\U0001F525", Description: "Bold, fast, confident. First to declare a verdict. Antithesis: overconfident, premature conclusions.", Color: "#ee0000"},
				{Name: "Seeker (Water)", Icon: "\U0001F30A", Description: "Methodical, thorough, evidence-first. Examines every log. Antithesis: analysis paralysis, never concludes.", Color: "#37a3a3"},
				{Name: "Sentinel (Earth)", Icon: "\U0001F30D", Description: "Pragmatic, categorical, infrastructure-focused. Files and moves on. Antithesis: oversimplifies, misses nuance.", Color: "#5e40be"},
				{Name: "Weaver (Air)", Icon: "\U0001F4A8", Description: "Creative, lateral thinker, cross-domain. Finds unexpected connections. Antithesis: tangential, unfocused.", Color: "#f0ab00"},
				{Name: "Arbiter (Diamond)", Icon: "\U0001F48E", Description: "Skeptical, evidence-demanding, the final quality gate. Antithesis: paralyzing perfectionism.", Color: "#0066cc"},
				{Name: "Catalyst (Lightning)", Icon: "\u26A1", Description: "Dispatcher, orchestrator, progress-focused. Keeps the pipeline moving. Antithesis: sacrifices quality for speed."},
				{Name: "Antithesis Personas", Description: "Each primary persona has an antithesis — the adversarial version used in the Dialectic system to challenge conclusions."},
				{Name: "Element Affinity", Description: "Personas are matched to elements (Fire, Water, Earth, Air, Diamond, Lightning) which determine visual identity and behavioral tendencies."},
			},
		},
		{
			ID:       "act3-process",
			Title:    "The Process / End-to-End Flow",
			Subtitle: "How a single CI failure flows through the entire system",
			Cards: []kami.ConceptCard{
				{Name: "1. Ingest", Description: "RP launch detected. Failure data pulled via ReportPortal API. Items deduplicated, cases created."},
				{Name: "2. Knowledge Injection", Description: "ReadPolicy: Always sources auto-injected into prompts. Architecture notes, disambiguations, component mapping — no dedicated graph node."},
				{Name: "3. Recall → Triage", Description: "Historical lookup (seen this before?). If miss: classify defect type, identify candidate repos."},
				{Name: "4. Resolve → Investigate", Description: "Repository selection, then deep evidence gathering: logs, commits, pipeline data."},
				{Name: "5. Correlate → Review", Description: "Pattern matching against known failures. Adversarial review: prosecution vs defense."},
				{Name: "6. Report", Description: "Final RCA verdict with evidence chain, confidence score, suspected components, and recommended actions."},
				{Name: "Coordination", Description: "Parallel agents via Papercup, debugged by Kami, tools exposed via three MCP servers, quality-checked by Adversarial Dialectic."},
				{Name: "Calibration", Description: "30-case blind eval against Jira ground truth. 21 metrics. Calibrated by Ouroboros meta-calibration."},
			},
		},
	}
}

// SectionOrder returns the three-act presentation order:
// Act 1 (Product), Act 2 (Engine), Act 3 (Science), then bookends.
func (PoliceStationKabuki) SectionOrder() []string {
	return []string{
		// Act 1 — Asterisk: The Product
		"hero", "agenda", "problem", "solution", "agents",
		"transition", "demo", "results", "competitive",
		// Act 2 — Origami: The Engine
		"act2-dsl", "act2-colors", "act2-architecture", "act2-kami",
		"act2-mcp-servers", "act2-dialectic", "act2-masks",
		"act2-knowledge", "act2-adapters",
		// Act 3 — Deep Science
		"act3-ouroboros", "act3-personas", "act3-papercup",
		"act3-skills", "act3-rtfm", "act3-process",
		// Bookends
		"architecture", "roadmap", "closing",
	}
}
