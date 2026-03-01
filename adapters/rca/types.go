// Package orchestrate implements the F0–F6 prompt circuit engine.
// It evaluates heuristics, fills templates, persists intermediate artifacts,
// controls loops, and manages per-case state.
package rca

// Thresholds holds configurable threshold values for circuit edge evaluation.
type Thresholds struct {
	RecallHit             float64 // when to short-circuit on prior RCA (default 0.80)
	RecallUncertain       float64 // below this = definite miss (default 0.40)
	ConvergenceSufficient float64 // when to stop investigating (default 0.70)
	MaxInvestigateLoops   int     // cap on F3→F2→F3 iterations (default 2)
	CorrelateDup          float64 // when to auto-link cases to same RCA (default 0.80)
}

// DefaultThresholds returns conservative default thresholds.
func DefaultThresholds() Thresholds {
	return Thresholds{
		RecallHit:             0.80,
		RecallUncertain:       0.40,
		ConvergenceSufficient: 0.50,
		MaxInvestigateLoops:   1,
		CorrelateDup:          0.80,
	}
}

// CircuitStep represents a step in the F0-F6 (Thesis) or D0-D4 (Antithesis) circuit.
type CircuitStep string

const (
	StepInit       CircuitStep = "INIT"
	StepF0Recall   CircuitStep = "F0_RECALL"
	StepF1Triage   CircuitStep = "F1_TRIAGE"
	StepF2Resolve  CircuitStep = "F2_RESOLVE"
	StepF3Invest   CircuitStep = "F3_INVESTIGATE"
	StepF4Correlate CircuitStep = "F4_CORRELATE"
	StepF5Review   CircuitStep = "F5_REVIEW"
	StepF6Report   CircuitStep = "F6_REPORT"
	StepDone       CircuitStep = "DONE"

	StepD0Indict  CircuitStep = "D0_INDICT"
	StepD1Discover CircuitStep = "D1_DISCOVER"
	StepD2Defend  CircuitStep = "D2_DEFEND"
	StepD3Hearing CircuitStep = "D3_HEARING"
	StepD4Verdict CircuitStep = "D4_VERDICT"
	StepDialecticDone CircuitStep = "DIALECTIC_DONE"
)

// Family returns the prompt family name (for directory/file naming).
func (s CircuitStep) Family() string {
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

// IsDialecticStep returns true if the step belongs to the D0-D4 Antithesis circuit.
func (s CircuitStep) IsDialecticStep() bool {
	switch s {
	case StepD0Indict, StepD1Discover, StepD2Defend, StepD3Hearing, StepD4Verdict:
		return true
	default:
		return false
	}
}

// CaseState tracks per-case progress through the circuit.
// Persisted to disk (JSON) so the orchestrator can resume across CLI invocations.
type CaseState struct {
	CaseID      int64            `json:"case_id"`
	SuiteID     int64            `json:"suite_id"`
	CurrentStep CircuitStep     `json:"current_step"`
	LoopCounts  map[string]int   `json:"loop_counts"`  // e.g. "investigate": 2
	Status      string           `json:"status"`        // running, paused, done, error
	History     []StepRecord     `json:"history"`       // log of completed steps
}

// StepRecord logs a completed step with its outcome.
type StepRecord struct {
	Step        CircuitStep `json:"step"`
	Outcome     string       `json:"outcome"`      // e.g. "recall-hit", "triage-investigate"
	HeuristicID string       `json:"heuristic_id"` // which heuristic rule matched
	Timestamp   string       `json:"timestamp"`    // ISO 8601
}

// HeuristicAction is the result of evaluating a heuristic: what step to go to next
// and any context to carry forward.
type HeuristicAction struct {
	NextStep         CircuitStep       `json:"next_step"`
	ContextAdditions map[string]any     `json:"context_additions,omitempty"`
	Explanation      string             `json:"explanation"`
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
	LaunchID         string    `json:"launch_id"`
	CaseIDs          []int     `json:"case_ids"`
	RCAMessage       string    `json:"rca_message"`
	DefectType       string    `json:"defect_type"`
	Component        string    `json:"component,omitempty"`
	ConvergenceScore float64   `json:"convergence_score"`
	EvidenceRefs     []string  `json:"evidence_refs"`
	GapBrief         *GapBrief `json:"gap_brief,omitempty"`
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
	LoopTarget    CircuitStep  `json:"loop_target,omitempty"` // for reassess
}

// HumanOverride is the human's correction in an overturn decision.
type HumanOverride struct {
	DefectType string `json:"defect_type"`
	RCAMessage string `json:"rca_message"`
}
