# Contract — yaml-dx-cleanup

**Status:** draft  
**Goal:** Every Asterisk YAML file is self-documenting, domain-specific data lives under `domains/<name>/`, zero redundant identity, vocabulary shorthand, schemas collapsed, scenario defaults+overrides, and the 2.3K-line ingest monster is externalized.  
**Serves:** 100% DSL — Zero Go

## Contract rules

- Zero behavioral changes. Every circuit, calibration, and build produces identical output before and after.
- Origami loader changes (vocabulary shorthand, scenario defaults, schema collapse, domain directory resolution) land first. Asterisk YAML migration follows in the same session.
- Each phase leaves the build green.

## Context

Audit of 24 YAML files (5,076 lines) in Asterisk revealed:

- **Domain sprawl:** Asterisk is a generic RCA tool, not a PTP tool. But PTP-specific data (heuristics, scenarios, sources, tuning, datasets) is scattered across the repo root alongside generic RCA config. A new developer adding SRIOV or RAN support would have to interleave their domain data with PTP's. Domain data should live under `domains/<name>/` and be loaded on demand.
- **Naming collisions:** `schema.yaml` (SQLite DDL) vs `schemas/rca/` (LLM output validation). `vocabulary.yaml` has a `heuristics:` section AND `heuristics.yaml` is a separate file.
- **Identity stutter:** 19/24 files say their identity 3x — `kind:` envelope, `metadata.name:`, and a bare `name:` field. The envelope was added by dsl-lexicon but the legacy bare fields were left behind.
- **Vocabulary ceremony:** `M1: {short: M1, long: Defect Type Accuracy}` — the key IS the short code. Triple redundancy. Stage aliases (`F0` AND `F0_RECALL`) are duplicates.
- **Artifact schema sprawl:** 7 files (120 LOC, 42 LOC ceremony) for what could be 1 file with sections.
- **Scenario verbosity:** Each case repeats ~40 lines of ground-truth structure with no defaults or inheritance. Recall-hit cases are trivial but spelled out in full.
- **Inline payloads:** `ptp-real-ingest.yaml` is 2,272 lines — 45% of all YAML — because it inlines full RP JSON responses.
- ~~**`files:` junk drawer**~~ Partially resolved — connectors are now declared. `files:` still holds vocabulary, heuristics, and schema. Kill it.
- ~~**Invisible infrastructure**~~ **Resolved.** Declarative binding (schematics + connectors) landed. The manifest now declares ReportPortal, Knowledge, GitHub, and Docs connectors. `origami fold` generates a unified wired binary (`bin/asterisk`) with all connector wiring from YAML — zero Go in Asterisk.

### What the binding work changed

The declarative binding resolution (schematics/connectors + resolver + codegen) was implemented as a precursor. This obsoletes some Phase 6 tasks and creates new cohesion opportunities:

| Contract task | Status | Notes |
|--------------|--------|-------|
| P6.3 (connectors in manifest) | **done** | `connectors:` with `path:` + component.yaml resolution |
| P6.6 (codegen wires connectors) | **done** | `GenerateWiredBinary` → `bin/asterisk` with all imports + factory calls |
| P6.1 (vocabulary field) | pending | Still in `files:` |
| P6.2 (store section) | pending | Still in `files:` |
| P6.4 (kill files:) | pending | Three entries remain |
| P6.5 (rewrite manifest) | partially done | Schematics + connectors added; `files:` still exists |

**New state of `origami.yaml`:** 83 lines — identity (3), schematics (10), connectors (7), domain_serve assets (63). The asset section is now the dominant cost. P1 (domains) will shrink it by ~20 lines; P6 remainder (kill files:) removes 4 more.

### Cohesion audit

With declarative binding in place, five cohesion improvements emerge:

1. **Unified binary output.** `origami fold` now produces `bin/asterisk` (unified server: domain data + circuit MCP + connectors) instead of `bin/asterisk-domain-serve` (static files only). The justfile still references the old `domain_serve` variable and build comments. The Dockerfile's `container-run` recipe references `asterisk serve` which doesn't exist on the unified binary yet — the binary just calls `http.ListenAndServe` in `main()`. These need alignment.

