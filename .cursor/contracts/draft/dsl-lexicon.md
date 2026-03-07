# Contract — dsl-lexicon

**Status:** draft  
**Goal:** Every YAML file in `internal/` self-identifies via a standard envelope (`kind`, `version`), uses the same vocabulary for shared concepts, and contains zero redundant definitions.  
**Serves:** 100% DSL — Zero Go

## Contract rules

- Backward compatibility: existing parsers accept both old and new syntax during migration. Old syntax triggers a deprecation lint warning.
- Changes span Origami (parsers) and Asterisk (consumer YAML files). Origami lands first.
- No behavioral changes. All circuits, calibrations, and builds produce identical output before and after.
- Complements `yaml-cohesion` (file organization, manifest wiring) and `circuit-dsl-shorthand` (circuit syntax). No overlap — this contract owns the **shared grammar** across all file kinds.

## Context

Conversation: [DSL lexicon audit](65013565-a183-40d2-ae82-707267f65454) cataloged 10 YAML dialects in `internal/` with zero shared grammar. Each file type invented its own identity field, its own type system, and its own relationship syntax. A parser receiving any of these files blind cannot determine what kind of document it is.

### The 7 sins

| # | Sin | Example |
|---|-----|---------|
| S1 | **No envelope** | `schema.yaml` starts with `version: 1` — could be anything. `heuristics.yaml` has no header at all. |
| S2 | **Three type systems** | Schema: `integer not_null -> jobs`. Artifact schema: `type: string, required: true`. Circuit edge: `when: "output.match == true"`. |
| S3 | **Magic foreign keys** | `-> investigation_suites` requires reading the comment header to understand. |
| S4 | **Self-redundant schemas** | Artifact schemas define each field twice — `fields:` flat map AND `defs:` structured list. |
| S5 | **Disconnected vocabulary** | `pb001` in scenarios and `pb001` in vocabulary.yaml are connected only by human memory. No `$ref`, no `enum:`. |
| S6 | **Misplaced knowledge** | `datasets/docs/ptp/architecture.md` is agent domain knowledge hiding in the test data folder. `roadmap.md` is project planning served as domain data. |
| S7 | **Inconsistent identity** | `circuit: rca`, `scorecard: rca`, `name: ptp-mock` — three different patterns for "what am I?" |

### Current state (10 dialects)

| Kind | Identity | Version? | Self-describing? |
|------|----------|----------|------------------|
| Circuit | `circuit: rca` | no | partly |
| Store schema | `version: 1` | yes | no |
| Scorecard | `scorecard: rca` | `version: 2` | partly |
| Scenario | `name: ptp-mock` | no | no |
| Artifact schema | `name: F1_TRIAGE` | no | no |
| Report template | `name: rca-report` | no | no |
| Heuristics | *(none)* | no | no |
| Vocabulary | *(none)* | no | no |
| Source pack | `name: ptp` | no | no |
| Quick wins | *(none)* | no | no |

### Desired state (1 grammar)

Every YAML file opens with a standard envelope:

```yaml
kind: store-schema        # self-identifying type discriminator
version: v1               # schema version for this kind
metadata:
  name: asterisk           # human-readable name
  description: "..."       # one-liner

# ... kind-specific content below
```

Benefits:
- A parser can route by `kind:` without knowing the file path.
- `version:` enables schema evolution without breaking changes.
- `metadata.name` + `metadata.description` replace the inconsistent identity fields.
- Lint rules can validate structure per `kind`.
- `origami lint` gains file-type validation: "expected `kind: circuit`, got `kind: scorecard`."

## FSC artifacts

| Artifact | Target | Compartment |
|----------|--------|-------------|
| DSL lexicon reference | `rules/domain/dsl-lexicon.mdc` (Asterisk) | domain |
| Schema DSL syntax update | `notes/schema-dsl-migration.md` (Origami) | domain |

## Execution strategy

Three phases. Each leaves the build green and is independently shippable.

**Phase 1 — Standard envelope + schema FK (Origami parser + Asterisk YAML)**
Add envelope parsing to all Origami YAML loaders (circuit, schema, scorecard). Add `references:` as a column modifier alongside `->` (backward-compat). Migrate Asterisk's `schema.yaml` to explicit `references:`. Add envelope to all 24 YAML files.

**Phase 2 — Redundancy elimination + vocabulary formalization (Asterisk)**
Collapse artifact schema `fields:` + `defs:` into a single `fields:` definition. Add `enum:` declarations to vocabulary.yaml. Scenario files reference vocabulary enums for defect types.

**Phase 3 — Misplaced content + lint rules (Asterisk + Origami)**
Move `datasets/docs/` to `internal/knowledge/`. Delete or relocate `roadmap.md`. Add `origami lint` rule: every YAML under `internal/` must have a `kind:` field.

## Coverage matrix

