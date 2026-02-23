// Package calibrate implements the E2E calibration framework for Asterisk.
// It drives the F0–F6 pipeline against known ground truth (synthetic or real)
// and measures how closely the agent's conclusions match the known answers.
package calibrate

import (
	"github.com/dpopsuev/origami/dispatch"
)

// Scenario defines a complete calibration scenario with ground truth data.
type Scenario struct {
	Name        string               `json:"name"`
	Description string               `json:"description"`
	RCAs        []GroundTruthRCA     `json:"rcas"`
	Symptoms    []GroundTruthSymptom `json:"symptoms"`
	Cases       []GroundTruthCase    `json:"cases"`
	Candidates  []GroundTruthCase    `json:"candidates,omitempty"` // unverified cases tracked for dataset growth, never scored
	Workspace   WorkspaceConfig      `json:"workspace"`
}

// GroundTruthRCA is a known root cause for calibration scoring.
type GroundTruthRCA struct {
	ID               string   `json:"id"`                // e.g. "R1"
	Title            string   `json:"title"`
	Description      string   `json:"description"`
	DefectType       string   `json:"defect_type"`       // e.g. "pb001"
	Category         string   `json:"category"`           // product / automation / infra
	Component        string   `json:"component"`
	AffectedVersions []string `json:"affected_versions"`
	JiraID           string   `json:"jira_id,omitempty"`
	RequiredKeywords []string `json:"required_keywords"`  // for stub-mode semantic match
	KeywordThreshold int      `json:"keyword_threshold"`  // min keywords needed
	RelevantRepos    []string `json:"relevant_repos"`     // repos that should be selected for this RCA
	FixPRs           []string `json:"fix_prs,omitempty"`
	Verified         bool     `json:"verified"`                 // true = PR-proven ground truth; false = candidate (not scored)
	SmokingGun       string   `json:"smoking_gun,omitempty"`    // key phrase from the fix PR proving the root cause
}

// GroundTruthSymptom is a known symptom pattern.
type GroundTruthSymptom struct {
	ID           string `json:"id"`            // e.g. "S1"
	Name         string `json:"name"`
	ErrorPattern string `json:"error_pattern"` // regex-like pattern for matching
	Component    string `json:"component"`
	MapsToRCA    string `json:"maps_to_rca"`   // GroundTruthRCA.ID
}

// GroundTruthCase is a known test failure with expected outcomes.
type GroundTruthCase struct {
	ID           string   `json:"id"`           // e.g. "C1"
	Version      string   `json:"version"`      // e.g. "4.20"
	Job          string   `json:"job"`           // e.g. "[T-TSC]"
	TestName     string   `json:"test_name"`
	TestID       string   `json:"test_id,omitempty"` // RP item ID
	ErrorMessage string   `json:"error_message"` // planted error message
	LogSnippet   string   `json:"log_snippet"`   // planted log snippet
	SymptomID    string   `json:"symptom_id"`    // expected GroundTruthSymptom.ID
	RCAID        string   `json:"rca_id"`        // expected GroundTruthRCA.ID
	ExpectedPath []string `json:"expected_path"` // expected pipeline steps, e.g. ["F0","F1","F2","F3","F4","F5","F6"]

	// Expected per-step outcomes (for stub adapter responses)
	ExpectedRecall    *ExpectedRecall    `json:"expected_recall,omitempty"`
	ExpectedTriage    *ExpectedTriage    `json:"expected_triage,omitempty"`
	ExpectedResolve   *ExpectedResolve   `json:"expected_resolve,omitempty"`
	ExpectedInvest    *ExpectedInvest    `json:"expected_invest,omitempty"`
	ExpectedCorrelate *ExpectedCorrelate `json:"expected_correlate,omitempty"`
	ExpectedReview    *ExpectedReview    `json:"expected_review,omitempty"`

	// Flags for metric computation
	ExpectRecallHit   bool `json:"expect_recall_hit"`
	ExpectSkip        bool `json:"expect_skip"`         // infra/flake skip
	ExpectCascade     bool `json:"expect_cascade"`
	ExpectedLoops     int  `json:"expected_loops"`       // expected F3→F2→F3 loops

	// RP source fields (optional). When RPLaunchID > 0, the calibration runner
	// fetches real failure data from RP at runtime instead of using the embedded
	// ErrorMessage/LogSnippet. Ground truth expectations remain embedded.
	RPLaunchID     int    `json:"rp_launch_id,omitempty"`
	RPItemID       int    `json:"rp_item_id,omitempty"`
	RPIssueType    string `json:"rp_issue_type,omitempty"`    // populated at runtime by ResolveRPCases
	RPAutoAnalyzed bool   `json:"rp_auto_analyzed,omitempty"` // populated at runtime by ResolveRPCases

	// Shadow dialectic expectations (optional)
	ExpectedSynthesis string `json:"expected_synthesis,omitempty"` // expected SynthesisDecision if dialectic activates
}

