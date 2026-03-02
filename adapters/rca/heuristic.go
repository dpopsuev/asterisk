package rca

import (
	_ "embed"
	"fmt"
	"regexp"
	"strings"

	framework "github.com/dpopsuev/origami"
	"github.com/dpopsuev/origami/transformers"

	"asterisk/adapters/store"

	"gopkg.in/yaml.v3"
)

//go:embed heuristics.yaml
var heuristicsYAML []byte

type heuristicTransformer struct {
	st    store.Store
	repos []string
	eval  *transformers.MatchEvaluator
	conv  convergenceConfig
}

type convergenceConfig struct {
	JiraKW        []string `yaml:"jira_keywords"`
	DescriptiveKW []string `yaml:"descriptive_keywords"`
	VersionKW     []string `yaml:"version_keywords"`
}

func NewHeuristicTransformer(st store.Store, repos []string) *heuristicTransformer {
	eval, err := transformers.NewMatchEvaluator(heuristicsYAML)
	if err != nil {
		panic(fmt.Sprintf("load heuristics.yaml: %v", err))
	}
	conv := loadConvergenceConfig(heuristicsYAML)
	return &heuristicTransformer{st: st, repos: repos, eval: eval, conv: conv}
}

type failureInfo struct {
	name         string
	errorMessage string
	logSnippet   string
}

func failureFromContext(ws *framework.WalkerState) failureInfo {
	if ws == nil {
		return failureInfo{}
	}
	if fp, ok := ws.Context[KeyParamsFailure].(*FailureParams); ok {
		return failureInfo{
			name:         fp.TestName,
			errorMessage: fp.ErrorMessage,
			logSnippet:   fp.LogSnippet,
		}
	}
	if cd, ok := ws.Context[KeyCaseData].(*store.Case); ok {
		return failureInfo{
			name:         cd.Name,
			errorMessage: cd.ErrorMessage,
			logSnippet:   cd.LogSnippet,
		}
	}
	return failureInfo{}
}

func (t *heuristicTransformer) textFromFailure(fp failureInfo) string {
	return strings.ToLower(fp.name + " " + fp.errorMessage + " " + fp.logSnippet)
}

// classifyDefect uses the match evaluator against the defect_classification rule set.
func (t *heuristicTransformer) classifyDefect(text string) (category, hypothesis string, skip bool) {
	result, _, err := t.eval.Evaluate("defect_classification", text)
	if err != nil {
		return "product", "pb001", false
	}
	return parseClassification(result)
}

func parseClassification(result any) (category, hypothesis string, skip bool) {
	m, ok := result.(map[string]any)
	if !ok {
		return "product", "pb001", false
	}
	cat, _ := m["category"].(string)
	hypo, _ := m["hypothesis"].(string)
	sk, _ := m["skip"].(bool)
	if cat == "" {
		cat = "product"
	}
	if hypo == "" {
		hypo = "pb001"
	}
	return cat, hypo, sk
}

// identifyComponent uses the match evaluator against the component_identification rule set.
func (t *heuristicTransformer) identifyComponent(text string) string {
	rs, err := t.eval.Get("component_identification")
	if err != nil {
		return "unknown"
	}
	return rs.EvaluateString(text)
}

