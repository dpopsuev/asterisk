package metacalmcp

import (
	"sync"
	"testing"

	"asterisk/pkg/framework/metacal"
)

func validResponse(model, provider, version string) string {
	return `{"model_name": "` + model + `", "provider": "` + provider + `", "version": "` + version + `"}` + "\n" +
		"```go\nfunc improved(nums []int, label string, verbose bool) (int, string, error) {\n" +
		"\t// sum absolute values\n\tvar total int\n\tfor _, n := range nums {\n" +
		"\t\tif n > 0 { total += n } else if n < 0 { total -= n }\n\t}\n" +
		"\tif total == 0 { return 0, \"\", fmt.Errorf(\"empty result for %s\", label) }\n" +
		"\treturn total, \"\", nil\n}\n```"
}

// --- Session lifecycle ---

func TestSession_NewSession_InitialState(t *testing.T) {
	s := NewSession(metacal.DefaultConfig())

	if s.GetState() != StateRunning {
		t.Errorf("new session state = %v, want running", s.GetState())
	}
	if s.UniqueCount() != 0 {
		t.Errorf("new session unique count = %d, want 0", s.UniqueCount())
	}
	if s.ID == "" {
		t.Error("session ID is empty")
	}
	if s.Bus == nil {
		t.Error("session Bus is nil")
	}
	if s.Bus.Len() < 1 {
		t.Error("expected at least session_started signal")
	}
}

func TestSession_NewSession_ZeroConfig_UsesDefaults(t *testing.T) {
	s := NewSession(metacal.DiscoveryConfig{})

	if s.Config.MaxIterations != 15 {
		t.Errorf("default MaxIterations = %d, want 15", s.Config.MaxIterations)
	}
	if s.Config.ProbeID != "refactor-v1" {
		t.Errorf("default ProbeID = %q, want refactor-v1", s.Config.ProbeID)
	}
}

func TestSession_NextPrompt_ReturnsPromptOnFirstCall(t *testing.T) {
	s := NewSession(metacal.DefaultConfig())

	prompt, done := s.NextPrompt()
	if done {
		t.Error("first NextPrompt should not be done")
	}
	if prompt == "" {
		t.Error("first prompt should not be empty")
	}
}

func TestSession_SubmitResponse_AdvancesIteration(t *testing.T) {
	s := NewSession(metacal.DefaultConfig())

	result, repeated, err := s.SubmitResponse(validResponse("model-a", "ProvA", "1.0"))
	if err != nil {
		t.Fatalf("SubmitResponse: %v", err)
	}
	if repeated {
		t.Error("first submission should not be repeated")
	}
	if result.Model.ModelName != "model-a" {
		t.Errorf("model name = %q, want model-a", result.Model.ModelName)
	}
	if s.UniqueCount() != 1 {
		t.Errorf("unique count = %d, want 1", s.UniqueCount())
	}
}

// --- Submit to done session ---

func TestSession_SubmitResponse_AfterFinalize_ReturnsError(t *testing.T) {
	s := NewSession(metacal.DefaultConfig())
	s.Finalize("test")

	_, _, err := s.SubmitResponse(validResponse("model-a", "ProvA", "1.0"))
	if err == nil {
		t.Fatal("expected error submitting to finalized session")
	}
}

func TestSession_SubmitResponse_AfterMaxIterations_Done(t *testing.T) {
	s := NewSession(metacal.DiscoveryConfig{
		MaxIterations:     2,
		ProbeID:           "refactor-v1",
		TerminateOnRepeat: true,
	})

	s.SubmitResponse(validResponse("model-a", "ProvA", "1.0"))
	s.SubmitResponse(validResponse("model-b", "ProvB", "2.0"))

	_, done := s.NextPrompt()
	if !done {
		t.Error("expected done after max_iterations reached")
	}
	if s.GetState() != StateDone {
		t.Errorf("state = %v, want done", s.GetState())
	}
}

// --- Repeat detection ---

