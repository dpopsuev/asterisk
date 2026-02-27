package rca

import (
	"context"
	"encoding/json"
	"fmt"

	"asterisk/internal/orchestrate"

	framework "github.com/dpopsuev/origami"
)

// WalkerContextKeys used by RCA nodes to read runtime dependencies from
// the walker's context map.
const (
	KeyAdapter   = "rca.adapter"
	KeyCaseLabel = "rca.case_label"
)

// rcaNode is a real processing node that delegates to a ModelAdapter.
// Unlike the old passthrough bridgeNode, this actually does work.
type rcaNode struct {
	nodeName string
	step     orchestrate.PipelineStep
	element  framework.Element
}

func (n *rcaNode) Name() string                { return n.nodeName }
func (n *rcaNode) ElementAffinity() framework.Element { return n.element }

func (n *rcaNode) Process(_ context.Context, nc framework.NodeContext) (framework.Artifact, error) {
	adapter, ok := nc.WalkerState.Context[KeyAdapter].(ModelAdapter)
	if !ok {
		return nil, fmt.Errorf("rca node %q: missing %s in walker context", n.nodeName, KeyAdapter)
	}
	caseLabel, _ := nc.WalkerState.Context[KeyCaseLabel].(string)
	if caseLabel == "" {
		caseLabel = nc.WalkerState.ID
	}

	response, err := adapter.SendPrompt(caseLabel, string(n.step), "")
	if err != nil {
		return nil, fmt.Errorf("rca node %q: adapter.SendPrompt: %w", n.nodeName, err)
	}

	artifact, err := parseStepResponse(n.step, []byte(response))
	if err != nil {
		return nil, fmt.Errorf("rca node %q: parse response: %w", n.nodeName, err)
	}

	return &rcaArtifact{raw: artifact, typeName: string(n.step)}, nil
}

// rcaArtifact wraps a typed orchestrate artifact as a framework.Artifact.
type rcaArtifact struct {
	raw      any
	typeName string
}

func (a *rcaArtifact) Type() string        { return a.typeName }
func (a *rcaArtifact) Confidence() float64 { return 0 }
func (a *rcaArtifact) Raw() any            { return a.raw }

func parseStepResponse(step orchestrate.PipelineStep, data []byte) (any, error) {
	switch step {
	case orchestrate.StepF0Recall:
		return unmarshalStep[orchestrate.RecallResult](data)
	case orchestrate.StepF1Triage:
		return unmarshalStep[orchestrate.TriageResult](data)
	case orchestrate.StepF2Resolve:
		return unmarshalStep[orchestrate.ResolveResult](data)
	case orchestrate.StepF3Invest:
		return unmarshalStep[orchestrate.InvestigateArtifact](data)
	case orchestrate.StepF4Correlate:
		return unmarshalStep[orchestrate.CorrelateResult](data)
	case orchestrate.StepF5Review:
		return unmarshalStep[orchestrate.ReviewDecision](data)
	case orchestrate.StepF6Report:
		return unmarshalStep[map[string]any](data)
	default:
		return nil, fmt.Errorf("unknown step %q", step)
	}
}

func unmarshalStep[T any](data []byte) (*T, error) {
	var result T
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// NodeRegistry returns a framework.NodeRegistry with real processing nodes
// for each RCA pipeline step. These nodes delegate to the ModelAdapter
// stored in the walker's context.
func NodeRegistry() framework.NodeRegistry {
	return framework.NodeRegistry{
		"recall":      newRCANodeFactory("recall", orchestrate.StepF0Recall),
		"triage":      newRCANodeFactory("triage", orchestrate.StepF1Triage),
		"resolve":     newRCANodeFactory("resolve", orchestrate.StepF2Resolve),
		"investigate": newRCANodeFactory("investigate", orchestrate.StepF3Invest),
		"correlate":   newRCANodeFactory("correlate", orchestrate.StepF4Correlate),
		"review":      newRCANodeFactory("review", orchestrate.StepF5Review),
		"report":      newRCANodeFactory("report", orchestrate.StepF6Report),
	}
}

func newRCANodeFactory(name string, step orchestrate.PipelineStep) func(framework.NodeDef) framework.Node {
	return func(def framework.NodeDef) framework.Node {
		return &rcaNode{
			nodeName: def.Name,
			step:     step,
			element:  framework.Element(def.Element),
		}
	}
}

// MarbleRegistry returns a framework.MarbleRegistry wrapping each RCA node
// as an atomic marble, plus a composite marble for the D0-D4 dialectic.
func MarbleRegistry() framework.MarbleRegistry {
	reg := framework.MarbleRegistry{}
	for name, factory := range NodeRegistry() {
		marbleName := "rca." + name
		f := factory
		reg[marbleName] = func(def framework.NodeDef) framework.Marble {
			return framework.NewAtomicMarble(f(def))
		}
	}
	return reg
}
