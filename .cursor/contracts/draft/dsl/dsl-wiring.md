# Contract — dsl-wiring

**Status:** draft  
**Goal:** Circuit nodes declare their own prompt and schema, bindings are namespace-scoped, fold reads component.yaml, and naming inconsistencies (family/transformer, circuit names, terminal nodes) are resolved.  
**Serves:** 100% DSL — Zero Go

## Contract rules

- Changes span both Origami (framework `fold`, `dsl.go`) and Asterisk/Achilles (consumer YAML files).
- Origami changes land first; consumers update in the same session.
- Every gap resolved must have a test proving the old path is gone.
- Depends on Origami `manifest-as-map` for manifest schema (`assets:` section). This contract does NOT own the manifest-as-map concern (G3/G6/G7 absorbed).

## Context

Originally `yaml-cohesion` — 18 gaps identified in the YAML structure. After analysis:
- Phase 1 (root cleanup) **complete** — root is clean, files under `internal/`.
- G3 (disconnected circuits), G6 (redundant embed), G7 (no single source of truth) **absorbed** by Origami `manifest-as-map`.
- G17 (connector naming), G18 (hardcoded app name) **done**.
- P2.6 (TemplatePathForStep) **done** during `prompt-first-class-consumer`.

**Remaining scope** (11 open gaps): DSL wiring — how fold resolves components, how bindings work, how nodes declare their prompt/schema, and naming conventions.

### Current architecture

```mermaid
flowchart LR
  manifest["origami.yaml"] -->|"flat bindings"| fold["fold"]
  fold -->|"hardcoded socketOptionMap"| binary["binary"]
  circuit["circuit YAML"] -.->|"family: on node"| fold
  circuit -.->|"no prompt: field"| fold
  circuit -.->|"imports: per file"| fold
  component["component.yaml"] -.->|"ignored"| fold
```

### Desired architecture

```mermaid
flowchart LR
  manifest["origami.yaml"] -->|"rca.source: reportportal"| fold["fold"]
  fold -->|"reads component.yaml"| component["component.yaml"]
  circuit["circuit YAML"] -->|"prompt: + output_schema: on node"| fold
  circuit -->|"scorecard: on calibration"| fold
  manifest -->|"imports: (sole owner)"| fold
```

## FSC artifacts

| Artifact | Target | Compartment |
|----------|--------|-------------|
| DSL wiring reference | `docs/dsl-wiring.md` | domain |

## Execution strategy

Two phases (Phase 1 was root cleanup, now complete). Each leaves the build green.

**Phase 1 — Framework DSL enhancements (Origami)**
Fold reads component.yaml. Namespaced bindings. `prompt:` and `output_schema:` on NodeDef. `scorecard:` on calibration circuit. Single field for family/transformer. Entrypoint owns all imports. Remove ghost fields.

**Phase 2 — Consumer migration + validation (Asterisk + Achilles)**
Update circuit YAMLs with prompt:/output_schema: on nodes, scorecard: on calibration, namespaced bindings, fix naming. Validate and tune.

## Coverage matrix

| Layer | Applies | Rationale |
|-------|---------|-----------|
| **Unit** | yes | Binding resolution, component.yaml loading, prompt path derivation, NodeDef field parsing |
| **Integration** | yes | `origami fold` with namespaced bindings; `origami lint` validates new fields |
| **Contract** | yes | Old binding syntax triggers deprecation warning (backward compat during migration) |
| **E2E** | yes | `just build` produces working binary; `just calibrate-stub` passes |
| **Concurrency** | no | No shared state changes |
| **Security** | no | No trust boundary changes |

## Tasks

### Phase 1: Framework DSL enhancements (Origami)

- [ ] W1 — Fold reads `component.yaml` for socket declarations and factory names; delete `socketOptionMap` and `lookupFactory` (G2)
- [ ] W2 — Namespaced bindings: `rca.source` instead of `source`; fold strips prefix and matches to import namespace (G1)
- [ ] W3 — `imports:` includes connectors (`origami.connectors.reportportal`, `origami.connectors.sqlite`); bindings use short namespace names (G8)
- [ ] W4 — Add `prompt:` and `output_schema:` fields to `NodeDef`; fold/DSL loader populates transformer context from them (G5)
- [ ] W5 — Add `scorecard:` field to circuit YAML; calibration runner reads it from circuit config (G4)
- [ ] W6 — Resolve `family:` vs `transformer:` — single field; component.yaml maps names to transformers (G10). **Note:** circuit-dsl-shorthand depends on this decision for implicit-family logic.
- [ ] W7 — Remove `imports:` from individual circuit files; entrypoint owns all imports (G12)
- [ ] W8 — Remove unused `CLI`, `Serve`, `Demo` fields from manifest struct (G15)
- [ ] W9 — Validate: `go test -race ./...`, `go build ./...`

