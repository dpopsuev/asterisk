# Contract -- Agentic Framework III.1: Personae

**Status:** draft
**Goal:** Define the complete agent identity as a composite of four axes (Color, Element, Position, Alignment) and declare 4 Light + 4 Shadow personas for the PoC pipeline. Absorb scope of agent-adapter-overloading.md.
**Serves:** Architecture evolution (Framework identity)

## Contract rules

- Agent identity types are defined in `internal/framework/` alongside ontology and element types.
- The `AgentIdentity` struct replaces the placeholder from I.1-ontology and the `AdapterTraits` from agent-adapter-overloading.md.
- Single-adapter mode (`--adapter basic`) must work unchanged. Identity defaults to a neutral "unaligned" configuration.
- The four axes (Color, Element, Position, Alignment) are independent -- any combination is valid, though the PoC uses a curated set of 8 personas.
- This contract absorbs the scope of agent-adapter-overloading.md Phase 1 (AdapterPool + AffinityScheduler + color traits). Phases 2-4 of that contract remain deferred.
- Inspired by: Jungian archetypes (persona/shadow/self), Inside Out (distinct emotion-agents), Warhammer Primarchs (half loyal/half traitor), Political Compass (multi-axis classification).

## Context

- `contracts/draft/agent-adapter-overloading.md` -- defines AdapterTraits, Position, MetaPhase, color palette, stickiness gradient. All absorbed here.
- `contracts/draft/agentic-framework-I.1-ontology.md` -- defines AgentIdentity placeholder.
- `contracts/draft/agentic-framework-II.1-elements.md` -- defines Element type with behavioral traits.
- `internal/calibrate/adapter.go` -- current ModelAdapter interface + `Identifiable` interface for runtime model probing.
- `internal/framework/identity.go` -- `ModelIdentity` (already implemented). Records foundation LLM name, provider, version, and wrapper. Every persona is powered by a model; `ModelIdentity` is the "ghost" behind the persona "shell".
- `internal/framework/known_models.go` -- `KnownModels` registry, `KnownWrappers` set. Foundation models are registered; wrappers (Cursor, Copilot) are rejected as model names.
- `rules/domain/agent-bus.mdc` -- court positions (PG, SG, PF, C) and zone definitions.
- Plan reference: agentic_framework_contracts_2daf3e14.plan.md -- Tome III: Personae.

## Agent identity axes

```
Axis 1: COLOR (WHO) -- personality, from warm/aggressive to cool/analytical
Axis 2: ELEMENT (HOW) -- behavioral physics, from fast/greedy to deep/adaptive
Axis 3: ALIGNMENT (WHY) -- motivation, from Light (cooperative) to Shadow (adversarial)
Axis 4: POSITION (WHERE) -- court position, from Backcourt (intake) to Frontcourt (investigation) to Paint (close-out)
Axis 5: MODEL (WHAT) -- foundation LLM powering the agent (ModelIdentity, already implemented)
```

Axes 1-4 are persona traits (defined by this contract). Axis 5 is infrastructure (already implemented as `ModelIdentity` in `identity.go`). A persona is a mask; the model is the ghost wearing it. The same persona can be backed by different models, and the same model can wear different personas.

## Light Personas (Cadai)

| Name | Color | Element | Position | Role |
|------|-------|---------|----------|------|
| Herald | Crimson | Fire | PG | Fast intake, optimistic classification |
| Seeker | Cerulean | Water | C | Deep investigator, builds evidence chains |
| Sentinel | Cobalt | Earth | PF | Steady resolver, follows proven paths |
| Weaver | Amber | Air | SG | Holistic closer, synthesizes findings |

## Shadow Personas (Cytharai)

| Name | Color | Element | Position | Role |
|------|-------|---------|----------|------|
| Challenger | Scarlet | Fire | PG | Aggressive skeptic, rejects weak triage |
| Abyss | Sapphire | Water | C | Deep adversary, finds counter-evidence |
| Bulwark | Iron | Diamond | PF | Precision verifier, shatters ambiguity |
| Specter | Obsidian | Lightning | SG | Fastest path to contradiction |