// ExpectedRecall defines the ideal F0 output for a case.
type ExpectedRecall struct {
	Match      bool    `json:"match"`
	PriorRCAID int64   `json:"prior_rca_id,omitempty"`
	SymptomID  int64   `json:"symptom_id,omitempty"`
	Confidence float64 `json:"confidence"`
}

// ExpectedTriage defines the ideal F1 output.
type ExpectedTriage struct {
	SymptomCategory      string   `json:"symptom_category"`
	Severity             string   `json:"severity"`
	DefectTypeHypothesis string   `json:"defect_type_hypothesis"`
	CandidateRepos       []string `json:"candidate_repos"`
	SkipInvestigation    bool     `json:"skip_investigation"`
	CascadeSuspected     bool     `json:"cascade_suspected"`
}

// ExpectedResolve defines the ideal F2 output.
type ExpectedResolve struct {
	SelectedRepos []ExpectedResolveRepo `json:"selected_repos"`
}

// ExpectedResolveRepo is a simplified repo selection for ground truth.
type ExpectedResolveRepo struct {
	Name   string `json:"name"`
	Reason string `json:"reason"`
}

// ExpectedInvest defines the ideal F3 output.
type ExpectedInvest struct {
	RCAMessage       string   `json:"rca_message"`
	DefectType       string   `json:"defect_type"`
	Component        string   `json:"component"`
	ConvergenceScore float64  `json:"convergence_score"`
	EvidenceRefs     []string `json:"evidence_refs"`
}

// ExpectedCorrelate defines the ideal F4 output.
type ExpectedCorrelate struct {
	IsDuplicate       bool    `json:"is_duplicate"`
	LinkedRCAID       int64   `json:"linked_rca_id,omitempty"`
	Confidence        float64 `json:"confidence"`
	CrossVersionMatch bool    `json:"cross_version_match"`
}

// ExpectedReview defines the ideal F5 output.
type ExpectedReview struct {
	Decision string `json:"decision"` // approve
}

// WorkspaceConfig describes the context workspace for F2/F3.
type WorkspaceConfig struct {
	Repos []RepoConfig `json:"repos"`
}

// RepoConfig describes one repo in the workspace.
type RepoConfig struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	Purpose string `json:"purpose"`
	Branch  string `json:"branch"`

	// Ground truth: is this repo relevant to any RCA?
	RelevantToRCAs []string `json:"relevant_to_rcas,omitempty"`
	IsRedHerring   bool     `json:"is_red_herring,omitempty"`
}

// --- Metric types ---

// Metric is a single calibration metric with value, threshold, and pass/fail.
type Metric struct {
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	Value     float64 `json:"value"`
	Threshold float64 `json:"threshold"`
	Pass      bool    `json:"pass"`
	Detail    string  `json:"detail"` // e.g. "10/12"
}

// MetricSet holds all computed metrics for a calibration run.
type MetricSet struct {
	Structured []Metric `json:"structured"` // M1-M8
	Workspace  []Metric `json:"workspace"`  // M9-M11
	Evidence   []Metric `json:"evidence"`   // M12-M13
	Semantic   []Metric `json:"semantic"`   // M14-M15
	Pipeline   []Metric `json:"pipeline"`   // M16-M18
	Aggregate  []Metric `json:"aggregate"`  // M19-M20
}

// AllMetrics returns all metrics as a flat list.
func (ms *MetricSet) AllMetrics() []Metric {
	var all []Metric
	all = append(all, ms.Structured...)
	all = append(all, ms.Workspace...)
	all = append(all, ms.Evidence...)
	all = append(all, ms.Semantic...)
	all = append(all, ms.Pipeline...)
	all = append(all, ms.Aggregate...)
	return all
}

