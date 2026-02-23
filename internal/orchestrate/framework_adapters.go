package orchestrate

import (
	"context"
	"fmt"

	"github.com/dpopsuev/origami"
)

// DoneNodeName is the terminal pseudo-node name used in pipeline definitions.
const DoneNodeName = "DONE"

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
	case StepDone:
		return DoneNodeName
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

	if e.def.Loop {
		key := loopKeyForNode(e.def.From)
		s.IncrementLoop(key)
	}

	return &framework.Transition{
		NextNode:         StepToNodeName(result.NextStep),
		ContextAdditions: result.ContextAdditions,
		Explanation:      result.Explanation,
	}
}

func loopKeyForNode(nodeName string) string {
	switch nodeName {
	case "investigate":
		return "investigate"
	case "review":
		return "reassess"
	default:
		return nodeName
	}
}

func transitionToAction(t *framework.Transition) *HeuristicAction {
	return &HeuristicAction{
		NextStep:         NodeNameToStep(t.NextNode),
		ContextAdditions: t.ContextAdditions,
		Explanation:      t.Explanation,
	}
}

// NodeNameToStep converts a YAML node name back to a PipelineStep enum.
func NodeNameToStep(name string) PipelineStep {
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

// WrapArtifact wraps a typed orchestrate artifact as a framework.Artifact.
func WrapArtifact(step PipelineStep, artifact any) framework.Artifact {
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

// walkerStateToCaseState converts a framework WalkerState to a domain CaseState
// so heuristic closures can inspect loop counts and status.
func walkerStateToCaseState(ws *framework.WalkerState) *CaseState {
	state := &CaseState{
		CurrentStep: NodeNameToStep(ws.CurrentNode),
		Status:      ws.Status,
		LoopCounts:  make(map[string]int, len(ws.LoopCounts)),
	}
	for k, v := range ws.LoopCounts {
		state.LoopCounts[k] = v
	}
	return state
}

// bridgeNode is a passthrough Node used by the framework graph.
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
