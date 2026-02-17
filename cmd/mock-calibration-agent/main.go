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

func dbg(format string, args ...any) {
	if debug {
		log.Printf("[debug] "+format, args...)
	}
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
		return lower // fallback: whole prompt (shouldn't happen)
	}
	return strings.Join(sections, "\n")
}

func produceArtifact(step, caseID, prompt string) map[string]any {
	failureData := extractFailureData(prompt)
	dbg("extracted failure data (%d bytes from %d byte prompt)", len(failureData), len(prompt))

	switch step {
	case "F0_RECALL":
		return produceRecall(caseID, failureData)
	case "F1_TRIAGE":
		return produceTriage(caseID, failureData)
	case "F2_RESOLVE":
		return produceResolve(failureData)
	case "F3_INVESTIGATE":
		return produceInvestigate(failureData)
	case "F4_CORRELATE":
		return produceCorrelate(caseID, failureData)
	case "F5_REVIEW":
		return produceReview()
	case "F6_REPORT":
		return produceReport(caseID, failureData)
	default:
		return map[string]any{"error": "unknown step"}
	}
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

	return map[string]any{
		"symptom_category":       cat,
		"severity":               severity,
		"defect_type_hypothesis": defect,
		"candidate_repos":        repos,
		"skip_investigation":     skip,
		"cascade_suspected":      false,
	}
}

// classifyFailure determines the defect category from failure data.
func classifyFailure(prompt string) (category, severity, defectType string, skip bool) {
	// Environment/infra indicators
	envKeywords := []string{
		"deployment fail", "deploy fail", "failed to deploy",
		"node not ready", "machine not ready", "cluster not available",
		"network unreachable", "connection refused", "timeout waiting for cluster",
		"interface going up unexpectedly", "assisted install",
		"configuration issue", "environment issue",
	}
	for _, kw := range envKeywords {
		if strings.Contains(prompt, kw) {
			return "environment", "medium", "en001", true
		}
	}

	// Automation/flake indicators
	autoKeywords := []string{
		"automation issue", "automation bug", "qe bug",
		"test framework", "as designed",
		"flaky", "intermittent", "flake", "timing issue",
	}
	for _, kw := range autoKeywords {
		if strings.Contains(prompt, kw) {
			return "automation", "low", "au001", true
		}
	}

	// Firmware indicators
	fwKeywords := []string{
		"firmware", "clock not locking", "gnss module",
		"ice driver", "hardware clock",
	}
	for _, kw := range fwKeywords {
		if strings.Contains(prompt, kw) {
			return "product", "high", "fw001", false
		}
	}

	// Default: product bug
	severity = "critical"
	if strings.Contains(prompt, "warning") || strings.Contains(prompt, "minor") {
		severity = "medium"
	}
	return "product", severity, "pb001", false
}

// identifyComponent determines the primary component from the prompt.
func identifyComponent(prompt string) string {
	// Cloud event proxy indicators
	cepKeywords := []string{
		"cloud-event-proxy", "cloud event proxy", "events.sock",
		"event socket", "cloud native event",
	}
	for _, kw := range cepKeywords {
		if strings.Contains(prompt, kw) {
			return "cloud-event-proxy"
		}
	}

	// PTP operator indicators
	opKeywords := []string{
		"ptp-operator", "ptpconfig crd", "ptp operator",
		"operator lifecycle", "daemonset",
	}
	for _, kw := range opKeywords {
		if strings.Contains(prompt, kw) {
			return "ptp-operator"
		}
	}

	// Test framework indicators
	testKeywords := []string{
		"cnf-gotests", "ginkgo", "beforesuite", "aftersuite",
		"test framework", "test helper",
	}
	for _, kw := range testKeywords {
		if strings.Contains(prompt, kw) {
			return "cnf-gotests"
		}
	}

	// Default: linuxptp-daemon is the most common component for PTP
	return "linuxptp-daemon"
}

func produceResolve(prompt string) map[string]any {
	component := identifyComponent(prompt)
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

	// Build an RCA message from the error content
	rca := buildRCAMessage(prompt, component)

	return map[string]any{
		"launch_id":         "",
		"case_ids":          []int{},
		"rca_message":       rca,
		"defect_type":       defectType,
		"component":         component,
		"convergence_score": 0.80,
		"evidence_refs":     []string{component + ":relevant_source_file"},
	}
}

// buildRCAMessage constructs a descriptive RCA from the failure data.
func buildRCAMessage(prompt string, component string) string {
	// Extract key phrases from the failure
	var findings []string

	phrases := map[string]string{
		"phc2sys":           "phc2sys process failure",
		"ptp4l":             "ptp4l synchronization issue",
		"holdover":          "PTP holdover state transition failure",
		"freerun":           "PTP entered freerun state unexpectedly",
		"recovery":          "PTP process recovery mechanism failure",
		"restart":           "PTP daemon restart issue",
		"config change":     "configuration change handling failure",
		"broken pipe":       "event socket broken pipe",
		"events.sock":       "cloud event proxy socket communication failure",
		"gnss":              "GNSS sync state mapping issue",
		"clock class":       "clock class not reported correctly",
		"sync state":        "PTP sync state transition failure",
		"not in sync":       "PTP not achieving sync state",
		"metrics":           "PTP metrics reporting failure",
		"beforesuite":       "test setup failure in BeforeSuite",
		"beforeeach":        "test setup failure in BeforeEach",
		"interface":         "network interface state issue",
		"grandmaster":       "PTP grandmaster configuration issue",
		"boundary clock":    "boundary clock configuration issue",
		"ordinary clock":    "ordinary clock configuration issue",
		"2-port":            "dual-port ordinary clock issue",
		"consumer":          "event consumer lifecycle issue",
		"ntp":               "NTP failover mechanism issue",
		"process restart":   "PTP process restart failure",
		"daemon":            "linuxptp daemon operational failure",
		"timeout":           "operation timed out",
		"expected":          "assertion failure - expected state not reached",
	}

	for kw, desc := range phrases {
		if strings.Contains(prompt, kw) {
			findings = append(findings, desc)
		}
	}

	if len(findings) == 0 {
		return fmt.Sprintf("Root cause in %s requires investigation. Failure evidence suggests a defect in the PTP subsystem.", component)
	}

	// Build summary from top findings
	if len(findings) > 4 {
		findings = findings[:4]
	}
	return fmt.Sprintf("Investigation of %s identified: %s. The failure originates in the %s component.",
		component, strings.Join(findings, "; "), component)
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
