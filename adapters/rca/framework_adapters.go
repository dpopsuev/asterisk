package rca

import (
	framework "github.com/dpopsuev/origami"
)

// DoneNodeName is the terminal pseudo-node name used in circuit definitions.
const DoneNodeName = "DONE"

// NodeNameToStep converts a YAML node name back to a CircuitStep enum.
func NodeNameToStep(name string) CircuitStep {
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
func WrapArtifact(step CircuitStep, artifact any) framework.Artifact {
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


