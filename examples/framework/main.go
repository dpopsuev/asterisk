// Framework Playground — a self-contained demo of the Asterisk agentic framework.
//
// Run it:
//
//	go run ./examples/framework/
//
// No ReportPortal, no external services, no AI subscriptions.
// This program demonstrates every major framework concept in one flow.
package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	fw "asterisk/internal/framework"
)

func main() {
	printHeader()

	section("1. ELEMENTS — Behavioral Archetypes")
	showElements()

	section("2. PERSONAS — Agent Identities")
	showPersonas()

	section("3. PIPELINE DSL — Load and Validate YAML")
	triageDef := loadTriagePipeline()

	section("4. MERMAID — Render Pipeline as Diagram")
	showMermaid("Bug Triage Pipeline", triageDef)

	section("5. GRAPH WALK — Walker Traverses the Pipeline")
	walkTriagePipeline(triageDef)

	section("6. MASKS — Middleware Capabilities")
	showMasks()

	section("7. ELEMENT CYCLES — Generative and Destructive Interactions")
	showCycles()

	section("8. SHADOW COURT — Adversarial Deliberation Pipeline")
	showCourt()

	printFooter()
}

// ---------------------------------------------------------------------------
// Section helpers
// ---------------------------------------------------------------------------

const (
	reset  = "\033[0m"
	bold   = "\033[1m"
	dim    = "\033[2m"
	red    = "\033[31m"
	green  = "\033[32m"
	yellow = "\033[33m"
	blue   = "\033[34m"
	cyan   = "\033[36m"
	white  = "\033[37m"
)

func printHeader() {
	fmt.Println()
	fmt.Printf("%s%s=== Asterisk Framework Playground ===%s\n", bold, cyan, reset)
	fmt.Println()
	fmt.Printf("%sThis program demonstrates the agentic pipeline framework%s\n", dim, reset)
	fmt.Printf("%sthat powers Asterisk's root cause analysis engine.%s\n", dim, reset)
	fmt.Printf("%sNo AI, no external services — pure graph-driven agent orchestration.%s\n\n", dim, reset)
}

func printFooter() {
	fmt.Println()
	fmt.Printf("%s%s=== End of Playground ===%s\n\n", bold, cyan, reset)
	fmt.Printf("To explore the framework source: %sinternal/framework/%s\n", bold, reset)
	fmt.Printf("Production pipelines:            %spipelines/*.yaml%s\n", bold, reset)
	fmt.Printf("Developer guide:                 %sdocs/framework-guide.md%s\n\n", bold, reset)
}

func section(title string) {
	fmt.Println()
	fmt.Printf("%s%s--- %s ---%s\n\n", bold, yellow, title, reset)
}

func indent(s string) string {
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		lines[i] = "    " + l
	}
	return strings.Join(lines, "\n")
}

// ---------------------------------------------------------------------------
// 1. Elements
// ---------------------------------------------------------------------------

func showElements() {
	fmt.Printf("The framework defines %s6 core elements%s, each an archetype governing\n", bold, reset)
	fmt.Printf("how an agent behaves: how fast it moves, how many times it retries,\n")
	fmt.Printf("when it declares convergence, and how it fails.\n\n")

	fmt.Printf("  %-12s %-10s %-6s %-8s %-10s %-5s %s\n",
		"Element", "Speed", "Loops", "Converg.", "Shortcuts", "Depth", "Failure Mode")
	fmt.Printf("  %s\n", strings.Repeat("-", 85))

	for _, el := range fw.AllElements() {
		t := fw.DefaultTraits(el)
		color := elementColor(el)
		fmt.Printf("  %s%-12s%s %-10s %-6d %-8.2f %-10.1f %-5d %s\n",
			color, el, reset,
			t.Speed, t.MaxLoops, t.ConvergenceThreshold,
			t.ShortcutAffinity, t.EvidenceDepth, t.FailureMode)
	}

	fmt.Println()
	fmt.Printf("  %s+ Iron%s (derived from Earth via calibration accuracy — not a core element)\n", dim, reset)
	iron := fw.IronFromEarth(0.80)
	fmt.Printf("    IronFromEarth(0.80): loops=%d, convergence=%.2f, failure=%q\n",
		iron.MaxLoops, iron.ConvergenceThreshold, iron.FailureMode)
}

