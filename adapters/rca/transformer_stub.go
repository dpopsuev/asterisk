package rca

import (
	"context"
	"fmt"
	"sync"

	framework "github.com/dpopsuev/origami"
)

type stubTransformer struct {
	scenario     *Scenario
	mu           sync.RWMutex
	rcaIDMap     map[string]int64
	symptomIDMap map[string]int64
}

func NewStubTransformer(scenario *Scenario) *stubTransformer {
	return &stubTransformer{
		scenario:     scenario,
		rcaIDMap:     make(map[string]int64),
		symptomIDMap: make(map[string]int64),
	}
}

func (t *stubTransformer) Name() string { return "stub" }

func (t *stubTransformer) SetRCAID(gtID string, storeID int64) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.rcaIDMap[gtID] = storeID
}

func (t *stubTransformer) SetSymptomID(gtID string, storeID int64) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.symptomIDMap[gtID] = storeID
}

func (t *stubTransformer) getRCAID(gtID string) int64 {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.rcaIDMap[gtID]
}

func (t *stubTransformer) getSymptomID(gtID string) int64 {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.symptomIDMap[gtID]
}

func (t *stubTransformer) Transform(_ context.Context, tc *framework.TransformerContext) (any, error) {
	step := NodeNameToStep(tc.NodeName)
	if step == "" {
		return nil, fmt.Errorf("stub transformer: unknown node %q", tc.NodeName)
	}

	caseID := tc.WalkerState.ID
	gtCase := t.findCase(caseID)
	if gtCase == nil {
		return nil, fmt.Errorf("stub transformer: unknown case %q", caseID)
	}

	switch step {
	case StepF0Recall:
		return t.buildRecall(gtCase), nil
	case StepF1Triage:
		return t.buildTriage(gtCase), nil
	case StepF2Resolve:
		return t.buildResolve(gtCase), nil
	case StepF3Invest:
		return t.buildInvestigate(gtCase), nil
	case StepF4Correlate:
		return t.buildCorrelate(gtCase), nil
	case StepF5Review:
		return t.buildReview(gtCase), nil
	case StepF6Report:
		return t.buildReport(gtCase), nil
	default:
		return nil, fmt.Errorf("stub transformer: no response for step %s", step)
	}
}

func (t *stubTransformer) findCase(id string) *GroundTruthCase {
	for i := range t.scenario.Cases {
		if t.scenario.Cases[i].ID == id {
			return &t.scenario.Cases[i]
		}
	}
	return nil
}

func (t *stubTransformer) findRCA(id string) *GroundTruthRCA {
	for i := range t.scenario.RCAs {
		if t.scenario.RCAs[i].ID == id {
			return &t.scenario.RCAs[i]
		}
	}
	return nil
}

func (t *stubTransformer) buildRecall(c *GroundTruthCase) *RecallResult {
	if c.ExpectedRecall != nil {
		r := &RecallResult{Match: c.ExpectedRecall.Match, Confidence: c.ExpectedRecall.Confidence}
		if c.ExpectedRecall.Match {
			r.Reasoning = fmt.Sprintf("Recalled prior RCA for symptom matching case %s", c.ID)
			if c.RCAID != "" {
				r.PriorRCAID = t.getRCAID(c.RCAID)
			}
			if c.SymptomID != "" {
				r.SymptomID = t.getSymptomID(c.SymptomID)
			}
		} else {
			r.Reasoning = "No prior RCA found matching this failure pattern"
		}
		return r
	}
	return &RecallResult{Match: false, Confidence: 0.0, Reasoning: "no recall data"}
}

func (t *stubTransformer) buildTriage(c *GroundTruthCase) *TriageResult {
	if c.ExpectedTriage != nil {
		return &TriageResult{
			SymptomCategory:      c.ExpectedTriage.SymptomCategory,
			Severity:             c.ExpectedTriage.Severity,
			DefectTypeHypothesis: c.ExpectedTriage.DefectTypeHypothesis,
			CandidateRepos:       c.ExpectedTriage.CandidateRepos,
			SkipInvestigation:    c.ExpectedTriage.SkipInvestigation,
			CascadeSuspected:     c.ExpectedTriage.CascadeSuspected,
		}
	}
	return &TriageResult{SymptomCategory: "unknown"}
}

func (t *stubTransformer) buildResolve(c *GroundTruthCase) *ResolveResult {
	if c.ExpectedResolve != nil {
		var repos []RepoSelection
		for _, r := range c.ExpectedResolve.SelectedRepos {
			repos = append(repos, RepoSelection{Name: r.Name, Reason: r.Reason})
		}
		return &ResolveResult{SelectedRepos: repos}
	}
	return &ResolveResult{}
}

func (t *stubTransformer) buildInvestigate(c *GroundTruthCase) *InvestigateArtifact {
	if c.ExpectedInvest != nil {
		return &InvestigateArtifact{
			RCAMessage:       c.ExpectedInvest.RCAMessage,
			DefectType:       c.ExpectedInvest.DefectType,
			Component:        c.ExpectedInvest.Component,
			ConvergenceScore: c.ExpectedInvest.ConvergenceScore,
			EvidenceRefs:     c.ExpectedInvest.EvidenceRefs,
		}
	}
	return &InvestigateArtifact{ConvergenceScore: 0.5}
}

func (t *stubTransformer) buildCorrelate(c *GroundTruthCase) *CorrelateResult {
	if c.ExpectedCorrelate != nil {
		r := &CorrelateResult{
			IsDuplicate:       c.ExpectedCorrelate.IsDuplicate,
			Confidence:        c.ExpectedCorrelate.Confidence,
			CrossVersionMatch: c.ExpectedCorrelate.CrossVersionMatch,
		}
		if c.ExpectedCorrelate.IsDuplicate && c.RCAID != "" {
			r.LinkedRCAID = t.getRCAID(c.RCAID)
		}
		return r
	}
	return &CorrelateResult{IsDuplicate: false}
}

func (t *stubTransformer) buildReview(c *GroundTruthCase) *ReviewDecision {
	if c.ExpectedReview != nil {
		return &ReviewDecision{Decision: c.ExpectedReview.Decision}
	}
	return &ReviewDecision{Decision: "approve"}
}

func (t *stubTransformer) buildReport(c *GroundTruthCase) map[string]any {
	rcaDef := t.findRCA(c.RCAID)
	report := map[string]any{"case_id": c.ID, "test_name": c.TestName, "defect_type": "nd001"}
	if rcaDef != nil {
		report["defect_type"] = rcaDef.DefectType
		report["jira_id"] = rcaDef.JiraID
		report["component"] = rcaDef.Component
		report["summary"] = rcaDef.Title
	}
	return report
}
