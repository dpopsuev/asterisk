package calibrate

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	cal "github.com/dpopsuev/origami/calibrate"
	"github.com/dpopsuev/origami/dispatch"
	"github.com/dpopsuev/origami/logging"

	"asterisk/internal/orchestrate"
	"asterisk/adapters/rp"
	"asterisk/adapters/store"
)

// RunConfig holds configuration for a calibration run.
type RunConfig struct {
	Scenario     *Scenario
	Adapter      ModelAdapter
	Runs         int
	PromptDir    string
	Thresholds   orchestrate.Thresholds
	TokenTracker dispatch.TokenTracker // optional; when set, records per-step token usage
	Parallel     int          // number of parallel workers (default 1 = serial)
	TokenBudget  int          // max concurrent dispatches (token semaphore); 0 = Parallel
	BatchSize    int          // max signals per batch for batch-file dispatch mode; 0 = Parallel
	BasePath     string       // root directory for investigation artifacts; defaults to DefaultBasePath
	RPFetcher    rp.EnvelopeFetcher // optional; when set, RP-sourced cases fetch real failure data
	ScoreCard    *cal.ScoreCard // declarative metric definitions; loaded from YAML at startup
}

// DefaultRunConfig returns defaults for calibration.
func DefaultRunConfig(scenario *Scenario, adapter ModelAdapter) RunConfig {
	return RunConfig{
		Scenario:   scenario,
		Adapter:    adapter,
		Runs:       1,
		PromptDir:  ".cursor/prompts",
		Thresholds: orchestrate.DefaultThresholds(),
		BasePath:   orchestrate.DefaultBasePath,
	}
}

// RunCalibration executes the full calibration loop.
// For each run: create a fresh store, run all cases through the pipeline, score.
// The context enables cancellation of in-flight work across all goroutines.
func RunCalibration(ctx context.Context, cfg RunConfig) (*CalibrationReport, error) {
	if cfg.BasePath == "" {
		cfg.BasePath = orchestrate.DefaultBasePath
	}
	if cfg.ScoreCard == nil {
		return nil, fmt.Errorf("RunConfig.ScoreCard is required (load from scorecards/asterisk-rca.yaml)")
	}

	report := &CalibrationReport{
		CalibrationReport: cal.CalibrationReport{
			Scenario: cfg.Scenario.Name,
			Adapter:  cfg.Adapter.Name(),
			Runs:     cfg.Runs,
		},
		BasePath: cfg.BasePath,
		Dataset:  buildDatasetHealth(cfg.Scenario),
	}

	var allRunMetrics []MetricSet

	logger := logging.New("calibrate")

	for run := 0; run < cfg.Runs; run++ {
		logger.Info("starting run", "run", run+1, "total", cfg.Runs)

		results, suiteID, err := runSingleCalibration(ctx, cfg)
		if err != nil {
			return nil, fmt.Errorf("run %d: %w", run+1, err)
		}

		report.SuiteID = suiteID // keep last run's suite ID

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

		metrics := computeMetrics(cfg.Scenario, results, cfg.ScoreCard)
		allRunMetrics = append(allRunMetrics, metrics)
	}

	if len(allRunMetrics) == 1 {
		report.Metrics = allRunMetrics[0]
	} else {
		report.RunMetrics = allRunMetrics
		report.Metrics = aggregateRunMetrics(allRunMetrics, cfg.ScoreCard)
	}

	return report, nil
}

