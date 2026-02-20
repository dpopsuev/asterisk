package calibrate

import (
	"strings"
	"testing"

	"asterisk/internal/orchestrate"
)

func TestClusterCases_Groups(t *testing.T) {
	results := []TriageResult{
		{
			Index:      0,
			CaseResult: &CaseResult{CaseID: "C1", Version: "4.18"},
			TriageArtifact: &orchestrate.TriageResult{
				SymptomCategory:      "product",
				DefectTypeHypothesis: "pb001",
				CandidateRepos:       []string{"linuxptp-daemon-operator"},
			},
		},
		{
			Index:      1,
			CaseResult: &CaseResult{CaseID: "C2"},
			RecallHit:  true,
		},
		{
			Index:      2,
			CaseResult: &CaseResult{CaseID: "C3", Version: "4.18"},
			TriageArtifact: &orchestrate.TriageResult{
				SymptomCategory:      "product",
				DefectTypeHypothesis: "pb001",
				CandidateRepos:       []string{"linuxptp-daemon-operator"},
			},
		},
		{
			Index:      3,
			CaseResult: &CaseResult{CaseID: "C4", Version: "4.18"},
			TriageArtifact: &orchestrate.TriageResult{
				SymptomCategory:      "automation",
				DefectTypeHypothesis: "au001",
				CandidateRepos:       []string{"ptp-test-framework"},
			},
		},
		{
			Index:      4,
			CaseResult: &CaseResult{CaseID: "C5"},
			TriageArtifact: &orchestrate.TriageResult{
				SymptomCategory:      "infra",
				SkipInvestigation:    true,
			},
		},
	}

	clusters := ClusterCases(results, nil)

	if len(clusters) != 4 {
		t.Fatalf("expected 4 clusters, got %d: %v", len(clusters), clusterKeys(clusters))
	}

	expectedKey := "product|linuxptp-daemon-operator|pb001|4.18|"
	var productCluster *SymptomCluster
	for i := range clusters {
		if clusters[i].Key == expectedKey {
			productCluster = &clusters[i]
			break
		}
	}
	if productCluster == nil {
		t.Fatalf("product cluster not found; keys: %v", clusterKeys(clusters))
	}
	if len(productCluster.Members) != 2 {
		t.Errorf("product cluster: expected 2 members, got %d", len(productCluster.Members))
	}
	if productCluster.Representative.CaseResult.CaseID != "C1" {
		t.Errorf("product cluster representative: got %s, want C1",
			productCluster.Representative.CaseResult.CaseID)
	}
}

func clusterKeys(clusters []SymptomCluster) []string {
	keys := make([]string, len(clusters))
	for i, c := range clusters {
		keys[i] = c.Key
	}
	return keys
}

func TestClusterCases_AllSingletons(t *testing.T) {
	results := []TriageResult{
		{Index: 0, CaseResult: &CaseResult{CaseID: "C1"}, RecallHit: true},
		{Index: 1, CaseResult: &CaseResult{CaseID: "C2"}, RecallHit: true},
	}
	clusters := ClusterCases(results, nil)
	if len(clusters) != 2 {
		t.Fatalf("expected 2 singleton clusters, got %d", len(clusters))
	}
}

func TestClusterDedup_ReducesSteps(t *testing.T) {
	results := make([]TriageResult, 5)
	for i := 0; i < 5; i++ {
		results[i] = TriageResult{
			Index:      i,
			CaseResult: &CaseResult{CaseID: "C" + string(rune('1'+i))},
			TriageArtifact: &orchestrate.TriageResult{
				SymptomCategory:      "product",
				DefectTypeHypothesis: "pb001",
				CandidateRepos:       []string{"myrepo"},
			},
		}
	}

	clusters := ClusterCases(results, nil)
	if len(clusters) != 3 {
		t.Errorf("expected 3 clusters (1 capped at %d + 2 singletons), got %d: %v",
			MaxClusterSize, len(clusters), clusterKeys(clusters))
	}
	if len(clusters[0].Members) != MaxClusterSize {
		t.Errorf("expected %d members in first cluster, got %d",
			MaxClusterSize, len(clusters[0].Members))
	}
}

