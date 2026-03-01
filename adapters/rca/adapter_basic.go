package rca

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	framework "github.com/dpopsuev/origami"

	"asterisk/adapters/store"

	"gopkg.in/yaml.v3"
)

//go:embed heuristics.yaml
var heuristicsYAML []byte

type heuristicsData struct {
	DefectClassification struct {
		AutomationSkip  [][]string `yaml:"automation_skip"`
		EnvironmentSkip [][]string `yaml:"environment_skip"`
		AutomationGen   [][]string `yaml:"automation_generic"`
		Product         [][]string `yaml:"product"`
		Firmware        [][]string `yaml:"firmware"`
		Infra           [][]string `yaml:"infra"`
	} `yaml:"defect_classification"`
	ComponentRules  map[string][][]string `yaml:"component_rules"`
	CascadeKeywords []string              `yaml:"cascade_keywords"`
	Convergence     struct {
		JiraKW        []string `yaml:"jira_keywords"`
		DescriptiveKW []string `yaml:"descriptive_keywords"`
		VersionKW     []string `yaml:"version_keywords"`
	} `yaml:"convergence"`
}

var loadedHeuristics *heuristicsData

func getHeuristics() *heuristicsData {
	if loadedHeuristics != nil {
		return loadedHeuristics
	}
	var h heuristicsData
	if err := yaml.Unmarshal(heuristicsYAML, &h); err != nil {
		panic(fmt.Sprintf("load heuristics.yaml: %v", err))
	}
	loadedHeuristics = &h
	return &h
}

// BasicAdapter provides automated heuristic-based responses for each pipeline step.
// Keyword rules are loaded from heuristics.yaml (embedded at build time).
type BasicAdapter struct {
	st    store.Store
	cases map[string]*BasicCaseInfo
	repos []string
	h     *heuristicsData
}

// BasicCaseInfo holds the metadata the adapter needs for a specific case.
type BasicCaseInfo struct {
	Name         string
	ErrorMessage string
	LogSnippet   string
	StoreCaseID  int64
}

func NewBasicAdapter(st store.Store, repos []string) *BasicAdapter {
	return &BasicAdapter{
		st:    st,
		cases: make(map[string]*BasicCaseInfo),
		repos: repos,
		h:     getHeuristics(),
	}
}

func (a *BasicAdapter) Name() string { return "basic" }

func (a *BasicAdapter) Identify() (framework.ModelIdentity, error) {
	return framework.ModelIdentity{
		ModelName: "basic-heuristic",
		Provider:  "asterisk",
	}, nil
}

func (a *BasicAdapter) RegisterCase(caseLabel string, info *BasicCaseInfo) {
	a.cases[caseLabel] = info
}

