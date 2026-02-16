// signal-responder is a simple auto-responder for wet calibration.
// It watches for signal.json files and produces minimal valid artifacts.
// This is used for the first wet calibration iteration to validate the
// FileDispatcher pipeline works end-to-end.
//
// Usage: signal-responder [--debug] [watch-dir]
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
	// The recall prompt renders "## known symptom" and "## prior rcas linked"
	// sections when history exists. If either is present, we have prior data.
	hasPriorData := strings.Contains(prompt, "## known symptom") || strings.Contains(prompt, "rca #")

	if hasPriorData && strings.Contains(prompt, "holdover") {
		return map[string]any{
			"match":         true,
			"prior_rca_id":  1,
			"symptom_id":    1,
			"confidence":    0.9,
			"reasoning":     "Error pattern matches known symptom S1 (ptp4l holdover timeout).",
			"is_regression": false,
		}
	}
	if hasPriorData && (strings.Contains(prompt, "stale") || strings.Contains(prompt, "ptpconfig")) {
		return map[string]any{
			"match":         true,
			"prior_rca_id":  2,
			"symptom_id":    2,
			"confidence":    0.85,
			"reasoning":     "Error pattern matches known symptom for stale PtpConfig CRD.",
			"is_regression": false,
		}
	}
	if hasPriorData && strings.Contains(prompt, "cleanup") {
		return map[string]any{
			"match":         true,
			"prior_rca_id":  2,
			"symptom_id":    2,
			"confidence":    0.85,
			"reasoning":     "Cleanup issue matches known symptom for stale config.",
			"is_regression": false,
		}
	}

	// No prior data or no match — fresh case
	return map[string]any{
		"match":         false,
		"prior_rca_id":  0,
		"symptom_id":    0,
		"confidence":    0.1,
		"reasoning":     "No prior symptom matches this failure pattern.",
		"is_regression": false,
	}
}

func produceTriage(caseID string, prompt string) map[string]any {
	if strings.Contains(prompt, "ntp") && strings.Contains(prompt, "sync validation") {
		return map[string]any{
			"symptom_category":       "infra",
			"severity":               "medium",
			"defect_type_hypothesis": "si001",
			"candidate_repos":        []string{},
			"skip_investigation":     true,
			"cascade_suspected":      false,
		}
	}
	if strings.Contains(prompt, "flak") || strings.Contains(prompt, "intermittent") || strings.Contains(prompt, "flaky timing") {
		return map[string]any{
			"symptom_category":       "flake",
			"severity":               "low",
			"defect_type_hypothesis": "nd001",
			"candidate_repos":        []string{},
			"skip_investigation":     true,
			"cascade_suspected":      false,
		}
	}
	if strings.Contains(prompt, "stale") || strings.Contains(prompt, "ptpconfig") || strings.Contains(prompt, "cleanup") {
		repos := []string{"linuxptp-daemon-operator"}
		return map[string]any{
			"symptom_category":       "automation",
			"severity":               "high",
			"defect_type_hypothesis": "ab001",
			"candidate_repos":        repos,
			"skip_investigation":     false,
			"cascade_suspected":      strings.Contains(prompt, "cascade") || strings.Contains(prompt, "setup"),
		}
	}
	if strings.Contains(prompt, "holdover") || strings.Contains(prompt, "freerun") || strings.Contains(prompt, "ptp4l") {
		repos := []string{"linuxptp-daemon-operator"}
		return map[string]any{
			"symptom_category":       "product",
			"severity":               "critical",
			"defect_type_hypothesis": "pb001",
			"candidate_repos":        repos,
			"skip_investigation":     false,
			"cascade_suspected":      false,
		}
	}
	return map[string]any{
		"symptom_category":       "product",
		"severity":               "medium",
		"defect_type_hypothesis": "pb001",
		"candidate_repos":        []string{"linuxptp-daemon-operator"},
		"skip_investigation":     false,
		"cascade_suspected":      false,
	}
}

func produceResolve(prompt string) map[string]any {
	repos := []any{
		map[string]any{
			"name":   "linuxptp-daemon-operator",
			"reason": "Primary codebase for PTP daemon configuration and management",
		},
	}
	return map[string]any{
		"selected_repos": repos,
	}
}

func produceInvestigate(prompt string) map[string]any {
	defectType := "pb001"
	component := "linuxptp-daemon"
	rca := "Root cause requires further investigation based on failure evidence in the prompt."

	if strings.Contains(prompt, "holdover") {
		rca = "Holdover timeout was reduced from 300s to 60s in linuxptp-daemon configuration, causing PTP sync failure."
	}
	if strings.Contains(prompt, "stale") || strings.Contains(prompt, "ptpconfig") {
		defectType = "ab001"
		component = "ptp-test-suite"
		rca = "Stale PtpConfig CRD from previous test not cleaned up, causing config conflict in subsequent test."
	}
	if strings.Contains(prompt, "ordered") || strings.Contains(prompt, "setup") || strings.Contains(prompt, "beforesuite") {
		defectType = "ab001"
		component = "ptp-test-suite"
		rca = "Test setup ordering issue causing cascading failures in dependent tests."
	}
	if strings.Contains(prompt, "recovery") || strings.Contains(prompt, "restart") {
		rca = "PTP recovery mechanism fails due to holdover timeout being too short for the recovery scenario."
	}

	return map[string]any{
		"launch_id":         "",
		"case_ids":          []int{},
		"rca_message":       rca,
		"defect_type":       defectType,
		"component":         component,
		"convergence_score": 0.75,
		"evidence_refs":     []string{"linuxptp-daemon-operator:pkg/daemon/config.go"},
	}
}

func produceCorrelate(caseID string, prompt string) map[string]any {
	// Check if there are prior RCAs mentioned
	if strings.Contains(prompt, "rca") && strings.Contains(prompt, "rca #") {
		return map[string]any{
			"is_duplicate":        true,
			"linked_rca_id":       1,
			"confidence":          0.85,
			"reasoning":           "Same root cause pattern as prior RCA.",
			"cross_version_match": false,
		}
	}
	return map[string]any{
		"is_duplicate":        false,
		"linked_rca_id":       0,
		"confidence":          0.1,
		"reasoning":           "First RCA for this failure pattern.",
		"cross_version_match": false,
	}
}

func produceReview() map[string]any {
	return map[string]any{
		"decision": "approve",
	}
}

func produceReport(caseID string, prompt string) map[string]any {
	return map[string]any{
		"case_id":     caseID,
		"test_name":   "PTP test case",
		"summary":     "Investigation complete.",
		"defect_type": "pb001",
		"component":   "linuxptp-daemon",
	}
}