2. **Manifest sections map to architecture.** The manifest now has clean layers: identity → architecture (schematics/connectors) → data (assets). But `files:` breaks this by mixing infrastructure config (vocabulary, db-schema) with data assets. Killing `files:` completes the layering.

3. **Asset section shrinks after domains.** With P1 (domain directories), `scenarios:`, `sources:`, and `files.heuristics` move to convention-based discovery. The `domain_serve.assets` section drops from 63 lines to ~40 — only generic RCA config remains (circuits, prompts, schemas, scorecards, reports).

4. **Binding validates what fold embeds.** The resolver already loads component.yaml and checks socket satisfaction. Extending fold to validate asset paths (P5.1) now has a stronger foundation — the resolver provides the full dependency graph.

5. **Justfile + container recipes are stale.** The `domain_serve` variable, build comment ("Build domain-serve binary"), and container `serve` command no longer match the unified binary. A quick housekeeping pass aligns them.

Related conversations: [YAML audit](6a3c6eaa-c863-42d3-8b75-2fb408a60299).

### Must vs optional

**Must** — circuit won't run without these:

| File | Role |
|------|------|
| `origami.yaml` | Manifest. `origami fold` entry point. |
| `circuits/*.yaml` | RCA + calibration pipelines. |
| `prompts/*.md` | LLM instructions per step. |
| `db-schema.yaml` | Store schema (cases, RCAs, symptoms). |
| `llm-output-schemas/` | LLM response validation. |
| `scorecards/` | Metric definitions + thresholds. |
| `domains/*/scenarios/` | Ground truth for calibration. |
| `domains/*/sources/` | Repo metadata for resolve/investigate. |

**Optional** — enhances quality or DX:

| File | Degradation without it |
|------|------------------------|
| `vocabulary.yaml` | Raw codes instead of display names. |
| `domains/*/heuristics.yaml` | No stub mode, no pre-filter. LLM-only still works. |
| `reports/*.yaml` | No formatted output. Raw data available. |
| `domains/*/tuning/` | Informational only. Nothing loads at runtime. |

### Domain vs generic split

| Location | Generic RCA | PTP-specific |
|----------|:-----------:|:------------:|
| `circuits/` | x | |
| `prompts/` | x | |
| `llm-output-schemas/` | x | |
| `scorecards/` | x | |
| `reports/` | x | |
| `db-schema.yaml` | x | |
| `vocabulary.yaml` | x | |
| `origami.yaml` | x | |
| `heuristics.yaml` | | x (`name: ptp-heuristics`) |
| `scenarios/` | | x (all 4 are PTP) |
| `sources/` | | x (ptp.yaml, ocp-platform.yaml) |
| `tuning/` | | x |
| `datasets/` | | x |

### Current architecture

```mermaid
flowchart TD
  subgraph root ["Repo root"]
    subgraph manifest ["origami.yaml (83 lines)"]
      m_schematics["schematics: rca, knowledge"]
      m_connectors["connectors: rp, github, docs"]
      m_assets["domain_serve.assets: 63 lines"]
      m_files["files: vocabulary, heuristics, schema"]
    end
    schema["schema.yaml (210L)"]
    schemas["schemas/rca/ (7 files, 120L)"]
    vocab["vocabulary.yaml (87L)"]
    heur["heuristics.yaml (142L)"]
    scen["scenarios/ (4 files, 3,573L)"]
    sources["sources/ (2 files)"]
    rest["circuits/, prompts/, reports/, scorecards/"]
  end

  m_schematics -->|"resolved by fold"| rest
  m_files -.->|"junk drawer"| vocab
  m_files -.->|"junk drawer"| heur
  m_files -.->|"junk drawer"| schema
  schema -.->|"name collision"| schemas
  heur -.->|"PTP-specific at root"| scen
```

### Desired architecture