Shadow personas map to Defect Court roles: Challenger = Prosecutor, Abyss = Devil's Advocate, Bulwark = Forensic Expert, Specter = Summary Judgment.

## Go types

```go
package framework

// Color represents an agent's personality on the warm-cool spectrum.
type Color struct {
    Name        string `json:"name"`
    DisplayName string `json:"display_name"`
    Hex         string `json:"hex"`
    Family      string `json:"family"`
}

// Alignment represents an agent's motivational orientation.
type Alignment string

const (
    AlignmentLight  Alignment = "light"
    AlignmentShadow Alignment = "shadow"
)

// Position represents an agent's court position (structural role).
type Position string

const (
    PositionPG Position = "PG"
    PositionSG Position = "SG"
    PositionPF Position = "PF"
    PositionC  Position = "C"
)

// MetaPhase represents a zone in the pipeline graph.
type MetaPhase string

const (
    MetaPhaseBk MetaPhase = "Backcourt"
    MetaPhaseFc MetaPhase = "Frontcourt"
    MetaPhasePt MetaPhase = "Paint"
)

// AgentIdentity is the complete identity of an agent in the Framework.
// Axes 1-4 (persona) are set at configuration time.
// Axis 5 (Model) is discovered at runtime via the Identifiable interface.
type AgentIdentity struct {
    // Persona axes (1-4)
    PersonaName     string    `json:"persona_name"`
    Color           Color     `json:"color"`
    Element         Element   `json:"element"`
    Position        Position  `json:"position"`
    Alignment       Alignment `json:"alignment"`
    HomeZone        MetaPhase `json:"home_zone"`
    StickinessLevel int       `json:"stickiness_level"`

    // Model axis (5) -- which foundation LLM powers this agent.
    // Populated at session start via Identifiable.Identify().
    // Zero value means model is unknown (e.g. stub adapter).
    Model           ModelIdentity `json:"model"`

    StepAffinity    map[string]float64 `json:"step_affinity"`
    PersonalityTags []string           `json:"personality_tags"`
    PromptPreamble  string             `json:"prompt_preamble"`
    CostProfile     CostProfile        `json:"cost_profile"`
}

// CostProfile describes the resource cost of using an agent.
type CostProfile struct {
    TokensPerStep int     `json:"tokens_per_step"`
    LatencyMs     int     `json:"latency_ms"`
    CostPerToken  float64 `json:"cost_per_token"`
}

// Persona is a named, pre-configured agent identity template.
type Persona struct {
    Identity    AgentIdentity
    Description string
}

// LightPersonas returns the 4 Light (Cadai) personas.
func LightPersonas() []Persona

// ShadowPersonas returns the 4 Shadow (Cytharai) personas.
func ShadowPersonas() []Persona

// AllPersonas returns all 8 personas (4 Light + 4 Shadow).
func AllPersonas() []Persona

// PersonaByName looks up a persona by name (case-insensitive).
func PersonaByName(name string) (Persona, bool)

// HomeZoneFor returns the MetaPhase for a given Position.
func HomeZoneFor(p Position) MetaPhase
```

## Color palette

### PoC palette (8 colors: 4 Light + 4 Shadow)

| Color | Hex | Family | Alignment | Personality |
|-------|-----|--------|-----------|-------------|
| Crimson | #DC143C | Reds | Light | Fast, decisive, optimistic |
| Cerulean | #007BA7 | Blues | Light | Analytical, thorough, evidence-first |
| Cobalt | #0047AB | Blues | Light | Methodical, steady, convergence-first |
| Amber | #FFBF00 | Yellows | Light | Balanced, holistic, synthesizing |
| Scarlet | #FF2400 | Reds | Shadow | Aggressive, skeptical, challenging |
| Sapphire | #0F52BA | Blues | Shadow | Deep, adversarial, counter-evidence |
| Obsidian | #3C3C3C | Neutrals | Shadow | Fast, disruptive, contradiction-seeking |
| Iron | #48494B | Neutrals | Shadow | Precise, uncompromising, tempered |

### Extended palette (for growth)

