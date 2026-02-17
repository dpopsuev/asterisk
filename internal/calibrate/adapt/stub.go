package adapt

import (
	"encoding/json"
	"fmt"
	"sync"

	"asterisk/internal/calibrate"
	"asterisk/internal/orchestrate"
)

// StubAdapter returns pre-authored "ideal" responses for each case+step.
// Deterministic: validates the pipeline/heuristic/metric machinery without LLM variance.
// Thread-safe: maps are protected by a mutex for parallel mode.
type StubAdapter struct {
	scenario *calibrate.Scenario
	mu       sync.RWMutex
	rcaIDMap     map[string]int64
	symptomIDMap map[string]int64
}

// NewStubAdapter creates a StubAdapter from a scenario definition.
func NewStubAdapter(scenario *calibrate.Scenario) *StubAdapter {
	return &StubAdapter{
		scenario:     scenario,
		rcaIDMap:     make(map[string]int64),
		symptomIDMap: make(map[string]int64),
	}
}

func (a *StubAdapter) Name() string { return "stub" }

// SetRCAID maps a ground truth RCA ID to a store-assigned ID. Thread-safe.
func (a *StubAdapter) SetRCAID(gtID string, storeID int64) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.rcaIDMap[gtID] = storeID
}

// SetSymptomID maps a ground truth symptom ID to a store-assigned ID. Thread-safe.
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

// SendPrompt returns the pre-authored ideal artifact for the given case and step.
func (a *StubAdapter) SendPrompt(caseID string, step orchestrate.PipelineStep, _ string) (json.RawMessage, error) {
	gtCase := a.findCase(caseID)
	if gtCase == nil {
		return nil, fmt.Errorf("stub: unknown case %q", caseID)
	}

	var artifact any
	switch step {
	case orchestrate.StepF0Recall:
		artifact = a.buildRecall(gtCase)
	case orchestrate.StepF1Triage:
		artifact = a.buildTriage(gtCase)
	case orchestrate.StepF2Resolve:
		artifact = a.buildResolve(gtCase)
	case orchestrate.StepF3Invest:
		artifact = a.buildInvestigate(gtCase)
	case orchestrate.StepF4Correlate:
		artifact = a.buildCorrelate(gtCase)
	case orchestrate.StepF5Review:
		artifact = a.buildReview(gtCase)
	case orchestrate.StepF6Report:
		artifact = a.buildReport(gtCase)
	default:
		return nil, fmt.Errorf("stub: no response for step %s", step)
	}

	data, err := json.Marshal(artifact)
	if err != nil {
		return nil, fmt.Errorf("stub: marshal artifact for %s/%s: %w", caseID, step, err)
	}
	return data, nil
}

func (a *StubAdapter) findCase(id string) *calibrate.GroundTruthCase {
	for i := range a.scenario.Cases {
		if a.scenario.Cases[i].ID == id {
			return &a.scenario.Cases[i]
		}
	}
	return nil
}

func (a *StubAdapter) findRCA(id string) *calibrate.GroundTruthRCA {
	for i := range a.scenario.RCAs {
		if a.scenario.RCAs[i].ID == id {
			return &a.scenario.RCAs[i]
		}
	}
	return nil
}

func (a *StubAdapter) buildRecall(c *calibrate.GroundTruthCase) *orchestrate.RecallResult {
	if c.ExpectedRecall != nil {
		r := &orchestrate.RecallResult{
			Match:      c.ExpectedRecall.Match,
			Confidence: c.ExpectedRecall.Confidence,
		}
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
	return &orchestrate.RecallResult{Match: false, Confidence: 0.0, Reasoning: "no recall data"}
}

func (a *StubAdapter) buildTriage(c *calibrate.GroundTruthCase) *orchestrate.TriageResult {
	if c.ExpectedTriage != nil {
		return &orchestrate.TriageResult{
			SymptomCategory:      c.ExpectedTriage.SymptomCategory,
			Severity:             c.ExpectedTriage.Severity,
			DefectTypeHypothesis: c.ExpectedTriage.DefectTypeHypothesis,
			CandidateRepos:       c.ExpectedTriage.CandidateRepos,
			SkipInvestigation:    c.ExpectedTriage.SkipInvestigation,
			CascadeSuspected:     c.ExpectedTriage.CascadeSuspected,
		}
	}
	return &orchestrate.TriageResult{SymptomCategory: "unknown"}
}

func (a *StubAdapter) buildResolve(c *calibrate.GroundTruthCase) *orchestrate.ResolveResult {
	if c.ExpectedResolve != nil {
		var repos []orchestrate.RepoSelection
		for _, r := range c.ExpectedResolve.SelectedRepos {
			repos = append(repos, orchestrate.RepoSelection{
				Name:   r.Name,
				Reason: r.Reason,
			})
		}
		return &orchestrate.ResolveResult{SelectedRepos: repos}
	}
	return &orchestrate.ResolveResult{}
}

func (a *StubAdapter) buildInvestigate(c *calibrate.GroundTruthCase) *orchestrate.InvestigateArtifact {
	if c.ExpectedInvest != nil {
		return &orchestrate.InvestigateArtifact{
			RCAMessage:       c.ExpectedInvest.RCAMessage,
			DefectType:       c.ExpectedInvest.DefectType,
			Component:        c.ExpectedInvest.Component,
			ConvergenceScore: c.ExpectedInvest.ConvergenceScore,
			EvidenceRefs:     c.ExpectedInvest.EvidenceRefs,
		}
	}
	return &orchestrate.InvestigateArtifact{ConvergenceScore: 0.5}
}

func (a *StubAdapter) buildCorrelate(c *calibrate.GroundTruthCase) *orchestrate.CorrelateResult {
	if c.ExpectedCorrelate != nil {
		r := &orchestrate.CorrelateResult{
			IsDuplicate:       c.ExpectedCorrelate.IsDuplicate,
			Confidence:        c.ExpectedCorrelate.Confidence,
			CrossVersionMatch: c.ExpectedCorrelate.CrossVersionMatch,
		}
		if c.ExpectedCorrelate.IsDuplicate && c.RCAID != "" {
			r.LinkedRCAID = a.getRCAID(c.RCAID)
		}
		return r
	}
	return &orchestrate.CorrelateResult{IsDuplicate: false}
}

func (a *StubAdapter) buildReview(c *calibrate.GroundTruthCase) *orchestrate.ReviewDecision {
	if c.ExpectedReview != nil {
		return &orchestrate.ReviewDecision{Decision: c.ExpectedReview.Decision}
	}
	return &orchestrate.ReviewDecision{Decision: "approve"}
}

func (a *StubAdapter) buildReport(c *calibrate.GroundTruthCase) map[string]any {
	rca := a.findRCA(c.RCAID)
	report := map[string]any{
		"case_id":     c.ID,
		"test_name":   c.TestName,
		"defect_type": "nd001",
	}
	if rca != nil {
		report["defect_type"] = rca.DefectType
		report["jira_id"] = rca.JiraID
		report["component"] = rca.Component
		report["summary"] = rca.Title
	}
	return report
}