// runSingleCalibration runs one complete calibration pass: all cases, fresh store.
// Returns the case results and the suite ID used for artifact directories.
func runSingleCalibration(ctx context.Context, cfg RunConfig) ([]CaseResult, int64, error) {
	st := store.NewMemStore()

	// Create the investigation scaffolding in the store
	suite := &store.InvestigationSuite{Name: cfg.Scenario.Name, Status: "active"}
	suiteID, err := st.CreateSuite(suite)
	if err != nil {
		return nil, 0, fmt.Errorf("create suite: %w", err)
	}

	// Create versions
	versionMap := make(map[string]int64)
	for _, c := range cfg.Scenario.Cases {
		if _, exists := versionMap[c.Version]; !exists {
			v := &store.Version{Label: c.Version}
			vid, err := st.CreateVersion(v)
			if err != nil {
				return nil, suiteID, fmt.Errorf("create version %s: %w", c.Version, err)
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
				return nil, suiteID, fmt.Errorf("create pipeline: %w", err)
			}
			pipelineMap[pk] = pipeID

			launch := &store.Launch{
				PipelineID: pipeID, RPLaunchID: 0,
				Name: fmt.Sprintf("Launch %s %s", c.Version, c.Job), Status: "complete",
			}
			launchID, err := st.CreateLaunch(launch)
			if err != nil {
				return nil, suiteID, fmt.Errorf("create launch: %w", err)
			}
			launchMap[pk] = launchID

			job := &store.Job{
				LaunchID: launchID,
				Name:     c.Job, Status: "complete",
			}
			jobID, err := st.CreateJob(job)
			if err != nil {
				return nil, suiteID, fmt.Errorf("create job: %w", err)
			}
			jobMap[pk] = jobID
		}
	}

	// When parallel > 1, delegate to the parallel runner
	if cfg.Parallel > 1 {
		results, err := runParallelCalibration(ctx, cfg, st, suiteID, versionMap, jobMap, launchMap)
		return results, suiteID, err
	}

	// Wire store-aware adapters to this run's store and suite
	if sa, ok := cfg.Adapter.(StoreAware); ok {
		sa.SetStore(st)
		sa.SetSuiteID(suiteID)
		sa.SetCatalog(ScenarioToCatalog(cfg.Scenario.Workspace))
	}

	// Check if adapter supports ID mapping (for post-pipeline updates)
	idMapper, hasIDMap := cfg.Adapter.(IDMappable)

	// Process each case in order
	logger := logging.New("calibrate")

	var results []CaseResult
	for i, gtCase := range cfg.Scenario.Cases {
		logger.Info("processing case",
			"case_id", gtCase.ID, "index", i+1, "total", len(cfg.Scenario.Cases), "test", gtCase.TestName)

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
			return nil, suiteID, fmt.Errorf("create case %s: %w", gtCase.ID, err)
		}
		caseData.ID = caseID

		// Register case with store-aware adapters so they can build prompts
		if sa, ok := cfg.Adapter.(StoreAware); ok {
			sa.RegisterCase(gtCase.ID, caseData)
		}

		result, err := runCasePipeline(ctx, st, caseData, suiteID, gtCase, cfg)
		if err != nil {
			logger.Error("case pipeline failed", "case_id", gtCase.ID, "error", err)
			result = &CaseResult{
				CaseID:        gtCase.ID,
				TestName:      gtCase.TestName,
				Version:       gtCase.Version,
				Job:           gtCase.Job,
				StoreCaseID:   caseID,
				PipelineError: err.Error(),
			}
		}

		// After the pipeline, update ID maps from store
		if hasIDMap {
			updateIDMaps(idMapper, st, caseData, gtCase, cfg.Scenario)
		}

		results = append(results, *result)
	}

	// Post-process: set per-case scoring flags
	for i := range results {
		scoreCaseResult(&results[i], cfg.Scenario)
	}

	return results, suiteID, nil
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

