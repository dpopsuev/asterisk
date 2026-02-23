package calibrate

import (
	"github.com/dpopsuev/origami"
	"asterisk/internal/orchestrate"
	"encoding/json"
	"fmt"
	"log/slog"
)

// DialecticResult captures the outcome of a Shadow dialectic pipeline run.
type DialecticResult struct {
	Activated         bool                           `json:"activated"`
	OriginalDefect    string                         `json:"original_defect,omitempty"`
	SynthesisDecision framework.SynthesisDecision     `json:"synthesis_decision,omitempty"`
	FinalDefect       string                         `json:"final_defect,omitempty"`
	Flipped           bool                           `json:"flipped"`
	NegationCount     int                            `json:"negation_count"`
	Rounds            int                            `json:"rounds"`
	Gaps              []framework.DialecticEvidenceGap `json:"gaps,omitempty"`
}

// dialecticSteps defines the D0-D4 progression order.
var dialecticSteps = []orchestrate.PipelineStep{
	orchestrate.StepD0Indict,
	orchestrate.StepD1Discover,
	orchestrate.StepD2Defend,
	orchestrate.StepD3Hearing,
	orchestrate.StepD4Verdict,
}

// RunDialectic executes the Shadow dialectic pipeline for a single case.
// It sends prompts through the adapter for each dialectic step, evaluates
// edge conditions using the dialectic edge factory, and handles remand loops.
//
// The lightResult provides the F5/F6 output that the dialectic challenges.
func RunDialectic(
	cfg RunConfig,
	caseID string,
	lightConfidence float64,
	lightDefectType string,
	adapter ModelAdapter,
) DialecticResult {
	if !cfg.DialecticConfig.Enabled || !cfg.DialecticConfig.NeedsAntithesis(lightConfidence) {
		return DialecticResult{Activated: false}
	}

	slog.Info("dialectic activated",
		slog.String("case_id", caseID),
		slog.Float64("light_confidence", lightConfidence),
		slog.String("light_defect", lightDefectType),
	)

	edgeFactory := framework.BuildDialecticEdgeFactory(cfg.DialecticConfig)
	result := DialecticResult{
		Activated:      true,
		OriginalDefect: lightDefectType,
	}

	state := &framework.WalkerState{
		ID:         caseID + "-dialectic",
		LoopCounts: make(map[string]int),
	}

	currentStep := orchestrate.StepD0Indict
	var lastArtifact json.RawMessage

	for iterations := 0; iterations < cfg.DialecticConfig.MaxTurns; iterations++ {
		prompt := buildDialecticPrompt(currentStep, caseID, lightDefectType, lightConfidence, lastArtifact)

		raw, err := adapter.SendPrompt(caseID, currentStep, prompt)
		if err != nil {
			slog.Error("dialectic step failed",
				slog.String("case_id", caseID),
				slog.String("step", string(currentStep)),
				slog.String("error", err.Error()),
			)
			result.SynthesisDecision = framework.SynthesisUnresolved
			break
		}
		lastArtifact = raw
		result.Rounds++

		artifact := parseDialecticArtifact(currentStep, raw)
		if artifact == nil {
			slog.Warn("dialectic artifact parse failed, declaring unresolved",
				slog.String("case_id", caseID),
				slog.String("step", string(currentStep)),
			)
			result.SynthesisDecision = framework.SynthesisUnresolved
			break
		}

		nextStep := evaluateDialecticEdges(edgeFactory, currentStep, artifact, state)
		if nextStep == orchestrate.StepDialecticDone || nextStep == "" {
			if s, ok := artifact.Raw().(*framework.Synthesis); ok {
				result.SynthesisDecision = s.Decision
				result.FinalDefect = s.FinalClassification
				result.Flipped = s.FinalClassification != lightDefectType
				if s.Decision == framework.SynthesisRemand && s.NegationFeedback != nil {
					result.NegationCount++
				}
			}
			break
		}

		if nextStep == orchestrate.StepD0Indict {
			state.LoopCounts["verdict"]++
			result.NegationCount++
		}

		currentStep = nextStep
	}

	slog.Info("dialectic complete",
		slog.String("case_id", caseID),
		slog.String("synthesis", string(result.SynthesisDecision)),
		slog.Bool("flipped", result.Flipped),
		slog.Int("negations", result.NegationCount),
	)

	return result
}

