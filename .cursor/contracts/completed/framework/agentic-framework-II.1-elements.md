# Contract — Agentic Framework II.1: Elements

**Status:** complete  
**Goal:** Define the six elements (Fire, Lightning, Earth, Diamond, Water, Air) as Go types with quantified behavioral traits that govern how agents move through a pipeline graph. Add Iron as an evolved form of Earth.  
**Serves:** Architecture evolution (Framework physics)

## Contract rules

- Elements are defined in `internal/framework/` alongside the ontology types.
- Element traits are numeric and deterministic — no randomness, no LLM inference. The scheduler uses them as routing weights.
- The six elements plus Iron form a closed set for the PoC. Extension mechanism exists but is not exercised.
- Elements do not import domain packages. They are pure behavioral descriptors.
- Inspired by Classical Elements, Wuxing (Five Phases), Bionicle elemental tribes, and Astrology's elemental qualities.

## Context

- `contracts/draft/agentic-framework-I.1-ontology.md` — defines `Element` as a placeholder type. This contract fills it in.
- `internal/orchestrate/heuristics.go` — heuristic thresholds (`ConvergenceSufficient`, `MaxInvestigateLoops`) are implicit element traits.
- Plan reference: `agentic_framework_contracts_2daf3e14.plan.md` — Tome II: Elementa.

## Element definitions

| Element | Speed | MaxLoops | ConvergenceThreshold | ShortcutAffinity | EvidenceDepth | Failure Mode |
|---------|-------|----------|---------------------|-----------------|---------------|--------------|
| **Fire** | Fast | 0 | 0.50 | 0.9 | 2 | Burns out (token waste) |
| **Lightning** | Fastest | 0 | 0.40 | 1.0 | 1 | Brittle (wrong path, no recovery) |
| **Earth** | Steady | 1 | 0.70 | 0.1 | 5 | Bloat (too many steps) |
| **Diamond** | Precise | 0 | 0.95 | 0.5 | 10 | Shatters (ambiguity kills it) |
| **Water** | Deep | 3 | 0.85 | 0.1 | 8 | Slow (analysis paralysis) |
| **Air** | Holistic | 1 | 0.60 | 0.6 | 3 | Floaty (vague, no evidence) |

**Iron** is not a 7th element but an evolved form of Earth, tempered by calibration data. Its traits are Earth's traits adjusted by historical accuracy: if calibration shows Earth agents bloat on a step, Iron tightens `MaxLoops` and raises `ConvergenceThreshold`.

## Go types

```go
package framework

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
    ElementIron      Element = "iron" // evolved Earth
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

// DefaultTraits returns the canonical trait set for a given element.
func DefaultTraits(e Element) ElementTraits

// AllElements returns the six core elements (excludes Iron).
func AllElements() []Element

// IronFromEarth derives Iron traits from Earth traits adjusted by
// calibration accuracy data. accuracy is 0.0-1.0 representing
// historical correctness on the relevant step.
func IronFromEarth(accuracy float64) ElementTraits
```

## Execution strategy

1. Replace the `Element` placeholder in `internal/framework/element.go` with full type definitions.
2. Define `ElementTraits` struct and `DefaultTraits` lookup function.
3. Implement `IronFromEarth` — derives adjusted traits from calibration data.
4. Write comprehensive tests: every element has correct traits, Iron derivation works, all elements are enumerable.

## Tasks

- [x] Define `Element` constants: fire, lightning, earth, diamond, water, air, iron
- [x] Define `SpeedClass` type and constants
- [x] Define `ElementTraits` struct with all six numeric fields
- [x] Implement `DefaultTraits(Element) ElementTraits` — lookup table for canonical traits
- [x] Implement `AllElements() []Element` — returns the six core elements
- [x] Implement `IronFromEarth(accuracy float64) ElementTraits` — derive Iron from Earth + calibration data
- [x] Write `internal/framework/element_test.go` — verify all default traits, Iron derivation, boundary conditions
- [x] Validate (green) — `go build ./...`, all tests pass
- [x] Tune (blue) — review trait values against calibration experience
- [x] Validate (green) — all tests still pass after tuning

## Acceptance criteria

- **Given** each of the six core elements,
- **When** `DefaultTraits(element)` is called,
- **Then** the returned traits match the table above exactly.

- **Given** an Earth element with historical accuracy 0.90,
- **When** `IronFromEarth(0.90)` is called,
- **Then** the returned traits have tighter `MaxLoops` and higher `ConvergenceThreshold` than base Earth.

- **Given** `AllElements()` is called,
- **When** the result is inspected,
- **Then** it contains exactly 6 elements (no Iron — Iron is derived, not a core element).

## Notes

- 2026-02-21 15:00 — Contract complete. All element types, traits, and IronFromEarth implemented. 8 element tests passing. Moved to `completed/framework/`.
- 2026-02-20 — Contract created. The trait values are initial calibrations based on Phase 5a observations. Fire/Lightning map to the fast-classification pattern (F0+F1). Water maps to deep investigation (F3). Diamond maps to precise review (F5). Air maps to holistic synthesis (F4+F6). Earth maps to steady resolution (F2).
- Iron's derivation formula: `MaxLoops = max(0, Earth.MaxLoops - floor(accuracy * 2))`, `ConvergenceThreshold = Earth.ConvergenceThreshold + (1 - accuracy) * 0.1`. Subject to tuning.
- Depends on I.1-ontology for the `Element` type definition location.