func elementColor(e fw.Element) string {
	switch e {
	case fw.ElementFire:
		return red
	case fw.ElementLightning:
		return yellow
	case fw.ElementEarth:
		return green
	case fw.ElementDiamond:
		return white
	case fw.ElementWater:
		return blue
	case fw.ElementAir:
		return cyan
	default:
		return dim
	}
}

// ---------------------------------------------------------------------------
// 2. Personas
// ---------------------------------------------------------------------------

func showPersonas() {
	fmt.Printf("Personas are pre-configured agent identities. Each has a %scolor%s,\n", bold, reset)
	fmt.Printf("an %selement%s, a court %sposition%s, and either %sLight%s or %sShadow%s alignment.\n\n", bold, reset, bold, reset, green, reset, red, reset)

	fmt.Printf("  %sLight (Cadai) — the investigation team:%s\n", green, reset)
	for _, p := range fw.LightPersonas() {
		id := p.Identity
		fmt.Printf("    %s%-12s%s %-10s %-10s %-12s %s\n",
			elementColor(id.Element), id.PersonaName, reset,
			id.Color.DisplayName, id.Element, id.Position, p.Description)
	}

	fmt.Println()
	fmt.Printf("  %sShadow (Cytharai) — the adversarial court:%s\n", red, reset)
	for _, p := range fw.ShadowPersonas() {
		id := p.Identity
		fmt.Printf("    %s%-12s%s %-10s %-10s %-12s %s\n",
			elementColor(id.Element), id.PersonaName, reset,
			id.Color.DisplayName, id.Element, id.Position, p.Description)
	}
}

// ---------------------------------------------------------------------------
// 3. Pipeline DSL
// ---------------------------------------------------------------------------

func loadTriagePipeline() *fw.PipelineDef {
	_, thisFile, _, _ := runtime.Caller(0)
	yamlPath := filepath.Join(filepath.Dir(thisFile), "triage.yaml")
	data, err := os.ReadFile(yamlPath)
	if err != nil {
		fmt.Printf("  %sError reading triage.yaml: %v%s\n", red, err, reset)
		os.Exit(1)
	}

	def, err := fw.LoadPipeline(data)
	if err != nil {
		fmt.Printf("  %sError parsing pipeline: %v%s\n", red, err, reset)
		os.Exit(1)
	}

	if err := def.Validate(); err != nil {
		fmt.Printf("  %sValidation failed: %v%s\n", red, err, reset)
		os.Exit(1)
	}

	fmt.Printf("  Loaded pipeline: %s%s%s\n", bold, def.Pipeline, reset)
	fmt.Printf("  Description:     %s\n", def.Description)
	fmt.Printf("  Nodes:           %d\n", len(def.Nodes))
	fmt.Printf("  Edges:           %d\n", len(def.Edges))
	fmt.Printf("  Zones:           %d\n", len(def.Zones))
	fmt.Printf("  Start:           %s\n", def.Start)
	fmt.Printf("  Done:            %s\n", def.Done)

	fmt.Println()
	fmt.Printf("  Nodes:\n")
	for _, n := range def.Nodes {
		fmt.Printf("    %s%-14s%s element=%-10s family=%s\n",
			elementColor(fw.Element(n.Element)), n.Name, reset, n.Element, n.Family)
	}

	fmt.Println()
	fmt.Printf("  Edges:\n")
	for _, e := range def.Edges {
		arrow := "-->"
		if e.Shortcut {
			arrow = "==>"
		}
		if e.Loop {
			arrow = "~~>"
		}
		fmt.Printf("    %-4s %-12s %s %-12s  %s%s%s\n",
			e.ID, e.From, arrow, e.To, dim, e.Condition, reset)
	}

	return def
}

// ---------------------------------------------------------------------------
// 4. Mermaid rendering
// ---------------------------------------------------------------------------

func showMermaid(title string, def *fw.PipelineDef) {
	mermaid := fw.Render(def)
	fmt.Printf("  %sPaste this into any Mermaid viewer (https://mermaid.live):%s\n\n", dim, reset)
	fmt.Println(indent(mermaid))

	courtData, err := os.ReadFile(findPipelinesDir() + "/defect-court.yaml")
	if err == nil {
		courtDef, err := fw.LoadPipeline(courtData)
		if err == nil {
			fmt.Printf("\n  %sBonus — the production Defect Court pipeline:%s\n\n", dim, reset)
			fmt.Println(indent(fw.Render(courtDef)))
		}
	}
}

func findPipelinesDir() string {
	_, thisFile, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(thisFile), "..", "..", "pipelines")
}

