package rca

import (
	"fmt"

	"asterisk/adapters/store"

	"github.com/dpopsuev/origami/logging"
)

// ApplyStoreEffects updates Store entities based on the completed step's artifact.
// These are the side effects defined in the prompt-orchestrator contract §Phase 5.
// Exported so that callers (e.g. calibrate runner) can reuse the same logic.
func ApplyStoreEffects(
	st store.Store,
	caseData *store.Case,
	step CircuitStep,
	artifact any,
) error {
	switch step {
	case StepF0Recall:
		return applyRecallEffects(st, caseData, artifact)
	case StepF1Triage:
		return applyTriageEffects(st, caseData, artifact)
	case StepF3Invest:
		return applyInvestigateEffects(st, caseData, artifact)
	case StepF4Correlate:
		return applyCorrelateEffects(st, caseData, artifact)
	case StepF5Review:
		return applyReviewEffects(st, caseData, artifact)
	}
	return nil
}

func applyRecallEffects(st store.Store, caseData *store.Case, artifact any) error {
	r, ok := artifact.(*RecallResult)
	if !ok || r == nil || !r.Match {
		return nil
	}
	if r.SymptomID != 0 {
		if err := st.LinkCaseToSymptom(caseData.ID, r.SymptomID); err != nil {
			return fmt.Errorf("link case to symptom: %w", err)
		}
		caseData.SymptomID = r.SymptomID
		_ = st.UpdateSymptomSeen(r.SymptomID)
	}
	if r.PriorRCAID != 0 {
		if err := st.LinkCaseToRCA(caseData.ID, r.PriorRCAID); err != nil {
			return fmt.Errorf("link case to rca: %w", err)
		}
		caseData.RCAID = r.PriorRCAID
	}
	return nil
}

func applyTriageEffects(st store.Store, caseData *store.Case, artifact any) error {
	r, ok := artifact.(*TriageResult)
	if !ok || r == nil {
		return nil
	}
	triage := &store.Triage{
		CaseID:               caseData.ID,
		SymptomCategory:      r.SymptomCategory,
		Severity:             r.Severity,
		DefectTypeHypothesis: r.DefectTypeHypothesis,
		SkipInvestigation:    r.SkipInvestigation,
		ClockSkewSuspected:   r.ClockSkewSuspected,
		CascadeSuspected:     r.CascadeSuspected,
		DataQualityNotes:     r.DataQualityNotes,
	}
	if _, err := st.CreateTriage(triage); err != nil {
		logging.New("orchestrate").Warn("create triage failed", "error", err)
	}

	fingerprint := ComputeFingerprint(caseData.Name, caseData.ErrorMessage, r.SymptomCategory)
	sym, err := st.GetSymptomByFingerprint(fingerprint)
	if err != nil {
		logging.New("orchestrate").Warn("get symptom by fingerprint failed", "error", err)
	}
	if sym == nil {
		newSym := &store.Symptom{
			Name:            caseData.Name,
			Fingerprint:     fingerprint,
			ErrorPattern:    caseData.ErrorMessage,
			Component:       r.SymptomCategory,
			Status:          "active",
			OccurrenceCount: 1,
		}
		symID, err := st.CreateSymptom(newSym)
		if err != nil {
			return fmt.Errorf("create symptom: %w", err)
		}
		caseData.SymptomID = symID
	} else {
		_ = st.UpdateSymptomSeen(sym.ID)
		caseData.SymptomID = sym.ID
	}

	if caseData.SymptomID != 0 {
		if err := st.LinkCaseToSymptom(caseData.ID, caseData.SymptomID); err != nil {
			logging.New("orchestrate").Warn("link case to symptom failed", "error", err)
		}
	}
	if err := st.UpdateCaseStatus(caseData.ID, "triaged"); err != nil {
		return fmt.Errorf("update case status after triage: %w", err)
	}
	caseData.Status = "triaged"
	return nil
}

