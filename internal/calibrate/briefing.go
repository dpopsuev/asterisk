package calibrate

import (
	"fmt"
	"strings"
)

// GenerateBriefing produces a markdown briefing from the current calibration
// state. The briefing provides shared context for subagents in a batch.
//
// Parameters:
//   - scenarioName: name of the calibration scenario
//   - suiteID: the investigation suite ID
//   - phase: "triage" or "investigation"
//   - batchID: current batch number
//   - batchCount: number of cases in this batch
//   - totalCases: total cases in the scenario
//   - completedCases: number of cases already processed
//   - triageResults: completed triage results (may be nil for triage phase)
//   - clusters: symptom clusters (may be nil for triage phase)
//   - priorRCAs: prior RCA summaries from completed investigations
func GenerateBriefing(
	scenarioName string,
	suiteID int64,
	phase string,
	batchID int64,
	batchCount int,
	totalCases int,
	completedCases int,
	triageResults []TriageResult,
	clusters []SymptomCluster,
	priorRCAs []BriefingRCA,
) string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("# Calibration Briefing — Batch %d\n\n", batchID))

	// Run context
	b.WriteString("## Run context\n\n")
	b.WriteString(fmt.Sprintf("- Scenario: %s\n", scenarioName))
	b.WriteString(fmt.Sprintf("- Suite ID: %d\n", suiteID))
	b.WriteString(fmt.Sprintf("- Phase: %s\n", phase))
	b.WriteString(fmt.Sprintf("- Cases in this batch: %d\n", batchCount))
	b.WriteString(fmt.Sprintf("- Total cases in run: %d\n", totalCases))
	b.WriteString(fmt.Sprintf("- Completed so far: %d\n", completedCases))
	b.WriteString("\n")

	// Known symptoms from triage
	if len(triageResults) > 0 {
		b.WriteString("## Known symptoms (from prior batches)\n\n")
		b.WriteString("| Case | Category | Component | Defect Hypothesis | Severity |\n")
		b.WriteString("|------|----------|-----------|-------------------|----------|\n")
		for _, tr := range triageResults {
			if tr.TriageArtifact != nil {
				caseID := ""
				if tr.CaseResult != nil {
					caseID = tr.CaseResult.CaseID
				}
				b.WriteString(fmt.Sprintf("| %s | %s | %s | %s | %s |\n",
					caseID,
					tr.TriageArtifact.SymptomCategory,
					safeField(tr.CaseResult, func(cr *CaseResult) string { return cr.ActualComponent }),
					tr.TriageArtifact.DefectTypeHypothesis,
					tr.TriageArtifact.Severity,
				))
			}
		}
		b.WriteString("\n")
	}

	// Cluster assignments (investigation phase)
	if len(clusters) > 0 {
		b.WriteString("## Cluster assignments (investigation phase)\n\n")
		b.WriteString("| Cluster | Representative | Members | Key |\n")
		b.WriteString("|---------|---------------|---------|-----|\n")
		for i, cl := range clusters {
			repID := ""
			if cl.Representative != nil && cl.Representative.CaseResult != nil {
				repID = cl.Representative.CaseResult.CaseID
			}
			memberIDs := make([]string, 0, len(cl.Members))
			for _, m := range cl.Members {
				if m.CaseResult != nil {
					memberIDs = append(memberIDs, m.CaseResult.CaseID)
				}
			}
			b.WriteString(fmt.Sprintf("| K%d | %s | %s | %s |\n",
				i+1, repID, strings.Join(memberIDs, ", "), cl.Key))
		}
		b.WriteString("\n")
	}

	// Prior RCAs
	if len(priorRCAs) > 0 {
		b.WriteString("## Prior RCAs (from completed investigations)\n\n")
		b.WriteString("| RCA ID | Component | Defect Type | Summary |\n")
		b.WriteString("|--------|-----------|-------------|----------|\n")
		for _, rca := range priorRCAs {
			b.WriteString(fmt.Sprintf("| %s | %s | %s | %s |\n",
				rca.ID, rca.Component, rca.DefectType, rca.Summary))
		}
		b.WriteString("\n")
	}

	return b.String()
}

// BriefingRCA is a simplified RCA summary for the briefing file.
type BriefingRCA struct {
	ID         string
	Component  string
	DefectType string
	Summary    string
}

// safeField extracts a field from CaseResult, returning "" if nil.
func safeField(cr *CaseResult, fn func(*CaseResult) string) string {
	if cr == nil {
		return ""
	}
	return fn(cr)
}
