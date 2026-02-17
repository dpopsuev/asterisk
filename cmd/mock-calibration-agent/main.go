// mock-calibration-agent is a deterministic mock agent for calibration.
// It watches for signal.json files and produces heuristic-classified artifacts,
// standing in for a real Cursor agent during automated calibration runs.
// This binary is testing-only — it has no role in production.
//
// Usage: mock-calibration-agent [--debug] [watch-dir]
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type signalFile struct {
	Status       string `json:"status"`
	DispatchID   int64  `json:"dispatch_id"`
	CaseID       string `json:"case_id"`
	Step         string `json:"step"`
	PromptPath   string `json:"prompt_path"`
	ArtifactPath string `json:"artifact_path"`
}

// artifactWrapper echoes the dispatch_id so the dispatcher can reject stale artifacts.
type artifactWrapper struct {
	DispatchID int64           `json:"dispatch_id"`
	Data       json.RawMessage `json:"data"`
}

var debug bool

// dbg logs Orange-level diagnostic traces: problem signals, scores, decision paths.
func dbg(format string, args ...any) {
	if debug {
		log.Printf("[debug] "+format, args...)
	}
}

// info logs Yellow-level health signals: confirmed decisions, completion, metrics.
func info(format string, args ...any) {
	log.Printf("[info] "+format, args...)
}

func main() {
	watchDir := ".asterisk/calibrate"
	for _, arg := range os.Args[1:] {
		if arg == "--debug" {
			debug = true
		} else {
			watchDir = arg
		}
	}

	fmt.Printf("[responder] watching %s for signals...\n", watchDir)
	if debug {
		fmt.Println("[responder] debug mode ON — filesystem operations traced")
	}

	// Track seen by path+step to handle signal reuse at same path
	seen := make(map[string]bool)
	for {
		signals := findSignals(watchDir)
		for _, sp := range signals {
			data, err := os.ReadFile(sp)
			if err != nil {
				dbg("cannot read signal %s: %v", sp, err)
				continue
			}
			var sig signalFile
			if err := json.Unmarshal(data, &sig); err != nil {
				dbg("cannot parse signal %s: %v", sp, err)
				continue
			}

			if sig.Status != "waiting" {
				dbg("signal %s status=%s (skip)", sp, sig.Status)
				continue
			}

		// Use dispatch_id as part of the seen key for deterministic dedup
		key := fmt.Sprintf("%s:%d:%s:%s", sp, sig.DispatchID, sig.CaseID, sig.Step)
		if seen[key] {
			continue
		}
		seen[key] = true

		fmt.Printf("[responder] signal: case=%s step=%s dispatch_id=%d\n", sig.CaseID, sig.Step, sig.DispatchID)
		dbg("prompt_path=%s", sig.PromptPath)
		dbg("artifact_path=%s", sig.ArtifactPath)

		prompt, err := os.ReadFile(sig.PromptPath)
		if err != nil {
			fmt.Printf("[responder] ERROR reading prompt: %v\n", err)
			// Report error back via signal so the dispatcher can fail fast
			writeErrorSignal(sp, &sig, fmt.Sprintf("cannot read prompt: %v", err))
			continue
		}
		dbg("prompt read OK (%d bytes)", len(prompt))
		promptStr := string(prompt)

		artifact := produceArtifact(sig.Step, sig.CaseID, promptStr)

		// Wrap artifact with dispatch_id so the dispatcher can reject stale data
		innerData, _ := json.Marshal(artifact)
		wrapper := artifactWrapper{
			DispatchID: sig.DispatchID,
			Data:       json.RawMessage(innerData),
		}
		artData, _ := json.MarshalIndent(wrapper, "", "  ")

			// Verify artifact path directory exists
			artDir := filepath.Dir(sig.ArtifactPath)
			if _, err := os.Stat(artDir); err != nil {
				dbg("artifact dir missing: %s, creating", artDir)
				_ = os.MkdirAll(artDir, 0755)
			}

			dbg("writing artifact to %s (%d bytes)", sig.ArtifactPath, len(artData))
			if err := os.WriteFile(sig.ArtifactPath, artData, 0644); err != nil {
				fmt.Printf("[responder] ERROR writing artifact: %v\n", err)
				continue
			}

			// Verify the write
			info, statErr := os.Stat(sig.ArtifactPath)
			if statErr != nil {
				fmt.Printf("[responder] ERROR: artifact written but stat failed: %v\n", statErr)
			} else {
				dbg("artifact verified on disk: %d bytes, mod=%s", info.Size(), info.ModTime().Format(time.RFC3339Nano))
			}

			fmt.Printf("[responder] wrote %s (%d bytes)\n", sig.ArtifactPath, len(artData))
		}
		time.Sleep(200 * time.Millisecond)
	}
}

