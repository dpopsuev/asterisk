package orchestrate

import (
	"fmt"

	"github.com/dpopsuev/origami/logging"
	"asterisk/internal/preinvest"
	"asterisk/internal/store"
	"github.com/dpopsuev/origami/knowledge"
)

// RunnerConfig holds configuration for the pipeline runner.
type RunnerConfig struct {
	PromptDir  string     // directory containing prompt templates (e.g. ".cursor/prompts")
	Thresholds Thresholds // configurable thresholds for heuristics
	BasePath   string     // root directory for investigation artifacts; defaults to DefaultBasePath
}

// DefaultRunnerConfig returns a RunnerConfig with sensible defaults.
func DefaultRunnerConfig() RunnerConfig {
	return RunnerConfig{
		PromptDir:  ".cursor/prompts",
		Thresholds: DefaultThresholds(),
	}
}

// StepResult is returned by RunStep to the CLI caller.
type StepResult struct {
	PromptPath  string       // path to the generated prompt file (user pastes into Cursor)
	NextStep    PipelineStep // the step that was just prepared
	IsDone      bool         // true if the pipeline is complete
	Explanation string       // heuristic explanation for the routing decision
}

// RunStep is the main pipeline driver. It:
//  1. Loads or initializes per-case state.
//  2. If the current step is INIT, advances to F0.
//  3. If an artifact exists for the current step (user already ran it), evaluates
//     heuristics and advances to the next step.
//  4. Builds params, fills the template for the current step, writes the prompt.
//  5. Returns the prompt path for the user to paste into Cursor.
//
// The orchestrator does NOT call an AI model. It generates prompts for the user
// to paste into Cursor. The user runs `asterisk save` to ingest the artifact,
// then runs `asterisk cursor` again to get the next prompt.
func RunStep(
	st store.Store,
	caseData *store.Case,
	env *preinvest.Envelope,
	catalog *knowledge.KnowledgeSourceCatalog,
	cfg RunnerConfig,
) (*StepResult, error) {
	suiteID := int64(1) // default suite for PoC
	if caseData.JobID != 0 {
		// Try to derive suite from the case's job chain
		job, err := st.GetJob(caseData.JobID)
		if err == nil && job != nil {
			launch, err := st.GetLaunch(job.LaunchID)
			if err == nil && launch != nil {
				pipe, err := st.GetPipeline(launch.PipelineID)
				if err == nil && pipe != nil {
					suiteID = pipe.SuiteID
				}
			}
		}
	}

	basePath := cfg.BasePath
	if basePath == "" {
		basePath = DefaultBasePath
	}
	caseDir, err := EnsureCaseDir(basePath, suiteID, caseData.ID)
	if err != nil {
		return nil, fmt.Errorf("ensure case dir: %w", err)
	}

	// Load or initialize state
	state, err := LoadState(caseDir)
	if err != nil {
		return nil, fmt.Errorf("load state: %w", err)
	}
	if state == nil {
		state = InitState(caseData.ID, suiteID)
	}

	// If done, return immediately
	if state.Status == "done" || state.CurrentStep == StepDone {
		return &StepResult{IsDone: true, Explanation: "pipeline complete"}, nil
	}

	// Advance from INIT to F0
	if state.CurrentStep == StepInit {
		AdvanceStep(state, StepF0Recall, "INIT", "start pipeline")
		if err := SaveState(caseDir, state); err != nil {
			return nil, fmt.Errorf("save state: %w", err)
		}
	}

	// Check if the current step's artifact already exists (user completed it).
	// If so, evaluate heuristics and advance until we find a step that needs a prompt.
	for {
		artifact := loadCurrentArtifact(caseDir, state.CurrentStep)
		if artifact == nil {
			break // no artifact yet; need to generate prompt for this step
		}

		// Artifact exists — evaluate graph edges
		action, ruleID := cfg.evaluateStep(state.CurrentStep, artifact, state)

		logging.New("orchestrate").Info("heuristic evaluated",
			"step", string(state.CurrentStep), "rule", ruleID, "next", string(action.NextStep), "explanation", action.Explanation)

		// Handle investigate loop increment
		if state.CurrentStep == StepF3Invest && action.NextStep == StepF2Resolve {
			IncrementLoop(state, "investigate")
		}
		// Handle reassess loop increment
		if state.CurrentStep == StepF5Review && action.NextStep != StepF6Report && action.NextStep != StepDone {
			IncrementLoop(state, "reassess")
		}

		// Apply store side effects for the completed step
		if err := ApplyStoreEffects(st, caseData, state.CurrentStep, artifact); err != nil {
			logging.New("orchestrate").Warn("store side-effect error", "step", string(state.CurrentStep), "error", err)
		}

		AdvanceStep(state, action.NextStep, ruleID, action.Explanation)
		if err := SaveState(caseDir, state); err != nil {
			return nil, fmt.Errorf("save state: %w", err)
		}

		if state.CurrentStep == StepDone {
			return &StepResult{IsDone: true, Explanation: action.Explanation}, nil
		}
	}

	// Deterministic steps: produce artifact without prompting and re-evaluate.
	if state.CurrentStep == StepF1BContext {
		cr := &ContextResult{}
		if err := WriteArtifact(caseDir, ArtifactFilename(StepF1BContext), cr); err != nil {
			return nil, fmt.Errorf("write context artifact: %w", err)
		}
		artifact := loadCurrentArtifact(caseDir, state.CurrentStep)
		action, ruleID := cfg.evaluateStep(state.CurrentStep, artifact, state)
		if err := ApplyStoreEffects(st, caseData, state.CurrentStep, artifact); err != nil {
			logging.New("orchestrate").Warn("store side-effect error", "step", string(state.CurrentStep), "error", err)
		}
		AdvanceStep(state, action.NextStep, ruleID, action.Explanation)
		if err := SaveState(caseDir, state); err != nil {
			return nil, fmt.Errorf("save state: %w", err)
		}
		if state.CurrentStep == StepDone {
			return &StepResult{IsDone: true, Explanation: action.Explanation}, nil
		}
	}

	// Generate prompt for the current step
	step := state.CurrentStep
	loopIter := 0
	if step == StepF3Invest {
		loopIter = LoopCount(state, "investigate")
	}

	params := BuildParams(st, caseData, env, catalog, step, caseDir)
	templatePath := TemplatePathForStep(cfg.PromptDir, step)
	if templatePath == "" {
		return nil, fmt.Errorf("no template for step %s", step)
	}

	prompt, err := FillTemplate(templatePath, params)
	if err != nil {
		return nil, fmt.Errorf("fill template for %s: %w", step, err)
	}

	promptPath, err := WritePrompt(caseDir, step, loopIter, prompt)
	if err != nil {
		return nil, fmt.Errorf("write prompt for %s: %w", step, err)
	}

	// Pause state (waiting for user to complete)
	state.Status = "paused"
	if err := SaveState(caseDir, state); err != nil {
		return nil, fmt.Errorf("save state: %w", err)
	}

	return &StepResult{
		PromptPath:  promptPath,
		NextStep:    step,
		Explanation: fmt.Sprintf("generated prompt for %s (paste into Cursor)", step),
	}, nil
}

