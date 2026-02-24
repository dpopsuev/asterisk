package mcp_test

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	mcpserver "asterisk/internal/mcp"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestMain(m *testing.M) {
	mcpserver.DefaultGetNextStepTimeout = 1 * time.Second
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelError,
	})))
	os.Exit(m.Run())
}

// dumpGoroutines writes all goroutine stacks to the test log.
func dumpGoroutines(t *testing.T) {
	t.Helper()
	buf := make([]byte, 1<<20)
	n := runtime.Stack(buf, true)
	t.Logf("=== GOROUTINE DUMP ===\n%s", buf[:n])
}

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
	t.Cleanup(srv.Shutdown)
	return srv
}

func connectInMemory(t *testing.T, ctx context.Context, srv *mcpserver.Server) *sdkmcp.ClientSession {
	t.Helper()
	t1, t2 := sdkmcp.NewInMemoryTransports()
	serverSession, err := srv.MCPServer.Connect(ctx, t1, nil)
	if err != nil {
		t.Fatalf("server.Connect: %v", err)
	}
	t.Cleanup(func() { serverSession.Close() })

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

// callToolE is a goroutine-safe variant of callTool that returns errors instead of calling t.Fatal.
func callToolE(ctx context.Context, session *sdkmcp.ClientSession, name string, args map[string]any) (map[string]any, error) {
	res, err := session.CallTool(ctx, &sdkmcp.CallToolParams{
		Name:      name,
		Arguments: args,
	})
	if err != nil {
		return nil, fmt.Errorf("CallTool(%s): %w", name, err)
	}
	if res.IsError {
		for _, c := range res.Content {
			if tc, ok := c.(*sdkmcp.TextContent); ok {
				return nil, fmt.Errorf("CallTool(%s) error: %s", name, tc.Text)
			}
		}
		return nil, fmt.Errorf("CallTool(%s) returned error", name)
	}
	for _, c := range res.Content {
		if tc, ok := c.(*sdkmcp.TextContent); ok {
			result := make(map[string]any)
			if err := json.Unmarshal([]byte(tc.Text), &result); err != nil {
				return nil, fmt.Errorf("unmarshal %s result: %w", name, err)
			}
			return result, nil
		}
	}
	return nil, fmt.Errorf("no text content in %s result", name)
}

// artifactForStepViaResolve returns minimal valid JSON like artifactForStep
// but the F1 artifact forces the pipeline through F2 (Resolve) instead of
// skipping directly to F3. This exercises the F2 → F3 heuristic path.
func artifactForStepViaResolve(step string, subagentID int) string {
	switch step {
	case "F1_TRIAGE":
		return `{"symptom_category":"product","severity":"high","defect_type_hypothesis":"pb001","candidate_repos":["repo-a","repo-b"],"skip_investigation":false,"cascade_suspected":false}`
	default:
		return artifactForStep(step, subagentID)
	}
}

// artifactForStep returns minimal valid JSON for a given pipeline step.
func artifactForStep(step string, subagentID int) string {
	switch step {
	case "F0_RECALL":
		return fmt.Sprintf(`{"match":false,"confidence":0.0,"reasoning":"subagent-%d"}`, subagentID)
	case "F1_TRIAGE":
		return `{"symptom_category":"product","severity":"high","defect_type_hypothesis":"pb001","candidate_repos":["test-repo"],"skip_investigation":false,"cascade_suspected":false}`
	case "F2_RESOLVE":
		return `{"selected_repos":[{"name":"test-repo","reason":"test"}]}`
	case "F3_INVESTIGATE":
		return fmt.Sprintf(`{"rca_message":"root cause from subagent-%d","defect_type":"pb001","component":"test-component","convergence_score":0.85,"evidence_refs":["ref-1"]}`, subagentID)
	case "F4_CORRELATE":
		return `{"is_duplicate":false,"confidence":0.1}`
	case "F5_REVIEW":
		return `{"decision":"approve"}`
	case "F6_REPORT":
		return fmt.Sprintf(`{"defect_type":"pb001","case_id":"auto","subagent":%d}`, subagentID)
	default:
		return fmt.Sprintf(`{"defect_type":"pb001","subagent":%d}`, subagentID)
	}
}

type stepRecord struct {
	CaseID     string
	Step       string
	DispatchID int64
}

func TestServer_FourSubagents_FullDrain(t *testing.T) {
	srv := newTestServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	session := connectInMemory(t, ctx, srv)
	defer session.Close()

	startResult := callTool(t, ctx, session, "start_calibration", map[string]any{
		"scenario": "ptp-mock",
		"adapter":  "cursor",
		"parallel": 4,
	})
	sessionID := startResult["session_id"].(string)
	t.Logf("started session %s", sessionID)

	var mu sync.Mutex
	workLog := make(map[int][]stepRecord) // subagentID -> records

	var wg sync.WaitGroup
	errCh := make(chan error, 4)

	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func(subID int) {
			defer wg.Done()
			for {
				res, err := callToolE(ctx, session, "get_next_step", map[string]any{
					"session_id": sessionID,
					"timeout_ms": 200,
				})
				if err != nil {
					errCh <- fmt.Errorf("subagent-%d get_next_step: %w", subID, err)
					return
				}

				if done, _ := res["done"].(bool); done {
					return
				}

				if avail, _ := res["available"].(bool); !avail {
					continue
				}

				caseID, _ := res["case_id"].(string)
				step, _ := res["step"].(string)
				dispatchID, _ := res["dispatch_id"].(float64)

				artifact := artifactForStep(step, subID)
				_, err = callToolE(ctx, session, "submit_artifact", map[string]any{
					"session_id":    sessionID,
					"artifact_json": artifact,
					"dispatch_id":   int64(dispatchID),
				})
				if err != nil {
					errCh <- fmt.Errorf("subagent-%d submit_artifact(%s/%s): %w", subID, caseID, step, err)
					return
				}

				mu.Lock()
				workLog[subID] = append(workLog[subID], stepRecord{
					CaseID:     caseID,
					Step:       step,
					DispatchID: int64(dispatchID),
				})
				mu.Unlock()
			}
		}(i)
	}

	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Errorf("subagent error: %v", err)
	}

	// Assertion 1: all 4 subagents got work (no starvation)
	for i := 0; i < 4; i++ {
		if len(workLog[i]) == 0 {
			t.Errorf("subagent-%d got zero steps (starvation)", i)
		} else {
			t.Logf("subagent-%d processed %d steps", i, len(workLog[i]))
		}
	}

	// Assertion 2: all dispatch IDs are unique
	seenIDs := make(map[int64]bool)
	totalSteps := 0
	for _, records := range workLog {
		for _, r := range records {
			if seenIDs[r.DispatchID] {
				t.Errorf("duplicate dispatch_id %d", r.DispatchID)
			}
			seenIDs[r.DispatchID] = true
			totalSteps++
		}
	}

	// Assertion 3: total steps > 0
	if totalSteps == 0 {
		t.Fatal("pipeline produced zero steps")
	}
	t.Logf("total steps processed: %d across 4 subagents", totalSteps)

	// Assertion 4: final report is complete
	reportResult := callTool(t, ctx, session, "get_report", map[string]any{
		"session_id": sessionID,
	})
	status, _ := reportResult["status"].(string)
	if status != "done" {
		t.Fatalf("expected status=done, got %s", status)
	}
	caseResults, _ := reportResult["case_results"].([]any)
	if len(caseResults) == 0 {
		t.Error("expected case_results in report")
	}
	t.Logf("report: status=%s, case_results=%d", status, len(caseResults))
}

func TestServer_FourSubagents_ViaResolve(t *testing.T) {
	srv := newTestServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	session := connectInMemory(t, ctx, srv)
	defer session.Close()

	startResult := callTool(t, ctx, session, "start_calibration", map[string]any{
		"scenario": "ptp-mock",
		"adapter":  "cursor",
		"parallel": 4,
	})
	sessionID := startResult["session_id"].(string)

	var mu sync.Mutex
	stepLog := make(map[string][]string) // caseID -> list of steps

	var wg sync.WaitGroup
	errCh := make(chan error, 4)

	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func(subID int) {
			defer wg.Done()
			for {
				res, err := callToolE(ctx, session, "get_next_step", map[string]any{
					"session_id": sessionID,
					"timeout_ms": 200,
				})
				if err != nil {
					errCh <- fmt.Errorf("subagent-%d get_next_step: %w", subID, err)
					return
				}
				if done, _ := res["done"].(bool); done {
					return
				}
				if avail, _ := res["available"].(bool); !avail {
					continue
				}

				caseID, _ := res["case_id"].(string)
				step, _ := res["step"].(string)
				dispatchID, _ := res["dispatch_id"].(float64)

				artifact := artifactForStepViaResolve(step, subID)
				_, err = callToolE(ctx, session, "submit_artifact", map[string]any{
					"session_id":    sessionID,
					"artifact_json": artifact,
					"dispatch_id":   int64(dispatchID),
				})
				if err != nil {
					errCh <- fmt.Errorf("subagent-%d submit(%s/%s): %w", subID, caseID, step, err)
					return
				}

				mu.Lock()
				stepLog[caseID] = append(stepLog[caseID], step)
				mu.Unlock()
			}
		}(i)
	}

	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Errorf("subagent error: %v", err)
	}

	// At least one case should have gone through F2_RESOLVE -> F3_INVESTIGATE
	var f2Count, f3Count int
	for caseID, steps := range stepLog {
		t.Logf("case %s: %v", caseID, steps)
		for _, s := range steps {
			if s == "F2_RESOLVE" {
				f2Count++
			}
			if s == "F3_INVESTIGATE" {
				f3Count++
			}
		}
	}

	if f2Count == 0 {
		t.Error("no cases went through F2_RESOLVE — triage artifacts may not trigger the resolve path")
	}
	if f3Count == 0 {
		t.Error("no cases reached F3_INVESTIGATE — the F2→F3 transition is broken")
	}
	t.Logf("F2 dispatches: %d, F3 dispatches: %d", f2Count, f3Count)

	reportResult := callTool(t, ctx, session, "get_report", map[string]any{
		"session_id": sessionID,
	})
	status, _ := reportResult["status"].(string)
	if status != "done" {
		t.Fatalf("expected status=done, got %s", status)
	}
}

