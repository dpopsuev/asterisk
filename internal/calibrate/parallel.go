package calibrate

import (
	"asterisk/internal/display"
	"asterisk/internal/logging"
	"context"
	"fmt"
	"sync"

	"asterisk/internal/orchestrate"
	"asterisk/internal/store"

	"golang.org/x/sync/errgroup"
)

// TriageJob is sent into the triage worker pool channel.
type TriageJob struct {
	Index  int
	Case   GroundTruthCase
	CaseID int64 // store ID
	Data   *store.Case
}

// TriageResult is produced by a triage worker.
type TriageResult struct {
	Index      int
	CaseResult *CaseResult
	Err        error

	// Triage-phase artifacts needed for clustering and investigation routing
	RecallArtifact  *orchestrate.RecallResult
	TriageArtifact  *orchestrate.TriageResult
	RecallHit       bool   // true if H1 fired (skip to review)
	FinalStep       orchestrate.PipelineStep
	State           *orchestrate.CaseState
	CaseDir         string
}

// InvestigationJob is sent into the investigation worker pool channel.
type InvestigationJob struct {
	TriageResult *TriageResult
	GTCase       GroundTruthCase
	Cfg          RunConfig
	Store        store.Store
	SuiteID      int64
	IDMapper     IDMappable
	HasIDMap     bool
}