// SaveArtifactAndAdvance is called after the user runs `asterisk save` with the
// artifact produced by Cursor. It reads the artifact, evaluates heuristics,
// updates Store, and advances the state.
func SaveArtifactAndAdvance(
	st store.Store,
	caseData *store.Case,
	caseDir string,
	cfg RunnerConfig,
) (*StepResult, error) {
	state, err := LoadState(caseDir)
	if err != nil || state == nil {
		return nil, fmt.Errorf("load state from %s: %w", caseDir, err)
	}

	artifact := loadCurrentArtifact(caseDir, state.CurrentStep)
	if artifact == nil {
		return nil, fmt.Errorf("no artifact found for step %s in %s", state.CurrentStep, caseDir)
	}

	action, ruleID := cfg.evaluateStep(state.CurrentStep, artifact, state)

	logging.New("orchestrate").Info("save: heuristic evaluated",
		"step", string(state.CurrentStep), "rule", ruleID, "next", string(action.NextStep), "explanation", action.Explanation)

	if state.CurrentStep == StepF3Invest && action.NextStep == StepF2Resolve {
		IncrementLoop(state, "investigate")
	}
	if state.CurrentStep == StepF5Review && action.NextStep != StepF6Report && action.NextStep != StepDone {
		IncrementLoop(state, "reassess")
	}

	if err := ApplyStoreEffects(st, caseData, state.CurrentStep, artifact); err != nil {
		logging.New("orchestrate").Warn("store side-effect error", "step", string(state.CurrentStep), "error", err)
	}

	AdvanceStep(state, action.NextStep, ruleID, action.Explanation)

	state.Status = "running"
	if state.CurrentStep == StepDone {
		state.Status = "done"
	}

	if err := SaveState(caseDir, state); err != nil {
		return nil, fmt.Errorf("save state: %w", err)
	}

	return &StepResult{
		NextStep:    state.CurrentStep,
		IsDone:      state.CurrentStep == StepDone,
		Explanation: action.Explanation,
	}, nil
}