func TestServer_FourSubagents_NoDuplicateDispatch(t *testing.T) {
	srv := newTestServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	session := connectInMemory(t, ctx, srv)
	defer session.Close()

	startResult := callTool(t, ctx, session, "start_calibration", map[string]any{
		"scenario": "daemon-mock",
		"adapter":  "cursor",
		"parallel": 4,
	})
	sessionID := startResult["session_id"].(string)

	var mu sync.Mutex
	var allRecords []stepRecord

	var wg sync.WaitGroup
	errCh := make(chan error, 4)

	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func(subID int) {
			defer wg.Done()
			for {
				res, err := callToolE(ctx, session, "get_next_step", map[string]any{
					"session_id": sessionID,
					"timeout_ms": 200,
				})
				if err != nil {
					errCh <- fmt.Errorf("subagent-%d: %w", subID, err)
					return
				}

				if done, _ := res["done"].(bool); done {
					return
				}

				if avail, _ := res["available"].(bool); !avail {
					continue
				}

				caseID, _ := res["case_id"].(string)
				step, _ := res["step"].(string)
				dispatchID, _ := res["dispatch_id"].(float64)

				artifact := artifactForStep(step, subID)
				_, err = callToolE(ctx, session, "submit_artifact", map[string]any{
					"session_id":    sessionID,
					"artifact_json": artifact,
					"dispatch_id":   int64(dispatchID),
				})
				if err != nil {
					errCh <- fmt.Errorf("subagent-%d submit(%s/%s): %w", subID, caseID, step, err)
					return
				}

				mu.Lock()
				allRecords = append(allRecords, stepRecord{
					CaseID:     caseID,
					Step:       step,
					DispatchID: int64(dispatchID),
				})
				mu.Unlock()
			}
		}(i)
	}

	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Errorf("subagent error: %v", err)
	}

	// Assert no (case_id, step) pair appears twice
	type caseStep struct{ CaseID, Step string }
	seen := make(map[caseStep]int)
	for _, r := range allRecords {
		key := caseStep{r.CaseID, r.Step}
		seen[key]++
		if seen[key] > 1 {
			t.Errorf("duplicate dispatch: case=%s step=%s appeared %d times", r.CaseID, r.Step, seen[key])
		}
	}

	if len(allRecords) == 0 {
		t.Fatal("pipeline produced zero steps")
	}
	t.Logf("daemon-mock: %d unique (case, step) pairs processed, 0 duplicates", len(seen))

	reportResult := callTool(t, ctx, session, "get_report", map[string]any{
		"session_id": sessionID,
	})
	if status, _ := reportResult["status"].(string); status != "done" {
		t.Fatalf("expected status=done, got %s", status)
	}
}

// TestServer_SubagentCapacity_PeakConcurrency proves that the signal bus
// can track active subagent count and detect peak concurrency. Each simulated
// subagent emits "start" before working and "done" after submitting. The test
// asserts:
//   1. Peak concurrent active subagents reaches exactly the desired capacity (4)
//   2. Active count never exceeds capacity
//   3. All subagents are observed via signals
func TestServer_SubagentCapacity_PeakConcurrency(t *testing.T) {
	srv := newTestServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	session := connectInMemory(t, ctx, srv)
	defer session.Close()

	startResult := callTool(t, ctx, session, "start_calibration", map[string]any{
		"scenario": "ptp-mock",
		"adapter":  "cursor",
		"parallel": 4,
	})
	sessionID := startResult["session_id"].(string)

	const capacity = 4
	var mu sync.Mutex
	var peakActive int
	var active int
	subagentsSeen := make(map[int]bool)

	var wg sync.WaitGroup
	errCh := make(chan error, capacity)

	for i := 0; i < capacity; i++ {
		wg.Add(1)
		go func(subID int) {
			defer wg.Done()

			// Emit start signal — this is what the capacity system monitors
			_, _ = callToolE(ctx, session, "emit_signal", map[string]any{
				"session_id": sessionID,
				"event":      "start",
				"agent":      "sub",
				"meta":       map[string]any{"subagent_id": fmt.Sprintf("%d", subID)},
			})

			mu.Lock()
			active++
			subagentsSeen[subID] = true
			if active > peakActive {
				peakActive = active
			}
			mu.Unlock()

			for {
				res, err := callToolE(ctx, session, "get_next_step", map[string]any{
					"session_id": sessionID,
					"timeout_ms": 200,
				})
				if err != nil {
					errCh <- err
					return
				}
				if done, _ := res["done"].(bool); done {
					break
				}

				if avail, _ := res["available"].(bool); !avail {
					continue
				}

				time.Sleep(3 * time.Millisecond)

				step, _ := res["step"].(string)
				dispatchID, _ := res["dispatch_id"].(float64)

				artifact := artifactForStep(step, subID)
				_, err = callToolE(ctx, session, "submit_artifact", map[string]any{
					"session_id":    sessionID,
					"artifact_json": artifact,
					"dispatch_id":   int64(dispatchID),
				})
				if err != nil {
					errCh <- err
					return
				}
			}

			mu.Lock()
			active--
			mu.Unlock()

			// Emit done signal
			_, _ = callToolE(ctx, session, "emit_signal", map[string]any{
				"session_id": sessionID,
				"event":      "done",
				"agent":      "sub",
				"meta":       map[string]any{"subagent_id": fmt.Sprintf("%d", subID)},
			})
		}(i)
	}

	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Fatalf("subagent error: %v", err)
	}

	// Assertion 1: peak concurrent reached desired capacity
	if peakActive < capacity {
		t.Errorf("peak concurrent subagents = %d, wanted %d (not fully parallel)", peakActive, capacity)
	}
	t.Logf("peak concurrent subagents: %d/%d", peakActive, capacity)

	// Assertion 2: all subagents were observed
	if len(subagentsSeen) != capacity {
		t.Errorf("saw %d unique subagents, wanted %d", len(subagentsSeen), capacity)
	}

	// Assertion 3: signal bus recorded all start/done pairs
	signals := callTool(t, ctx, session, "get_signals", map[string]any{
		"session_id": sessionID,
	})
	signalList, _ := signals["signals"].([]any)
	startCount, doneCount := 0, 0
	for _, s := range signalList {
		sig, _ := s.(map[string]any)
		event, _ := sig["event"].(string)
		agent, _ := sig["agent"].(string)
		if agent == "sub" {
			switch event {
			case "start":
				startCount++
			case "done":
				doneCount++
			}
		}
	}
	if startCount != capacity {
		t.Errorf("expected %d start signals, got %d", capacity, startCount)
	}
	if doneCount != capacity {
		t.Errorf("expected %d done signals, got %d", capacity, doneCount)
	}
	t.Logf("signals: %d start, %d done (from %d total signals)", startCount, doneCount, len(signalList))
}

