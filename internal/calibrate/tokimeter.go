package calibrate

import (
	"fmt"

	"asterisk/display"

	"github.com/dpopsuev/origami/dispatch"
)

// Asterisk pipeline step ordering for cost bill display.
var asteriskStepOrder = []string{
	"F0_RECALL", "F1_TRIAGE", "F2_RESOLVE", "F3_INVESTIGATE",
	"F4_CORRELATE", "F5_REVIEW", "F6_REPORT",
}

// BuildTokiMeterBill constructs a dispatch.CostBill from an Asterisk
// CalibrationReport, injecting domain-specific step names and case metadata.
func BuildTokiMeterBill(report *CalibrationReport) *dispatch.CostBill {
	if report.Tokens == nil {
		return nil
	}

	caseMap := make(map[string]CaseResult, len(report.CaseResults))
	for _, cr := range report.CaseResults {
		caseMap[cr.CaseID] = cr
	}

	return dispatch.BuildCostBill(report.Tokens,
		dispatch.WithTitle("TokiMeter"),
		dispatch.WithSubtitle(fmt.Sprintf("**%s** | adapter: `%s`", report.Scenario, report.Adapter)),
		dispatch.WithStepOrder(asteriskStepOrder),
		dispatch.WithStepNames(func(step string) string {
			return display.StageWithCode(step)
		}),
		dispatch.WithCaseLabels(func(id string) string { return id }),
		dispatch.WithCaseDetails(func(id string) string {
			cr, ok := caseMap[id]
			if !ok {
				return "-"
			}
			return fmt.Sprintf("%s/%s", cr.Version, cr.Job)
		}),
	)
}

// FormatTokiMeter produces a markdown-formatted cost bill.
func FormatTokiMeter(bill *dispatch.CostBill) string {
	return dispatch.FormatCostBill(bill)
}
