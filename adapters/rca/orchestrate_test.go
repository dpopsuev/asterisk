package rca

import (
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
		step PipelineStep
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

// --- State management tests ---

func TestStateInitAndAdvance(t *testing.T) {
	state := InitState(10, 1)
	if state.CurrentStep != StepInit || state.Status != "running" {
		t.Fatalf("InitState: %+v", state)
	}

	AdvanceStep(state, StepF0Recall, "INIT", "start")
	if state.CurrentStep != StepF0Recall || len(state.History) != 1 {
		t.Fatalf("after advance to F0: %+v", state)
	}
	if state.History[0].Step != StepInit {
		t.Errorf("history[0].Step: %s", state.History[0].Step)
	}

	AdvanceStep(state, StepDone, "H12", "approve")
	if state.Status != "done" {
		t.Errorf("status after done: %q", state.Status)
	}
}

func TestStatePersistence(t *testing.T) {
	dir := t.TempDir()
	caseDir := filepath.Join(dir, "1", "10")
	if err := os.MkdirAll(caseDir, 0755); err != nil {
		t.Fatal(err)
	}

	state := InitState(10, 1)
	AdvanceStep(state, StepF0Recall, "INIT", "start")
	state.LoopCounts["investigate"] = 1

	if err := SaveState(caseDir, state); err != nil {
		t.Fatalf("SaveState: %v", err)
	}

	loaded, err := LoadState(caseDir)
	if err != nil {
		t.Fatalf("LoadState: %v", err)
	}
	if loaded == nil {
		t.Fatal("LoadState returned nil")
	}
	if loaded.CurrentStep != StepF0Recall || loaded.CaseID != 10 || loaded.SuiteID != 1 {
		t.Errorf("loaded state mismatch: %+v", loaded)
	}
	if loaded.LoopCounts["investigate"] != 1 {
		t.Errorf("loaded loop count: %d", loaded.LoopCounts["investigate"])
	}

	// LoadState on empty dir = nil
	emptyDir := filepath.Join(dir, "empty")
	if err := os.MkdirAll(emptyDir, 0755); err != nil {
		t.Fatal(err)
	}
	empty, err := LoadState(emptyDir)
	if err != nil {
		t.Fatalf("LoadState empty: %v", err)
	}
	if empty != nil {
		t.Errorf("expected nil for empty dir, got %+v", empty)
	}
}

func TestLoopCounting(t *testing.T) {
	state := InitState(10, 1)

	// Not exhausted initially
	if IsLoopExhausted(state, "investigate", 2) {
		t.Error("should not be exhausted at 0")
	}

	IncrementLoop(state, "investigate")
	if IsLoopExhausted(state, "investigate", 2) {
		t.Error("should not be exhausted at 1")
	}

	IncrementLoop(state, "investigate")
	if !IsLoopExhausted(state, "investigate", 2) {
		t.Error("should be exhausted at 2")
	}
}

// --- Graph-edge evaluation tests ---

// buildTestRunner creates a shared runner for all edge evaluation tests.
func buildTestRunner(t *testing.T) *framework.Runner {
	t.Helper()
	runner, err := BuildRunner(DefaultThresholds())
	if err != nil {
		t.Fatalf("BuildRunner: %v", err)
	}
	return runner
}

func TestEdge_RecallHit(t *testing.T) {
	runner := buildTestRunner(t)
	state := InitState(10, 1)

	recall := &RecallResult{Match: true, PriorRCAID: 5, Confidence: 0.9}
	action, edgeID := EvaluateGraphEdge(runner, StepF0Recall, recall, state)
	if action.NextStep != StepF5Review || edgeID != "H1" {
		t.Errorf("recall-hit: got step=%s edge=%s", action.NextStep, edgeID)
	}
}

func TestEdge_RecallMiss(t *testing.T) {
	runner := buildTestRunner(t)
	state := InitState(10, 1)

	recall := &RecallResult{Match: false, Confidence: 0}
	action, edgeID := EvaluateGraphEdge(runner, StepF0Recall, recall, state)
	if action.NextStep != StepF1Triage || edgeID != "H2" {
		t.Errorf("recall-miss: got step=%s edge=%s", action.NextStep, edgeID)
	}
}

func TestEdge_RecallUncertain(t *testing.T) {
	runner := buildTestRunner(t)
	state := InitState(10, 1)

	recall := &RecallResult{Match: true, PriorRCAID: 5, Confidence: 0.6}
	action, edgeID := EvaluateGraphEdge(runner, StepF0Recall, recall, state)
	if action.NextStep != StepF1Triage || edgeID != "H3" {
		t.Errorf("recall-uncertain: got step=%s edge=%s", action.NextStep, edgeID)
	}
}

func TestEdge_TriageSkipInfra(t *testing.T) {
	runner := buildTestRunner(t)
	state := InitState(10, 1)

	triage := &TriageResult{SymptomCategory: "infra", SkipInvestigation: true}
	action, edgeID := EvaluateGraphEdge(runner, StepF1Triage, triage, state)
	if action.NextStep != StepF5Review || edgeID != "H4" {
		t.Errorf("triage-skip-infra: got step=%s edge=%s", action.NextStep, edgeID)
	}
}

func TestEdge_TriageInvestigate(t *testing.T) {
	runner := buildTestRunner(t)
	state := InitState(10, 1)

	triage := &TriageResult{SymptomCategory: "assertion", SkipInvestigation: false, CandidateRepos: []string{"repo-a", "repo-b"}}
	action, edgeID := EvaluateGraphEdge(runner, StepF1Triage, triage, state)
	if action.NextStep != StepF2Resolve || edgeID != "H6" {
		t.Errorf("triage-investigate: got step=%s edge=%s", action.NextStep, edgeID)
	}
}

func TestEdge_TriageSingleRepo(t *testing.T) {
	runner := buildTestRunner(t)
	state := InitState(10, 1)

	triage := &TriageResult{SymptomCategory: "assertion", SkipInvestigation: false, CandidateRepos: []string{"repo-a"}}
	action, edgeID := EvaluateGraphEdge(runner, StepF1Triage, triage, state)
	if action.NextStep != StepF3Invest || edgeID != "H7" {
		t.Errorf("triage-single-repo: got step=%s edge=%s", action.NextStep, edgeID)
	}
}

func TestEdge_InvestigateConverged(t *testing.T) {
	runner := buildTestRunner(t)
	state := InitState(10, 1)

	artifact := &InvestigateArtifact{ConvergenceScore: 0.85}
	action, edgeID := EvaluateGraphEdge(runner, StepF3Invest, artifact, state)
	if action.NextStep != StepF4Correlate || edgeID != "H9" {
		t.Errorf("investigate-converged: got step=%s edge=%s", action.NextStep, edgeID)
	}
}

func TestEdge_InvestigateLowLoop(t *testing.T) {
	runner := buildTestRunner(t)
	state := InitState(10, 1)

	artifact := &InvestigateArtifact{ConvergenceScore: 0.40, EvidenceRefs: []string{"some-evidence"}}
	action, edgeID := EvaluateGraphEdge(runner, StepF3Invest, artifact, state)
	if action.NextStep != StepF2Resolve || edgeID != "H10" {
		t.Errorf("investigate-low: got step=%s edge=%s", action.NextStep, edgeID)
	}
}

func TestEdge_InvestigateExhausted(t *testing.T) {
	runner := buildTestRunner(t)
	state := InitState(10, 1)
	state.LoopCounts["investigate"] = 1 // exhausted (MaxInvestigateLoops=1)

	artifact := &InvestigateArtifact{ConvergenceScore: 0.40, EvidenceRefs: []string{"some-evidence"}}
	action, edgeID := EvaluateGraphEdge(runner, StepF3Invest, artifact, state)
	if action.NextStep != StepF5Review || edgeID != "H11" {
		t.Errorf("investigate-exhausted: got step=%s edge=%s", action.NextStep, edgeID)
	}
}

func TestEdge_ReviewApprove(t *testing.T) {
	runner := buildTestRunner(t)
	state := InitState(10, 1)

	review := &ReviewDecision{Decision: "approve"}
	action, edgeID := EvaluateGraphEdge(runner, StepF5Review, review, state)
	if action.NextStep != StepF6Report || edgeID != "H12" {
		t.Errorf("review-approve: got step=%s edge=%s", action.NextStep, edgeID)
	}
}

func TestEdge_ReviewReassess(t *testing.T) {
	runner := buildTestRunner(t)
	state := InitState(10, 1)

	review := &ReviewDecision{Decision: "reassess", LoopTarget: StepF3Invest}
	action, edgeID := EvaluateGraphEdge(runner, StepF5Review, review, state)
	// YAML H13 always loops back to resolve (which leads to investigate)
	if action.NextStep != StepF2Resolve || edgeID != "H13" {
		t.Errorf("review-reassess: got step=%s edge=%s", action.NextStep, edgeID)
	}
}

func TestEdge_ReviewOverturn(t *testing.T) {
	runner := buildTestRunner(t)
	state := InitState(10, 1)

	review := &ReviewDecision{
		Decision:      "overturn",
		HumanOverride: &HumanOverride{DefectType: "pb001", RCAMessage: "human says this"},
	}
	action, edgeID := EvaluateGraphEdge(runner, StepF5Review, review, state)
	if action.NextStep != StepF6Report || edgeID != "H14" {
		t.Errorf("review-overturn: got step=%s edge=%s", action.NextStep, edgeID)
	}
}

func TestEdge_DefaultFallback(t *testing.T) {
	runner := buildTestRunner(t)
	state := InitState(10, 1)

	action, edgeID := EvaluateGraphEdge(runner, StepF6Report, nil, state)
	if action.NextStep != StepDone || edgeID != "FALLBACK" {
		t.Errorf("f6-fallback: got step=%s edge=%s", action.NextStep, edgeID)
	}
}
