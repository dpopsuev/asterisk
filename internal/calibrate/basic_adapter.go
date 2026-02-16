package calibrate

import (
	"encoding/json"
	"fmt"
	"strings"

	"asterisk/internal/orchestrate"
	"asterisk/internal/store"
)

// BasicAdapter provides automated heuristic-based responses for each pipeline step.
// Unlike StubAdapter (which returns pre-authored ideal answers from ground truth),
// BasicAdapter derives responses from actual case data using keyword analysis and
// store lookups. This is the "zero-LLM" baseline adapter for real investigation.
type BasicAdapter struct {
	st    store.Store
	cases map[string]*BasicCaseInfo
	repos []string
}

// BasicCaseInfo holds the metadata the adapter needs for a specific case.
type BasicCaseInfo struct {
	Name         string
	ErrorMessage string
	LogSnippet   string
	StoreCaseID  int64
}

// NewBasicAdapter creates a BasicAdapter backed by the given store and repo list.
func NewBasicAdapter(st store.Store, repos []string) *BasicAdapter {
	return &BasicAdapter{
		st:    st,
		cases: make(map[string]*BasicCaseInfo),
		repos: repos,
	}
}

// Name returns the adapter identifier.
func (a *BasicAdapter) Name() string { return "basic" }

// RegisterCase adds a case to the adapter's internal registry so SendPrompt can
// look up error messages, log snippets, and test names by case label.
func (a *BasicAdapter) RegisterCase(caseLabel string, info *BasicCaseInfo) {
	a.cases[caseLabel] = info
}

// SendPrompt returns a heuristic-derived response for the given case and step.
func (a *BasicAdapter) SendPrompt(caseID string, step orchestrate.PipelineStep, _ string) (json.RawMessage, error) {
	ci := a.cases[caseID]
	if ci == nil {
		return nil, fmt.Errorf("basic: unknown case %q", caseID)
	}

	var artifact any
	switch step {
	case orchestrate.StepF0Recall:
		artifact = a.buildRecall(ci)
	case orchestrate.StepF1Triage:
		artifact = a.buildTriage(ci)
	case orchestrate.StepF2Resolve:
		artifact = a.buildResolve(ci)
	case orchestrate.StepF3Invest:
		artifact = a.buildInvestigate(ci)
	case orchestrate.StepF4Correlate:
		artifact = a.buildCorrelate(ci)
	case orchestrate.StepF5Review:
		artifact = &orchestrate.ReviewDecision{Decision: "approve"}
	case orchestrate.StepF6Report:
		artifact = map[string]any{
			"case_id":   caseID,
			"test_name": ci.Name,
			"summary":   "automated baseline analysis",
		}
	default:
		return nil, fmt.Errorf("basic: no response for step %s", step)
	}

	data, err := json.Marshal(artifact)
	if err != nil {
		return nil, fmt.Errorf("basic: marshal: %w", err)
	}
	return data, nil
}

// buildRecall checks the store for a matching symptom by fingerprint.
// Returns a recall hit if a symptom (and ideally an RCA) already exists.
func (a *BasicAdapter) buildRecall(ci *BasicCaseInfo) *orchestrate.RecallResult {
	fp := orchestrate.ComputeFingerprint(ci.Name, ci.ErrorMessage, "")
	sym, err := a.st.GetSymptomByFingerprint(fp)
	if err != nil || sym == nil {
		return &orchestrate.RecallResult{
			Match:      false,
			Confidence: 0.0,
			Reasoning:  "no matching symptom in store",
		}
	}

	links, err := a.st.GetRCAsForSymptom(sym.ID)
	if err != nil || len(links) == 0 {
		return &orchestrate.RecallResult{
			Match:      true,
			SymptomID:  sym.ID,
			Confidence: 0.60,
			Reasoning:  fmt.Sprintf("matched symptom %q (count=%d) but no linked RCA", sym.Name, sym.OccurrenceCount),
		}
	}

	return &orchestrate.RecallResult{
		Match:      true,
		PriorRCAID: links[0].RCAID,
		SymptomID:  sym.ID,
		Confidence: 0.85,
		Reasoning:  fmt.Sprintf("recalled symptom %q with RCA #%d", sym.Name, links[0].RCAID),
	}
}

