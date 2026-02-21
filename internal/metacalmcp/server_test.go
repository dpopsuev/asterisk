package metacalmcp_test

import (
	"context"
	"encoding/json"
	"testing"

	"asterisk/internal/metacalmcp"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

func newTestServer(t *testing.T) *metacalmcp.Server {
	t.Helper()
	return metacalmcp.NewServer(t.TempDir())
}

func connectInMemory(t *testing.T, ctx context.Context, srv *metacalmcp.Server) *sdkmcp.ClientSession {
	t.Helper()
	t1, t2 := sdkmcp.NewInMemoryTransports()
	if _, err := srv.MCPServer.Connect(ctx, t1, nil); err != nil {
		t.Fatalf("server.Connect: %v", err)
	}
	client := sdkmcp.NewClient(&sdkmcp.Implementation{Name: "test-client", Version: "v0.0.1"}, nil)
	session, err := client.Connect(ctx, t2, nil)
	if err != nil {
		t.Fatalf("client.Connect: %v", err)
	}
	return session
}

func callTool(t *testing.T, ctx context.Context, session *sdkmcp.ClientSession, name string, args map[string]any) map[string]any {
	t.Helper()
	res, err := session.CallTool(ctx, &sdkmcp.CallToolParams{
		Name:      name,
		Arguments: args,
	})
	if err != nil {
		t.Fatalf("CallTool(%s): %v", name, err)
	}
	if res.IsError {
		for _, c := range res.Content {
			if tc, ok := c.(*sdkmcp.TextContent); ok {
				t.Fatalf("CallTool(%s) returned error: %s", name, tc.Text)
			}
		}
		t.Fatalf("CallTool(%s) returned error", name)
	}
	result := make(map[string]any)
	for _, c := range res.Content {
		if tc, ok := c.(*sdkmcp.TextContent); ok {
			if err := json.Unmarshal([]byte(tc.Text), &result); err != nil {
				t.Fatalf("unmarshal tool result: %v (text: %s)", err, tc.Text)
			}
			return result
		}
	}
	t.Fatalf("no text content in tool result")
	return nil
}

func callToolExpectError(t *testing.T, ctx context.Context, session *sdkmcp.ClientSession, name string, args map[string]any) string {
	t.Helper()
	res, err := session.CallTool(ctx, &sdkmcp.CallToolParams{
		Name:      name,
		Arguments: args,
	})
	if err != nil {
		return err.Error()
	}
	if res.IsError {
		for _, c := range res.Content {
			if tc, ok := c.(*sdkmcp.TextContent); ok {
				return tc.Text
			}
		}
		return "unknown error"
	}
	t.Fatal("expected error but got success")
	return ""
}

func mockResponse(modelName, provider, version string) string {
	return `{"model_name": "` + modelName + `", "provider": "` + provider + `", "version": "` + version + `", "wrapper": "Cursor"}

` + "```go\n" + `func calculateSum(numbers []int, label string, verbose bool) (int, string, error) {
	// Calculate the absolute sum of numbers and build a description
	total := 0
	var description string
	for _, num := range numbers {
		if num > 0 {
			total += num
			if verbose {
				description += fmt.Sprintf("%d,", num)
			}
		} else if num < 0 {
			total -= num
			if verbose {
				description += fmt.Sprintf("(%d),", num)
			}
		}
	}
	if total == 0 {
		return 0, "", fmt.Errorf("empty result for %s", label)
	}
	if label != "" {
		description = label + ": " + description
	}
	return total, description, nil
}` + "\n```"
}

func TestServer_ToolDiscovery(t *testing.T) {
	srv := newTestServer(t)
	ctx := context.Background()
	session := connectInMemory(t, ctx, srv)

	tools, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}

	expected := map[string]bool{
		"start_discovery":          false,
		"get_discovery_prompt":     false,
		"submit_discovery_response": false,
		"get_discovery_report":     false,
		"emit_signal":             false,
		"get_signals":             false,
	}

	for _, tool := range tools.Tools {
		if _, ok := expected[tool.Name]; ok {
			expected[tool.Name] = true
		}
	}

	for name, found := range expected {
		if !found {
			t.Errorf("expected tool %q not found", name)
		}
	}
}