func applyInvestigateEffects(st store.Store, caseData *store.Case, artifact any) error {
	r, ok := artifact.(*InvestigateArtifact)
	if !ok || r == nil {
		return nil
	}
	title := r.RCAMessage
	if len(title) > 80 {
		title = title[:80] + "..."
	}
	if title == "" {
		title = "RCA from investigation"
	}
	rca := &store.RCA{
		Title:            title,
		Description:      r.RCAMessage,
		DefectType:       r.DefectType,
		Component:        r.Component,
		ConvergenceScore: r.ConvergenceScore,
		Status:           "open",
	}
	rcaID, err := st.SaveRCA(rca)
	if err != nil {
		return fmt.Errorf("save rca: %w", err)
	}

	if err := st.LinkCaseToRCA(caseData.ID, rcaID); err != nil {
		return fmt.Errorf("link case to rca: %w", err)
	}
	if err := st.UpdateCaseStatus(caseData.ID, "investigated"); err != nil {
		return fmt.Errorf("update case status: %w", err)
	}
	caseData.RCAID = rcaID
	caseData.Status = "investigated"

	if caseData.SymptomID != 0 {
		link := &store.SymptomRCA{
			SymptomID:  caseData.SymptomID,
			RCAID:      rcaID,
			Confidence: r.ConvergenceScore,
			Notes:      "linked from F3 investigation",
		}
		if _, err := st.LinkSymptomToRCA(link); err != nil {
			logging.New("orchestrate").Warn("link symptom to RCA failed", "error", err)
		}
	}
	return nil
}

func applyCorrelateEffects(st store.Store, caseData *store.Case, artifact any) error {
	r, ok := artifact.(*CorrelateResult)
	if !ok || r == nil || !r.IsDuplicate || r.LinkedRCAID == 0 {
		return nil
	}
	if err := st.LinkCaseToRCA(caseData.ID, r.LinkedRCAID); err != nil {
		return fmt.Errorf("link case to shared rca: %w", err)
	}
	caseData.RCAID = r.LinkedRCAID

	if caseData.SymptomID != 0 {
		link := &store.SymptomRCA{
			SymptomID:  caseData.SymptomID,
			RCAID:      r.LinkedRCAID,
			Confidence: r.Confidence,
			Notes:      "linked from F4 correlation",
		}
		if _, err := st.LinkSymptomToRCA(link); err != nil {
			logging.New("orchestrate").Warn("link symptom to RCA failed (correlate)", "error", err)
		}
	}
	return nil
}

func applyReviewEffects(st store.Store, caseData *store.Case, artifact any) error {
	r, ok := artifact.(*ReviewDecision)
	if !ok || r == nil {
		return nil
	}
	if r.Decision == "approve" {
		if err := st.UpdateCaseStatus(caseData.ID, "reviewed"); err != nil {
			return fmt.Errorf("update case after review: %w", err)
		}
		caseData.Status = "reviewed"
	}
	if r.Decision == "overturn" && r.HumanOverride != nil {
		if caseData.RCAID != 0 {
			rca, err := st.GetRCA(caseData.RCAID)
			if err == nil && rca != nil {
				rca.Description = r.HumanOverride.RCAMessage
				rca.DefectType = r.HumanOverride.DefectType
				if _, err := st.SaveRCA(rca); err != nil {
					logging.New("orchestrate").Warn("update RCA after overturn failed", "error", err)
				}
			}
		}
		if err := st.UpdateCaseStatus(caseData.ID, "reviewed"); err != nil {
			return fmt.Errorf("update case after overturn: %w", err)
		}
		caseData.Status = "reviewed"
	}
	return nil
}

// ComputeFingerprint generates a deterministic fingerprint from failure attributes.
func ComputeFingerprint(testName, errorMessage, component string) string {
	input := testName + "|" + errorMessage + "|" + component
	var h uint64 = 14695981039346656037
	for i := 0; i < len(input); i++ {
		h ^= uint64(input[i])
		h *= 1099511628211
	}
	return fmt.Sprintf("%016x", h)
}
