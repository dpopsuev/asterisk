package rca

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	framework "github.com/dpopsuev/origami"
	"github.com/dpopsuev/origami/logging"

	"asterisk/adapters/store"
)

// calibrationWalker implements framework.Walker for calibration runs.
// It wraps the adapter-based prompt/response cycle, artifact I/O, metric
// extraction, and store side effects into the framework's Walk protocol.
type calibrationWalker struct {
	identity framework.AgentIdentity
	state    *framework.WalkerState

	adapter  ModelAdapter
	st       store.Store
	caseData *store.Case
	gtCase   GroundTruthCase
	cfg      RunConfig
	result   *CaseResult
	caseDir  string
	log      *slog.Logger
}

type calibrationWalkerConfig struct {
	Adapter  ModelAdapter
	Store    store.Store
	CaseData *store.Case
	GTCase   GroundTruthCase
	RunCfg   RunConfig
	Result   *CaseResult
	CaseDir  string
	SuiteID  int64
}

func newCalibrationWalker(cfg calibrationWalkerConfig) *calibrationWalker {
	return &calibrationWalker{
		identity: framework.AgentIdentity{PersonaName: "calibration"},
		state:    framework.NewWalkerState(cfg.GTCase.ID),
		adapter:  cfg.Adapter,
		st:       cfg.Store,
		caseData: cfg.CaseData,
		gtCase:   cfg.GTCase,
		cfg:      cfg.RunCfg,
		result:   cfg.Result,
		caseDir:  cfg.CaseDir,
		log:      logging.New("calibrate-walker"),
	}
}

func (w *calibrationWalker) Identity() framework.AgentIdentity      { return w.identity }
func (w *calibrationWalker) SetIdentity(id framework.AgentIdentity)  { w.identity = id }
func (w *calibrationWalker) State() *framework.WalkerState           { return w.state }

// Handle processes a single circuit node: sends the prompt via the adapter,
// parses the response, writes the artifact, extracts metrics, and applies
// store side effects. The framework graph walk handles edge evaluation and
// state advancement.
func (w *calibrationWalker) Handle(ctx context.Context, node framework.Node, nc framework.NodeContext) (framework.Artifact, error) {
	step := NodeNameToStep(node.Name())
	w.result.ActualPath = append(w.result.ActualPath, stepName(step))

	response, err := w.adapter.SendPrompt(w.gtCase.ID, string(step), "")
	if err != nil {
		return nil, fmt.Errorf("adapter.SendPrompt(%s, %s): %w", w.gtCase.ID, step, err)
	}

	artifact, err := parseTypedArtifact(step, response)
	if err != nil {
		return nil, fmt.Errorf("parse artifact for %s: %w", step, err)
	}

	artifactFile := ArtifactFilename(step)
	if err := WriteArtifact(w.caseDir, artifactFile, artifact); err != nil {
		return nil, fmt.Errorf("write artifact: %w", err)
	}

	extractStepMetrics(w.result, step, artifact, w.gtCase)

	w.log.Info("node processed",
		"node", node.Name(), "case_id", w.gtCase.ID, "artifact_bytes", len(response))

	return WrapArtifact(step, artifact), nil
}

// parseTypedArtifact parses a JSON response into the appropriate typed artifact
// based on the circuit step.
func parseTypedArtifact(step CircuitStep, data json.RawMessage) (any, error) {
	switch step {
	case StepF0Recall:
		return parseJSON[RecallResult](data)
	case StepF1Triage:
		return parseJSON[TriageResult](data)
	case StepF2Resolve:
		return parseJSON[ResolveResult](data)
	case StepF3Invest:
		return parseJSON[InvestigateArtifact](data)
	case StepF4Correlate:
		return parseJSON[CorrelateResult](data)
	case StepF5Review:
		return parseJSON[ReviewDecision](data)
	case StepF6Report:
		return parseJSON[map[string]any](data)
	default:
		return nil, fmt.Errorf("unknown step %s", step)
	}
}