```mermaid
flowchart TD
  subgraph root ["Repo root (generic RCA only)"]
    subgraph manifest ["origami.yaml (~40 lines)"]
      m_identity["name: asterisk, version: 1.0"]
      m_schematics["schematics: rca, knowledge"]
      m_connectors["connectors: rp, github, docs"]
      m_store["store: { engine: sqlite, schema: db-schema.yaml }"]
      m_vocab["vocabulary: vocabulary.yaml"]
      m_domains["domains: [ocp/ptp]"]
      m_assets["domain_serve.assets: circuits, prompts,\nllm-output-schemas, scorecards, reports"]
    end
    generic["circuits/ prompts/ llm-output-schemas/\nscorecards/ reports/"]
    dbschema["db-schema.yaml"]
    vocab2["vocabulary.yaml (shorthand)"]
  end

  subgraph domains ["domains/"]
    subgraph ocp ["ocp/"]
      subgraph ptp ["ptp/"]
        heur2["heuristics.yaml"]
        scen2["scenarios/ (defaults+overrides)"]
        src2["sources/"]
        ds2["datasets/ (extracted payloads)"]
        tune2["tuning/"]
      end
    end
    future["sriov/  ran/  ..."]
  end

  subgraph enforcement ["Enforcement (Phase 5)"]
    fold_val["fold: validate output_schema paths"]
    b12["lint B12: declared domain missing"]
    b13["lint B13: orphan domain files"]
    b14["lint B14: incomplete domain"]
  end

  manifest -->|"domains: [ocp/ptp]"| ptp
  manifest -->|"schematics: resolved by fold"| generic
  scen2 -->|"local_path:"| ds2
  manifest -.->|"validated by"| enforcement
```

## FSC artifacts

Code only — no FSC artifacts. The changes are syntactic cleanup.

## Execution strategy

Six phases, each independently shippable. Phase 6 is partially complete (binding resolution landed).

**Phase 0 — Housekeeping (Asterisk)**
Quick alignment pass. Update justfile to reference unified binary (`bin/asterisk`), fix build comment and stale `domain_serve` variable. Container recipes reference the unified binary. No Origami changes.

**Phase 1 — Domain directories (Origami + Asterisk)**
The structural move. Origami: fold/manifest accepts `domains:` list, scans each domain directory for scenarios/heuristics/sources/datasets/tuning by convention — no manual path enumeration. Asterisk: move PTP-specific files to `domains/ocp/ptp/`, update `origami.yaml` to declare `domains: [ocp/ptp]`.

**Phase 2 — Naming + ceremony (Asterisk, light Origami)**
Rename `schema.yaml` → `db-schema.yaml`, `schemas/` → `llm-output-schemas/`, kill redundant identity fields, vocabulary shorthand, rename vocabulary `heuristics:` → `decisions:`.

**Phase 3 — Schema collapse + scenario defaults (Origami loader + Asterisk)**
Origami: artifact schema loader accepts single-file multi-stage format. Origami: scenario loader supports `defaults:` block with per-case overrides. Asterisk: collapse 7 schema files to 1, rewrite scenarios with defaults.

**Phase 4 — Payload externalization (Origami loader + Asterisk)**
Origami: scenario loader resolves `local_path:` references for inline payloads. Asterisk: extract `ptp-real-ingest` payloads to `domains/ocp/ptp/datasets/`, reference by path.

**Phase 5 — Enforcement (Origami lint + fold)**
Lint rules and fold validations that make the domain convention enforceable, not just a suggestion. New domain can't ship with broken structure.

**Phase 6 — Manifest clarity (Origami + Asterisk)** — partially complete
Kill the remaining `files:` junk drawer. Vocabulary and store get typed manifest fields. The manifest becomes the single "what does this tool do?" document. Connector binding resolution is already done (schematics/connectors/resolver/codegen).

## Coverage matrix

| Layer | Applies | Rationale |
|-------|---------|-----------|
| **Unit** | yes | Domain directory scanning, vocabulary shorthand parsing, schema collapse loading, scenario defaults merging, payload reference resolution, lint rule validation |
| **Integration** | yes | `origami fold` builds with domain-structured YAML; `origami lint --profile strict` enforces domain structure; asset path validation catches typos |
| **Contract** | yes | Old format still accepted (backward compat) where Origami loaders change |
| **E2E** | yes | `just build`, `just calibrate-stub` produce identical results |
| **Concurrency** | no | No shared state changes |
| **Security** | no | No trust boundaries affected |

## Tasks

### Phase 0 — Housekeeping