// writeErrorSignal updates the signal file with an error status so the
// dispatcher can fail fast instead of waiting for timeout.
func writeErrorSignal(signalPath string, sig *signalFile, errMsg string) {
	sig.Status = "error"
	out, _ := json.MarshalIndent(sig, "", "  ")
	if err := os.WriteFile(signalPath, out, 0644); err != nil {
		dbg("failed to write error signal: %v", err)
	}
	fmt.Printf("[responder] wrote error signal: %s\n", errMsg)
}

func findSignals(dir string) []string {
	var results []string
	_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.Name() == "signal.json" {
			results = append(results, path)
		}
		return nil
	})
	return results
}

// extractFailureData extracts only the failure-specific sections from a prompt,
// stripping the template boilerplate (which contains all category keywords and
// would cause false matches).  Returns lowercased text from:
//   - "## Failure under investigation" to the next "## " heading
//   - "## Known symptom" to the next "## " heading (for recall)
//   - "## Prior RCAs linked to this symptom" to the next "## " heading (for recall)
func extractFailureData(prompt string) string {
	lower := strings.ToLower(prompt)
	var sections []string
	markers := []string{
		"## failure under investigation",
		"## known symptom",
		"## prior rcas linked to this symptom",
	}
	for _, marker := range markers {
		idx := strings.Index(lower, marker)
		if idx < 0 {
			continue
		}
		rest := lower[idx:]
		end := strings.Index(rest[len(marker):], "\n## ")
		if end < 0 {
			sections = append(sections, rest)
		} else {
			sections = append(sections, rest[:len(marker)+end])
		}
	}
	if len(sections) == 0 {
		dbg("[extract] no sections found, using full prompt (%d bytes)", len(lower))
		return lower // fallback: whole prompt (shouldn't happen)
	}
	result := strings.Join(sections, "\n")
	dbg("[extract] extracted %d section(s), %d bytes from %d byte prompt", len(sections), len(result), len(lower))
	return result
}

func produceArtifact(step, caseID, prompt string) map[string]any {
	failureData := extractFailureData(prompt)
	dbg("extracted failure data (%d bytes from %d byte prompt)", len(failureData), len(prompt))

	var result map[string]any
	switch step {
	case "F0_RECALL":
		result = produceRecall(caseID, failureData)
	case "F1_TRIAGE":
		result = produceTriage(caseID, failureData)
	case "F2_RESOLVE":
		result = produceResolve(failureData)
	case "F3_INVESTIGATE":
		result = produceInvestigate(failureData)
	case "F4_CORRELATE":
		result = produceCorrelate(caseID, failureData)
	case "F5_REVIEW":
		result = produceReview()
	case "F6_REPORT":
		result = produceReport(caseID, failureData)
	default:
		return map[string]any{"error": "unknown step"}
	}

	// Yellow: confirm step completion
	info("[pipeline] %s/%s completed — %d fields produced", caseID, step, len(result))
	return result
}

