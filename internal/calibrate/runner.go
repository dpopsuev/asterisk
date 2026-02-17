package calibrate

import (
	"encoding/json"
	"fmt"
	"log"
	"path/filepath"
	"strings"

	"asterisk/internal/orchestrate"
	"asterisk/internal/store"
)

// RunConfig holds configuration for a calibration run.
type RunConfig struct {
	Scenario     *Scenario
	Adapter      ModelAdapter
	Runs         int
	PromptDir    string
	Thresholds   orchestrate.Thresholds
	TokenTracker TokenTracker // optional; when set, records per-step token usage
	Parallel     int          // number of parallel workers (default 1 = serial)
	TokenBudget  int          // max concurrent dispatches (token semaphore); 0 = Parallel
	BatchSize    int          // max signals per batch for batch-file dispatch mode; 0 = Parallel
}

// DefaultRunConfig returns defaults for calibration.
func DefaultRunConfig(scenario *Scenario, adapter ModelAdapter) RunConfig {
	return RunConfig{
		Scenario:   scenario,
		Adapter:    adapter,
		Runs:       1,
		PromptDir:  ".cursor/prompts",
		Thresholds: orchestrate.DefaultThresholds(),
	}
}

// RunCalibration executes the full calibration loop.
// For each run: create a fresh store, run all cases through the pipeline, score.
func RunCalibration(cfg RunConfig) (*CalibrationReport, error) {
	report := &CalibrationReport{
		Scenario: cfg.Scenario.Name,
		Adapter:  cfg.Adapter.Name(),
		Runs:     cfg.Runs,
	}

	var allRunMetrics []MetricSet

	for run := 0; run < cfg.Runs; run++ {
		log.Printf("[calibrate] === Run %d/%d ===", run+1, cfg.Runs)

		results, err := runSingleCalibration(cfg)
		if err != nil {
			return nil, fmt.Errorf("run %d: %w", run+1, err)
		}

		if run == cfg.Runs-1 {
			report.CaseResults = results
		}

		// Attach token summary if tracker was present — must happen
		// BEFORE computeMetrics so M18 sees real token counts.
		if cfg.TokenTracker != nil {
			ts := cfg.TokenTracker.Summary()
			report.Tokens = &ts

			target := report.CaseResults
			if run < cfg.Runs-1 {
				target = results
			}
			for i := range target {
				cid := target[i].CaseID
				if cs, ok := ts.PerCase[cid]; ok {
					target[i].PromptTokensTotal = cs.PromptTokens
					target[i].ArtifactTokensTotal = cs.ArtifactTokens
					target[i].StepCount = cs.Steps
					target[i].WallClockMs = cs.WallClockMs
				}
			}
		}

		metrics := computeMetrics(cfg.Scenario, results)
		allRunMetrics = append(allRunMetrics, metrics)
	}

	if len(allRunMetrics) == 1 {
		report.Metrics = allRunMetrics[0]
	} else {
		report.RunMetrics = allRunMetrics
		report.Metrics = aggregateRunMetrics(allRunMetrics)
	}

	return report, nil
}

