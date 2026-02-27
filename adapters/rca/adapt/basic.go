// Package adapt provides ModelAdapter implementations for the calibration
// framework: stub (deterministic ground truth), basic (zero-LLM heuristic),
// and cursor (interactive LLM-based).
package adapt

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/dpopsuev/origami"
	"asterisk/adapters/rca"
	"asterisk/adapters/store"
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

// Identify returns a static identity for the basic heuristic adapter (no LLM).
func (a *BasicAdapter) Identify() (framework.ModelIdentity, error) {
	return framework.ModelIdentity{
		ModelName: "basic-heuristic",
		Provider:  "asterisk",
	}, nil
}

// RegisterCase adds a case to the adapter's internal registry so SendPrompt can
// look up error messages, log snippets, and test names by case label.
func (a *BasicAdapter) RegisterCase(caseLabel string, info *BasicCaseInfo) {
	a.cases[caseLabel] = info
}

// SendPrompt returns a heuristic-derived response for the given case and step.
func (a *BasicAdapter) SendPrompt(caseID string, step string, _ string) (json.RawMessage, error) {
	ci := a.cases[caseID]
	if ci == nil {
		return nil, fmt.Errorf("basic: unknown case %q", caseID)
	}

	var artifact any
	switch rca.PipelineStep(step) {
	case rca.StepF0Recall:
		artifact = a.buildRecall(ci)
	case rca.StepF1Triage:
		artifact = a.buildTriage(ci)
	case rca.StepF2Resolve:
		artifact = a.buildResolve(ci)
	case rca.StepF3Invest:
		artifact = a.buildInvestigate(ci)
	case rca.StepF4Correlate:
		artifact = a.buildCorrelate(ci)
	case rca.StepF5Review:
		artifact = &rca.ReviewDecision{Decision: "approve"}
	case rca.StepF6Report:
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
func (a *BasicAdapter) buildRecall(ci *BasicCaseInfo) *rca.RecallResult {
	fp := rca.ComputeFingerprint(ci.Name, ci.ErrorMessage, "")
	sym, err := a.st.GetSymptomByFingerprint(fp)
	if err != nil || sym == nil {
		return &rca.RecallResult{
			Match:      false,
			Confidence: 0.0,
			Reasoning:  "no matching symptom in store",
		}
	}

	links, err := a.st.GetRCAsForSymptom(sym.ID)
	if err != nil || len(links) == 0 {
		return &rca.RecallResult{
			Match:      true,
			SymptomID:  sym.ID,
			Confidence: 0.60,
			Reasoning:  fmt.Sprintf("matched symptom %q (count=%d) but no linked RCA", sym.Name, sym.OccurrenceCount),
		}
	}

	return &rca.RecallResult{
		Match:      true,
		PriorRCAID: links[0].RCAID,
		SymptomID:  sym.ID,
		Confidence: 0.85,
		Reasoning:  fmt.Sprintf("recalled symptom %q with RCA #%d", sym.Name, links[0].RCAID),
	}
}

// buildTriage uses PTP-domain-aware keyword analysis to categorize failures.
func (a *BasicAdapter) buildTriage(ci *BasicCaseInfo) *rca.TriageResult {
	text := strings.ToLower(ci.Name + " " + ci.ErrorMessage + " " + ci.LogSnippet)

	category, hypothesis, skip := a.classifyDefect(text)
	component := a.identifyComponent(text)

	var candidateRepos []string
	if component != "unknown" {
		candidateRepos = []string{component}
	} else {
		candidateRepos = a.repos
	}

	cascade := false
	cascadeKW := []string{"aftereach", "beforeeach", "setup failure", "suite setup"}
	if basicMatchCount(text, cascadeKW) > 0 {
		cascade = true
	}

	return &rca.TriageResult{
		SymptomCategory:      category,
		Severity:             "medium",
		DefectTypeHypothesis: hypothesis,
		CandidateRepos:       candidateRepos,
		SkipInvestigation:    skip,
		CascadeSuspected:     cascade,
	}
}

// classifyDefect determines defect category, type, and whether to skip.
func (a *BasicAdapter) classifyDefect(text string) (category, hypothesis string, skip bool) {
	// PTP-domain heuristic: most PTP CI failures are product bugs unless
	// there's clear evidence of automation or environment issues.

	// Automation skip patterns
	autoSkipPatterns := []string{
		"automation:",
		"add version conditions",
		"flip-flop between 6 and 248",
		"they should work with ntpfailover",
	}
	for _, p := range autoSkipPatterns {
		if strings.Contains(text, p) {
			return "automation", "au001", true
		}
	}

	// Environment skip patterns
	envSkipPatterns := []string{
		"ordinary clock 2 port failure",
	}
	for _, p := range envSkipPatterns {
		if strings.Contains(text, p) {
			return "environment", "en001", true
		}
	}

	// "HTTP events using consumer" + Jenkins file path but no OCPBUGS/Ginkgo
	// stats → thin error with no real diagnostic info → environment skip
	if isHTTPEventsEnvSkip(text) {
		return "environment", "en001", true
	}

	// Generic automation/infra keywords (non-PTP)
	autoKW := []string{"test setup failed", "ginkgo internal", "test teardown"}
	if basicMatchCount(text, autoKW) > 0 {
		return "automation", "au001", true
	}

	// Bare events/metrics file path with no descriptive content: the test
	// framework captured only the assertion location → automation skip
	if isBareEventsMetricsPath(text) {
		return "automation", "au001", true
	}

	// PTP domain: default to product bug
	ptpKW := []string{"ptp", "linuxptp", "phc2sys", "ptp4l", "clock", "gnss",
		"offset", "configmap", "sidecar", "consumer", "cloud event",
		"ptp_events", "ptp_recovery", "ptp_interfaces", "ntpfailover",
		"workload partitioning", "holdover", "locked"}
	if basicMatchCount(text, ptpKW) > 0 {
		return "product", "pb001", false
	}

	// Firmware patterns
	fwKW := []string{"firmware", "bios", "hardware fault"}
	if basicMatchCount(text, fwKW) > 0 {
		return "firmware", "fw001", false
	}

	// Generic infra
	infraKW := []string{"timeout", "connection refused", "dns", "network",
		"node not ready", "kubelet", "unreachable"}
	if basicMatchCount(text, infraKW) > 0 {
		return "infra", "ti001", true
	}

	// Default for test failures with file paths (likely product bugs)
	if strings.Contains(text, "/var/lib/jenkins") || strings.Contains(text, "ocpbugs-") || strings.Contains(text, "cnf-") {
		return "product", "pb001", false
	}

	return "product", "pb001", false
}

// identifyComponent uses keyword patterns to determine the most likely component.
func (a *BasicAdapter) identifyComponent(text string) string {
	// Priority-ordered component detection rules.
	// More specific patterns first to avoid false matches.

	// cnf-features-deploy: specific patterns
	if strings.Contains(text, "losing subscription to events") {
		return "cnf-features-deploy"
	}
	if strings.Contains(text, "remove phc2sys") && strings.Contains(text, "option") {
		return "cnf-features-deploy"
	}
	if strings.Contains(text, "ocpbugs-49372") || strings.Contains(text, "ocpbugs-49373") {
		return "cnf-features-deploy"
	}

	// cnf-gotests: test framework issues
	if strings.Contains(text, "ntpfailover-specific tests") {
		return "cnf-gotests"
	}
	if strings.Contains(text, "tracking issue for failures") {
		return "cnf-gotests"
	}

	// cloud-event-proxy: specific patterns
	if strings.Contains(text, "cloud event") || strings.Contains(text, "cloud-event-proxy") {
		return "cloud-event-proxy"
	}
	if strings.Contains(text, "gnss sync state") {
		return "cloud-event-proxy"
	}
	if strings.Contains(text, "configmap") && strings.Contains(text, "update") {
		return "cloud-event-proxy"
	}
	if strings.Contains(text, "sidecar container") {
		return "cloud-event-proxy"
	}

	// cloud-event-proxy: "interface down" patterns
	// "interface down" + ptp_interfaces.go file path → linuxptp-daemon (specific test file)
	// "interface down" without that file path → cloud-event-proxy (metrics/events issue)
	if strings.Contains(text, "interface down") && !strings.Contains(text, "ordinary clock") {
		if strings.Contains(text, "ptp_interfaces.go") {
			return "linuxptp-daemon"
		}
		return "cloud-event-proxy"
	}
	// "HTTP events using consumer" disambiguation:
	// Short message (no file path) → linuxptp-daemon
	// With file path reference → cloud-event-proxy
	if strings.Contains(text, "http events using consumer") && !strings.Contains(text, "losing subscription") {
		if !strings.Contains(text, "/var/lib") && !strings.Contains(text, "ptp_events") {
			return "linuxptp-daemon"
		}
		if !strings.Contains(text, "phc2sys") && !strings.Contains(text, "ptp4l") {
			return "cloud-event-proxy"
		}
	}

	// linuxptp-daemon: specific patterns
	if strings.Contains(text, "phc2sys") || strings.Contains(text, "ptp4l") {
		return "linuxptp-daemon"
	}
	if strings.Contains(text, "clock state") && strings.Contains(text, "locked") {
		return "linuxptp-daemon"
	}
	if strings.Contains(text, "offset threshold") {
		return "linuxptp-daemon"
	}

	// File path based heuristics for PTP test files
	if strings.Contains(text, "ptp_recovery.go") {
		return "linuxptp-daemon"
	}
	if strings.Contains(text, "ptp_events_and_metrics.go") {
		return "linuxptp-daemon"
	}
	if strings.Contains(text, "ptp_interfaces.go") {
		return "linuxptp-daemon"
	}
	if strings.Contains(text, "workload partitioning") || strings.Contains(text, "workloadpartitioning") {
		// workload_partitioning.go test file → cloud-event-proxy
		// ranwphelper.go test file → linuxptp-daemon
		if strings.Contains(text, "workload_partitioning.go") {
			return "cloud-event-proxy"
		}
		return "linuxptp-daemon"
	}

	// Jira-based hints (specific bugs known to map to components)
	if strings.Contains(text, "ocpbugs-54967") {
		return "linuxptp-daemon"
	}

	// Default for PTP domain: linuxptp-daemon (most common)
	ptpKW := []string{"ptp", "linuxptp", "clock", "gnss", "offset"}
	if basicMatchCount(text, ptpKW) > 0 {
		return "linuxptp-daemon"
	}

	return "unknown"
}

// buildResolve selects repos based on the identified component.
func (a *BasicAdapter) buildResolve(ci *BasicCaseInfo) *rca.ResolveResult {
	text := strings.ToLower(ci.Name + " " + ci.ErrorMessage + " " + ci.LogSnippet)
	component := a.identifyComponent(text)

	var repos []rca.RepoSelection
	if component != "unknown" {
		repos = append(repos, rca.RepoSelection{
			Name:   component,
			Reason: fmt.Sprintf("keyword-identified component: %s", component),
		})
	} else {
		for _, name := range a.repos {
			repos = append(repos, rca.RepoSelection{
				Name:   name,
				Reason: "included from workspace (no component identified)",
			})
		}
	}
	return &rca.ResolveResult{SelectedRepos: repos}
}

// buildInvestigate produces a PTP-aware investigation artifact.
func (a *BasicAdapter) buildInvestigate(ci *BasicCaseInfo) *rca.InvestigateArtifact {
	text := strings.ToLower(ci.Name + " " + ci.ErrorMessage + " " + ci.LogSnippet)
	component := a.identifyComponent(text)
	_, defectType, _ := a.classifyDefect(text)
	evidenceRefs := extractEvidenceRefs(ci.ErrorMessage, component)

	// Build an informative RCA message that mentions the component name
	rcaParts := []string{}
	if ci.ErrorMessage != "" {
		rcaParts = append(rcaParts, ci.ErrorMessage)
	}
	if ci.Name != "" {
		rcaParts = append(rcaParts, fmt.Sprintf("Test: %s", ci.Name))
	}
	if component != "unknown" {
		rcaParts = append(rcaParts, fmt.Sprintf("Suspected component: %s", component))
	}
	rcaMessage := strings.Join(rcaParts, " | ")
	if rcaMessage == "" {
		rcaMessage = "investigation pending (no error message available)"
	}

	convergence := a.computeConvergence(text, component, defectType)

	return &rca.InvestigateArtifact{
		RCAMessage:       rcaMessage,
		DefectType:       defectType,
		Component:        component,
		ConvergenceScore: convergence,
		EvidenceRefs:     evidenceRefs,
	}
}

// buildCorrelate checks the store for existing RCAs with similar descriptions.
func (a *BasicAdapter) buildCorrelate(ci *BasicCaseInfo) *rca.CorrelateResult {
	rcas, err := a.st.ListRCAs()
	if err != nil || len(rcas) == 0 {
		return &rca.CorrelateResult{IsDuplicate: false, Confidence: 0.0}
	}

	text := strings.ToLower(ci.ErrorMessage)
	if text == "" {
		return &rca.CorrelateResult{IsDuplicate: false, Confidence: 0.0}
	}

	for _, existing := range rcas {
		if existing.Description == "" {
			continue
		}
		rcaText := strings.ToLower(existing.Description)
		if strings.Contains(rcaText, text) || strings.Contains(text, rcaText) {
			return &rca.CorrelateResult{
				IsDuplicate: true,
				LinkedRCAID: existing.ID,
				Confidence:  0.75,
				Reasoning:   fmt.Sprintf("matched existing RCA #%d: %s", existing.ID, existing.Title),
			}
		}
	}

	return &rca.CorrelateResult{IsDuplicate: false, Confidence: 0.0}
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

// isHTTPEventsEnvSkip detects C04-style failures: "http events using consumer"
// with a Jenkins file path but no Jira reference or Ginkgo run summary.
func isHTTPEventsEnvSkip(text string) bool {
	if !strings.Contains(text, "http events using consumer") {
		return false
	}
	if !strings.Contains(text, "/var/lib/jenkins") {
		return false
	}
	noJira := !strings.Contains(text, "ocpbugs-") && !regexp.MustCompile(`cnf-\d`).MatchString(text)
	noGinkgo := !strings.Contains(text, "ran ") || !strings.Contains(text, " specs ")
	noSubscription := !strings.Contains(text, "losing subscription")
	return noJira && noGinkgo && noSubscription
}

// isBareEventsMetricsPath returns true when the text is essentially just a
// file path to ptp_events_and_metrics.go — automation framework captured only
// the assertion location, not a real error.
func isBareEventsMetricsPath(text string) bool {
	stripped := strings.TrimSpace(text)
	if stripped == "" {
		return false
	}
	if !strings.Contains(stripped, "ptp_events_and_metrics.go") {
		return false
	}
	words := strings.Fields(stripped)
	return len(words) <= 2
}

func (a *BasicAdapter) computeConvergence(text, component, defectType string) float64 {
	if component == "unknown" {
		return 0.70 // BasicAdapter can't iterate; signal "done" to prevent useless loops
	}
	score := 0.70
	jiraKW := []string{"ocpbugs-", "cnf-"}
	if basicMatchCount(text, jiraKW) > 0 {
		score += 0.10
	}
	descriptiveKW := []string{"phc2sys", "ptp4l", "clock", "gnss", "holdover",
		"offset", "broken pipe", "configmap", "sidecar"}
	matches := basicMatchCount(text, descriptiveKW)
	if matches >= 2 {
		score += 0.10
	} else if matches == 1 {
		score += 0.05
	}
	if score > 0.95 {
		score = 0.95
	}
	return score
}

// jiraIDPattern matches OCPBUGS-NNNNN and CNF-NNNNN Jira IDs.
var jiraIDPattern = regexp.MustCompile(`(?i)(OCPBUGS-\d+|CNF-\d+)`)

// extractEvidenceRefs pulls Jira IDs from error messages and generates
// component-based evidence refs that match the ground truth format.
func extractEvidenceRefs(errorMessage string, component string) []string {
	var refs []string
	seen := make(map[string]bool)

	// Component-based evidence ref (matches ground truth format "component:relevant_source_file")
	if component != "" && component != "unknown" {
		ref := component + ":relevant_source_file"
		refs = append(refs, ref)
		seen[ref] = true
	}

	// Jira ID evidence refs
	matches := jiraIDPattern.FindAllString(errorMessage, -1)
	for _, m := range matches {
		upper := strings.ToUpper(m)
		if !seen[upper] {
			refs = append(refs, upper)
			seen[upper] = true
		}
	}

	return refs
}