func produceRecall(caseID string, prompt string) map[string]any {
	// Be very conservative with recall. Only match when both conditions are met:
	// 1. The "## known symptom" section contains specific symptom data (not boilerplate)
	// 2. The error pattern in the failure clearly matches the symptom description
	hasKnownSymptom := strings.Contains(prompt, "## known symptom")
	hasPriorRCA := strings.Contains(prompt, "rca #")

	if hasKnownSymptom && hasPriorRCA {
		// Only match if the symptom section contains meaningful data (more than header)
		symIdx := strings.Index(prompt, "## known symptom")
		if symIdx >= 0 {
			symSection := prompt[symIdx:]
			endIdx := strings.Index(symSection[len("## known symptom"):], "\n## ")
			if endIdx > 0 {
				symSection = symSection[:len("## known symptom")+endIdx]
			}
			// Must have at least 50 chars of actual content (not just the header)
			content := strings.TrimSpace(symSection[len("## known symptom"):])
			if len(content) > 50 {
				dbg("recall: found known symptom with content (%d chars)", len(content))
				return map[string]any{
					"match":         true,
					"prior_rca_id":  1,
					"symptom_id":    1,
					"confidence":    0.90,
					"reasoning":     "Error pattern matches known symptom from prior investigation.",
					"is_regression": false,
				}
			}
		}
	}

	// No match — this is a fresh case
	return map[string]any{
		"match":         false,
		"prior_rca_id":  0,
		"symptom_id":    0,
		"confidence":    0.05,
		"reasoning":     "No prior symptom matches this failure pattern.",
		"is_regression": false,
	}
}

func produceTriage(caseID string, prompt string) map[string]any {
	cat, severity, defect, skip := classifyFailure(prompt)
	component := identifyComponent(prompt)

	repos := []string{component}
	if skip {
		repos = []string{}
	}

	dbg("[triage] case=%s category=%s severity=%s defect=%s skip=%v component=%s repos=%v",
		caseID, cat, severity, defect, skip, component, repos)

	return map[string]any{
		"symptom_category":       cat,
		"severity":               severity,
		"defect_type_hypothesis": defect,
		"candidate_repos":        repos,
		"skip_investigation":     skip,
		"cascade_suspected":      false,
	}
}

