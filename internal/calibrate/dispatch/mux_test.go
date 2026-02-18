package dispatch_test

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"testing"
	"time"

	"asterisk/internal/calibrate/dispatch"
)

func TestMux_SingleRoundTrip(t *testing.T) {
	d := dispatch.NewMuxDispatcher(context.Background())
	ctx := context.Background()
	want := []byte(`{"defect_type":"pb001"}`)

	dc := dispatch.DispatchContext{
		CaseID:       "C1",
		Step:         "F0_RECALL",
		PromptPath:   "/tmp/prompt.md",
		ArtifactPath: "/tmp/artifact.json",
	}

	go func() {
		got, err := d.GetNextStep(ctx)
		if err != nil {
			t.Errorf("GetNextStep error: %v", err)
			return
		}
		if got.CaseID != dc.CaseID || got.Step != dc.Step {
			t.Errorf("GetNextStep got case=%s step=%s, want case=%s step=%s",
				got.CaseID, got.Step, dc.CaseID, dc.Step)
		}
		if got.DispatchID == 0 {
			t.Error("expected non-zero DispatchID")
		}
		if err := d.SubmitArtifact(ctx, got.DispatchID, want); err != nil {
			t.Errorf("SubmitArtifact error: %v", err)
		}
	}()

	got, err := d.Dispatch(dc)
	if err != nil {
		t.Fatalf("Dispatch error: %v", err)
	}
	if string(got) != string(want) {
		t.Errorf("Dispatch got %s, want %s", got, want)
	}
}

func TestMux_ConcurrentDispatch_CorrectRouting(t *testing.T) {
	d := dispatch.NewMuxDispatcher(context.Background())
	ctx := context.Background()

	type result struct {
		caseID string
		data   []byte
		err    error
	}

	results := make(chan result, 2)

	for _, cid := range []string{"C1", "C2"} {
		cid := cid
		go func() {
			data, err := d.Dispatch(dispatch.DispatchContext{
				CaseID: cid,
				Step:   "F0_RECALL",
			})
			results <- result{caseID: cid, data: data, err: err}
		}()
	}

	// Collect both steps, then submit in REVERSE order
	time.Sleep(50 * time.Millisecond)

	step1, err := d.GetNextStep(ctx)
	if err != nil {
		t.Fatalf("GetNextStep 1: %v", err)
	}
	step2, err := d.GetNextStep(ctx)
	if err != nil {
		t.Fatalf("GetNextStep 2: %v", err)
	}

	// Submit in reverse: step2 first, then step1
	if err := d.SubmitArtifact(ctx, step2.DispatchID, []byte(fmt.Sprintf(`{"case":"%s"}`, step2.CaseID))); err != nil {
		t.Fatalf("SubmitArtifact step2: %v", err)
	}
	if err := d.SubmitArtifact(ctx, step1.DispatchID, []byte(fmt.Sprintf(`{"case":"%s"}`, step1.CaseID))); err != nil {
		t.Fatalf("SubmitArtifact step1: %v", err)
	}

	for i := 0; i < 2; i++ {
		r := <-results
		if r.err != nil {
			t.Fatalf("Dispatch %s error: %v", r.caseID, r.err)
		}
		expected := fmt.Sprintf(`{"case":"%s"}`, r.caseID)
		if string(r.data) != expected {
			t.Errorf("case %s got %s, want %s — artifact routed to wrong dispatcher", r.caseID, r.data, expected)
		}
	}
}