| Family | Colors | Trait cluster |
|--------|--------|---------------|
| Reds (warm, vivid) | Crimson, Cardinal, Scarlet, Vermilion | Fast, aggressive, opinionated |
| Blues (cool, deep) | Cerulean, Cobalt, Azure, Sapphire | Slow, analytical, thorough |
| Yellows (warm, bright) | Amber, Aureolin, Canary, Citrine | Exploratory, creative, lateral |
| Purples (cool, muted) | Amethyst, Byzantine, Eminence, Finn | Judicial, impartial, meta-reasoning |
| Greens (neutral, balanced) | Emerald, Fern, Celadon, Cambridge | Collaborative, synthesizing |
| Neutrals (achromatic) | Obsidian, Iron, Slate, Ash | Adversarial, tempered, disruptive |

## Execution strategy

1. Replace the AgentIdentity placeholder in `internal/framework/identity.go` with the full struct. Preserve the existing `ModelIdentity` type and add a `Model ModelIdentity` field to `AgentIdentity`.
2. Define Color, Alignment, Position, MetaPhase, CostProfile types.
3. Define the 8 personas (4 Light + 4 Shadow) as a curated registry.
4. Implement lookup functions (PersonaByName, HomeZoneFor, LightPersonas, ShadowPersonas).
5. Wire persona identity into log output: `[crimson/herald] F1 triage: product_bug (0.92)`.
6. At session start, if the adapter implements `Identifiable`, call `Identify()` and populate `AgentIdentity.Model` for all personas backed by that adapter.

## Tasks

- [ ] Define Color struct with Name, DisplayName, Hex, Family
- [ ] Define Alignment type and constants (Light, Shadow)
- [ ] Define Position type and constants (PG, SG, PF, C)
- [ ] Define MetaPhase type and constants (Backcourt, Frontcourt, Paint)
- [ ] Define CostProfile struct
- [ ] Define AgentIdentity struct with all four axes + operational fields
- [ ] Define Persona struct with Identity + Description
- [ ] Implement LightPersonas() -- Herald, Seeker, Sentinel, Weaver with full traits
- [ ] Implement ShadowPersonas() -- Challenger, Abyss, Bulwark, Specter with full traits
- [ ] Implement AllPersonas(), PersonaByName(), HomeZoneFor()
- [ ] Write `internal/framework/identity_test.go` -- verify all personas, lookup functions, axis independence
- [ ] Write `internal/framework/color_test.go` -- verify palette completeness, hex values, family groupings
- [ ] Validate (green) -- go build, all tests pass, single-adapter mode unchanged
- [ ] Tune (blue) -- review persona trait values, align with calibration experience
- [ ] Validate (green) -- all tests still pass after tuning

## Acceptance criteria

- **Given** AllPersonas() is called,
- **When** the result is inspected,
- **Then** it contains exactly 8 personas: 4 Light and 4 Shadow.

- **Given** PersonaByName("Herald") is called,
- **When** the result is inspected,
- **Then** it returns a Persona with Color=Crimson, Element=Fire, Position=PG, Alignment=Light.

- **Given** HomeZoneFor(PositionPG) is called,
- **When** the result is inspected,
- **Then** it returns MetaPhaseBk (Backcourt).

- **Given** single-adapter mode with --adapter basic,
- **When** a calibration run executes,
- **Then** behavior is identical to pre-contract (no identity resolution needed).

## Notes

- 2026-02-20 21:30 -- Agent identification diffusion: added Axis 5 (Model) to AgentIdentity. `ModelIdentity`, `KnownModels`, `KnownWrappers`, and `Identifiable` are already implemented. Live probes confirmed `claude-sonnet-4-20250514/Anthropic (via Cursor)`. This contract must preserve the existing `ModelIdentity` type when replacing the `AgentIdentity` placeholder, and add a `Model` field so every persona carries its ghost identity.
- 2026-02-20 -- Contract created. Absorbs agent-adapter-overloading.md Phase 1 scope. The AdapterTraits struct from that contract is replaced by AgentIdentity here, which adds Element and Alignment axes on top of the existing Color and Position axes.
- Shadow personas are not implemented in the pipeline until III.3-shadow is complete. This contract defines their identity; III.3 activates them.
- Depends on I.1-ontology for AgentIdentity placeholder location, II.1-elements for Element type.
