package calibrate

import (
	"asterisk/internal/format"
	"fmt"
	"sync"
	"time"
)

// TokenRecord captures token usage for a single pipeline step dispatch.
type TokenRecord struct {
	CaseID         string    `json:"case_id"`
	Step           string    `json:"step"`
	PromptBytes    int       `json:"prompt_bytes"`
	ArtifactBytes  int       `json:"artifact_bytes"`
	PromptTokens   int       `json:"prompt_tokens"`   // estimated: bytes / 4
	ArtifactTokens int       `json:"artifact_tokens"` // estimated: bytes / 4
	Timestamp      time.Time `json:"timestamp"`
	WallClockMs    int64     `json:"wall_clock_ms"`
}

// CaseTokenSummary aggregates token usage for a single case.
type CaseTokenSummary struct {
	PromptTokens   int   `json:"prompt_tokens"`
	ArtifactTokens int   `json:"artifact_tokens"`
	TotalTokens    int   `json:"total_tokens"`
	Steps          int   `json:"steps"`
	WallClockMs    int64 `json:"wall_clock_ms"`
}

// StepTokenSummary aggregates token usage across all cases for a single step.
type StepTokenSummary struct {
	PromptTokens   int `json:"prompt_tokens"`
	ArtifactTokens int `json:"artifact_tokens"`
	TotalTokens    int `json:"total_tokens"`
	Invocations    int `json:"invocations"`
}

// CostConfig holds pricing for token cost estimation.
type CostConfig struct {
	InputPricePerMToken  float64 // USD per million input tokens (default: $3)
	OutputPricePerMToken float64 // USD per million output tokens (default: $15)
}

// DefaultCostConfig returns typical LLM pricing.
func DefaultCostConfig() CostConfig {
	return CostConfig{
		InputPricePerMToken:  3.0,
		OutputPricePerMToken: 15.0,
	}
}

// TokenSummary is the aggregate view of all token usage in a calibration run.
type TokenSummary struct {
	TotalPromptTokens   int                      `json:"total_prompt_tokens"`
	TotalArtifactTokens int                      `json:"total_artifact_tokens"`
	TotalTokens         int                      `json:"total_tokens"`
	TotalCostUSD        float64                  `json:"total_cost_usd"`
	PerCase             map[string]CaseTokenSummary `json:"per_case"`
	PerStep             map[string]StepTokenSummary `json:"per_step"`
	TotalSteps          int                      `json:"total_steps"`
	TotalWallClockMs    int64                    `json:"total_wall_clock_ms"`
}

// TokenTracker records and summarizes token usage.
type TokenTracker interface {
	Record(r TokenRecord)
	Summary() TokenSummary
}

// InMemoryTokenTracker is a thread-safe in-memory token tracker.
type InMemoryTokenTracker struct {
	mu      sync.Mutex
	records []TokenRecord
	cost    CostConfig
}

// NewTokenTracker creates an InMemoryTokenTracker with default cost config.
func NewTokenTracker() *InMemoryTokenTracker {
	return &InMemoryTokenTracker{cost: DefaultCostConfig()}
}

// NewTokenTrackerWithCost creates an InMemoryTokenTracker with custom pricing.
func NewTokenTrackerWithCost(c CostConfig) *InMemoryTokenTracker {
	return &InMemoryTokenTracker{cost: c}
}

// Record appends a token record (thread-safe).
func (t *InMemoryTokenTracker) Record(r TokenRecord) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.records = append(t.records, r)
}

// Summary computes the aggregate token summary.
func (t *InMemoryTokenTracker) Summary() TokenSummary {
	t.mu.Lock()
	defer t.mu.Unlock()

	s := TokenSummary{
		PerCase: make(map[string]CaseTokenSummary),
		PerStep: make(map[string]StepTokenSummary),
	}

	for _, r := range t.records {
		s.TotalPromptTokens += r.PromptTokens
		s.TotalArtifactTokens += r.ArtifactTokens
		s.TotalSteps++
		s.TotalWallClockMs += r.WallClockMs

		cs := s.PerCase[r.CaseID]
		cs.PromptTokens += r.PromptTokens
		cs.ArtifactTokens += r.ArtifactTokens
		cs.TotalTokens += r.PromptTokens + r.ArtifactTokens
		cs.Steps++
		cs.WallClockMs += r.WallClockMs
		s.PerCase[r.CaseID] = cs

		ss := s.PerStep[r.Step]
		ss.PromptTokens += r.PromptTokens
		ss.ArtifactTokens += r.ArtifactTokens
		ss.TotalTokens += r.PromptTokens + r.ArtifactTokens
		ss.Invocations++
		s.PerStep[r.Step] = ss
	}

	s.TotalTokens = s.TotalPromptTokens + s.TotalArtifactTokens

	inputCost := float64(s.TotalPromptTokens) / 1_000_000 * t.cost.InputPricePerMToken
	outputCost := float64(s.TotalArtifactTokens) / 1_000_000 * t.cost.OutputPricePerMToken
	s.TotalCostUSD = inputCost + outputCost

	return s
}

// FormatTokenSummary returns a human-readable token and cost section.
func FormatTokenSummary(s TokenSummary) string {
	avgPerCase := 0
	if len(s.PerCase) > 0 {
		avgPerCase = s.TotalTokens / len(s.PerCase)
	}
	avgPerStep := 0
	if s.TotalSteps > 0 {
		avgPerStep = s.TotalTokens / s.TotalSteps
	}

	wallSec := float64(s.TotalWallClockMs) / 1000.0
	minutes := int(wallSec) / 60
	seconds := int(wallSec) % 60

	tbl := format.NewTable(format.ASCII)
	tbl.Header("Metric", "Value")
	tbl.Columns(
		format.ColumnConfig{Number: 1, Align: format.AlignLeft},
		format.ColumnConfig{Number: 2, Align: format.AlignRight},
	)
	tbl.Row("Total prompts", fmt.Sprintf("%d tokens ($%.4f)", s.TotalPromptTokens, float64(s.TotalPromptTokens)/1_000_000*3.0))
	tbl.Row("Total artifacts", fmt.Sprintf("%d tokens ($%.4f)", s.TotalArtifactTokens, float64(s.TotalArtifactTokens)/1_000_000*15.0))
	tbl.Row("Total", fmt.Sprintf("%d tokens ($%.4f)", s.TotalTokens, s.TotalCostUSD))
	tbl.Row("Per case avg", fmt.Sprintf("%d tokens", avgPerCase))
	tbl.Row("Per step avg", fmt.Sprintf("%d tokens", avgPerStep))
	tbl.Row("Steps", fmt.Sprintf("%d", s.TotalSteps))
	tbl.Row("Wall clock", fmt.Sprintf("%dm %ds", minutes, seconds))

	return "=== Token & Cost ===\n" + tbl.String() + "\n"
}

// EstimateTokens converts byte count to estimated token count (bytes / 4).
func EstimateTokens(bytes int) int {
	if bytes <= 0 {
		return 0
	}
	return bytes / 4
}
