package mcp_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	mcpserver "asterisk/internal/mcp"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// projectRoot walks up from the test directory to find the module root (go.mod).
func projectRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find project root (go.mod)")
		}
		dir = parent
	}
}

func newTestServer(t *testing.T) *mcpserver.Server {
	t.Helper()
	srv := mcpserver.NewServer()
	srv.ProjectRoot = projectRoot(t)
	return srv
}

func connectInMemory(t *testing.T, ctx context.Context, srv *mcpserver.Server) *sdkmcp.ClientSession {
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

func TestServer_ToolDiscovery(t *testing.T) {
	srv := newTestServer(t)
	ctx := context.Background()
	session := connectInMemory(t, ctx, srv)
	defer session.Close()

	tools, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}

	want := map[string]bool{
		"start_calibration": false,
		"get_next_step":     false,
		"submit_artifact":   false,
		"get_report":        false,
		"emit_signal":       false,
		"get_signals":       false,
	}
	for _, tool := range tools.Tools {
		if _, ok := want[tool.Name]; ok {
			want[tool.Name] = true
		}
	}
	for name, found := range want {
		if !found {
			t.Errorf("tool %q not found in ListTools", name)
		}
	}
}

func TestServer_StubCalibration_FullLoop(t *testing.T) {
	srv := newTestServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	session := connectInMemory(t, ctx, srv)
	defer session.Close()

	// Start calibration with stub adapter on ptp-mock (12 cases, completes instantly)
	startResult := callTool(t, ctx, session, "start_calibration", map[string]any{
		"scenario": "ptp-mock",
		"adapter":  "stub",
	})

	sessionID, ok := startResult["session_id"].(string)
	if !ok || sessionID == "" {
		t.Fatalf("expected non-empty session_id, got %v", startResult["session_id"])
	}
	totalCases, _ := startResult["total_cases"].(float64)
	if totalCases < 1 {
		t.Fatalf("expected total_cases >= 1, got %v", totalCases)
	}
	t.Logf("started session %s with %v cases", sessionID, totalCases)

	// Stub adapter runs instantly; get_next_step should return done
	time.Sleep(500 * time.Millisecond) // give runner goroutine time to finish
	stepResult := callTool(t, ctx, session, "get_next_step", map[string]any{
		"session_id": sessionID,
	})
	done, _ := stepResult["done"].(bool)
	if !done {
		t.Fatalf("expected done=true for stub adapter, got %v", stepResult)
	}

	// Get report
	reportResult := callTool(t, ctx, session, "get_report", map[string]any{
		"session_id": sessionID,
	})

	status, _ := reportResult["status"].(string)
	if status != "done" {
		t.Fatalf("expected status=done, got %s", status)
	}

	reportStr, _ := reportResult["report"].(string)
	if reportStr == "" {
		t.Fatal("expected non-empty report string")
	}
	t.Logf("report preview: %.200s...", reportStr)

	metrics, ok := reportResult["metrics"].(map[string]any)
	if !ok {
		t.Fatal("expected metrics in report")
	}
	if _, hasAggregate := metrics["aggregate"]; !hasAggregate {
		t.Error("expected aggregate metrics")
	}
}

func TestServer_GetNextStep_NoSession(t *testing.T) {
	srv := newTestServer(t)
	ctx := context.Background()
	session := connectInMemory(t, ctx, srv)
	defer session.Close()

	res, err := session.CallTool(ctx, &sdkmcp.CallToolParams{
		Name:      "get_next_step",
		Arguments: map[string]any{"session_id": "nonexistent"},
	})
	if err != nil {
		t.Fatalf("expected tool error, got transport error: %v", err)
	}
	if !res.IsError {
		t.Fatal("expected IsError=true for missing session")
	}
}