// ---------------------------------------------------------------------------
// 5. Graph Walk
// ---------------------------------------------------------------------------

func walkTriagePipeline(def *fw.PipelineDef) {
	fmt.Printf("  Building a graph from the pipeline DSL, then walking it with a Herald.\n\n")

	nodeReg := fw.NodeRegistry{
		"classify":    func(d fw.NodeDef) fw.Node { return &demoNode{name: d.Name, element: fw.Element(d.Element)} },
		"investigate": func(d fw.NodeDef) fw.Node { return &demoNode{name: d.Name, element: fw.Element(d.Element)} },
		"decide":      func(d fw.NodeDef) fw.Node { return &demoNode{name: d.Name, element: fw.Element(d.Element)} },
		"close":       func(d fw.NodeDef) fw.Node { return &demoNode{name: d.Name, element: fw.Element(d.Element)} },
	}

	scenario := newDemoScenario()

	edgeFactory := fw.EdgeFactory{
		"E1": func(d fw.EdgeDef) fw.Edge { return &demoEdge{def: d, scenario: scenario} },
		"E2": func(d fw.EdgeDef) fw.Edge { return &demoEdge{def: d, scenario: scenario} },
		"E3": func(d fw.EdgeDef) fw.Edge { return &demoEdge{def: d, scenario: scenario} },
		"E4": func(d fw.EdgeDef) fw.Edge { return &demoEdge{def: d, scenario: scenario} },
		"E5": func(d fw.EdgeDef) fw.Edge { return &demoEdge{def: d, scenario: scenario} },
		"E6": func(d fw.EdgeDef) fw.Edge { return &demoEdge{def: d, scenario: scenario} },
		"E7": func(d fw.EdgeDef) fw.Edge { return &demoEdge{def: d, scenario: scenario} },
	}

	graph, err := def.BuildGraph(nodeReg, edgeFactory)
	if err != nil {
		fmt.Printf("  %sBuild error: %v%s\n", red, err, reset)
		return
	}

	herald, _ := fw.PersonaByName("Herald")
	walker := &demoWalker{
		identity: herald.Identity,
		state:    fw.NewWalkerState("demo-walk-1"),
		scenario: scenario,
	}

	fmt.Printf("  Walker: %s%s%s (element=%s, position=%s)\n\n",
		bold, herald.Identity.PersonaName, reset,
		herald.Identity.Element, herald.Identity.Position)

	err = graph.Walk(context.Background(), walker, def.Start)
	if err != nil {
		fmt.Printf("  %sWalk error: %v%s\n", red, err, reset)
		return
	}

	fmt.Println()
	fmt.Printf("  %sWalk complete!%s Status: %s\n", green, reset, walker.state.Status)
	fmt.Printf("  Steps taken: %d\n", len(walker.state.History))
	for i, step := range walker.state.History {
		fmt.Printf("    %d. node=%-14s edge=%s\n", i+1, step.Node, step.EdgeID)
	}
}

// demoScenario controls the walk path for demonstration purposes.
type demoScenario struct {
	classifyConfidence    float64
	investigateConverge   float64
	investigateLoopCount  int
	decisionApprove       bool
}

func newDemoScenario() *demoScenario {
	return &demoScenario{
		classifyConfidence:  0.65,
		investigateConverge: 0.75,
		investigateLoopCount: 0,
		decisionApprove:     true,
	}
}

// demoNode implements framework.Node for the playground.
type demoNode struct {
	name    string
	element fw.Element
}

func (n *demoNode) Name() string              { return n.name }
func (n *demoNode) ElementAffinity() fw.Element { return n.element }
func (n *demoNode) Process(ctx context.Context, nc fw.NodeContext) (fw.Artifact, error) {
	return &demoArtifact{typ: n.name, conf: 0.75}, nil
}

// demoArtifact implements framework.Artifact.
type demoArtifact struct {
	typ  string
	conf float64
}

func (a *demoArtifact) Type() string       { return a.typ }
func (a *demoArtifact) Confidence() float64 { return a.conf }
func (a *demoArtifact) Raw() any            { return a }

// demoWalker implements framework.Walker with annotated output.
type demoWalker struct {
	identity fw.AgentIdentity
	state    *fw.WalkerState
	scenario *demoScenario
}

func (w *demoWalker) Identity() fw.AgentIdentity { return w.identity }
func (w *demoWalker) State() *fw.WalkerState     { return w.state }