func (t *heuristicTransformer) buildRecall(fp failureInfo) *RecallResult {
	fingerprint := ComputeFingerprint(fp.name, fp.errorMessage, "")
	sym, err := t.st.GetSymptomByFingerprint(fingerprint)
	if err != nil || sym == nil {
		return &RecallResult{
			Match: false, Confidence: 0.0,
			Reasoning: "no matching symptom in store",
		}
	}
	links, err := t.st.GetRCAsForSymptom(sym.ID)
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

func (t *heuristicTransformer) buildTriage(fp failureInfo) *TriageResult {
	text := t.textFromFailure(fp)
	category, hypothesis, skip := t.classifyDefect(text)
	component := t.identifyComponent(text)

	var candidateRepos []string
	if component != "unknown" {
		candidateRepos = []string{component}
	} else {
		candidateRepos = t.repos
	}

	cascade := matchCount(text, cascadeKeywords()) > 0

	return &TriageResult{
		SymptomCategory:      category,
		Severity:             "medium",
		DefectTypeHypothesis: hypothesis,
		CandidateRepos:       candidateRepos,
		SkipInvestigation:    skip,
		CascadeSuspected:     cascade,
	}
}

func (t *heuristicTransformer) buildResolve(fp failureInfo) *ResolveResult {
	text := t.textFromFailure(fp)
	component := t.identifyComponent(text)
	var repos []RepoSelection
	if component != "unknown" {
		repos = append(repos, RepoSelection{Name: component, Reason: fmt.Sprintf("keyword-identified component: %s", component)})
	} else {
		for _, name := range t.repos {
			repos = append(repos, RepoSelection{Name: name, Reason: "included from workspace (no component identified)"})
		}
	}
	return &ResolveResult{SelectedRepos: repos}
}

func (t *heuristicTransformer) buildInvestigate(fp failureInfo) *InvestigateArtifact {
	text := t.textFromFailure(fp)
	component := t.identifyComponent(text)
	_, defectType, _ := t.classifyDefect(text)
	evidenceRefs := extractEvidenceRefs(fp.errorMessage, component)

	rcaParts := []string{}
	if fp.errorMessage != "" {
		rcaParts = append(rcaParts, fp.errorMessage)
	}
	if fp.name != "" {
		rcaParts = append(rcaParts, fmt.Sprintf("Test: %s", fp.name))
	}
	if component != "unknown" {
		rcaParts = append(rcaParts, fmt.Sprintf("Suspected component: %s", component))
	}
	rcaMessage := strings.Join(rcaParts, " | ")
	if rcaMessage == "" {
		rcaMessage = "investigation pending (no error message available)"
	}

	convergence := t.computeConvergence(text, component)
	gapBrief := t.buildGapBrief(fp, text, component, defectType, convergence)

	return &InvestigateArtifact{
		RCAMessage:       rcaMessage,
		DefectType:       defectType,
		Component:        component,
		ConvergenceScore: convergence,
		EvidenceRefs:     evidenceRefs,
		GapBrief:         gapBrief,
	}
}

func (t *heuristicTransformer) buildGapBrief(fp failureInfo, text, component, defectType string, convergence float64) *GapBrief {
	verdict := ClassifyVerdict(convergence, defectType, DefaultGapConfidentThreshold, DefaultGapInconclusiveThreshold)
	var gaps []EvidenceGap

	if len(fp.errorMessage)+len(fp.logSnippet) < 200 {
		gaps = append(gaps, EvidenceGap{Category: GapLogDepth, Description: "Only a short error message is available; no full logs or stack trace", WouldHelp: "Full pod logs from the failure window would show the actual error chain", Source: "CI job console log"})
	}
	if !jiraIDPattern.MatchString(text) {
		gaps = append(gaps, EvidenceGap{Category: GapJiraContext, Description: "No Jira ticket references found in the failure data", WouldHelp: "Linked Jira ticket description would confirm or deny the hypothesis", Source: "Jira / issue tracker"})
	}
	if component == "unknown" {
		gaps = append(gaps, EvidenceGap{Category: GapSourceCode, Description: "Could not identify the affected component from available data", WouldHelp: "Source code inspection would confirm the suspected regression", Source: "Git repository"})
	}
	if matchCount(text, t.conv.VersionKW) == 0 {
		gaps = append(gaps, EvidenceGap{Category: GapVersionInfo, Description: "No OCP/operator version information found in the failure data", WouldHelp: "Matching against known bugs for the specific version would narrow candidates", Source: "RP launch attributes"})
	}

	if verdict == VerdictConfident && len(gaps) == 0 {
		return nil
	}
	return &GapBrief{Verdict: verdict, GapItems: gaps}
}

func (t *heuristicTransformer) buildCorrelate(fp failureInfo) *CorrelateResult {
	rcas, err := t.st.ListRCAs()
	if err != nil || len(rcas) == 0 {
		return &CorrelateResult{IsDuplicate: false, Confidence: 0.0}
	}
	text := strings.ToLower(fp.errorMessage)
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

func (t *heuristicTransformer) computeConvergence(text, component string) float64 {
	if component == "unknown" {
		return 0.70
	}
	score := 0.70
	if matchCount(text, t.conv.JiraKW) > 0 {
		score += 0.10
	}
	matches := matchCount(text, t.conv.DescriptiveKW)
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

func matchCount(text string, keywords []string) int {
	count := 0
	for _, kw := range keywords {
		if strings.Contains(text, kw) {
			count++
		}
	}
	return count
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

// cascadeKeywords returns the cascade keyword list. The evaluator's rule sets
// don't cover this because cascade detection is a count, not a classification.
func cascadeKeywords() []string {
	return []string{"aftereach", "beforeeach", "setup failure", "suite setup"}
}

// loadConvergenceConfig extracts the convergence section from the YAML.
type heuristicsConvergenceWrapper struct {
	Convergence convergenceConfig `yaml:"convergence"`
}

func loadConvergenceConfig(yamlData []byte) convergenceConfig {
	var w heuristicsConvergenceWrapper
	if err := yaml.Unmarshal(yamlData, &w); err != nil {
		return convergenceConfig{}
	}
	return w.Convergence
}
