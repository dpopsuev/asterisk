package rca

import (
	"context"
	"fmt"
	"strings"

	"asterisk/adapters/store"

	"github.com/dpopsuev/origami/format"
	"github.com/dpopsuev/origami/logging"
)

// AnalysisConfig holds configuration for an analysis run.
type AnalysisConfig struct {
	Adapter    ModelAdapter
	Thresholds Thresholds
	BasePath   string // root directory for investigation artifacts; defaults to DefaultBasePath
}

// AnalysisReport is the output of an analysis run.
// Unlike CalibrationReport, there is no ground truth scoring — just investigation results.
type AnalysisReport struct {
	LaunchName  string               `json:"launch_name"`
	Adapter     string               `json:"adapter"`
	TotalCases  int                  `json:"total_cases"`
	CaseResults []AnalysisCaseResult `json:"case_results"`
}

// AnalysisCaseResult captures per-case investigation outcome without ground truth scoring.
type AnalysisCaseResult struct {
	CaseLabel     string   `json:"case_label"`
	TestName      string   `json:"test_name"`
	StoreCaseID   int64    `json:"store_case_id"`
	DefectType    string   `json:"defect_type"`
	Category      string   `json:"category"`
	RCAMessage    string   `json:"rca_message"`
	Component     string   `json:"component"`
	Path          []string `json:"path"`
	RecallHit     bool     `json:"recall_hit"`
	Skip          bool     `json:"skip"`
	Cascade       bool     `json:"cascade"`
	EvidenceRefs  []string `json:"evidence_refs"`
	SelectedRepos []string `json:"selected_repos"`
	Convergence    float64  `json:"convergence"`
	RCAID          int64    `json:"rca_id"`
	RPIssueType    string   `json:"rp_issue_type,omitempty"`
	RPAutoAnalyzed bool     `json:"rp_auto_analyzed,omitempty"`
}

// RunAnalysis drives the F0–F6 pipeline for a set of cases using the provided adapter.
// Unlike RunCalibration, there is no ground truth scoring — just investigation results.
// Each case is walked through the pipeline graph using WalkCase with store-effect hooks.
func RunAnalysis(st store.Store, cases []*store.Case, suiteID int64, cfg AnalysisConfig) (*AnalysisReport, error) {
	report := &AnalysisReport{
		Adapter:    cfg.Adapter.Name(),
		TotalCases: len(cases),
	}

	logger := logging.New("analyze")

	for i, caseData := range cases {
		caseLabel := fmt.Sprintf("A%d", i+1)
		logger.Info("processing case",
			"label", caseLabel, "index", i+1, "total", len(cases), "test", caseData.Name)

		result, err := walkAnalysisCase(st, caseData, caseLabel, cfg)
		if err != nil {
			logger.Error("case pipeline failed", "label", caseLabel, "error", err)
			result = &AnalysisCaseResult{
				CaseLabel:   caseLabel,
				TestName:    caseData.Name,
				StoreCaseID: caseData.ID,
			}
		}
		report.CaseResults = append(report.CaseResults, *result)
	}

	return report, nil
}

// walkAnalysisCase runs a single case through the RCA pipeline via a framework
// graph walk. Store effects fire automatically via hooks declared in the pipeline YAML.
func walkAnalysisCase(
	st store.Store,
	caseData *store.Case,
	caseLabel string,
	cfg AnalysisConfig,
) (*AnalysisCaseResult, error) {
	result := &AnalysisCaseResult{
		CaseLabel:   caseLabel,
		TestName:    caseData.Name,
		StoreCaseID: caseData.ID,
	}

	hooks := StoreHooks(st, caseData)

	walkCfg := WalkConfig{
		Store:      st,
		CaseData:   caseData,
		Adapter:    cfg.Adapter,
		CaseLabel:  caseLabel,
		Thresholds: cfg.Thresholds,
		Hooks:      hooks,
	}

	walkResult, err := WalkCase(context.Background(), walkCfg)
	if err != nil {
		return result, fmt.Errorf("walk: %w", err)
	}

	result.Path = walkResult.Path

	for nodeName, art := range walkResult.StepArtifacts {
		step := NodeNameToStep(nodeName)
		extractAnalysisStepData(result, step, art.Raw())
	}

	updated, err := st.GetCaseV2(caseData.ID)
	if err == nil && updated != nil {
		result.RCAID = updated.RCAID
		if updated.RCAID != 0 {
			rcaRec, err := st.GetRCAV2(updated.RCAID)
			if err == nil && rcaRec != nil {
				result.DefectType = rcaRec.DefectType
				result.RCAMessage = rcaRec.Description
				result.Component = rcaRec.Component
				result.Convergence = rcaRec.ConvergenceScore
			}
		}
	}

	return result, nil
}

