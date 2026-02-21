package calibrate

import (
	"asterisk/pkg/framework"
	"asterisk/internal/orchestrate"
	"encoding/json"
	"fmt"
	"log/slog"
)

// HearingLoop runs the multi-round D3 hearing phase until convergence or
// max rounds. Returns the final HearingRecord.
func HearingLoop(
	adapter ModelAdapter,
	caseID string,
	indictment *framework.Indictment,
	defenseBrief *framework.DefenseBrief,
	maxRounds int,
) (*framework.HearingRecord, error) {
	record := &framework.HearingRecord{
		MaxRounds: maxRounds,
	}

	for round := 1; round <= maxRounds; round++ {
		prompt := buildHearingRoundPrompt(round, maxRounds, caseID, indictment, defenseBrief, record)

		raw, err := adapter.SendPrompt(caseID, orchestrate.StepD3Hearing, prompt)
		if err != nil {
			return record, fmt.Errorf("hearing round %d: %w", round, err)
		}

		var roundResult hearingRoundResponse
		if err := json.Unmarshal(raw, &roundResult); err != nil {
			return record, fmt.Errorf("hearing round %d parse: %w", round, err)
		}

		hr := framework.HearingRound{
			Round:              round,
			ProsecutionArgument: roundResult.ProsecutionArgument,
			DefenseRebuttal:    roundResult.DefenseRebuttal,
			JudgeNotes:         roundResult.JudgeNotes,
		}
		record.Rounds = append(record.Rounds, hr)

		slog.Debug("hearing round complete",
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

type hearingRoundResponse struct {
	ProsecutionArgument string `json:"prosecution_argument"`
	DefenseRebuttal     string `json:"defense_rebuttal"`
	JudgeNotes          string `json:"judge_notes"`
	Converged           bool   `json:"converged"`
}

func buildHearingRoundPrompt(round, maxRounds int, caseID string, ind *framework.Indictment, def *framework.DefenseBrief, record *framework.HearingRecord) string {
	prompt := fmt.Sprintf("Hearing round %d of %d for case %s.\n", round, maxRounds, caseID)

	if ind != nil {
		prompt += fmt.Sprintf("\nProsecution charge: %s (confidence: %.2f)\nNarrative: %s\n",
			ind.ChargedDefectType, ind.ConfidenceScore, ind.ProsecutionNarrative)
	}

	if def != nil {
		prompt += fmt.Sprintf("\nDefense position: plea_deal=%v, alternative=%s\n",
			def.PleaDeal, def.AlternativeHypothesis)
	}

	if len(record.Rounds) > 0 {
		last := record.Rounds[len(record.Rounds)-1]
		prompt += fmt.Sprintf("\nPrior round %d:\n  Prosecution: %s\n  Defense: %s\n  Judge notes: %s\n",
			last.Round, last.ProsecutionArgument, last.DefenseRebuttal, last.JudgeNotes)
	}

	prompt += "\nProduce a hearing round: prosecution argument, defense rebuttal, judge notes, and whether the hearing has converged. Output JSON with fields: prosecution_argument, defense_rebuttal, judge_notes, converged (bool)."

	return prompt
}

// RemandFeedbackInjection captures the structured feedback that gets injected
// into the Light path F2/F3 when the court issues a remand verdict.
type RemandFeedbackInjection struct {
	CaseID                string                       `json:"case_id"`
	ChallengedEvidenceIdx []int                        `json:"challenged_evidence_indices"`
	AlternativeHypothesis string                       `json:"alternative_hypothesis,omitempty"`
	SpecificQuestions     []string                     `json:"specific_questions"`
	OriginalDefect        string                       `json:"original_defect"`
	CourtGaps             []framework.CourtEvidenceGap `json:"court_gaps,omitempty"`
}

// BuildRemandInjection extracts structured feedback from a remand verdict
// to inject into the Light path's F2_RESOLVE and F3_INVESTIGATE steps.
func BuildRemandInjection(
	caseID string,
	verdict *framework.Verdict,
	defenseBrief *framework.DefenseBrief,
	originalDefect string,
) *RemandFeedbackInjection {
	if verdict == nil || verdict.Decision != framework.VerdictRemand || verdict.RemandFeedback == nil {
		return nil
	}

	inj := &RemandFeedbackInjection{
		CaseID:                caseID,
		ChallengedEvidenceIdx: verdict.RemandFeedback.ChallengedEvidence,
		SpecificQuestions:     verdict.RemandFeedback.SpecificQuestions,
		OriginalDefect:        originalDefect,
	}

	if defenseBrief != nil && defenseBrief.AlternativeHypothesis != "" {
		inj.AlternativeHypothesis = defenseBrief.AlternativeHypothesis
	}

	return inj
}

// InjectRemandContext appends remand feedback to the prompt for Light path
// F2/F3 re-investigation. The feedback directs the agent to address specific
// court challenges and alternative hypotheses.
func InjectRemandContext(basePrompt string, injection *RemandFeedbackInjection) string {
	if injection == nil {
		return basePrompt
	}

	ctx := fmt.Sprintf("\n\n--- COURT REMAND FEEDBACK ---\n"+
		"The Defect Court has remanded case %s for reinvestigation.\n"+
		"Original classification: %s\n",
		injection.CaseID, injection.OriginalDefect)

	if injection.AlternativeHypothesis != "" {
		ctx += fmt.Sprintf("Alternative hypothesis from defense: %s\n", injection.AlternativeHypothesis)
	}

	if len(injection.ChallengedEvidenceIdx) > 0 {
		ctx += "Challenged evidence indices (address these gaps):\n"
		for _, idx := range injection.ChallengedEvidenceIdx {
			ctx += fmt.Sprintf("  - Evidence item #%d\n", idx)
		}
	}

	if len(injection.SpecificQuestions) > 0 {
		ctx += "Specific questions from the court:\n"
		for i, q := range injection.SpecificQuestions {
			ctx += fmt.Sprintf("  %d. %s\n", i+1, q)
		}
	}

	ctx += "--- END COURT FEEDBACK ---\n"

	return basePrompt + ctx
}
