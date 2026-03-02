# Contract — design-principle-deterministic

**Status:** draft  
**Goal:** Codify "Deterministic over Stochastic" as a first-class framework design principle with lint enforcement and calibration guidance.  
**Serves:** Cost optimization; reliability; reproducibility; Safety > Speed invariant

## Contract rules

- The rule lives in Origami `rules/universal/` — it applies to any Origami-based tool.
- The lint rule is informational (not blocking). Stochastic nodes are legitimate; the principle is about conscious choice.

## Context

Stochastic (LLM-powered) transformers are expensive (tokens), slow (seconds vs microseconds), non-reproducible, and require wet calibration. Deterministic transformers (match rules, JQ, templates, file I/O) are free, instant, reproducible, and testable via stub calibration.

The RCA circuit demonstrates this: heuristic transformers handle ~70% of cases without touching an LLM. Every node that can be deterministic **should** be.

At 50,000x ROI ($1 AI cost saves $50K labor), token budget is irrelevant for correctness — but unnecessary stochastic processing is still waste. A deterministic node that produces the same answer as a stochastic one is strictly superior on every axis.

Depends on `naming-taxonomy` Phase 3 for the `Deterministic() bool` marker on transformers.

The **D/S boundary** — the edge where processing transitions from deterministic to stochastic — is a critical design point. In electronics, the analog-digital boundary requires special attention (shielding, filtering, impedance matching). In circuits, the D/S boundary is where prompt engineering quality, input structure, and extraction robustness matter most.

## FSC artifacts

| Artifact | Target | Compartment |
|----------|--------|-------------|
| Design principle rule | `rules/universal/deterministic-first.mdc` (Origami) | universal |
| Glossary cross-references | `glossary/glossary.mdc` (Origami) | domain |

## Execution strategy

### Phase 1: Design principle rule
- Create `rules/universal/deterministic-first.mdc` in Origami with the principle, rationale, examples, and electronics analogy.
- Add to rule router in Origami (trigger: "deterministic", "stochastic", "token cost", "D/S boundary").

### Phase 2: Project standards update
- Add principle reference to `project-standards.mdc` in both Origami and Asterisk.
- Reference the calibration ordering: stub validates deterministic; dry/wet validates stochastic.

### Phase 3: Lint integration
- Add `origami lint` rule: when a stochastic transformer is bound to a node, emit informational note.
- Include in `--profile strict` output.
- List stochastic node count and names in lint summary.

### Phase 4: D/S boundary documentation
- Document the "ADC node" concept: the node at the D/S boundary where deterministic data enters stochastic processing.
- Add D/S boundary guidance to the design principle rule.
- Cross-reference with `origami-autodoc` contract for visualization.

## Coverage matrix

| Layer | Applies | Rationale |
|-------|---------|-----------|
| **Unit** | yes | Lint rule detection of stochastic nodes |
| **Integration** | yes | `origami lint --profile strict` on real circuits |
| **Contract** | no | Rule is documentation + lint, not API |
| **E2E** | no | No circuit execution involved |
| **Concurrency** | no | No shared state |
| **Security** | no | No trust boundaries |

## Tasks

- [ ] Phase 1 — Write `rules/universal/deterministic-first.mdc` in Origami
- [ ] Phase 2 — Update `project-standards.mdc` in Origami and Asterisk
- [ ] Phase 3 — Add lint rule for stochastic node flagging (informational)
- [ ] Phase 4 — Document D/S boundary concept and "ADC node" pattern
- [ ] Validate — `origami lint --profile strict` on all Asterisk circuits
- [ ] Validate (green) — all tests pass, acceptance criteria met.
- [ ] Tune (blue) — refactor for quality. No behavior changes.
- [ ] Validate (green) — all tests still pass after tuning.

## Acceptance criteria

- **Given** the rule file `deterministic-first.mdc`, **when** a new developer reads it, **then** the principle, rationale, and D/S boundary concept are clear.
- **Given** `origami lint --profile strict` on `asterisk-rca.yaml` with `adapter=llm`, **when** stochastic nodes exist, **then** they are listed in output with informational severity.
- **Given** project-standards in both repos, **when** searching for "deterministic", **then** the principle is documented with calibration mapping.
- **Given** the glossary, **when** reading the D/S boundary entry, **then** the electronics ADC analogy is explained.

## Security assessment

No trust boundaries affected. Documentation and lint rules only.

## Notes

2026-03-02 00:00 — Contract drafted. Codifies the insight that deterministic transformers are strictly superior to stochastic ones on every axis except reasoning capability. D/S boundary visualization deferred to `origami-autodoc`.
