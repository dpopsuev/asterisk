package orchestrate

import (
	"context"
	"fmt"
	"os"

	"asterisk/internal/framework"
)

// PipelineGraph wraps a framework.Graph with the heuristic evaluation logic
// needed by the orchestrate runner. It is the bridge between the YAML-defined
// graph topology and the Go-defined heuristic closures.
type PipelineGraph struct {
	graph framework.Graph
	def   *framework.PipelineDef
}

// LoadPipelineGraph loads a pipeline YAML and wires heuristic closures into it.
func LoadPipelineGraph(pipelineData []byte, th Thresholds) (*PipelineGraph, error) {
	def, err := framework.LoadPipeline(pipelineData)
	if err != nil {
		return nil, fmt.Errorf("load pipeline: %w", err)
	}
	if err := def.Validate(); err != nil {
		return nil, fmt.Errorf("validate pipeline: %w", err)
	}

	nodeReg := buildNodeRegistry()
	edgeFactory := BuildEdgeFactory(th)

	graph, err := def.BuildGraph(nodeReg, edgeFactory)
	if err != nil {
		return nil, fmt.Errorf("build graph: %w", err)
	}

	return &PipelineGraph{graph: graph, def: def}, nil
}

// LoadPipelineGraphFromFile loads a pipeline YAML from a file path.
func LoadPipelineGraphFromFile(path string, th Thresholds) (*PipelineGraph, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read pipeline file %s: %w", path, err)
	}
	return LoadPipelineGraph(data, th)
}

// Graph returns the underlying framework.Graph.
func (pg *PipelineGraph) Graph() framework.Graph { return pg.graph }

// Def returns the parsed pipeline definition.
func (pg *PipelineGraph) Def() *framework.PipelineDef { return pg.def }

// EvaluateEdges evaluates edges from the given step's node against the artifact
// and state. Returns the matching action and edge ID, or a fallback action.
func (pg *PipelineGraph) EvaluateEdges(
	step PipelineStep,
	artifact any,
	state *CaseState,
) (action *HeuristicAction, matchedRule string) {
	nodeName := StepToNodeName(step)
	edges := pg.graph.EdgesFrom(nodeName)

	wrappedArtifact := wrapArtifact(step, artifact)
	wrappedState := caseStateToWalkerState(state)

	for _, e := range edges {
		t := e.Evaluate(wrappedArtifact, wrappedState)
		if t != nil {
			return transitionToAction(t), e.ID()
		}
	}

	return defaultFallback(step), "FALLBACK"
}

// StepToNodeName converts a PipelineStep enum to a YAML node name.
func StepToNodeName(step PipelineStep) string {
	switch step {
	case StepF0Recall:
		return "recall"
	case StepF1Triage:
		return "triage"
	case StepF2Resolve:
		return "resolve"
	case StepF3Invest:
		return "investigate"
	case StepF4Correlate:
		return "correlate"
	case StepF5Review:
		return "review"
	case StepF6Report:
		return "report"
	default:
		return ""
	}
}

// buildNodeRegistry creates a NodeRegistry with passthrough nodes for each family.
func buildNodeRegistry() framework.NodeRegistry {
	return framework.NodeRegistry{
		"recall":      passthroughNode,
		"triage":      passthroughNode,
		"resolve":     passthroughNode,
		"investigate": passthroughNode,
		"correlate":   passthroughNode,
		"review":      passthroughNode,
		"report":      passthroughNode,
	}
}

func passthroughNode(def framework.NodeDef) framework.Node {
	return &bridgeNode{name: def.Name, element: framework.Element(def.Element)}
}

// BuildEdgeFactory returns a framework.EdgeFactory that maps YAML edge IDs
// to heuristic evaluation closures.
func BuildEdgeFactory(th Thresholds) framework.EdgeFactory {
	heuristics := buildHeuristicMap(th)
	factory := make(framework.EdgeFactory, len(heuristics))
	for id, rule := range heuristics {
		rule := rule
		factory[id] = func(def framework.EdgeDef) framework.Edge {
			return &heuristicEdge{def: def, rule: rule}
		}
	}
	return factory
}

