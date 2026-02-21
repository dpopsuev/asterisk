package calibrate

import (
	"asterisk/pkg/framework"
	"asterisk/internal/orchestrate"
	"encoding/json"
	"fmt"
	"log/slog"
)

// CourtResult captures the outcome of a Shadow court pipeline run.
type CourtResult struct {
	Activated       bool                       `json:"activated"`
	OriginalDefect  string                     `json:"original_defect,omitempty"`
	VerdictDecision framework.VerdictDecision   `json:"verdict_decision,omitempty"`
	FinalDefect     string                     `json:"final_defect,omitempty"`
	Flipped         bool                       `json:"flipped"`
	RemandCount     int                        `json:"remand_count"`
	Rounds          int                        `json:"rounds"`
	Gaps            []framework.CourtEvidenceGap `json:"gaps,omitempty"`
}

// courtSteps defines the D0-D4 progression order.
var courtSteps = []orchestrate.PipelineStep{
	orchestrate.StepD0Indict,
	orchestrate.StepD1Discover,
	orchestrate.StepD2Defend,
	orchestrate.StepD3Hearing,
	orchestrate.StepD4Verdict,
}

// RunCourt executes the Shadow court pipeline for a single case.
// It sends prompts through the adapter for each court step, evaluates
// edge conditions using the court edge factory, and handles remand loops.
//
// The lightResult provides the F5/F6 output that the court challenges.
func RunCourt(
	cfg RunConfig,
	caseID string,
	lightConfidence float64,
	lightDefectType string,
	adapter ModelAdapter,
) CourtResult {
	if !cfg.CourtConfig.Enabled || !cfg.CourtConfig.ShouldActivate(lightConfidence) {
		return CourtResult{Activated: false}
	}

	slog.Info("court activated",
		slog.String("case_id", caseID),
		slog.Float64("light_confidence", lightConfidence),
		slog.String("light_defect", lightDefectType),
	)

	edgeFactory := framework.BuildCourtEdgeFactory(cfg.CourtConfig)
	result := CourtResult{
		Activated:      true,
		OriginalDefect: lightDefectType,
	}

	state := &framework.WalkerState{
		ID:         caseID + "-court",
		LoopCounts: make(map[string]int),
	}

	currentStep := orchestrate.StepD0Indict
	var lastArtifact json.RawMessage

	for iterations := 0; iterations < cfg.CourtConfig.MaxHandoffs; iterations++ {
		prompt := buildCourtPrompt(currentStep, caseID, lightDefectType, lightConfidence, lastArtifact)

		raw, err := adapter.SendPrompt(caseID, currentStep, prompt)
		if err != nil {
			slog.Error("court step failed",
				slog.String("case_id", caseID),
				slog.String("step", string(currentStep)),
				slog.String("error", err.Error()),
			)
			result.VerdictDecision = framework.VerdictMistrial
			break
		}
		lastArtifact = raw
		result.Rounds++

		artifact := parseCourtArtifact(currentStep, raw)
		if artifact == nil {
			slog.Warn("court artifact parse failed, declaring mistrial",
				slog.String("case_id", caseID),
				slog.String("step", string(currentStep)),
			)
			result.VerdictDecision = framework.VerdictMistrial
			break
		}

		nextStep := evaluateCourtEdges(edgeFactory, currentStep, artifact, state)
		if nextStep == orchestrate.StepCourtDone || nextStep == "" {
			if v, ok := artifact.Raw().(*framework.Verdict); ok {
				result.VerdictDecision = v.Decision
				result.FinalDefect = v.FinalClassification
				result.Flipped = v.FinalClassification != lightDefectType
				if v.Decision == framework.VerdictRemand && v.RemandFeedback != nil {
					result.RemandCount++
				}
			}
			break
		}

		if nextStep == orchestrate.StepD0Indict {
			state.LoopCounts["verdict"]++
			result.RemandCount++
		}

		currentStep = nextStep
	}

	slog.Info("court complete",
		slog.String("case_id", caseID),
		slog.String("verdict", string(result.VerdictDecision)),
		slog.Bool("flipped", result.Flipped),
		slog.Int("remands", result.RemandCount),
	)

	return result
}

func buildCourtPrompt(step orchestrate.PipelineStep, caseID, lightDefect string, lightConf float64, priorArtifact json.RawMessage) string {
	base := fmt.Sprintf("Case: %s\nLight classification: %s (confidence: %.2f)\nCourt step: %s\n",
		caseID, lightDefect, lightConf, step.Family())

	if priorArtifact != nil {
		base += fmt.Sprintf("\nPrior court artifact:\n%s\n", string(priorArtifact))
	}

	switch step {
	case orchestrate.StepD0Indict:
		base += "\nRole: Prosecution (Challenger). Examine the Light path evidence and produce an Indictment with charged defect type, prosecution narrative, and itemized evidence with weights."
	case orchestrate.StepD1Discover:
		base += "\nRole: Discovery. Identify additional evidence sources not examined by prosecution."
	case orchestrate.StepD2Defend:
		base += "\nRole: Defense (Abyss). Challenge the prosecution's evidence, propose alternative hypotheses, or offer a plea deal if the evidence is overwhelming."
	case orchestrate.StepD3Hearing:
		base += "\nRole: Judge (Bulwark). Evaluate prosecution and defense arguments. Produce hearing notes and determine if the hearing has converged."
	case orchestrate.StepD4Verdict:
		base += "\nRole: Judge (Specter). Render final verdict: affirm, amend, acquit, remand, or mistrial. Include reasoning and confidence."
	}

	return base
}