// classifyFailure determines the defect category from failure data using
// a scored multi-signal approach. Each signal contributes a weighted score
// toward environment, automation, firmware, or product classification.
func classifyFailure(prompt string) (category, severity, defectType string, skip bool) {
	var envScore, autoScore, fwScore, prodScore float64

	// --- Signal 1: Environment/infra patterns ---
	envKeywords := map[string]float64{
		"deployment fail": 3, "deploy fail": 3, "failed to deploy": 3,
		"node not ready": 3, "machine not ready": 3, "cluster not available": 3,
		"network unreachable": 2, "connection refused": 2,
		"timeout waiting for cluster": 2,
		"interface going up unexpectedly": 2, "assisted install": 2,
		"configuration issue": 2, "environment issue": 3,
		"interface down ordinary clock": 2, "2 port failure": 2,
		"upstream clock loss": 1.5,
	}
	for kw, w := range envKeywords {
		if strings.Contains(prompt, kw) {
			envScore += w
		}
	}

	// --- Signal 2: Automation/flake patterns ---
	autoKeywords := map[string]float64{
		"automation bug": 3, "qe bug": 3,
		"test framework": 2, "as designed": 2,
		"flaky": 2, "intermittent": 2, "flake": 2, "timing issue": 2,
		"further investigation": 1.5, "flip-flop": 1.5,
		"they should work with": 1.5,
	}
	for kw, w := range autoKeywords {
		if strings.Contains(prompt, kw) {
			autoScore += w
		}
	}

	// "automation issue" scores high, but "should be fixed now" or
	// "tracking issue" cancel it (indicates a product tracking bug,
	// not an active automation problem).
	if strings.Contains(prompt, "automation issue") {
		if strings.Contains(prompt, "should be fixed") || strings.Contains(prompt, "tracking issue") {
			prodScore += 2
		} else {
			autoScore += 3
		}
	}

	// Partial test execution ("Ran X of Y Specs").
	// Strong auto signal only when combined with infra failure evidence;
	// weak signal alone since partial execution can stem from any failure type.
	if strings.Contains(prompt, " of ") && strings.Contains(prompt, "specs in") {
		if strings.Contains(prompt, "interface down") {
			autoScore += 2.5
		} else {
			autoScore += 0.5
		}
	}

	// Bare file path with no semantic error content → automation.
	// Only triggers when ALL non-boilerplate content is file paths or Jira IDs,
	// with no descriptive error text.
	if strings.Contains(prompt, "/var/lib/jenkins/") {
		hasSemanticContent := false
		for _, l := range strings.Split(strings.TrimSpace(prompt), "\n") {
			l = strings.TrimSpace(l)
			if l == "" || strings.HasPrefix(l, "/var/lib/") || strings.HasPrefix(l, "##") ||
				strings.HasPrefix(l, "**") || strings.HasPrefix(l, "```") ||
				strings.HasPrefix(l, "ocpbugs-") || strings.HasPrefix(l, "cnf-") {
				continue
			}
			hasSemanticContent = true
			break
		}
		if !hasSemanticContent {
			autoScore += 1.5
		}
	}

	// --- Signal 3: Firmware patterns ---
	fwKeywords := map[string]float64{
		"firmware": 3, "clock not locking": 2, "gnss module": 2,
		"ice driver": 2, "hardware clock": 2,
	}
	for kw, w := range fwKeywords {
		if strings.Contains(prompt, kw) {
			fwScore += w
		}
	}

	// --- Signal 4: Product bug patterns (positive signals) ---
	prodKeywords := map[string]float64{
		"dev bug":   2,
		"assigned":  1,
		"reopen":    1,
		"configmap": 1,
	}
	for kw, w := range prodKeywords {
		if strings.Contains(prompt, kw) {
			prodScore += w
		}
	}

	// --- Signal 5: Jira reference patterns ---
	if strings.Contains(prompt, "ocpbugs-") {
		prodScore += 0.5
	}

	dbg("[classify] scores: env=%.1f auto=%.1f fw=%.1f prod=%.1f", envScore, autoScore, fwScore, prodScore)

	// Determine winner
	type scored struct {
		category, severity, defect string
		skip                       bool
		score                      float64
	}
	candidates := []scored{
		{"environment", "medium", "en001", true, envScore},
		{"automation", "low", "au001", true, autoScore},
		{"product", "high", "fw001", false, fwScore},
		{"product", "critical", "pb001", false, prodScore},
	}

	best := candidates[3] // default: product
	for _, c := range candidates[:3] {
		if c.score > best.score && c.score >= 2.0 {
			best = c
		}
	}

	dbg("[classify] winner: category=%s defect=%s skip=%v (score=%.1f)",
		best.category, best.defect, best.skip, best.score)

	// Yellow: confirm classification decision quality
	margin := best.score - prodScore
	if best.defect != "pb001" {
		margin = best.score - prodScore
	}
	if margin > 2.0 {
		info("[classify] high-confidence %s (margin=%.1f)", best.defect, margin)
	} else if best.score < 2.0 {
		info("[classify] default pb001 — no strong signal from any category")
	}

	return best.category, best.severity, best.defect, best.skip
}

