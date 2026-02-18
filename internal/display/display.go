// Package display provides human-readable names for machine codes.
//
// Rule: code is for machines, words are for humans.
// Use these functions in CLI output, markdown reports, logs, and docs.
// Keep raw codes for JSON fields, map keys, and equality comparisons.
package display

import "strings"

// --- Defect Types ---

var defectTypes = map[string]string{
	"pb001": "Product Bug",
	"ab001": "Automation Bug",
	"au001": "Automation Bug",
	"si001": "System Issue",
	"en001": "Environment Issue",
	"fw001": "Firmware Issue",
	"nd001": "No Defect",
	"ti001": "To Investigate",
	"ib003": "Infrastructure Bug",
}

// DefectType returns the human-readable name for a defect code.
// Unknown codes are returned as-is.
func DefectType(code string) string {
	if name, ok := defectTypes[code]; ok {
		return name
	}
	return code
}

// DefectTypeWithCode returns "Product Bug (pb001)" format.
func DefectTypeWithCode(code string) string {
	if name, ok := defectTypes[code]; ok {
		return name + " (" + code + ")"
	}
	return code
}

// RPIssueTag formats an RP-provided issue type with a trust indicator.
// autoAnalyzed=true → "[auto]" (ML-assigned, low trust); false → "[human]".
// Returns "" when issueType is empty.
func RPIssueTag(issueType string, autoAnalyzed bool) string {
	if issueType == "" {
		return ""
	}
	tag := "[human]"
	if autoAnalyzed {
		tag = "[auto]"
	}
	return DefectType(issueType) + " " + tag
}

// --- Pipeline Stages ---

var stages = map[string]string{
	"F0":             "Recall",
	"F1":             "Triage",
	"F2":             "Resolve",
	"F3":             "Investigate",
	"F4":             "Correlate",
	"F5":             "Review",
	"F6":             "Report",
	"F0_RECALL":      "Recall",
	"F1_TRIAGE":      "Triage",
	"F2_RESOLVE":     "Resolve",
	"F3_INVESTIGATE": "Investigate",
	"F4_CORRELATE":   "Correlate",
	"F5_REVIEW":      "Review",
	"F6_REPORT":      "Report",
	"INIT":           "Init",
	"DONE":           "Done",
}

// shortStage maps any stage code variant to its short code (F0, F1, ...).
var shortStage = map[string]string{
	"F0":             "F0",
	"F1":             "F1",
	"F2":             "F2",
	"F3":             "F3",
	"F4":             "F4",
	"F5":             "F5",
	"F6":             "F6",
	"F0_RECALL":      "F0",
	"F1_TRIAGE":      "F1",
	"F2_RESOLVE":     "F2",
	"F3_INVESTIGATE": "F3",
	"F4_CORRELATE":   "F4",
	"F5_REVIEW":      "F5",
	"F6_REPORT":      "F6",
}

// Stage returns the human-readable name for a pipeline step code.
// "F0_RECALL" -> "Recall", "F2" -> "Resolve".
func Stage(code string) string {
	if name, ok := stages[code]; ok {
		return name
	}
	return code
}

// StageWithCode returns "Recall (F0)" format for dual-audience contexts.
func StageWithCode(code string) string {
	name, ok := stages[code]
	if !ok {
		return code
	}
	short, ok := shortStage[code]
	if !ok {
		return name
	}
	return name + " (" + short + ")"
}

// StagePath converts a slice of stage codes to a human-readable path.
// ["F0", "F1", "F2"] -> "Recall -> Triage -> Resolve"
func StagePath(codes []string) string {
	names := make([]string, len(codes))
	for i, c := range codes {
		names[i] = Stage(c)
	}
	return strings.Join(names, " \u2192 ")
}

// --- Metrics ---

var metrics = map[string]string{
	"M1":  "Defect Type Accuracy",
	"M2":  "Symptom Category Accuracy",
	"M3":  "Recall Hit Rate",
	"M4":  "Recall False Positive Rate",
	"M5":  "Serial Killer Detection",
	"M6":  "Skip Accuracy",
	"M7":  "Cascade Detection",
	"M8":  "Convergence Calibration",
	"M9":  "Repo Selection Precision",
	"M10": "Repo Selection Recall",
	"M11": "Red Herring Rejection",
	"M12": "Evidence Recall",
	"M13": "Evidence Precision",
	"M14": "RCA Message Relevance",
	"M15": "Component Identification",
	"M16": "Pipeline Path Accuracy",
	"M17": "Loop Efficiency",
	"M18": "Total Prompt Tokens",
	"M19": "Overall Accuracy",
	"M20": "Run Variance",
}

// Metric returns the human-readable name for a metric ID.
// "M1" -> "Defect Type Accuracy".
func Metric(id string) string {
	if name, ok := metrics[id]; ok {
		return name
	}
	return id
}

// MetricWithCode returns "Defect Type Accuracy (M1)" format.
func MetricWithCode(id string) string {
	if name, ok := metrics[id]; ok {
		return name + " (" + id + ")"
	}
	return id
}

// --- Heuristics ---

var heuristics = map[string]string{
	"H1":  "Recall Hit",
	"H2":  "Recall Miss",
	"H3":  "Recall Uncertain",
	"H4":  "Triage Investigate",
	"H5":  "Triage Skip",
	"H6":  "Triage Cascade",
	"H7":  "Resolve Single Repo",
	"H8":  "Resolve Multi Repo",
	"H9":  "Investigate Converged",
	"H10": "Investigate Loop",
	"H11": "Investigate Exhausted",
	"H12": "Correlate Duplicate",
	"H13": "Correlate Unique",
	"H14": "Review Approve",
	"H15": "Review Reassess",
	"H16": "Review Overturn",
	"H17": "Report Emit",
	"H18": "Investigate Reopen",
}

// Heuristic returns the human-readable name for a heuristic ID.
// "H1" -> "Recall Hit".
func Heuristic(id string) string {
	if name, ok := heuristics[id]; ok {
		return name
	}
	return id
}

// HeuristicWithCode returns "Recall Hit (H1)" format.
func HeuristicWithCode(id string) string {
	if name, ok := heuristics[id]; ok {
		return name + " (" + id + ")"
	}
	return id
}

// --- Cluster Keys ---

// ClusterKey humanizes a pipe-delimited cluster key.
// "product|ptp4l|pb001" -> "product / ptp4l / Product Bug"
func ClusterKey(key string) string {
	parts := strings.Split(key, "|")
	for i, p := range parts {
		if name, ok := defectTypes[p]; ok {
			parts[i] = name
		}
	}
	return strings.Join(parts, " / ")
}
