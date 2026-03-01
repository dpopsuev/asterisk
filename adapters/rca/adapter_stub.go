package rca

import (
	"encoding/json"
	"fmt"
	"sync"

	framework "github.com/dpopsuev/origami"
)

// StubAdapter returns pre-authored "ideal" responses for each case+step.
// Deterministic: validates circuit/heuristic/metric machinery without LLM variance.
// Thread-safe: maps are protected by a mutex for parallel mode.
type StubAdapter struct {
	scenario     *Scenario
	mu           sync.RWMutex
	rcaIDMap     map[string]int64
	symptomIDMap map[string]int64
}

func NewStubAdapter(scenario *Scenario) *StubAdapter {
	return &StubAdapter{
		scenario:     scenario,
		rcaIDMap:     make(map[string]int64),
		symptomIDMap: make(map[string]int64),
	}
}

func (a *StubAdapter) Name() string { return "stub" }

func (a *StubAdapter) Identify() (framework.ModelIdentity, error) {
	return framework.ModelIdentity{ModelName: "stub", Provider: "asterisk"}, nil
}

func (a *StubAdapter) SetRCAID(gtID string, storeID int64) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.rcaIDMap[gtID] = storeID
}

func (a *StubAdapter) SetSymptomID(gtID string, storeID int64) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.symptomIDMap[gtID] = storeID
}

func (a *StubAdapter) getRCAID(gtID string) int64 {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.rcaIDMap[gtID]
}

func (a *StubAdapter) getSymptomID(gtID string) int64 {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.symptomIDMap[gtID]
}

func (a *StubAdapter) SendPrompt(caseID string, step string, _ string) (json.RawMessage, error) {
	gtCase := a.findCase(caseID)
	if gtCase == nil {
		return nil, fmt.Errorf("stub: unknown case %q", caseID)
	}

	var artifact any
	switch CircuitStep(step) {
	case StepF0Recall:
		artifact = a.buildRecall(gtCase)
	case StepF1Triage:
		artifact = a.buildTriage(gtCase)
	case StepF2Resolve:
		artifact = a.buildResolve(gtCase)
	case StepF3Invest:
		artifact = a.buildInvestigate(gtCase)
	case StepF4Correlate:
		artifact = a.buildCorrelate(gtCase)
	case StepF5Review:
		artifact = a.buildReview(gtCase)
	case StepF6Report:
		artifact = a.buildReport(gtCase)
	default:
		return nil, fmt.Errorf("stub: no response for step %s", step)
	}

	data, err := json.Marshal(artifact)
	if err != nil {
		return nil, fmt.Errorf("stub: marshal: %w", err)
	}
	return data, nil
}

func (a *StubAdapter) findCase(id string) *GroundTruthCase {
	for i := range a.scenario.Cases {
		if a.scenario.Cases[i].ID == id {
			return &a.scenario.Cases[i]
		}
	}
	return nil
}

func (a *StubAdapter) findRCA(id string) *GroundTruthRCA {
	for i := range a.scenario.RCAs {
		if a.scenario.RCAs[i].ID == id {
			return &a.scenario.RCAs[i]
		}
	}
	return nil
}

func (a *StubAdapter) buildRecall(c *GroundTruthCase) *RecallResult {
	if c.ExpectedRecall != nil {
		r := &RecallResult{Match: c.ExpectedRecall.Match, Confidence: c.ExpectedRecall.Confidence}
		if c.ExpectedRecall.Match {
			r.Reasoning = fmt.Sprintf("Recalled prior RCA for symptom matching case %s", c.ID)
			if c.RCAID != "" {
				r.PriorRCAID = a.getRCAID(c.RCAID)
			}
			if c.SymptomID != "" {
				r.SymptomID = a.getSymptomID(c.SymptomID)
			}
		} else {
			r.Reasoning = "No prior RCA found matching this failure pattern"
		}
		return r
	}
	return &RecallResult{Match: false, Confidence: 0.0, Reasoning: "no recall data"}
}

func (a *StubAdapter) buildTriage(c *GroundTruthCase) *TriageResult {
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

func (a *StubAdapter) buildResolve(c *GroundTruthCase) *ResolveResult {
	if c.ExpectedResolve != nil {
		var repos []RepoSelection
		for _, r := range c.ExpectedResolve.SelectedRepos {
			repos = append(repos, RepoSelection{Name: r.Name, Reason: r.Reason})
		}
		return &ResolveResult{SelectedRepos: repos}
	}
	return &ResolveResult{}
}

func (a *StubAdapter) buildInvestigate(c *GroundTruthCase) *InvestigateArtifact {
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

func (a *StubAdapter) buildCorrelate(c *GroundTruthCase) *CorrelateResult {
	if c.ExpectedCorrelate != nil {
		r := &CorrelateResult{
			IsDuplicate:       c.ExpectedCorrelate.IsDuplicate,
			Confidence:        c.ExpectedCorrelate.Confidence,
			CrossVersionMatch: c.ExpectedCorrelate.CrossVersionMatch,
		}
		if c.ExpectedCorrelate.IsDuplicate && c.RCAID != "" {
			r.LinkedRCAID = a.getRCAID(c.RCAID)
		}
		return r
	}
	return &CorrelateResult{IsDuplicate: false}
}

func (a *StubAdapter) buildReview(c *GroundTruthCase) *ReviewDecision {
	if c.ExpectedReview != nil {
		return &ReviewDecision{Decision: c.ExpectedReview.Decision}
	}
	return &ReviewDecision{Decision: "approve"}
}

func (a *StubAdapter) buildReport(c *GroundTruthCase) map[string]any {
	rcaDef := a.findRCA(c.RCAID)
	report := map[string]any{"case_id": c.ID, "test_name": c.TestName, "defect_type": "nd001"}
	if rcaDef != nil {
		report["defect_type"] = rcaDef.DefectType
		report["jira_id"] = rcaDef.JiraID
		report["component"] = rcaDef.Component
		report["summary"] = rcaDef.Title
	}
	return report
}