// loadCurrentArtifact reads the typed artifact for the given step from the case dir.
func loadCurrentArtifact(caseDir string, step PipelineStep) any {
	switch step {
	case StepF0Recall:
		r, _ := ReadArtifact[RecallResult](caseDir, ArtifactFilename(step))
		if r != nil {
			return r
		}
	case StepF1Triage:
		r, _ := ReadArtifact[TriageResult](caseDir, ArtifactFilename(step))
		if r != nil {
			return r
		}
	case StepF1BContext:
		r, _ := ReadArtifact[ContextResult](caseDir, ArtifactFilename(step))
		if r != nil {
			return r
		}
	case StepF2Resolve:
		r, _ := ReadArtifact[ResolveResult](caseDir, ArtifactFilename(step))
		if r != nil {
			return r
		}
	case StepF3Invest:
		r, _ := ReadArtifact[InvestigateArtifact](caseDir, ArtifactFilename(step))
		if r != nil {
			return r
		}
	case StepF4Correlate:
		r, _ := ReadArtifact[CorrelateResult](caseDir, ArtifactFilename(step))
		if r != nil {
			return r
		}
	case StepF5Review:
		r, _ := ReadArtifact[ReviewDecision](caseDir, ArtifactFilename(step))
		if r != nil {
			return r
		}
	case StepF6Report:
		// F6 doesn't produce an artifact that drives heuristics; check for jira-draft
		r, _ := ReadArtifact[map[string]any](caseDir, ArtifactFilename(step))
		if r != nil {
			return r
		}
	}
	return nil
}

// ApplyStoreEffects updates Store entities based on the completed step's artifact.
// These are the side effects defined in the prompt-orchestrator contract §Phase 5.
// Exported so that callers (e.g. calibrate runner) can reuse the same logic.
func ApplyStoreEffects(
	st store.Store,
	caseData *store.Case,
	step PipelineStep,
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

// applyRecallEffects: F0 → set case.symptom_id, case.rca_id on match.
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

// applyTriageEffects: F1 → create triage row, upsert symptom, set case.symptom_id.
func applyTriageEffects(st store.Store, caseData *store.Case, artifact any) error {
	r, ok := artifact.(*TriageResult)
	if !ok || r == nil {
		return nil
	}
	// Create triage row
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

	// Upsert symptom (fingerprint from test name + error + category)
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

	// Link case to symptom and update status
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

// applyInvestigateEffects: F3 → create/link RCA, update case status.
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
	rcaID, err := st.SaveRCAV2(rca)
	if err != nil {
		return fmt.Errorf("save rca: %w", err)
	}

	// Link case to RCA and update status
	if err := st.LinkCaseToRCA(caseData.ID, rcaID); err != nil {
		return fmt.Errorf("link case to rca: %w", err)
	}
	if err := st.UpdateCaseStatus(caseData.ID, "investigated"); err != nil {
		return fmt.Errorf("update case status: %w", err)
	}
	caseData.RCAID = rcaID
	caseData.Status = "investigated"

	// Link symptom to RCA if symptom exists
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

// applyCorrelateEffects: F4 → link case to shared RCA, update symptom_rca.
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

// applyReviewEffects: F5 approve → update case status; overturn → update artifact.
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
		// Update the RCA with human's correction
		if caseData.RCAID != 0 {
			rca, err := st.GetRCAV2(caseData.RCAID)
			if err == nil && rca != nil {
				rca.Description = r.HumanOverride.RCAMessage
				rca.DefectType = r.HumanOverride.DefectType
				if _, err := st.SaveRCAV2(rca); err != nil {
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

// evaluateStep evaluates heuristic edges using the framework graph built from
// AsteriskPipelineDef. The graph uses the same heuristic closures as Runner.Walk().
func (cfg RunnerConfig) evaluateStep(step PipelineStep, artifact any, state *CaseState) (*HeuristicAction, string) {
	runner, err := BuildRunner(cfg.Thresholds)
	if err != nil {
		logging.New("orchestrate").Warn("build runner for edge eval failed, using legacy heuristics", "error", err)
		rules := DefaultHeuristics(cfg.Thresholds)
		return EvaluateHeuristics(rules, step, artifact, state)
	}

	nodeName := StepToNodeName(step)
	edges := runner.Graph.EdgesFrom(nodeName)
	wrappedArtifact := WrapArtifact(step, artifact)
	wrappedState := caseStateToWalkerState(state)

	for _, e := range edges {
		t := e.Evaluate(wrappedArtifact, wrappedState)
		if t != nil {
			action := &HeuristicAction{
				NextStep:         NodeNameToStep(t.NextNode),
				ContextAdditions: t.ContextAdditions,
				Explanation:      t.Explanation,
			}
			return action, e.ID()
		}
	}

	return defaultFallback(step), "FALLBACK"
}

// ComputeFingerprint generates a deterministic fingerprint from failure attributes.
// This is a simple hash for PoC; can be upgraded to a more sophisticated algorithm.
func ComputeFingerprint(testName, errorMessage, component string) string {
	input := testName + "|" + errorMessage + "|" + component
	// FNV-1a hash
	var h uint64 = 14695981039346656037
	for i := 0; i < len(input); i++ {
		h ^= uint64(input[i])
		h *= 1099511628211
	}
	return fmt.Sprintf("%016x", h)
}