// runCasePipeline drives the pipeline for a single case using framework
// Runner.Walk(). The calibrationWalker handles adapter dispatch, artifact
// parsing, metric extraction, and store side effects. The framework graph
// walk handles edge evaluation (heuristics) and state advancement.
func runCasePipeline(
	ctx context.Context,
	st store.Store,
	caseData *store.Case,
	suiteID int64,
	gtCase GroundTruthCase,
	cfg RunConfig,
) (*CaseResult, error) {
	result := &CaseResult{
		CaseID:         gtCase.ID,
		TestName:       gtCase.TestName,
		Version:        gtCase.Version,
		Job:            gtCase.Job,
		StoreCaseID:    caseData.ID,
		RPIssueType:    gtCase.RPIssueType,
		RPAutoAnalyzed: gtCase.RPAutoAnalyzed,
	}

	caseDir, err := orchestrate.EnsureCaseDir(cfg.BasePath, suiteID, caseData.ID)
	if err != nil {
		return result, fmt.Errorf("ensure case dir: %w", err)
	}

	hooks := orchestrate.StoreHooks(st, caseData)
	runner, err := orchestrate.BuildRunner(cfg.Thresholds, hooks)
	if err != nil {
		return result, fmt.Errorf("build runner: %w", err)
	}

	walker := newCalibrationWalker(calibrationWalkerConfig{
		Adapter:  cfg.Adapter,
		Store:    st,
		CaseData: caseData,
		GTCase:   gtCase,
		RunCfg:   cfg,
		Result:   result,
		CaseDir:  caseDir,
		SuiteID:  suiteID,
	})

	if err := runner.Walk(ctx, walker, "recall"); err != nil {
		return result, fmt.Errorf("pipeline walk: %w", err)
	}

	// Persist the case state to disk so transcript weaving can read it.
	history := make([]orchestrate.StepRecord, 0, len(walker.state.History))
	for _, h := range walker.state.History {
		history = append(history, orchestrate.StepRecord{
			Step:        orchestrate.NodeNameToStep(h.Node),
			Outcome:     h.Outcome,
			HeuristicID: h.EdgeID,
			Timestamp:   h.Timestamp,
		})
	}
	caseState := &orchestrate.CaseState{
		CaseID:      caseData.ID,
		SuiteID:     suiteID,
		CurrentStep: orchestrate.NodeNameToStep(walker.state.CurrentNode),
		Status:      walker.state.Status,
		LoopCounts:  walker.state.LoopCounts,
		History:     history,
	}
	if err := orchestrate.SaveState(caseDir, caseState); err != nil {
		logging.New("calibrate").Warn("save final state", "error", err)
	}

	result.ActualLoops = walker.state.LoopCounts["investigate"]

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
		result.ActualSkip = r.SkipInvestigation ||
			r.SymptomCategory == "infra" || r.SymptomCategory == "flake"
		result.ActualCascade = r.CascadeSuspected
		// Always capture triage hypothesis as fallback defect type.
		// Investigation (F3) overwrites this if it produces a defect type.
		// This ensures cases that skip investigation via heuristics (e.g. infra/flake
		// routed F1→F5 by H14) still get credit for correct classification.
		if r.DefectTypeHypothesis != "" && result.ActualDefectType == "" {
			result.ActualDefectType = r.DefectTypeHypothesis
		}
			// When H7 fires (single candidate repo), F2 is skipped but the repo is
			// effectively selected by triage. Capture it for repo selection metrics.
			if len(r.CandidateRepos) == 1 && !r.SkipInvestigation {
				result.ActualSelectedRepos = append(result.ActualSelectedRepos, r.CandidateRepos[0])
			}
		}
	case orchestrate.StepF2Resolve:
		if r, ok := artifact.(*orchestrate.ResolveResult); ok && r != nil {
			result.ActualSelectedRepos = result.ActualSelectedRepos[:0]
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
			if r.Component != "" {
				result.ActualComponent = r.Component
			}
		}
	}
}

// selectRepoByHypothesis maps a defect_type_hypothesis to workspace repos
// using Purpose keyword matching. Returns nil if no match is found (caller
// should fall through to the AI-driven Resolve step).
func selectRepoByHypothesis(hypothesis string, repos []RepoConfig) []string {
	if hypothesis == "" || len(repos) == 0 {
		return nil
	}

	type rule struct {
		include []string
		exclude []string
	}
	prefix := strings.ToLower(hypothesis)

	var r rule
	switch {
	case strings.HasPrefix(prefix, "pb"):
		r = rule{
			include: []string{"operator", "daemon", "product"},
			exclude: []string{"test", "framework", "e2e", "deploy", "manifests"},
		}
	case strings.HasPrefix(prefix, "au"):
		r = rule{
			include: []string{"test", "framework", "e2e"},
			exclude: []string{},
		}
	case strings.HasPrefix(prefix, "en"):
		r = rule{
			include: []string{"config", "infra", "ci "},
			exclude: []string{},
		}
	default:
		return nil
	}

	var matched []string
	for _, repo := range repos {
		if repo.IsRedHerring {
			continue
		}
		purpose := strings.ToLower(repo.Purpose)

		excluded := false
		for _, kw := range r.exclude {
			if strings.Contains(purpose, kw) {
				excluded = true
				break
			}
		}
		if excluded {
			continue
		}

		for _, kw := range r.include {
			if strings.Contains(purpose, kw) {
				matched = append(matched, repo.Name)
				break
			}
		}
	}
	if len(matched) == 0 {
		return nil
	}
	return matched
}