- [x] P0.1 — Asterisk justfile: rename `domain_serve` variable to `binary := bin_dir / "asterisk"`. Update build comment to "Build unified binary via origami fold".
- [x] P0.2 — Asterisk justfile: container-run recipe uses `bin/asterisk` as entrypoint (the unified binary listens on a single port now — no separate serve subcommand).
- [x] P0.3 — Validate: `just build` produces `bin/asterisk`.

### Phase 1 — Domain directories

- [x] P1.1 — Origami: fold/manifest accepts `Domains []string` (`yaml:"domains,omitempty"`). For each domain path, fold scans `domains/<path>/` for known subdirs (`scenarios/`, `sources/`, `heuristics.yaml`, `datasets/`, `tuning/`) and auto-merges discovered files into `AssetMap`. No manual path enumeration needed.
- [x] P1.2 — Asterisk: create `domains/ocp/ptp/` and move domain-specific files:
  - `heuristics.yaml` → `domains/ocp/ptp/heuristics.yaml`
  - `scenarios/*.yaml` → `domains/ocp/ptp/scenarios/`
  - `sources/*.yaml` → `domains/ocp/ptp/sources/`
  - `tuning/*.yaml` → `domains/ocp/ptp/tuning/`
- [x] P1.3 — Asterisk: update `origami.yaml` — add `domains: [ocp/ptp]`, remove explicit per-file references for moved files (scenarios, sources, heuristics from assets section).
- [x] P1.4 — Validate: `just build`, `origami lint --profile strict`.

### Phase 2 — Naming + ceremony

- [x] P2.1 — Rename `schema.yaml` → `db-schema.yaml`. Update `origami.yaml` ref.
- [x] P2.2 — Rename `schemas/` → `llm-output-schemas/`. Collapse 7 individual files into `llm-output-schemas/rca.yaml`. Update `origami.yaml` and circuit `output_schema:` paths.
- [ ] P2.3 — Kill bare `name:` and `description:` from all files that have `metadata:` envelope (~19 files). The envelope is the sole identity.
- [ ] P2.4 — Vocabulary shorthand: defect_types `{long:, description:}` map → string shorthand. Stages and metrics already use shorthand.
- [x] P2.5 — Kill stage aliases (`F0_RECALL` etc.) — derive at load time from `F0` + node name.
- [x] P2.6 — Rename vocabulary `heuristics:` section → `decisions:`. Update any Go code that reads this section name.
- [ ] P2.7 — Validate: `just build`, `origami lint --profile strict`.

### Phase 3 — Scenario defaults

- [ ] P3.1 — Origami: artifact schema loader accepts single-file format with sections keyed by stage name.
- [ ] P3.2 — Origami: scenario loader supports `defaults:` block. Case fields inherit from defaults; explicit case fields override.
- [ ] P3.3 — Asterisk: add `defaults:` to all 4 scenario files under `domains/ocp/ptp/scenarios/`. Remove redundant fields from individual cases.
- [ ] P3.4 — Validate: `just build`, `just calibrate-stub`, `go test -race ./...` (Origami).

### Phase 4 — Payload externalization

- [ ] P4.1 — Origami: scenario loader resolves `local_path:` for case payload fields.
- [ ] P4.2 — Asterisk: extract inline RP JSON payloads from `ptp-real-ingest.yaml` to `domains/ocp/ptp/datasets/`. Replace with `local_path:` references.
- [ ] P4.3 — Validate (green): `just build`, `just calibrate-stub`, all metrics identical.

### Phase 5 — Enforcement (Origami lint + fold)

- [ ] P5.1 — Origami fold: validate all `output_schema:` paths in circuit YAML resolve to real files. Fail the build on typos (currently silently embeds nothing).
- [ ] P5.2 — Origami lint B12: domain declared in `domains:` but `domains/<path>/` directory missing or empty → error.
- [ ] P5.3 — Origami lint B13: YAML file exists under `domains/<path>/` but `<path>` not in manifest `domains:` list → warning (orphan detection).
- [ ] P5.4 — Origami lint B14: domain directory missing required subdirs. If `domains/X/scenarios/` exists but `domains/X/sources/` does not → warning ("scenarios reference repos but no source pack found").
- [ ] P5.5 — Origami lint: update B11 `domainDirs` to include `"domains/"` prefix so `kind:` enforcement applies to domain-scoped files.
- [ ] P5.6 — Origami fold: validate manifest `domains:` entries have no duplicate or overlapping paths.
- [ ] P5.7 — Validate (green): all builds, tests, lints pass.

