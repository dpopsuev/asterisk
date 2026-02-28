package rca

import (
	"fmt"

	framework "github.com/dpopsuev/origami"
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

