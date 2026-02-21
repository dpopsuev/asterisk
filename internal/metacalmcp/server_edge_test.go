package metacalmcp_test

import (
	"context"
	"strings"
	"testing"

	_ "asterisk/internal/metacalmcp"
	"asterisk/pkg/framework/metacal"
)

// --- TerminateOnRepeat default bug ---

func TestServer_StartDiscovery_DefaultTerminateOnRepeat(t *testing.T) {
	srv := newTestServer(t)
	ctx := context.Background()
	session := connectInMemory(t, ctx, srv)

	// Start without specifying terminate_on_repeat — should default to true
	startResult := callTool(t, ctx, session, "start_discovery", map[string]any{
		"max_iterations": 10,
	})
	sessionID := startResult["session_id"].(string)

	// Discover one model
	callTool(t, ctx, session, "get_discovery_prompt", map[string]any{"session_id": sessionID})
	callTool(t, ctx, session, "submit_discovery_response", map[string]any{
		"session_id": sessionID,
		"response":   mockResponse("model-a", "ProviderA", "1.0"),
	})

	// Submit a repeat — should terminate because default is true
	callTool(t, ctx, session, "get_discovery_prompt", map[string]any{"session_id": sessionID})
	result := callTool(t, ctx, session, "submit_discovery_response", map[string]any{
		"session_id": sessionID,
		"response":   mockResponse("model-a", "ProviderA", "1.0"),
	})
	if result["repeated"] != true {
		t.Fatal("expected repeat to be detected")
	}
	if result["done"] != true {
		t.Fatal("default terminate_on_repeat should be true — session should be done on repeat")
	}
}

// --- Lifecycle: double start ---

func TestServer_DoubleStart_ErrorWhileRunning(t *testing.T) {
	srv := newTestServer(t)
	ctx := context.Background()
	session := connectInMemory(t, ctx, srv)

	callTool(t, ctx, session, "start_discovery", map[string]any{})

	errMsg := callToolExpectError(t, ctx, session, "start_discovery", map[string]any{})
	if errMsg == "" {
		t.Fatal("expected error when starting a second session while one is running")
	}
}

// --- Lifecycle: restart after done ---

func TestServer_StartAfterDone_Succeeds(t *testing.T) {
	srv := newTestServer(t)
	ctx := context.Background()
	session := connectInMemory(t, ctx, srv)

	// First session
	start1 := callTool(t, ctx, session, "start_discovery", map[string]any{
		"max_iterations": 1,
	})
	id1 := start1["session_id"].(string)

	callTool(t, ctx, session, "get_discovery_prompt", map[string]any{"session_id": id1})
	callTool(t, ctx, session, "submit_discovery_response", map[string]any{
		"session_id": id1,
		"response":   mockResponse("model-a", "ProvA", "1.0"),
	})
	// Session is done (max_iterations=1)
	callTool(t, ctx, session, "get_discovery_prompt", map[string]any{"session_id": id1})

	// Second session should succeed
	start2 := callTool(t, ctx, session, "start_discovery", map[string]any{})
	id2 := start2["session_id"].(string)
	if id2 == id1 {
		t.Error("new session should have a different ID")
	}
}

// --- Lifecycle: session ID mismatch ---

func TestServer_SessionIDMismatch_Error(t *testing.T) {
	srv := newTestServer(t)
	ctx := context.Background()
	session := connectInMemory(t, ctx, srv)

	callTool(t, ctx, session, "start_discovery", map[string]any{})

	errMsg := callToolExpectError(t, ctx, session, "get_discovery_prompt", map[string]any{
		"session_id": "wrong-id",
	})
	if errMsg == "" {
		t.Fatal("expected error for session ID mismatch")
	}
}

// --- Lifecycle: shutdown cleanup ---

func TestServer_Shutdown_FinalizesRunningSession(t *testing.T) {
	srv := newTestServer(t)
	ctx := context.Background()
	session := connectInMemory(t, ctx, srv)

	startResult := callTool(t, ctx, session, "start_discovery", map[string]any{})
	sessionID := startResult["session_id"].(string)

	callTool(t, ctx, session, "get_discovery_prompt", map[string]any{"session_id": sessionID})
	callTool(t, ctx, session, "submit_discovery_response", map[string]any{
		"session_id": sessionID,
		"response":   mockResponse("model-a", "ProvA", "1.0"),
	})

	srv.Shutdown()

	if srv.SessionID() != "" {
		t.Error("shutdown should clear the session")
	}
}

func TestServer_Shutdown_NoSession_NoOp(t *testing.T) {
	srv := newTestServer(t)
	srv.Shutdown() // should not panic
}

// --- Edge: submit empty response ---

func TestServer_SubmitEmptyResponse_Error(t *testing.T) {
	srv := newTestServer(t)
	ctx := context.Background()
	session := connectInMemory(t, ctx, srv)

	startResult := callTool(t, ctx, session, "start_discovery", map[string]any{})
	sessionID := startResult["session_id"].(string)

	errMsg := callToolExpectError(t, ctx, session, "submit_discovery_response", map[string]any{
		"session_id": sessionID,
		"response":   "",
	})
	if errMsg == "" {
		t.Fatal("expected error for empty response")
	}
}