### Phase 2: Consumer migration + validation (Asterisk + Achilles)

- [ ] W10 — Fix circuit name redundancy: `asterisk-rca` → `rca` (G11)
- [ ] W11 — Consistent terminal node name: pick one convention (G13)
- [ ] W12 — Fix `component: asterisk-rca` → `component: origami-rca` in schematic component.yaml (G16)
- [ ] W13 — Update Asterisk circuit YAMLs: add `prompt:`, `output_schema:`, `scorecard:`, namespaced bindings
- [ ] W14 — Update Achilles manifest to match new shape
- [ ] W15 — Validate (green) — `just build`, `go test ./...`, `origami lint --profile strict` all pass
- [ ] W16 — Tune (blue) — refactor for quality, no behavior changes
- [ ] W17 — Validate (green) — all tests still pass after tuning

## Acceptance criteria

```gherkin
Given a circuit YAML with nodes
When a node has a stochastic transformer
Then the node definition includes prompt: and output_schema: fields
  And no hardcoded Go mapping exists for prompt paths

Given the calibration circuit YAML
When it declares scorecard: scorecards/rca.yaml
Then the calibration runner loads the scorecard from that path
  And no convention-based path guessing exists

Given origami.yaml with bindings
When I write rca.source: reportportal
Then fold resolves "rca" to the import namespace, "source" to the socket,
  and "reportportal" to the connector's component.yaml satisfies entry
  And socketOptionMap and lookupFactory do not exist

Given two schematics imported
When both declare a "source" socket
Then bindings rca.source and vulnscan.source resolve independently
  And no collision occurs

Given a NodeDef with family: "rca-triage"
When the circuit is parsed
Then the family (or its resolved name after G10) maps to the correct transformer
  And there is exactly one field for this concept, not two
```

## Security assessment

No trust boundaries affected.

## Gap reference (remaining)

| # | Area | Current | Desired | Task |
|---|------|---------|---------|------|
| G1 | Binding scope | Flat `map[string]string`, no namespace | `rca.source` prefix | W2 |
| G2 | component.yaml ignored | Hardcoded `socketOptionMap` | Fold reads component.yaml | W1 |
| G4 | Orphaned scorecard | Convention-based loading | `scorecard:` on circuit | W5 |
| G5 | Prompt-node cohesion | Separate manifest + hardcoded Go | `prompt:` on node | W4 |
| G8 | Hidden imports | Connectors resolved implicitly | `imports:` complete | W3 |
| G9 | Duplicate circuit defs | Schematic + consumer both define | One authoritative | W10 |
| G10 | family: vs transformer: | Two fields | Single field | W6 |
| G11 | Circuit name redundancy | `asterisk-rca` | Bare `rca` | W10 |
| G12 | imports: in circuit files | Each re-declares | Entrypoint owns | W7 |
| G13 | Inconsistent terminal node | `DONE` vs `_done` | One convention | W11 |
| G14 | before: hooks split | Split across consumer + schematic | One place | W13 |
| G15 | Ghost fields | CLI/Serve/Demo unused | Remove | W8 |
| G16 | Schematic component name | `asterisk-rca` | `origami-rca` | W12 |

**Absorbed by other contracts:** G3 → manifest-as-map, G6 → manifest-as-map, G7 → manifest-as-map, G17 → done, G18 → done, P2.6 → prompt-first-class-consumer.

## Relationship to other contracts

| Contract | Relationship |
|----------|-------------|
| `manifest-as-map` (Origami) | Prerequisite for manifest schema. dsl-wiring assumes `assets:` section exists. G3/G6/G7 absorbed. |
| `dsl-lexicon` (Asterisk) | Complementary. Lexicon owns file self-identity (`kind:` envelope). Wiring owns how fold resolves components and bindings. |
| `circuit-dsl-shorthand` (Asterisk) | Downstream. Shorthand depends on W6 (family/transformer resolution) for implicit-family logic. |

## Notes

2026-03-07 — Refocused from `yaml-cohesion` to `dsl-wiring`. Dropped "one entrypoint" goal (absorbed by Origami `manifest-as-map`). Remaining scope: 13 gaps about DSL wiring (bindings, node fields, naming). Tasks renumbered W1-W17. Phase 1 (root cleanup) removed (complete). Cross-repo nature preserved.

2026-03-07 — Housekeeping: Phase 1 marked complete, P2.4/P2.6/P2.8 absorbed.

2026-03-04 00:30 — Original contract created as `yaml-cohesion` from conversation analysis. 18 gaps identified.