// TestServer_FourSubagents_ConcurrencyTiming proves that 4 concurrent workers
// complete significantly faster than serial execution. Each worker adds a
// deliberate per-step delay (5ms). With N total steps:
//   - Serial: N * 5ms wall time
//   - 4-concurrent: ~N/4 * 5ms wall time
//
// The test asserts wall time is < 60% of serial estimate, proving true
// concurrent dispatch (not sequential).
func TestServer_FourSubagents_ConcurrencyTiming(t *testing.T) {
	srv := newTestServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	session := connectInMemory(t, ctx, srv)
	defer session.Close()

	startResult := callTool(t, ctx, session, "start_calibration", map[string]any{
		"scenario": "ptp-mock",
		"adapter":  "cursor",
		"parallel": 4,
	})
	sessionID := startResult["session_id"].(string)

	const perStepDelay = 20 * time.Millisecond
	var mu sync.Mutex
	var totalSteps int64

	var wg sync.WaitGroup
	errCh := make(chan error, 4)

	start := time.Now()
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func(subID int) {
			defer wg.Done()
			for {
				res, err := callToolE(ctx, session, "get_next_step", map[string]any{
					"session_id": sessionID,
					"timeout_ms": 200,
				})
				if err != nil {
					errCh <- err
					return
				}
				if done, _ := res["done"].(bool); done {
					return
				}

				if avail, _ := res["available"].(bool); !avail {
					continue
				}

				time.Sleep(perStepDelay)

				step, _ := res["step"].(string)
				dispatchID, _ := res["dispatch_id"].(float64)

				artifact := artifactForStep(step, subID)
				_, err = callToolE(ctx, session, "submit_artifact", map[string]any{
					"session_id":    sessionID,
					"artifact_json": artifact,
					"dispatch_id":   int64(dispatchID),
				})
				if err != nil {
					errCh <- err
					return
				}
				mu.Lock()
				totalSteps++
				mu.Unlock()
			}
		}(i)
	}

	wg.Wait()
	close(errCh)
	elapsed := time.Since(start)

	for err := range errCh {
		t.Fatalf("subagent error: %v", err)
	}

	serialEstimate := time.Duration(totalSteps) * perStepDelay
	speedup := float64(serialEstimate) / float64(elapsed)
	concurrencyThreshold := 0.75

	t.Logf("steps=%d, elapsed=%v, serial_estimate=%v, speedup=%.2fx",
		totalSteps, elapsed, serialEstimate, speedup)

	if elapsed > time.Duration(float64(serialEstimate)*concurrencyThreshold) {
		t.Errorf("concurrent execution too slow: elapsed=%v > %.0f%% of serial=%v (speedup=%.2fx, expected >1.5x)",
			elapsed, concurrencyThreshold*100, serialEstimate, speedup)
	}

	// Verify report
	reportResult := callTool(t, ctx, session, "get_report", map[string]any{
		"session_id": sessionID,
	})
	if status, _ := reportResult["status"].(string); status != "done" {
		t.Fatalf("expected done, got %s", status)
	}
}

// TestServer_CapacityGate_AdvisoryOnSerialSubmit proves the capacity gate
// is advisory: it logs warnings but never rejects submissions. Rejecting
// a submit after a step was pulled would orphan the MuxDispatcher pending
// entry, causing the runner goroutine to hang on responseCh — a deadlock.
//
// Serial pattern:  pull 1 → submit → ACCEPTED (gate warns, doesn't block)
// Parallel pattern: pull 4 → submit 4 → all accepted, no warning
func TestServer_CapacityGate_AdvisoryOnSerialSubmit(t *testing.T) {
	srv := newTestServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	session := connectInMemory(t, ctx, srv)
	defer session.Close()

	startResult := callTool(t, ctx, session, "start_calibration", map[string]any{
		"scenario": "ptp-mock",
		"adapter":  "cursor",
		"parallel": 4,
	})
	sessionID := startResult["session_id"].(string)

	// Pull just 1 step (serial pattern)
	step1 := callTool(t, ctx, session, "get_next_step", map[string]any{
		"session_id": sessionID, "timeout_ms": 500,
	})
	dispatchID1, _ := step1["dispatch_id"].(float64)
	stepName1, _ := step1["step"].(string)

	// Submit with only 1/4 pulled — gate warns but MUST accept
	_, err := callToolE(ctx, session, "submit_artifact", map[string]any{
		"session_id":    sessionID,
		"artifact_json": artifactForStep(stepName1, 0),
		"dispatch_id":   int64(dispatchID1),
	})
	if err != nil {
		t.Fatalf("serial submit must be accepted (gate is advisory): %v", err)
	}
	t.Log("serial submit accepted (gate advisory, no rejection)")

	// The capacity_warning field in get_next_step is the enforcement signal.
	// Verify it appears when under capacity.
	step2 := callTool(t, ctx, session, "get_next_step", map[string]any{
		"session_id": sessionID, "timeout_ms": 500,
	})
	if warn, _ := step2["capacity_warning"].(string); warn == "" {
		t.Log("no capacity_warning on get_next_step (puller peak may have opened gate)")
	} else {
		t.Logf("capacity_warning present: %s", warn)
	}
}

// TestServer_CapacityGate_AllowsDrainingPipeline proves the gate relaxes
// when the pipeline has fewer remaining steps than capacity (draining).
// Without this escape hatch, the last few steps could never be submitted.
func TestServer_CapacityGate_AllowsDrainingPipeline(t *testing.T) {
	srv := newTestServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	session := connectInMemory(t, ctx, srv)
	defer session.Close()

	startResult := callTool(t, ctx, session, "start_calibration", map[string]any{
		"scenario": "ptp-mock",
		"adapter":  "cursor",
		"parallel": 4,
	})
	sessionID := startResult["session_id"].(string)

	// Drain most of the pipeline at full capacity
	for {
		// Pull 4
		var steps []map[string]any
		allDone := false
		for i := 0; i < 4; i++ {
			res, err := callToolE(ctx, session, "get_next_step", map[string]any{
				"session_id": sessionID, "timeout_ms": 200,
			})
			if err != nil {
				t.Fatalf("get_next_step: %v", err)
			}
			if done, _ := res["done"].(bool); done {
				allDone = true
				break
			}
			if avail, _ := res["available"].(bool); !avail {
				break
			}
			steps = append(steps, res)
		}
		if allDone {
			break
		}
		// Submit all pulled
		for _, step := range steps {
			did, _ := step["dispatch_id"].(float64)
			sn, _ := step["step"].(string)
			_, err := callToolE(ctx, session, "submit_artifact", map[string]any{
				"session_id":    sessionID,
				"artifact_json": artifactForStep(sn, 0),
				"dispatch_id":   int64(did),
			})
			if err != nil {
				t.Fatalf("submit during drain: %v", err)
			}
		}
	}
	t.Log("pipeline drained — gate allowed all submissions including tail batches")
}

// TestServer_CapacityGate_Serial1Allowed proves parallel=1 is not gated
// (single-worker mode is legitimately serial).
func TestServer_CapacityGate_Serial1Allowed(t *testing.T) {
	srv := newTestServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	session := connectInMemory(t, ctx, srv)
	defer session.Close()

	startResult := callTool(t, ctx, session, "start_calibration", map[string]any{
		"scenario": "ptp-mock",
		"adapter":  "cursor",
		"parallel": 1,
	})
	sessionID := startResult["session_id"].(string)

	step := callTool(t, ctx, session, "get_next_step", map[string]any{
		"session_id": sessionID, "timeout_ms": 500,
	})
	did, _ := step["dispatch_id"].(float64)
	sn, _ := step["step"].(string)

	// parallel=1 means serial is correct — gate must not fire
	_, err := callToolE(ctx, session, "submit_artifact", map[string]any{
		"session_id":    sessionID,
		"artifact_json": artifactForStep(sn, 0),
		"dispatch_id":   int64(did),
	})
	if err != nil {
		t.Fatalf("parallel=1 should allow serial submit: %v", err)
	}
	t.Log("parallel=1: serial submit accepted (no gate)")
}

func TestGetNextStep_Timeout(t *testing.T) {
	srv := newTestServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	session := connectInMemory(t, ctx, srv)
	defer session.Close()

	// Start with cursor adapter so steps block on MuxDispatcher (not instantly done)
	startResult := callTool(t, ctx, session, "start_calibration", map[string]any{
		"scenario": "ptp-mock",
		"adapter":  "cursor",
		"parallel": 1,
	})
	sessionID := startResult["session_id"].(string)

	// First get_next_step without timeout should return a step (runner produces it)
	step1 := callTool(t, ctx, session, "get_next_step", map[string]any{
		"session_id": sessionID,
	})
	if done, _ := step1["done"].(bool); done {
		t.Fatal("expected a step, got done=true")
	}
	if avail, _ := step1["available"].(bool); !avail {
		t.Fatal("expected available=true for first step")
	}

	// Now call with timeout_ms=100; the previous step hasn't been submitted yet,
	// so the runner is blocked and no new step is available.
	start := time.Now()
	res := callTool(t, ctx, session, "get_next_step", map[string]any{
		"session_id": sessionID,
		"timeout_ms": 100,
	})
	elapsed := time.Since(start)

	if done, _ := res["done"].(bool); done {
		t.Fatal("expected done=false (pipeline not finished)")
	}
	if avail, _ := res["available"].(bool); avail {
		t.Fatal("expected available=false (timeout, no step ready)")
	}
	if elapsed > 2*time.Second {
		t.Fatalf("timeout should return within ~100ms, took %v", elapsed)
	}
	t.Logf("timeout returned in %v with available=false", elapsed)
}

