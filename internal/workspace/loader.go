package workspace

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	yaml "go.yaml.in/yaml/v3"
)

// LoadFromPath reads a workspace file (YAML or JSON) and returns the parsed Workspace.
// Format is detected by extension (.yaml/.yml → YAML, .json → JSON) or by content (first non-whitespace char).
func LoadFromPath(path string) (*Workspace, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read workspace: %w", err)
	}
	return Load(data, filepath.Ext(path))
}

// Load parses workspace from bytes. ext is the file extension (e.g. ".json", ".yaml") for format hint; empty = detect from content.
func Load(data []byte, ext string) (*Workspace, error) {
	ext = strings.ToLower(ext)
	if ext == ".yml" {
		ext = ".yaml"
	}
	if ext == ".yaml" {
		var w Workspace
		if err := yaml.Unmarshal(data, &w); err != nil {
			return nil, fmt.Errorf("parse workspace yaml: %w", err)
		}
		return &w, nil
	}
	if ext == ".json" {
		var w Workspace
		if err := json.Unmarshal(data, &w); err != nil {
			return nil, fmt.Errorf("parse workspace json: %w", err)
		}
		return &w, nil
	}
	// Detect: try JSON first (starts with {), else YAML
	trimmed := strings.TrimSpace(string(data))
	if strings.HasPrefix(trimmed, "{") {
		var w Workspace
		if err := json.Unmarshal(data, &w); err != nil {
			return nil, fmt.Errorf("parse workspace json: %w", err)
		}
		return &w, nil
	}
	var w Workspace
	if err := yaml.Unmarshal(data, &w); err != nil {
		return nil, fmt.Errorf("parse workspace yaml: %w", err)
	}
	return &w, nil
}