// updateIDMaps updates the adapter's RCA/symptom ID maps after a case
// completes, so subsequent cases can reference prior RCAs/symptoms by store ID.
func updateIDMaps(mapper IDMappable, st store.Store, caseData *store.Case, gtCase GroundTruthCase, scenario *Scenario) {
	updated, err := st.GetCaseV2(caseData.ID)
	if err != nil || updated == nil {
		return
	}

	// Map ground truth RCA ID to store RCA ID
	if updated.RCAID != 0 && gtCase.RCAID != "" {
		mapper.SetRCAID(gtCase.RCAID, updated.RCAID)
	}

	// Map ground truth symptom ID to store symptom ID
	if updated.SymptomID != 0 && gtCase.SymptomID != "" {
		mapper.SetSymptomID(gtCase.SymptomID, updated.SymptomID)
	}
}

// pipeKey uniquely identifies a (version, job) combination for pipeline/launch/job mapping.
type pipeKey struct{ version, job string }

// stepName returns the short machine code (F0, F1, ...) for internal path tracking.
// Use vocabName() or vocabStagePath() to humanize for output.
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
	cleaned := cleanJSON(data)
	var result T
	if err := json.Unmarshal(cleaned, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// cleanJSON strips markdown code fences and leading/trailing whitespace from
// LLM responses. Models often wrap JSON in ```json ... ``` blocks. This
// handles: ```json\n{...}\n```, ```\n{...}\n```, and bare JSON.
func cleanJSON(data []byte) []byte {
	s := bytes.TrimSpace(data)
	if len(s) == 0 {
		return s
	}

	if bytes.HasPrefix(s, []byte("```")) {
		// Strip opening fence line
		if idx := bytes.IndexByte(s, '\n'); idx >= 0 {
			s = s[idx+1:]
		}
		// Strip closing fence
		if bytes.HasSuffix(s, []byte("```")) {
			s = s[:len(s)-3]
		}
		s = bytes.TrimSpace(s)
	}

	return s
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

// buildDatasetHealth creates a dataset health summary from the scenario.
func buildDatasetHealth(s *Scenario) *DatasetHealth {
	rcaMap := make(map[string]*GroundTruthRCA, len(s.RCAs))
	for i := range s.RCAs {
		rcaMap[s.RCAs[i].ID] = &s.RCAs[i]
	}

	dh := &DatasetHealth{
		VerifiedCount:  len(s.Cases),
		CandidateCount: len(s.Candidates),
	}
	for _, c := range s.Candidates {
		ci := CandidateInfo{
			CaseID: c.ID,
			RCAID:  c.RCAID,
		}
		if rca, ok := rcaMap[c.RCAID]; ok {
			ci.JiraID = rca.JiraID
			if len(rca.FixPRs) == 0 {
				ci.Reason = "no fix PR"
			} else {
				ci.Reason = "disputed/unverified"
			}
		}
		dh.Candidates = append(dh.Candidates, ci)
	}
	return dh
}

// evidenceOverlap computes set overlap between actual and expected evidence refs.
// Uses normalized path matching (partial path match allowed).
// Format: "repo:file_path:identifier". Matching is lenient:
//   1. Exact substring match (either direction)
//   2. filepath.Base match
//   3. Same repo + same file path (ignoring identifier suffix)
func evidenceOverlap(actual, expected []string) (found, total int) {
	total = len(expected)
	if total == 0 {
		return 0, 0
	}
	for _, exp := range expected {
		expNorm := filepath.Base(exp)
		matched := false
		for _, act := range actual {
			if strings.Contains(act, expNorm) || strings.Contains(exp, act) || act == exp {
				matched = true
				break
			}
		}
		if !matched {
			expParts := strings.SplitN(exp, ":", 3)
			if len(expParts) >= 2 {
				for _, act := range actual {
					if strings.HasPrefix(act, expParts[0]+":") && strings.Contains(act, expParts[1]) {
						matched = true
						break
					}
				}
			}
		}
		if matched {
			found++
		}
	}
	return found, total
}
