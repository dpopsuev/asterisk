package calibrate

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"asterisk/internal/display"
	"asterisk/internal/format"
)

// RenderRCAReport produces a human-readable Markdown RCA report from an AnalysisReport.
// The output is grouped by suspected component and uses human-readable labels throughout.
// timestamp is the analysis run time (passed in so the function remains pure/testable).
func RenderRCAReport(report *AnalysisReport, timestamp time.Time) string {
	if report == nil || len(report.CaseResults) == 0 {
		return "# RCA Report\n\nNo failures analyzed.\n"
	}

	var b strings.Builder

	writeHeader(&b, report, timestamp)
	writeSummary(&b, report)
	writeComponentFindings(&b, report)
	writeCaseDetails(&b, report)

	return b.String()
}

func writeHeader(b *strings.Builder, report *AnalysisReport, ts time.Time) {
	title := "RCA Report"
	if report.LaunchName != "" {
		title += " — " + report.LaunchName
	}
	b.WriteString("# " + title + "\n\n")

	tbl := format.NewTable(format.Markdown)
	tbl.Header("Field", "Value")
	if report.LaunchName != "" {
		tbl.Row("Launch", report.LaunchName)
	}
	tbl.Row("Analyzed", ts.UTC().Format("2006-01-02 15:04 UTC"))
	tbl.Row("Adapter", report.Adapter)
	tbl.Row("Failures", report.TotalCases)
	b.WriteString(tbl.String())
	b.WriteString("\n\n")
}

func writeSummary(b *strings.Builder, report *AnalysisReport) {
	var investigated, skipped, recallHits, cascades int
	compCounts := make(map[string]int)
	defectCounts := make(map[string]int)

	for _, cr := range report.CaseResults {
		if cr.RCAID != 0 {
			investigated++
		}
		if cr.Skip {
			skipped++
		}
		if cr.RecallHit {
			recallHits++
		}
		if cr.Cascade {
			cascades++
		}
		comp := cr.Component
		if comp == "" {
			comp = "unknown"
		}
		compCounts[comp]++
		defectCounts[cr.DefectType]++
	}

	b.WriteString("## Summary\n\n")
	b.WriteString(fmt.Sprintf("- **%d** failures analyzed, **%d** investigated", report.TotalCases, investigated))
	if skipped > 0 {
		b.WriteString(fmt.Sprintf(", %d skipped", skipped))
	}
	if recallHits > 0 {
		b.WriteString(fmt.Sprintf(", %d recall hits", recallHits))
	}
	if cascades > 0 {
		b.WriteString(fmt.Sprintf(", %d cascades", cascades))
	}
	b.WriteString("\n")

	b.WriteString("- **Components:** " + formatDistribution(compCounts, false) + "\n")
	b.WriteString("- **Defect types:** " + formatDistribution(defectCounts, true) + "\n")
	b.WriteString("\n")
}

func writeComponentFindings(b *strings.Builder, report *AnalysisReport) {
	groups := groupByComponent(report.CaseResults)
	sortedComponents := sortedKeys(groups)

	b.WriteString("## Findings by Component\n\n")

	for _, comp := range sortedComponents {
		cases := groups[comp]
		b.WriteString(fmt.Sprintf("### %s (%d %s)\n\n",
			comp, len(cases), pluralize(len(cases), "failure", "failures")))

		tbl := format.NewTable(format.Markdown)
		tbl.Header("#", "Test", "Verdict", "Confidence", "RP Status")
		tbl.Columns(
			format.ColumnConfig{Number: 2, MaxWidth: 60},
		)

		for _, cr := range cases {
			rpTag := display.RPIssueTag(cr.RPIssueType, cr.RPAutoAnalyzed)
			if rpTag == "" {
				rpTag = "--"
			}
			tbl.Row(
				cr.CaseLabel,
				format.Truncate(cr.TestName, 60),
				display.DefectTypeWithCode(cr.DefectType),
				fmt.Sprintf("%.0f%%", roundConvergence(cr.Convergence)*100),
				rpTag,
			)
		}
		b.WriteString(tbl.String())
		b.WriteString("\n\n")

		evidenceSet := collectEvidence(cases)
		if len(evidenceSet) > 0 {
			b.WriteString("**Evidence:** " + strings.Join(evidenceSet, ", ") + "\n\n")
		}
	}
}