// buildTriage uses keyword analysis on the error message and log to categorize.
func (a *BasicAdapter) buildTriage(ci *BasicCaseInfo) *orchestrate.TriageResult {
	text := strings.ToLower(ci.Name + " " + ci.ErrorMessage + " " + ci.LogSnippet)

	infraKW := []string{"timeout", "connection refused", "dns", "network", "node not ready",
		"kubelet", "unreachable", "certificate", "tls", "503", "502"}
	productKW := []string{"broken pipe", "nil pointer", "panic", "segfault", "config",
		"null pointer", "bus error", "signal", "fatal"}
	autoKW := []string{"aftereach", "beforeeach", "test setup", "test teardown",
		"ginkgo", "gomega", "assertion", "expected"}
	flakeKW := []string{"eventually", "consistently", "race", "intermittent", "flaky", "retry"}
	cascadeKW := []string{"aftereach", "beforeeach", "setup failure", "suite setup"}

	infraScore := basicMatchCount(text, infraKW)
	productScore := basicMatchCount(text, productKW)
	autoScore := basicMatchCount(text, autoKW)
	flakeScore := basicMatchCount(text, flakeKW)

	category := "unknown"
	hypothesis := "nd001"
	severity := "medium"
	cascade := false

	switch {
	case flakeScore > 0 && flakeScore >= productScore:
		category = "flake"
		hypothesis = "fl001"
	case infraScore > productScore && infraScore > autoScore:
		category = "infra"
		hypothesis = "ti001"
	case autoScore > productScore:
		category = "automation"
		hypothesis = "ab001"
	case productScore > 0:
		category = "product"
		hypothesis = "pb001"
	}

	if basicMatchCount(text, cascadeKW) > 0 {
		cascade = true
	}

	skip := category == "infra" || category == "flake"

	return &orchestrate.TriageResult{
		SymptomCategory:      category,
		Severity:             severity,
		DefectTypeHypothesis: hypothesis,
		CandidateRepos:       a.repos,
		SkipInvestigation:    skip,
		CascadeSuspected:     cascade,
	}
}

// buildResolve selects all workspace repos for investigation.
func (a *BasicAdapter) buildResolve(ci *BasicCaseInfo) *orchestrate.ResolveResult {
	var repos []orchestrate.RepoSelection
	for _, name := range a.repos {
		repos = append(repos, orchestrate.RepoSelection{
			Name:   name,
			Reason: "included from workspace",
		})
	}
	return &orchestrate.ResolveResult{SelectedRepos: repos}
}

// buildInvestigate produces a baseline investigation artifact from the error data.
func (a *BasicAdapter) buildInvestigate(ci *BasicCaseInfo) *orchestrate.InvestigateArtifact {
	rca := ci.ErrorMessage
	if rca == "" {
		rca = "investigation pending (no error message available)"
	}

	defectType := "nd001"
	component := "unknown"
	convergence := 0.40

	text := strings.ToLower(ci.ErrorMessage + " " + ci.LogSnippet)
	if strings.Contains(text, "broken pipe") || strings.Contains(text, "panic") {
		defectType = "pb001"
		convergence = 0.55
	} else if strings.Contains(text, "timeout") || strings.Contains(text, "connection refused") {
		defectType = "ti001"
		convergence = 0.50
	}

	return &orchestrate.InvestigateArtifact{
		RCAMessage:       rca,
		DefectType:       defectType,
		Component:        component,
		ConvergenceScore: convergence,
		EvidenceRefs:     []string{},
	}
}

// buildCorrelate checks the store for existing RCAs with similar descriptions.
func (a *BasicAdapter) buildCorrelate(ci *BasicCaseInfo) *orchestrate.CorrelateResult {
	rcas, err := a.st.ListRCAs()
	if err != nil || len(rcas) == 0 {
		return &orchestrate.CorrelateResult{IsDuplicate: false, Confidence: 0.0}
	}

	text := strings.ToLower(ci.ErrorMessage)
	if text == "" {
		return &orchestrate.CorrelateResult{IsDuplicate: false, Confidence: 0.0}
	}

	for _, rca := range rcas {
		if rca.Description == "" {
			continue
		}
		rcaText := strings.ToLower(rca.Description)
		if strings.Contains(rcaText, text) || strings.Contains(text, rcaText) {
			return &orchestrate.CorrelateResult{
				IsDuplicate: true,
				LinkedRCAID: rca.ID,
				Confidence:  0.75,
				Reasoning:   fmt.Sprintf("matched existing RCA #%d: %s", rca.ID, rca.Title),
			}
		}
	}

	return &orchestrate.CorrelateResult{IsDuplicate: false, Confidence: 0.0}
}

// basicMatchCount counts how many keywords appear in the text.
func basicMatchCount(text string, keywords []string) int {
	count := 0
	for _, kw := range keywords {
		if strings.Contains(text, kw) {
			count++
		}
	}
	return count
}
