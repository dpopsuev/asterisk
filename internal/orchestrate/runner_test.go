package orchestrate

import (
	"os"
	"path/filepath"
	"testing"

	"asterisk/adapters/store"
)

// TestRunnerFullPipeline exercises the complete F0→F6 pipeline with mock artifacts.
// It verifies heuristic routing, state transitions, and store side effects.
func TestRunnerFullPipeline(t *testing.T) {
	// Set up temp dirs and MemStore
	tmpDir := t.TempDir()
	testBasePath := filepath.Join(tmpDir, "investigations")

	st := store.NewMemStore()

	// Create scaffolding in store
	suiteID, _ := st.CreateSuite(&store.InvestigationSuite{Name: "test suite"})
	vID, _ := st.CreateVersion(&store.Version{Label: "4.21"})
	pID, _ := st.CreatePipeline(&store.Pipeline{SuiteID: suiteID, VersionID: vID, Name: "test pipeline"})
	lID, _ := st.CreateLaunch(&store.Launch{PipelineID: pID, RPLaunchID: 33195, Name: "launch"})
	jID, _ := st.CreateJob(&store.Job{LaunchID: lID, RPItemID: 100, Name: "test job"})
	caseID, _ := st.CreateCaseV2(&store.Case{
		JobID:        jID,
		LaunchID:     lID,
		RPItemID:     200,
		Name:         "[T-TSC] PTP Recovery test",
		ErrorMessage: "Expected 0 to equal 1",
		Status:       "open",
	})

	caseData, _ := st.GetCaseV2(caseID)
	if caseData == nil {
		t.Fatal("case not found")
	}

	// Set up prompt templates dir with minimal templates
	promptDir := filepath.Join(tmpDir, "prompts")
	for _, sub := range []string{"recall", "triage", "resolve", "investigate", "correlate", "review", "report"} {
		if err := os.MkdirAll(filepath.Join(promptDir, sub), 0755); err != nil {
			t.Fatal(err)
		}
	}
	writeTmpl := func(path, content string) {
		if err := os.WriteFile(filepath.Join(promptDir, path), []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	writeTmpl("recall/judge-similarity.md", "# Recall\nCase: #{{.CaseID}}\nTest: {{.Failure.TestName}}")
	writeTmpl("triage/classify-symptoms.md", "# Triage\nCase: #{{.CaseID}}")
	writeTmpl("resolve/select-repo.md", "# Resolve\nCase: #{{.CaseID}}")
	writeTmpl("investigate/deep-rca.md", "# Investigate\nCase: #{{.CaseID}}")
	writeTmpl("correlate/match-cases.md", "# Correlate\nCase: #{{.CaseID}}")
	writeTmpl("review/present-findings.md", "# Review\nCase: #{{.CaseID}}")
	writeTmpl("report/regression-table.md", "# Report\nCase: #{{.CaseID}}")

	cfg := RunnerConfig{
		PromptDir:  promptDir,
		Thresholds: DefaultThresholds(),
		BasePath:   testBasePath,
	}

	// Step 1: RunStep should produce F0 prompt
	result, err := RunStep(st, caseData, nil, nil, cfg)
	if err != nil {
		t.Fatalf("RunStep F0: %v", err)
	}
	if result.IsDone {
		t.Fatal("pipeline should not be done yet")
	}
	if result.NextStep != StepF0Recall {
		t.Errorf("expected F0_RECALL, got %s", result.NextStep)
	}
	if result.PromptPath == "" {
		t.Error("expected prompt path")
	}

	// Verify state file exists
	caseDir := CaseDir(testBasePath, suiteID, caseID)
	state, err := LoadState(caseDir)
	if err != nil {
		t.Fatalf("LoadState: %v", err)
	}
	if state == nil || state.CurrentStep != StepF0Recall || state.Status != "paused" {
		t.Errorf("unexpected state: %+v", state)
	}

	// Step 2: Simulate user completing F0 with a recall miss
	recallResult := &RecallResult{Match: false, Confidence: 0.1, Reasoning: "no match"}
	if err := WriteArtifact(caseDir, ArtifactFilename(StepF0Recall), recallResult); err != nil {
		t.Fatal(err)
	}

	// RunStep again should advance past F0 and generate F1 prompt
	result, err = RunStep(st, caseData, nil, nil, cfg)
	if err != nil {
		t.Fatalf("RunStep F1: %v", err)
	}
	if result.NextStep != StepF1Triage {
		t.Errorf("expected F1_TRIAGE, got %s", result.NextStep)
	}

	// Step 3: Simulate triage with investigation needed
	triageResult := &TriageResult{
		SymptomCategory:      "assertion",
		DefectTypeHypothesis: "pb001",
		CandidateRepos:       []string{"ptp-operator", "cnf-gotests"},
		SkipInvestigation:    false,
	}
	if err := WriteArtifact(caseDir, ArtifactFilename(StepF1Triage), triageResult); err != nil {
		t.Fatal(err)
	}

	// After triage, H6 now routes directly to F2_RESOLVE.
	result, err = RunStep(st, caseData, nil, nil, cfg)
	if err != nil {
		t.Fatalf("RunStep F2 (after triage): %v", err)
	}
	if result.NextStep != StepF2Resolve {
		t.Errorf("expected F2_RESOLVE, got %s", result.NextStep)
	}

	// Verify triage side effects
	caseData, _ = st.GetCaseV2(caseID)
	if caseData.Status != "triaged" {
		t.Errorf("expected case status 'triaged', got %q", caseData.Status)
	}

	// Step 4: Simulate resolve
	resolveResult := &ResolveResult{
		SelectedRepos: []RepoSelection{{Name: "ptp-operator", Path: "/repo", Reason: "main suspect"}},
	}
	if err := WriteArtifact(caseDir, ArtifactFilename(StepF2Resolve), resolveResult); err != nil {
		t.Fatal(err)
	}

	result, err = RunStep(st, caseData, nil, nil, cfg)
	if err != nil {
		t.Fatalf("RunStep F3: %v", err)
	}
	if result.NextStep != StepF3Invest {
		t.Errorf("expected F3_INVESTIGATE, got %s", result.NextStep)
	}

	// Step 5: Simulate investigation with good convergence
	investigateResult := &InvestigateArtifact{
		LaunchID:         "33195",
		CaseIDs:          []int{200},
		RCAMessage:       "Holdover timeout reduced from 300s to 60s",
		DefectType:       "pb001",
		ConvergenceScore: 0.85,
		EvidenceRefs:     []string{"pkg/daemon/config.go:42"},
	}
	if err := WriteArtifact(caseDir, ArtifactFilename(StepF3Invest), investigateResult); err != nil {
		t.Fatal(err)
	}

	result, err = RunStep(st, caseData, nil, nil, cfg)
	if err != nil {
		t.Fatalf("RunStep F4: %v", err)
	}
	if result.NextStep != StepF4Correlate {
		t.Errorf("expected F4_CORRELATE, got %s", result.NextStep)
	}

	// Verify investigate side effects
	caseData, _ = st.GetCaseV2(caseID)
	if caseData.Status != "investigated" {
		t.Errorf("expected case status 'investigated', got %q", caseData.Status)
	}
	if caseData.RCAID == 0 {
		t.Error("expected case to be linked to RCA")
	}

	// Step 6: Simulate correlate (no duplicate)
	correlateResult := &CorrelateResult{
		IsDuplicate: false, Confidence: 0.2, Reasoning: "unique failure",
	}
	if err := WriteArtifact(caseDir, ArtifactFilename(StepF4Correlate), correlateResult); err != nil {
		t.Fatal(err)
	}

	result, err = RunStep(st, caseData, nil, nil, cfg)
	if err != nil {
		t.Fatalf("RunStep F5: %v", err)
	}
	if result.NextStep != StepF5Review {
		t.Errorf("expected F5_REVIEW, got %s", result.NextStep)
	}

	// Step 7: Simulate review approval
	reviewDecision := &ReviewDecision{Decision: "approve"}
	if err := WriteArtifact(caseDir, ArtifactFilename(StepF5Review), reviewDecision); err != nil {
		t.Fatal(err)
	}

	result, err = RunStep(st, caseData, nil, nil, cfg)
	if err != nil {
		t.Fatalf("RunStep F6: %v", err)
	}
	if result.NextStep != StepF6Report {
		t.Errorf("expected F6_REPORT, got %s", result.NextStep)
	}

	// Verify review side effects
	caseData, _ = st.GetCaseV2(caseID)
	if caseData.Status != "reviewed" {
		t.Errorf("expected case status 'reviewed', got %q", caseData.Status)
	}

	// Step 8: Simulate report completion
	jiraDraft := map[string]any{
		"summary":     "PTP holdover timeout issue",
		"defect_type": "pb001",
	}
	if err := WriteArtifact(caseDir, ArtifactFilename(StepF6Report), jiraDraft); err != nil {
		t.Fatal(err)
	}

	result, err = RunStep(st, caseData, nil, nil, cfg)
	if err != nil {
		t.Fatalf("RunStep DONE: %v", err)
	}
	if !result.IsDone {
		t.Error("expected pipeline to be done")
	}

	// Verify final state
	state, _ = LoadState(caseDir)
	if state.Status != "done" || state.CurrentStep != StepDone {
		t.Errorf("final state: step=%s status=%s", state.CurrentStep, state.Status)
	}
	if len(state.History) < 7 {
		t.Errorf("expected at least 7 history entries, got %d", len(state.History))
	}
}

// TestRunnerRecallHitShortCircuit tests the F0→F5→F6 short-circuit path.
func TestRunnerRecallHitShortCircuit(t *testing.T) {
	tmpDir := t.TempDir()
	testBasePath := filepath.Join(tmpDir, "investigations")

	st := store.NewMemStore()
	suiteID, _ := st.CreateSuite(&store.InvestigationSuite{Name: "test"})
	vID, _ := st.CreateVersion(&store.Version{Label: "4.21"})
	pID, _ := st.CreatePipeline(&store.Pipeline{SuiteID: suiteID, VersionID: vID, Name: "p"})
	lID, _ := st.CreateLaunch(&store.Launch{PipelineID: pID, RPLaunchID: 1})
	jID, _ := st.CreateJob(&store.Job{LaunchID: lID, RPItemID: 1})
	caseID, _ := st.CreateCaseV2(&store.Case{JobID: jID, LaunchID: lID, RPItemID: 2, Name: "test", Status: "open"})
	caseData, _ := st.GetCaseV2(caseID)

	promptDir := filepath.Join(tmpDir, "prompts")
	for _, sub := range []string{"recall", "review", "report"} {
		os.MkdirAll(filepath.Join(promptDir, sub), 0755)
	}
	write := func(path, content string) {
		os.WriteFile(filepath.Join(promptDir, path), []byte(content), 0644)
	}
	write("recall/judge-similarity.md", "# Recall {{.CaseID}}")
	write("review/present-findings.md", "# Review {{.CaseID}}")
	write("report/regression-table.md", "# Report {{.CaseID}}")

	cfg := RunnerConfig{PromptDir: promptDir, Thresholds: DefaultThresholds(), BasePath: testBasePath}

	// F0 prompt
	result, err := RunStep(st, caseData, nil, nil, cfg)
	if err != nil {
		t.Fatalf("RunStep F0: %v", err)
	}
	if result.NextStep != StepF0Recall {
		t.Errorf("expected F0, got %s", result.NextStep)
	}

	// Recall hit → should skip to F5
	caseDir := CaseDir(testBasePath, suiteID, caseID)
	WriteArtifact(caseDir, ArtifactFilename(StepF0Recall), &RecallResult{
		Match: true, PriorRCAID: 42, Confidence: 0.92, Reasoning: "exact match",
	})

	result, err = RunStep(st, caseData, nil, nil, cfg)
	if err != nil {
		t.Fatalf("RunStep after recall: %v", err)
	}
	if result.NextStep != StepF5Review {
		t.Errorf("expected F5_REVIEW (short-circuit), got %s", result.NextStep)
	}
}

// TestRunnerInvestigateLoop tests the F3→F2→F3 low-confidence loop.
func TestRunnerInvestigateLoop(t *testing.T) {
	tmpDir := t.TempDir()
	testBasePath := filepath.Join(tmpDir, "investigations")

	st := store.NewMemStore()
	suiteID, _ := st.CreateSuite(&store.InvestigationSuite{Name: "test"})
	vID, _ := st.CreateVersion(&store.Version{Label: "4.21"})
	pID, _ := st.CreatePipeline(&store.Pipeline{SuiteID: suiteID, VersionID: vID, Name: "p"})
	lID, _ := st.CreateLaunch(&store.Launch{PipelineID: pID, RPLaunchID: 1})
	jID, _ := st.CreateJob(&store.Job{LaunchID: lID, RPItemID: 1})
	caseID, _ := st.CreateCaseV2(&store.Case{JobID: jID, LaunchID: lID, RPItemID: 2, Name: "test", Status: "open"})
	caseData, _ := st.GetCaseV2(caseID)

	promptDir := filepath.Join(tmpDir, "prompts")
	for _, sub := range []string{"recall", "triage", "resolve", "investigate", "review"} {
		os.MkdirAll(filepath.Join(promptDir, sub), 0755)
	}
	write := func(path, content string) {
		os.WriteFile(filepath.Join(promptDir, path), []byte(content), 0644)
	}
	write("recall/judge-similarity.md", "# Recall {{.CaseID}}")
	write("triage/classify-symptoms.md", "# Triage {{.CaseID}}")
	write("resolve/select-repo.md", "# Resolve {{.CaseID}}")
	write("investigate/deep-rca.md", "# Investigate {{.CaseID}}")
	write("review/present-findings.md", "# Review {{.CaseID}}")

	cfg := RunnerConfig{PromptDir: promptDir, Thresholds: DefaultThresholds(), BasePath: testBasePath}
	caseDir := CaseDir(testBasePath, suiteID, caseID)

	// Advance to F3: set up F0 miss, F1 investigate, F2 result
	RunStep(st, caseData, nil, nil, cfg) // F0 prompt
	WriteArtifact(caseDir, ArtifactFilename(StepF0Recall), &RecallResult{Match: false})
	RunStep(st, caseData, nil, nil, cfg) // F1 prompt
	WriteArtifact(caseDir, ArtifactFilename(StepF1Triage), &TriageResult{
		SymptomCategory: "assertion", CandidateRepos: []string{"a", "b"}, SkipInvestigation: false,
	})
	RunStep(st, caseData, nil, nil, cfg) // F2 prompt
	WriteArtifact(caseDir, ArtifactFilename(StepF2Resolve), &ResolveResult{
		SelectedRepos: []RepoSelection{{Name: "a", Reason: "primary"}},
	})
	result, _ := RunStep(st, caseData, nil, nil, cfg) // F3 prompt
	if result.NextStep != StepF3Invest {
		t.Fatalf("expected F3, got %s", result.NextStep)
	}

	// F3 with low convergence → should loop back to F2.
	// The runner will read the F3 artifact, evaluate heuristics, and loop back.
	// Remove the old F2 resolve-result so the runner doesn't re-read it in the loop.
	os.Remove(filepath.Join(caseDir, ArtifactFilename(StepF2Resolve)))

	WriteArtifact(caseDir, ArtifactFilename(StepF3Invest), &InvestigateArtifact{
		ConvergenceScore: 0.4, RCAMessage: "uncertain", DefectType: "ti001",
		EvidenceRefs: []string{"partial-evidence"},
	})

	result, err := RunStep(st, caseData, nil, nil, cfg)
	if err != nil {
		t.Fatalf("RunStep loop: %v", err)
	}
	if result.NextStep != StepF2Resolve {
		t.Errorf("expected F2_RESOLVE (loop), got %s", result.NextStep)
	}

	// Check loop counter
	state, _ := LoadState(caseDir)
	if state.LoopCounts["investigate"] != 1 {
		t.Errorf("expected investigate loop count 1, got %d", state.LoopCounts["investigate"])
	}
}

func TestComputeFingerprint(t *testing.T) {
	fp1 := ComputeFingerprint("test1", "error1", "comp1")
	fp2 := ComputeFingerprint("test1", "error1", "comp1")
	fp3 := ComputeFingerprint("test2", "error1", "comp1")

	if fp1 != fp2 {
		t.Error("same inputs should produce same fingerprint")
	}
	if fp1 == fp3 {
		t.Error("different inputs should produce different fingerprints")
	}
	if len(fp1) != 16 {
		t.Errorf("fingerprint length: got %d want 16", len(fp1))
	}
}
