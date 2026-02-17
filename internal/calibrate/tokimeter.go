package calibrate

import (
	"asterisk/internal/calibrate/dispatch"
	"asterisk/internal/display"
	"asterisk/internal/format"
	"fmt"
	"sort"
	"strings"
	"time"
)

// TokiMeterBill is the structured cost bill for a calibration or investigation run.
type TokiMeterBill struct {
	Scenario     string
	Adapter      string
	Timestamp    string
	CaseLines    []TokiMeterCaseLine
	StepLines    []TokiMeterStepLine
	TotalIn      int
	TotalOut     int
	TotalTokens  int
	TotalCostUSD float64
	TotalSteps   int
	WallClock    time.Duration
	CaseCount    int
}

// TokiMeterCaseLine is one row in the per-case section of the bill.
type TokiMeterCaseLine struct {
	CaseID   string
	TestName string
	Version  string
	Job      string
	Steps    int
	In       int
	Out      int
	Total    int
	CostUSD  float64
	WallMs   int64
}

// TokiMeterStepLine is one row in the per-step section.
type TokiMeterStepLine struct {
	Step        string
	Invocations int
	In          int
	Out         int
	Total       int
	CostUSD     float64
}

// BuildTokiMeterBill constructs a bill from a CalibrationReport.
func BuildTokiMeterBill(report *CalibrationReport) *TokiMeterBill {
	if report.Tokens == nil {
		return nil
	}
	ts := report.Tokens
	cost := dispatch.DefaultCostConfig()

	bill := &TokiMeterBill{
		Scenario:     report.Scenario,
		Adapter:      report.Adapter,
		Timestamp:    time.Now().UTC().Format("2006-01-02 15:04 UTC"),
		TotalIn:      ts.TotalPromptTokens,
		TotalOut:     ts.TotalArtifactTokens,
		TotalTokens:  ts.TotalTokens,
		TotalCostUSD: ts.TotalCostUSD,
		TotalSteps:   ts.TotalSteps,
		WallClock:    time.Duration(ts.TotalWallClockMs) * time.Millisecond,
		CaseCount:    len(report.CaseResults),
	}

	// Per-case lines — join CaseResults with token data
	for _, cr := range report.CaseResults {
		cs, ok := ts.PerCase[cr.CaseID]
		if !ok {
			continue
		}
		inCost := float64(cs.PromptTokens) / 1_000_000 * cost.InputPricePerMToken
		outCost := float64(cs.ArtifactTokens) / 1_000_000 * cost.OutputPricePerMToken
		bill.CaseLines = append(bill.CaseLines, TokiMeterCaseLine{
			CaseID:   cr.CaseID,
			TestName: cr.TestName,
			Version:  cr.Version,
			Job:      cr.Job,
			Steps:    cs.Steps,
			In:       cs.PromptTokens,
			Out:      cs.ArtifactTokens,
			Total:    cs.TotalTokens,
			CostUSD:  inCost + outCost,
			WallMs:   cs.WallClockMs,
		})
	}

	// Per-step lines — sort by pipeline order
	stepOrder := []string{
		"F0_RECALL", "F1_TRIAGE", "F2_RESOLVE", "F3_INVESTIGATE",
		"F4_CORRELATE", "F5_REVIEW", "F6_REPORT",
	}
	for _, step := range stepOrder {
		ss, ok := ts.PerStep[step]
		if !ok {
			continue
		}
		inCost := float64(ss.PromptTokens) / 1_000_000 * cost.InputPricePerMToken
		outCost := float64(ss.ArtifactTokens) / 1_000_000 * cost.OutputPricePerMToken
		bill.StepLines = append(bill.StepLines, TokiMeterStepLine{
			Step:        step,
			Invocations: ss.Invocations,
			In:          ss.PromptTokens,
			Out:         ss.ArtifactTokens,
			Total:       ss.TotalTokens,
			CostUSD:     inCost + outCost,
		})
	}

	return bill
}

