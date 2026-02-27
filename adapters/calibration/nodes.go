// Package calibration provides Origami pipeline nodes for the calibration flow.
// Each node wraps existing calibrate package functions, enabling the calibration
// to be expressed as a YAML-defined pipeline while preserving battle-tested logic.
package calibration

import (
	"context"
	"fmt"

	"asterisk/adapters/rca"

	framework "github.com/dpopsuev/origami"
)

const (
	KeyRunConfig = "calibration.config"
	KeyReport    = "calibration.report"
)

type calArtifact struct {
	typ  string
	data any
	conf float64
}

func (a calArtifact) Type() string        { return a.typ }
func (a calArtifact) Confidence() float64  { return a.conf }
func (a calArtifact) Raw() any { return a.data }

// --- Setup Node ---

type setupNode struct{}

func (n *setupNode) Name() string                  { return "setup" }
func (n *setupNode) ElementAffinity() framework.Element { return "" }
func (n *setupNode) Process(ctx context.Context, nc framework.NodeContext) (framework.Artifact, error) {
	cfg, ok := nc.WalkerState.Context[KeyRunConfig].(rca.RunConfig)
	if !ok {
		return nil, fmt.Errorf("missing %s in walker context", KeyRunConfig)
	}
	if cfg.Scenario == nil {
		return nil, fmt.Errorf("scenario is nil")
	}
	return calArtifact{
		typ:  "calibration-setup",
		data: map[string]any{"scenario": cfg.Scenario.Name, "cases": len(cfg.Scenario.Cases)},
		conf: 1.0,
	}, nil
}

// --- Run Cases Node ---

type runCasesNode struct{}

func (n *runCasesNode) Name() string                  { return "run-cases" }
func (n *runCasesNode) ElementAffinity() framework.Element { return "" }
func (n *runCasesNode) Process(ctx context.Context, nc framework.NodeContext) (framework.Artifact, error) {
	cfg, ok := nc.WalkerState.Context[KeyRunConfig].(rca.RunConfig)
	if !ok {
		return nil, fmt.Errorf("missing %s in walker context", KeyRunConfig)
	}

	report, err := rca.RunCalibration(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("run calibration: %w", err)
	}

	nc.WalkerState.Context[KeyReport] = report
	return calArtifact{
		typ:  "calibration-results",
		data: map[string]any{"cases": len(report.CaseResults), "scenario": cfg.Scenario.Name},
		conf: 1.0,
	}, nil
}

// --- Score Node ---

type scoreNode struct{}

func (n *scoreNode) Name() string                  { return "score" }
func (n *scoreNode) ElementAffinity() framework.Element { return "" }
func (n *scoreNode) Process(ctx context.Context, nc framework.NodeContext) (framework.Artifact, error) {
	_, ok := nc.WalkerState.Context[KeyReport].(*rca.CalibrationReport)
	if !ok {
		return nil, fmt.Errorf("missing %s in walker context", KeyReport)
	}
	return calArtifact{
		typ:  "calibration-scored",
		data: map[string]any{"scored": true},
		conf: 1.0,
	}, nil
}

// --- Aggregate Node ---

type aggregateNode struct{}

func (n *aggregateNode) Name() string                  { return "aggregate" }
func (n *aggregateNode) ElementAffinity() framework.Element { return "" }
func (n *aggregateNode) Process(ctx context.Context, nc framework.NodeContext) (framework.Artifact, error) {
	report, ok := nc.WalkerState.Context[KeyReport].(*rca.CalibrationReport)
	if !ok {
		return nil, fmt.Errorf("missing %s in walker context", KeyReport)
	}
	return calArtifact{
		typ:  "calibration-aggregated",
		data: map[string]any{"runs": report.Runs},
		conf: 1.0,
	}, nil
}

// --- Report Node ---

type reportNode struct{}

func (n *reportNode) Name() string                  { return "report" }
func (n *reportNode) ElementAffinity() framework.Element { return "" }
func (n *reportNode) Process(ctx context.Context, nc framework.NodeContext) (framework.Artifact, error) {
	report, ok := nc.WalkerState.Context[KeyReport].(*rca.CalibrationReport)
	if !ok {
		return nil, fmt.Errorf("missing %s in walker context", KeyReport)
	}
	return calArtifact{typ: "calibration-report", data: report, conf: 1.0}, nil
}

// NodeRegistry returns a registry with all calibration pipeline nodes.
func NodeRegistry() framework.NodeRegistry {
	return framework.NodeRegistry{
		"setup":     func(_ framework.NodeDef) framework.Node { return &setupNode{} },
		"run-cases": func(_ framework.NodeDef) framework.Node { return &runCasesNode{} },
		"score":     func(_ framework.NodeDef) framework.Node { return &scoreNode{} },
		"aggregate": func(_ framework.NodeDef) framework.Node { return &aggregateNode{} },
		"report":    func(_ framework.NodeDef) framework.Node { return &reportNode{} },
	}
}