// runParallelCalibration runs calibration with parallel triage and investigation phases.
func runParallelCalibration(ctx context.Context, cfg RunConfig, st *store.MemStore, suiteID int64,
	versionMap map[string]int64, jobMap map[pipeKey]int64, launchMap map[pipeKey]int64,
) ([]CaseResult, error) {
	// Wire store-aware adapters
	if sa, ok := cfg.Adapter.(StoreAware); ok {
		sa.SetStore(st)
		sa.SetSuiteID(suiteID)
		sa.SetWorkspace(ScenarioToWorkspace(cfg.Scenario.Workspace))
	}

	// Check for ID mapping support
	idMapper, hasIDMap := cfg.Adapter.(IDMappable)

	// Token semaphore bounds concurrent dispatches
	tokenSem := make(chan struct{}, cfg.TokenBudget)

	logger := logging.New("parallel")

	// Phase 1: Triage (F0 + F1) with worker pool
	logger.Info("Phase 1: Triage", "workers", cfg.Parallel, "token_budget", cfg.TokenBudget)

	triageResults := make([]TriageResult, len(cfg.Scenario.Cases))

	// Prepare all cases
	var triageJobs []TriageJob
	for i, gtCase := range cfg.Scenario.Cases {
		pk := pipeKey{gtCase.Version, gtCase.Job}
		caseData := &store.Case{
			JobID:        jobMap[pk],
			LaunchID:     launchMap[pk],
			Name:         gtCase.TestName,
			Status:       "open",
			ErrorMessage: gtCase.ErrorMessage,
			LogSnippet:   gtCase.LogSnippet,
		}
		caseID, err := st.CreateCaseV2(caseData)
		if err != nil {
			return nil, fmt.Errorf("create case %s: %w", gtCase.ID, err)
		}
		caseData.ID = caseID

		if sa, ok := cfg.Adapter.(StoreAware); ok {
			sa.RegisterCase(gtCase.ID, caseData)
		}

		triageJobs = append(triageJobs, TriageJob{
			Index:  i,
			Case:   gtCase,
			CaseID: caseID,
			Data:   caseData,
		})
	}

	// Run triage workers using errgroup with context for cancellation
	triageG, triageCtx := errgroup.WithContext(ctx)
	triageG.SetLimit(cfg.Parallel)
	for _, job := range triageJobs {
		job := job
		triageG.Go(func() error {
			triageResults[job.Index] = runTriagePhase(triageCtx, st, job, suiteID, cfg, tokenSem, hasIDMap, idMapper)
			return nil
		})
	}
	_ = triageG.Wait() // errors captured in TriageResult.Err

	// Check for triage errors
	for i, tr := range triageResults {
		if tr.Err != nil {
			logger.Error("triage failed", "case_id", cfg.Scenario.Cases[i].ID, "error", tr.Err)
		}
	}

	// Phase 2: Cluster cases by symptom
	logger.Info("Phase 2: Symptom clustering")
	clusters := ClusterCases(triageResults, cfg.Scenario)

	// Phase 3: Investigation (F2-F6) with worker pool
	logger.Info("Phase 3: Investigation", "clusters", len(clusters), "workers", cfg.Parallel)

	investResults := make(map[int]*CaseResult) // index -> result
	var investMu sync.Mutex

	investG, investCtx := errgroup.WithContext(ctx)
	investG.SetLimit(cfg.Parallel)
	for _, cluster := range clusters {
		rep := cluster.Representative
		job := InvestigationJob{
			TriageResult: rep,
			GTCase:       cfg.Scenario.Cases[rep.Index],
			Cfg:          cfg,
			Store:        st,
			SuiteID:      suiteID,
			IDMapper:     idMapper,
			HasIDMap:     hasIDMap,
		}
		investG.Go(func() error {
			result := runInvestigationPhase(investCtx, job, tokenSem)
			investMu.Lock()
			investResults[job.TriageResult.Index] = result
			investMu.Unlock()
			return nil
		})
	}
	_ = investG.Wait() // errors captured in CaseResult

	// Phase 4: Assemble final results
	logger.Info("Phase 4: Assembling results")
	results := make([]CaseResult, len(cfg.Scenario.Cases))

	// First pass: populate all results from their source (investigation or triage-only)
	for _, cluster := range clusters {
		repIdx := cluster.Representative.Index
		repResult, hasInvest := investResults[repIdx]
		if !hasInvest {
			repResult = triageResults[repIdx].CaseResult
		}
		results[repIdx] = *repResult

		for _, member := range cluster.Members {
			if member.Index == repIdx {
				continue
			}
			memberResult := *triageResults[member.Index].CaseResult

			// Non-recall-hit members get investigation data from representative
			if !triageResults[member.Index].RecallHit && hasInvest {
				memberResult.ActualDefectType = repResult.ActualDefectType
				memberResult.ActualRCAMessage = repResult.ActualRCAMessage
				memberResult.ActualComponent = repResult.ActualComponent
				memberResult.ActualConvergence = repResult.ActualConvergence
				memberResult.ActualEvidenceRefs = repResult.ActualEvidenceRefs
				memberResult.ActualSelectedRepos = repResult.ActualSelectedRepos
				memberResult.ActualRCAID = repResult.ActualRCAID
			}
			results[member.Index] = memberResult
		}
	}

	// Second pass: re-refresh store data for all cases.
	// This is critical for recall-hit cases whose linked RCA was created
	// during the investigation phase (which ran after their triage).
	for i := range results {
		refreshCaseResults(st, results[i].StoreCaseID, &results[i])
	}

	// Post-process: update ID maps and scoring
	for i, gtCase := range cfg.Scenario.Cases {
		if hasIDMap && idMapper != nil {
			updated, err := st.GetCaseV2(triageResults[i].CaseResult.StoreCaseID)
			if err == nil && updated != nil {
				if updated.RCAID != 0 && gtCase.RCAID != "" {
					idMapper.SetRCAID(gtCase.RCAID, updated.RCAID)
				}
				if updated.SymptomID != 0 && gtCase.SymptomID != "" {
					idMapper.SetSymptomID(gtCase.SymptomID, updated.SymptomID)
				}
			}
		}
		scoreCaseResult(&results[i], cfg.Scenario)
	}

	return results, nil
}