func TestServer_SubmitArtifact_InvalidJSON(t *testing.T) {
	srv := newTestServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	session := connectInMemory(t, ctx, srv)
	defer session.Close()

	// Start a stub session first (so we have a valid session_id)
	startResult := callTool(t, ctx, session, "start_calibration", map[string]any{
		"scenario": "ptp-mock",
		"adapter":  "stub",
	})
	sessionID, _ := startResult["session_id"].(string)

	res, err := session.CallTool(ctx, &sdkmcp.CallToolParams{
		Name: "submit_artifact",
		Arguments: map[string]any{
			"session_id":    sessionID,
			"artifact_json": "not valid json{{{",
		},
	})
	if err != nil {
		t.Fatalf("expected tool error, got transport error: %v", err)
	}
	if !res.IsError {
		t.Fatal("expected IsError=true for invalid JSON artifact")
	}
}

func TestServer_DoubleStart_WhileRunning(t *testing.T) {
	srv := newTestServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	session := connectInMemory(t, ctx, srv)
	defer session.Close()

	// Start with cursor adapter (will block on MuxDispatcher channel, staying "running")
	startResult := callTool(t, ctx, session, "start_calibration", map[string]any{
		"scenario": "ptp-mock",
		"adapter":  "cursor",
	})
	if _, ok := startResult["session_id"].(string); !ok {
		t.Fatalf("first start failed: %v", startResult)
	}

	// Second start should fail because session is still running
	res, err := session.CallTool(ctx, &sdkmcp.CallToolParams{
		Name: "start_calibration",
		Arguments: map[string]any{
			"scenario": "ptp-mock",
			"adapter":  "cursor",
		},
	})
	if err != nil {
		t.Fatalf("expected tool error, got transport error: %v", err)
	}
	if !res.IsError {
		t.Fatal("expected IsError=true for double start while running")
	}
}

func TestServer_StartAfterDone(t *testing.T) {
	srv := newTestServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	session := connectInMemory(t, ctx, srv)
	defer session.Close()

	// Start with stub adapter (completes instantly)
	callTool(t, ctx, session, "start_calibration", map[string]any{
		"scenario": "ptp-mock",
		"adapter":  "stub",
	})

	time.Sleep(500 * time.Millisecond)

	// Second start should succeed because first is done
	startResult := callTool(t, ctx, session, "start_calibration", map[string]any{
		"scenario": "ptp-mock",
		"adapter":  "stub",
	})
	if _, ok := startResult["session_id"].(string); !ok {
		t.Fatalf("second start failed: %v", startResult)
	}
}

func TestServer_SignalBus_EmitAndGet(t *testing.T) {
	srv := newTestServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	session := connectInMemory(t, ctx, srv)
	defer session.Close()

	startResult := callTool(t, ctx, session, "start_calibration", map[string]any{
		"scenario": "ptp-mock",
		"adapter":  "stub",
	})
	sessionID := startResult["session_id"].(string)
	time.Sleep(500 * time.Millisecond)

	emitResult := callTool(t, ctx, session, "emit_signal", map[string]any{
		"session_id": sessionID,
		"event":      "dispatch",
		"agent":      "main",
		"case_id":    "C01",
		"step":       "F1",
		"meta":       map[string]any{"detail": "test"},
	})
	if emitResult["ok"] != "signal emitted" {
		t.Fatalf("expected ok='signal emitted', got %v", emitResult)
	}

	getResult := callTool(t, ctx, session, "get_signals", map[string]any{
		"session_id": sessionID,
	})
	total, _ := getResult["total"].(float64)
	if total < 3 {
		// session_started(1) + pipeline_done/session_done(2) + our emit(1) = at least 3
		t.Fatalf("expected at least 3 signals (server auto-emits), got %v", total)
	}

	signals, ok := getResult["signals"].([]any)
	if !ok || len(signals) == 0 {
		t.Fatal("expected signals array")
	}

	found := false
	for _, s := range signals {
		sig, ok := s.(map[string]any)
		if !ok {
			continue
		}
		if sig["event"] == "dispatch" && sig["agent"] == "main" && sig["case_id"] == "C01" {
			found = true
			break
		}
	}
	if !found {
		t.Error("agent-emitted dispatch signal not found in bus")
	}
}

