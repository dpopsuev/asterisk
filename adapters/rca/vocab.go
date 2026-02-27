package rca

import (
	framework "github.com/dpopsuev/origami"

	"asterisk/adapters/vocabulary"
)

var defaultVocab = vocabulary.New()

func vocabName(code string) string {
	return defaultVocab.Name(code)
}

func vocabNameWithCode(code string) string {
	return framework.NameWithCode(defaultVocab, code)
}

func vocabStagePath(codes []string) string {
	return vocabulary.StagePath(defaultVocab, codes)
}

func vocabRPIssueTag(issueType string, autoAnalyzed bool) string {
	return vocabulary.RPIssueTag(defaultVocab, issueType, autoAnalyzed)
}

func vocabClusterKey(key string) string {
	return vocabulary.ClusterKey(defaultVocab, key)
}