func TestClusterDedup_WithinCap(t *testing.T) {
	results := make([]TriageResult, MaxClusterSize)
	for i := range results {
		results[i] = TriageResult{
			Index:      i,
			CaseResult: &CaseResult{CaseID: "C" + string(rune('1'+i))},
			TriageArtifact: &orchestrate.TriageResult{
				SymptomCategory:      "product",
				DefectTypeHypothesis: "pb001",
				CandidateRepos:       []string{"myrepo"},
			},
		}
	}
	clusters := ClusterCases(results, nil)
	if len(clusters) != 1 {
		t.Fatalf("expected 1 cluster at cap boundary, got %d", len(clusters))
	}
	if len(clusters[0].Members) != MaxClusterSize {
		t.Errorf("expected %d members, got %d", MaxClusterSize, len(clusters[0].Members))
	}
}

func TestClusterKey_VersionDiscrimination(t *testing.T) {
	results := []TriageResult{
		{
			Index:      0,
			CaseResult: &CaseResult{CaseID: "C1", Version: "4.16"},
			TriageArtifact: &orchestrate.TriageResult{
				SymptomCategory:      "product",
				DefectTypeHypothesis: "pb001",
				CandidateRepos:       []string{"linuxptp-daemon"},
			},
		},
		{
			Index:      1,
			CaseResult: &CaseResult{CaseID: "C2", Version: "4.18"},
			TriageArtifact: &orchestrate.TriageResult{
				SymptomCategory:      "product",
				DefectTypeHypothesis: "pb001",
				CandidateRepos:       []string{"linuxptp-daemon"},
			},
		},
	}
	clusters := ClusterCases(results, nil)
	if len(clusters) != 2 {
		t.Errorf("expected 2 clusters (different versions), got %d: %v",
			len(clusters), clusterKeys(clusters))
	}
}

// phase5aTriageResults builds 18 TriageResults mirroring the real Phase 5a
// calibration run: mostly product/pb001 with linuxptp-daemon as first repo,
// spread across 7 OCP versions with varied data-quality notes.
func phase5aTriageResults() []TriageResult {
	type spec struct {
		caseID, version, repo, category, defect, notes string
	}
	specs := []spec{
		{"C04", "4.18", "cloud-event-proxy", "product", "pb001", "Error message appears truncated"},
		{"C05", "4.17", "linuxptp-daemon", "product", "pb001", "Error message concatenates two bug descriptions"},
		{"C06", "4.14", "", "product", "ti001", "No error message available"},
		{"C08", "4.15", "ptp-operator", "product", "pb001", "Sparse error context; test name empty"},
		{"C09", "4.21", "linuxptp-daemon", "product", "pb001", "Test name and launch ID empty; single failure"},
		{"C10", "4.21", "linuxptp-daemon", "product", "pb001", "Error message minimal; only suite summary visible"},
		{"C13", "4.20", "linuxptp-daemon", "product", "pb001", "Test name empty; error message truncated"},
		{"C14", "4.21", "linuxptp-daemon", "product", "pb001", "Test name empty; launch attributes absent"},
		{"C15", "4.21", "linuxptp-daemon", "product", "pb001", "Test name empty; error message is terse"},
		{"C17", "4.18", "cloud-event-proxy", "product", "pb001", "Ginkgo summary line; G9 >40% skipped"},
		{"C21", "4.16", "linuxptp-daemon", "product", "pb001", "Test name empty"},
		{"C22", "4.16", "cnf-gotests", "product", "pb001", "Error message truncated; minimal context"},
		{"C23", "4.16", "linuxptp-daemon", "product", "pb001", "Test name and launch ID empty; sparse metadata"},
		{"C24", "4.16", "ptp-operator", "environment", "ti001", "Error message is only a Jira URL"},
		{"C26", "4.16", "linuxptp-daemon", "product", "pb001", ""},
		{"C27", "4.14", "cnf-gotests", "automation", "au001", "Error field contains Jira ticket title"},
		{"C28", "4.17", "linuxptp-daemon", "product", "pb001", "Sparse metadata; error message is test name only"},
		{"C29", "4.17", "linuxptp-daemon", "product", "pb001", "Test name empty; typo PHC2SYSY in error text"},
	}

	results := make([]TriageResult, len(specs))
	for i, s := range specs {
		repos := []string{}
		if s.repo != "" {
			repos = []string{s.repo}
		}
		results[i] = TriageResult{
			Index:      i,
			CaseResult: &CaseResult{CaseID: s.caseID, Version: s.version},
			TriageArtifact: &orchestrate.TriageResult{
				SymptomCategory:      s.category,
				DefectTypeHypothesis: s.defect,
				CandidateRepos:       repos,
				DataQualityNotes:     s.notes,
			},
		}
	}
	return results
}

