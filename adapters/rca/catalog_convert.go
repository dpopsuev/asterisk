package rca

import (
	"strings"

	"github.com/dpopsuev/origami/knowledge"
)

// ScenarioToCatalog converts a WorkspaceConfig to a knowledge.KnowledgeSourceCatalog
// for inject hooks and template parameter assembly.
func ScenarioToCatalog(wc WorkspaceConfig) *knowledge.KnowledgeSourceCatalog {
	cat := &knowledge.KnowledgeSourceCatalog{}
	for _, r := range wc.Repos {
		tags := map[string]string{
			"layer": "base",
		}
		if r.Purpose != "" {
			tags["role"] = inferRole(r.Purpose)
		}
		cat.Sources = append(cat.Sources, knowledge.Source{
			Name:    r.Name,
			Kind:    knowledge.SourceKindRepo,
			URI:     r.Path,
			Purpose: r.Purpose,
			Branch:  r.Branch,
			Tags:    tags,
		})
	}
	cat.Sources = append(cat.Sources, wc.Sources...)
	return cat
}

// inferRole derives a tag role from a source's purpose string.
func inferRole(purpose string) string {
	switch {
	case containsAny(purpose, "SUT", "lifecycle", "operator", "daemon"):
		return "sut"
	case containsAny(purpose, "test", "e2e", "framework", "gotests"):
		return "test"
	case containsAny(purpose, "doc", "architecture", "reference"):
		return "reference"
	case containsAny(purpose, "deploy", "manifests", "CI", "config"):
		return "infra"
	default:
		return "other"
	}
}

func containsAny(s string, substrs ...string) bool {
	lower := strings.ToLower(s)
	for _, sub := range substrs {
		if strings.Contains(lower, strings.ToLower(sub)) {
			return true
		}
	}
	return false
}