// PassCount returns (passed, total).
func (ms *MetricSet) PassCount() (int, int) {
	all := ms.AllMetrics()
	passed := 0
	for _, m := range all {
		if m.Pass {
			passed++
		}
	}
	return passed, len(all)
}

// DatasetHealth summarizes the ground truth dataset composition.
type DatasetHealth struct {
	VerifiedCount  int             `json:"verified_count"`
	CandidateCount int             `json:"candidate_count"`
	Candidates     []CandidateInfo `json:"candidates,omitempty"`
}

// CandidateInfo describes an unverified candidate case.
type CandidateInfo struct {
	CaseID string `json:"case_id"`
	RCAID  string `json:"rca_id"`
	JiraID string `json:"jira_id,omitempty"`
	Reason string `json:"reason"`
}

// CalibrationReport is the final output of a calibration run.
type CalibrationReport struct {
	Scenario     string           `json:"scenario"`
	Adapter      string           `json:"adapter"`
	Runs         int              `json:"runs"`
	SuiteID      int64            `json:"suite_id"`               // last run's suite ID; used by transcript weaver
	BasePath     string           `json:"-"`                      // artifact root; not serialized
	Metrics      MetricSet        `json:"metrics"`
	CaseResults  []CaseResult     `json:"case_results"`
	RunMetrics   []MetricSet            `json:"run_metrics,omitempty"`  // per-run for variance
	Tokens       *dispatch.TokenSummary  `json:"tokens,omitempty"`      // populated when TokenTracker is present
	Dataset      *DatasetHealth          `json:"dataset,omitempty"`     // ground truth composition
}

// CaseResult captures the per-case investigation outcome.
type CaseResult struct {
	CaseID       string   `json:"case_id"`       // ground truth ID, e.g. "C1"
	TestName     string   `json:"test_name"`
	Version      string   `json:"version"`
	Job          string   `json:"job"`
	StoreCaseID  int64    `json:"store_case_id"`  // internal store ID

	// Actual outcomes
	ActualDefectType  string   `json:"actual_defect_type"`
	ActualCategory    string   `json:"actual_category"`
	ActualRCAMessage  string   `json:"actual_rca_message"`
	ActualComponent   string   `json:"actual_component"`
	ActualPath        []string `json:"actual_path"`         // actual pipeline steps taken
	ActualRecallHit   bool     `json:"actual_recall_hit"`
	ActualSkip        bool     `json:"actual_skip"`
	ActualCascade     bool     `json:"actual_cascade"`
	ActualLoops       int      `json:"actual_loops"`
	ActualEvidenceRefs []string `json:"actual_evidence_refs"`
	ActualSelectedRepos []string `json:"actual_selected_repos"`
	ActualRCAID       int64    `json:"actual_rca_id"`
	ActualConvergence float64  `json:"actual_convergence"`

	// RP-provided classification (populated for RP-sourced cases)
	RPIssueType    string `json:"rp_issue_type,omitempty"`
	RPAutoAnalyzed bool   `json:"rp_auto_analyzed,omitempty"`

	// Token tracking (populated when dispatch.TokenTracker is present)
	PromptTokensTotal   int   `json:"prompt_tokens_total,omitempty"`
	ArtifactTokensTotal int   `json:"artifact_tokens_total,omitempty"`
	StepCount           int   `json:"step_count,omitempty"`
	WallClockMs         int64 `json:"wall_clock_ms,omitempty"`

	// Per-case scoring
	DefectTypeCorrect  bool    `json:"defect_type_correct"`
	CategoryCorrect    bool    `json:"category_correct"`
	PathCorrect        bool    `json:"path_correct"`
	ComponentCorrect   bool    `json:"component_correct"`
	SemanticScore      float64 `json:"semantic_score"` // 0-1

	// Shadow dialectic results (populated when dialectic is enabled and activates)
	DialecticActivated  bool   `json:"dialectic_activated,omitempty"`
	DialecticSynthesis  string `json:"dialectic_synthesis,omitempty"`
	DialecticFlipped    bool   `json:"dialectic_flipped,omitempty"`
	DialecticNegations  int    `json:"dialectic_negations,omitempty"`
	DialecticFinalDefect string `json:"dialectic_final_defect,omitempty"`

	// Pipeline error (non-empty when the case failed during execution)
	PipelineError string `json:"pipeline_error,omitempty"`
}
