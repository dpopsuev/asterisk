package mcp

import (
	"testing"
)

func TestSignalBus_Emit(t *testing.T) {
	bus := NewSignalBus()
	if bus.Len() != 0 {
		t.Fatalf("new bus len: got %d, want 0", bus.Len())
	}
	bus.Emit("test", "agent1", "C1", "F0", map[string]string{"k": "v"})
	if bus.Len() != 1 {
		t.Fatalf("after emit len: got %d, want 1", bus.Len())
	}
	sigs := bus.Since(0)
	if len(sigs) != 1 {
		t.Fatalf("Since(0) len: got %d, want 1", len(sigs))
	}
	if sigs[0].Event != "test" || sigs[0].Agent != "agent1" || sigs[0].CaseID != "C1" || sigs[0].Step != "F0" {
		t.Errorf("signal: event=%q agent=%q case_id=%q step=%q", sigs[0].Event, sigs[0].Agent, sigs[0].CaseID, sigs[0].Step)
	}
	if sigs[0].Meta["k"] != "v" {
		t.Errorf("meta: got %v", sigs[0].Meta)
	}
	if sigs[0].Timestamp == "" {
		t.Error("timestamp should be set")
	}
}

func TestSignalBus_Since(t *testing.T) {
	bus := NewSignalBus()
	bus.Emit("a", "", "", "", nil)
	bus.Emit("b", "", "", "", nil)
	bus.Emit("c", "", "", "", nil)
	if bus.Len() != 3 {
		t.Fatalf("len: got %d, want 3", bus.Len())
	}
	s0 := bus.Since(0)
	if len(s0) != 3 {
		t.Fatalf("Since(0): got %d, want 3", len(s0))
	}
	s1 := bus.Since(1)
	if len(s1) != 2 {
		t.Fatalf("Since(1): got %d, want 2", len(s1))
	}
	if s1[0].Event != "b" {
		t.Errorf("Since(1)[0].Event: got %q, want b", s1[0].Event)
	}
	s3 := bus.Since(3)
	if s3 != nil {
		t.Errorf("Since(3): got %v, want nil", s3)
	}
	sNeg := bus.Since(-1)
	if len(sNeg) != 3 {
		t.Errorf("Since(-1) should clamp to 0: got len %d", len(sNeg))
	}
}

func TestSignalBus_Len(t *testing.T) {
	bus := NewSignalBus()
	for i := 0; i < 5; i++ {
		bus.Emit("e", "", "", "", nil)
		if bus.Len() != i+1 {
			t.Errorf("after %d emits: Len()=%d, want %d", i+1, bus.Len(), i+1)
		}
	}
}