func TestServer_SignalBus_AutoEmitOnStepAndSubmit(t *testing.T) {
	srv := newTestServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	session := connectInMemory(t, ctx, srv)
	defer session.Close()

	startResult := callTool(t, ctx, session, "start_calibration", map[string]any{
		"scenario": "ptp-mock",
		"adapter":  "stub",
	})
	sessionID := startResult["session_id"].(string)
	time.Sleep(500 * time.Millisecond)

	getResult := callTool(t, ctx, session, "get_signals", map[string]any{
		"session_id": sessionID,
	})

	signals, ok := getResult["signals"].([]any)
	if !ok {
		t.Fatal("expected signals array")
	}

	events := make(map[string]bool)
	for _, s := range signals {
		sig, ok := s.(map[string]any)
		if !ok {
			continue
		}
		events[sig["event"].(string)] = true
	}

	for _, want := range []string{"session_started", "session_done"} {
		if !events[want] {
			t.Errorf("expected auto-emitted %q signal, got events: %v", want, events)
		}
	}
}

func TestServer_SignalBus_SinceFilter(t *testing.T) {
	srv := newTestServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	session := connectInMemory(t, ctx, srv)
	defer session.Close()

	startResult := callTool(t, ctx, session, "start_calibration", map[string]any{
		"scenario": "ptp-mock",
		"adapter":  "stub",
	})
	sessionID := startResult["session_id"].(string)
	time.Sleep(500 * time.Millisecond)

	allResult := callTool(t, ctx, session, "get_signals", map[string]any{
		"session_id": sessionID,
	})
	allTotal, _ := allResult["total"].(float64)

	callTool(t, ctx, session, "emit_signal", map[string]any{
		"session_id": sessionID,
		"event":      "test_since",
		"agent":      "main",
	})

	sinceResult := callTool(t, ctx, session, "get_signals", map[string]any{
		"session_id": sessionID,
		"since":      allTotal,
	})
	sinceSigs, ok := sinceResult["signals"].([]any)
	if !ok || len(sinceSigs) != 1 {
		t.Fatalf("expected 1 signal since %v, got %d", allTotal, len(sinceSigs))
	}
	sig := sinceSigs[0].(map[string]any)
	if sig["event"] != "test_since" {
		t.Errorf("expected event=test_since, got %v", sig["event"])
	}
}

func TestServer_SignalBus_SinceAtTotal_ReturnsEmpty(t *testing.T) {
	srv := newTestServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	session := connectInMemory(t, ctx, srv)
	defer session.Close()

	startResult := callTool(t, ctx, session, "start_calibration", map[string]any{
		"scenario": "ptp-mock",
		"adapter":  "stub",
	})
	sessionID := startResult["session_id"].(string)
	time.Sleep(500 * time.Millisecond)

	allResult := callTool(t, ctx, session, "get_signals", map[string]any{
		"session_id": sessionID,
	})
	total, _ := allResult["total"].(float64)
	if total < 1 {
		t.Fatalf("expected at least 1 signal, got %v", total)
	}

	sinceResult := callTool(t, ctx, session, "get_signals", map[string]any{
		"session_id": sessionID,
		"since":      total,
	})
	sinceSigs, _ := sinceResult["signals"].([]any)
	if len(sinceSigs) != 0 {
		t.Fatalf("since=total should return 0 new signals, got %d", len(sinceSigs))
	}
}

func TestServer_SignalBus_SinceBeyondTotal_ReturnsEmpty(t *testing.T) {
	srv := newTestServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	session := connectInMemory(t, ctx, srv)
	defer session.Close()

	startResult := callTool(t, ctx, session, "start_calibration", map[string]any{
		"scenario": "ptp-mock",
		"adapter":  "stub",
	})
	sessionID := startResult["session_id"].(string)
	time.Sleep(500 * time.Millisecond)

	sinceResult := callTool(t, ctx, session, "get_signals", map[string]any{
		"session_id": sessionID,
		"since":      9999,
	})
	sinceSigs, _ := sinceResult["signals"].([]any)
	if len(sinceSigs) != 0 {
		t.Fatalf("since=9999 should return 0 signals, got %d", len(sinceSigs))
	}
}

