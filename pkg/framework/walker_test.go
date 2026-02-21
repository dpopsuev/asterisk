package framework

import "testing"

func TestWalkerState_NewWalkerState(t *testing.T) {
	ws := NewWalkerState("test-1")
	if ws.ID != "test-1" {
		t.Errorf("expected ID test-1, got %s", ws.ID)
	}
	if ws.Status != "running" {
		t.Errorf("expected status running, got %s", ws.Status)
	}
	if ws.LoopCounts == nil {
		t.Fatal("expected initialized LoopCounts map")
	}
	if ws.Context == nil {
		t.Fatal("expected initialized Context map")
	}
}

func TestWalkerState_RecordStep(t *testing.T) {
	ws := NewWalkerState("test-2")

	ws.RecordStep("recall", "recall-hit", "H1", "2026-02-20T10:00:00Z")
	ws.RecordStep("review", "review-approve", "H13", "2026-02-20T10:01:00Z")

	if len(ws.History) != 2 {
		t.Fatalf("expected 2 history entries, got %d", len(ws.History))
	}
	if ws.History[0].Node != "recall" {
		t.Errorf("history[0].Node = %s, want recall", ws.History[0].Node)
	}
	if ws.History[0].EdgeID != "H1" {
		t.Errorf("history[0].EdgeID = %s, want H1", ws.History[0].EdgeID)
	}
	if ws.History[1].Node != "review" {
		t.Errorf("history[1].Node = %s, want review", ws.History[1].Node)
	}
	if ws.CurrentNode != "review" {
		t.Errorf("expected CurrentNode review, got %s", ws.CurrentNode)
	}
}

func TestWalkerState_IncrementLoop(t *testing.T) {
	ws := NewWalkerState("test-3")

	c1 := ws.IncrementLoop("H10")
	c2 := ws.IncrementLoop("H10")
	c3 := ws.IncrementLoop("H10")

	if c1 != 1 || c2 != 2 || c3 != 3 {
		t.Errorf("expected counts 1,2,3 got %d,%d,%d", c1, c2, c3)
	}
	if ws.LoopCounts["H10"] != 3 {
		t.Errorf("expected LoopCounts[H10]=3, got %d", ws.LoopCounts["H10"])
	}

	// Different edge, independent counter
	d1 := ws.IncrementLoop("H14")
	if d1 != 1 {
		t.Errorf("expected independent counter to start at 1, got %d", d1)
	}
}

func TestWalkerState_MergeContext(t *testing.T) {
	ws := NewWalkerState("test-4")

	ws.MergeContext(map[string]any{"repo": "/path/to/repo"})
	ws.MergeContext(map[string]any{"branch": "main", "repo": "/updated/path"})

	if ws.Context["repo"] != "/updated/path" {
		t.Errorf("expected repo to be overwritten, got %v", ws.Context["repo"])
	}
	if ws.Context["branch"] != "main" {
		t.Errorf("expected branch=main, got %v", ws.Context["branch"])
	}

	// nil merge is a no-op
	ws.MergeContext(nil)
	if len(ws.Context) != 2 {
		t.Errorf("expected 2 context entries after nil merge, got %d", len(ws.Context))
	}
}