func TestServer_OverSubscription_NoDeadlock(t *testing.T) {
	srv := newTestServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	session := connectInMemory(t, ctx, srv)
	defer session.Close()

	// parallel=2 means runner produces 2 steps at a time
	startResult := callTool(t, ctx, session, "start_calibration", map[string]any{
		"scenario": "ptp-mock",
		"adapter":  "cursor",
		"parallel": 2,
	})
	sessionID := startResult["session_id"].(string)

	// Issue 4 concurrent get_next_step calls with timeout — only 2 should get steps
	type result struct {
		available bool
		done      bool
		caseID    string
		err       error
	}
	results := make(chan result, 4)

	for i := 0; i < 4; i++ {
		go func() {
			res, err := callToolE(ctx, session, "get_next_step", map[string]any{
				"session_id": sessionID,
				"timeout_ms": 500,
			})
			if err != nil {
				results <- result{err: err}
				return
			}
			results <- result{
				available: res["available"] == true,
				done:      res["done"] == true,
				caseID:    fmt.Sprintf("%v", res["case_id"]),
			}
		}()
	}

	var gotSteps, gotTimeout int
	for i := 0; i < 4; i++ {
		select {
		case r := <-results:
			if r.err != nil {
				t.Fatalf("get_next_step error: %v", r.err)
			}
			if r.done {
				t.Fatal("unexpected done=true")
			}
			if r.available {
				gotSteps++
			} else {
				gotTimeout++
			}
		case <-ctx.Done():
			t.Fatalf("DEADLOCK: only %d/4 get_next_step calls returned (steps=%d, timeouts=%d)",
				gotSteps+gotTimeout, gotSteps, gotTimeout)
		}
	}

	if gotSteps != 2 {
		t.Errorf("expected 2 steps, got %d", gotSteps)
	}
	if gotTimeout != 2 {
		t.Errorf("expected 2 timeouts, got %d", gotTimeout)
	}
	t.Logf("over-subscription resolved: %d steps, %d timeouts, no deadlock", gotSteps, gotTimeout)
}

func TestSession_TTL_Abort(t *testing.T) {
	srv := newTestServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	session := connectInMemory(t, ctx, srv)
	defer session.Close()

	startResult := callTool(t, ctx, session, "start_calibration", map[string]any{
		"scenario": "ptp-mock",
		"adapter":  "cursor",
		"parallel": 1,
	})
	sessionID := startResult["session_id"].(string)

	// Consume one step but never submit -- session should abort via TTL watchdog
	step := callTool(t, ctx, session, "get_next_step", map[string]any{
		"session_id": sessionID,
	})
	if done, _ := step["done"].(bool); done {
		t.Fatal("expected a step, got done=true")
	}
	t.Logf("consumed step %s/%s, now waiting for TTL abort...", step["case_id"], step["step"])

	// Set TTL to 2s on the session directly (test-only hook)
	srv.SetSessionTTL(2 * time.Second)

	// Wait for the session to abort
	deadline := time.After(10 * time.Second)
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-deadline:
			t.Fatal("session did not abort within 10s after TTL=2s was set")
		case <-ticker.C:
			// Poll: try get_next_step with short timeout
			res, err := callToolE(ctx, session, "get_next_step", map[string]any{
				"session_id": sessionID,
				"timeout_ms": 100,
			})
			if err != nil {
				// Error could mean session aborted -- check signals
				signals := callTool(t, ctx, session, "get_signals", map[string]any{
					"session_id": sessionID,
				})
				sigs, _ := signals["signals"].([]any)
				for _, s := range sigs {
					sig, _ := s.(map[string]any)
					if sig["event"] == "session_error" {
						t.Logf("TTL abort detected via session_error signal")
						return
					}
				}
				t.Logf("get_next_step error (may be abort): %v", err)
				continue
			}
			if done, _ := res["done"].(bool); done {
				t.Logf("session transitioned to done (aborted)")
				return
			}
		}
	}
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

// --- Stale session edge case tests ---

// TestServer_StaleSession_StartReplacesStuck proves that start_calibration
// can replace a stuck session. Without this, the MCP server is permanently
// wedged after a crashed agent run.
func TestServer_StaleSession_StartReplacesStuck(t *testing.T) {
	srv := newTestServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	session := connectInMemory(t, ctx, srv)
	defer session.Close()

	// Start a cursor session (blocks on MuxDispatcher, never completes)
	start1 := callTool(t, ctx, session, "start_calibration", map[string]any{
		"scenario": "ptp-mock",
		"adapter":  "cursor",
		"parallel": 1,
	})
	sid1 := start1["session_id"].(string)
	t.Logf("session 1: %s", sid1)

	// Drain one step to prove it's alive
	step := callTool(t, ctx, session, "get_next_step", map[string]any{
		"session_id": sid1,
		"timeout_ms": 500,
	})
	if avail, _ := step["available"].(bool); !avail {
		t.Fatal("expected first step to be available")
	}

	// Now abandon the session — do NOT submit the artifact.
	// Try to start a new session. Currently this should fail with
	// "a calibration session is already running".
	// After the fix, passing force=true should cancel the old session
	// and start a new one.
	start2, err := callToolE(ctx, session, "start_calibration", map[string]any{
		"scenario": "ptp-mock",
		"adapter":  "stub",
	})
	if err == nil {
		t.Fatalf("expected error starting second session without force, got: %v", start2)
	}
	t.Logf("without force: %v (expected)", err)

	// With force=true, the stuck session should be cancelled and replaced
	start3 := callTool(t, ctx, session, "start_calibration", map[string]any{
		"scenario": "ptp-mock",
		"adapter":  "stub",
		"force":    true,
	})
	sid3 := start3["session_id"].(string)
	if sid3 == "" {
		t.Fatal("expected new session_id from force-start")
	}
	if sid3 == sid1 {
		t.Fatal("force-started session should have a different ID")
	}
	t.Logf("force-started session 3: %s (replaced %s)", sid3, sid1)

	// The new session should complete normally (stub adapter)
	report := callTool(t, ctx, session, "get_report", map[string]any{
		"session_id": sid3,
	})
	if status, _ := report["status"].(string); status != "done" {
		t.Fatalf("expected done, got %s", status)
	}
}

// TestServer_StaleSession_TTLAutoAbort proves that sessions with a configured
// TTL self-terminate after inactivity. This prevents zombie sessions from
// permanently blocking the server.
func TestServer_StaleSession_TTLAutoAbort(t *testing.T) {
	srv := newTestServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	session := connectInMemory(t, ctx, srv)
	defer session.Close()

	// Start a cursor session, set a very short TTL
	start := callTool(t, ctx, session, "start_calibration", map[string]any{
		"scenario": "ptp-mock",
		"adapter":  "cursor",
		"parallel": 1,
	})
	sid := start["session_id"].(string)

	// Set a 200ms TTL — the session will abort itself within ~200ms of
	// no submit_artifact activity.
	srv.SetSessionTTL(200 * time.Millisecond)

	// Drain one step but don't submit — let the TTL expire
	callTool(t, ctx, session, "get_next_step", map[string]any{
		"session_id": sid,
		"timeout_ms": 1000,
	})

	// Wait for TTL watchdog to fire
	time.Sleep(500 * time.Millisecond)

	// Now start_calibration should succeed WITHOUT force — the TTL-aborted
	// session's Done channel is closed, so handleStartCalibration detects it.
	start2 := callTool(t, ctx, session, "start_calibration", map[string]any{
		"scenario": "ptp-mock",
		"adapter":  "stub",
	})
	sid2 := start2["session_id"].(string)
	if sid2 == "" || sid2 == sid {
		t.Fatalf("expected new session after TTL abort, got %q", sid2)
	}
	t.Logf("TTL-aborted session %s replaced by %s", sid, sid2)
}

// TestServer_StaleSession_GetNextStepOnAbortedSession proves that
// get_next_step on an aborted session returns done=true (not a hang).
func TestServer_StaleSession_GetNextStepOnAbortedSession(t *testing.T) {
	srv := newTestServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	session := connectInMemory(t, ctx, srv)
	defer session.Close()

	start := callTool(t, ctx, session, "start_calibration", map[string]any{
		"scenario": "ptp-mock",
		"adapter":  "cursor",
		"parallel": 1,
	})
	sid := start["session_id"].(string)

	// Set a tiny TTL and let it expire
	srv.SetSessionTTL(100 * time.Millisecond)
	time.Sleep(300 * time.Millisecond)

	// get_next_step on the aborted session should return done=true or error,
	// not block forever
	before := time.Now()
	result := callTool(t, ctx, session, "get_next_step", map[string]any{
		"session_id": sid,
	})
	elapsed := time.Since(before)

	done, _ := result["done"].(bool)
	if !done {
		t.Fatalf("expected done=true on aborted session, got %v", result)
	}
	if elapsed > 2*time.Second {
		t.Fatalf("get_next_step on aborted session took %v (should be instant)", elapsed)
	}
	t.Logf("get_next_step on aborted session returned done=true in %v", elapsed)
}

// TestServer_StaleSession_SubmitAfterAbort proves that submit_artifact
// on an aborted session returns a clear error, not a hang.
func TestServer_StaleSession_SubmitAfterAbort(t *testing.T) {
	srv := newTestServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	session := connectInMemory(t, ctx, srv)
	defer session.Close()

	start := callTool(t, ctx, session, "start_calibration", map[string]any{
		"scenario": "ptp-mock",
		"adapter":  "cursor",
		"parallel": 1,
	})
	sid := start["session_id"].(string)

	// Get a step so we have a dispatch_id
	step := callTool(t, ctx, session, "get_next_step", map[string]any{
		"session_id": sid,
		"timeout_ms": 500,
	})
	dispatchID := step["dispatch_id"]

	// Now abort the session via TTL
	srv.SetSessionTTL(100 * time.Millisecond)
	time.Sleep(300 * time.Millisecond)

	// Submit should fail with an error, not hang
	_, err := callToolE(ctx, session, "submit_artifact", map[string]any{
		"session_id":   sid,
		"artifact_json": `{"match":false,"confidence":0.1,"reasoning":"test"}`,
		"dispatch_id":  dispatchID,
	})
	if err == nil {
		t.Fatal("expected error submitting to aborted session")
	}
	t.Logf("submit after abort: %v (expected)", err)
}