func TestServer_SignalBus_SinceNegative_ReturnsAll(t *testing.T) {
	srv := newTestServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	session := connectInMemory(t, ctx, srv)
	defer session.Close()

	startResult := callTool(t, ctx, session, "start_calibration", map[string]any{
		"scenario": "ptp-mock",
		"adapter":  "stub",
	})
	sessionID := startResult["session_id"].(string)
	time.Sleep(500 * time.Millisecond)

	allResult := callTool(t, ctx, session, "get_signals", map[string]any{
		"session_id": sessionID,
	})
	allTotal, _ := allResult["total"].(float64)

	negResult := callTool(t, ctx, session, "get_signals", map[string]any{
		"session_id": sessionID,
		"since":      -5,
	})
	negSigs, _ := negResult["signals"].([]any)
	if len(negSigs) != int(allTotal) {
		t.Fatalf("since=-5 should return all %v signals, got %d", allTotal, len(negSigs))
	}
}

func TestServer_SignalBus_EmitRejectsEmptyEvent(t *testing.T) {
	srv := newTestServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	session := connectInMemory(t, ctx, srv)
	defer session.Close()

	startResult := callTool(t, ctx, session, "start_calibration", map[string]any{
		"scenario": "ptp-mock",
		"adapter":  "stub",
	})
	sessionID := startResult["session_id"].(string)
	time.Sleep(500 * time.Millisecond)

	res, err := session.CallTool(ctx, &sdkmcp.CallToolParams{
		Name: "emit_signal",
		Arguments: map[string]any{
			"session_id": sessionID,
			"event":      "",
			"agent":      "main",
		},
	})
	if err != nil {
		t.Fatalf("expected tool error, got transport error: %v", err)
	}
	if !res.IsError {
		t.Fatal("expected IsError=true for empty event")
	}
}

func TestServer_SignalBus_EmitRejectsEmptyAgent(t *testing.T) {
	srv := newTestServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	session := connectInMemory(t, ctx, srv)
	defer session.Close()

	startResult := callTool(t, ctx, session, "start_calibration", map[string]any{
		"scenario": "ptp-mock",
		"adapter":  "stub",
	})
	sessionID := startResult["session_id"].(string)
	time.Sleep(500 * time.Millisecond)

	res, err := session.CallTool(ctx, &sdkmcp.CallToolParams{
		Name: "emit_signal",
		Arguments: map[string]any{
			"session_id": sessionID,
			"event":      "dispatch",
			"agent":      "",
		},
	})
	if err != nil {
		t.Fatalf("expected tool error, got transport error: %v", err)
	}
	if !res.IsError {
		t.Fatal("expected IsError=true for empty agent")
	}
}

func TestServer_SignalBus_ConcurrentEmit(t *testing.T) {
	srv := newTestServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	session := connectInMemory(t, ctx, srv)
	defer session.Close()

	startResult := callTool(t, ctx, session, "start_calibration", map[string]any{
		"scenario": "ptp-mock",
		"adapter":  "stub",
	})
	sessionID := startResult["session_id"].(string)
	time.Sleep(500 * time.Millisecond)

	baseResult := callTool(t, ctx, session, "get_signals", map[string]any{
		"session_id": sessionID,
	})
	baseTotal, _ := baseResult["total"].(float64)

	const goroutines = 10
	const emitsPerGoroutine = 5
	errs := make(chan error, goroutines*emitsPerGoroutine)

	var wg sync.WaitGroup
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(gID int) {
			defer wg.Done()
			for i := 0; i < emitsPerGoroutine; i++ {
				res, err := session.CallTool(ctx, &sdkmcp.CallToolParams{
					Name: "emit_signal",
					Arguments: map[string]any{
						"session_id": sessionID,
						"event":      "concurrent_test",
						"agent":      fmt.Sprintf("goroutine-%d", gID),
						"meta":       map[string]any{"iter": fmt.Sprintf("%d", i)},
					},
				})
				if err != nil {
					errs <- fmt.Errorf("g%d-i%d transport: %w", gID, i, err)
					continue
				}
				if res.IsError {
					errs <- fmt.Errorf("g%d-i%d tool error", gID, i)
				}
			}
		}(g)
	}
	wg.Wait()
	close(errs)

	for e := range errs {
		t.Errorf("concurrent emit error: %v", e)
	}

	finalResult := callTool(t, ctx, session, "get_signals", map[string]any{
		"session_id": sessionID,
	})
	finalTotal, _ := finalResult["total"].(float64)
	expected := int(baseTotal) + goroutines*emitsPerGoroutine
	if int(finalTotal) != expected {
		t.Errorf("expected %d total signals, got %v", expected, finalTotal)
	}
}

