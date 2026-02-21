package metacal

import (
	"time"

	"asterisk/pkg/framework"
)

// DiscoveryConfig controls the recursive discovery loop.
type DiscoveryConfig struct {
	MaxIterations      int    `json:"max_iterations"`
	ProbeID            string `json:"probe_id"`
	TerminateOnRepeat  bool   `json:"terminate_on_repeat"`
}

// DefaultConfig returns a sensible starting configuration.
func DefaultConfig() DiscoveryConfig {
	return DiscoveryConfig{
		MaxIterations:     15,
		ProbeID:           "refactor-v1",
		TerminateOnRepeat: true,
	}
}

// ProbeScore holds the scored dimensions from a refactoring probe.
type ProbeScore struct {
	Renames           int     `json:"renames"`
	FunctionSplits    int     `json:"function_splits"`
	CommentsAdded     int     `json:"comments_added"`
	StructuralChanges int     `json:"structural_changes"`
	TotalScore        float64 `json:"total_score"`
}

// ProbeResult captures the raw output and scored result of a single probe.
type ProbeResult struct {
	ProbeID   string        `json:"probe_id"`
	RawOutput string        `json:"raw_output"`
	Score     ProbeScore    `json:"score"`
	Elapsed   time.Duration `json:"elapsed_ns"`
}

// DiscoveryResult records one iteration of the negation discovery loop.
type DiscoveryResult struct {
	Iteration       int                    `json:"iteration"`
	Model           framework.ModelIdentity `json:"model"`
	ExclusionPrompt string                 `json:"exclusion_prompt"`
	Probe           ProbeResult            `json:"probe"`
	Timestamp       time.Time              `json:"timestamp"`
}

// RunReport is the complete output of a discovery run. Persisted as
// append-only JSON — each run gets its own file, never overwritten.
type RunReport struct {
	RunID        string                    `json:"run_id"`
	StartTime    time.Time                 `json:"start_time"`
	EndTime      time.Time                 `json:"end_time"`
	Config       DiscoveryConfig           `json:"config"`
	Results      []DiscoveryResult         `json:"results"`
	UniqueModels []framework.ModelIdentity `json:"unique_models"`
	TermReason   string                    `json:"termination_reason"`
}

// ModelNames returns a sorted list of unique model names from the report.
func (r *RunReport) ModelNames() []string {
	names := make([]string, len(r.UniqueModels))
	for i, m := range r.UniqueModels {
		names[i] = m.String()
	}
	return names
}
