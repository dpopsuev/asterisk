package calibrate

import (
	"sync"
	"testing"
	"time"
)

func TestInMemoryTokenTracker_RecordAndSummary(t *testing.T) {
	tracker := NewTokenTracker()

	records := []TokenRecord{
		{CaseID: "C1", Step: "F0", PromptBytes: 4000, ArtifactBytes: 400, PromptTokens: 1000, ArtifactTokens: 100, WallClockMs: 500},
		{CaseID: "C1", Step: "F1", PromptBytes: 8000, ArtifactBytes: 800, PromptTokens: 2000, ArtifactTokens: 200, WallClockMs: 600},
		{CaseID: "C2", Step: "F0", PromptBytes: 4000, ArtifactBytes: 400, PromptTokens: 1000, ArtifactTokens: 100, WallClockMs: 450},
		{CaseID: "C2", Step: "F1", PromptBytes: 6000, ArtifactBytes: 600, PromptTokens: 1500, ArtifactTokens: 150, WallClockMs: 550},
		{CaseID: "C1", Step: "F2", PromptBytes: 2000, ArtifactBytes: 200, PromptTokens: 500, ArtifactTokens: 50, WallClockMs: 300},
		{CaseID: "C2", Step: "F2", PromptBytes: 2400, ArtifactBytes: 240, PromptTokens: 600, ArtifactTokens: 60, WallClockMs: 320},
		{CaseID: "C1", Step: "F3", PromptBytes: 12000, ArtifactBytes: 1200, PromptTokens: 3000, ArtifactTokens: 300, WallClockMs: 800},
		{CaseID: "C2", Step: "F3", PromptBytes: 10000, ArtifactBytes: 1000, PromptTokens: 2500, ArtifactTokens: 250, WallClockMs: 700},
		{CaseID: "C1", Step: "F5", PromptBytes: 4000, ArtifactBytes: 400, PromptTokens: 1000, ArtifactTokens: 100, WallClockMs: 400},
		{CaseID: "C2", Step: "F5", PromptBytes: 3600, ArtifactBytes: 360, PromptTokens: 900, ArtifactTokens: 90, WallClockMs: 380},
	}

	for _, r := range records {
		tracker.Record(r)
	}

	s := tracker.Summary()

	// Total prompt tokens: 1000+2000+1000+1500+500+600+3000+2500+1000+900 = 14000
	if s.TotalPromptTokens != 14000 {
		t.Errorf("TotalPromptTokens: got %d, want 14000", s.TotalPromptTokens)
	}
	// Total artifact tokens: 100+200+100+150+50+60+300+250+100+90 = 1400
	if s.TotalArtifactTokens != 1400 {
		t.Errorf("TotalArtifactTokens: got %d, want 1400", s.TotalArtifactTokens)
	}
	if s.TotalTokens != 15400 {
		t.Errorf("TotalTokens: got %d, want 15400", s.TotalTokens)
	}
	if s.TotalSteps != 10 {
		t.Errorf("TotalSteps: got %d, want 10", s.TotalSteps)
	}

	// Per-case: C1 has 5 records, C2 has 5 records
	if len(s.PerCase) != 2 {
		t.Fatalf("PerCase count: got %d, want 2", len(s.PerCase))
	}
	c1 := s.PerCase["C1"]
	if c1.Steps != 5 {
		t.Errorf("C1 steps: got %d, want 5", c1.Steps)
	}
	if c1.PromptTokens != 7500 {
		t.Errorf("C1 PromptTokens: got %d, want 7500", c1.PromptTokens)
	}

	c2 := s.PerCase["C2"]
	if c2.Steps != 5 {
		t.Errorf("C2 steps: got %d, want 5", c2.Steps)
	}
	if c2.PromptTokens != 6500 {
		t.Errorf("C2 PromptTokens: got %d, want 6500", c2.PromptTokens)
	}

	// Per-step: F0 has 2 invocations, F1 has 2, etc.
	if len(s.PerStep) != 5 {
		t.Fatalf("PerStep count: got %d, want 5", len(s.PerStep))
	}
	f0 := s.PerStep["F0"]
	if f0.Invocations != 2 {
		t.Errorf("F0 invocations: got %d, want 2", f0.Invocations)
	}
	if f0.PromptTokens != 2000 {
		t.Errorf("F0 PromptTokens: got %d, want 2000", f0.PromptTokens)
	}

	// Cost: 14000 prompt / 1M * $3 = $0.042, 1400 artifact / 1M * $15 = $0.021 → $0.063
	expectedCost := 0.042 + 0.021
	if s.TotalCostUSD < expectedCost-0.001 || s.TotalCostUSD > expectedCost+0.001 {
		t.Errorf("TotalCostUSD: got %.6f, want ~%.6f", s.TotalCostUSD, expectedCost)
	}
}