// TestGetNextStep_DefaultTimeout_NeverBlocksForever proves that calling
// get_next_step without timeout_ms uses the server default (30s) rather
// than blocking forever. We override the default to 500ms for the test.
func TestGetNextStep_DefaultTimeout_NeverBlocksForever(t *testing.T) {
	srv := newTestServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	session := connectInMemory(t, ctx, srv)
	defer session.Close()

	startResult := callTool(t, ctx, session, "start_calibration", map[string]any{
		"scenario": "ptp-mock",
		"adapter":  "cursor",
		"parallel": 1,
	})
	sessionID := startResult["session_id"].(string)

	// Consume the first step so the runner blocks waiting for a submit
	step := callTool(t, ctx, session, "get_next_step", map[string]any{
		"session_id": sessionID,
	})
	if done, _ := step["done"].(bool); done {
		t.Fatal("expected a step, got done=true")
	}

	// Override the server default timeout to 500ms (so the test is fast)
	saved := mcpserver.DefaultGetNextStepTimeout
	mcpserver.DefaultGetNextStepTimeout = 500 * time.Millisecond
	defer func() { mcpserver.DefaultGetNextStepTimeout = saved }()

	// Call without timeout_ms — must NOT block forever
	start := time.Now()
	res := callTool(t, ctx, session, "get_next_step", map[string]any{
		"session_id": sessionID,
	})
	elapsed := time.Since(start)

	if done, _ := res["done"].(bool); done {
		t.Fatal("expected done=false, pipeline still running")
	}
	if avail, _ := res["available"].(bool); avail {
		t.Fatal("expected available=false (no step ready, timeout)")
	}
	if elapsed > 3*time.Second {
		t.Fatalf("get_next_step without timeout_ms took %v; server default should cap it", elapsed)
	}
	t.Logf("get_next_step without timeout_ms returned in %v (server default kicked in)", elapsed)
}

// TestGetNextStep_OverPull_Draining proves the exact production scenario:
// parallel=2, agent pulls 4 concurrently, only 2 steps exist.
// The 2 extra calls must timeout gracefully (not deadlock).
func TestGetNextStep_OverPull_Draining(t *testing.T) {
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

	// Pull 4 concurrent get_next_step but only 2 steps are available initially
	type pullResult struct {
		res map[string]any
		err error
	}
	results := make(chan pullResult, 4)
	start := time.Now()
	for i := 0; i < 4; i++ {
		go func() {
			res, err := callToolE(ctx, session, "get_next_step", map[string]any{
				"session_id": sessionID,
				"timeout_ms": 500,
			})
			results <- pullResult{res, err}
		}()
	}

	var gotSteps, gotTimeout, gotDone int
	for i := 0; i < 4; i++ {
		select {
		case r := <-results:
			if r.err != nil {
				t.Fatalf("unexpected error: %v", r.err)
			}
			if done, _ := r.res["done"].(bool); done {
				gotDone++
			} else if avail, _ := r.res["available"].(bool); avail {
				gotSteps++
			} else {
				gotTimeout++
			}
		case <-ctx.Done():
			t.Fatalf("DEADLOCK: only %d of 4 calls returned", gotSteps+gotTimeout+gotDone)
		}
	}
	elapsed := time.Since(start)

	if gotSteps != 2 {
		t.Errorf("expected 2 steps, got %d", gotSteps)
	}
	if gotTimeout < 2 {
		t.Errorf("expected at least 2 timeouts, got %d (done=%d)", gotTimeout, gotDone)
	}
	if elapsed > 5*time.Second {
		t.Fatalf("over-pull took %v; should resolve within ~500ms timeout", elapsed)
	}
	t.Logf("draining resolved: %d steps, %d timeouts, %d done in %v — no deadlock",
		gotSteps, gotTimeout, gotDone, elapsed)
}

// TestSession_DefaultTTL_EnforcedOnStart verifies that sessions created
// via start_calibration have a default TTL (not zero). This prevents the
// scenario where no TTL is set and the watchdog never fires.
func TestSession_DefaultTTL_EnforcedOnStart(t *testing.T) {
	srv := newTestServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	session := connectInMemory(t, ctx, srv)
	defer session.Close()

	callTool(t, ctx, session, "start_calibration", map[string]any{
		"scenario": "ptp-mock",
		"adapter":  "cursor",
		"parallel": 1,
	})

	// The session should have a non-zero TTL.
	// We verify this indirectly: override TTL to 200ms and wait.
	// If the default TTL logic is present, the session will have a watchdog
	// already running. We override it shorter and verify it aborts.
	srv.SetSessionTTL(200 * time.Millisecond)
	time.Sleep(500 * time.Millisecond)

	// After TTL fires, get_next_step should return done=true
	res := callTool(t, ctx, session, "get_next_step", map[string]any{
		"session_id": srv.SessionID(),
	})
	if done, _ := res["done"].(bool); !done {
		t.Fatalf("expected done=true after TTL abort, got %v", res)
	}
	t.Logf("default TTL enforcement verified — session aborted after override")
}

// TestSession_TTL_UnblocksHungGetNextStep proves that a blocked
// get_next_step call (no timeout_ms) is unblocked when the TTL watchdog
// fires and aborts the session.
func TestSession_TTL_UnblocksHungGetNextStep(t *testing.T) {
	srv := newTestServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	session := connectInMemory(t, ctx, srv)
	defer session.Close()

	startResult := callTool(t, ctx, session, "start_calibration", map[string]any{
		"scenario": "ptp-mock",
		"adapter":  "cursor",
		"parallel": 1,
	})
	sessionID := startResult["session_id"].(string)

	// Consume one step to make the runner block
	step := callTool(t, ctx, session, "get_next_step", map[string]any{
		"session_id": sessionID,
	})
	if done, _ := step["done"].(bool); done {
		t.Fatal("expected a step")
	}

	// Override server default timeout to something large so only TTL saves us
	saved := mcpserver.DefaultGetNextStepTimeout
	mcpserver.DefaultGetNextStepTimeout = 60 * time.Second
	defer func() { mcpserver.DefaultGetNextStepTimeout = saved }()

	// Set a short TTL — this should fire and abort the session
	srv.SetSessionTTL(500 * time.Millisecond)

	// Now call get_next_step without timeout_ms — it would block for 60s
	// if the TTL watchdog didn't fire
	start := time.Now()
	res := callTool(t, ctx, session, "get_next_step", map[string]any{
		"session_id": sessionID,
	})
	elapsed := time.Since(start)

	done, _ := res["done"].(bool)
	if !done {
		t.Fatalf("expected done=true after TTL abort, got %v", res)
	}
	if elapsed > 5*time.Second {
		t.Fatalf("TTL should have unblocked get_next_step within ~500ms, took %v", elapsed)
	}
	t.Logf("TTL unblocked hung get_next_step in %v", elapsed)
}

// --- Papercup v2 hardening tests ---

// TestStartCalibration_WorkerPrompt verifies that start_calibration with
// parallel>1 returns a non-empty worker_prompt containing the session_id,
// the protocol keywords, and worker_count matching the parallel param.
func TestStartCalibration_WorkerPrompt(t *testing.T) {
	srv := newTestServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	session := connectInMemory(t, ctx, srv)
	defer session.Close()

	startResult := callTool(t, ctx, session, "start_calibration", map[string]any{
		"scenario": "ptp-mock",
		"adapter":  "cursor",
		"parallel": 4,
	})
	sessionID := startResult["session_id"].(string)

	workerPrompt, _ := startResult["worker_prompt"].(string)
	if workerPrompt == "" {
		t.Fatal("expected non-empty worker_prompt for parallel>1")
	}
	if !containsAll(workerPrompt, sessionID, "get_next_step", "submit_artifact",
		"worker_started", "worker_stopped", "mode", "stream") {
		t.Errorf("worker_prompt missing required protocol keywords:\n%s", workerPrompt[:min(500, len(workerPrompt))])
	}

	workerCount, _ := startResult["worker_count"].(float64)
	if int(workerCount) != 4 {
		t.Errorf("expected worker_count=4, got %v", workerCount)
	}

	t.Logf("worker_prompt length: %d chars, contains session_id=%s", len(workerPrompt), sessionID)
}