func TestSession_SubmitResponse_RepeatTerminates(t *testing.T) {
	s := NewSession(metacal.DiscoveryConfig{
		MaxIterations:     10,
		ProbeID:           "refactor-v1",
		TerminateOnRepeat: true,
	})

	s.SubmitResponse(validResponse("model-a", "ProvA", "1.0"))
	_, repeated, err := s.SubmitResponse(validResponse("model-a", "ProvA", "1.0"))
	if err != nil {
		t.Fatalf("SubmitResponse: %v", err)
	}
	if !repeated {
		t.Error("expected repeat to be detected")
	}
	if s.GetState() != StateDone {
		t.Errorf("state = %v, want done after repeat with TerminateOnRepeat", s.GetState())
	}
}

func TestSession_SubmitResponse_RepeatWithoutTerminate_ContinuesRunning(t *testing.T) {
	s := NewSession(metacal.DiscoveryConfig{
		MaxIterations:     10,
		ProbeID:           "refactor-v1",
		TerminateOnRepeat: false,
	})

	s.SubmitResponse(validResponse("model-a", "ProvA", "1.0"))
	_, repeated, _ := s.SubmitResponse(validResponse("model-a", "ProvA", "1.0"))

	if !repeated {
		t.Error("should detect repeat even without termination")
	}
	if s.GetState() != StateRunning {
		t.Errorf("state = %v, want running (TerminateOnRepeat=false)", s.GetState())
	}
}

// --- Double finalize is idempotent ---

func TestSession_Finalize_Idempotent(t *testing.T) {
	s := NewSession(metacal.DefaultConfig())
	s.SubmitResponse(validResponse("model-a", "ProvA", "1.0"))

	r1 := s.Finalize("reason1")
	r2 := s.Finalize("reason2")

	if r1 != r2 {
		t.Error("double finalize should return the same report pointer")
	}
	if r1.TermReason != "reason1" {
		t.Errorf("term reason = %q, want reason1 (first finalize wins)", r1.TermReason)
	}
}

// --- Parse errors ---

func TestSession_SubmitResponse_GarbageInput_ReturnsError(t *testing.T) {
	s := NewSession(metacal.DefaultConfig())

	_, _, err := s.SubmitResponse("just some random text without JSON")
	if err == nil {
		t.Fatal("expected error for unparseable response")
	}
}

func TestSession_SubmitResponse_IdentityButNoCode_ReturnsError(t *testing.T) {
	s := NewSession(metacal.DefaultConfig())

	raw := `{"model_name": "model-a", "provider": "ProvA", "version": "1.0"}
I'm going to explain the code instead of refactoring it...`

	_, _, err := s.SubmitResponse(raw)
	if err == nil {
		t.Fatal("expected error when code block is missing")
	}
}

// --- Report contents ---

func TestSession_GetReport_NilBeforeFinalize(t *testing.T) {
	s := NewSession(metacal.DefaultConfig())

	if s.GetReport() != nil {
		t.Error("report should be nil before finalize")
	}
}

func TestSession_Finalize_ReportContainsAllModels(t *testing.T) {
	s := NewSession(metacal.DefaultConfig())
	s.SubmitResponse(validResponse("model-a", "ProvA", "1.0"))
	s.SubmitResponse(validResponse("model-b", "ProvB", "2.0"))
	s.SubmitResponse(validResponse("model-c", "ProvC", "3.0"))

	report := s.Finalize("done")

	if len(report.Results) != 3 {
		t.Errorf("report results = %d, want 3", len(report.Results))
	}
	if len(report.UniqueModels) != 3 {
		t.Errorf("report unique models = %d, want 3", len(report.UniqueModels))
	}
	if report.TermReason != "done" {
		t.Errorf("term reason = %q, want done", report.TermReason)
	}
	if report.RunID != s.ID {
		t.Errorf("run ID = %q, want %q", report.RunID, s.ID)
	}
}

func TestSession_ModelNames_PreservesOrder(t *testing.T) {
	s := NewSession(metacal.DefaultConfig())
	s.SubmitResponse(validResponse("charlie", "ProvC", "1"))
	s.SubmitResponse(validResponse("alpha", "ProvA", "1"))
	s.SubmitResponse(validResponse("bravo", "ProvB", "1"))

	names := s.ModelNames()
	if names != "charlie, alpha, bravo" {
		t.Errorf("model names = %q, want insertion order", names)
	}
}