### Phase 6 — Manifest clarity (partially complete)

- [x] P6.3 — ~~Origami: add `Connectors map[string]ConnectorConfig` to manifest.~~ **Done.** Implemented as `Connectors map[string]ConnectorRef` with `path:` pointing to component.yaml. Runtime config (API keys, URLs) stays in connector factories via env vars — cleaner than putting secrets in the manifest.
- [x] P6.6 — ~~Origami: update codegen template to wire connectors from manifest config.~~ **Done.** `GenerateWiredBinary()` resolves bindings, generates Go import + factory calls, and compiles `bin/asterisk` — a unified server binary with domain data + circuit MCP + all connectors wired from YAML.
- [x] P6.1 — Origami: `Vocabulary string` field on `AssetMap`. Vocabulary is a first-class concept under `domain_serve.assets.vocabulary:`.
- [x] P6.2 — Origami: `Store` section on `DomainServeConfig` with `Engine` and `Schema`. Manifest declares `store: {engine: sqlite, schema: db-schema.yaml}`.
- [x] P6.4 — Origami: `files:` section eliminated from manifest parsing. All former entries have typed homes (vocabulary: field, store: section, heuristics: domain convention).
- [x] P6.5 — Asterisk: `origami.yaml` rewritten — clean layering: identity → architecture (schematics/connectors) → domains → infrastructure (store, vocabulary) → assets. No `files:` block.
- [x] P6.7 — Validate (green): `just build` produces `bin/asterisk`, all tests pass.
- [ ] P6.8 — Tune (blue): refactor for quality, no behavior changes.
- [ ] P6.9 — Validate (green): all tests still pass after tuning.

## Acceptance criteria

```gherkin
Given the repo root
When I list top-level YAML files and directories
Then no PTP-specific files exist at root (heuristics, scenarios, sources, tuning)
  And domains/ocp/ptp/ contains all PTP-specific data
  And a new domain can be added by creating domains/<name>/ with no root changes

Given origami.yaml
When I read the domains: list
Then it contains ["ocp/ptp"]
  And fold auto-discovers scenarios, sources, heuristics from domains/ocp/ptp/ by convention
  And no per-file path enumeration is needed for domain files

Given origami.yaml
When I read the schematics: and connectors: sections
Then I see rca + knowledge schematics with explicit bindings
  And I see reportportal, github, docs connectors with component.yaml paths
  And origami fold generates a unified binary with all wiring from YAML

Given schema.yaml renamed to db-schema.yaml
When I read the filename
Then I know it's a database schema without opening the file
  And no file named schema.yaml exists

Given any YAML file with a kind: envelope
When I search for bare name: or description: outside metadata:
Then zero matches (the envelope is the sole identity)

Given vocabulary.yaml
When I look up a metric code
Then the entry is "M1: Defect Type Accuracy" (string shorthand)
  And the loader produces {short: "M1", long: "Defect Type Accuracy"}
  And no F0_RECALL/F1_TRIAGE aliases exist (derived at load time)
  And the heuristics: section is named decisions:

Given llm-output-schemas/rca.yaml (single file)
When I look up F1_TRIAGE fields
Then they are defined under a stages.F1_TRIAGE: key in the single file
  And no llm-output-schemas/rca/ directory exists

Given a scenario with defaults: block
When a case omits expected_triage:
Then the case inherits defaults.expected_triage
  And explicit case fields override defaults

Given domains/ocp/ptp/scenarios/ptp-real-ingest.yaml
When I count lines
Then it is < 500 lines
  And inline RP payloads live in domains/ocp/ptp/datasets/ as separate files
  And calibration produces identical metrics

Given a circuit YAML with output_schema: llm-output-schemas/rca/TYPO.yaml
When origami fold runs
Then the build fails with "output_schema path not found: llm-output-schemas/rca/TYPO.yaml"

Given origami.yaml declares domains: [ocp/sriov]
When domains/ocp/sriov/ does not exist
Then origami lint emits B12 error "domain ocp/sriov declared but directory missing"

Given a YAML file at domains/ocp/ran/heuristics.yaml
When ocp/ran is not in the manifest domains: list
Then origami lint emits B13 warning "orphan domain file: ocp/ran not declared in manifest"

Given domains/ocp/ptp/ has scenarios/ but no sources/
Then origami lint emits B14 warning "domain ocp/ptp has scenarios but no source pack"

Given origami.yaml
When I read the manifest top to bottom
Then I see schematics: with rca and knowledge bindings
  And I see connectors: with reportportal, github, docs
  And I see store: with engine and schema path
  And I see vocabulary: pointing to vocabulary.yaml
  And no files: section exists
  And a new reader knows the full architecture without opening any other file

Given origami.yaml with a files: section
When origami fold runs
Then the build fails with "files: is deprecated — use vocabulary:, store:, or domain convention"

Given just build
When the build completes
Then bin/asterisk exists (unified binary, not bin/asterisk-domain-serve)
  And the binary serves domain data on /domain/ and circuit MCP on /mcp
```

