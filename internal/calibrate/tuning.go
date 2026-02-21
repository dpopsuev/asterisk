package calibrate

import (
	"fmt"
	"log/slog"
)

// QuickWin defines a targeted improvement to the calibration pipeline.
// Each QW is atomic: implement, re-calibrate, measure, commit or revert.
type QuickWin struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	MetricTarget string           `json:"metric_target"`
	Prereqs     []string          `json:"prereqs,omitempty"`
	Apply       func(cfg *RunConfig) error `json:"-"`
}

// TuningResult records the before/after measurement for a single QW.
type TuningResult struct {
	QWID        string  `json:"qw_id"`
	BaselineM19 float64 `json:"baseline_m19"`
	AfterM19    float64 `json:"after_m19"`
	Delta       float64 `json:"delta"`
	Applied     bool    `json:"applied"`
	Reverted    bool    `json:"reverted"`
	Error       string  `json:"error,omitempty"`
}

// TuningReport aggregates all QW results for a tuning session.
type TuningReport struct {
	Results          []TuningResult `json:"results"`
	FinalM19         float64        `json:"final_m19"`
	BaselineM19      float64        `json:"baseline_m19"`
	CumulativeDelta  float64        `json:"cumulative_delta"`
	QWsApplied       int            `json:"qws_applied"`
	QWsReverted      int            `json:"qws_reverted"`
	StopReason       string         `json:"stop_reason"`
}

// DefaultQuickWins returns the standard QW-1 through QW-4 scaffold.
// Implementation functions are nil (skeleton); they'll be wired when
// the rp-e2e-launch gate is complete.
func DefaultQuickWins() []QuickWin {
	return []QuickWin{
		{
			ID:           "QW-1",
			Name:         "Surface RP launch attributes in prompts",
			Description:  "Add LaunchAttributes to TemplateParams. Render as metadata table in triage/resolve prompts.",
			MetricTarget: "M12/M13 improvement via better component identification",
		},
		{
			ID:           "QW-2",
			Name:         "Wire Jira links from ExternalSystemIssues",
			Description:  "Extract ExternalSystemIssues from RP Issue. Pass Jira URLs into TemplateParams for triage/investigate.",
			MetricTarget: "M1/M15 improvement via direct evidence links",
			Prereqs:      []string{"QW-1"},
		},
		{
			ID:           "QW-3",
			Name:         "Tune BasicAdapter keyword maps",
			Description:  "Analyze misclassification patterns from E2E run. Adjust keyword-to-component mappings.",
			MetricTarget: "M1 improvement on recurring misclassifications",
			Prereqs:      []string{"QW-1", "QW-2"},
		},
		{
			ID:           "QW-4",
			Name:         "Evidence gap-driven tuning",
			Description:  "Use EvidenceGapBrief output to prioritize tuning actions. Gap categories map to QW-1/2/3 code locations.",
			MetricTarget: "Data-driven tuning prioritization",
			Prereqs:      []string{"QW-1", "QW-2", "QW-3"},
		},
	}
}

// TuningRunner executes a sequence of QuickWins with before/after measurement.
// It stops when a stop condition is met (target M19, no improvement, or exhausted).
type TuningRunner struct {
	Config      RunConfig
	QuickWins   []QuickWin
	TargetM19   float64
	MaxNoImprove int
}

// NewTuningRunner creates a runner with default stop conditions.
func NewTuningRunner(cfg RunConfig, qws []QuickWin) *TuningRunner {
	return &TuningRunner{
		Config:       cfg,
		QuickWins:    qws,
		TargetM19:    0.90,
		MaxNoImprove: 3,
	}
}

// Run executes the tuning loop. It applies each QW in order, measures M19,
// and decides whether to keep or revert. Returns a TuningReport.
//
// This is the skeleton structure. The actual calibration measurement requires
// the rp-e2e-launch gate to be complete. Until then, Apply functions are nil
// and the runner logs the planned action without executing.
func (r *TuningRunner) Run(baselineM19 float64) TuningReport {
	report := TuningReport{
		BaselineM19: baselineM19,
		FinalM19:    baselineM19,
	}

	currentM19 := baselineM19
	noImproveStreak := 0

	for _, qw := range r.QuickWins {
		if currentM19 >= r.TargetM19 {
			report.StopReason = fmt.Sprintf("target M19 %.2f reached", r.TargetM19)
			break
		}
		if noImproveStreak >= r.MaxNoImprove {
			report.StopReason = fmt.Sprintf("no improvement for %d consecutive QWs", r.MaxNoImprove)
			break
		}

		result := TuningResult{
			QWID:        qw.ID,
			BaselineM19: currentM19,
		}

		if qw.Apply == nil {
			slog.Info("tuning QW skipped (not yet implemented)",
				slog.String("qw", qw.ID),
				slog.String("name", qw.Name),
			)
			result.Error = "not yet implemented"
			report.Results = append(report.Results, result)
			noImproveStreak++
			continue
		}

		if err := qw.Apply(&r.Config); err != nil {
			slog.Error("tuning QW apply failed",
				slog.String("qw", qw.ID),
				slog.String("error", err.Error()),
			)
			result.Error = err.Error()
			report.Results = append(report.Results, result)
			noImproveStreak++
			continue
		}

		// Placeholder: actual calibration measurement goes here.
		// For now, after M19 is computed by running calibration,
		// the result would be compared with currentM19.
		afterM19 := currentM19
		result.AfterM19 = afterM19
		result.Delta = afterM19 - currentM19

		if result.Delta >= 0 {
			result.Applied = true
			currentM19 = afterM19
			report.QWsApplied++
			noImproveStreak = 0
			slog.Info("tuning QW applied",
				slog.String("qw", qw.ID),
				slog.Float64("delta", result.Delta),
			)
		} else {
			result.Reverted = true
			report.QWsReverted++
			noImproveStreak++
			slog.Warn("tuning QW reverted (regression)",
				slog.String("qw", qw.ID),
				slog.Float64("delta", result.Delta),
			)
		}

		report.Results = append(report.Results, result)
	}

	if report.StopReason == "" {
		report.StopReason = "all quick wins exhausted"
	}

	report.FinalM19 = currentM19
	report.CumulativeDelta = currentM19 - baselineM19

	return report
}
