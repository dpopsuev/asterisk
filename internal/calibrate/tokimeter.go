package calibrate

import (
	"asterisk/internal/display"
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
	cost := DefaultCostConfig()

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

	// Header
	b.WriteString("\n")
	b.WriteString("# TokiMeter\n\n")
	b.WriteString(fmt.Sprintf("> **%s** | adapter: `%s` | %s\n\n", bill.Scenario, bill.Adapter, bill.Timestamp))

	// Summary box
	b.WriteString("## Summary\n\n")
	b.WriteString("| Metric | Value |\n")
	b.WriteString("|--------|-------|\n")
	b.WriteString(fmt.Sprintf("| Cases | %d |\n", bill.CaseCount))
	b.WriteString(fmt.Sprintf("| Steps | %d |\n", bill.TotalSteps))
	b.WriteString(fmt.Sprintf("| Input tokens | %s |\n", fmtTokens(bill.TotalIn)))
	b.WriteString(fmt.Sprintf("| Output tokens | %s |\n", fmtTokens(bill.TotalOut)))
	b.WriteString(fmt.Sprintf("| **Total tokens** | **%s** |\n", fmtTokens(bill.TotalTokens)))
	b.WriteString(fmt.Sprintf("| **Total cost** | **$%.4f** |\n", bill.TotalCostUSD))
	b.WriteString(fmt.Sprintf("| Wall clock | %s |\n", fmtDuration(bill.WallClock)))
	if bill.CaseCount > 0 {
		b.WriteString(fmt.Sprintf("| Avg per case | %s ($%.4f) |\n",
			fmtTokens(bill.TotalTokens/bill.CaseCount),
			bill.TotalCostUSD/float64(bill.CaseCount)))
	}
	b.WriteString("\n")

	// Per-case table
	b.WriteString("## Per-case costs\n\n")
	b.WriteString("| Case | Test | Ver/Job | Steps | In | Out | Total | Cost | Time |\n")
	b.WriteString("|------|------|---------|------:|---:|----:|------:|-----:|-----:|\n")

	// Sort cases naturally
	sort.Slice(bill.CaseLines, func(i, j int) bool {
		return bill.CaseLines[i].CaseID < bill.CaseLines[j].CaseID
	})

	for _, cl := range bill.CaseLines {
		testName := truncate(cl.TestName, 25)
		if testName == "" {
			testName = "-"
		}
		b.WriteString(fmt.Sprintf("| %s | %s | %s/%s | %d | %s | %s | %s | $%.4f | %.1fs |\n",
			cl.CaseID,
			testName,
			cl.Version, cl.Job,
			cl.Steps,
			fmtTokens(cl.In),
			fmtTokens(cl.Out),
			fmtTokens(cl.Total),
			cl.CostUSD,
			float64(cl.WallMs)/1000.0,
		))
	}
	b.WriteString("\n")

	// Per-step table
	b.WriteString("## Per-step costs\n\n")
	b.WriteString("| Step | Calls | In | Out | Total | Cost |\n")
	b.WriteString("|------|------:|---:|----:|------:|-----:|\n")
	for _, sl := range bill.StepLines {
		b.WriteString(fmt.Sprintf("| %s | %d | %s | %s | %s | $%.4f |\n",
			display.StageWithCode(sl.Step), sl.Invocations,
			fmtTokens(sl.In), fmtTokens(sl.Out),
			fmtTokens(sl.Total), sl.CostUSD))
	}

	// Footer totals
	b.WriteString(fmt.Sprintf("| **TOTAL** | **%d** | **%s** | **%s** | **%s** | **$%.4f** |\n",
		bill.TotalSteps,
		fmtTokens(bill.TotalIn),
		fmtTokens(bill.TotalOut),
		fmtTokens(bill.TotalTokens),
		bill.TotalCostUSD))
	b.WriteString("\n")

	// Pricing footnote
	b.WriteString("---\n\n")
	b.WriteString("*Pricing: $3/M input, $15/M output tokens. Tokens estimated at ~4 chars/token.*\n")

	return b.String()
}

// fmtTokens formats a token count with K suffix for readability.
func fmtTokens(n int) string {
	if n >= 1000 {
		return fmt.Sprintf("%.1fK", float64(n)/1000.0)
	}
	return fmt.Sprintf("%d", n)
}

// fmtDuration formats a duration as "Xm Ys" or "Ys".
func fmtDuration(d time.Duration) string {
	s := int(d.Seconds())
	if s >= 60 {
		return fmt.Sprintf("%dm %ds", s/60, s%60)
	}
	return fmt.Sprintf("%ds", s)
}