## Security assessment

No trust boundaries affected. All changes are syntactic — no new I/O, credentials, or network calls. Connector runtime config (API keys, URLs) stays in env vars at the factory level, not in the manifest.

## Relationship to other contracts

| Contract | Relationship |
|----------|-------------|
| `dsl-lexicon` (complete) | Lexicon added the envelopes. This contract removes the legacy identity fields that lexicon left behind. |
| `dsl-wiring` (complete) | Wiring added `handler_type`/`handler` on nodes and `output_schema`. The binding resolution (schematics/connectors/resolver/codegen) was implemented as a precursor to this contract's Phase 6. |
| `circuit-dsl-shorthand` (draft) | No overlap. Shorthand is circuit syntax; this is data file cleanup. |

## Notes

2026-03-08 — Binding resolution landed (P6.3 + P6.6 done). Declarative schematics + connectors in origami.yaml → resolver matches sockets to factories via component.yaml → codegen generates unified wired binary. `bin/asterisk` replaces `bin/asterisk-domain-serve`. Added Phase 0 (housekeeping) for justfile/container alignment. Updated context, desired architecture diagram, and acceptance criteria to reflect new state. Cohesion audit identified 5 improvements: unified output, manifest layering, asset shrinkage, validation foundation, stale recipes.

2026-03-08 — Added Phase 6 (manifest clarity). Kill `files:` junk drawer — vocabulary gets its own field, schema goes under `store:` with engine declaration, RP connector declared under `connectors:`. The manifest becomes the single "what are all the moving parts?" document. `docs/ptp/` nuked — derivative of knowledge circuit.

2026-03-08 — Added Phase 5 (enforcement). Fold validates `output_schema:` paths exist at build time. New lint rules: B12 (declared domain missing), B13 (orphan domain files), B14 (incomplete domain structure). B11 updated for `domains/` prefix. Convention without enforcement is just a suggestion.

2026-03-08 — Added domain directories (Phase 1). Asterisk is a generic RCA tool, not PTP-specific. PTP data (heuristics, scenarios, sources, tuning, datasets) belongs under `domains/ocp/ptp/`. New developers extend via `domains/<name>/`. Phases renumbered: domain move → naming → scenario defaults → payload externalization → enforcement.

2026-03-08 — Contract drafted from full YAML audit. 24 files, 5,076 lines. Key insight: 70% of YAML is scenarios, 45% is one file (`ptp-real-ingest`). The ceremony problem is real but the biggest win is scenario defaults + payload externalization.

2026-03-09 — Housekeeping audit. Phases 0, 1, and 6 are fully complete. Phase 2 is partially complete (P2.1, P2.2, P2.5, P2.6 done; P2.3 bare identity and P2.4 defect_type shorthand remain). Phases 3 (scenario defaults), 4 (payload externalization), and 5 (enforcement lint rules) are not started. Remaining: P2.3, P2.4, P2.7, P3, P4, P5, P6.8-P6.9.
