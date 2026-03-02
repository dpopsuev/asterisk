package rca

import (
	"context"

	framework "github.com/dpopsuev/origami"
)

type triageHeuristic struct {
	ht *heuristicTransformer
}

func (t *triageHeuristic) Name() string { return "triage-heuristic" }

func (t *triageHeuristic) Transform(_ context.Context, tc *framework.TransformerContext) (any, error) {
	fp := failureFromContext(tc.WalkerState)
	text := t.ht.textFromFailure(fp)
	category, hypothesis, skip := t.ht.classifyDefect(text)
	component := t.ht.identifyComponent(text)

	var candidateRepos []string
	if component != "unknown" {
		candidateRepos = []string{component}
	} else {
		candidateRepos = t.ht.repos
	}

	cascade := matchCount(text, cascadeKeywords()) > 0

	return &TriageResult{
		SymptomCategory:      category,
		Severity:             "medium",
		DefectTypeHypothesis: hypothesis,
		CandidateRepos:       candidateRepos,
		SkipInvestigation:    skip,
		CascadeSuspected:     cascade,
	}, nil
}
