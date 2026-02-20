# Contract — Agentic Framework II.2: Cycles

**Status:** complete
**Goal:** Define generative (sheng) and destructive (ke) interaction cycles between elements, mapping them to concrete scheduler routing rules that determine which agent handles a step after a given agent's output.
**Serves:** Architecture evolution (Framework physics)

## Contract rules

- Cycles are defined in `internal/framework/` alongside element types.
- Cycle rules are deterministic: given an element and a cycle type, the next element is always the same.
- Cycles inform the scheduler but do not override explicit affinity rules. They are tiebreakers and preference signals, not hard constraints.
- Inspired by Wuxing's sheng (generative) and ke (destructive) cycles -- elements building on or challenging each other's work.

## Context

- `contracts/draft/agentic-framework-II.1-elements.md` -- defines the six elements and their traits.
- `contracts/draft/agentic-framework-III.1-personae.md` -- defines agent identity with element axis.
- `contracts/draft/agentic-framework-III.3-shadow.md` -- uses the destructive cycle for adversarial pipeline routing.
- `internal/orchestrate/heuristics.go` -- the F0-F6 pipeline already implicitly follows a generative cycle: Fire(classify) -> Earth(stabilize) -> Water(deepen) -> Air(synthesize).
- Plan reference: `agentic_framework_contracts_2daf3e14.plan.md` -- Tome II: Elementa.

## Cycle definitions

### Generative cycle (sheng) -- agents building on each other's work

The generative cycle describes the natural flow of investigation, where each element's output feeds the next:

```
Fire(classify) -> Earth(stabilize) -> Water(deepen) -> Air(synthesize) -> Fire(re-classify)
```

| From | To | Interaction | Pipeline example |
|------|----|-------------|------------------|
| Fire | Earth | Classification provides structure for steady investigation | F1 Triage -> F2 Resolve |
| Earth | Water | Stable repo selection enables deep code investigation | F2 Resolve -> F3 Investigate |
| Water | Air | Deep evidence enables holistic synthesis | F3 Investigate -> F4 Correlate |
| Air | Fire | Synthesis reveals patterns for re-classification | F4 Correlate -> F5 Review (re-triage on reassess) |

Lightning and Diamond participate as accelerators and validators:
- Lightning shortcuts any generative step (skip ahead)
- Diamond validates any generative step (demand higher precision)

### Destructive cycle (ke) -- agents challenging each other's work

The destructive cycle describes adversarial interactions where one element challenges another:

```
Fire(challenge) -> Water(absorb challenge, deepen) -> Earth(solidify revised answer)
Lightning(shortcut) -> Diamond(verify precision) -> Air(broaden scope)
```

| Challenger | Target | Interaction | Court example |
|------------|--------|-------------|---------------|
| Fire | Water | Fire's aggressive challenge forces Water to deepen its evidence | Prosecution challenges investigation |
| Water | Earth | Water's depth destabilizes Earth's stable conclusion | Defense undermines prosecution's case |
| Earth | Fire | Earth's methodical evidence extinguishes Fire's hasty challenge | Steady evidence defeats aggressive challenge |
| Lightning | Diamond | Lightning's speed exposes Diamond's brittleness to ambiguity | Fast contradiction shatters precise conclusion |
| Diamond | Air | Diamond's precision grounds Air's vague synthesis | Forensic detail defeats hand-waving |
| Air | Lightning | Air's breadth covers Lightning's blind spots | Broad scope catches narrow shortcut mistakes |

## Go types

```go
package framework

// CycleType represents the interaction mode between elements.
type CycleType string

const (
    CycleGenerative  CycleType = "generative"  // sheng -- building on each other
    CycleDestructive CycleType = "destructive"  // ke -- challenging each other
)

// CycleRule defines a directed interaction between two elements.
type CycleRule struct {
    Cycle       CycleType `json:"cycle"`
    From        Element   `json:"from"`
    To          Element   `json:"to"`
    Interaction string    `json:"interaction"` // human-readable description
}

// GenerativeCycle returns the generative (sheng) cycle rules.
func GenerativeCycle() []CycleRule

// DestructiveCycle returns the destructive (ke) cycle rules.
func DestructiveCycle() []CycleRule

// NextGenerative returns the element that naturally follows the given
// element in the generative cycle. Returns empty if not in the main cycle.
func NextGenerative(from Element) Element

// Challenges returns the element that the given element challenges
// in the destructive cycle. Returns empty if not in a destructive pair.
func Challenges(from Element) Element

// ChallengedBy returns the element that challenges the given element
// in the destructive cycle.
func ChallengedBy(target Element) Element
```

## Scheduler integration

The cycles inform the `AffinityScheduler` (from III.1-personae) as tiebreakers:

1. **Primary routing**: Step affinity weights (which agent is best for this step).
2. **Tiebreaker**: If two agents have equal affinity, prefer the one whose element follows the previous agent's element in the generative cycle.
3. **Shadow routing**: When the Shadow path activates (III.3), use the destructive cycle to select the challenger: pick the agent whose element challenges the previous Light agent's element.

## Tasks

- [x] Define `CycleType`, `CycleRule` types
- [x] Implement `GenerativeCycle() []CycleRule` -- 4 main + 2 modifier rules
- [x] Implement `DestructiveCycle() []CycleRule` -- 6 destructive pair rules
- [x] Implement `NextGenerative(Element) Element` -- lookup function
- [x] Implement `Challenges(Element) Element` -- lookup function
- [x] Implement `ChallengedBy(Element) Element` -- reverse lookup
- [x] Write `internal/framework/cycle_test.go` -- verify all cycle rules, lookup functions, cycle completeness
- [x] Validate (green) -- `go build ./...`, all tests pass
- [x] Tune (blue) -- verify cycle rules match F0-F6 pipeline flow
- [x] Validate (green) -- all tests still pass after tuning

## Acceptance criteria

- **Given** the generative cycle,
- **When** `NextGenerative(ElementFire)` is called,
- **Then** it returns `ElementEarth`.

- **Given** the destructive cycle,
- **When** `Challenges(ElementFire)` is called,
- **Then** it returns `ElementWater`.

- **Given** two agents with equal step affinity and the previous agent was Fire,
- **When** the scheduler applies the generative tiebreaker,
- **Then** it prefers the Earth-element agent over others.

## Notes

- 2026-02-21 19:00 -- Contract complete. CycleType, CycleRule, GenerativeCycle (6 rules), DestructiveCycle (6 rules), NextGenerative, Challenges, ChallengedBy implemented. 8 tests including cycle completeness and symmetry verification. Moved to `completed/framework/`.
- 2026-02-21 14:30 -- DSL design principles diffusion (P2, P5): cycle rules could be expressed as edge annotations in the pipeline DSL (I.2-characteristica). An edge with `cycle: generative` or `cycle: destructive` would declare which interaction pattern it follows. This is a future extension -- the current contract defines cycles as Go lookup functions; the DSL annotation layer can be added once I.2 is implemented and the annotation mechanism is proven.
- 2026-02-20 -- Contract created. The cycles are not just metaphors -- they encode real routing preferences. The generative cycle maps to the natural F0-F6 flow. The destructive cycle maps to the Defect Court's adversarial pattern.
- Depends on II.1-elements for Element type and traits.
- Used by III.3-shadow for adversarial agent selection.