// runSingleCalibration runs one complete calibration pass: all cases, fresh store.
func runSingleCalibration(cfg RunConfig) ([]CaseResult, error) {
	st := store.NewMemStore()

	// Create the investigation scaffolding in the store
	suite := &store.InvestigationSuite{Name: cfg.Scenario.Name, Status: "active"}
	suiteID, err := st.CreateSuite(suite)
	if err != nil {
		return nil, fmt.Errorf("create suite: %w", err)
	}

	// Create versions
	versionMap := make(map[string]int64)
	for _, c := range cfg.Scenario.Cases {
		if _, exists := versionMap[c.Version]; !exists {
			v := &store.Version{Label: c.Version}
			vid, err := st.CreateVersion(v)
			if err != nil {
				return nil, fmt.Errorf("create version %s: %w", c.Version, err)
			}
			versionMap[c.Version] = vid
		}
	}

	// Create pipelines and jobs per version+job combo
	pipelineMap := make(map[pipeKey]int64)
	jobMap := make(map[pipeKey]int64)
	launchMap := make(map[pipeKey]int64)

	for _, c := range cfg.Scenario.Cases {
		pk := pipeKey{c.Version, c.Job}
		if _, exists := pipelineMap[pk]; !exists {
			pipe := &store.Pipeline{
				SuiteID: suiteID, VersionID: versionMap[c.Version],
				Name: fmt.Sprintf("CI %s %s", c.Version, c.Job), Status: "complete",
			}
			pipeID, err := st.CreatePipeline(pipe)
			if err != nil {
				return nil, fmt.Errorf("create pipeline: %w", err)
			}
			pipelineMap[pk] = pipeID

			launch := &store.Launch{
				PipelineID: pipeID, RPLaunchID: 0,
				Name: fmt.Sprintf("Launch %s %s", c.Version, c.Job), Status: "complete",
			}
			launchID, err := st.CreateLaunch(launch)
			if err != nil {
				return nil, fmt.Errorf("create launch: %w", err)
			}
			launchMap[pk] = launchID

			job := &store.Job{
				LaunchID: launchID,
				Name:     c.Job, Status: "complete",
			}
			jobID, err := st.CreateJob(job)
			if err != nil {
				return nil, fmt.Errorf("create job: %w", err)
			}
			jobMap[pk] = jobID
		}
	}

	// When parallel > 1, delegate to the parallel runner
	if cfg.Parallel > 1 {
		return runParallelCalibration(cfg, st, suiteID, versionMap, jobMap, launchMap)
	}

	// Detect adapter type for per-case wiring
	stubAdapter, isStub := cfg.Adapter.(*StubAdapter)
	cursorAdapter, isCursor := cfg.Adapter.(*CursorAdapter)

	// Wire cursor adapter to this run's store and suite
	if isCursor {
		cursorAdapter.SetStore(st)
		cursorAdapter.SetSuiteID(suiteID)
		cursorAdapter.SetWorkspace(ScenarioToWorkspace(cfg.Scenario.Workspace))
	}

	// Process each case in order
	var results []CaseResult
	for i, gtCase := range cfg.Scenario.Cases {
		log.Printf("[calibrate] --- Case %s (%d/%d): %s ---",
			gtCase.ID, i+1, len(cfg.Scenario.Cases), gtCase.TestName)

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

		// Register case with cursor adapter so it can build prompts
		if isCursor && cursorAdapter != nil {
			cursorAdapter.RegisterCase(gtCase.ID, caseData)
		}

		result, err := runCasePipeline(st, caseData, suiteID, gtCase, cfg, stubAdapter, isStub)
		if err != nil {
			log.Printf("[calibrate] ERROR on case %s: %v", gtCase.ID, err)
			result = &CaseResult{
				CaseID:      gtCase.ID,
				TestName:    gtCase.TestName,
				Version:     gtCase.Version,
				Job:         gtCase.Job,
				StoreCaseID: caseID,
			}
		}

		// After the pipeline, update stub adapter's ID maps from store
		if isStub && stubAdapter != nil {
			updateStubIDMaps(stubAdapter, st, caseData, gtCase, cfg.Scenario)
		}

		results = append(results, *result)
	}

	// Post-process: set per-case scoring flags
	for i := range results {
		scoreCaseResult(&results[i], cfg.Scenario)
	}

	return results, nil
}

// scoreCaseResult sets the DefectTypeCorrect, PathCorrect, and ComponentCorrect
// flags on a CaseResult by comparing against ground truth.
func scoreCaseResult(r *CaseResult, scenario *Scenario) {
	var gt *GroundTruthCase
	for j := range scenario.Cases {
		if scenario.Cases[j].ID == r.CaseID {
			gt = &scenario.Cases[j]
			break
		}
	}
	if gt == nil {
		return
	}

	// Path accuracy
	r.PathCorrect = pathsEqual(r.ActualPath, gt.ExpectedPath)

	// Defect type and component — look up ground truth RCA
	if gt.RCAID != "" {
		for _, rca := range scenario.RCAs {
			if rca.ID == gt.RCAID {
				r.DefectTypeCorrect = (r.ActualDefectType == rca.DefectType)
				r.ComponentCorrect = (r.ActualComponent == rca.Component) ||
					(r.ActualRCAMessage != "" && strings.Contains(
						strings.ToLower(r.ActualRCAMessage),
						strings.ToLower(rca.Component)))
				break
			}
		}
	}
}

