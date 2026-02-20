package calibrate

import (
	"asterisk/internal/display"
	"asterisk/internal/format"
	"asterisk/internal/logging"
	"fmt"
	"strings"

	"asterisk/internal/orchestrate"
	"asterisk/internal/store"
)

// AnalysisConfig holds configuration for an analysis run.
type AnalysisConfig struct {
	Adapter    ModelAdapter
	Thresholds orchestrate.Thresholds
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

		result, err := runAnalysisCasePipeline(st, caseData, suiteID, caseLabel, cfg)
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

// runAnalysisCasePipeline drives the orchestrator for a single case until done.
// Same pipeline as calibrate's runCasePipeline but without ground truth extraction.
func runAnalysisCasePipeline(
	st store.Store,
	caseData *store.Case,
	suiteID int64,
	caseLabel string,
	cfg AnalysisConfig,
) (*AnalysisCaseResult, error) {
	result := &AnalysisCaseResult{
		CaseLabel:   caseLabel,
		TestName:    caseData.Name,
		StoreCaseID: caseData.ID,
	}

	basePath := cfg.BasePath
	if basePath == "" {
		basePath = orchestrate.DefaultBasePath
	}
	caseDir, err := orchestrate.EnsureCaseDir(basePath, suiteID, caseData.ID)
	if err != nil {
		return result, fmt.Errorf("ensure case dir: %w", err)
	}

	state := orchestrate.InitState(caseData.ID, suiteID)
	orchestrate.AdvanceStep(state, orchestrate.StepF0Recall, "INIT", "start pipeline")
	if err := orchestrate.SaveState(caseDir, state); err != nil {
		return result, fmt.Errorf("save state: %w", err)
	}

	rules := orchestrate.DefaultHeuristics(cfg.Thresholds)
	maxSteps := 20

	for step := 0; step < maxSteps; step++ {
		if state.CurrentStep == orchestrate.StepDone {
			break
		}

		currentStep := state.CurrentStep
		result.Path = append(result.Path, stepName(currentStep))

		response, err := cfg.Adapter.SendPrompt(caseLabel, currentStep, "")
		if err != nil {
			return result, fmt.Errorf("adapter.SendPrompt(%s, %s): %w", caseLabel, currentStep, err)
		}

		var artifact any
		switch currentStep {
		case orchestrate.StepF0Recall:
			artifact, err = parseJSON[orchestrate.RecallResult](response)
		case orchestrate.StepF1Triage:
			artifact, err = parseJSON[orchestrate.TriageResult](response)
		case orchestrate.StepF2Resolve:
			artifact, err = parseJSON[orchestrate.ResolveResult](response)
		case orchestrate.StepF3Invest:
			artifact, err = parseJSON[orchestrate.InvestigateArtifact](response)
		case orchestrate.StepF4Correlate:
			artifact, err = parseJSON[orchestrate.CorrelateResult](response)
		case orchestrate.StepF5Review:
			artifact, err = parseJSON[orchestrate.ReviewDecision](response)
		case orchestrate.StepF6Report:
			artifact, err = parseJSON[map[string]any](response)
		}
		if err != nil {
			return result, fmt.Errorf("parse artifact for %s: %w", currentStep, err)
		}

		artifactFile := orchestrate.ArtifactFilename(currentStep)
		if err := orchestrate.WriteArtifact(caseDir, artifactFile, artifact); err != nil {
			return result, fmt.Errorf("write artifact: %w", err)
		}

		extractAnalysisStepData(result, currentStep, artifact)

		action, ruleID := orchestrate.EvaluateHeuristics(rules, currentStep, artifact, state)
		logging.New("analyze").Info("heuristic evaluated",
			"step", display.Stage(string(currentStep)), "rule", display.HeuristicWithCode(ruleID),
			"next", display.Stage(string(action.NextStep)), "explanation", action.Explanation)

		if currentStep == orchestrate.StepF3Invest && action.NextStep == orchestrate.StepF2Resolve {
			orchestrate.IncrementLoop(state, "investigate")
		}
		if currentStep == orchestrate.StepF5Review &&
			action.NextStep != orchestrate.StepF6Report &&
			action.NextStep != orchestrate.StepDone {
			orchestrate.IncrementLoop(state, "reassess")
		}

		if err := orchestrate.ApplyStoreEffects(st, caseData, currentStep, artifact); err != nil {
			logging.New("analyze").Warn("store side-effect error", "step", string(currentStep), "error", err)
		}

		orchestrate.AdvanceStep(state, action.NextStep, ruleID, action.Explanation)
		if err := orchestrate.SaveState(caseDir, state); err != nil {
			return result, fmt.Errorf("save state: %w", err)
		}
	}

	// Refresh case from store for final field values
	updated, err := st.GetCaseV2(caseData.ID)
	if err == nil && updated != nil {
		result.RCAID = updated.RCAID
		if updated.RCAID != 0 {
			rca, err := st.GetRCAV2(updated.RCAID)
			if err == nil && rca != nil {
				result.DefectType = rca.DefectType
				result.RCAMessage = rca.Description
				result.Component = rca.Component
				result.Convergence = rca.ConvergenceScore
			}
		}
	}

	return result, nil
}

// extractAnalysisStepData captures per-step results without ground truth comparison.
func extractAnalysisStepData(result *AnalysisCaseResult, step orchestrate.PipelineStep, artifact any) {
	switch step {
	case orchestrate.StepF0Recall:
		if r, ok := artifact.(*orchestrate.RecallResult); ok && r != nil {
			result.RecallHit = r.Match && r.Confidence >= 0.80
		}
	case orchestrate.StepF1Triage:
		if r, ok := artifact.(*orchestrate.TriageResult); ok && r != nil {
			result.Category = r.SymptomCategory
			result.Skip = r.SkipInvestigation
			result.Cascade = r.CascadeSuspected
			if len(r.CandidateRepos) == 1 && !r.SkipInvestigation {
				result.SelectedRepos = append(result.SelectedRepos, r.CandidateRepos[0])
			}
		}
	case orchestrate.StepF2Resolve:
		if r, ok := artifact.(*orchestrate.ResolveResult); ok && r != nil {
			for _, repo := range r.SelectedRepos {
				result.SelectedRepos = append(result.SelectedRepos, repo.Name)
			}
		}
	case orchestrate.StepF3Invest:
		if r, ok := artifact.(*orchestrate.InvestigateArtifact); ok && r != nil {
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
		path := display.StagePath(cr.Path)
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
		rpTag := display.RPIssueTag(cr.RPIssueType, cr.RPAutoAnalyzed)
		if rpTag == "" {
			rpTag = "-"
		}
		tbl.Row(
			cr.CaseLabel,
			format.Truncate(cr.TestName, 50),
			display.DefectTypeWithCode(cr.DefectType),
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
