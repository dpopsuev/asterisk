package rca

import (
	"context"
	"fmt"

	framework "github.com/dpopsuev/origami"
)

type recallHeuristic struct {
	ht *heuristicTransformer
}

func (t *recallHeuristic) Name() string { return "recall-heuristic" }

func (t *recallHeuristic) Transform(_ context.Context, tc *framework.TransformerContext) (any, error) {
	fp := failureFromContext(tc.WalkerState)
	fingerprint := ComputeFingerprint(fp.name, fp.errorMessage, "")
	sym, err := t.ht.st.GetSymptomByFingerprint(fingerprint)
	if err != nil || sym == nil {
		return &RecallResult{
			Match: false, Confidence: 0.0,
			Reasoning: "no matching symptom in store",
		}, nil
	}
	links, err := t.ht.st.GetRCAsForSymptom(sym.ID)
	if err != nil || len(links) == 0 {
		return &RecallResult{
			Match: true, SymptomID: sym.ID, Confidence: 0.60,
			Reasoning: fmt.Sprintf("matched symptom %q (count=%d) but no linked RCA", sym.Name, sym.OccurrenceCount),
		}, nil
	}
	return &RecallResult{
		Match: true, PriorRCAID: links[0].RCAID, SymptomID: sym.ID, Confidence: 0.85,
		Reasoning: fmt.Sprintf("recalled symptom %q with RCA #%d", sym.Name, links[0].RCAID),
	}, nil
}