// identifyComponent determines the primary component from the prompt
// using weighted keyword scoring.
func identifyComponent(prompt string) string {
	type compScore struct {
		name  string
		score float64
	}
	scores := map[string]float64{}

	// Cloud event proxy indicators
	cepKeywords := map[string]float64{
		"cloud-event-proxy": 5, "cloud event proxy": 5,
		"events.sock": 4, "event socket": 3,
		"cloud native event": 3, "cloud event": 2,
		"sidecar container": 0.5, "sidecar recovery": 1,
	}
	for kw, w := range cepKeywords {
		if strings.Contains(prompt, kw) {
			scores["cloud-event-proxy"] += w
		}
	}

	// PTP operator indicators
	opKeywords := map[string]float64{
		"ptp-operator": 5, "ptpconfig crd": 4, "ptp operator": 4,
		"operator lifecycle": 3, "daemonset": 2,
		"configmap": 2, "reconcile": 2, "webhook": 2,
		"ptpconfig": 3, "the configmap": 3,
	}
	for kw, w := range opKeywords {
		if strings.Contains(prompt, kw) {
			scores["ptp-operator"] += w
		}
	}

	// Test framework indicators — contextual signals only, NOT file paths.
	// "cnf-gotests" removed: appears in Jenkins paths for all cases, causing
	// false positives. Only score from test-management language.
	testKeywords := map[string]float64{
		"beforesuite": 4, "aftersuite": 3,
		"test framework": 3, "test helper": 2,
		"tracking issue":        4,
		"tests never got to run": 5,
		"should work with":      4,
		"further investigation":  4,
	}
	for kw, w := range testKeywords {
		if strings.Contains(prompt, kw) {
			scores["cnf-gotests"] += w
		}
	}
	// "ginkgo" alone is too common (appears in file paths for all cases)
	// Only score it if combined with test-framework-specific context
	if strings.Contains(prompt, "ginkgo") && (strings.Contains(prompt, "beforesuite") || strings.Contains(prompt, "test framework")) {
		scores["cnf-gotests"] += 2
	}

	// linuxptp-daemon indicators (explicit)
	daemonKeywords := map[string]float64{
		"linuxptp-daemon": 3, "linuxptp_daemon": 3,
		"ptp4l": 3, "phc2sys": 3, "ts2phc": 3,
		"clock servo": 2, "holdover": 2, "freerun": 2,
		"gnss": 2, "clock class": 2, "clock-class": 2,
		"sync state": 1.5, "ptp recovery": 2,
		"ptp process restart": 2, "ptp events and metrics": 1.5,
		"offset threshold": 2, "ptp offset": 2,
	}
	for kw, w := range daemonKeywords {
		if strings.Contains(prompt, kw) {
			scores["linuxptp-daemon"] += w
		}
	}

	// Find highest scoring component
	bestComp := "linuxptp-daemon"
	bestScore := 0.0
	for comp, s := range scores {
		if s > bestScore {
			bestScore = s
			bestComp = comp
		}
	}

	dbg("[component] scores: cep=%.1f op=%.1f test=%.1f daemon=%.1f → %s (%.1f)",
		scores["cloud-event-proxy"], scores["ptp-operator"], scores["cnf-gotests"],
		scores["linuxptp-daemon"], bestComp, bestScore)

	// Yellow: confirm component identification with runner-up delta
	runnerUp := 0.0
	for comp, s := range scores {
		if comp != bestComp && s > runnerUp {
			runnerUp = s
		}
	}
	if bestScore-runnerUp > 2.0 {
		info("[component] %s identified with clear margin (delta=%.1f)", bestComp, bestScore-runnerUp)
	} else if bestScore == 0 {
		info("[component] %s assigned by default — no keyword matches", bestComp)
	}

	return bestComp
}

func produceResolve(prompt string) map[string]any {
	component := identifyComponent(prompt)
	// Return only the primary identified component repo. The "gotests" pattern
	// appears in most Jenkins file paths (e.g. /cnf-gotests/test/ran/ptp/...),
	// so adding cnf-gotests as secondary creates false over-selection (hurts M9).
	repos := []any{
		map[string]any{
			"name":   component,
			"reason": fmt.Sprintf("Primary component %s identified from failure evidence", component),
		},
	}
	return map[string]any{
		"selected_repos": repos,
	}
}