// runCasePipeline drives the orchestrator for a single case until done.
// Instead of using RunStep (which requires prompt template files), this drives
// the pipeline directly using lower-level orchestrate primitives: state management,
// artifact I/O, heuristic evaluation, and store side effects.
func runCasePipeline(
	st store.Store,
	caseData *store.Case,
	suiteID int64,
	gtCase GroundTruthCase,
	cfg RunConfig,
	stub *StubAdapter,
	isStub bool,
) (*CaseResult, error) {
	result := &CaseResult{
		CaseID:      gtCase.ID,
		TestName:    gtCase.TestName,
		Version:     gtCase.Version,
		Job:         gtCase.Job,
		StoreCaseID: caseData.ID,
	}

	caseDir, err := orchestrate.EnsureCaseDir(suiteID, caseData.ID)
	if err != nil {
		return result, fmt.Errorf("ensure case dir: %w", err)
	}

	// Initialize state
	state := orchestrate.InitState(caseData.ID, suiteID)
	orchestrate.AdvanceStep(state, orchestrate.StepF0Recall, "INIT", "start pipeline")
	if err := orchestrate.SaveState(caseDir, state); err != nil {
		return result, fmt.Errorf("save state: %w", err)
	}

	rules := orchestrate.DefaultHeuristics(cfg.Thresholds)
	maxSteps := 20

	for step := 0; step < maxSteps; step++ {
		if state.CurrentStep == orchestrate.StepDone {
			break
		}

		currentStep := state.CurrentStep
		result.ActualPath = append(result.ActualPath, stepName(currentStep))

		// Get the adapter response for this step
		response, err := cfg.Adapter.SendPrompt(gtCase.ID, currentStep, "")
		if err != nil {
			return result, fmt.Errorf("adapter.SendPrompt(%s, %s): %w", gtCase.ID, currentStep, err)
		}

		// Parse the raw JSON into the appropriate typed artifact
		var artifact any
		switch currentStep {
		case orchestrate.StepF0Recall:
			artifact, err = parseJSON[orchestrate.RecallResult](response)
		case orchestrate.StepF1Triage:
			artifact, err = parseJSON[orchestrate.TriageResult](response)
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
		}
		if err != nil {
			return result, fmt.Errorf("parse artifact for %s: %w", currentStep, err)
		}

		// Write artifact to case directory
		artifactFile := orchestrate.ArtifactFilename(currentStep)
		if err := orchestrate.WriteArtifact(caseDir, artifactFile, artifact); err != nil {
			return result, fmt.Errorf("write artifact: %w", err)
		}

		// Extract per-step metrics
		extractStepMetrics(result, currentStep, artifact, gtCase)

		// Evaluate heuristics
		action, ruleID := orchestrate.EvaluateHeuristics(rules, currentStep, artifact, state)
		log.Printf("[orchestrate] step=%s rule=%s next=%s: %s",
			currentStep, ruleID, action.NextStep, action.Explanation)

		// Handle loop counters
		if currentStep == orchestrate.StepF3Invest && action.NextStep == orchestrate.StepF2Resolve {
			orchestrate.IncrementLoop(state, "investigate")
		}
		if currentStep == orchestrate.StepF5Review &&
			action.NextStep != orchestrate.StepF6Report &&
			action.NextStep != orchestrate.StepDone {
			orchestrate.IncrementLoop(state, "reassess")
		}

		// Apply store side effects via the orchestrator's exported function
		if err := orchestrate.ApplyStoreEffects(st, caseData, currentStep, artifact); err != nil {
			log.Printf("[calibrate] store side-effect error at %s: %v", currentStep, err)
		}

		// Advance state
		orchestrate.AdvanceStep(state, action.NextStep, ruleID, action.Explanation)
		if err := orchestrate.SaveState(caseDir, state); err != nil {
			return result, fmt.Errorf("save state: %w", err)
		}
	}

	// Final state extraction
	result.ActualLoops = orchestrate.LoopCount(state, "investigate")

	// Refresh case data from store for final field values
	updatedCase, err := st.GetCaseV2(caseData.ID)
	if err == nil && updatedCase != nil {
		result.ActualRCAID = updatedCase.RCAID
		if updatedCase.RCAID != 0 {
			rca, err := st.GetRCAV2(updatedCase.RCAID)
			if err == nil && rca != nil {
				result.ActualDefectType = rca.DefectType
				result.ActualRCAMessage = rca.Description
				result.ActualComponent = rca.Component
				result.ActualConvergence = rca.ConvergenceScore
			}
		}
	}

	return result, nil
}