// --- Edge: get report while running (should finalize) ---

func TestServer_GetReport_WhileRunning_Finalizes(t *testing.T) {
	srv := newTestServer(t)
	ctx := context.Background()
	session := connectInMemory(t, ctx, srv)

	startResult := callTool(t, ctx, session, "start_discovery", map[string]any{})
	sessionID := startResult["session_id"].(string)

	callTool(t, ctx, session, "get_discovery_prompt", map[string]any{"session_id": sessionID})
	callTool(t, ctx, session, "submit_discovery_response", map[string]any{
		"session_id": sessionID,
		"response":   mockResponse("model-a", "ProvA", "1.0"),
	})

	reportResult := callTool(t, ctx, session, "get_discovery_report", map[string]any{
		"session_id": sessionID,
	})
	if reportResult["status"] != "done" {
		t.Fatalf("expected status=done, got %v", reportResult["status"])
	}
	if reportResult["term_reason"] != "report_requested" {
		t.Fatalf("expected term_reason=report_requested, got %v", reportResult["term_reason"])
	}
}

// --- Edge: get report with no session ---

func TestServer_GetReport_NoSession_Error(t *testing.T) {
	srv := newTestServer(t)
	ctx := context.Background()
	session := connectInMemory(t, ctx, srv)

	errMsg := callToolExpectError(t, ctx, session, "get_discovery_report", map[string]any{
		"session_id": "nonexistent",
	})
	if errMsg == "" {
		t.Fatal("expected error for get_report with no session")
	}
}

// --- Edge: emit signal validation ---

func TestServer_EmitSignal_EmptyEvent_Error(t *testing.T) {
	srv := newTestServer(t)
	ctx := context.Background()
	session := connectInMemory(t, ctx, srv)

	startResult := callTool(t, ctx, session, "start_discovery", map[string]any{})
	sessionID := startResult["session_id"].(string)

	errMsg := callToolExpectError(t, ctx, session, "emit_signal", map[string]any{
		"session_id": sessionID,
		"event":      "",
		"agent":      "test",
	})
	if errMsg == "" {
		t.Fatal("expected error for empty event")
	}
}

func TestServer_EmitSignal_EmptyAgent_Error(t *testing.T) {
	srv := newTestServer(t)
	ctx := context.Background()
	session := connectInMemory(t, ctx, srv)

	startResult := callTool(t, ctx, session, "start_discovery", map[string]any{})
	sessionID := startResult["session_id"].(string)

	errMsg := callToolExpectError(t, ctx, session, "emit_signal", map[string]any{
		"session_id": sessionID,
		"event":      "test",
		"agent":      "",
	})
	if errMsg == "" {
		t.Fatal("expected error for empty agent")
	}
}

// --- Edge: get_signals with since beyond total ---

func TestServer_GetSignals_SinceBeyondTotal_EmptyResult(t *testing.T) {
	srv := newTestServer(t)
	ctx := context.Background()
	session := connectInMemory(t, ctx, srv)

	startResult := callTool(t, ctx, session, "start_discovery", map[string]any{})
	sessionID := startResult["session_id"].(string)

	result := callTool(t, ctx, session, "get_signals", map[string]any{
		"session_id": sessionID,
		"since":      9999,
	})
	signals := result["signals"]
	if signals != nil {
		t.Fatalf("expected nil signals for since > total, got %v", signals)
	}
}

// --- Wrapper identity rejection at MCP level ---

func TestServer_SubmitWrapper_ReturnsToolError(t *testing.T) {
	srv := newTestServer(t)
	ctx := context.Background()
	session := connectInMemory(t, ctx, srv)

	startResult := callTool(t, ctx, session, "start_discovery", map[string]any{})
	sessionID := startResult["session_id"].(string)

	callTool(t, ctx, session, "get_discovery_prompt", map[string]any{"session_id": sessionID})

	errMsg := callToolExpectError(t, ctx, session, "submit_discovery_response", map[string]any{
		"session_id": sessionID,
		"response":   mockResponse("Auto", "unknown", ""),
	})
	if errMsg == "" {
		t.Fatal("expected error for wrapper identity via MCP")
	}
	if !strings.Contains(errMsg, "wrapper identity rejected") {
		t.Errorf("error = %q, want 'wrapper identity rejected'", errMsg)
	}
}