// --- Concurrency ---

func TestSession_ConcurrentSubmit_NoRace(t *testing.T) {
	s := NewSession(metacal.DiscoveryConfig{
		MaxIterations:     100,
		ProbeID:           "refactor-v1",
		TerminateOnRepeat: false,
	})

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			resp := validResponse(
				"model-"+string(rune('a'+n%26)),
				"Prov",
				"1.0",
			)
			s.SubmitResponse(resp)
		}(i)
	}
	wg.Wait()

	if s.GetState() != StateRunning {
		t.Logf("state = %v (may be done if repeat detected)", s.GetState())
	}
	if s.UniqueCount() == 0 {
		t.Error("expected at least one unique model after concurrent submissions")
	}
}

func TestSession_ConcurrentPromptAndSubmit_NoRace(t *testing.T) {
	s := NewSession(metacal.DiscoveryConfig{
		MaxIterations:     100,
		ProbeID:           "refactor-v1",
		TerminateOnRepeat: false,
	})

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			s.NextPrompt()
		}()
		go func(n int) {
			defer wg.Done()
			s.SubmitResponse(validResponse("m-"+string(rune('a'+n%26)), "P", "1"))
		}(i)
	}
	wg.Wait()
}

func TestSession_ConcurrentFinalize_NoRace(t *testing.T) {
	s := NewSession(metacal.DefaultConfig())
	s.SubmitResponse(validResponse("model-a", "ProvA", "1.0"))

	var wg sync.WaitGroup
	reports := make([]*metacal.RunReport, 10)
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			reports[idx] = s.Finalize("concurrent")
		}(i)
	}
	wg.Wait()

	for i := 1; i < len(reports); i++ {
		if reports[i] != reports[0] {
			t.Fatal("concurrent finalize should return the same report pointer")
		}
	}
}

// --- Signal bus correctness ---

func TestSession_SignalBus_EmitsSessionStarted(t *testing.T) {
	s := NewSession(metacal.DefaultConfig())
	signals := s.Bus.Since(0)

	if len(signals) == 0 {
		t.Fatal("expected at least one signal")
	}
	if signals[0].Event != "session_started" {
		t.Errorf("first signal = %q, want session_started", signals[0].Event)
	}
}

func TestSession_SignalBus_EmitsModelDiscovered(t *testing.T) {
	s := NewSession(metacal.DefaultConfig())
	startLen := s.Bus.Len()

	s.SubmitResponse(validResponse("model-a", "ProvA", "1.0"))

	signals := s.Bus.Since(startLen)
	found := false
	for _, sig := range signals {
		if sig.Event == "model_discovered" {
			found = true
			if sig.Meta["model"] != "model-a" {
				t.Errorf("model_discovered meta model = %q", sig.Meta["model"])
			}
		}
	}
	if !found {
		t.Error("expected model_discovered signal after submit")
	}
}

func TestSession_SignalBus_EmitsModelRepeated(t *testing.T) {
	s := NewSession(metacal.DiscoveryConfig{
		MaxIterations:     10,
		ProbeID:           "refactor-v1",
		TerminateOnRepeat: true,
	})
	s.SubmitResponse(validResponse("model-a", "ProvA", "1.0"))
	beforeRepeat := s.Bus.Len()

	s.SubmitResponse(validResponse("model-a", "ProvA", "1.0"))

	signals := s.Bus.Since(beforeRepeat)
	found := false
	for _, sig := range signals {
		if sig.Event == "model_repeated" {
			found = true
		}
	}
	if !found {
		t.Error("expected model_repeated signal on repeat")
	}
}

func TestSession_SignalBus_EmitsSessionDone(t *testing.T) {
	s := NewSession(metacal.DefaultConfig())
	s.SubmitResponse(validResponse("model-a", "ProvA", "1.0"))
	s.Finalize("test")

	signals := s.Bus.Since(0)
	found := false
	for _, sig := range signals {
		if sig.Event == "session_done" {
			found = true
			if sig.Meta["term_reason"] != "test" {
				t.Errorf("session_done term_reason = %q", sig.Meta["term_reason"])
			}
		}
	}
	if !found {
		t.Error("expected session_done signal after finalize")
	}
}