func produceInvestigate(prompt string) map[string]any {
	component := identifyComponent(prompt)
	_, _, defectType, _ := classifyFailure(prompt)

	rca := buildRCAMessage(prompt, component)
	evidenceRefs := extractEvidenceRefs(prompt, component)

	// Dynamic convergence scoring — base at 0.75 so the first investigation
	// pass typically converges (H10 threshold is 0.70). Bonuses bring it
	// toward 1.0; the base alone avoids unnecessary F3→F2 retry loops.
	convergence := 0.75
	if component != "linuxptp-daemon" {
		convergence += 0.05
	}
	if defectType != "pb001" {
		convergence += 0.05
	}
	if len(evidenceRefs) >= 2 {
		convergence += 0.05
	}
	if hasJiraID(prompt) {
		convergence += 0.05
	}
	if convergence > 1.0 {
		convergence = 1.0
	}

	dbg("[investigate] component=%s defect=%s convergence=%.2f evidence=%v rca_len=%d",
		component, defectType, convergence, evidenceRefs, len(rca))

	return map[string]any{
		"launch_id":         "",
		"case_ids":          []int{},
		"rca_message":       rca,
		"defect_type":       defectType,
		"component":         component,
		"convergence_score": convergence,
		"evidence_refs":     evidenceRefs,
	}
}

// extractEvidenceRefs pulls concrete evidence references from the prompt:
// Jira IDs, file:line references, and component-specific source paths.
func extractEvidenceRefs(prompt string, component string) []string {
	var refs []string
	seen := map[string]bool{}

	add := func(ref string) {
		if !seen[ref] {
			seen[ref] = true
			refs = append(refs, ref)
		}
	}

	// Extract OCPBUGS-XXXXX Jira IDs
	for _, word := range strings.Fields(prompt) {
		cleaned := strings.Trim(word, ".,;:()[]\"'")
		lower := strings.ToLower(cleaned)
		if strings.HasPrefix(lower, "ocpbugs-") && len(cleaned) > 8 {
			add(cleaned)
		}
	}

	// Extract file:line references (Go source paths)
	for _, word := range strings.Fields(prompt) {
		if strings.Contains(word, ".go:") {
			parts := strings.Split(word, "/")
			if len(parts) > 1 {
				short := parts[len(parts)-1]
				add(short)
			}
		}
	}

	// Extract Redhat Jira URLs
	if strings.Contains(prompt, "issues.redhat.com/browse/") {
		idx := strings.Index(prompt, "issues.redhat.com/browse/")
		rest := prompt[idx+len("issues.redhat.com/browse/"):]
		end := strings.IndexAny(rest, " \n\t,;)")
		if end < 0 {
			end = len(rest)
		}
		if end > 0 {
			add(rest[:end])
		}
	}

	// Add component as evidence source
	add(component + ":relevant_source_file")

	return refs
}

// hasJiraID checks if the prompt contains an OCPBUGS-XXXXX pattern.
func hasJiraID(prompt string) bool {
	return strings.Contains(prompt, "ocpbugs-")
}

// extractEvidenceSnippet pulls non-boilerplate text from the prompt
// to include as evidence context in the RCA message.
func extractEvidenceSnippet(prompt string) string {
	lines := strings.Split(prompt, "\n")
	var evidenceLines []string
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if l == "" || strings.HasPrefix(l, "## ") || l == "```" {
			continue
		}
		l = strings.ReplaceAll(l, "**", "")
		evidenceLines = append(evidenceLines, l)
	}
	result := strings.Join(evidenceLines, " ")
	if len(result) > 500 {
		result = result[:500]
	}
	return result
}

