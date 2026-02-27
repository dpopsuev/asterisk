// Package orchestrate implements the F0–F6 prompt pipeline engine.
// It evaluates heuristics, fills templates, persists intermediate artifacts,
// controls loops, and manages per-case state.
package rca

// PipelineStep represents a step in the F0-F6 (Light) or D0-D4 (Shadow) pipeline.
type PipelineStep string

const (
	StepInit       PipelineStep = "INIT"
	StepF0Recall   PipelineStep = "F0_RECALL"
	StepF1Triage   PipelineStep = "F1_TRIAGE"
	StepF2Resolve  PipelineStep = "F2_RESOLVE"
	StepF3Invest   PipelineStep = "F3_INVESTIGATE"
	StepF4Correlate PipelineStep = "F4_CORRELATE"
	StepF5Review   PipelineStep = "F5_REVIEW"
	StepF6Report   PipelineStep = "F6_REPORT"
	StepDone       PipelineStep = "DONE"

	StepD0Indict  PipelineStep = "D0_INDICT"
	StepD1Discover PipelineStep = "D1_DISCOVER"
	StepD2Defend  PipelineStep = "D2_DEFEND"
	StepD3Hearing PipelineStep = "D3_HEARING"
	StepD4Verdict PipelineStep = "D4_VERDICT"
	StepDialecticDone PipelineStep = "DIALECTIC_DONE"
)

// Family returns the prompt family name (for directory/file naming).
func (s PipelineStep) Family() string {
	switch s {
	case StepF0Recall:
		return "recall"
	case StepF1Triage:
		return "triage"
	case StepF2Resolve:
		return "resolve"
	case StepF3Invest:
		return "investigate"
	case StepF4Correlate:
		return "correlate"
	case StepF5Review:
		return "review"
	case StepF6Report:
		return "report"
	case StepD0Indict:
		return "indict"
	case StepD1Discover:
		return "discover"
	case StepD2Defend:
		return "defend"
	case StepD3Hearing:
		return "hearing"
	case StepD4Verdict:
		return "verdict"
	default:
		return ""
	}
}

// IsDialecticStep returns true if the step belongs to the D0-D4 Shadow pipeline.
func (s PipelineStep) IsDialecticStep() bool {
	switch s {
	case StepD0Indict, StepD1Discover, StepD2Defend, StepD3Hearing, StepD4Verdict:
		return true
	default:
		return false
	}
}

// CaseState tracks per-case progress through the pipeline.
// Persisted to disk (JSON) so the orchestrator can resume across CLI invocations.
type CaseState struct {
	CaseID      int64            `json:"case_id"`
	SuiteID     int64            `json:"suite_id"`
	CurrentStep PipelineStep     `json:"current_step"`
	LoopCounts  map[string]int   `json:"loop_counts"`  // e.g. "investigate": 2
	Status      string           `json:"status"`        // running, paused, done, error
	History     []StepRecord     `json:"history"`       // log of completed steps
}

// StepRecord logs a completed step with its outcome.
type StepRecord struct {
	Step        PipelineStep `json:"step"`
	Outcome     string       `json:"outcome"`      // e.g. "recall-hit", "triage-investigate"
	HeuristicID string       `json:"heuristic_id"` // which heuristic rule matched
	Timestamp   string       `json:"timestamp"`    // ISO 8601
}

// HeuristicAction is the result of evaluating a heuristic: what step to go to next
// and any context to carry forward.
type HeuristicAction struct {
	NextStep         PipelineStep       `json:"next_step"`
	ContextAdditions map[string]any     `json:"context_additions,omitempty"`
	Explanation      string             `json:"explanation"`
}

// HeuristicRule is a named decision rule that the engine evaluates against
// the current artifact/state to determine the next step.
type HeuristicRule struct {
	ID          string       `json:"id"`          // e.g. "H1"
	Name        string       `json:"name"`        // e.g. "recall-hit"
	SignalField string       `json:"signal_field"` // field in the artifact to inspect
	Stage       PipelineStep `json:"stage"`        // which stage output this applies to
	Evaluate    func(artifact any, state *CaseState) *HeuristicAction `json:"-"`
}

// --- Typed intermediate artifacts (one per family) ---

// RecallResult is the F0 output.
type RecallResult struct {
	Match       bool    `json:"match"`
	PriorRCAID  int64   `json:"prior_rca_id,omitempty"`
	SymptomID   int64   `json:"symptom_id,omitempty"`
	Confidence  float64 `json:"confidence"`
	Reasoning   string  `json:"reasoning"`
	IsRegression bool   `json:"is_regression,omitempty"`
}

// TriageResult is the F1 output.
type TriageResult struct {
	SymptomCategory      string   `json:"symptom_category"`
	Severity             string   `json:"severity,omitempty"`
	DefectTypeHypothesis string   `json:"defect_type_hypothesis"`
	CandidateRepos       []string `json:"candidate_repos"`
	SkipInvestigation    bool     `json:"skip_investigation"`
	ClockSkewSuspected   bool     `json:"clock_skew_suspected,omitempty"`
	CascadeSuspected     bool     `json:"cascade_suspected,omitempty"`
	DataQualityNotes     string   `json:"data_quality_notes,omitempty"`
}

// ResolveResult is the F2 output.
type ResolveResult struct {
	SelectedRepos    []RepoSelection `json:"selected_repos"`
	CrossRefStrategy string          `json:"cross_ref_strategy,omitempty"`
}

// RepoSelection describes one selected repo from F2.
type RepoSelection struct {
	Name       string   `json:"name"`
	Path       string   `json:"path"`
	FocusPaths []string `json:"focus_paths,omitempty"`
	Branch     string   `json:"branch,omitempty"`
	Reason     string   `json:"reason"`
}

// InvestigateArtifact is the F3 output (main investigation artifact).
type InvestigateArtifact struct {
	LaunchID         string   `json:"launch_id"`
	CaseIDs          []int    `json:"case_ids"`
	RCAMessage       string   `json:"rca_message"`
	DefectType       string   `json:"defect_type"`
	Component        string   `json:"component,omitempty"`
	ConvergenceScore float64  `json:"convergence_score"`
	EvidenceRefs     []string `json:"evidence_refs"`
}

// CorrelateResult is the F4 output.
type CorrelateResult struct {
	IsDuplicate       bool    `json:"is_duplicate"`
	LinkedRCAID       int64   `json:"linked_rca_id,omitempty"`
	Confidence        float64 `json:"confidence"`
	Reasoning         string  `json:"reasoning"`
	CrossVersionMatch bool    `json:"cross_version_match,omitempty"`
	AffectedVersions  []string `json:"affected_versions,omitempty"`
}

// ReviewDecision is the F5 output.
type ReviewDecision struct {
	Decision      string        `json:"decision"` // approve, reassess, overturn
	HumanOverride *HumanOverride `json:"human_override,omitempty"`
	LoopTarget    PipelineStep  `json:"loop_target,omitempty"` // for reassess
}

// HumanOverride is the human's correction in an overturn decision.
type HumanOverride struct {
	DefectType string `json:"defect_type"`
	RCAMessage string `json:"rca_message"`
}