// legacyClusterKey simulates the old 3-field key that caused starvation.
func legacyClusterKey(tr *TriageResult) string {
	if tr.TriageArtifact == nil {
		return "singleton|" + tr.CaseResult.CaseID
	}
	ta := tr.TriageArtifact
	category := strings.ToLower(strings.TrimSpace(ta.SymptomCategory))
	defect := strings.ToLower(strings.TrimSpace(ta.DefectTypeHypothesis))
	component := ""
	if len(ta.CandidateRepos) > 0 {
		component = strings.ToLower(ta.CandidateRepos[0])
	}
	return category + "|" + component + "|" + defect
}

func TestClusterStarvation_ReplicatesPhase5a(t *testing.T) {
	results := phase5aTriageResults()

	// --- Part 1: Prove old key causes starvation ---
	// Count distinct keys using the legacy 3-field formula.
	oldKeys := make(map[string]int)
	for i := range results {
		k := legacyClusterKey(&results[i])
		oldKeys[k]++
	}
	// With the old key, "product|linuxptp-daemon|pb001" captures the majority.
	dominant := oldKeys["product|linuxptp-daemon|pb001"]
	if dominant < 8 {
		t.Errorf("old key: expected product|linuxptp-daemon|pb001 to capture >=8 cases, got %d", dominant)
	}
	if len(oldKeys) > 8 {
		t.Errorf("old key: expected <=8 distinct keys (few clusters), got %d", len(oldKeys))
	}
	t.Logf("old key: %d distinct keys; dominant cluster has %d cases", len(oldKeys), dominant)

	// --- Part 2: Prove current key prevents starvation ---
	clusters := ClusterCases(results, nil)

	// With version + fingerprint + MaxClusterSize=3, we expect many more clusters.
	if len(clusters) < 14 {
		t.Errorf("new clustering: expected >=14 clusters, got %d: %v",
			len(clusters), clusterKeys(clusters))
	}

	// No cluster should exceed MaxClusterSize.
	for _, c := range clusters {
		if len(c.Members) > MaxClusterSize {
			t.Errorf("cluster %q has %d members, exceeds MaxClusterSize=%d",
				c.Key, len(c.Members), MaxClusterSize)
		}
	}

	// Every version should have at least one representative.
	repVersions := make(map[string]bool)
	for _, c := range clusters {
		if c.Representative.CaseResult != nil {
			repVersions[c.Representative.CaseResult.Version] = true
		}
	}
	expectedVersions := []string{"4.14", "4.15", "4.16", "4.17", "4.18", "4.20", "4.21"}
	for _, v := range expectedVersions {
		if !repVersions[v] {
			t.Errorf("version %s has no cluster representative", v)
		}
	}

	// Count total investigated (representatives of non-singleton clusters that need investigation).
	investigated := 0
	for _, c := range clusters {
		if c.Representative.TriageArtifact != nil && !c.Representative.TriageArtifact.SkipInvestigation {
			investigated++
		}
	}
	// With the old key, only ~6-7 got investigated. Now we expect >=14.
	if investigated < 14 {
		t.Errorf("new clustering: expected >=14 investigated representatives, got %d", investigated)
	}
	t.Logf("new clustering: %d clusters, %d investigated, %d versions covered",
		len(clusters), investigated, len(repVersions))
}

func TestJaccardSimilarity(t *testing.T) {
	tests := []struct {
		a, b     []string
		expected float64
	}{
		{nil, nil, 1.0},
		{[]string{"a", "b"}, []string{"a", "b"}, 1.0},
		{[]string{"a", "b"}, []string{"c", "d"}, 0.0},
		{[]string{"a", "b", "c"}, []string{"b", "c", "d"}, 0.5},
		{[]string{"A", "B"}, []string{"a", "b"}, 1.0},
	}
	for _, tt := range tests {
		got := JaccardSimilarity(tt.a, tt.b)
		if got < tt.expected-0.01 || got > tt.expected+0.01 {
			t.Errorf("Jaccard(%v, %v) = %.3f, want %.3f", tt.a, tt.b, got, tt.expected)
		}
	}
}

func TestTokenize(t *testing.T) {
	tokens := Tokenize("Error: failed to sync PTP clock (timeout=30s)")
	if len(tokens) < 4 {
		t.Fatalf("Tokenize: expected at least 4 tokens, got %d: %v", len(tokens), tokens)
	}
	tokenSet := make(map[string]bool)
	for _, tk := range tokens {
		tokenSet[tk] = true
	}
	for _, want := range []string{"error", "failed", "sync", "ptp", "clock"} {
		if !tokenSet[want] {
			t.Errorf("Tokenize: missing expected token %q in %v", want, tokens)
		}
	}
}