// runTriagePhase runs F0 (Recall) and F1 (Triage) for a single case.
// Returns a TriageResult with the partial CaseResult and routing information.
func runTriagePhase(ctx context.Context, st store.Store, job TriageJob, suiteID int64,
	cfg RunConfig, tokenSem chan struct{}, hasIDMap bool, idMapper IDMappable,
) TriageResult {
	gtCase := job.Case
	caseData := job.Data

	result := &CaseResult{
		CaseID:      gtCase.ID,
		TestName:    gtCase.TestName,
		Version:     gtCase.Version,
		Job:         gtCase.Job,
		StoreCaseID: job.CaseID,
	}

	caseDir, err := orchestrate.EnsureCaseDir(cfg.BasePath, suiteID, job.CaseID)
	if err != nil {
		return TriageResult{Index: job.Index, CaseResult: result, Err: fmt.Errorf("ensure case dir: %w", err)}
	}

	state := orchestrate.InitState(job.CaseID, suiteID)
	orchestrate.AdvanceStep(state, orchestrate.StepF0Recall, "INIT", "start pipeline")
	if err := orchestrate.SaveState(caseDir, state); err != nil {
		return TriageResult{Index: job.Index, CaseResult: result, Err: fmt.Errorf("save state: %w", err)}
	}

	rules := orchestrate.DefaultHeuristics(cfg.Thresholds)

	tr := TriageResult{
		Index:      job.Index,
		CaseResult: result,
		CaseDir:    caseDir,
		State:      state,
	}

	// F0: Recall
	result.ActualPath = append(result.ActualPath, "F0")

	if err := acquireToken(ctx, tokenSem); err != nil {
		tr.Err = err
		return tr
	}
	response, err := cfg.Adapter.SendPrompt(gtCase.ID, orchestrate.StepF0Recall, "")
	<-tokenSem

	if err != nil {
		tr.Err = fmt.Errorf("adapter.SendPrompt(F0): %w", err)
		return tr
	}

	recallResult, err := parseJSON[orchestrate.RecallResult](response)
	if err != nil {
		tr.Err = fmt.Errorf("parse recall: %w", err)
		return tr
	}
	tr.RecallArtifact = recallResult

	artifactFile := orchestrate.ArtifactFilename(orchestrate.StepF0Recall)
	if err := orchestrate.WriteArtifact(caseDir, artifactFile, recallResult); err != nil {
		tr.Err = fmt.Errorf("write recall artifact: %w", err)
		return tr
	}

	extractStepMetrics(result, orchestrate.StepF0Recall, recallResult, gtCase)
	logger := logging.New("parallel")
	if err := orchestrate.ApplyStoreEffects(st, caseData, orchestrate.StepF0Recall, recallResult); err != nil {
		logger.Warn("store effect error", "step", "Recall", "error", err)
	}

	action, ruleID := orchestrate.EvaluateHeuristics(rules, orchestrate.StepF0Recall, recallResult, state)
	logger.Info("heuristic evaluated",
		"case_id", gtCase.ID, "step", "Recall", "rule", display.HeuristicWithCode(ruleID), "next", display.Stage(string(action.NextStep)))
	orchestrate.AdvanceStep(state, action.NextStep, ruleID, action.Explanation)
	_ = orchestrate.SaveState(caseDir, state)

	// If recall hit — skip triage and go to review directly
	if action.NextStep == orchestrate.StepF5Review {
		tr.RecallHit = true
		tr.FinalStep = state.CurrentStep

		// Run remaining steps (F5, F6) in the triage phase
		remaining := runRemainingSteps(ctx, st, caseData, state, caseDir, gtCase, cfg, result, rules, tokenSem)
		if remaining != nil {
			tr.Err = remaining
		}
		return tr
	}

	// F1: Triage (only if we didn't short-circuit from recall)
	if state.CurrentStep == orchestrate.StepF1Triage {
		result.ActualPath = append(result.ActualPath, "F1")

		if err := acquireToken(ctx, tokenSem); err != nil {
			tr.Err = err
			return tr
		}
		response, err = cfg.Adapter.SendPrompt(gtCase.ID, orchestrate.StepF1Triage, "")
		<-tokenSem

		if err != nil {
			tr.Err = fmt.Errorf("adapter.SendPrompt(F1): %w", err)
			return tr
		}

		triageResult, err := parseJSON[orchestrate.TriageResult](response)
		if err != nil {
			tr.Err = fmt.Errorf("parse triage: %w", err)
			return tr
		}
		tr.TriageArtifact = triageResult

		artifactFile = orchestrate.ArtifactFilename(orchestrate.StepF1Triage)
		if err := orchestrate.WriteArtifact(caseDir, artifactFile, triageResult); err != nil {
			tr.Err = fmt.Errorf("write triage artifact: %w", err)
			return tr
		}

		extractStepMetrics(result, orchestrate.StepF1Triage, triageResult, gtCase)
		if err := orchestrate.ApplyStoreEffects(st, caseData, orchestrate.StepF1Triage, triageResult); err != nil {
			logger.Warn("store effect error", "step", "Triage", "error", err)
		}

		action, ruleID = orchestrate.EvaluateHeuristics(rules, orchestrate.StepF1Triage, triageResult, state)
		logger.Info("heuristic evaluated",
			"case_id", gtCase.ID, "step", "Triage", "rule", display.HeuristicWithCode(ruleID), "next", display.Stage(string(action.NextStep)))
		orchestrate.AdvanceStep(state, action.NextStep, ruleID, action.Explanation)
		_ = orchestrate.SaveState(caseDir, state)

		// If triage says skip investigation (H4/H5/H18), finish in triage phase
		if action.NextStep == orchestrate.StepF5Review || action.NextStep == orchestrate.StepDone {
			tr.FinalStep = state.CurrentStep
			if action.NextStep == orchestrate.StepF5Review {
				remaining := runRemainingSteps(ctx, st, caseData, state, caseDir, gtCase, cfg, result, rules, tokenSem)
				if remaining != nil {
					tr.Err = remaining
				}
			}
			return tr
		}
	}

	tr.FinalStep = state.CurrentStep
	return tr
}