// buildHeuristicMap returns heuristic closures keyed by edge ID.
func buildHeuristicMap(th Thresholds) map[string]HeuristicRule {
	rules := DefaultHeuristics(th)
	m := make(map[string]HeuristicRule, len(rules))
	for _, r := range rules {
		m[r.ID] = r
	}
	return m
}

// heuristicEdge adapts a HeuristicRule closure to the framework.Edge interface.
type heuristicEdge struct {
	def  framework.EdgeDef
	rule HeuristicRule
}

func (e *heuristicEdge) ID() string       { return e.def.ID }
func (e *heuristicEdge) From() string     { return e.def.From }
func (e *heuristicEdge) To() string       { return e.def.To }
func (e *heuristicEdge) IsShortcut() bool { return e.def.Shortcut }
func (e *heuristicEdge) IsLoop() bool     { return e.def.Loop }

func (e *heuristicEdge) Evaluate(a framework.Artifact, s *framework.WalkerState) *framework.Transition {
	var rawArtifact any
	if a != nil {
		rawArtifact = a.Raw()
	}

	caseState := walkerStateToCaseState(s)
	result := e.rule.Evaluate(rawArtifact, caseState)
	if result == nil {
		return nil
	}

	return &framework.Transition{
		NextNode:         StepToNodeName(result.NextStep),
		ContextAdditions: result.ContextAdditions,
		Explanation:      result.Explanation,
	}
}

func transitionToAction(t *framework.Transition) *HeuristicAction {
	return &HeuristicAction{
		NextStep:         nodeNameToStep(t.NextNode),
		ContextAdditions: t.ContextAdditions,
		Explanation:      t.Explanation,
	}
}

func nodeNameToStep(name string) PipelineStep {
	switch name {
	case "recall":
		return StepF0Recall
	case "triage":
		return StepF1Triage
	case "resolve":
		return StepF2Resolve
	case "investigate":
		return StepF3Invest
	case "correlate":
		return StepF4Correlate
	case "review":
		return StepF5Review
	case "report":
		return StepF6Report
	default:
		return StepDone
	}
}

// wrapArtifact wraps a typed orchestrate artifact as a framework.Artifact.
func wrapArtifact(step PipelineStep, artifact any) framework.Artifact {
	if artifact == nil {
		return nil
	}
	return &bridgeArtifact{
		raw:      artifact,
		typeName: string(step),
	}
}

type bridgeArtifact struct {
	raw      any
	typeName string
}

func (a *bridgeArtifact) Type() string       { return a.typeName }
func (a *bridgeArtifact) Confidence() float64 { return 0 }
func (a *bridgeArtifact) Raw() any            { return a.raw }

func caseStateToWalkerState(state *CaseState) *framework.WalkerState {
	ws := framework.NewWalkerState(fmt.Sprintf("%d", state.CaseID))
	ws.CurrentNode = StepToNodeName(state.CurrentStep)
	ws.Status = state.Status
	for k, v := range state.LoopCounts {
		ws.LoopCounts[k] = v
	}
	return ws
}

func walkerStateToCaseState(ws *framework.WalkerState) *CaseState {
	state := &CaseState{
		CurrentStep: nodeNameToStep(ws.CurrentNode),
		Status:      ws.Status,
		LoopCounts:  make(map[string]int, len(ws.LoopCounts)),
	}
	for k, v := range ws.LoopCounts {
		state.LoopCounts[k] = v
	}
	return state
}

// bridgeNode is a passthrough Node used by the graph bridge.
// Processing is handled externally by the orchestrate runner.
type bridgeNode struct {
	name    string
	element framework.Element
}

func (n *bridgeNode) Name() string                { return n.name }
func (n *bridgeNode) ElementAffinity() framework.Element { return n.element }
func (n *bridgeNode) Process(_ context.Context, _ framework.NodeContext) (framework.Artifact, error) {
	return nil, fmt.Errorf("bridge nodes do not process directly; use the orchestrate runner")
}
