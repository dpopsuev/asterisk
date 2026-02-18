package calibrate

import (
	"asterisk/internal/calibrate/dispatch"
	"asterisk/internal/display"
	"asterisk/internal/format"
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
		tbl := format.NewTable(format.ASCII)
		tbl.Header("ID", "Metric", "Value", "Detail", "Pass", "Threshold")
		tbl.Columns(
			format.ColumnConfig{Number: 1, Align: format.AlignLeft},
			format.ColumnConfig{Number: 2, Align: format.AlignLeft},
			format.ColumnConfig{Number: 3, Align: format.AlignRight},
			format.ColumnConfig{Number: 4, Align: format.AlignLeft},
			format.ColumnConfig{Number: 5, Align: format.AlignCenter},
			format.ColumnConfig{Number: 6, Align: format.AlignLeft},
		)
		for _, m := range metrics {
			tbl.Row(
				m.ID,
				display.Metric(m.ID),
				fmt.Sprintf("%.2f", m.Value),
				m.Detail,
				format.BoolMark(m.Pass),
				formatThreshold(m),
			)
		}
		b.WriteString(tbl.String())
		b.WriteString("\n\n")
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

	// Token & Cost section (when tracker was present)
	if report.Tokens != nil {
		b.WriteString(dispatch.FormatTokenSummary(*report.Tokens))
		b.WriteString("\n")
	}

	// Per-case breakdown
	b.WriteString("--- Per-case breakdown ---\n")
	caseTbl := format.NewTable(format.ASCII)
	caseTbl.Header("Case", "Test", "Ver/Job", "Defect", "DT", "RP", "Comp", "Path", "Path✓")
	caseTbl.Columns(
		format.ColumnConfig{Number: 2, MaxWidth: 40},
	)
	for _, cr := range report.CaseResults {
		path := display.StagePath(cr.ActualPath)
		if path == "" {
			path = "(no steps)"
		}
		rpTag := display.RPIssueTag(cr.RPIssueType, cr.RPAutoAnalyzed)
		if rpTag == "" {
			rpTag = "-"
		}
		caseTbl.Row(
			cr.CaseID,
			format.Truncate(cr.TestName, 40),
			fmt.Sprintf("%s/%s", cr.Version, cr.Job),
			display.DefectTypeWithCode(cr.ActualDefectType),
			format.BoolMark(cr.DefectTypeCorrect),
			rpTag,
			format.BoolMark(cr.ComponentCorrect),
			path,
			format.BoolMark(cr.PathCorrect),
		)
	}
	b.WriteString(caseTbl.String())
	b.WriteString("\n")

	return b.String()
}

func formatThreshold(m Metric) string {
	switch m.ID {
	case "M4", "M20":
		return fmt.Sprintf("≤%.2f", m.Threshold)
	case "M17":
		return "0.5–2.0"
	case "M18":
		return fmt.Sprintf("≤%.0f", m.Threshold)
	default:
		return fmt.Sprintf("≥%.2f", m.Threshold)
	}
}
