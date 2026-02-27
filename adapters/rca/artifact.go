package rca

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// DefaultBasePath is the default root directory for investigation data.
const DefaultBasePath = ".asterisk/investigations"

// CaseDir returns the per-case directory path: {basePath}/{suiteID}/{caseID}/
func CaseDir(basePath string, suiteID, caseID int64) string {
	return filepath.Join(basePath, fmt.Sprintf("%d", suiteID), fmt.Sprintf("%d", caseID))
}

// EnsureCaseDir creates the per-case directory if it doesn't exist.
func EnsureCaseDir(basePath string, suiteID, caseID int64) (string, error) {
	dir := CaseDir(basePath, suiteID, caseID)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("create case dir: %w", err)
	}
	return dir, nil
}

// ListCaseDirs lists all case directories under a suite.
func ListCaseDirs(basePath string, suiteID int64) ([]string, error) {
	suiteDir := filepath.Join(basePath, fmt.Sprintf("%d", suiteID))
	entries, err := os.ReadDir(suiteDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("list case dirs: %w", err)
	}
	var dirs []string
	for _, e := range entries {
		if e.IsDir() {
			dirs = append(dirs, filepath.Join(suiteDir, e.Name()))
		}
	}
	return dirs, nil
}

// ArtifactFilename returns the standard filename for each family's artifact.
func ArtifactFilename(step PipelineStep) string {
	switch step {
	case StepF0Recall:
		return "recall-result.json"
	case StepF1Triage:
		return "triage-result.json"
	case StepF2Resolve:
		return "resolve-result.json"
	case StepF3Invest:
		return "artifact.json"
	case StepF4Correlate:
		return "correlate-result.json"
	case StepF5Review:
		return "review-decision.json"
	case StepF6Report:
		return "jira-draft.json"
	default:
		return ""
	}
}

// PromptFilename returns the prompt output filename for a step and loop iteration.
func PromptFilename(step PipelineStep, loopIter int) string {
	family := step.Family()
	if family == "" {
		return ""
	}
	if loopIter > 0 {
		return fmt.Sprintf("prompt-%s-loop-%d.md", family, loopIter)
	}
	return fmt.Sprintf("prompt-%s.md", family)
}

// ReadArtifact reads a typed JSON artifact from the per-case directory.
func ReadArtifact[T any](caseDir, filename string) (*T, error) {
	path := filepath.Join(caseDir, filename)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read artifact %s: %w", filename, err)
	}
	var result T
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parse artifact %s: %w", filename, err)
	}
	return &result, nil
}

// WriteArtifact writes a typed JSON artifact to the per-case directory.
func WriteArtifact(caseDir, filename string, data any) error {
	path := filepath.Join(caseDir, filename)
	raw, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal artifact %s: %w", filename, err)
	}
	if err := os.WriteFile(path, raw, 0644); err != nil {
		return fmt.Errorf("write artifact %s: %w", filename, err)
	}
	return nil
}

// WritePrompt writes a filled prompt to the per-case directory.
// Returns the full path for the user to open.
func WritePrompt(caseDir string, step PipelineStep, loopIter int, content string) (string, error) {
	filename := PromptFilename(step, loopIter)
	if filename == "" {
		return "", fmt.Errorf("no prompt filename for step %s", step)
	}
	path := filepath.Join(caseDir, filename)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return "", fmt.Errorf("write prompt: %w", err)
	}
	return path, nil
}