func TestServer_Parallel_GetNextStep_TwoConcurrent(t *testing.T) {
	srv := newTestServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	session := connectInMemory(t, ctx, srv)
	defer session.Close()

	startResult := callTool(t, ctx, session, "start_calibration", map[string]any{
		"scenario": "ptp-mock",
		"adapter":  "cursor",
		"parallel": 2,
	})
	sessionID := startResult["session_id"].(string)

	type stepResult struct {
		caseID string
		step   string
		err    error
	}

	results := make(chan stepResult, 2)
	for i := 0; i < 2; i++ {
		go func() {
			res := callTool(t, ctx, session, "get_next_step", map[string]any{
				"session_id": sessionID,
			})
			results <- stepResult{
				caseID: res["case_id"].(string),
				step:   res["step"].(string),
			}
		}()
	}

	var steps []stepResult
	for i := 0; i < 2; i++ {
		select {
		case r := <-results:
			steps = append(steps, r)
		case <-ctx.Done():
			t.Fatalf("timed out waiting for get_next_step %d/2 (only %d returned)", i+1, len(steps))
		}
	}

	if steps[0].caseID == steps[1].caseID && steps[0].step == steps[1].step {
		t.Fatalf("expected 2 different steps, got same: %s/%s", steps[0].caseID, steps[0].step)
	}
	t.Logf("got 2 concurrent steps: %s/%s and %s/%s", steps[0].caseID, steps[0].step, steps[1].caseID, steps[1].step)
}

func TestServer_Parallel_FullFlow(t *testing.T) {
	srv := newTestServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	session := connectInMemory(t, ctx, srv)
	defer session.Close()

	startResult := callTool(t, ctx, session, "start_calibration", map[string]any{
		"scenario": "ptp-mock",
		"adapter":  "cursor",
		"parallel": 2,
	})
	sessionID := startResult["session_id"].(string)

	// Get 2 steps concurrently
	type stepInfo struct {
		caseID     string
		step       string
		dispatchID float64
	}
	stepCh := make(chan stepInfo, 2)
	for i := 0; i < 2; i++ {
		go func() {
			res := callTool(t, ctx, session, "get_next_step", map[string]any{
				"session_id": sessionID,
			})
			stepCh <- stepInfo{
				caseID:     res["case_id"].(string),
				step:       res["step"].(string),
				dispatchID: res["dispatch_id"].(float64),
			}
		}()
	}

	var steps []stepInfo
	for i := 0; i < 2; i++ {
		select {
		case s := <-stepCh:
			steps = append(steps, s)
		case <-ctx.Done():
			t.Fatalf("timed out waiting for get_next_step %d/2", i+1)
		}
	}

	if steps[0].dispatchID == steps[1].dispatchID {
		t.Fatalf("expected different dispatch_ids, got %v and %v", steps[0].dispatchID, steps[1].dispatchID)
	}

	// Submit artifacts with dispatch_id
	for _, s := range steps {
		artifact := fmt.Sprintf(`{"defect_type":"pb001","case":"%s"}`, s.caseID)
		callTool(t, ctx, session, "submit_artifact", map[string]any{
			"session_id":   sessionID,
			"artifact_json": artifact,
			"dispatch_id":  int64(s.dispatchID),
		})
	}

	t.Logf("parallel full flow: both steps submitted with dispatch_id routing")
}