// FormatTokiMeter produces a markdown-formatted cost bill suitable for
// Cursor terminal output. The name is a portmanteau of "token" + "taximeter".
func FormatTokiMeter(bill *TokiMeterBill) string {
	if bill == nil {
		return ""
	}
	var b strings.Builder

	b.WriteString("\n")
	b.WriteString("# TokiMeter\n\n")
	b.WriteString(fmt.Sprintf("> **%s** | adapter: `%s` | %s\n\n", bill.Scenario, bill.Adapter, bill.Timestamp))

	// Summary table
	b.WriteString("## Summary\n\n")
	summary := format.NewTable(format.Markdown)
	summary.Header("Metric", "Value")
	summary.Row("Cases", bill.CaseCount)
	summary.Row("Steps", bill.TotalSteps)
	summary.Row("Input tokens", format.FmtTokens(bill.TotalIn))
	summary.Row("Output tokens", format.FmtTokens(bill.TotalOut))
	summary.Row("**Total tokens**", fmt.Sprintf("**%s**", format.FmtTokens(bill.TotalTokens)))
	summary.Row("**Total cost**", fmt.Sprintf("**$%.4f**", bill.TotalCostUSD))
	summary.Row("Wall clock", format.FmtDuration(bill.WallClock))
	if bill.CaseCount > 0 {
		summary.Row("Avg per case", fmt.Sprintf("%s ($%.4f)",
			format.FmtTokens(bill.TotalTokens/bill.CaseCount),
			bill.TotalCostUSD/float64(bill.CaseCount)))
	}
	b.WriteString(summary.String())
	b.WriteString("\n\n")

	// Per-case table
	b.WriteString("## Per-case costs\n\n")

	sort.Slice(bill.CaseLines, func(i, j int) bool {
		return bill.CaseLines[i].CaseID < bill.CaseLines[j].CaseID
	})

	cases := format.NewTable(format.Markdown)
	cases.Header("Case", "Test", "Ver/Job", "Steps", "In", "Out", "Total", "Cost", "Time")
	for _, cl := range bill.CaseLines {
		testName := format.Truncate(cl.TestName, 25)
		if testName == "" {
			testName = "-"
		}
		cases.Row(
			cl.CaseID,
			testName,
			fmt.Sprintf("%s/%s", cl.Version, cl.Job),
			cl.Steps,
			format.FmtTokens(cl.In),
			format.FmtTokens(cl.Out),
			format.FmtTokens(cl.Total),
			fmt.Sprintf("$%.4f", cl.CostUSD),
			fmt.Sprintf("%.1fs", float64(cl.WallMs)/1000.0),
		)
	}
	b.WriteString(cases.String())
	b.WriteString("\n\n")

	// Per-step table
	b.WriteString("## Per-step costs\n\n")
	steps := format.NewTable(format.Markdown)
	steps.Header("Step", "Calls", "In", "Out", "Total", "Cost")
	for _, sl := range bill.StepLines {
		steps.Row(
			display.StageWithCode(sl.Step),
			sl.Invocations,
			format.FmtTokens(sl.In),
			format.FmtTokens(sl.Out),
			format.FmtTokens(sl.Total),
			fmt.Sprintf("$%.4f", sl.CostUSD),
		)
	}
	steps.Footer(
		"**TOTAL**",
		fmt.Sprintf("**%d**", bill.TotalSteps),
		fmt.Sprintf("**%s**", format.FmtTokens(bill.TotalIn)),
		fmt.Sprintf("**%s**", format.FmtTokens(bill.TotalOut)),
		fmt.Sprintf("**%s**", format.FmtTokens(bill.TotalTokens)),
		fmt.Sprintf("**$%.4f**", bill.TotalCostUSD),
	)
	b.WriteString(steps.String())
	b.WriteString("\n\n")

	// Pricing footnote
	b.WriteString("---\n\n")
	b.WriteString("*Pricing: $3/M input, $15/M output tokens. Tokens estimated at ~4 chars/token.*\n")

	return b.String()
}

