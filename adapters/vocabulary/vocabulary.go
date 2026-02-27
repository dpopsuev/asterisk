// Package vocabulary provides the Asterisk domain vocabulary as a
// RichMapVocabulary. It registers defect types, pipeline stages,
// metrics, and heuristics — all four display-name domains.
package vocabulary

import (
	_ "embed"
	"fmt"
	"strings"

	framework "github.com/dpopsuev/origami"
	"gopkg.in/yaml.v3"
)

//go:embed vocabulary.yaml
var vocabData []byte

type vocabFile struct {
	DefectTypes map[string]framework.VocabEntry `yaml:"defect_types"`
	Stages      map[string]framework.VocabEntry `yaml:"stages"`
	Metrics     map[string]framework.VocabEntry `yaml:"metrics"`
	Heuristics  map[string]framework.VocabEntry `yaml:"heuristics"`
}

// New builds and returns a fully populated RichMapVocabulary containing
// all Asterisk domain codes: defect types, pipeline stages, metrics,
// and heuristics.
func New() *framework.RichMapVocabulary {
	v := framework.NewRichMapVocabulary()

	var f vocabFile
	if err := yaml.Unmarshal(vocabData, &f); err != nil {
		panic(fmt.Sprintf("vocabulary: parse embedded YAML: %v", err))
	}

	v.RegisterEntries(f.DefectTypes)
	v.RegisterEntries(f.Stages)
	v.RegisterEntries(f.Metrics)
	v.RegisterEntries(f.Heuristics)
	return v
}

// --- Domain helpers (composite logic beyond simple lookup) ---

// RPIssueTag formats an RP-provided issue type with a trust indicator.
// autoAnalyzed=true -> "[auto]" (ML-assigned, low trust); false -> "[human]".
// Returns "" when issueType is empty.
func RPIssueTag(v framework.Vocabulary, issueType string, autoAnalyzed bool) string {
	if issueType == "" {
		return ""
	}
	tag := "[human]"
	if autoAnalyzed {
		tag = "[auto]"
	}
	return v.Name(issueType) + " " + tag
}

// StagePath converts a slice of stage codes to a human-readable path.
// ["F0", "F1", "F2"] -> "Recall -> Triage -> Resolve"
func StagePath(v framework.Vocabulary, codes []string) string {
	names := make([]string, len(codes))
	for i, c := range codes {
		names[i] = v.Name(c)
	}
	return strings.Join(names, " \u2192 ")
}

// ClusterKey humanizes a pipe-delimited cluster key.
// "product|ptp4l|pb001" -> "product / ptp4l / Product Bug"
func ClusterKey(v framework.Vocabulary, key string) string {
	parts := strings.Split(key, "|")
	for i, p := range parts {
		if name := v.Name(p); name != p {
			parts[i] = name
		}
	}
	return strings.Join(parts, " / ")
}