func (a *BasicAdapter) SendPrompt(caseID string, step string, _ string) (json.RawMessage, error) {
	ci := a.cases[caseID]
	if ci == nil {
		return nil, fmt.Errorf("basic: unknown case %q", caseID)
	}

	var artifact any
	switch PipelineStep(step) {
	case StepF0Recall:
		artifact = a.buildRecall(ci)
	case StepF1Triage:
		artifact = a.buildTriage(ci)
	case StepF2Resolve:
		artifact = a.buildResolve(ci)
	case StepF3Invest:
		artifact = a.buildInvestigate(ci)
	case StepF4Correlate:
		artifact = a.buildCorrelate(ci)
	case StepF5Review:
		artifact = &ReviewDecision{Decision: "approve"}
	case StepF6Report:
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

func (a *BasicAdapter) buildRecall(ci *BasicCaseInfo) *RecallResult {
	fp := ComputeFingerprint(ci.Name, ci.ErrorMessage, "")
	sym, err := a.st.GetSymptomByFingerprint(fp)
	if err != nil || sym == nil {
		return &RecallResult{
			Match: false, Confidence: 0.0,
			Reasoning: "no matching symptom in store",
		}
	}
	links, err := a.st.GetRCAsForSymptom(sym.ID)
	if err != nil || len(links) == 0 {
		return &RecallResult{
			Match: true, SymptomID: sym.ID, Confidence: 0.60,
			Reasoning: fmt.Sprintf("matched symptom %q (count=%d) but no linked RCA", sym.Name, sym.OccurrenceCount),
		}
	}
	return &RecallResult{
		Match: true, PriorRCAID: links[0].RCAID, SymptomID: sym.ID, Confidence: 0.85,
		Reasoning: fmt.Sprintf("recalled symptom %q with RCA #%d", sym.Name, links[0].RCAID),
	}
}

func (a *BasicAdapter) buildTriage(ci *BasicCaseInfo) *TriageResult {
	text := strings.ToLower(ci.Name + " " + ci.ErrorMessage + " " + ci.LogSnippet)
	category, hypothesis, skip := a.classifyDefect(text)
	component := a.identifyComponent(text)

	var candidateRepos []string
	if component != "unknown" {
		candidateRepos = []string{component}
	} else {
		candidateRepos = a.repos
	}

	cascade := basicMatchCount(text, a.h.CascadeKeywords) > 0

	return &TriageResult{
		SymptomCategory:      category,
		Severity:             "medium",
		DefectTypeHypothesis: hypothesis,
		CandidateRepos:       candidateRepos,
		SkipInvestigation:    skip,
		CascadeSuspected:     cascade,
	}
}

func (a *BasicAdapter) classifyDefect(text string) (category, hypothesis string, skip bool) {
	for _, group := range a.h.DefectClassification.AutomationSkip {
		for _, p := range group {
			if strings.Contains(text, p) {
				return "automation", "au001", true
			}
		}
	}
	for _, group := range a.h.DefectClassification.EnvironmentSkip {
		for _, p := range group {
			if strings.Contains(text, p) {
				return "environment", "en001", true
			}
		}
	}
	if isHTTPEventsEnvSkip(text) {
		return "environment", "en001", true
	}
	for _, group := range a.h.DefectClassification.AutomationGen {
		if basicMatchCount(text, group) > 0 {
			return "automation", "au001", true
		}
	}
	if isBareEventsMetricsPath(text) {
		return "automation", "au001", true
	}
	for _, group := range a.h.DefectClassification.Product {
		if basicMatchCount(text, group) > 0 {
			return "product", "pb001", false
		}
	}
	for _, group := range a.h.DefectClassification.Firmware {
		if basicMatchCount(text, group) > 0 {
			return "firmware", "fw001", false
		}
	}
	for _, group := range a.h.DefectClassification.Infra {
		if basicMatchCount(text, group) > 0 {
			return "infra", "ti001", true
		}
	}
	if strings.Contains(text, "/var/lib/jenkins") || strings.Contains(text, "ocpbugs-") || strings.Contains(text, "cnf-") {
		return "product", "pb001", false
	}
	return "product", "pb001", false
}

func (a *BasicAdapter) identifyComponent(text string) string {
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
	// cnf-gotests
	if strings.Contains(text, "ntpfailover-specific tests") || strings.Contains(text, "tracking issue for failures") {
		return "cnf-gotests"
	}
	// cloud-event-proxy
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
	if strings.Contains(text, "interface down") && !strings.Contains(text, "ordinary clock") {
		if strings.Contains(text, "ptp_interfaces.go") {
			return "linuxptp-daemon"
		}
		return "cloud-event-proxy"
	}
	if strings.Contains(text, "http events using consumer") && !strings.Contains(text, "losing subscription") {
		if !strings.Contains(text, "/var/lib") && !strings.Contains(text, "ptp_events") {
			return "linuxptp-daemon"
		}
		if !strings.Contains(text, "phc2sys") && !strings.Contains(text, "ptp4l") {
			return "cloud-event-proxy"
		}
	}
	// linuxptp-daemon
	if strings.Contains(text, "phc2sys") || strings.Contains(text, "ptp4l") {
		return "linuxptp-daemon"
	}
	if strings.Contains(text, "clock state") && strings.Contains(text, "locked") {
		return "linuxptp-daemon"
	}
	if strings.Contains(text, "offset threshold") {
		return "linuxptp-daemon"
	}
	if strings.Contains(text, "ptp_recovery.go") || strings.Contains(text, "ptp_events_and_metrics.go") || strings.Contains(text, "ptp_interfaces.go") {
		return "linuxptp-daemon"
	}
	if strings.Contains(text, "workload partitioning") || strings.Contains(text, "workloadpartitioning") {
		if strings.Contains(text, "workload_partitioning.go") {
			return "cloud-event-proxy"
		}
		return "linuxptp-daemon"
	}
	if strings.Contains(text, "ocpbugs-54967") {
		return "linuxptp-daemon"
	}
	ptpKW := []string{"ptp", "linuxptp", "clock", "gnss", "offset"}
	if basicMatchCount(text, ptpKW) > 0 {
		return "linuxptp-daemon"
	}
	return "unknown"
}

func (a *BasicAdapter) buildResolve(ci *BasicCaseInfo) *ResolveResult {
	text := strings.ToLower(ci.Name + " " + ci.ErrorMessage + " " + ci.LogSnippet)
	component := a.identifyComponent(text)
	var repos []RepoSelection
	if component != "unknown" {
		repos = append(repos, RepoSelection{Name: component, Reason: fmt.Sprintf("keyword-identified component: %s", component)})
	} else {
		for _, name := range a.repos {
			repos = append(repos, RepoSelection{Name: name, Reason: "included from workspace (no component identified)"})
		}
	}
	return &ResolveResult{SelectedRepos: repos}
}

func (a *BasicAdapter) buildInvestigate(ci *BasicCaseInfo) *InvestigateArtifact {
	text := strings.ToLower(ci.Name + " " + ci.ErrorMessage + " " + ci.LogSnippet)
	component := a.identifyComponent(text)
	_, defectType, _ := a.classifyDefect(text)
	evidenceRefs := extractEvidenceRefs(ci.ErrorMessage, component)

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

	convergence := a.computeConvergence(text, component)
	gapBrief := a.buildGapBrief(ci, text, component, defectType, convergence)

	return &InvestigateArtifact{
		RCAMessage:       rcaMessage,
		DefectType:       defectType,
		Component:        component,
		ConvergenceScore: convergence,
		EvidenceRefs:     evidenceRefs,
		GapBrief:         gapBrief,
	}
}

func (a *BasicAdapter) buildGapBrief(ci *BasicCaseInfo, text, component, defectType string, convergence float64) *GapBrief {
	verdict := ClassifyVerdict(convergence, defectType, DefaultGapConfidentThreshold, DefaultGapInconclusiveThreshold)
	var gaps []EvidenceGap

	if len(ci.ErrorMessage)+len(ci.LogSnippet) < 200 {
		gaps = append(gaps, EvidenceGap{Category: GapLogDepth, Description: "Only a short error message is available; no full logs or stack trace", WouldHelp: "Full pod logs from the failure window would show the actual error chain", Source: "CI job console log"})
	}
	if !jiraIDPattern.MatchString(text) {
		gaps = append(gaps, EvidenceGap{Category: GapJiraContext, Description: "No Jira ticket references found in the failure data", WouldHelp: "Linked Jira ticket description would confirm or deny the hypothesis", Source: "Jira / issue tracker"})
	}
	if component == "unknown" {
		gaps = append(gaps, EvidenceGap{Category: GapSourceCode, Description: "Could not identify the affected component from available data", WouldHelp: "Source code inspection would confirm the suspected regression", Source: "Git repository"})
	}
	if basicMatchCount(text, a.h.Convergence.VersionKW) == 0 {
		gaps = append(gaps, EvidenceGap{Category: GapVersionInfo, Description: "No OCP/operator version information found in the failure data", WouldHelp: "Matching against known bugs for the specific version would narrow candidates", Source: "RP launch attributes"})
	}

	if verdict == VerdictConfident && len(gaps) == 0 {
		return nil
	}
	return &GapBrief{Verdict: verdict, GapItems: gaps}
}

func (a *BasicAdapter) buildCorrelate(ci *BasicCaseInfo) *CorrelateResult {
	rcas, err := a.st.ListRCAs()
	if err != nil || len(rcas) == 0 {
		return &CorrelateResult{IsDuplicate: false, Confidence: 0.0}
	}
	text := strings.ToLower(ci.ErrorMessage)
	if text == "" {
		return &CorrelateResult{IsDuplicate: false, Confidence: 0.0}
	}
	for _, existing := range rcas {
		if existing.Description == "" {
			continue
		}
		rcaText := strings.ToLower(existing.Description)
		if strings.Contains(rcaText, text) || strings.Contains(text, rcaText) {
			return &CorrelateResult{
				IsDuplicate: true, LinkedRCAID: existing.ID, Confidence: 0.75,
				Reasoning: fmt.Sprintf("matched existing RCA #%d: %s", existing.ID, existing.Title),
			}
		}
	}
	return &CorrelateResult{IsDuplicate: false, Confidence: 0.0}
}

func basicMatchCount(text string, keywords []string) int {
	count := 0
	for _, kw := range keywords {
		if strings.Contains(text, kw) {
			count++
		}
	}
	return count
}

func isHTTPEventsEnvSkip(text string) bool {
	if !strings.Contains(text, "http events using consumer") || !strings.Contains(text, "/var/lib/jenkins") {
		return false
	}
	noJira := !strings.Contains(text, "ocpbugs-") && !regexp.MustCompile(`cnf-\d`).MatchString(text)
	noGinkgo := !strings.Contains(text, "ran ") || !strings.Contains(text, " specs ")
	return noJira && noGinkgo && !strings.Contains(text, "losing subscription")
}

func isBareEventsMetricsPath(text string) bool {
	stripped := strings.TrimSpace(text)
	if stripped == "" || !strings.Contains(stripped, "ptp_events_and_metrics.go") {
		return false
	}
	return len(strings.Fields(stripped)) <= 2
}

func (a *BasicAdapter) computeConvergence(text, component string) float64 {
	if component == "unknown" {
		return 0.70
	}
	score := 0.70
	if basicMatchCount(text, a.h.Convergence.JiraKW) > 0 {
		score += 0.10
	}
	matches := basicMatchCount(text, a.h.Convergence.DescriptiveKW)
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

var jiraIDPattern = regexp.MustCompile(`(?i)(OCPBUGS-\d+|CNF-\d+)`)

func extractEvidenceRefs(errorMessage string, component string) []string {
	var refs []string
	seen := make(map[string]bool)
	if component != "" && component != "unknown" {
		ref := component + ":relevant_source_file"
		refs = append(refs, ref)
		seen[ref] = true
	}
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