| Layer | Applies | Rationale |
|-------|---------|-----------|
| **Unit** | yes | Envelope parsing, `references:` modifier, artifact schema single-form parsing |
| **Integration** | yes | `origami fold` builds with envelope'd YAML; `origami lint` validates `kind:` |
| **Contract** | yes | Old YAML without envelope still parses (backward compat); deprecation warning |
| **E2E** | yes | `just build` produces working binary; `just calibrate-stub` passes |
| **Concurrency** | no | No shared state changes |
| **Security** | no | No trust boundary changes |

## Tasks

### Phase 1 — Standard envelope + schema FK

- [ ] P1.1 — Origami: Add `references:` as alternative column modifier to `->` in `connectors/sqlite/schema.go`. Both accepted; `->` emits deprecation lint warning.
- [ ] P1.2 — Origami: Define `Envelope` struct (`Kind`, `Version`, `Metadata`) parsed from YAML top-level fields. Non-breaking: missing `kind:` is accepted (backward compat).
- [ ] P1.3 — Origami: Circuit loader (`dsl.go`) recognizes envelope. `kind: circuit` maps to existing `CircuitDef` parsing.
- [ ] P1.4 — Origami: Schema loader (`schema.go`) recognizes envelope. `kind: store-schema` maps to existing `SchemaFile` parsing.
- [ ] P1.5 — Origami: Add `origami lint` rule: `kind:` field recommended on all YAML files.
- [ ] P1.6 — Asterisk: Add `kind:` + `version:` + `metadata:` envelope to all 24 YAML files in `internal/`.
- [ ] P1.7 — Asterisk: Replace `->` with `references:` in `schema.yaml` (all 10 FK columns + 1 join table).
- [ ] P1.8 — Validate: `just build`, `go test -race ./...` (Origami), `origami lint`.

### Phase 2 — Redundancy elimination + vocabulary formalization

- [ ] P2.1 — Asterisk: Collapse artifact schemas — remove `defs:` section, keep only `fields:` with `type` and `required` inline.
- [ ] P2.2 — Origami: Update artifact schema loader to accept single-form `fields:` (no `defs:`). Backward compat: `defs:` still accepted.
- [ ] P2.3 — Asterisk: Add `enum:` declarations to vocabulary.yaml for defect types, stages, severity levels.
- [ ] P2.4 — Asterisk: Scenario YAML references vocabulary enums (document convention; runtime validation deferred).
- [ ] P2.5 — Validate: `just build`, `just calibrate-stub`.

### Phase 3 — Misplaced content + lint enforcement

- [ ] P3.1 — Asterisk: Move `datasets/docs/ptp/architecture.md` → `internal/knowledge/ptp/architecture.md`.
- [ ] P3.2 — Asterisk: Delete or move `internal/roadmap.md` to `.cursor/docs/` (project planning, not domain data).
- [ ] P3.3 — Origami: `origami lint` rule — warn when YAML file under known domain paths lacks `kind:`.
- [ ] P3.4 — Validate (green) — all builds, tests, and lints pass.
- [ ] P3.5 — Tune (blue) — refactor for quality. No behavior changes.
- [ ] P3.6 — Validate (green) — all tests still pass after tuning.

## Acceptance criteria

```gherkin
Given any YAML file under internal/
When I read the first 3 lines
Then I can identify its type from the `kind:` field
  And I can identify its schema version from the `version:` field

Given schema.yaml
When I read a column definition
Then foreign keys use `references: table_name` (not `->`)
  And `->` triggers a deprecation lint warning

Given an artifact schema file (e.g. F1_TRIAGE.yaml)
When I read the field definitions
Then each field is defined exactly once (no `fields:` + `defs:` duplication)

Given vocabulary.yaml
When I look up a defect type code (e.g. pb001)
Then it is declared in an `enum:` block with its human-readable label
  And scenario files reference the same codes

Given `origami lint --profile strict` run on internal/
When any YAML file lacks a `kind:` field
Then the linter emits a warning
```

## Security assessment

No trust boundaries affected. All changes are syntactic; no new I/O, no credentials, no network calls.

## Relationship to other contracts

| Contract | Relationship |
|----------|-------------|
| `yaml-cohesion` | Complementary. Cohesion owns file organization (root cleanup, manifest wiring, 18 gaps). Lexicon owns shared grammar (envelope, FK syntax, vocabulary). |
| `circuit-dsl-shorthand` | Complementary. Shorthand owns circuit-specific syntax (compact edges, topology inference). Lexicon owns the envelope that wraps circuit files. |

## Notes

2026-03-05 — Contract drafted from DSL lexicon audit. 10 dialects with zero shared grammar identified. 7 sins cataloged. Three-phase approach: envelope + FK syntax → redundancy + vocabulary → misplaced content + lint. Origami changes are backward-compatible (old syntax accepted with deprecation warning).