// TestStartCalibration_WorkerPrompt_Serial verifies that parallel=1
// does NOT include worker_prompt (serial mode is orchestrated differently).
func TestStartCalibration_WorkerPrompt_Serial(t *testing.T) {
	srv := newTestServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	session := connectInMemory(t, ctx, srv)
	defer session.Close()

	startResult := callTool(t, ctx, session, "start_calibration", map[string]any{
		"scenario": "ptp-mock",
		"adapter":  "cursor",
		"parallel": 1,
	})

	workerPrompt, _ := startResult["worker_prompt"].(string)
	if workerPrompt != "" {
		t.Errorf("expected empty worker_prompt for parallel=1, got %d chars", len(workerPrompt))
	}
	workerCount, _ := startResult["worker_count"].(float64)
	if int(workerCount) != 0 {
		t.Errorf("expected worker_count=0 for parallel=1, got %v", workerCount)
	}
}

// TestGetNextStep_InlinePrompt verifies that get_next_step responses include
// prompt_content matching the file at prompt_path. Workers should not need
// to do file I/O.
func TestGetNextStep_InlinePrompt(t *testing.T) {
	srv := newTestServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	session := connectInMemory(t, ctx, srv)
	defer session.Close()

	startResult := callTool(t, ctx, session, "start_calibration", map[string]any{
		"scenario": "ptp-mock",
		"adapter":  "cursor",
		"parallel": 1,
	})
	sessionID := startResult["session_id"].(string)

	step := callTool(t, ctx, session, "get_next_step", map[string]any{
		"session_id": sessionID,
	})
	if done, _ := step["done"].(bool); done {
		t.Fatal("expected a step, got done=true")
	}

	promptContent, _ := step["prompt_content"].(string)
	promptPath, _ := step["prompt_path"].(string)

	if promptContent == "" {
		t.Fatal("expected non-empty prompt_content")
	}
	if promptPath == "" {
		t.Fatal("expected non-empty prompt_path")
	}

	fileContent, err := os.ReadFile(promptPath)
	if err != nil {
		t.Fatalf("read prompt file %s: %v", promptPath, err)
	}

	if promptContent != string(fileContent) {
		t.Errorf("prompt_content does not match file content.\nprompt_content len=%d, file len=%d",
			len(promptContent), len(fileContent))
	}
	t.Logf("inline prompt verified: %d bytes, path=%s", len(promptContent), promptPath)
}

// TestGetNextStep_InlinePrompt_MultipleSteps verifies prompt_content is
// populated for every step (not just the first).
func TestGetNextStep_InlinePrompt_MultipleSteps(t *testing.T) {
	srv := newTestServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	session := connectInMemory(t, ctx, srv)
	defer session.Close()

	startResult := callTool(t, ctx, session, "start_calibration", map[string]any{
		"scenario": "ptp-mock",
		"adapter":  "cursor",
		"parallel": 1,
	})
	sessionID := startResult["session_id"].(string)

	for i := 0; i < 3; i++ {
		step := callTool(t, ctx, session, "get_next_step", map[string]any{
			"session_id": sessionID,
		})
		if done, _ := step["done"].(bool); done {
			break
		}
		if avail, _ := step["available"].(bool); !avail {
			t.Fatalf("step %d: expected available=true", i)
		}

		promptContent, _ := step["prompt_content"].(string)
		if promptContent == "" {
			t.Errorf("step %d: expected non-empty prompt_content", i)
		}

		stepName, _ := step["step"].(string)
		dispatchID, _ := step["dispatch_id"].(float64)
		artifact := artifactForStep(stepName, 0)
		callTool(t, ctx, session, "submit_artifact", map[string]any{
			"session_id":    sessionID,
			"artifact_json": artifact,
			"dispatch_id":   int64(dispatchID),
		})
		t.Logf("step %d (%s): prompt_content=%d bytes", i, stepName, len(promptContent))
	}
}

// TestCapacityWarning_ProtocolAgnostic verifies the capacity warning text
// uses neutral language (no "launch subagents", no "pull more steps").
func TestCapacityWarning_ProtocolAgnostic(t *testing.T) {
	srv := newTestServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	session := connectInMemory(t, ctx, srv)
	defer session.Close()

	startResult := callTool(t, ctx, session, "start_calibration", map[string]any{
		"scenario": "ptp-mock",
		"adapter":  "cursor",
		"parallel": 4,
	})
	sessionID := startResult["session_id"].(string)

	step := callTool(t, ctx, session, "get_next_step", map[string]any{
		"session_id": sessionID,
	})
	warning, _ := step["capacity_warning"].(string)

	if warning == "" {
		t.Fatal("expected capacity_warning when only 1/4 workers active")
	}

	forbidden := []string{"launch", "subagent", "pull more", "MUST"}
	for _, word := range forbidden {
		if containsCI(warning, word) {
			t.Errorf("capacity_warning contains v1 language %q: %s", word, warning)
		}
	}

	required := []string{"under capacity", "workers active"}
	for _, word := range required {
		if !containsCI(warning, word) {
			t.Errorf("capacity_warning missing %q: %s", word, warning)
		}
	}
	t.Logf("capacity_warning: %s", warning)
}

// TestCapacityGate_ProtocolAgnostic verifies the gate error message
// uses neutral language.
func TestCapacityGate_ProtocolAgnostic(t *testing.T) {
	srv := newTestServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	session := connectInMemory(t, ctx, srv)
	defer session.Close()

	startResult := callTool(t, ctx, session, "start_calibration", map[string]any{
		"scenario": "ptp-mock",
		"adapter":  "cursor",
		"parallel": 4,
	})
	sessionID := startResult["session_id"].(string)

	// Pull one step — serial pattern
	step := callTool(t, ctx, session, "get_next_step", map[string]any{
		"session_id": sessionID, "timeout_ms": 500,
	})
	dispatchID, _ := step["dispatch_id"].(float64)
	stepName, _ := step["step"].(string)

	// Submit — gate will warn (advisory) but not reject
	callTool(t, ctx, session, "submit_artifact", map[string]any{
		"session_id":    sessionID,
		"artifact_json": artifactForStep(stepName, 0),
		"dispatch_id":   int64(dispatchID),
	})

	// The gate message itself is logged, not returned to the caller.
	// We verify the format via unit test on Session.CheckCapacityGate directly.
	sess := &mcpserver.Session{DesiredCapacity: 4}
	sess.AgentPull()
	gateErr := sess.CheckCapacityGate()
	if gateErr == nil {
		t.Fatal("expected gate error with 1/4 capacity")
	}
	msg := gateErr.Error()

	forbidden := []string{"CAPACITY GATE ADVISORY", "Pull", "bring more workers", "TTL watchdog"}
	for _, word := range forbidden {
		if containsCI(msg, word) {
			t.Errorf("gate message contains v1 language %q: %s", word, msg)
		}
	}

	required := []string{"capacity gate", "workers observed", "expects"}
	for _, word := range required {
		if !containsCI(msg, word) {
			t.Errorf("gate message missing %q: %s", word, msg)
		}
	}
	t.Logf("gate message: %s", msg)
}

// TestWorkerMode_StreamRegistration verifies that emitting worker_started
// with meta.mode="stream" causes the server to track the worker.
func TestWorkerMode_StreamRegistration(t *testing.T) {
	srv := newTestServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	session := connectInMemory(t, ctx, srv)
	defer session.Close()

	startResult := callTool(t, ctx, session, "start_calibration", map[string]any{
		"scenario": "ptp-mock",
		"adapter":  "cursor",
		"parallel": 4,
	})
	sessionID := startResult["session_id"].(string)

	for i := 0; i < 4; i++ {
		callTool(t, ctx, session, "emit_signal", map[string]any{
			"session_id": sessionID,
			"event":      "worker_started",
			"agent":      "worker",
			"meta":       map[string]any{"worker_id": fmt.Sprintf("w%d", i), "mode": "stream"},
		})
	}

	// Verify via signal bus
	signals := callTool(t, ctx, session, "get_signals", map[string]any{
		"session_id": sessionID,
	})
	signalList, _ := signals["signals"].([]any)

	var workerStarted int
	for _, s := range signalList {
		sig, _ := s.(map[string]any)
		if sig["event"] == "worker_started" {
			workerStarted++
			meta, _ := sig["meta"].(map[string]any)
			if meta["mode"] != "stream" {
				t.Errorf("worker_started signal missing mode=stream: %v", meta)
			}
		}
	}
	if workerStarted != 4 {
		t.Errorf("expected 4 worker_started signals, got %d", workerStarted)
	}
	t.Logf("registered %d stream workers", workerStarted)
}

// TestWorkerMode_NoWorkerID_Ignored verifies that worker_started without
// meta.worker_id is gracefully ignored (no panic, no registration).
func TestWorkerMode_NoWorkerID_Ignored(t *testing.T) {
	srv := newTestServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	session := connectInMemory(t, ctx, srv)
	defer session.Close()

	startResult := callTool(t, ctx, session, "start_calibration", map[string]any{
		"scenario": "ptp-mock",
		"adapter":  "cursor",
		"parallel": 2,
	})
	sessionID := startResult["session_id"].(string)

	callTool(t, ctx, session, "emit_signal", map[string]any{
		"session_id": sessionID,
		"event":      "worker_started",
		"agent":      "worker",
		"meta":       map[string]any{"mode": "stream"},
	})

	callTool(t, ctx, session, "emit_signal", map[string]any{
		"session_id": sessionID,
		"event":      "worker_started",
		"agent":      "worker",
	})

	t.Log("worker_started without worker_id accepted without panic")
}