func TestServer_Parallel_SubmitWrongDispatchID(t *testing.T) {
	srv := newTestServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	session := connectInMemory(t, ctx, srv)
	defer session.Close()

	startResult := callTool(t, ctx, session, "start_calibration", map[string]any{
		"scenario": "ptp-mock",
		"adapter":  "cursor",
		"parallel": 2,
	})
	sessionID := startResult["session_id"].(string)

	// Get one step to have a valid dispatch_id range
	res := callTool(t, ctx, session, "get_next_step", map[string]any{
		"session_id": sessionID,
	})
	_ = res["dispatch_id"].(float64)

	// Submit with a clearly wrong dispatch_id
	result, err := session.CallTool(ctx, &sdkmcp.CallToolParams{
		Name: "submit_artifact",
		Arguments: map[string]any{
			"session_id":    sessionID,
			"artifact_json": `{"defect_type":"pb001"}`,
			"dispatch_id":   int64(99999),
		},
	})
	if err != nil {
		t.Fatalf("transport error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error for submit with wrong dispatch_id")
	}
	t.Log("got expected error for wrong dispatch_id")
}

func TestServer_Parallel_InterleavedSubmit(t *testing.T) {
	srv := newTestServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	session := connectInMemory(t, ctx, srv)
	defer session.Close()

	startResult := callTool(t, ctx, session, "start_calibration", map[string]any{
		"scenario": "ptp-mock",
		"adapter":  "cursor",
		"parallel": 2,
	})
	sessionID := startResult["session_id"].(string)

	// Get 2 steps
	type stepInfo struct {
		caseID     string
		dispatchID float64
	}
	stepCh := make(chan stepInfo, 2)
	for i := 0; i < 2; i++ {
		go func() {
			res := callTool(t, ctx, session, "get_next_step", map[string]any{
				"session_id": sessionID,
			})
			stepCh <- stepInfo{
				caseID:     res["case_id"].(string),
				dispatchID: res["dispatch_id"].(float64),
			}
		}()
	}

	var steps []stepInfo
	for i := 0; i < 2; i++ {
		select {
		case s := <-stepCh:
			steps = append(steps, s)
		case <-ctx.Done():
			t.Fatalf("timed out waiting for step %d/2", i+1)
		}
	}

	// Submit in REVERSE order to test correct routing
	for i := len(steps) - 1; i >= 0; i-- {
		s := steps[i]
		artifact := fmt.Sprintf(`{"defect_type":"pb001","routed_to":"%s"}`, s.caseID)
		callTool(t, ctx, session, "submit_artifact", map[string]any{
			"session_id":    sessionID,
			"artifact_json": artifact,
			"dispatch_id":   int64(s.dispatchID),
		})
	}

	t.Log("interleaved submit: both steps submitted in reverse order")
}

func TestServer_SignalBus_NoSession(t *testing.T) {
	srv := newTestServer(t)
	ctx := context.Background()
	session := connectInMemory(t, ctx, srv)
	defer session.Close()

	res, err := session.CallTool(ctx, &sdkmcp.CallToolParams{
		Name:      "emit_signal",
		Arguments: map[string]any{"session_id": "nonexistent", "event": "test", "agent": "main"},
	})
	if err != nil {
		t.Fatalf("expected tool error, got transport error: %v", err)
	}
	if !res.IsError {
		t.Fatal("expected IsError=true for emit_signal with no session")
	}

	res, err = session.CallTool(ctx, &sdkmcp.CallToolParams{
		Name:      "get_signals",
		Arguments: map[string]any{"session_id": "nonexistent"},
	})
	if err != nil {
		t.Fatalf("expected tool error, got transport error: %v", err)
	}
	if !res.IsError {
		t.Fatal("expected IsError=true for get_signals with no session")
	}
}