func (w *demoWalker) Handle(ctx context.Context, node fw.Node, nc fw.NodeContext) (fw.Artifact, error) {
	color := elementColor(node.ElementAffinity())
	fmt.Printf("  %s[%s]%s %s%-14s%s processing...",
		dim, w.identity.PersonaName, reset, color, node.Name(), reset)

	var conf float64
	switch node.Name() {
	case "classify":
		conf = w.scenario.classifyConfidence
		fmt.Printf(" confidence=%.2f", conf)
		if conf >= 0.90 {
			fmt.Printf(" %s(shortcut!)%s", yellow, reset)
		} else {
			fmt.Printf(" %s(needs investigation)%s", dim, reset)
		}
	case "investigate":
		w.scenario.investigateLoopCount++
		conf = w.scenario.investigateConverge
		fmt.Printf(" convergence=%.2f loop=%d", conf, w.scenario.investigateLoopCount)
	case "decide":
		conf = 0.85
		if w.scenario.decisionApprove {
			fmt.Printf(" decision=approve")
		} else {
			fmt.Printf(" decision=reassess")
		}
	case "close":
		conf = 1.0
		fmt.Printf(" %sfinalizing report%s", green, reset)
	}
	fmt.Println()

	return &demoArtifact{typ: node.Name(), conf: conf}, nil
}

// demoEdge implements framework.Edge with scenario-driven evaluation.
type demoEdge struct {
	def      fw.EdgeDef
	scenario *demoScenario
}

func (e *demoEdge) ID() string       { return e.def.ID }
func (e *demoEdge) From() string     { return e.def.From }
func (e *demoEdge) To() string       { return e.def.To }
func (e *demoEdge) IsShortcut() bool { return e.def.Shortcut }
func (e *demoEdge) IsLoop() bool     { return e.def.Loop }

func (e *demoEdge) Evaluate(a fw.Artifact, s *fw.WalkerState) *fw.Transition {
	switch e.def.ID {
	case "E1": // obvious-bug shortcut
		if e.scenario.classifyConfidence >= 0.90 {
			return &fw.Transition{NextNode: e.def.To, Explanation: "high confidence shortcut"}
		}
		return nil
	case "E2": // needs-investigation
		if e.scenario.classifyConfidence < 0.90 {
			return &fw.Transition{NextNode: e.def.To, Explanation: "confidence below threshold"}
		}
		return nil
	case "E3": // evidence-found
		if e.scenario.investigateConverge >= 0.70 {
			return &fw.Transition{NextNode: e.def.To, Explanation: "sufficient evidence"}
		}
		return nil
	case "E4": // keep-digging loop
		if e.scenario.investigateConverge < 0.70 && e.scenario.investigateLoopCount < 3 {
			return &fw.Transition{NextNode: e.def.To, Explanation: "keep investigating"}
		}
		return nil
	case "E5": // approved
		if e.scenario.decisionApprove {
			return &fw.Transition{NextNode: e.def.To, Explanation: "decision approved"}
		}
		return nil
	case "E6": // reassess
		if !e.scenario.decisionApprove {
			return &fw.Transition{NextNode: e.def.To, Explanation: "send back for reassessment"}
		}
		return nil
	case "E7": // done
		return &fw.Transition{NextNode: e.def.To, Explanation: "pipeline complete"}
	}
	return nil
}

// ---------------------------------------------------------------------------
// 6. Masks
// ---------------------------------------------------------------------------

func showMasks() {
	fmt.Printf("  Masks are detachable middleware that grant powers at specific nodes.\n")
	fmt.Printf("  They wrap a node's processing: %spre -> node -> post%s.\n\n", bold, reset)

	masks := fw.DefaultLightMasks()
	fmt.Printf("  %s4 Light Masks:%s\n", bold, reset)
	for name, mask := range masks {
		nodes := strings.Join(mask.ValidNodes(), ", ")
		fmt.Printf("    %-24s at %-14s %s%s%s\n",
			name, nodes, dim, mask.Description(), reset)
	}

	fmt.Println()
	fmt.Printf("  %sExample — equipping Mask of Recall on a 'recall' node:%s\n\n", dim, reset)

	recallNode := &demoNode{name: "recall", element: fw.ElementFire}
	recallMask := fw.NewRecallMask()
	masked, err := fw.EquipMask(recallNode, recallMask)
	if err != nil {
		fmt.Printf("    %sError: %v%s\n", red, err, reset)
		return
	}

	ctx := context.Background()
	nc := fw.NodeContext{
		WalkerState: fw.NewWalkerState("mask-demo"),
		Meta:        make(map[string]any),
	}
	artifact, _ := masked.Process(ctx, nc)
	fmt.Printf("    Processed masked node: type=%q, meta=%v\n",
		artifact.Type(), nc.Meta)
	fmt.Printf("    %sThe mask injected 'prior_rca_available=true' into the context.%s\n", dim, reset)
}