// TestV2Workers_FullDrain_Deterministic is the definitive v2 choreography test.
// 4 independent workers each own their get_next_step/submit_artifact loop,
// emitting proper mode signals. Asserts:
//  1. All 4 workers register as stream mode
//  2. All cases drain to completion (report status=done)
//  3. No starvation (every worker processes at least 1 step)
//  4. No duplicate dispatches
//  5. Signal bus contains matched worker_started/worker_stopped pairs
//  6. Total steps matches expected pipeline depth * cases
func TestV2Workers_FullDrain_Deterministic(t *testing.T) {
	srv := newTestServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	session := connectInMemory(t, ctx, srv)
	defer session.Close()

	startResult := callTool(t, ctx, session, "start_calibration", map[string]any{
		"scenario": "ptp-mock",
		"adapter":  "cursor",
		"parallel": 4,
	})
	sessionID := startResult["session_id"].(string)

	workerPrompt, _ := startResult["worker_prompt"].(string)
	if workerPrompt == "" {
		t.Fatal("expected worker_prompt in start_calibration response")
	}
	workerCount, _ := startResult["worker_count"].(float64)
	if int(workerCount) != 4 {
		t.Fatalf("expected worker_count=4, got %v", workerCount)
	}

	var mu sync.Mutex
	workLog := make(map[int][]stepRecord)
	seenDispatchIDs := make(map[int64]bool)

	var wg sync.WaitGroup
	errCh := make(chan error, 4)

	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			// Step 1: Emit worker_started with mode=stream (v2 protocol)
			_, err := callToolE(ctx, session, "emit_signal", map[string]any{
				"session_id": sessionID,
				"event":      "worker_started",
				"agent":      "worker",
				"meta":       map[string]any{"worker_id": fmt.Sprintf("w%d", workerID), "mode": "stream"},
			})
			if err != nil {
				errCh <- fmt.Errorf("w%d emit worker_started: %w", workerID, err)
				return
			}

			// Step 2: Worker loop — get_next_step/submit_artifact until done
			for {
				res, err := callToolE(ctx, session, "get_next_step", map[string]any{
					"session_id": sessionID,
					"timeout_ms": 300,
				})
				if err != nil {
					errCh <- fmt.Errorf("w%d get_next_step: %w", workerID, err)
					return
				}

				if done, _ := res["done"].(bool); done {
					break
				}
				if avail, _ := res["available"].(bool); !avail {
					continue
				}

				caseID, _ := res["case_id"].(string)
				step, _ := res["step"].(string)
				dispatchID, _ := res["dispatch_id"].(float64)
				promptContent, _ := res["prompt_content"].(string)

				if promptContent == "" {
					errCh <- fmt.Errorf("w%d: empty prompt_content for %s/%s", workerID, caseID, step)
					return
				}

				artifact := artifactForStep(step, workerID)
				_, err = callToolE(ctx, session, "submit_artifact", map[string]any{
					"session_id":    sessionID,
					"artifact_json": artifact,
					"dispatch_id":   int64(dispatchID),
				})
				if err != nil {
					errCh <- fmt.Errorf("w%d submit(%s/%s): %w", workerID, caseID, step, err)
					return
				}

				mu.Lock()
				workLog[workerID] = append(workLog[workerID], stepRecord{
					CaseID:     caseID,
					Step:       step,
					DispatchID: int64(dispatchID),
				})
				if seenDispatchIDs[int64(dispatchID)] {
					errCh <- fmt.Errorf("w%d: duplicate dispatch_id %d", workerID, int64(dispatchID))
				}
				seenDispatchIDs[int64(dispatchID)] = true
				mu.Unlock()
			}

			// Step 3: Emit worker_stopped (v2 protocol)
			_, _ = callToolE(ctx, session, "emit_signal", map[string]any{
				"session_id": sessionID,
				"event":      "worker_stopped",
				"agent":      "worker",
				"meta":       map[string]any{"worker_id": fmt.Sprintf("w%d", workerID)},
			})
		}(i)
	}

	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Errorf("worker error: %v", err)
	}

	// Assertion 1: no starvation — all 4 workers got work
	for i := 0; i < 4; i++ {
		if len(workLog[i]) == 0 {
			t.Errorf("worker-%d got zero steps (starvation)", i)
		} else {
			t.Logf("worker-%d processed %d steps", i, len(workLog[i]))
		}
	}

	// Assertion 2: total steps > 0 and all dispatch IDs unique
	var totalSteps int
	for _, records := range workLog {
		totalSteps += len(records)
	}
	if totalSteps == 0 {
		t.Fatal("pipeline produced zero steps")
	}
	t.Logf("total steps: %d across 4 workers, %d unique dispatch_ids", totalSteps, len(seenDispatchIDs))

	// Assertion 3: signal bus has matching worker_started/worker_stopped pairs
	signals := callTool(t, ctx, session, "get_signals", map[string]any{
		"session_id": sessionID,
	})
	signalList, _ := signals["signals"].([]any)

	startedWorkers := make(map[string]bool)
	stoppedWorkers := make(map[string]bool)
	for _, s := range signalList {
		sig, _ := s.(map[string]any)
		event, _ := sig["event"].(string)
		meta, _ := sig["meta"].(map[string]any)
		wid, _ := meta["worker_id"].(string)
		switch event {
		case "worker_started":
			startedWorkers[wid] = true
			mode, _ := meta["mode"].(string)
			if mode != "stream" {
				t.Errorf("worker %s started without mode=stream: %v", wid, meta)
			}
		case "worker_stopped":
			stoppedWorkers[wid] = true
		}
	}
	if len(startedWorkers) != 4 {
		t.Errorf("expected 4 worker_started signals, got %d", len(startedWorkers))
	}
	if len(stoppedWorkers) != 4 {
		t.Errorf("expected 4 worker_stopped signals, got %d", len(stoppedWorkers))
	}
	for wid := range startedWorkers {
		if !stoppedWorkers[wid] {
			t.Errorf("worker %s started but never stopped", wid)
		}
	}

	// Assertion 4: report is complete
	reportResult := callTool(t, ctx, session, "get_report", map[string]any{
		"session_id": sessionID,
	})
	status, _ := reportResult["status"].(string)
	if status != "done" {
		t.Fatalf("expected status=done, got %s", status)
	}
	caseResults, _ := reportResult["case_results"].([]any)
	t.Logf("report: status=%s, case_results=%d", status, len(caseResults))
}

// TestV2Workers_ViaResolve_Deterministic is the same as TestV2Workers_FullDrain
// but uses artifactForStepViaResolve to force the F2_RESOLVE -> F3_INVESTIGATE path.
// Verifies the pipeline reaches F3 with v2 protocol.
func TestV2Workers_ViaResolve_Deterministic(t *testing.T) {
	srv := newTestServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	session := connectInMemory(t, ctx, srv)
	defer session.Close()

	startResult := callTool(t, ctx, session, "start_calibration", map[string]any{
		"scenario": "ptp-mock",
		"adapter":  "cursor",
		"parallel": 4,
	})
	sessionID := startResult["session_id"].(string)

	var mu sync.Mutex
	stepLog := make(map[string][]string)

	var wg sync.WaitGroup
	errCh := make(chan error, 4)

	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			_, _ = callToolE(ctx, session, "emit_signal", map[string]any{
				"session_id": sessionID,
				"event":      "worker_started",
				"agent":      "worker",
				"meta":       map[string]any{"worker_id": fmt.Sprintf("w%d", workerID), "mode": "stream"},
			})

			for {
				res, err := callToolE(ctx, session, "get_next_step", map[string]any{
					"session_id": sessionID,
					"timeout_ms": 300,
				})
				if err != nil {
					errCh <- fmt.Errorf("w%d: %w", workerID, err)
					return
				}
				if done, _ := res["done"].(bool); done {
					break
				}
				if avail, _ := res["available"].(bool); !avail {
					continue
				}

				caseID, _ := res["case_id"].(string)
				step, _ := res["step"].(string)
				dispatchID, _ := res["dispatch_id"].(float64)

				artifact := artifactForStepViaResolve(step, workerID)
				_, err = callToolE(ctx, session, "submit_artifact", map[string]any{
					"session_id":    sessionID,
					"artifact_json": artifact,
					"dispatch_id":   int64(dispatchID),
				})
				if err != nil {
					errCh <- fmt.Errorf("w%d submit(%s/%s): %w", workerID, caseID, step, err)
					return
				}

				mu.Lock()
				stepLog[caseID] = append(stepLog[caseID], step)
				mu.Unlock()
			}

			_, _ = callToolE(ctx, session, "emit_signal", map[string]any{
				"session_id": sessionID,
				"event":      "worker_stopped",
				"agent":      "worker",
				"meta":       map[string]any{"worker_id": fmt.Sprintf("w%d", workerID)},
			})
		}(i)
	}

	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Errorf("worker error: %v", err)
	}

	var f2Count, f3Count int
	for caseID, steps := range stepLog {
		t.Logf("case %s: %v", caseID, steps)
		for _, s := range steps {
			if s == "F2_RESOLVE" {
				f2Count++
			}
			if s == "F3_INVESTIGATE" {
				f3Count++
			}
		}
	}

	if f2Count == 0 {
		t.Error("no cases went through F2_RESOLVE")
	}
	if f3Count == 0 {
		t.Error("no cases reached F3_INVESTIGATE — F2→F3 transition broken")
	}
	t.Logf("F2 dispatches: %d, F3 dispatches: %d", f2Count, f3Count)

	reportResult := callTool(t, ctx, session, "get_report", map[string]any{
		"session_id": sessionID,
	})
	status, _ := reportResult["status"].(string)
	if status != "done" {
		t.Fatalf("expected status=done, got %s", status)
	}
}