func writeCaseDetails(b *strings.Builder, report *AnalysisReport) {
	b.WriteString("## Case Details\n\n")

	for _, cr := range report.CaseResults {
		b.WriteString(fmt.Sprintf("### %s: %s\n\n", cr.CaseLabel, cr.TestName))

		tbl := format.NewTable(format.Markdown)
		tbl.Header("Field", "Value")
		tbl.Row("Verdict", display.DefectTypeWithCode(cr.DefectType))
		tbl.Row("Category", cr.Category)

		comp := cr.Component
		if comp == "" {
			comp = "unknown"
		}
		tbl.Row("Component", comp)
		tbl.Row("Confidence", fmt.Sprintf("%.0f%%", roundConvergence(cr.Convergence)*100))
		tbl.Row("Pipeline", display.StagePath(cr.Path))

		if cr.RPIssueType != "" {
			tbl.Row("RP Classification", display.RPIssueTag(cr.RPIssueType, cr.RPAutoAnalyzed))
		}

		if len(cr.SelectedRepos) > 0 {
			tbl.Row("Repos investigated", strings.Join(cr.SelectedRepos, ", "))
		}
		if len(cr.EvidenceRefs) > 0 {
			tbl.Row("Evidence", strings.Join(cr.EvidenceRefs, ", "))
		}

		var flags []string
		if cr.RecallHit {
			flags = append(flags, "recall-hit")
		}
		if cr.Skip {
			flags = append(flags, "skipped")
		}
		if cr.Cascade {
			flags = append(flags, "cascade")
		}
		if len(flags) > 0 {
			tbl.Row("Flags", strings.Join(flags, ", "))
		}

		b.WriteString(tbl.String())
		b.WriteString("\n\n")

		if cr.RCAMessage != "" {
			b.WriteString("**RCA:** " + cr.RCAMessage + "\n\n")
		}

		b.WriteString("---\n\n")
	}
}

// --- helpers ---

func groupByComponent(cases []AnalysisCaseResult) map[string][]AnalysisCaseResult {
	groups := make(map[string][]AnalysisCaseResult)
	for _, cr := range cases {
		comp := cr.Component
		if comp == "" {
			comp = "unknown"
		}
		groups[comp] = append(groups[comp], cr)
	}
	return groups
}

func sortedKeys(m map[string][]AnalysisCaseResult) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func formatDistribution(counts map[string]int, humanize bool) string {
	type kv struct {
		Key   string
		Count int
	}
	sorted := make([]kv, 0, len(counts))
	for k, v := range counts {
		sorted = append(sorted, kv{k, v})
	}
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Count > sorted[j].Count })

	parts := make([]string, len(sorted))
	for i, item := range sorted {
		label := item.Key
		if humanize {
			label = display.DefectTypeWithCode(item.Key)
		}
		parts[i] = fmt.Sprintf("%s (%d)", label, item.Count)
	}
	return strings.Join(parts, ", ")
}

func collectEvidence(cases []AnalysisCaseResult) []string {
	seen := make(map[string]bool)
	var result []string
	for _, cr := range cases {
		for _, ref := range cr.EvidenceRefs {
			if !seen[ref] {
				seen[ref] = true
				result = append(result, ref)
			}
		}
	}
	return result
}

func roundConvergence(v float64) float64 {
	return math.Round(v*100) / 100
}

func pluralize(n int, singular, plural string) string {
	if n == 1 {
		return singular
	}
	return plural
}