func TestMux_HighParallelism(t *testing.T) {
	d := dispatch.NewMuxDispatcher(context.Background())
	ctx := context.Background()
	n := 10

	type result struct {
		index int
		data  []byte
		err   error
	}

	results := make(chan result, n)

	for i := 0; i < n; i++ {
		i := i
		go func() {
			data, err := d.Dispatch(dispatch.DispatchContext{
				CaseID: fmt.Sprintf("C%d", i),
				Step:   "F0_RECALL",
			})
			results <- result{index: i, data: data, err: err}
		}()
	}

	time.Sleep(50 * time.Millisecond)

	// Collect all steps
	steps := make([]dispatch.DispatchContext, n)
	for i := 0; i < n; i++ {
		s, err := d.GetNextStep(ctx)
		if err != nil {
			t.Fatalf("GetNextStep %d: %v", i, err)
		}
		steps[i] = s
	}

	// Shuffle and submit in random order
	rand.Shuffle(n, func(i, j int) { steps[i], steps[j] = steps[j], steps[i] })

	for _, s := range steps {
		payload := []byte(fmt.Sprintf(`{"case":"%s"}`, s.CaseID))
		if err := d.SubmitArtifact(ctx, s.DispatchID, payload); err != nil {
			t.Fatalf("SubmitArtifact dispatch_id=%d: %v", s.DispatchID, err)
		}
	}

	for i := 0; i < n; i++ {
		r := <-results
		if r.err != nil {
			t.Fatalf("Dispatch C%d error: %v", r.index, r.err)
		}
		expected := fmt.Sprintf(`{"case":"C%d"}`, r.index)
		if string(r.data) != expected {
			t.Errorf("C%d got %s, want %s — artifact misrouted", r.index, r.data, expected)
		}
	}
}

func TestMux_SubmitUnknownDispatchID(t *testing.T) {
	d := dispatch.NewMuxDispatcher(context.Background())
	err := d.SubmitArtifact(context.Background(), 9999, []byte("{}"))
	if err == nil {
		t.Fatal("expected error for unknown dispatch ID")
	}
	t.Logf("got expected error: %v", err)
}

func TestMux_DoubleSubmitSameID(t *testing.T) {
	d := dispatch.NewMuxDispatcher(context.Background())
	ctx := context.Background()

	go func() {
		d.Dispatch(dispatch.DispatchContext{CaseID: "C1", Step: "F0_RECALL"})
	}()

	time.Sleep(50 * time.Millisecond)
	step, err := d.GetNextStep(ctx)
	if err != nil {
		t.Fatalf("GetNextStep: %v", err)
	}

	if err := d.SubmitArtifact(ctx, step.DispatchID, []byte(`{"first":true}`)); err != nil {
		t.Fatalf("first SubmitArtifact: %v", err)
	}

	// Second submit for the same ID should fail
	err = d.SubmitArtifact(ctx, step.DispatchID, []byte(`{"second":true}`))
	if err == nil {
		t.Fatal("expected error for double submit")
	}
	t.Logf("got expected error: %v", err)
}

