// Package vocabulary provides the Asterisk domain vocabulary as a
// RichMapVocabulary. It registers defect types, pipeline stages,
// metrics, and heuristics — all four display-name domains.
package vocabulary

import (
	"strings"

	framework "github.com/dpopsuev/origami"
)

// New builds and returns a fully populated RichMapVocabulary containing
// all Asterisk domain codes: defect types, pipeline stages, metrics,
// and heuristics.
func New() *framework.RichMapVocabulary {
	v := framework.NewRichMapVocabulary()
	registerDefectTypes(v)
	registerStages(v)
	registerMetrics(v)
	registerHeuristics(v)
	return v
}

func registerDefectTypes(v *framework.RichMapVocabulary) {
	v.RegisterEntries(map[string]framework.VocabEntry{
		"pb001": {Short: "pb001", Long: "Product Bug"},
		"ab001": {Short: "ab001", Long: "Automation Bug"},
		"au001": {Short: "au001", Long: "Automation Bug"},
		"si001": {Short: "si001", Long: "System Issue"},
		"en001": {Short: "en001", Long: "Environment Issue"},
		"fw001": {Short: "fw001", Long: "Firmware Issue"},
		"nd001": {Short: "nd001", Long: "No Defect"},
		"ti001": {Short: "ti001", Long: "To Investigate"},
		"ib003": {Short: "ib003", Long: "Infrastructure Bug"},
	})
}

func registerStages(v *framework.RichMapVocabulary) {
	type s = framework.VocabEntry
	v.RegisterEntries(map[string]framework.VocabEntry{
		"F0":             {Short: "F0", Long: "Recall"},
		"F1":             {Short: "F1", Long: "Triage"},
		"F2":             {Short: "F2", Long: "Resolve"},
		"F3":             {Short: "F3", Long: "Investigate"},
		"F4":             {Short: "F4", Long: "Correlate"},
		"F5":             {Short: "F5", Long: "Review"},
		"F6":             {Short: "F6", Long: "Report"},
		"F0_RECALL":      {Short: "F0", Long: "Recall"},
		"F1_TRIAGE":      {Short: "F1", Long: "Triage"},
		"F2_RESOLVE":     {Short: "F2", Long: "Resolve"},
		"F3_INVESTIGATE": {Short: "F3", Long: "Investigate"},
		"F4_CORRELATE":   {Short: "F4", Long: "Correlate"},
		"F5_REVIEW":      {Short: "F5", Long: "Review"},
		"F6_REPORT":      {Short: "F6", Long: "Report"},
		"INIT":           {Short: "INIT", Long: "Init"},
		"DONE":           {Short: "DONE", Long: "Done"},
	})
}

func registerMetrics(v *framework.RichMapVocabulary) {
	v.RegisterEntries(map[string]framework.VocabEntry{
		"M1":   {Short: "M1", Long: "Defect Type Accuracy"},
		"M2":   {Short: "M2", Long: "Symptom Category Accuracy"},
		"M3":   {Short: "M3", Long: "Recall Hit Rate"},
		"M4":   {Short: "M4", Long: "Recall False Positive Rate"},
		"M5":   {Short: "M5", Long: "Serial Killer Detection"},
		"M6":   {Short: "M6", Long: "Skip Accuracy"},
		"M7":   {Short: "M7", Long: "Cascade Detection"},
		"M8":   {Short: "M8", Long: "Convergence Calibration"},
		"M9":   {Short: "M9", Long: "Repo Selection Precision"},
		"M10":  {Short: "M10", Long: "Repo Selection Recall"},
		"M11":  {Short: "M11", Long: "Red Herring Rejection"},
		"M12":  {Short: "M12", Long: "Evidence Recall"},
		"M13":  {Short: "M13", Long: "Evidence Precision"},
		"M14":  {Short: "M14", Long: "RCA Message Relevance"},
		"M14b": {Short: "M14b", Long: "Smoking Gun Hit Rate"},
		"M15":  {Short: "M15", Long: "Component Identification"},
		"M16":  {Short: "M16", Long: "Pipeline Path Accuracy"},
		"M17":  {Short: "M17", Long: "Loop Efficiency"},
		"M18":  {Short: "M18", Long: "Total Prompt Tokens"},
		"M19":  {Short: "M19", Long: "Overall Accuracy"},
		"M20":  {Short: "M20", Long: "Run Variance"},
	})
}

func registerHeuristics(v *framework.RichMapVocabulary) {
	v.RegisterEntries(map[string]framework.VocabEntry{
		"H1":  {Short: "H1", Long: "Recall Hit"},
		"H2":  {Short: "H2", Long: "Recall Miss"},
		"H3":  {Short: "H3", Long: "Recall Uncertain"},
		"H4":  {Short: "H4", Long: "Triage Investigate"},
		"H5":  {Short: "H5", Long: "Triage Skip"},
		"H6":  {Short: "H6", Long: "Triage Cascade"},
		"H7":  {Short: "H7", Long: "Resolve Single Repo"},
		"H8":  {Short: "H8", Long: "Resolve Multi Repo"},
		"H9":  {Short: "H9", Long: "Investigate Converged"},
		"H10": {Short: "H10", Long: "Investigate Loop"},
		"H11": {Short: "H11", Long: "Investigate Exhausted"},
		"H12": {Short: "H12", Long: "Correlate Duplicate"},
		"H13": {Short: "H13", Long: "Correlate Unique"},
		"H14": {Short: "H14", Long: "Review Approve"},
		"H15": {Short: "H15", Long: "Review Reassess"},
		"H16": {Short: "H16", Long: "Review Overturn"},
		"H17": {Short: "H17", Long: "Report Emit"},
		"H18": {Short: "H18", Long: "Investigate Reopen"},
	})
}

// --- Domain helpers (composite logic beyond simple lookup) ---

// RPIssueTag formats an RP-provided issue type with a trust indicator.
// autoAnalyzed=true -> "[auto]" (ML-assigned, low trust); false -> "[human]".
// Returns "" when issueType is empty.
func RPIssueTag(v framework.Vocabulary, issueType string, autoAnalyzed bool) string {
	if issueType == "" {
		return ""
	}
	tag := "[human]"
	if autoAnalyzed {
		tag = "[auto]"
	}
	return v.Name(issueType) + " " + tag
}

// StagePath converts a slice of stage codes to a human-readable path.
// ["F0", "F1", "F2"] -> "Recall -> Triage -> Resolve"
func StagePath(v framework.Vocabulary, codes []string) string {
	names := make([]string, len(codes))
	for i, c := range codes {
		names[i] = v.Name(c)
	}
	return strings.Join(names, " \u2192 ")
}

// ClusterKey humanizes a pipe-delimited cluster key.
// "product|ptp4l|pb001" -> "product / ptp4l / Product Bug"
func ClusterKey(v framework.Vocabulary, key string) string {
	parts := strings.Split(key, "|")
	for i, p := range parts {
		if name := v.Name(p); name != p {
			parts[i] = name
		}
	}
	return strings.Join(parts, " / ")
}
