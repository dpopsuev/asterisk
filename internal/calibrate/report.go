package calibrate

import (
	"fmt"
	"strings"

	cal "github.com/dpopsuev/origami/calibrate"
	"github.com/dpopsuev/origami/format"

	"asterisk/internal/display"
)

// FormatReport produces the human-readable calibration report.
// It delegates metric table rendering to cal.FormatReport, then appends
// domain-specific sections (dataset health, per-case breakdown).
func FormatReport(report *CalibrationReport) string {
	genericReport := &report.CalibrationReport

	cfg := cal.FormatConfig{
		Title:          "Asterisk Calibration Report",
		MetricNameFunc: display.Metric,
		ThresholdFunc:  formatThreshold,
	}

	var b strings.Builder
	b.WriteString(cal.FormatReport(genericReport, cfg))

	// Dataset health (domain-specific)
	if report.Dataset != nil {
		b.WriteString("--- Dataset Health ---\n")
		b.WriteString(fmt.Sprintf("Verified cases (scored):   %d\n", report.Dataset.VerifiedCount))
		b.WriteString(fmt.Sprintf("Candidate cases (tracked): %d\n", report.Dataset.CandidateCount))
		if len(report.Dataset.Candidates) > 0 {
			candidateTbl := format.NewTable(format.ASCII)
			candidateTbl.Header("Case", "RCA", "Jira", "Reason")
			for _, c := range report.Dataset.Candidates {
				jira := c.JiraID
				if jira == "" {
					jira = "-"
				}
				candidateTbl.Row(c.CaseID, c.RCAID, jira, c.Reason)
			}
			b.WriteString(candidateTbl.String())
		}
		b.WriteString("\n\n")
	}

	// Per-case breakdown (domain-specific)
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
