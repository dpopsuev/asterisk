package framework

import (
	"math"
	"testing"
)

func TestDefaultTraits_AllSixElements(t *testing.T) {
	tests := []struct {
		element              Element
		speed                SpeedClass
		maxLoops             int
		convergenceThreshold float64
		shortcutAffinity     float64
		evidenceDepth        int
	}{
		{ElementFire, SpeedFast, 0, 0.50, 0.9, 2},
		{ElementLightning, SpeedFastest, 0, 0.40, 1.0, 1},
		{ElementEarth, SpeedSteady, 1, 0.70, 0.1, 5},
		{ElementDiamond, SpeedPrecise, 0, 0.95, 0.5, 10},
		{ElementWater, SpeedDeep, 3, 0.85, 0.1, 8},
		{ElementAir, SpeedHolistic, 1, 0.60, 0.6, 3},
	}

	for _, tt := range tests {
		t.Run(string(tt.element), func(t *testing.T) {
			traits := DefaultTraits(tt.element)
			if traits.Element != tt.element {
				t.Errorf("Element = %s, want %s", traits.Element, tt.element)
			}
			if traits.Speed != tt.speed {
				t.Errorf("Speed = %s, want %s", traits.Speed, tt.speed)
			}
			if traits.MaxLoops != tt.maxLoops {
				t.Errorf("MaxLoops = %d, want %d", traits.MaxLoops, tt.maxLoops)
			}
			if traits.ConvergenceThreshold != tt.convergenceThreshold {
				t.Errorf("ConvergenceThreshold = %.2f, want %.2f", traits.ConvergenceThreshold, tt.convergenceThreshold)
			}
			if traits.ShortcutAffinity != tt.shortcutAffinity {
				t.Errorf("ShortcutAffinity = %.2f, want %.2f", traits.ShortcutAffinity, tt.shortcutAffinity)
			}
			if traits.EvidenceDepth != tt.evidenceDepth {
				t.Errorf("EvidenceDepth = %d, want %d", traits.EvidenceDepth, tt.evidenceDepth)
			}
			if traits.FailureMode == "" {
				t.Error("FailureMode should not be empty")
			}
		})
	}
}

func TestDefaultTraits_UnknownElement(t *testing.T) {
	traits := DefaultTraits("nonexistent")
	if traits.Element != "" {
		t.Errorf("expected zero-value traits for unknown element, got %+v", traits)
	}
}

func TestAllElements(t *testing.T) {
	elems := AllElements()
	if len(elems) != 6 {
		t.Fatalf("expected 6 core elements, got %d", len(elems))
	}

	seen := make(map[Element]bool)
	for _, e := range elems {
		if seen[e] {
			t.Errorf("duplicate element: %s", e)
		}
		seen[e] = true
	}

	for _, want := range []Element{ElementFire, ElementLightning, ElementEarth, ElementDiamond, ElementWater, ElementAir} {
		if !seen[want] {
			t.Errorf("missing expected element: %s", want)
		}
	}

	if seen[ElementIron] {
		t.Error("Iron should not be in AllElements (it is derived, not core)")
	}
}

func TestAllElements_ReturnsCopy(t *testing.T) {
	a := AllElements()
	b := AllElements()
	a[0] = "mutated"
	if b[0] == "mutated" {
		t.Error("AllElements should return a copy, not a shared slice")
	}
}

func TestIronFromEarth_HighAccuracy(t *testing.T) {
	iron := IronFromEarth(0.90)
	earth := DefaultTraits(ElementEarth)

	if iron.Element != ElementIron {
		t.Errorf("Element = %s, want iron", iron.Element)
	}
	if iron.Speed != earth.Speed {
		t.Errorf("Speed = %s, want %s (same as Earth)", iron.Speed, earth.Speed)
	}

	// MaxLoops = max(0, 1 - floor(0.9*2)) = max(0, 1-1) = 0
	if iron.MaxLoops != 0 {
		t.Errorf("MaxLoops = %d, want 0 (tightened by high accuracy)", iron.MaxLoops)
	}

	// ConvergenceThreshold = 0.70 + (1-0.90)*0.1 = 0.70 + 0.01 = 0.71
	wantThreshold := 0.71
	if math.Abs(iron.ConvergenceThreshold-wantThreshold) > 0.001 {
		t.Errorf("ConvergenceThreshold = %.4f, want %.4f", iron.ConvergenceThreshold, wantThreshold)
	}

	if iron.ShortcutAffinity != earth.ShortcutAffinity {
		t.Errorf("ShortcutAffinity = %.2f, want %.2f (same as Earth)", iron.ShortcutAffinity, earth.ShortcutAffinity)
	}
	if iron.EvidenceDepth != earth.EvidenceDepth {
		t.Errorf("EvidenceDepth = %d, want %d (same as Earth)", iron.EvidenceDepth, earth.EvidenceDepth)
	}
}

func TestIronFromEarth_LowAccuracy(t *testing.T) {
	iron := IronFromEarth(0.0)

	// MaxLoops = max(0, 1 - floor(0.0*2)) = max(0, 1-0) = 1
	if iron.MaxLoops != 1 {
		t.Errorf("MaxLoops = %d, want 1 (no tightening at zero accuracy)", iron.MaxLoops)
	}

	// ConvergenceThreshold = 0.70 + (1-0.0)*0.1 = 0.70 + 0.1 = 0.80
	wantThreshold := 0.80
	if math.Abs(iron.ConvergenceThreshold-wantThreshold) > 0.001 {
		t.Errorf("ConvergenceThreshold = %.4f, want %.4f", iron.ConvergenceThreshold, wantThreshold)
	}
}

func TestIronFromEarth_PerfectAccuracy(t *testing.T) {
	iron := IronFromEarth(1.0)

	// MaxLoops = max(0, 1 - floor(1.0*2)) = max(0, 1-2) = 0
	if iron.MaxLoops != 0 {
		t.Errorf("MaxLoops = %d, want 0", iron.MaxLoops)
	}

	// ConvergenceThreshold = 0.70 + (1-1.0)*0.1 = 0.70
	wantThreshold := 0.70
	if math.Abs(iron.ConvergenceThreshold-wantThreshold) > 0.001 {
		t.Errorf("ConvergenceThreshold = %.4f, want %.4f", iron.ConvergenceThreshold, wantThreshold)
	}
}

func TestIronFromEarth_FailureMode(t *testing.T) {
	iron := IronFromEarth(0.5)
	if iron.FailureMode == "" {
		t.Error("Iron should have a distinct failure mode")
	}
	earth := DefaultTraits(ElementEarth)
	if iron.FailureMode == earth.FailureMode {
		t.Error("Iron failure mode should differ from Earth's")
	}
}