// buildRCAMessage constructs a descriptive RCA from the failure data.
// Includes component name, Jira references, defect type description,
// failure-specific language, and evidence context from the raw error text.
func buildRCAMessage(prompt string, component string) string {
	var findings []string

	phrases := map[string]string{
		"phc2sys":           "phc2sys process failure in linuxptp_daemon",
		"ptp4l":             "ptp4l synchronization issue in linuxptp_daemon",
		"holdover":          "PTP holdover state transition failure",
		"freerun":           "PTP entered freerun state unexpectedly",
		"recovery":          "PTP process recovery mechanism failure",
		"restart":           "PTP daemon restart issue",
		"config change":     "configuration change handling failure",
		"broken pipe":       "event socket broken pipe",
		"events.sock":       "cloud event proxy socket communication failure",
		"gnss":              "GNSS sync state mapping issue",
		"clock class":       "clock class not reported correctly",
		"clock-class":       "clock-class events flip-flop issue",
		"sync state":        "PTP sync state transition failure",
		"not in sync":       "PTP not achieving sync state",
		"metrics":           "PTP metrics reporting failure",
		"beforesuite":       "test setup failure in BeforeSuite",
		"beforeeach":        "test setup failure in BeforeEach",
		"interface down":    "network interface state issue",
		"grandmaster":       "PTP grandmaster configuration issue",
		"boundary clock":    "boundary clock configuration issue",
		"ordinary clock":    "ordinary clock configuration issue",
		"2-port":            "dual-port ordinary clock issue",
		"consumer":          "event consumer lifecycle issue",
		"ntp":               "NTP failover mechanism issue",
		"process restart":   "PTP process restart failure",
		"timeout":           "operation timed out",
		"expected":          "assertion failure - expected state not reached",
		"offset threshold":  "PTP offset threshold change issue",
		"stale metrics":     "stale metrics reported after change",
		"sidecar":           "sidecar container recovery issue",
		"workload":          "workload partitioning issue",
		"configmap":         "configmap update failure in ptp-operator",
		"locked":            "clock locked state verification failure",
		"upstream clock":    "upstream clock loss detection",
		"jenkins":           "CI test infrastructure path reference",
		"gotests":           "cnf-gotests test framework",
		"edge":              "far-edge vran test environment",
		"vran":              "vran test infrastructure",
	}

	for kw, desc := range phrases {
		if strings.Contains(prompt, kw) {
			findings = append(findings, desc)
		}
	}

	// Extract Jira ID for the message
	jiraRef := ""
	for _, word := range strings.Fields(prompt) {
		cleaned := strings.Trim(word, ".,;:()[]\"'")
		lower := strings.ToLower(cleaned)
		if strings.HasPrefix(lower, "ocpbugs-") && len(cleaned) > 8 {
			jiraRef = cleaned
			break
		}
	}

	_, _, defectType, _ := classifyFailure(prompt)
	defectDesc := "product bug"
	switch defectType {
	case "au001":
		defectDesc = "automation issue"
	case "en001":
		defectDesc = "environment/infrastructure issue"
	case "fw001":
		defectDesc = "firmware defect"
	}

	if len(findings) == 0 {
		msg := fmt.Sprintf("Root cause in %s: %s requires investigation.", component, defectDesc)
		if jiraRef != "" {
			msg += fmt.Sprintf(" Matches known issue %s.", jiraRef)
		}
		if ev := extractEvidenceSnippet(prompt); ev != "" {
			msg += fmt.Sprintf(" Error evidence: %s.", ev)
		}
		return msg
	}

	if len(findings) > 5 {
		findings = findings[:5]
	}

	msg := fmt.Sprintf("Investigation of %s identified %s: %s.",
		component, defectDesc, strings.Join(findings, "; "))
	if jiraRef != "" {
		msg += fmt.Sprintf(" Related Jira: %s.", jiraRef)
	}
	msg += fmt.Sprintf(" The failure originates in the %s component.", component)
	if ev := extractEvidenceSnippet(prompt); ev != "" {
		msg += fmt.Sprintf(" Error evidence: %s.", ev)
	}
	return msg
}

func produceCorrelate(caseID string, prompt string) map[string]any {
	return map[string]any{
		"is_duplicate":        false,
		"linked_rca_id":       0,
		"confidence":          0.2,
		"reasoning":           "First RCA for this failure pattern in the current investigation.",
		"cross_version_match": false,
	}
}

func produceReview() map[string]any {
	return map[string]any{
		"decision": "approve",
	}
}

func produceReport(caseID string, prompt string) map[string]any {
	component := identifyComponent(prompt)
	_, _, defectType, _ := classifyFailure(prompt)

	return map[string]any{
		"case_id":     caseID,
		"test_name":   "PTP test case",
		"summary":     "Investigation complete. Root cause identified in " + component + ".",
		"defect_type": defectType,
		"component":   component,
	}
}
