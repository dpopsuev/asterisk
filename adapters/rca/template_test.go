package rca

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFillTemplateString(t *testing.T) {
	params := &TemplateParams{
		CaseID:   42,
		StepName: "F1_TRIAGE",
		Failure: &FailureParams{
			TestName:     "[T-TSC] PTP Recovery test",
			ErrorMessage: "Expected 0 to equal 1",
			Status:       "FAILED",
		},
		Taxonomy: DefaultTaxonomy(),
	}

	tmpl := `# Test: {{.Failure.TestName}}
Case: #{{.CaseID}}
Step: {{.StepName}}
{{if .Failure.ErrorMessage}}Error: {{.Failure.ErrorMessage}}{{end}}
{{if .Failure.LogTruncated}}LOG TRUNCATED{{end}}`

	result, err := FillTemplateString("test", tmpl, params)
	if err != nil {
		t.Fatalf("FillTemplateString: %v", err)
	}
	if !strings.Contains(result, "[T-TSC] PTP Recovery test") {
		t.Errorf("missing test name in output: %s", result)
	}
	if !strings.Contains(result, "#42") {
		t.Errorf("missing case ID in output: %s", result)
	}
	if !strings.Contains(result, "Expected 0 to equal 1") {
		t.Errorf("missing error message in output: %s", result)
	}
	if strings.Contains(result, "LOG TRUNCATED") {
		t.Errorf("LogTruncated should be false, but output contains marker: %s", result)
	}
}

func TestFillTemplateString_Guards(t *testing.T) {
	params := &TemplateParams{
		CaseID: 1,
		Failure: &FailureParams{
			LogTruncated: true,
		},
		Taxonomy: DefaultTaxonomy(),
	}

	tmpl := `{{if .Failure.LogTruncated}}TRUNCATED{{end}}
{{if not .Failure.ErrorMessage}}NO_ERROR{{end}}`

	result, err := FillTemplateString("guards", tmpl, params)
	if err != nil {
		t.Fatalf("FillTemplateString: %v", err)
	}
	if !strings.Contains(result, "TRUNCATED") {
		t.Error("expected TRUNCATED guard to fire")
	}
	if !strings.Contains(result, "NO_ERROR") {
		t.Error("expected NO_ERROR guard to fire")
	}
}

func TestFillTemplateString_Siblings(t *testing.T) {
	params := &TemplateParams{
		CaseID: 1,
		Failure: &FailureParams{TestName: "test1"},
		Siblings: []SiblingParams{
			{ID: 1, Name: "test1", Status: "FAILED"},
			{ID: 2, Name: "test2", Status: "FAILED"},
		},
		Taxonomy: DefaultTaxonomy(),
	}

	tmpl := `{{range .Siblings}}{{.ID}}: {{.Name}}
{{end}}`

	result, err := FillTemplateString("siblings", tmpl, params)
	if err != nil {
		t.Fatalf("FillTemplateString: %v", err)
	}
	if !strings.Contains(result, "1: test1") || !strings.Contains(result, "2: test2") {
		t.Errorf("siblings not rendered: %s", result)
	}
}

func TestFillTemplate_File(t *testing.T) {
	dir := t.TempDir()
	tmplPath := filepath.Join(dir, "test.md")

	content := `# {{.StepName}}
Case: #{{.CaseID}}
Test: {{.Failure.TestName}}`

	if err := os.WriteFile(tmplPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	params := &TemplateParams{
		CaseID:   99,
		StepName: "F3_INVESTIGATE",
		Failure:  &FailureParams{TestName: "my test"},
		Taxonomy: DefaultTaxonomy(),
	}

	result, err := FillTemplate(tmplPath, params)
	if err != nil {
		t.Fatalf("FillTemplate: %v", err)
	}
	if !strings.Contains(result, "#99") || !strings.Contains(result, "my test") {
		t.Errorf("unexpected result: %s", result)
	}
}

func TestTemplatePathForStep(t *testing.T) {
	tests := []struct {
		step CircuitStep
		want string
	}{
		{StepF0Recall, "prompts/recall/judge-similarity.md"},
		{StepF1Triage, "prompts/triage/classify-symptoms.md"},
		{StepF2Resolve, "prompts/resolve/select-repo.md"},
		{StepF3Invest, "prompts/investigate/deep-rca.md"},
		{StepF4Correlate, "prompts/correlate/match-cases.md"},
		{StepF5Review, "prompts/review/present-findings.md"},
		{StepF6Report, "prompts/report/regression-table.md"},
		{StepInit, ""},
		{StepDone, ""},
	}
	for _, tt := range tests {
		got := TemplatePathForStep("prompts", tt.step)
		if got != tt.want {
			t.Errorf("TemplatePathForStep(%s): got %q want %q", tt.step, got, tt.want)
		}
	}
}

func TestFillTemplateString_PriorArtifacts(t *testing.T) {
	params := &TemplateParams{
		CaseID:  1,
		Failure: &FailureParams{TestName: "test"},
		Prior: &PriorParams{
			TriageResult: &TriageResult{
				SymptomCategory:      "assertion",
				DefectTypeHypothesis: "pb001",
			},
			InvestigateResult: &InvestigateArtifact{
				RCAMessage:       "Root cause found",
				DefectType:       "pb001",
				ConvergenceScore: 0.85,
			},
		},
		Taxonomy: DefaultTaxonomy(),
	}

	tmpl := `{{if .Prior}}{{if .Prior.TriageResult}}Category: {{.Prior.TriageResult.SymptomCategory}}{{end}}
{{if .Prior.InvestigateResult}}RCA: {{.Prior.InvestigateResult.RCAMessage}} ({{.Prior.InvestigateResult.ConvergenceScore}}){{end}}{{end}}`

	result, err := FillTemplateString("prior", tmpl, params)
	if err != nil {
		t.Fatalf("FillTemplateString: %v", err)
	}
	if !strings.Contains(result, "Category: assertion") {
		t.Errorf("missing triage category: %s", result)
	}
	if !strings.Contains(result, "Root cause found") {
		t.Errorf("missing RCA message: %s", result)
	}
}