// ---------------------------------------------------------------------------
// 7. Cycles
// ---------------------------------------------------------------------------

func showCycles() {
	fmt.Printf("  Elements interact through two cycles (inspired by Wu Xing):\n\n")

	fmt.Printf("  %sGenerative (sheng) — each element strengthens the next:%s\n", green, reset)
	for _, rule := range fw.GenerativeCycle() {
		fc := elementColor(rule.From)
		tc := elementColor(rule.To)
		fmt.Printf("    %s%-12s%s -> %s%-12s%s  %s%s%s\n",
			fc, rule.From, reset, tc, rule.To, reset, dim, rule.Interaction, reset)
	}

	fmt.Println()
	fmt.Printf("  %sDestructive (ke) — each element challenges another:%s\n", red, reset)
	for _, rule := range fw.DestructiveCycle() {
		fc := elementColor(rule.From)
		tc := elementColor(rule.To)
		fmt.Printf("    %s%-12s%s -> %s%-12s%s  %s%s%s\n",
			fc, rule.From, reset, tc, rule.To, reset, dim, rule.Interaction, reset)
	}

	fmt.Println()
	fmt.Printf("  %sThese cycles govern agent interactions: a Fire agent%s\n", dim, reset)
	fmt.Printf("  %sgenerates work for Earth, but destructively challenges Water.%s\n", dim, reset)
}

// ---------------------------------------------------------------------------
// 8. Shadow Court
// ---------------------------------------------------------------------------

func showCourt() {
	fmt.Printf("  When the Light pipeline's confidence is uncertain (0.50-0.85),\n")
	fmt.Printf("  the Shadow Court activates for adversarial review.\n\n")

	cfg := fw.DefaultCourtConfig()
	cfg.Enabled = true
	fmt.Printf("  Court config: activation=%.2f, max_handoffs=%d, max_remands=%d, ttl=%s\n\n",
		cfg.ActivationThreshold, cfg.MaxHandoffs, cfg.MaxRemands, cfg.TTL)

	fmt.Printf("  Activation check:\n")
	for _, conf := range []float64{0.40, 0.55, 0.75, 0.90} {
		activated := cfg.ShouldActivate(conf)
		marker := dim + "no" + reset
		if activated {
			marker = red + "YES" + reset
		}
		fmt.Printf("    confidence=%.2f  activate? %s\n", conf, marker)
	}

	courtData, err := os.ReadFile(findPipelinesDir() + "/defect-court.yaml")
	if err != nil {
		fmt.Printf("\n  %sCould not load defect-court.yaml: %v%s\n", dim, err, reset)
		return
	}

	courtDef, err := fw.LoadPipeline(courtData)
	if err != nil {
		fmt.Printf("\n  %sCould not parse court pipeline: %v%s\n", dim, err, reset)
		return
	}

	fmt.Println()
	fmt.Printf("  %sShadow Court pipeline (D0-D4):%s\n", bold, reset)
	for _, n := range courtDef.Nodes {
		fmt.Printf("    %s%-14s%s element=%-10s\n",
			elementColor(fw.Element(n.Element)), n.Name, reset, n.Element)
	}

	fmt.Println()
	fmt.Printf("  %sCourt verdicts:%s\n", bold, reset)
	verdicts := []struct {
		name string
		desc string
	}{
		{"affirm", "Original classification stands"},
		{"amend", "Classification changed based on evidence"},
		{"acquit", "Insufficient evidence — produce gap brief"},
		{"remand", "Send back to Light path for reinvestigation"},
		{"mistrial", "Irreconcilable — handoff limit or judge declares"},
	}
	for _, v := range verdicts {
		fmt.Printf("    %-12s %s%s%s\n", v.name, dim, v.desc, reset)
	}

	fmt.Println()
	fmt.Printf("  %sThe court uses typed artifacts (Indictment, DefenseBrief, HearingRecord,\n", dim)
	fmt.Printf("  Verdict) and HD1-HD12 heuristic edges — the same Edge interface used\n")
	fmt.Printf("  by the Light pipeline. Shadow is just another graph walk.%s\n", reset)
}