func TestServer_FullDiscoveryLoop(t *testing.T) {
	srv := newTestServer(t)
	ctx := context.Background()
	session := connectInMemory(t, ctx, srv)

	// Start discovery with max 5 iterations
	startResult := callTool(t, ctx, session, "start_discovery", map[string]any{
		"max_iterations":      5,
		"terminate_on_repeat": true,
	})
	sessionID, ok := startResult["session_id"].(string)
	if !ok || sessionID == "" {
		t.Fatal("start_discovery did not return session_id")
	}
	if startResult["status"] != "running" {
		t.Fatalf("expected status=running, got %v", startResult["status"])
	}

	// Iteration 0: discover claude
	promptResult := callTool(t, ctx, session, "get_discovery_prompt", map[string]any{
		"session_id": sessionID,
	})
	if promptResult["done"] == true {
		t.Fatal("should not be done on first prompt")
	}
	if promptResult["prompt"] == nil || promptResult["prompt"] == "" {
		t.Fatal("expected non-empty prompt")
	}

	submitResult := callTool(t, ctx, session, "submit_discovery_response", map[string]any{
		"session_id": sessionID,
		"response":   mockResponse("claude-sonnet-4-20250514", "Anthropic", "20250514"),
	})
	if submitResult["repeated"] == true {
		t.Fatal("first submission should not be repeated")
	}
	if submitResult["model_name"] != "claude-sonnet-4-20250514" {
		t.Fatalf("expected claude-sonnet-4-20250514, got %v", submitResult["model_name"])
	}

	// Iteration 1: discover gpt-4o
	callTool(t, ctx, session, "get_discovery_prompt", map[string]any{"session_id": sessionID})
	submitResult2 := callTool(t, ctx, session, "submit_discovery_response", map[string]any{
		"session_id": sessionID,
		"response":   mockResponse("gpt-4o", "OpenAI", "4o"),
	})
	if submitResult2["repeated"] == true {
		t.Fatal("second unique model should not be repeated")
	}

	// Iteration 2: repeat claude — should terminate
	callTool(t, ctx, session, "get_discovery_prompt", map[string]any{"session_id": sessionID})
	submitResult3 := callTool(t, ctx, session, "submit_discovery_response", map[string]any{
		"session_id": sessionID,
		"response":   mockResponse("claude-sonnet-4-20250514", "Anthropic", "20250514"),
	})
	if submitResult3["repeated"] != true {
		t.Fatal("repeated model should be flagged")
	}
	if submitResult3["done"] != true {
		t.Fatal("session should be done after repeat with terminate_on_repeat")
	}

	// Prompt should now say done
	promptDone := callTool(t, ctx, session, "get_discovery_prompt", map[string]any{"session_id": sessionID})
	if promptDone["done"] != true {
		t.Fatal("expected done=true after termination")
	}

	// Get report
	reportResult := callTool(t, ctx, session, "get_discovery_report", map[string]any{
		"session_id": sessionID,
	})
	if reportResult["status"] != "done" {
		t.Fatalf("expected status=done, got %v", reportResult["status"])
	}
	uniqueModels := reportResult["unique_models"].(float64)
	if uniqueModels != 2 {
		t.Fatalf("expected 2 unique models, got %v", uniqueModels)
	}
}

func TestServer_GetPrompt_NoSession(t *testing.T) {
	srv := newTestServer(t)
	ctx := context.Background()
	session := connectInMemory(t, ctx, srv)

	errMsg := callToolExpectError(t, ctx, session, "get_discovery_prompt", map[string]any{
		"session_id": "nonexistent",
	})
	if errMsg == "" {
		t.Fatal("expected error for nonexistent session")
	}
}

func TestServer_SubmitResponse_ParseError(t *testing.T) {
	srv := newTestServer(t)
	ctx := context.Background()
	session := connectInMemory(t, ctx, srv)

	startResult := callTool(t, ctx, session, "start_discovery", map[string]any{})
	sessionID := startResult["session_id"].(string)

	callTool(t, ctx, session, "get_discovery_prompt", map[string]any{"session_id": sessionID})

	errMsg := callToolExpectError(t, ctx, session, "submit_discovery_response", map[string]any{
		"session_id": sessionID,
		"response":   "this is garbage with no identity JSON",
	})
	if errMsg == "" {
		t.Fatal("expected error for unparseable response")
	}
}

func TestServer_SignalBus(t *testing.T) {
	srv := newTestServer(t)
	ctx := context.Background()
	session := connectInMemory(t, ctx, srv)

	startResult := callTool(t, ctx, session, "start_discovery", map[string]any{})
	sessionID := startResult["session_id"].(string)

	// Emit a custom signal
	emitResult := callTool(t, ctx, session, "emit_signal", map[string]any{
		"session_id": sessionID,
		"event":      "test_event",
		"agent":      "test",
		"meta":       map[string]any{"key": "value"},
	})
	if emitResult["ok"] != "signal emitted" {
		t.Fatalf("expected ok, got %v", emitResult["ok"])
	}

	// Get all signals (session_started + test_event)
	signalsResult := callTool(t, ctx, session, "get_signals", map[string]any{
		"session_id": sessionID,
	})
	total := signalsResult["total"].(float64)
	if total < 2 {
		t.Fatalf("expected at least 2 signals (session_started + test_event), got %v", total)
	}

	// Get signals since index 1 (skip session_started)
	sinceResult := callTool(t, ctx, session, "get_signals", map[string]any{
		"session_id": sessionID,
		"since":      1,
	})
	signals := sinceResult["signals"].([]any)
	if len(signals) == 0 {
		t.Fatal("expected at least 1 signal since index 1")
	}
	firstSignal := signals[0].(map[string]any)
	if firstSignal["event"] != "test_event" {
		t.Fatalf("expected test_event, got %v", firstSignal["event"])
	}
}

func TestServer_MaxIterations(t *testing.T) {
	srv := newTestServer(t)
	ctx := context.Background()
	session := connectInMemory(t, ctx, srv)

	startResult := callTool(t, ctx, session, "start_discovery", map[string]any{
		"max_iterations": 2,
	})
	sessionID := startResult["session_id"].(string)

	// Iteration 0
	callTool(t, ctx, session, "get_discovery_prompt", map[string]any{"session_id": sessionID})
	callTool(t, ctx, session, "submit_discovery_response", map[string]any{
		"session_id": sessionID,
		"response":   mockResponse("model-a", "ProviderA", "1.0"),
	})

	// Iteration 1
	callTool(t, ctx, session, "get_discovery_prompt", map[string]any{"session_id": sessionID})
	callTool(t, ctx, session, "submit_discovery_response", map[string]any{
		"session_id": sessionID,
		"response":   mockResponse("model-b", "ProviderB", "2.0"),
	})

	// Iteration 2 should be done (max_iterations=2)
	promptResult := callTool(t, ctx, session, "get_discovery_prompt", map[string]any{"session_id": sessionID})
	if promptResult["done"] != true {
		t.Fatal("expected done=true after reaching max_iterations")
	}
}