func TestServer_SubmitWrapper_SessionContinues(t *testing.T) {
	srv := newTestServer(t)
	ctx := context.Background()
	session := connectInMemory(t, ctx, srv)

	startResult := callTool(t, ctx, session, "start_discovery", map[string]any{
		"max_iterations":      10,
		"terminate_on_repeat": false,
	})
	sessionID := startResult["session_id"].(string)

	// Submit wrapper — should be rejected
	callTool(t, ctx, session, "get_discovery_prompt", map[string]any{"session_id": sessionID})
	callToolExpectError(t, ctx, session, "submit_discovery_response", map[string]any{
		"session_id": sessionID,
		"response":   mockResponse("Cursor", "Cursor", "1.0"),
	})

	// Submit foundation — session should still accept it
	callTool(t, ctx, session, "get_discovery_prompt", map[string]any{"session_id": sessionID})
	result := callTool(t, ctx, session, "submit_discovery_response", map[string]any{
		"session_id": sessionID,
		"response":   mockResponse("gpt-4o", "OpenAI", "4o"),
	})
	if result["model_name"] != "gpt-4o" {
		t.Fatalf("expected gpt-4o, got %v", result["model_name"])
	}

	// Report should contain only the foundation model, not the wrapper
	reportResult := callTool(t, ctx, session, "get_discovery_report", map[string]any{
		"session_id": sessionID,
	})
	uniqueModels := reportResult["unique_models"].(float64)
	if uniqueModels != 1 {
		t.Fatalf("expected 1 unique model (no wrapper), got %v", uniqueModels)
	}
}

func TestServer_SubmitEXCLUDED_ReturnsToolError(t *testing.T) {
	srv := newTestServer(t)
	ctx := context.Background()
	session := connectInMemory(t, ctx, srv)

	startResult := callTool(t, ctx, session, "start_discovery", map[string]any{})
	sessionID := startResult["session_id"].(string)

	callTool(t, ctx, session, "get_discovery_prompt", map[string]any{"session_id": sessionID})

	errMsg := callToolExpectError(t, ctx, session, "submit_discovery_response", map[string]any{
		"session_id": sessionID,
		"response":   "EXCLUDED",
	})
	if errMsg == "" {
		t.Fatal("expected error for EXCLUDED response via MCP")
	}
}

// --- Concurrency: parallel tool calls on the server ---

func TestServer_ConcurrentSubmits_NoRace(t *testing.T) {
	srv := newTestServer(t)
	ctx := context.Background()
	session := connectInMemory(t, ctx, srv)

	startResult := callTool(t, ctx, session, "start_discovery", map[string]any{
		"max_iterations":      50,
		"terminate_on_repeat": false,
	})
	sessionID := startResult["session_id"].(string)

	// Sequential submits to verify server handles multiple discoveries
	for i := 0; i < 5; i++ {
		model := "model-" + string(rune('a'+i))
		callTool(t, ctx, session, "get_discovery_prompt", map[string]any{"session_id": sessionID})
		callTool(t, ctx, session, "submit_discovery_response", map[string]any{
			"session_id": sessionID,
			"response":   mockResponse(model, "Prov", "1.0"),
		})
	}

	report := callTool(t, ctx, session, "get_discovery_report", map[string]any{
		"session_id": sessionID,
	})
	unique := report["unique_models"].(float64)
	if unique != 5 {
		t.Fatalf("expected 5 unique models, got %v", unique)
	}
}

// --- Store: report persistence ---

func TestServer_GetReport_PersistsToStore(t *testing.T) {
	srv := newTestServer(t) // uses t.TempDir()
	ctx := context.Background()
	session := connectInMemory(t, ctx, srv)

	startResult := callTool(t, ctx, session, "start_discovery", map[string]any{
		"max_iterations": 1,
	})
	sessionID := startResult["session_id"].(string)

	callTool(t, ctx, session, "get_discovery_prompt", map[string]any{"session_id": sessionID})
	callTool(t, ctx, session, "submit_discovery_response", map[string]any{
		"session_id": sessionID,
		"response":   mockResponse("model-a", "ProvA", "1.0"),
	})

	callTool(t, ctx, session, "get_discovery_report", map[string]any{
		"session_id": sessionID,
	})

	// Verify the file was created in the temp dir
	store, err := metacal.NewFileRunStore(srv.RunsDir)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	runs, err := store.ListRuns()
	if err != nil {
		t.Fatalf("list runs: %v", err)
	}
	if len(runs) == 0 {
		t.Fatal("expected at least one persisted run after get_discovery_report")
	}
}

// --- Store: double get_report does not duplicate ---

func TestServer_GetReport_Twice_NoDuplicateStore(t *testing.T) {
	srv := newTestServer(t)
	ctx := context.Background()
	session := connectInMemory(t, ctx, srv)

	startResult := callTool(t, ctx, session, "start_discovery", map[string]any{
		"max_iterations": 1,
	})
	sessionID := startResult["session_id"].(string)

	callTool(t, ctx, session, "get_discovery_prompt", map[string]any{"session_id": sessionID})
	callTool(t, ctx, session, "submit_discovery_response", map[string]any{
		"session_id": sessionID,
		"response":   mockResponse("model-a", "ProvA", "1.0"),
	})

	callTool(t, ctx, session, "get_discovery_report", map[string]any{"session_id": sessionID})

	// Second get_report tries to save again — should not error
	// (it would error with "already exists" if SaveRun is called twice)
	reportResult := callTool(t, ctx, session, "get_discovery_report", map[string]any{
		"session_id": sessionID,
	})
	if reportResult["status"] != "done" {
		t.Fatalf("expected status=done on second get_report, got %v", reportResult["status"])
	}
}