func buildDialecticPrompt(step orchestrate.PipelineStep, caseID, lightDefect string, lightConf float64, priorArtifact json.RawMessage) string {
	base := fmt.Sprintf("Case: %s\nLight classification: %s (confidence: %.2f)\nDialectic step: %s\n",
		caseID, lightDefect, lightConf, step.Family())

	if priorArtifact != nil {
		base += fmt.Sprintf("\nPrior dialectic artifact:\n%s\n", string(priorArtifact))
	}

	switch step {
	case orchestrate.StepD0Indict:
		base += "\nRole: Thesis-holder (Challenger). Examine the Light path evidence and produce a ThesisChallenge with charged defect type, thesis narrative, and itemized evidence with weights."
	case orchestrate.StepD1Discover:
		base += "\nRole: Discovery. Identify additional evidence sources not examined by thesis-holder."
	case orchestrate.StepD2Defend:
		base += "\nRole: Antithesis-holder (Abyss). Challenge the thesis-holder's evidence, propose alternative hypotheses, or concede if the evidence is overwhelming."
	case orchestrate.StepD3Hearing:
		base += "\nRole: Arbiter (Bulwark). Evaluate thesis and antithesis arguments. Produce dialectic notes and determine if the dialectic has converged."
	case orchestrate.StepD4Verdict:
		base += "\nRole: Arbiter (Specter). Render final synthesis: affirm, amend, acquit, remand, or unresolved. Include reasoning and confidence."
	}

	return base
}

// rawArtifact wraps a json.RawMessage as a pass-through Artifact for steps
// without a specific typed artifact (e.g. D1_DISCOVER).
type rawArtifact struct{ data json.RawMessage }

func (r *rawArtifact) Type() string       { return "raw" }
func (r *rawArtifact) Confidence() float64 { return 0 }
func (r *rawArtifact) Raw() any            { return r.data }

func parseDialecticArtifact(step orchestrate.PipelineStep, raw json.RawMessage) framework.Artifact {
	switch step {
	case orchestrate.StepD0Indict:
		var tc framework.ThesisChallenge
		if err := json.Unmarshal(raw, &tc); err != nil {
			return nil
		}
		return &tc
	case orchestrate.StepD1Discover:
		return &rawArtifact{data: raw}
	case orchestrate.StepD2Defend:
		var ar framework.AntithesisResponse
		if err := json.Unmarshal(raw, &ar); err != nil {
			return nil
		}
		return &ar
	case orchestrate.StepD3Hearing:
		var rec framework.DialecticRecord
		if err := json.Unmarshal(raw, &rec); err != nil {
			return nil
		}
		return &rec
	case orchestrate.StepD4Verdict:
		var s framework.Synthesis
		if err := json.Unmarshal(raw, &s); err != nil {
			return nil
		}
		return &s
	default:
		return nil
	}
}

// dialecticEdgeDefs maps each edge ID to its from/to node names (mirrors defect-dialectic.yaml).
var dialecticEdgeDefs = map[string]framework.EdgeDef{
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

func evaluateDialecticEdges(
	factory framework.EdgeFactory,
	currentStep orchestrate.PipelineStep,
	artifact framework.Artifact,
	state *framework.WalkerState,
) orchestrate.PipelineStep {
	fromNode := stepToDialecticNode(currentStep)

	for edgeID, edgeFn := range factory {
		def, ok := dialecticEdgeDefs[edgeID]
		if !ok || def.From != fromNode {
			continue
		}
		edge := edgeFn(def)
		tr := edge.Evaluate(artifact, state)
		if tr != nil {
			return dialecticNodeToStep(tr.NextNode)
		}
	}

	return nextDialecticStep(currentStep)
}

func stepToDialecticNode(step orchestrate.PipelineStep) string {
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

func dialecticNodeToStep(node string) orchestrate.PipelineStep {
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
		return orchestrate.StepDialecticDone
	default:
		return orchestrate.StepDialecticDone
	}
}

func nextDialecticStep(current orchestrate.PipelineStep) orchestrate.PipelineStep {
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
		return orchestrate.StepDialecticDone
	default:
		return orchestrate.StepDialecticDone
	}
}
