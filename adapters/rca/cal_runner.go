package rca

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	framework "github.com/dpopsuev/origami"
	cal "github.com/dpopsuev/origami/calibrate"
	"github.com/dpopsuev/origami/dispatch"
	"github.com/dpopsuev/origami/logging"

	"github.com/dpopsuev/origami/adapters/rp"
	"asterisk/adapters/store"
)

// IDMappable is implemented by transformers that track ground-truth-to-store
// ID mappings (e.g. stubTransformer). Used by calibration to wire recall/correlate
// cross-case references.
type IDMappable interface {
	SetRCAID(gtID string, storeID int64)
	SetSymptomID(gtID string, storeID int64)
}

// RunConfig holds configuration for a calibration run.
type RunConfig struct {
	Scenario     *Scenario
	Adapters     []*framework.Adapter // transformer adapter(s) for the circuit
	TransformerName string             // label for reports
	IDMapper     IDMappable           // optional; stub cross-case references
	Runs         int
	PromptDir    string
	Thresholds   Thresholds
	TokenTracker dispatch.TokenTracker // optional; when set, records per-step token usage
	Parallel     int          // number of parallel workers (default 1 = serial)
	TokenBudget  int          // max concurrent dispatches (token semaphore); 0 = Parallel
	BatchSize    int          // max signals per batch for batch-file dispatch mode; 0 = Parallel
	BasePath     string       // root directory for investigation artifacts; defaults to DefaultBasePath
	RPFetcher    rp.EnvelopeFetcher // optional; when set, RP-sourced cases fetch real failure data
	ScoreCard    *cal.ScoreCard // declarative metric definitions; loaded from YAML at startup

	GapConfidentThreshold    float64 // convergence >= this → confident (no gap brief); 0 uses default 0.80
	GapInconclusiveThreshold float64 // convergence < this → inconclusive (gap brief required); 0 uses default 0.50
}

// DefaultRunConfig returns defaults for calibration.
func DefaultRunConfig(scenario *Scenario, adapters []*framework.Adapter, transformerName string) RunConfig {
	return RunConfig{
		Scenario:                 scenario,
		Adapters:                 adapters,
		TransformerName:         transformerName,
		Runs:                     1,
		PromptDir:                ".cursor/prompts",
		Thresholds:               DefaultThresholds(),
		BasePath:                 DefaultBasePath,
		GapConfidentThreshold:    DefaultGapConfidentThreshold,
		GapInconclusiveThreshold: DefaultGapInconclusiveThreshold,
	}
}

// ResolvedGapConfidentThreshold returns the gap confident threshold,
// falling back to the default if zero.
func (c RunConfig) ResolvedGapConfidentThreshold() float64 {
	if c.GapConfidentThreshold > 0 {
		return c.GapConfidentThreshold
	}
	return DefaultGapConfidentThreshold
}

// ResolvedGapInconclusiveThreshold returns the gap inconclusive threshold,
// falling back to the default if zero.
func (c RunConfig) ResolvedGapInconclusiveThreshold() float64 {
	if c.GapInconclusiveThreshold > 0 {
		return c.GapInconclusiveThreshold
	}
	return DefaultGapInconclusiveThreshold
}

