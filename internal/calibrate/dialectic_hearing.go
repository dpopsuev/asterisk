package calibrate

import (
	"github.com/dpopsuev/origami"
	"asterisk/internal/orchestrate"
	"encoding/json"
	"fmt"
	"log/slog"
)

// DialecticLoop runs the multi-round D3 dialectic phase until convergence or
// max rounds. Returns the final DialecticRecord.
func DialecticLoop(
	adapter ModelAdapter,
	caseID string,
	thesis *framework.ThesisChallenge,
	antithesis *framework.AntithesisResponse,
	maxRounds int,
) (*framework.DialecticRecord, error) {
	record := &framework.DialecticRecord{
		MaxRounds: maxRounds,
	}

	for round := 1; round <= maxRounds; round++ {
		prompt := buildDialecticRoundPrompt(round, maxRounds, caseID, thesis, antithesis, record)

		raw, err := adapter.SendPrompt(caseID, orchestrate.StepD3Hearing, prompt)
		if err != nil {
			return record, fmt.Errorf("dialectic round %d: %w", round, err)
		}

		var roundResult dialecticRoundResponse
		if err := json.Unmarshal(raw, &roundResult); err != nil {
			return record, fmt.Errorf("dialectic round %d parse: %w", round, err)
		}

		dr := framework.DialecticRound{
			Round:              round,
			ThesisArgument:     roundResult.ThesisArgument,
			AntithesisRebuttal: roundResult.AntithesisRebuttal,
			ArbiterNotes:       roundResult.ArbiterNotes,
		}
		record.Rounds = append(record.Rounds, dr)

		slog.Debug("dialectic round complete",
			slog.String("case_id", caseID),
			slog.Int("round", round),
			slog.Bool("converged", roundResult.Converged),
		)

		if roundResult.Converged {
			record.Converged = true
			break
		}
	}

	return record, nil
}

type dialecticRoundResponse struct {
	ThesisArgument     string `json:"thesis_argument"`
	AntithesisRebuttal string `json:"antithesis_rebuttal"`
	ArbiterNotes       string `json:"arbiter_notes"`
	Converged          bool   `json:"converged"`
}

func buildDialecticRoundPrompt(round, maxRounds int, caseID string, thesis *framework.ThesisChallenge, antithesis *framework.AntithesisResponse, record *framework.DialecticRecord) string {
	prompt := fmt.Sprintf("Dialectic round %d of %d for case %s.\n", round, maxRounds, caseID)

	if thesis != nil {
		prompt += fmt.Sprintf("\nThesis charge: %s (confidence: %.2f)\nNarrative: %s\n",
			thesis.ChargedDefectType, thesis.ConfidenceScore, thesis.ThesisNarrative)
	}

	if antithesis != nil {
		prompt += fmt.Sprintf("\nAntithesis position: concession=%v, alternative=%s\n",
			antithesis.Concession, antithesis.AlternativeHypothesis)
	}

	if len(record.Rounds) > 0 {
		last := record.Rounds[len(record.Rounds)-1]
		prompt += fmt.Sprintf("\nPrior round %d:\n  Thesis: %s\n  Antithesis: %s\n  Arbiter notes: %s\n",
			last.Round, last.ThesisArgument, last.AntithesisRebuttal, last.ArbiterNotes)
	}

	prompt += "\nProduce a dialectic round: thesis argument, antithesis rebuttal, arbiter notes, and whether the dialectic has converged. Output JSON with fields: thesis_argument, antithesis_rebuttal, arbiter_notes, converged (bool)."

	return prompt
}

// NegationFeedbackInjection captures the structured feedback that gets injected
// into the Light path F2/F3 when the dialectic issues a remand synthesis.
type NegationFeedbackInjection struct {
	CaseID                string                           `json:"case_id"`
	ChallengedEvidenceIdx []int                            `json:"challenged_evidence_indices"`
	AlternativeHypothesis string                           `json:"alternative_hypothesis,omitempty"`
	SpecificQuestions     []string                         `json:"specific_questions"`
	OriginalDefect        string                           `json:"original_defect"`
	DialecticGaps         []framework.DialecticEvidenceGap `json:"dialectic_gaps,omitempty"`
}

// BuildNegationInjection extracts structured feedback from a remand synthesis
// to inject into the Light path's F2_RESOLVE and F3_INVESTIGATE steps.
func BuildNegationInjection(
	caseID string,
	synthesis *framework.Synthesis,
	antithesis *framework.AntithesisResponse,
	originalDefect string,
) *NegationFeedbackInjection {
	if synthesis == nil || synthesis.Decision != framework.SynthesisRemand || synthesis.NegationFeedback == nil {
		return nil
	}

	inj := &NegationFeedbackInjection{
		CaseID:                caseID,
		ChallengedEvidenceIdx: synthesis.NegationFeedback.ChallengedEvidence,
		SpecificQuestions:     synthesis.NegationFeedback.SpecificQuestions,
		OriginalDefect:        originalDefect,
	}

	if antithesis != nil && antithesis.AlternativeHypothesis != "" {
		inj.AlternativeHypothesis = antithesis.AlternativeHypothesis
	}

	return inj
}

// InjectNegationContext appends negation feedback to the prompt for Light path
// F2/F3 re-investigation. The feedback directs the agent to address specific
// dialectic challenges and alternative hypotheses.
func InjectNegationContext(basePrompt string, injection *NegationFeedbackInjection) string {
	if injection == nil {
		return basePrompt
	}

	ctx := fmt.Sprintf("\n\n--- DIALECTIC REMAND FEEDBACK ---\n"+
		"The Adversarial Dialectic has remanded case %s for reinvestigation.\n"+
		"Original classification: %s\n",
		injection.CaseID, injection.OriginalDefect)

	if injection.AlternativeHypothesis != "" {
		ctx += fmt.Sprintf("Alternative hypothesis from antithesis-holder: %s\n", injection.AlternativeHypothesis)
	}

	if len(injection.ChallengedEvidenceIdx) > 0 {
		ctx += "Challenged evidence indices (address these gaps):\n"
		for _, idx := range injection.ChallengedEvidenceIdx {
			ctx += fmt.Sprintf("  - Evidence item #%d\n", idx)
		}
	}

	if len(injection.SpecificQuestions) > 0 {
		ctx += "Specific questions from the dialectic:\n"
		for i, q := range injection.SpecificQuestions {
			ctx += fmt.Sprintf("  %d. %s\n", i+1, q)
		}
	}

	ctx += "--- END DIALECTIC FEEDBACK ---\n"

	return basePrompt + ctx
}
