package calibrate

import (
	"fmt"
	"strings"
)

// FormatReport produces the human-readable calibration report.
func FormatReport(report *CalibrationReport) string {
	var b strings.Builder

	b.WriteString("=== Asterisk Calibration Report ===\n")
	b.WriteString(fmt.Sprintf("Scenario: %s\n", report.Scenario))
	b.WriteString(fmt.Sprintf("Adapter:  %s\n", report.Adapter))
	b.WriteString(fmt.Sprintf("Runs:     %d\n\n", report.Runs))

	passed, total := report.Metrics.PassCount()

	writeSection := func(title string, metrics []Metric) {
		b.WriteString(fmt.Sprintf("--- %s ---\n", title))
		for _, m := range metrics {
			mark := "✓"
			if !m.Pass {
				mark = "✗"
			}
			threshStr := formatThreshold(m)
			b.WriteString(fmt.Sprintf("%-4s %-30s %6.2f %-12s %s %s\n",
				m.ID, m.Name, m.Value, "("+m.Detail+")", mark, threshStr))
		}
		b.WriteString("\n")
	}

	writeSection("Structured Metrics", report.Metrics.Structured)
	writeSection("Workspace / Repo Selection", report.Metrics.Workspace)
	writeSection("Evidence Metrics", report.Metrics.Evidence)
	writeSection("Semantic Metrics", report.Metrics.Semantic)
	writeSection("Pipeline Metrics", report.Metrics.Pipeline)
	writeSection("Aggregate", report.Metrics.Aggregate)

	result := "PASS"
	if passed < total {
		result = "FAIL"
	}
	b.WriteString(fmt.Sprintf("RESULT: %s (%d/%d metrics within threshold)\n\n", result, passed, total))

	// Per-case breakdown
	b.WriteString("--- Per-case breakdown ---\n")
	for _, cr := range report.CaseResults {
		dtMark := boolMark(cr.DefectTypeCorrect)
		pathMark := boolMark(cr.PathCorrect)
		compMark := boolMark(cr.ComponentCorrect)
		path := strings.Join(cr.ActualPath, "→")
		if path == "" {
			path = "(no steps)"
		}
		b.WriteString(fmt.Sprintf("%-4s %-40s (%s/%s): defect=%s %s  comp=%s  path=%s %s\n",
			cr.CaseID, truncate(cr.TestName, 40),
			cr.Version, cr.Job,
			cr.ActualDefectType, dtMark,
			compMark,
			path, pathMark))
	}

	return b.String()
}

func formatThreshold(m Metric) string {
	switch m.ID {
	case "M4", "M20":
		return fmt.Sprintf("(≤%.2f)", m.Threshold)
	case "M17":
		return "(0.5–2.0)"
	case "M18":
		return fmt.Sprintf("(≤%.0f)", m.Threshold)
	default:
		return fmt.Sprintf("(≥%.2f)", m.Threshold)
	}
}

func boolMark(v bool) string {
	if v {
		return "✓"
	}
	return "✗"
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