// RunCalibration executes the full calibration loop.
// For each run: create a fresh store, run all cases through the circuit, score.
// The context enables cancellation of in-flight work across all goroutines.
func RunCalibration(ctx context.Context, cfg RunConfig) (*CalibrationReport, error) {
	if cfg.BasePath == "" {
		cfg.BasePath = DefaultBasePath
	}
	if cfg.ScoreCard == nil {
		return nil, fmt.Errorf("RunConfig.ScoreCard is required (load from scorecards/asterisk-rca.yaml)")
	}

	report := &CalibrationReport{
		CalibrationReport: cal.CalibrationReport{
			Scenario:    cfg.Scenario.Name,
			Transformer: cfg.TransformerName,
			Runs:        cfg.Runs,
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
// When cfg.Parallel > 1, cases are processed concurrently via errgroup.
func runSingleCalibration(ctx context.Context, cfg RunConfig) ([]CaseResult, int64, error) {
	st, err := store.OpenMemory()
	if err != nil {
		return nil, 0, fmt.Errorf("open memory store: %w", err)
	}

	suite := &store.InvestigationSuite{Name: cfg.Scenario.Name, Status: "active"}
	suiteID, err := st.CreateSuite(suite)
	if err != nil {
		return nil, 0, fmt.Errorf("create suite: %w", err)
	}

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

	circuitMap := make(map[pipeKey]int64)
	jobMap := make(map[pipeKey]int64)
	launchMap := make(map[pipeKey]int64)

	for _, c := range cfg.Scenario.Cases {
		pk := pipeKey{c.Version, c.Job}
		if _, exists := circuitMap[pk]; !exists {
			pipe := &store.Circuit{
				SuiteID: suiteID, VersionID: versionMap[c.Version],
				Name: fmt.Sprintf("CI %s %s", c.Version, c.Job), Status: "complete",
			}
			pipeID, err := st.CreateCircuit(pipe)
			if err != nil {
				return nil, suiteID, fmt.Errorf("create circuit: %w", err)
			}
			circuitMap[pk] = pipeID

			launch := &store.Launch{
				CircuitID: pipeID, RPLaunchID: 0,
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

	idMapper, hasIDMap := cfg.IDMapper, cfg.IDMapper != nil
	logger := logging.New("calibrate")

	type caseEntry struct {
		gtCase   GroundTruthCase
		caseData *store.Case
		caseDir  string
	}
	entries := make([]caseEntry, len(cfg.Scenario.Cases))
	batchCases := make([]framework.BatchCase, len(cfg.Scenario.Cases))
	catalog := ScenarioToCatalog(cfg.Scenario.Workspace)

	for i, gtCase := range cfg.Scenario.Cases {
		pk := pipeKey{gtCase.Version, gtCase.Job}
		caseData := &store.Case{
			JobID:        jobMap[pk],
			LaunchID:     launchMap[pk],
			RPItemID:     i + 1,
			Name:         gtCase.TestName,
			Status:       "open",
			ErrorMessage: gtCase.ErrorMessage,
			LogSnippet:   gtCase.LogSnippet,
		}
		caseID, err := st.CreateCase(caseData)
		if err != nil {
			return nil, suiteID, fmt.Errorf("create case %s: %w", gtCase.ID, err)
		}
		caseData.ID = caseID

		env := &rp.Envelope{
			Name:        caseData.Name,
			FailureList: []rp.FailureItem{{Name: caseData.Name}},
		}
		caseDir, _ := EnsureCaseDir(cfg.BasePath, suiteID, caseData.ID)

		storeAdapter := &framework.Adapter{
			Namespace: "store",
			Name:      "rca-store-hooks",
			Hooks:     StoreHooks(st, caseData),
		}
		injectAdapter := &framework.Adapter{
			Namespace: "inject",
			Name:      "rca-inject-hooks",
			Hooks:     InjectHooks(st, caseData, env, catalog, caseDir),
		}

		entries[i] = caseEntry{gtCase: gtCase, caseData: caseData, caseDir: caseDir}

		adapters := make([]*framework.Adapter, len(cfg.Adapters), len(cfg.Adapters)+2)
		copy(adapters, cfg.Adapters)
		adapters = append(adapters, storeAdapter, injectAdapter)

		batchCases[i] = framework.BatchCase{
			ID: gtCase.ID,
			Context: map[string]any{
				KeyCaseData:  caseData,
				KeyEnvelope:  env,
				KeyCaseDir:   caseDir,
				KeyCaseLabel: gtCase.ID,
			},
			Adapters: adapters,
		}
	}

	def, err := AsteriskCircuitDef(cfg.Thresholds)
	if err != nil {
		return nil, suiteID, fmt.Errorf("load circuit def: %w", err)
	}

	var mu sync.Mutex
	batchResults := framework.BatchWalk(ctx, framework.BatchWalkConfig{
		Def:      def,
		Shared:   framework.GraphRegistries{},
		Cases:    batchCases,
		Parallel: cfg.Parallel,
		OnCaseComplete: func(i int, _ framework.BatchWalkResult) {
			if hasIDMap {
				mu.Lock()
				updateIDMaps(idMapper, st, entries[i].caseData, entries[i].gtCase, cfg.Scenario)
				mu.Unlock()
			}
		},
	})

	results := make([]CaseResult, len(entries))
	for i, br := range batchResults {
		entry := entries[i]
		logger.Info("processed case",
			"case_id", entry.gtCase.ID, "index", i+1, "total", len(entries), "test", entry.gtCase.TestName)

		results[i] = collectCaseResult(br, entry.gtCase, entry.caseData, entry.caseDir, suiteID, st, cfg)
	}

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
	r.PathCorrect = cal.PathsEqual(r.ActualPath, gt.ExpectedPath)

	// Defect type and component — look up ground truth RCA
	if gt.RCAID != "" {
		for _, gtRCA := range scenario.RCAs {
			if gtRCA.ID == gt.RCAID {
				r.DefectTypeCorrect = (r.ActualDefectType == gtRCA.DefectType)
				r.ComponentCorrect = (r.ActualComponent == gtRCA.Component) ||
					(r.ActualRCAMessage != "" && strings.Contains(
						strings.ToLower(r.ActualRCAMessage),
						strings.ToLower(gtRCA.Component)))
				break
			}
		}
	}
}

// collectCaseResult builds a CaseResult from a BatchWalkResult, extracting
// step metrics, writing artifacts, and reading final store state.
func collectCaseResult(
	br framework.BatchWalkResult,
	gtCase GroundTruthCase,
	caseData *store.Case,
	caseDir string,
	suiteID int64,
	st store.Store,
	cfg RunConfig,
) CaseResult {
	result := CaseResult{
		CaseID:         gtCase.ID,
		TestName:       gtCase.TestName,
		Version:        gtCase.Version,
		Job:            gtCase.Job,
		StoreCaseID:    caseData.ID,
		RPIssueType:    gtCase.RPIssueType,
		RPAutoAnalyzed: gtCase.RPAutoAnalyzed,
	}

	if br.Error != nil {
		result.CircuitError = br.Error.Error()
		return result
	}

	for _, nodeName := range br.Path {
		result.ActualPath = append(result.ActualPath, stepName(NodeNameToStep(nodeName)))
	}

	for nodeName, art := range br.StepArtifacts {
		step := NodeNameToStep(nodeName)
		extractStepMetrics(&result, step, art.Raw(), gtCase)
		if err := WriteArtifact(caseDir, ArtifactFilename(step), art.Raw()); err != nil {
			logging.New("calibrate").Warn("write artifact", "step", step, "error", err)
		}
	}

	if br.State != nil {
		ws := br.State
		history := make([]StepRecord, 0, len(ws.History))
		for _, h := range ws.History {
			history = append(history, StepRecord{
				Step:        NodeNameToStep(h.Node),
				Outcome:     h.Outcome,
				HeuristicID: h.EdgeID,
				Timestamp:   h.Timestamp,
			})
		}
		caseState := &CaseState{
			CaseID:      caseData.ID,
			SuiteID:     suiteID,
			CurrentStep: NodeNameToStep(ws.CurrentNode),
			Status:      ws.Status,
			LoopCounts:  ws.LoopCounts,
			History:     history,
		}
		if err := WriteArtifact(caseDir, "state.json", caseState); err != nil {
			logging.New("calibrate").Warn("save final state", "error", err)
		}
		result.ActualLoops = ws.LoopCounts["investigate"]
	}

	updatedCase, err := st.GetCase(caseData.ID)
	if err == nil && updatedCase != nil {
		result.ActualRCAID = updatedCase.RCAID
		if updatedCase.RCAID != 0 {
			rcaRec, err := st.GetRCA(updatedCase.RCAID)
			if err == nil && rcaRec != nil {
				result.ActualDefectType = rcaRec.DefectType
				result.ActualRCAMessage = rcaRec.Description
				result.ActualComponent = rcaRec.Component
				result.ActualConvergence = rcaRec.ConvergenceScore
			}
		}
	}

	return result
}


// extractStepMetrics populates CaseResult fields from per-step artifacts.
func extractStepMetrics(result *CaseResult, step CircuitStep, artifact any, gt GroundTruthCase) {
	switch step {
	case StepF0Recall:
		if r, ok := artifact.(*RecallResult); ok && r != nil {
			result.ActualRecallHit = r.Match && r.Confidence >= 0.80
		}
	case StepF1Triage:
		if r, ok := artifact.(*TriageResult); ok && r != nil {
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
	case StepF2Resolve:
		if r, ok := artifact.(*ResolveResult); ok && r != nil {
			result.ActualSelectedRepos = result.ActualSelectedRepos[:0]
			for _, repo := range r.SelectedRepos {
				result.ActualSelectedRepos = append(result.ActualSelectedRepos, repo.Name)
			}
		}
	case StepF3Invest:
		if r, ok := artifact.(*InvestigateArtifact); ok && r != nil {
			result.ActualDefectType = r.DefectType
			result.ActualRCAMessage = r.RCAMessage
			result.ActualEvidenceRefs = r.EvidenceRefs
			result.ActualConvergence = r.ConvergenceScore
			if r.Component != "" {
				result.ActualComponent = r.Component
			}
			if r.GapBrief != nil {
				result.VerdictConfidence = r.GapBrief.Verdict
				result.EvidenceGaps = r.GapBrief.GapItems
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

// updateIDMaps updates the transformer's RCA/symptom ID maps after a case
// completes, so subsequent cases can reference prior RCAs/symptoms by store ID.
func updateIDMaps(mapper IDMappable, st store.Store, caseData *store.Case, gtCase GroundTruthCase, scenario *Scenario) {
	updated, err := st.GetCase(caseData.ID)
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

// pipeKey uniquely identifies a (version, job) combination for circuit/launch/job mapping.
type pipeKey struct{ version, job string }

// stepName returns the short machine code (F0, F1, ...) for internal path tracking.
// Use vocabName() or vocabStagePath() to humanize for output.
func stepName(s CircuitStep) string {
	m := map[CircuitStep]string{
		StepF0Recall:    "F0",
		StepF1Triage:    "F1",
		StepF2Resolve:   "F2",
		StepF3Invest:    "F3",
		StepF4Correlate: "F4",
		StepF5Review:    "F5",
		StepF6Report:    "F6",
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
		if rcaRec, ok := rcaMap[c.RCAID]; ok {
			ci.JiraID = rcaRec.JiraID
			if len(rcaRec.FixPRs) == 0 {
				ci.Reason = "no fix PR"
			} else {
				ci.Reason = "disputed/unverified"
			}
		}
		dh.Candidates = append(dh.Candidates, ci)
	}
	return dh
}