// runRemainingSteps runs F5+F6 (review and report) for cases that skip investigation.
func runRemainingSteps(ctx context.Context, st store.Store, caseData *store.Case,
	state *orchestrate.CaseState, caseDir string,
	gtCase GroundTruthCase, cfg RunConfig, result *CaseResult,
	rules []orchestrate.HeuristicRule, tokenSem chan struct{},
) error {
	maxSteps := 10
	for step := 0; step < maxSteps; step++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		if state.CurrentStep == orchestrate.StepDone {
			break
		}

		currentStep := state.CurrentStep
		result.ActualPath = append(result.ActualPath, stepName(currentStep))

		if err := acquireToken(ctx, tokenSem); err != nil {
			return err
		}
		response, err := cfg.Adapter.SendPrompt(gtCase.ID, currentStep, "")
		<-tokenSem

		if err != nil {
			return fmt.Errorf("remaining.SendPrompt(%s): %w", currentStep, err)
		}

		var artifact any
		switch currentStep {
		case orchestrate.StepF5Review:
			artifact, err = parseJSON[orchestrate.ReviewDecision](response)
		case orchestrate.StepF6Report:
			artifact, err = parseJSON[map[string]any](response)
		default:
			artifact, err = parseJSON[map[string]any](response)
		}
		if err != nil {
			return fmt.Errorf("parse artifact %s: %w", currentStep, err)
		}

		artifactFile := orchestrate.ArtifactFilename(currentStep)
		if err := orchestrate.WriteArtifact(caseDir, artifactFile, artifact); err != nil {
			return fmt.Errorf("write artifact %s: %w", currentStep, err)
		}
		extractStepMetrics(result, currentStep, artifact, gtCase)

		logger := logging.New("parallel")
		if err := orchestrate.ApplyStoreEffects(st, caseData, currentStep, artifact); err != nil {
			logger.Warn("store effect error", "step", display.Stage(string(currentStep)), "error", err)
		}

		action, ruleID := orchestrate.EvaluateHeuristics(rules, currentStep, artifact, state)
		logger.Info("heuristic evaluated",
			"case_id", gtCase.ID, "step", display.Stage(string(currentStep)), "rule", display.HeuristicWithCode(ruleID), "next", display.Stage(string(action.NextStep)))
		orchestrate.AdvanceStep(state, action.NextStep, ruleID, action.Explanation)
		_ = orchestrate.SaveState(caseDir, state)
	}

	// Final state extraction
	result.ActualLoops = orchestrate.LoopCount(state, "investigate")
	refreshCaseResults(st, caseData.ID, result)

	return nil
}

