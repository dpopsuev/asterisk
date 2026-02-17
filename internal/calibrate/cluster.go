package calibrate

import (
	"strings"
)

// SymptomCluster groups triage results that share similar symptoms.
// Only the Representative undergoes investigation (F2-F6); results are
// propagated to Members.
type SymptomCluster struct {
	Key            string          // cluster key: "category|component|defect_hypothesis"
	Representative *TriageResult
	Members        []*TriageResult // includes representative
}

// ClusterCases groups triage results into symptom clusters.
// The primary key is {symptom_category, component, defect_type_hypothesis}.
// Cases that had a recall hit or were triaged as skip form their own singleton clusters.
// Within each cluster, the first case becomes the representative.
func ClusterCases(results []TriageResult, scenario *Scenario) []SymptomCluster {
	clusters := make(map[string]*SymptomCluster)
	var order []string // preserve insertion order

	for i := range results {
		tr := &results[i]

		// Recall hits and skip cases form singleton clusters
		if tr.RecallHit || (tr.TriageArtifact != nil && tr.TriageArtifact.SkipInvestigation) {
			key := singletonKey(tr)
			if _, ok := clusters[key]; !ok {
				clusters[key] = &SymptomCluster{
					Key:            key,
					Representative: tr,
				}
				order = append(order, key)
			}
			clusters[key].Members = append(clusters[key].Members, tr)
			continue
		}

		// Compute cluster key from triage result
		key := clusterKey(tr)
		if _, ok := clusters[key]; !ok {
			clusters[key] = &SymptomCluster{
				Key:            key,
				Representative: tr,
			}
			order = append(order, key)
		}
		clusters[key].Members = append(clusters[key].Members, tr)
	}

	// Collect clusters in insertion order
	result := make([]SymptomCluster, 0, len(order))
	for _, key := range order {
		result = append(result, *clusters[key])
	}

	return result
}

// clusterKey computes the primary clustering key from a triage result.
func clusterKey(tr *TriageResult) string {
	if tr.TriageArtifact == nil {
		return singletonKey(tr)
	}
	ta := tr.TriageArtifact
	category := strings.ToLower(strings.TrimSpace(ta.SymptomCategory))
	defect := strings.ToLower(strings.TrimSpace(ta.DefectTypeHypothesis))

	// Try to extract component from candidate repos
	component := ""
	if len(ta.CandidateRepos) > 0 {
		component = strings.ToLower(ta.CandidateRepos[0])
	}

	return category + "|" + component + "|" + defect
}

// singletonKey creates a unique key for a case that won't cluster with others.
func singletonKey(tr *TriageResult) string {
	return "singleton|" + tr.CaseResult.CaseID
}

// JaccardSimilarity computes the Jaccard coefficient between two token sets.
func JaccardSimilarity(a, b []string) float64 {
	if len(a) == 0 && len(b) == 0 {
		return 1.0
	}
	setA := make(map[string]bool, len(a))
	for _, s := range a {
		setA[strings.ToLower(s)] = true
	}
	setB := make(map[string]bool, len(b))
	for _, s := range b {
		setB[strings.ToLower(s)] = true
	}

	intersection := 0
	for s := range setA {
		if setB[s] {
			intersection++
		}
	}

	union := len(setA)
	for s := range setB {
		if !setA[s] {
			union++
		}
	}

	if union == 0 {
		return 1.0
	}
	return float64(intersection) / float64(union)
}

// Tokenize splits text into whitespace-delimited lowercase tokens.
func Tokenize(text string) []string {
	words := strings.Fields(strings.ToLower(text))
	result := make([]string, 0, len(words))
	for _, w := range words {
		w = strings.Trim(w, ".,;:!?()[]{}\"'`")
		if len(w) > 2 {
			result = append(result, w)
		}
	}
	return result
}