// rawArtifact wraps a json.RawMessage as a pass-through Artifact for steps
// without a specific typed artifact (e.g. D1_DISCOVER).
type rawArtifact struct{ data json.RawMessage }

func (r *rawArtifact) Type() string       { return "raw" }
func (r *rawArtifact) Confidence() float64 { return 0 }
func (r *rawArtifact) Raw() any            { return r.data }

func parseCourtArtifact(step orchestrate.PipelineStep, raw json.RawMessage) framework.Artifact {
	switch step {
	case orchestrate.StepD0Indict:
		var ind framework.Indictment
		if err := json.Unmarshal(raw, &ind); err != nil {
			return nil
		}
		return &ind
	case orchestrate.StepD1Discover:
		return &rawArtifact{data: raw}
	case orchestrate.StepD2Defend:
		var brief framework.DefenseBrief
		if err := json.Unmarshal(raw, &brief); err != nil {
			return nil
		}
		return &brief
	case orchestrate.StepD3Hearing:
		var rec framework.HearingRecord
		if err := json.Unmarshal(raw, &rec); err != nil {
			return nil
		}
		return &rec
	case orchestrate.StepD4Verdict:
		var v framework.Verdict
		if err := json.Unmarshal(raw, &v); err != nil {
			return nil
		}
		return &v
	default:
		return nil
	}
}

// courtEdgeDefs maps each edge ID to its from/to node names (mirrors defect-court.yaml).
var courtEdgeDefs = map[string]framework.EdgeDef{
	"HD1":  {ID: "HD1", From: "indict", To: "defend", Shortcut: true},
	"HD2":  {ID: "HD2", From: "defend", To: "verdict", Shortcut: true},
	"HD3":  {ID: "HD3", From: "defend", To: "hearing"},
	"HD4":  {ID: "HD4", From: "defend", To: "hearing"},
	"HD5":  {ID: "HD5", From: "hearing", To: "verdict"},
	"HD6":  {ID: "HD6", From: "verdict", To: "_done"},
	"HD7":  {ID: "HD7", From: "verdict", To: "_done"},
	"HD8":  {ID: "HD8", From: "verdict", To: "indict", Loop: true},
	"HD9":  {ID: "HD9", From: "verdict", To: "_done"},
	"HD10": {ID: "HD10", From: "verdict", To: "_done"},
	"HD11": {ID: "HD11", From: "verdict", To: "_done"},
	"HD12": {ID: "HD12", From: "verdict", To: "_done"},
}

func evaluateCourtEdges(
	factory framework.EdgeFactory,
	currentStep orchestrate.PipelineStep,
	artifact framework.Artifact,
	state *framework.WalkerState,
) orchestrate.PipelineStep {
	fromNode := stepToCourtNode(currentStep)

	for edgeID, edgeFn := range factory {
		def, ok := courtEdgeDefs[edgeID]
		if !ok || def.From != fromNode {
			continue
		}
		edge := edgeFn(def)
		tr := edge.Evaluate(artifact, state)
		if tr != nil {
			return courtNodeToStep(tr.NextNode)
		}
	}

	return nextCourtStep(currentStep)
}

func stepToCourtNode(step orchestrate.PipelineStep) string {
	switch step {
	case orchestrate.StepD0Indict:
		return "indict"
	case orchestrate.StepD1Discover:
		return "discover"
	case orchestrate.StepD2Defend:
		return "defend"
	case orchestrate.StepD3Hearing:
		return "hearing"
	case orchestrate.StepD4Verdict:
		return "verdict"
	default:
		return ""
	}
}

func courtNodeToStep(node string) orchestrate.PipelineStep {
	switch node {
	case "indict":
		return orchestrate.StepD0Indict
	case "discover":
		return orchestrate.StepD1Discover
	case "defend":
		return orchestrate.StepD2Defend
	case "hearing":
		return orchestrate.StepD3Hearing
	case "verdict":
		return orchestrate.StepD4Verdict
	case "_done":
		return orchestrate.StepCourtDone
	default:
		return orchestrate.StepCourtDone
	}
}

func nextCourtStep(current orchestrate.PipelineStep) orchestrate.PipelineStep {
	switch current {
	case orchestrate.StepD0Indict:
		return orchestrate.StepD1Discover
	case orchestrate.StepD1Discover:
		return orchestrate.StepD2Defend
	case orchestrate.StepD2Defend:
		return orchestrate.StepD3Hearing
	case orchestrate.StepD3Hearing:
		return orchestrate.StepD4Verdict
	case orchestrate.StepD4Verdict:
		return orchestrate.StepCourtDone
	default:
		return orchestrate.StepCourtDone
	}
}