// runInvestigationPhase runs F2-F6 for a cluster representative.
func runInvestigationPhase(ctx context.Context, job InvestigationJob, tokenSem chan struct{}) *CaseResult {
	tr := job.TriageResult
	result := tr.CaseResult
	state := tr.State
	caseDir := tr.CaseDir
	gtCase := job.GTCase
	cfg := job.Cfg
	st := job.Store

	logger := logging.New("parallel")
	caseData, err := st.GetCaseV2(result.StoreCaseID)
	if err != nil || caseData == nil {
		logger.Error("cannot load case from store", "case_id", gtCase.ID)
		return result
	}

	rules := orchestrate.DefaultHeuristics(cfg.Thresholds)
	maxSteps := 20

	for step := 0; step < maxSteps; step++ {
		if ctx.Err() != nil {
			break
		}
		if state.CurrentStep == orchestrate.StepDone {
			break
		}

		currentStep := state.CurrentStep
		result.ActualPath = append(result.ActualPath, stepName(currentStep))

		if err := acquireToken(ctx, tokenSem); err != nil {
			break
		}
		response, err := cfg.Adapter.SendPrompt(gtCase.ID, currentStep, "")
		<-tokenSem

		if err != nil {
			logger.Error("investigation dispatch error", "case_id", gtCase.ID, "step", string(currentStep), "error", err)
			break
		}

		var artifact any
		switch currentStep {
		case orchestrate.StepF2Resolve:
			artifact, err = parseJSON[orchestrate.ResolveResult](response)
		case orchestrate.StepF3Invest:
			artifact, err = parseJSON[orchestrate.InvestigateArtifact](response)
		case orchestrate.StepF4Correlate:
			artifact, err = parseJSON[orchestrate.CorrelateResult](response)
		case orchestrate.StepF5Review:
			artifact, err = parseJSON[orchestrate.ReviewDecision](response)
		case orchestrate.StepF6Report:
			artifact, err = parseJSON[map[string]any](response)
		default:
			artifact, err = parseJSON[map[string]any](response)
		}
		if err != nil {
			logger.Error("investigation parse error", "case_id", gtCase.ID, "step", string(currentStep), "error", err)
			break
		}

		artifactFile := orchestrate.ArtifactFilename(currentStep)
		if writeErr := orchestrate.WriteArtifact(caseDir, artifactFile, artifact); writeErr != nil {
			logger.Warn("write artifact failed", "step", string(currentStep), "error", writeErr)
		}

		extractStepMetrics(result, currentStep, artifact, gtCase)

		if currentStep == orchestrate.StepF3Invest {
			action, ruleID := orchestrate.EvaluateHeuristics(rules, currentStep, artifact, state)
			if action.NextStep == orchestrate.StepF2Resolve {
				orchestrate.IncrementLoop(state, "investigate")
			}
			logger.Info("heuristic evaluated",
				"case_id", gtCase.ID, "step", display.Stage(string(currentStep)), "rule", display.HeuristicWithCode(ruleID), "next", display.Stage(string(action.NextStep)))
			orchestrate.AdvanceStep(state, action.NextStep, ruleID, action.Explanation)
		} else {
			action, ruleID := orchestrate.EvaluateHeuristics(rules, currentStep, artifact, state)
			if currentStep == orchestrate.StepF5Review &&
				action.NextStep != orchestrate.StepF6Report &&
				action.NextStep != orchestrate.StepDone {
				orchestrate.IncrementLoop(state, "reassess")
			}
			logger.Info("heuristic evaluated",
				"case_id", gtCase.ID, "step", display.Stage(string(currentStep)), "rule", display.HeuristicWithCode(ruleID), "next", display.Stage(string(action.NextStep)))
			orchestrate.AdvanceStep(state, action.NextStep, ruleID, action.Explanation)
		}

		if err := orchestrate.ApplyStoreEffects(st, caseData, currentStep, artifact); err != nil {
			logger.Warn("store effect error", "step", string(currentStep), "error", err)
		}

		_ = orchestrate.SaveState(caseDir, state)
	}

	result.ActualLoops = orchestrate.LoopCount(state, "investigate")
	refreshCaseResults(st, caseData.ID, result)

	return result
}

// refreshCaseResults updates a CaseResult with final data from the store.
func refreshCaseResults(st store.Store, caseID int64, result *CaseResult) {
	updated, err := st.GetCaseV2(caseID)
	if err != nil || updated == nil {
		return
	}
	result.ActualRCAID = updated.RCAID
	if updated.RCAID != 0 {
		rca, err := st.GetRCAV2(updated.RCAID)
		if err == nil && rca != nil {
			result.ActualDefectType = rca.DefectType
			result.ActualRCAMessage = rca.Description
			result.ActualComponent = rca.Component
			result.ActualConvergence = rca.ConvergenceScore
		}
	}
}

// acquireToken acquires a token from the semaphore, respecting context cancellation.
func acquireToken(ctx context.Context, sem chan struct{}) error {
	select {
	case sem <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
