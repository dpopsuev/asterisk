package orchestrate

import (
	"bytes"
	"fmt"
	"os"
	"text/template"
)

// FillTemplate loads a Go text/template file, executes it with the given
// params, and returns the rendered string. Template guards (G1–G34) are
// embedded in the templates via conditional blocks ({{if .Field}}).
func FillTemplate(templatePath string, params *TemplateParams) (string, error) {
	data, err := os.ReadFile(templatePath)
	if err != nil {
		return "", fmt.Errorf("read template %s: %w", templatePath, err)
	}

	funcMap := template.FuncMap{
		"sub": func(a, b int) int { return a - b },
		"add": func(a, b int) int { return a + b },
	}

	tmpl, err := template.New("prompt").Funcs(funcMap).Parse(string(data))
	if err != nil {
		return "", fmt.Errorf("parse template %s: %w", templatePath, err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, params); err != nil {
		return "", fmt.Errorf("execute template %s: %w", templatePath, err)
	}
	return buf.String(), nil
}

// FillTemplateString executes a Go text/template from a raw string with the
// given params. Useful for embedded/inline templates and testing.
func FillTemplateString(name, tmplStr string, params *TemplateParams) (string, error) {
	funcMap := template.FuncMap{
		"sub": func(a, b int) int { return a - b },
		"add": func(a, b int) int { return a + b },
	}

	tmpl, err := template.New(name).Funcs(funcMap).Parse(tmplStr)
	if err != nil {
		return "", fmt.Errorf("parse template %s: %w", name, err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, params); err != nil {
		return "", fmt.Errorf("execute template %s: %w", name, err)
	}
	return buf.String(), nil
}

// TemplatePathForStep returns the default template file path for a pipeline step.
// Templates live under the promptDir (e.g. ".cursor/prompts/").
func TemplatePathForStep(promptDir string, step PipelineStep) string {
	switch step {
	case StepF0Recall:
		return promptDir + "/recall/judge-similarity.md"
	case StepF1Triage:
		return promptDir + "/triage/classify-symptoms.md"
	case StepF2Resolve:
		return promptDir + "/resolve/select-repo.md"
	case StepF3Invest:
		return promptDir + "/investigate/deep-rca.md"
	case StepF4Correlate:
		return promptDir + "/correlate/match-cases.md"
	case StepF5Review:
		return promptDir + "/review/present-findings.md"
	case StepF6Report:
		return promptDir + "/report/regression-table.md"
	default:
		return ""
	}
}
