package framework

import "testing"

func TestGenerativeCycle_Length(t *testing.T) {
	rules := GenerativeCycle()
	if len(rules) != 6 {
		t.Errorf("len(GenerativeCycle) = %d, want 6", len(rules))
	}
	for _, r := range rules {
		if r.Cycle != CycleGenerative {
			t.Errorf("rule %s->%s has cycle %q, want %q", r.From, r.To, r.Cycle, CycleGenerative)
		}
		if r.Interaction == "" {
			t.Errorf("rule %s->%s has empty interaction", r.From, r.To)
		}
	}
}

func TestDestructiveCycle_Length(t *testing.T) {
	rules := DestructiveCycle()
	if len(rules) != 6 {
		t.Errorf("len(DestructiveCycle) = %d, want 6", len(rules))
	}
	for _, r := range rules {
		if r.Cycle != CycleDestructive {
			t.Errorf("rule %s->%s has cycle %q, want %q", r.From, r.To, r.Cycle, CycleDestructive)
		}
		if r.Interaction == "" {
			t.Errorf("rule %s->%s has empty interaction", r.From, r.To)
		}
	}
}

func TestNextGenerative_MainCycle(t *testing.T) {
	cases := []struct {
		from Element
		want Element
	}{
		{ElementFire, ElementEarth},
		{ElementEarth, ElementWater},
		{ElementWater, ElementAir},
		{ElementAir, ElementFire},
	}
	for _, tc := range cases {
		got := NextGenerative(tc.from)
		if got != tc.want {
			t.Errorf("NextGenerative(%s) = %s, want %s", tc.from, got, tc.want)
		}
	}
}

func TestNextGenerative_Modifiers(t *testing.T) {
	if got := NextGenerative(ElementLightning); got != "" {
		t.Errorf("NextGenerative(lightning) = %s, want empty", got)
	}
	if got := NextGenerative(ElementDiamond); got != "" {
		t.Errorf("NextGenerative(diamond) = %s, want empty", got)
	}
}

func TestChallenges(t *testing.T) {
	cases := []struct {
		from Element
		want Element
	}{
		{ElementFire, ElementWater},
		{ElementWater, ElementEarth},
		{ElementEarth, ElementFire},
		{ElementLightning, ElementDiamond},
		{ElementDiamond, ElementAir},
		{ElementAir, ElementLightning},
	}
	for _, tc := range cases {
		got := Challenges(tc.from)
		if got != tc.want {
			t.Errorf("Challenges(%s) = %s, want %s", tc.from, got, tc.want)
		}
	}
}

func TestChallengedBy(t *testing.T) {
	cases := []struct {
		target Element
		want   Element
	}{
		{ElementWater, ElementFire},
		{ElementEarth, ElementWater},
		{ElementFire, ElementEarth},
		{ElementDiamond, ElementLightning},
		{ElementAir, ElementDiamond},
		{ElementLightning, ElementAir},
	}
	for _, tc := range cases {
		got := ChallengedBy(tc.target)
		if got != tc.want {
			t.Errorf("ChallengedBy(%s) = %s, want %s", tc.target, got, tc.want)
		}
	}
}

func TestCycleCompleteness_AllElements(t *testing.T) {
	all := AllElements()
	for _, e := range all {
		target := Challenges(e)
		if target == "" {
			t.Errorf("element %s has no destructive target", e)
		}
		source := ChallengedBy(e)
		if source == "" {
			t.Errorf("element %s has no destructive source", e)
		}
	}
}

func TestCycleSymmetry(t *testing.T) {
	all := AllElements()
	for _, e := range all {
		target := Challenges(e)
		reverse := ChallengedBy(target)
		if reverse != e {
			t.Errorf("Challenges(%s) = %s but ChallengedBy(%s) = %s, want %s", e, target, target, reverse, e)
		}
	}
}