// extractAnalysisStepData captures per-step results without ground truth comparison.
func extractAnalysisStepData(result *AnalysisCaseResult, step PipelineStep, artifact any) {
	switch step {
	case StepF0Recall:
		if r, ok := artifact.(*RecallResult); ok && r != nil {
			result.RecallHit = r.Match && r.Confidence >= 0.80
		}
	case StepF1Triage:
		if r, ok := artifact.(*TriageResult); ok && r != nil {
			result.Category = r.SymptomCategory
			result.Skip = r.SkipInvestigation
			result.Cascade = r.CascadeSuspected
			if len(r.CandidateRepos) == 1 && !r.SkipInvestigation {
				result.SelectedRepos = append(result.SelectedRepos, r.CandidateRepos[0])
			}
		}
	case StepF2Resolve:
		if r, ok := artifact.(*ResolveResult); ok && r != nil {
			for _, repo := range r.SelectedRepos {
				result.SelectedRepos = append(result.SelectedRepos, repo.Name)
			}
		}
	case StepF3Invest:
		if r, ok := artifact.(*InvestigateArtifact); ok && r != nil {
			result.DefectType = r.DefectType
			result.RCAMessage = r.RCAMessage
			result.EvidenceRefs = r.EvidenceRefs
			result.Convergence = r.ConvergenceScore
		}
	}
}

// FormatAnalysisReport produces a human-readable analysis report.
func FormatAnalysisReport(report *AnalysisReport) string {
	var b strings.Builder

	b.WriteString("=== Asterisk Analysis Report ===\n")
	if report.LaunchName != "" {
		b.WriteString(fmt.Sprintf("Launch:  %s\n", report.LaunchName))
	}
	b.WriteString(fmt.Sprintf("Adapter: %s\n", report.Adapter))
	b.WriteString(fmt.Sprintf("Cases:   %d\n\n", report.TotalCases))

	recallHits := 0
	skipped := 0
	cascades := 0
	investigated := 0
	for _, cr := range report.CaseResults {
		if cr.RecallHit {
			recallHits++
		}
		if cr.Skip {
			skipped++
		}
		if cr.Cascade {
			cascades++
		}
		if cr.RCAID != 0 {
			investigated++
		}
	}
	b.WriteString(fmt.Sprintf("Recall hits:  %d/%d\n", recallHits, report.TotalCases))
	b.WriteString(fmt.Sprintf("Skipped:      %d/%d\n", skipped, report.TotalCases))
	b.WriteString(fmt.Sprintf("Cascades:     %d/%d\n", cascades, report.TotalCases))
	b.WriteString(fmt.Sprintf("Investigated: %d/%d\n\n", investigated, report.TotalCases))

	b.WriteString("--- Per-case breakdown ---\n")
	tbl := format.NewTable(format.ASCII)
	tbl.Header("Case", "Test", "Defect", "RP", "Category", "Conv", "Path", "Flags")
	tbl.Columns(
		format.ColumnConfig{Number: 2, MaxWidth: 50},
		format.ColumnConfig{Number: 6, Align: format.AlignRight},
	)
	for _, cr := range report.CaseResults {
		path := vocabStagePath(cr.Path)
		if path == "" {
			path = "(no steps)"
		}
		flags := ""
		if cr.RecallHit {
			flags += "[recall]"
		}
		if cr.Skip {
			if flags != "" {
				flags += " "
			}
			flags += "[skip]"
		}
		if cr.Cascade {
			if flags != "" {
				flags += " "
			}
			flags += "[cascade]"
		}
		rpTag := vocabRPIssueTag(cr.RPIssueType, cr.RPAutoAnalyzed)
		if rpTag == "" {
			rpTag = "-"
		}
		tbl.Row(
			cr.CaseLabel,
			format.Truncate(cr.TestName, 50),
			vocabNameWithCode(cr.DefectType),
			rpTag,
			cr.Category,
			fmt.Sprintf("%.2f", cr.Convergence),
			path,
			flags,
		)
	}
	b.WriteString(tbl.String())
	b.WriteString("\n")

	// RCA messages below the table
	for _, cr := range report.CaseResults {
		if cr.RCAMessage != "" {
			b.WriteString(fmt.Sprintf("  %s RCA: %s\n", cr.CaseLabel, format.Truncate(cr.RCAMessage, 80)))
		}
	}

	return b.String()
}