// extractStepMetrics populates CaseResult fields from per-step artifacts.
func extractStepMetrics(result *CaseResult, step orchestrate.PipelineStep, artifact any, gt GroundTruthCase) {
	switch step {
	case orchestrate.StepF0Recall:
		if r, ok := artifact.(*orchestrate.RecallResult); ok && r != nil {
			result.ActualRecallHit = r.Match && r.Confidence >= 0.80
		}
	case orchestrate.StepF1Triage:
		if r, ok := artifact.(*orchestrate.TriageResult); ok && r != nil {
			result.ActualCategory = r.SymptomCategory
			result.ActualSkip = r.SkipInvestigation
			result.ActualCascade = r.CascadeSuspected
			// When H7 fires (single candidate repo), F2 is skipped but the repo is
			// effectively selected by triage. Capture it for repo selection metrics.
			if len(r.CandidateRepos) == 1 && !r.SkipInvestigation {
				result.ActualSelectedRepos = append(result.ActualSelectedRepos, r.CandidateRepos[0])
			}
		}
	case orchestrate.StepF2Resolve:
		if r, ok := artifact.(*orchestrate.ResolveResult); ok && r != nil {
			for _, repo := range r.SelectedRepos {
				result.ActualSelectedRepos = append(result.ActualSelectedRepos, repo.Name)
			}
		}
	case orchestrate.StepF3Invest:
		if r, ok := artifact.(*orchestrate.InvestigateArtifact); ok && r != nil {
			result.ActualDefectType = r.DefectType
			result.ActualRCAMessage = r.RCAMessage
			result.ActualEvidenceRefs = r.EvidenceRefs
			result.ActualConvergence = r.ConvergenceScore
		}
	}
}

// updateStubIDMaps updates the stub adapter's RCA/symptom ID maps after a case
// completes, so subsequent cases can reference prior RCAs/symptoms by store ID.
func updateStubIDMaps(stub *StubAdapter, st store.Store, caseData *store.Case, gtCase GroundTruthCase, scenario *Scenario) {
	updated, err := st.GetCaseV2(caseData.ID)
	if err != nil || updated == nil {
		return
	}

	// Map ground truth RCA ID to store RCA ID
	if updated.RCAID != 0 && gtCase.RCAID != "" {
		stub.SetRCAID(gtCase.RCAID, updated.RCAID)
	}

	// Map ground truth symptom ID to store symptom ID
	if updated.SymptomID != 0 && gtCase.SymptomID != "" {
		stub.SetSymptomID(gtCase.SymptomID, updated.SymptomID)
	}
}

// pipeKey uniquely identifies a (version, job) combination for pipeline/launch/job mapping.
type pipeKey struct{ version, job string }

// stepName returns the short machine code (F0, F1, ...) for internal path tracking.
// Use display.Stage() or display.StagePath() to humanize for output.
func stepName(s orchestrate.PipelineStep) string {
	m := map[orchestrate.PipelineStep]string{
		orchestrate.StepF0Recall:    "F0",
		orchestrate.StepF1Triage:    "F1",
		orchestrate.StepF2Resolve:   "F2",
		orchestrate.StepF3Invest:    "F3",
		orchestrate.StepF4Correlate: "F4",
		orchestrate.StepF5Review:    "F5",
		orchestrate.StepF6Report:    "F6",
	}
	if n, ok := m[s]; ok {
		return n
	}
	return string(s)
}

func parseJSON[T any](data json.RawMessage) (*T, error) {
	var result T
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// pathsEqual compares two pipeline paths (e.g. ["F0","F1","F2","F3","F5","F6"]).
func pathsEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}


// keywordMatch counts how many keywords from the list appear in the text.
func keywordMatch(text string, keywords []string) int {
	lower := strings.ToLower(text)
	count := 0
	for _, kw := range keywords {
		if strings.Contains(lower, strings.ToLower(kw)) {
			count++
		}
	}
	return count
}

// evidenceOverlap computes set overlap between actual and expected evidence refs.
// Uses normalized path matching (partial path match allowed).
func evidenceOverlap(actual, expected []string) (found, total int) {
	total = len(expected)
	if total == 0 {
		return 0, 0
	}
	for _, exp := range expected {
		expNorm := filepath.Base(exp)
		for _, act := range actual {
			if strings.Contains(act, expNorm) || strings.Contains(exp, act) || act == exp {
				found++
				break
			}
		}
	}
	return found, total
}