func TestMux_ContextCancel_OneOfMany(t *testing.T) {
	dispCtx, dispCancel := context.WithCancel(context.Background())
	defer dispCancel()
	d := dispatch.NewMuxDispatcher(dispCtx)
	ctx := context.Background()

	type result struct {
		caseID string
		data   []byte
		err    error
	}
	results := make(chan result, 3)

	// Start 3 dispatches
	for _, cid := range []string{"C1", "C2", "C3"} {
		cid := cid
		go func() {
			data, err := d.Dispatch(dispatch.DispatchContext{CaseID: cid, Step: "F0_RECALL"})
			results <- result{caseID: cid, data: data, err: err}
		}()
	}

	time.Sleep(50 * time.Millisecond)

	// Collect all 3 steps
	steps := make([]dispatch.DispatchContext, 3)
	for i := 0; i < 3; i++ {
		s, err := d.GetNextStep(ctx)
		if err != nil {
			t.Fatalf("GetNextStep %d: %v", i, err)
		}
		steps[i] = s
	}

	// Submit only 2 of 3 (skip the first one to simulate one dispatch never completing)
	for i := 1; i < 3; i++ {
		payload := []byte(fmt.Sprintf(`{"case":"%s"}`, steps[i].CaseID))
		if err := d.SubmitArtifact(ctx, steps[i].DispatchID, payload); err != nil {
			t.Fatalf("SubmitArtifact %d: %v", i, err)
		}
	}

	// 2 should complete successfully
	var successes int
	timeout := time.After(2 * time.Second)
	for i := 0; i < 2; i++ {
		select {
		case r := <-results:
			if r.err != nil {
				t.Errorf("dispatch %s error: %v", r.caseID, r.err)
			} else {
				successes++
			}
		case <-timeout:
			t.Fatal("timed out waiting for results")
		}
	}

	if successes != 2 {
		t.Errorf("expected 2 successes, got %d", successes)
	}

	// Cancel dispatcher context to unblock the orphaned dispatch
	dispCancel()
	select {
	case r := <-results:
		if r.err == nil {
			t.Error("expected error for unfulfilled dispatch after cancel")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("orphaned dispatch did not unblock after context cancel")
	}
}

func TestMux_DispatcherContextCancel(t *testing.T) {
	dispCtx, dispCancel := context.WithCancel(context.Background())
	d := dispatch.NewMuxDispatcher(dispCtx)

	errCh := make(chan error, 3)
	for i := 0; i < 3; i++ {
		i := i
		go func() {
			_, err := d.Dispatch(dispatch.DispatchContext{
				CaseID: fmt.Sprintf("C%d", i),
				Step:   "F0_RECALL",
			})
			errCh <- err
		}()
	}

	time.Sleep(50 * time.Millisecond)
	dispCancel()

	for i := 0; i < 3; i++ {
		select {
		case err := <-errCh:
			if err == nil {
				t.Error("expected error from cancelled dispatcher context")
			}
		case <-time.After(2 * time.Second):
			t.Fatal("Dispatch did not unblock after dispatcher context cancel")
		}
	}
}

func TestMux_Abort(t *testing.T) {
	d := dispatch.NewMuxDispatcher(context.Background())

	var wg sync.WaitGroup
	errCh := make(chan error, 3)

	for i := 0; i < 3; i++ {
		wg.Add(1)
		i := i
		go func() {
			defer wg.Done()
			_, err := d.Dispatch(dispatch.DispatchContext{
				CaseID: fmt.Sprintf("C%d", i),
				Step:   "F0_RECALL",
			})
			errCh <- err
		}()
	}

	time.Sleep(50 * time.Millisecond)
	d.Abort(fmt.Errorf("test abort"))

	wg.Wait()
	close(errCh)
	for err := range errCh {
		if err == nil {
			t.Error("expected error from Abort")
		}
	}
}

func TestMux_GetNextStep_BlocksUntilDispatch(t *testing.T) {
	d := dispatch.NewMuxDispatcher(context.Background())

	got := make(chan dispatch.DispatchContext, 1)
	go func() {
		dc, err := d.GetNextStep(context.Background())
		if err != nil {
			t.Errorf("GetNextStep error: %v", err)
			return
		}
		got <- dc
	}()

	// Should not have a result yet
	select {
	case <-got:
		t.Fatal("GetNextStep returned before any Dispatch call")
	case <-time.After(100 * time.Millisecond):
	}

	// Now dispatch
	go func() {
		d.Dispatch(dispatch.DispatchContext{CaseID: "C1", Step: "F0_RECALL"})
	}()

	select {
	case dc := <-got:
		if dc.CaseID != "C1" {
			t.Errorf("got case %s, want C1", dc.CaseID)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("GetNextStep did not unblock after Dispatch")
	}
}

func TestMux_GetNextStep_Cancelled(t *testing.T) {
	d := dispatch.NewMuxDispatcher(context.Background())
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := d.GetNextStep(ctx)
	if err == nil {
		t.Fatal("expected error from cancelled context")
	}
}

func TestMux_MultipleSequentialRoundTrips(t *testing.T) {
	d := dispatch.NewMuxDispatcher(context.Background())
	ctx := context.Background()

	go func() {
		for i := 0; i < 3; i++ {
			dc, err := d.GetNextStep(ctx)
			if err != nil {
				t.Errorf("round %d GetNextStep error: %v", i, err)
				return
			}
			artifact := []byte(fmt.Sprintf(`{"round":"%s"}`, dc.CaseID))
			if err := d.SubmitArtifact(ctx, dc.DispatchID, artifact); err != nil {
				t.Errorf("round %d SubmitArtifact error: %v", i, err)
				return
			}
		}
	}()

	for i := 0; i < 3; i++ {
		caseID := string(rune('0' + i))
		got, err := d.Dispatch(dispatch.DispatchContext{CaseID: caseID, Step: "F0_RECALL"})
		if err != nil {
			t.Fatalf("round %d Dispatch error: %v", i, err)
		}
		want := fmt.Sprintf(`{"round":"%s"}`, caseID)
		if string(got) != want {
			t.Errorf("round %d got %s, want %s", i, got, want)
		}
	}
}
