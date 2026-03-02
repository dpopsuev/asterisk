package rca

import (
	"context"
	"fmt"

	framework "github.com/dpopsuev/origami"
)

type resolveHeuristic struct {
	ht *heuristicTransformer
}

func (t *resolveHeuristic) Name() string { return "resolve-heuristic" }

func (t *resolveHeuristic) Transform(_ context.Context, tc *framework.TransformerContext) (any, error) {
	fp := failureFromContext(tc.WalkerState)
	text := t.ht.textFromFailure(fp)
	component := t.ht.identifyComponent(text)
	var repos []RepoSelection
	if component != "unknown" {
		repos = append(repos, RepoSelection{Name: component, Reason: fmt.Sprintf("keyword-identified component: %s", component)})
	} else {
		for _, name := range t.ht.repos {
			repos = append(repos, RepoSelection{Name: name, Reason: "included from workspace (no component identified)"})
		}
	}
	return &ResolveResult{SelectedRepos: repos}, nil
}