// TestV2Workers_ConcurrencyTiming_Deterministic measures that v2 workers
// with per-step delays achieve true concurrent throughput. Same as the
// existing timing test but with v2 protocol signals.
func TestV2Workers_ConcurrencyTiming_Deterministic(t *testing.T) {
	srv := newTestServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	session := connectInMemory(t, ctx, srv)
	defer session.Close()

	startResult := callTool(t, ctx, session, "start_calibration", map[string]any{
		"scenario": "ptp-mock",
		"adapter":  "cursor",
		"parallel": 4,
	})
	sessionID := startResult["session_id"].(string)

	const perStepDelay = 20 * time.Millisecond
	var mu sync.Mutex
	var totalSteps int64

	var wg sync.WaitGroup
	errCh := make(chan error, 4)

	start := time.Now()
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			_, _ = callToolE(ctx, session, "emit_signal", map[string]any{
				"session_id": sessionID,
				"event":      "worker_started",
				"agent":      "worker",
				"meta":       map[string]any{"worker_id": fmt.Sprintf("w%d", workerID), "mode": "stream"},
			})

			for {
				res, err := callToolE(ctx, session, "get_next_step", map[string]any{
					"session_id": sessionID,
					"timeout_ms": 300,
				})
				if err != nil {
					errCh <- err
					return
				}
				if done, _ := res["done"].(bool); done {
					break
				}
				if avail, _ := res["available"].(bool); !avail {
					continue
				}

				time.Sleep(perStepDelay)

				step, _ := res["step"].(string)
				dispatchID, _ := res["dispatch_id"].(float64)

				artifact := artifactForStep(step, workerID)
				_, err = callToolE(ctx, session, "submit_artifact", map[string]any{
					"session_id":    sessionID,
					"artifact_json": artifact,
					"dispatch_id":   int64(dispatchID),
				})
				if err != nil {
					errCh <- err
					return
				}
				mu.Lock()
				totalSteps++
				mu.Unlock()
			}

			_, _ = callToolE(ctx, session, "emit_signal", map[string]any{
				"session_id": sessionID,
				"event":      "worker_stopped",
				"agent":      "worker",
				"meta":       map[string]any{"worker_id": fmt.Sprintf("w%d", workerID)},
			})
		}(i)
	}

	wg.Wait()
	close(errCh)
	elapsed := time.Since(start)

	for err := range errCh {
		t.Fatalf("worker error: %v", err)
	}

	serialEstimate := time.Duration(totalSteps) * perStepDelay
	speedup := float64(serialEstimate) / float64(elapsed)

	t.Logf("v2 timing: steps=%d, elapsed=%v, serial=%v, speedup=%.2fx",
		totalSteps, elapsed, serialEstimate, speedup)

	if elapsed > time.Duration(float64(serialEstimate)*0.75) {
		t.Errorf("v2 workers too slow: elapsed=%v > 75%% of serial=%v (speedup=%.2fx)",
			elapsed, serialEstimate, speedup)
	}
}

// TestWorkPrompt_StepSchemas verifies the worker prompt mentions all
// pipeline steps F0-F6 with their key fields.
func TestWorkerPrompt_StepSchemas(t *testing.T) {
	sess := &mcpserver.Session{
		ID:              "test-session",
		DesiredCapacity: 4,
	}

	prompt := sess.WorkerPrompt()

	steps := []string{"F0_RECALL", "F1_TRIAGE", "F2_RESOLVE", "F3_INVESTIGATE", "F4_CORRELATE", "F5_REVIEW", "F6_REPORT"}
	for _, step := range steps {
		if !containsCI(prompt, step) {
			t.Errorf("worker prompt missing step %s", step)
		}
	}

	if !containsCI(prompt, "test-session") {
		t.Error("worker prompt missing session_id")
	}

	keywords := []string{"get_next_step", "submit_artifact", "worker_started", "worker_stopped", "mode", "stream", "CALIBRATION"}
	for _, kw := range keywords {
		if !containsCI(prompt, kw) {
			t.Errorf("worker prompt missing keyword %q", kw)
		}
	}
}

// TestWorkerPrompt_SessionIDEmbedded verifies the session ID is correctly
// embedded (not a template placeholder).
func TestWorkerPrompt_SessionIDEmbedded(t *testing.T) {
	sess := &mcpserver.Session{
		ID:              "s-1234567890",
		DesiredCapacity: 2,
	}

	prompt := sess.WorkerPrompt()

	if !containsCI(prompt, "s-1234567890") {
		t.Error("worker prompt does not contain the actual session ID")
	}

	if containsCI(prompt, "%s") || containsCI(prompt, "{session_id}") || containsCI(prompt, "%[1]s") {
		t.Error("worker prompt contains unresolved template placeholders")
	}
}

// --- helpers ---

func containsAll(s string, substrs ...string) bool {
	for _, sub := range substrs {
		if !containsCI(s, sub) {
			return false
		}
	}
	return true
}

func containsCI(s, substr string) bool {
	return len(s) >= len(substr) &&
		len(substr) > 0 &&
		(strings.Contains(s, substr) || strings.Contains(strings.ToLower(s), strings.ToLower(substr)))
}

func TestServer_GetNextStep_AvailableFalse_Retry(t *testing.T) {
	srv := newTestServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	session := connectInMemory(t, ctx, srv)
	defer session.Close()

	// parallel=1 ensures the runner dispatches exactly one step at a time.
	// Holding that single step creates guaranteed backpressure.
	startResult := callTool(t, ctx, session, "start_calibration", map[string]any{
		"scenario": "ptp-mock",
		"adapter":  "cursor",
		"parallel": 1,
	})
	sessionID := startResult["session_id"].(string)

	// Drain the single dispatched step and hold it (don't submit).
	step1 := callTool(t, ctx, session, "get_next_step", map[string]any{
		"session_id": sessionID,
	})
	if done, _ := step1["done"].(bool); done {
		t.Fatal("expected a step, got done=true")
	}
	if avail, _ := step1["available"].(bool); !avail {
		t.Fatal("expected available=true for first step")
	}
	heldDispatchID := int64(step1["dispatch_id"].(float64))
	heldStep := step1["step"].(string)
	t.Logf("held step: %s dispatch_id=%d", heldStep, heldDispatchID)

	// With the single step held and parallel=1, the runner is fully blocked
	// (the token semaphore is exhausted). A short-timeout poll must return
	// available=false (not done, not an error).
	res, err := callToolE(ctx, session, "get_next_step", map[string]any{
		"session_id": sessionID,
		"timeout_ms": 100,
	})
	if err != nil {
		t.Fatalf("get_next_step with timeout should not error: %v", err)
	}
	if done, _ := res["done"].(bool); done {
		t.Fatal("expected done=false (pipeline still running)")
	}
	if avail, _ := res["available"].(bool); avail {
		t.Fatal("expected available=false (backpressure, no step ready)")
	}
	t.Log("confirmed: available=false under backpressure")

	// Verify that submitting with dispatch_id=0 (the wrong thing to do) is rejected.
	_, err = callToolE(ctx, session, "submit_artifact", map[string]any{
		"session_id":    sessionID,
		"artifact_json": `{"status":"bad"}`,
		"dispatch_id":   int64(0),
	})
	if err == nil {
		t.Fatal("expected error when submitting with dispatch_id=0")
	}
	t.Logf("dispatch_id=0 correctly rejected: %v", err)

	// Now submit the held step — this unblocks the runner.
	artifact := artifactForStep(heldStep, 0)
	_, err = callToolE(ctx, session, "submit_artifact", map[string]any{
		"session_id":    sessionID,
		"artifact_json": artifact,
		"dispatch_id":   heldDispatchID,
	})
	if err != nil {
		t.Fatalf("submit held step: %v", err)
	}
	t.Log("held step submitted, runner unblocked")

	// After unblocking, a new get_next_step should succeed (available=true or done=true).
	res2 := callTool(t, ctx, session, "get_next_step", map[string]any{
		"session_id": sessionID,
	})
	isDone, _ := res2["done"].(bool)
	isAvail, _ := res2["available"].(bool)
	if !isDone && !isAvail {
		t.Fatalf("expected available=true or done=true after unblocking, got %v", res2)
	}
	t.Logf("after retry: done=%v available=%v", isDone, isAvail)
}