func TestInMemoryTokenTracker_EmptySummary(t *testing.T) {
	tracker := NewTokenTracker()
	s := tracker.Summary()

	if s.TotalTokens != 0 {
		t.Errorf("empty tracker TotalTokens: got %d, want 0", s.TotalTokens)
	}
	if s.TotalSteps != 0 {
		t.Errorf("empty tracker TotalSteps: got %d, want 0", s.TotalSteps)
	}
	if len(s.PerCase) != 0 {
		t.Errorf("empty tracker PerCase: got %d entries, want 0", len(s.PerCase))
	}
}

func TestInMemoryTokenTracker_ConcurrentAccess(t *testing.T) {
	tracker := NewTokenTracker()
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			tracker.Record(TokenRecord{
				CaseID:         "C1",
				Step:           "F0",
				PromptBytes:    400,
				ArtifactBytes:  40,
				PromptTokens:   100,
				ArtifactTokens: 10,
				Timestamp:      time.Now(),
				WallClockMs:    50,
			})
		}(i)
	}
	wg.Wait()

	s := tracker.Summary()
	if s.TotalSteps != 100 {
		t.Errorf("concurrent TotalSteps: got %d, want 100", s.TotalSteps)
	}
	if s.TotalPromptTokens != 10000 {
		t.Errorf("concurrent TotalPromptTokens: got %d, want 10000", s.TotalPromptTokens)
	}
}

func TestInMemoryTokenTracker_CustomCost(t *testing.T) {
	tracker := NewTokenTrackerWithCost(CostConfig{
		InputPricePerMToken:  10.0,
		OutputPricePerMToken: 30.0,
	})
	tracker.Record(TokenRecord{
		CaseID:         "C1",
		Step:           "F0",
		PromptTokens:   1_000_000,
		ArtifactTokens: 500_000,
	})

	s := tracker.Summary()
	// 1M * $10/M = $10, 0.5M * $30/M = $15 → $25
	if s.TotalCostUSD < 24.99 || s.TotalCostUSD > 25.01 {
		t.Errorf("custom cost: got $%.2f, want $25.00", s.TotalCostUSD)
	}
}

func TestEstimateTokens(t *testing.T) {
	tests := []struct {
		bytes int
		want  int
	}{
		{0, 0},
		{-1, 0},
		{4, 1},
		{100, 25},
		{4000, 1000},
	}
	for _, tt := range tests {
		got := EstimateTokens(tt.bytes)
		if got != tt.want {
			t.Errorf("EstimateTokens(%d): got %d, want %d", tt.bytes, got, tt.want)
		}
	}
}

func TestFormatTokenSummary(t *testing.T) {
	s := TokenSummary{
		TotalPromptTokens:   14000,
		TotalArtifactTokens: 1400,
		TotalTokens:         15400,
		TotalCostUSD:        0.063,
		PerCase:             map[string]CaseTokenSummary{"C1": {}, "C2": {}},
		PerStep:             map[string]StepTokenSummary{"F0": {}, "F1": {}},
		TotalSteps:          10,
		TotalWallClockMs:    272000, // 4m 32s
	}

	out := FormatTokenSummary(s)
	if out == "" {
		t.Error("FormatTokenSummary returned empty string")
	}
	if !strContains(out, "Token & Cost") {
		t.Error("missing header")
	}
	if !strContains(out, "14000") {
		t.Error("missing prompt token count")
	}
	if !strContains(out, "4m 32s") {
		t.Error("missing wall clock")
	}
}

func strContains(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
