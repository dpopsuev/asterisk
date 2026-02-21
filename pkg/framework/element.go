// Package framework defines the generic agent pipeline framework.
// It provides interfaces for building directed graphs of processing nodes
// traversed by identity-bearing walkers, with conditional edges governing
// transitions. Domain-specific pipelines (e.g. F0-F6 RCA) are instances
// of this generic structure.
package framework

import "math"

// Element represents a behavioral archetype governing how an agent
// moves through a pipeline graph.
type Element string

const (
	ElementFire      Element = "fire"
	ElementLightning Element = "lightning"
	ElementEarth     Element = "earth"
	ElementDiamond   Element = "diamond"
	ElementWater     Element = "water"
	ElementAir       Element = "air"
	ElementIron      Element = "iron" // evolved Earth, derived via IronFromEarth
)

// SpeedClass describes an element's processing velocity.
type SpeedClass string

const (
	SpeedFastest  SpeedClass = "fastest"
	SpeedFast     SpeedClass = "fast"
	SpeedSteady   SpeedClass = "steady"
	SpeedPrecise  SpeedClass = "precise"
	SpeedDeep     SpeedClass = "deep"
	SpeedHolistic SpeedClass = "holistic"
)

// ElementTraits quantifies an element's behavioral characteristics.
// These traits are used by the scheduler for routing decisions
// and by the graph walker for loop/convergence control.
type ElementTraits struct {
	Element              Element    `json:"element"`
	Speed                SpeedClass `json:"speed"`
	MaxLoops             int        `json:"max_loops"`
	ConvergenceThreshold float64    `json:"convergence_threshold"`
	ShortcutAffinity     float64    `json:"shortcut_affinity"` // 0.0-1.0
	EvidenceDepth        int        `json:"evidence_depth"`
	FailureMode          string     `json:"failure_mode"`
}

var defaultTraits = map[Element]ElementTraits{
	ElementFire: {
		Element: ElementFire, Speed: SpeedFast,
		MaxLoops: 0, ConvergenceThreshold: 0.50,
		ShortcutAffinity: 0.9, EvidenceDepth: 2,
		FailureMode: "burns out (token waste)",
	},
	ElementLightning: {
		Element: ElementLightning, Speed: SpeedFastest,
		MaxLoops: 0, ConvergenceThreshold: 0.40,
		ShortcutAffinity: 1.0, EvidenceDepth: 1,
		FailureMode: "brittle (wrong path, no recovery)",
	},
	ElementEarth: {
		Element: ElementEarth, Speed: SpeedSteady,
		MaxLoops: 1, ConvergenceThreshold: 0.70,
		ShortcutAffinity: 0.1, EvidenceDepth: 5,
		FailureMode: "bloat (too many steps)",
	},
	ElementDiamond: {
		Element: ElementDiamond, Speed: SpeedPrecise,
		MaxLoops: 0, ConvergenceThreshold: 0.95,
		ShortcutAffinity: 0.5, EvidenceDepth: 10,
		FailureMode: "shatters (ambiguity kills it)",
	},
	ElementWater: {
		Element: ElementWater, Speed: SpeedDeep,
		MaxLoops: 3, ConvergenceThreshold: 0.85,
		ShortcutAffinity: 0.1, EvidenceDepth: 8,
		FailureMode: "slow (analysis paralysis)",
	},
	ElementAir: {
		Element: ElementAir, Speed: SpeedHolistic,
		MaxLoops: 1, ConvergenceThreshold: 0.60,
		ShortcutAffinity: 0.6, EvidenceDepth: 3,
		FailureMode: "floaty (vague, no evidence)",
	},
}

// coreElements is the ordered list of the six fundamental elements.
// Iron is excluded because it is derived, not fundamental.
var coreElements = []Element{
	ElementFire, ElementLightning, ElementEarth,
	ElementDiamond, ElementWater, ElementAir,
}

// DefaultTraits returns the canonical trait set for a given element.
// Returns a zero-value ElementTraits if the element is unknown.
func DefaultTraits(e Element) ElementTraits {
	return defaultTraits[e]
}

// AllElements returns the six core elements. Iron is excluded
// because it is derived from Earth via IronFromEarth.
func AllElements() []Element {
	out := make([]Element, len(coreElements))
	copy(out, coreElements)
	return out
}

// IronFromEarth derives Iron traits from Earth traits adjusted by
// historical calibration accuracy. accuracy is 0.0-1.0 representing
// correctness on the relevant step.
//
// Derivation: MaxLoops = max(0, Earth.MaxLoops - floor(accuracy * 2)),
// ConvergenceThreshold = Earth.ConvergenceThreshold + (1 - accuracy) * 0.1.
func IronFromEarth(accuracy float64) ElementTraits {
	earth := defaultTraits[ElementEarth]
	return ElementTraits{
		Element:              ElementIron,
		Speed:                earth.Speed,
		MaxLoops:             max(0, earth.MaxLoops-int(math.Floor(accuracy*2))),
		ConvergenceThreshold: earth.ConvergenceThreshold + (1-accuracy)*0.1,
		ShortcutAffinity:     earth.ShortcutAffinity,
		EvidenceDepth:        earth.EvidenceDepth,
		FailureMode:          "rigid (over-calibrated to past data)",
	}
}
