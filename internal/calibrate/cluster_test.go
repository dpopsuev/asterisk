package calibrate

import (
	"testing"

	"asterisk/internal/orchestrate"
)

func TestClusterCases_Groups(t *testing.T) {
	results := []TriageResult{
		{
			Index:      0,
			CaseResult: &CaseResult{CaseID: "C1"},
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
			CaseResult: &CaseResult{CaseID: "C3"},
			TriageArtifact: &orchestrate.TriageResult{
				SymptomCategory:      "product",
				DefectTypeHypothesis: "pb001",
				CandidateRepos:       []string{"linuxptp-daemon-operator"},
			},
		},
		{
			Index:      3,
			CaseResult: &CaseResult{CaseID: "C4"},
			TriageArtifact: &orchestrate.TriageResult{
				SymptomCategory:      "automation",
				DefectTypeHypothesis: "ab001",
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

	// C2 (recall hit) → singleton
	// C5 (skip investigation) → singleton
	// C1 + C3 → cluster (product|linuxptp...|pb001)
	// C4 → cluster (automation|ptp...|ab001)
	// Total: 4 clusters

	if len(clusters) != 4 {
		t.Fatalf("expected 4 clusters, got %d", len(clusters))
	}

	// Find the product cluster
	var productCluster *SymptomCluster
	for i := range clusters {
		if clusters[i].Key == "product|linuxptp-daemon-operator|pb001" {
			productCluster = &clusters[i]
			break
		}
	}
	if productCluster == nil {
		t.Fatal("product cluster not found")
	}
	if len(productCluster.Members) != 2 {
		t.Errorf("product cluster: expected 2 members, got %d", len(productCluster.Members))
	}
	if productCluster.Representative.CaseResult.CaseID != "C1" {
		t.Errorf("product cluster representative: got %s, want C1",
			productCluster.Representative.CaseResult.CaseID)
	}
}

func TestClusterCases_AllSingletons(t *testing.T) {
	results := []TriageResult{
		{
			Index:      0,
			CaseResult: &CaseResult{CaseID: "C1"},
			RecallHit:  true,
		},
		{
			Index:      1,
			CaseResult: &CaseResult{CaseID: "C2"},
			RecallHit:  true,
		},
	}

	clusters := ClusterCases(results, nil)
	if len(clusters) != 2 {
		t.Fatalf("expected 2 singleton clusters, got %d", len(clusters))
	}
}

func TestClusterDedup_ReducesSteps(t *testing.T) {
	// 5 cases with the same triage result → 1 cluster → 1 investigation
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
	investigationCount := 0
	for _, c := range clusters {
		if !c.Representative.RecallHit &&
			(c.Representative.TriageArtifact == nil || !c.Representative.TriageArtifact.SkipInvestigation) {
			investigationCount++
		}
	}

	if investigationCount != 1 {
		t.Errorf("expected 1 investigation (dedup), got %d", investigationCount)
	}
	if len(clusters) != 1 {
		t.Errorf("expected 1 cluster, got %d", len(clusters))
	}
	if len(clusters[0].Members) != 5 {
		t.Errorf("expected 5 members in cluster, got %d", len(clusters[0].Members))
	}
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
		{[]string{"A", "B"}, []string{"a", "b"}, 1.0}, // case-insensitive
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
	// Words <= 2 chars are filtered, punctuation trimmed, "to" filtered
	if len(tokens) < 4 {
		t.Fatalf("Tokenize: expected at least 4 tokens, got %d: %v", len(tokens), tokens)
	}
	// Verify specific expected tokens are present
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
