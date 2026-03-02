package rca

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	framework "github.com/dpopsuev/origami"
)

// --- Artifact I/O tests ---

func TestArtifactReadWrite(t *testing.T) {
	dir := t.TempDir()
	caseDir := filepath.Join(dir, "1", "10")
	if err := os.MkdirAll(caseDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Write a recall result
	recall := &RecallResult{
		Match: true, PriorRCAID: 42, Confidence: 0.85, Reasoning: "same error pattern",
	}
	if err := WriteArtifact(caseDir, "recall-result.json", recall); err != nil {
		t.Fatalf("WriteArtifact: %v", err)
	}

	// Read it back
	got, err := ReadArtifact[RecallResult](caseDir, "recall-result.json")
	if err != nil {
		t.Fatalf("ReadArtifact: %v", err)
	}
	if got == nil || !got.Match || got.PriorRCAID != 42 || got.Confidence != 0.85 {
		t.Errorf("ReadArtifact mismatch: got %+v", got)
	}

	// Read non-existent = nil
	missing, err := ReadArtifact[RecallResult](caseDir, "missing.json")
	if err != nil {
		t.Fatalf("ReadArtifact missing: %v", err)
	}
	if missing != nil {
		t.Errorf("expected nil for missing artifact, got %+v", missing)
	}
}

func TestWritePrompt(t *testing.T) {
	dir := t.TempDir()
	caseDir := filepath.Join(dir, "1", "10")
	if err := os.MkdirAll(caseDir, 0755); err != nil {
		t.Fatal(err)
	}

	path, err := WritePrompt(caseDir, StepF1Triage, 0, "# Triage prompt\nContent here")
	if err != nil {
		t.Fatalf("WritePrompt: %v", err)
	}
	if filepath.Base(path) != "prompt-triage.md" {
		t.Errorf("prompt filename: got %q", filepath.Base(path))
	}

	// Loop iteration
	path, err = WritePrompt(caseDir, StepF3Invest, 2, "# Investigate loop 2")
	if err != nil {
		t.Fatalf("WritePrompt loop: %v", err)
	}
	if filepath.Base(path) != "prompt-investigate-loop-2.md" {
		t.Errorf("loop prompt filename: got %q", filepath.Base(path))
	}
}

func TestArtifactFilename(t *testing.T) {
	tests := []struct {
		step CircuitStep
		want string
	}{
		{StepF0Recall, "recall-result.json"},
		{StepF1Triage, "triage-result.json"},
		{StepF2Resolve, "resolve-result.json"},
		{StepF3Invest, "artifact.json"},
		{StepF4Correlate, "correlate-result.json"},
		{StepF5Review, "review-decision.json"},
		{StepF6Report, "jira-draft.json"},
		{StepInit, ""},
		{StepDone, ""},
	}
	for _, tt := range tests {
		got := ArtifactFilename(tt.step)
		if got != tt.want {
			t.Errorf("ArtifactFilename(%s): got %q want %q", tt.step, got, tt.want)
		}
	}
}

// --- Graph-edge evaluation tests ---
//
// These validate the YAML expression edges in circuit_rca.yaml using
// framework APIs directly (no domain wrappers).

type noopTransformer struct{}

func (noopTransformer) Name() string { return "noop" }
func (noopTransformer) Transform(_ context.Context, _ *framework.TransformerContext) (any, error) {
	return nil, nil
}

func buildTestRunner(t *testing.T) *framework.Runner {
	t.Helper()
	runner, err := BuildRunner(DefaultThresholds(), TransformerAdapter(&noopTransformer{}))
	if err != nil {
		t.Fatalf("BuildRunner: %v", err)
	}
	return runner
}

// evaluateEdge finds the first matching edge from nodeName and returns
// (target node, edge ID). Returns ("", "") if no edge matches.
func evaluateEdge(runner *framework.Runner, nodeName string, art framework.Artifact, ws *framework.WalkerState) (string, string) {
	for _, e := range runner.Graph.EdgesFrom(nodeName) {
		if tr := e.Evaluate(art, ws); tr != nil {
			return tr.NextNode, e.ID()
		}
	}
	return "", ""
}

func TestEdge_RecallHit(t *testing.T) {
	runner := buildTestRunner(t)
	art := WrapArtifact(StepF0Recall, &RecallResult{Match: true, PriorRCAID: 5, Confidence: 0.9})
	ws := framework.NewWalkerState("10")

	target, edgeID := evaluateEdge(runner, "recall", art, ws)
	if target != "review" || edgeID != "H1" {
		t.Errorf("recall-hit: got target=%s edge=%s", target, edgeID)
	}
}

func TestEdge_RecallMiss(t *testing.T) {
	runner := buildTestRunner(t)
	art := WrapArtifact(StepF0Recall, &RecallResult{Match: false, Confidence: 0})
	ws := framework.NewWalkerState("10")

	target, edgeID := evaluateEdge(runner, "recall", art, ws)
	if target != "triage" || edgeID != "H2" {
		t.Errorf("recall-miss: got target=%s edge=%s", target, edgeID)
	}
}

func TestEdge_RecallUncertain(t *testing.T) {
	runner := buildTestRunner(t)
	art := WrapArtifact(StepF0Recall, &RecallResult{Match: true, PriorRCAID: 5, Confidence: 0.6})
	ws := framework.NewWalkerState("10")

	target, edgeID := evaluateEdge(runner, "recall", art, ws)
	if target != "triage" || edgeID != "H3" {
		t.Errorf("recall-uncertain: got target=%s edge=%s", target, edgeID)
	}
}

func TestEdge_TriageSkipInfra(t *testing.T) {
	runner := buildTestRunner(t)
	art := WrapArtifact(StepF1Triage, &TriageResult{SymptomCategory: "infra", SkipInvestigation: true})
	ws := framework.NewWalkerState("10")

	target, edgeID := evaluateEdge(runner, "triage", art, ws)
	if target != "review" || edgeID != "H4" {
		t.Errorf("triage-skip-infra: got target=%s edge=%s", target, edgeID)
	}
}

func TestEdge_TriageInvestigate(t *testing.T) {
	runner := buildTestRunner(t)
	art := WrapArtifact(StepF1Triage, &TriageResult{
		SymptomCategory: "assertion", SkipInvestigation: false, CandidateRepos: []string{"repo-a", "repo-b"},
	})
	ws := framework.NewWalkerState("10")

	target, edgeID := evaluateEdge(runner, "triage", art, ws)
	if target != "resolve" || edgeID != "H6" {
		t.Errorf("triage-investigate: got target=%s edge=%s", target, edgeID)
	}
}

func TestEdge_TriageSingleRepo(t *testing.T) {
	runner := buildTestRunner(t)
	art := WrapArtifact(StepF1Triage, &TriageResult{
		SymptomCategory: "assertion", SkipInvestigation: false, CandidateRepos: []string{"repo-a"},
	})
	ws := framework.NewWalkerState("10")

	target, edgeID := evaluateEdge(runner, "triage", art, ws)
	if target != "investigate" || edgeID != "H7" {
		t.Errorf("triage-single-repo: got target=%s edge=%s", target, edgeID)
	}
}

func TestEdge_InvestigateConverged(t *testing.T) {
	runner := buildTestRunner(t)
	art := WrapArtifact(StepF3Invest, &InvestigateArtifact{ConvergenceScore: 0.85})
	ws := framework.NewWalkerState("10")

	target, edgeID := evaluateEdge(runner, "investigate", art, ws)
	if target != "correlate" || edgeID != "H9" {
		t.Errorf("investigate-converged: got target=%s edge=%s", target, edgeID)
	}
}

func TestEdge_InvestigateLowLoop(t *testing.T) {
	runner := buildTestRunner(t)
	art := WrapArtifact(StepF3Invest, &InvestigateArtifact{
		ConvergenceScore: 0.40, EvidenceRefs: []string{"some-evidence"},
	})
	ws := framework.NewWalkerState("10")

	target, edgeID := evaluateEdge(runner, "investigate", art, ws)
	if target != "resolve" || edgeID != "H10" {
		t.Errorf("investigate-low: got target=%s edge=%s", target, edgeID)
	}
}

func TestEdge_InvestigateExhausted(t *testing.T) {
	runner := buildTestRunner(t)
	art := WrapArtifact(StepF3Invest, &InvestigateArtifact{
		ConvergenceScore: 0.40, EvidenceRefs: []string{"some-evidence"},
	})
	ws := framework.NewWalkerState("10")
	ws.LoopCounts["investigate"] = 1

	target, edgeID := evaluateEdge(runner, "investigate", art, ws)
	if target != "review" || edgeID != "H11" {
		t.Errorf("investigate-exhausted: got target=%s edge=%s", target, edgeID)
	}
}

func TestEdge_ReviewApprove(t *testing.T) {
	runner := buildTestRunner(t)
	art := WrapArtifact(StepF5Review, &ReviewDecision{Decision: "approve"})
	ws := framework.NewWalkerState("10")

	target, edgeID := evaluateEdge(runner, "review", art, ws)
	if target != "report" || edgeID != "H12" {
		t.Errorf("review-approve: got target=%s edge=%s", target, edgeID)
	}
}

func TestEdge_ReviewReassess(t *testing.T) {
	runner := buildTestRunner(t)
	art := WrapArtifact(StepF5Review, &ReviewDecision{Decision: "reassess", LoopTarget: StepF3Invest})
	ws := framework.NewWalkerState("10")

	target, edgeID := evaluateEdge(runner, "review", art, ws)
	if target != "resolve" || edgeID != "H13" {
		t.Errorf("review-reassess: got target=%s edge=%s", target, edgeID)
	}
}

func TestEdge_ReviewOverturn(t *testing.T) {
	runner := buildTestRunner(t)
	art := WrapArtifact(StepF5Review, &ReviewDecision{
		Decision:      "overturn",
		HumanOverride: &HumanOverride{DefectType: "pb001", RCAMessage: "human says this"},
	})
	ws := framework.NewWalkerState("10")

	target, edgeID := evaluateEdge(runner, "review", art, ws)
	if target != "report" || edgeID != "H14" {
		t.Errorf("review-overturn: got target=%s edge=%s", target, edgeID)
	}
}

func TestEdge_ReportToDone(t *testing.T) {
	runner := buildTestRunner(t)
	ws := framework.NewWalkerState("10")

	target, _ := evaluateEdge(runner, "report", nil, ws)
	if target != "DONE" {
		t.Errorf("report->done: got target=%s", target)
	}
}
