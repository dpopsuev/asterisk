# Contract — rca-pure-dsl

**Status:** active  
**Goal:** The entire `adapters/rca/` directory (40+ files, 6,000+ LOC) is expressed as DSL — zero Go domain logic remains in Asterisk.  
**Serves:** 100% DSL — Zero Go

## Contract rules

- Any Go in `rca/` that cannot become YAML is a missing Origami primitive — file it, implement it, then delete the Go.
- Asterisk and Achilles are sibling playbook repositories on the same framework. Achilles does proactive security RCA; Asterisk does passive CI post-mortem RCA. Marbles extracted here must work for both.
- Scope: `adapters/rca/` and `adapters/rca/adapt/` are the primary decomposition target. Phase 1 also catalogs the 22% gap (ingest, cmd, mcpconfig, demo, dataset) to produce a complete Origami gaps inventory. CLI, store, and vocabulary implementation are handled by other contracts.

## Context

Asterisk's `adapters/rca/` is the largest non-DSL surface in the project. It contains:

- Circuit orchestration (runner, walker, state machine)
- Extractors (LLM prompt → structured output for each circuit step)
- Transformers (context building, prompt filling)
- Hooks (store persistence on step completion)
- Heuristic routing (now DSL via `circuit_rca.yaml` edges — already done)
- Calibration runner (drives multi-case scoring)
- Metrics and scoring (domain-specific metric calculators)
- Report formatting
- Framework adapters (basic, stub, LLM)

The circuit structure (`circuit_rca.yaml`) and routing edges are already DSL. The remaining Go code falls into categories that need decomposition — and the right decomposition target is **marbles**, not just "YAML configs."

### Marble discovery

The 6,000 lines of Go in `rca/` are not random domain code. They encode reusable analysis patterns that repeat across any RCA-style tool. The decomposition should identify **candidate marbles** — composable subgraph building blocks that both Asterisk and Achilles import:

| Pattern | Current Go | Marble candidate | Reuse signal |
|---------|-----------|-----------------|-------------|
| LLM extraction | Extractors: prompt template → structured output | `llm-extract` marble (prompt + schema → parsed result) | Both tools send prompts and parse structured responses |
| Context assembly | Transformers: walker state → prompt variables | `context-builder` marble (state + template → filled prompt) | Both tools build prompts from accumulated evidence |
| Store persistence | Hooks: step completion → save to DB | `persist` marble (artifact → sqlite.exec chain) | Both tools persist results per step |
| Scoring | Metrics: results → quality scores | `score` marble (results + scorecard → metric values) | Both tools evaluate output quality |
| Report generation | Report formatting: analysis → structured output | `report` marble (scored results → formatted report) | Both tools produce human-readable reports |
| Dispatch routing | Framework adapters: model selection + fallback | `dispatch` marble (intent → provider → fallback chain) | Both tools route to LLM providers |

These are Origami's first user-discovered marbles. The decomposition is not "delete Go" — it is "extract marbles from lived experience." The Go code is the prototype; the marbles are the product.

### Sibling validation: Achilles

Achilles (`github.com/dpopsuev/achilles`) is a proactive security vulnerability discovery tool. Its circuit is structurally identical: scan → classify → assess → report. Both tools:

- Walk a graph of analysis nodes
- Use LLM extractors for reasoning
- Score results against ground truth
- Produce structured reports

Achilles is the **litmus test** for every marble extracted from Asterisk. If a marble works for CI post-mortem RCA but not for security probing RCA, the abstraction is wrong — go back and generalize. The goal is not to serve Asterisk; the goal is to serve any analysis tool on Origami.

This makes `rca-pure-dsl` the most strategically important contract in the project: it produces the first marble catalog, validated by two real consumers.

## FSC artifacts

| Artifact | Target | Compartment |
|----------|--------|-------------|
| Marble catalog (first user-discovered marbles) | Origami `docs/` | domain |
| Origami framework gaps inventory | `docs/` | domain |
| RCA decomposition analysis | `docs/` | domain |

## Execution strategy

**Phase 1: Marble discovery** (design, no code changes)

Catalog every Go file in `adapters/rca/`. For each file/function, classify into a marble candidate or framework gap:

| Category | Marble candidate | Origami primitive needed | Achilles reuse? |
|----------|-----------------|------------------------|-----------------|
| Already DSL | — (circuit_rca.yaml) | None | Yes (achilles.yaml) |
| LLM extraction | `llm-extract` | Declarative extractor DSL | Yes (scan, classify, assess) |
| Context assembly | `context-builder` | State-to-prompt transformer | Yes (evidence assembly) |
| Store persistence | `persist` | SQLite adapter (from `adapter-migration`) | Yes (finding storage) |
| Scoring | `score` | `calibrate/` metric definition DSL | Yes (vulnerability scoring) |
| Report generation | `report` | Template/format marble | Yes (security report) |
| Dispatch routing | `dispatch` | Adapter-level YAML config | Yes (model routing) |
| Pure glue | Delete | None | — |

Cross-validate every candidate marble against Achilles's circuit. If a marble is Asterisk-only, the abstraction is wrong.

**Phase 2: Marble implementation** — build the marbles in Origami, validated by both Asterisk and Achilles.

**Phase 3: Migration** — replace `rca/` Go files with marble imports + YAML configuration.

**Phase 4: Validate** — `just calibrate-stub` and `just calibrate-wet` produce identical results.

## Coverage matrix

| Layer | Applies | Rationale |
|-------|---------|-----------|
| **Unit** | yes | Each new Origami primitive gets unit tests |
| **Integration** | yes | `just calibrate-stub` must produce identical results at every step |
| **Contract** | yes | New DSL primitives need schema validation in `origami lint` |
| **E2E** | yes | `just calibrate-stub` + `just calibrate-wet` are the final gates |
| **Concurrency** | yes | Walker parallelism must work with new DSL primitives |
| **Security** | yes | LLM prompt templates are trust boundaries (injection risk) |

## Tasks

### Phase 1: Marble discovery + full Zero Go inventory — COMPLETE

Deliverables produced:
- [x] Marble catalog: `origami/docs/marble-catalog.md` — 6 marbles with interfaces, source files, Achilles cross-validation matrix
- [x] Framework gaps inventory: `origami/docs/framework-gaps.md` — 11 gaps (G1-G11) with resolution path
- [x] RCA decomposition analysis: `asterisk/.cursor/docs/rca-decomposition.md` — per-file classification of 62 files (14,600 LOC) with marble/framework assignments
- [x] Achilles cross-validation: every marble validated against scan→classify→assess→report circuit
- [x] 22% gap areas cataloged: ingest, cmd, mcpconfig, demo, dataset

### Phase 2: Foundation gaps (Origami) — COMPLETE

- [x] G7: Add `Meta map[string]any` to `NodeDef` — smallest gap, unblocks all marbles
- [x] G1: Declarative extractor DSL — built-in `json-schema` extractor type, YAML-configured
- [x] G2: Declarative transformer DSL — `template-params` and `go-template` types, YAML-configured
- [x] G4: Hook persistence DSL — YAML-declared `file_write` and `sqlite_exec` hook actions

### Phase 3: Marble implementation (Origami) — COMPLETE

- [x] G3: YAML-configured provider chains (`dispatch` marble) — `adapt/` package eliminated; StaticDispatcher + `heuristics.yaml`
- [x] G5: Scorer registry + evaluation engine (`score` marble) — **completed via `scorer-pattern` contract**. All 10 batch patterns in Origami `calibrate/batch_scorer.go`. Comparators in `calibrate/comparators.go`. Asterisk scorecard uses framework names. `scorers.go` deleted.
- [x] G6: Report template engine (`report` marble) — data-prep functions in `report_data.go`; 4 YAML templates created; old `report.go`, `rca_report.go`, `transcript.go`, `briefing.go`, `tokimeter.go` deleted. Go renderers are thin wrappers calling `report.Render(def, data)` — accepted residual (domain data marshalling).

### Phase 4: Asterisk migration — PARTIAL

- [x] Replace `adapters/rca/adapt/` (1,278 LOC) with `dispatch` marble YAML config — adapt/ eliminated
- [x] Replace `adapters/rca/metrics.go` + domain logic — scorers registered in YAML via `scorer-pattern` contract. All 21 metrics use framework batch patterns.
- [x] Replace `adapters/rca/` report files — old files deleted, `report_data.go` + YAML templates created; Go rendering wrappers are thin `report.Render()` calls (accepted residual)
- [x] Replace `adapters/rca/` orchestration — RunAnalysis rewired to WalkCase + StoreHooks; bridgeNode/passthroughNode/buildNodeRegistry deleted; RunStep/SaveArtifactAndAdvance isolated in `runner.go` for manual modes (207 LOC); store effects extracted to `store_effects.go` (222 LOC)
- [~] Delete `adapters/rca/` persistence glue — StoreHooks integration preserved; full DSL hook migration pending
- [x] Move `rp_source.go` (93 LOC) to `adapters/rp/`
- [x] Extract `params_types.go` — 169 LOC of type definitions separated from 244 LOC of assembly logic in `params.go`
- [x] Move `tuning.go` QuickWin definitions to `tuning-quickwins.yaml` — loaded via `go:embed`

### Phase 5: Gap area migration — PARTIAL

- [x] G9: Generic transformers — `adapters/ingest/` eliminated, inlined into `cmd/asterisk/ingest.go` (will be eliminated by **origami-fold**)
- [→] G8: `origami fold` — **covered by `origami-fold` contract**
- [~] G11: Calibration-as-circuit — `parallel.go` (753 LOC), `cluster.go` (165 LOC), `briefing.go` (129 LOC) deleted; `cal_runner.go` simplified with errgroup parallelism. Remaining: full calibration-as-circuit migration
- [x] Extract `internal/demo/` — eliminated, inlined into `cmd/asterisk/demo_*.go` (will be eliminated by **origami-fold**)
- [x] Absorb `internal/dataset/` — eliminated, inlined into `cmd/asterisk/dataset.go` (will be eliminated by **origami-fold**)

### Tail

- [ ] Validate (green) — `go build`, `go test`, `just calibrate-stub`, `just test-race`
- [ ] Tune (blue) — review DSL definitions for consistency
- [ ] Validate (green) — all gates still pass

## Acceptance criteria

- **Given** `adapters/rca/`, **when** listing `.go` files after this contract, **then** zero domain logic files remain — only marble imports and YAML configuration.
- **Given** `just calibrate-stub`, **when** run before and after, **then** the report output is identical.
- **Given** the marble catalog produced in Phase 1, **when** applied to Achilles's circuit (scan → classify → assess → report), **then** every marble has a clear Achilles counterpart.
- **Given** a new analysis tool on Origami, **when** defining an RCA-style circuit, **then** it can compose the same marbles without writing Go.

## Security assessment

| OWASP | Finding | Mitigation |
|-------|---------|------------|
| A03: Injection | LLM prompt templates accept circuit variables | Sanitize variable interpolation; validate template inputs against schema |

## Notes

2026-03-01 17:00 — Phase 3 COMPLETE. G5 scorer-pattern verified: all 21 metrics use framework batch patterns, `scorers.go` deleted. G6 report marble complete (YAML templates + thin Go wrappers). Phase 4 advanced: `runner.go` refactored (483→207 LOC, store effects extracted to `store_effects.go` 222 LOC), `params.go` split (types→`params_types.go` 169 LOC, logic 244 LOC), `tuning.go` QuickWin defs moved to `tuning-quickwins.yaml`. `adapters/rca/` prod LOC: 5,610 (down from 6,253). `scorer-pattern` contract moved to completed.
2026-02-28 23:00 — G8 (origami fold) split into dedicated `origami-fold` contract. Properly scoped: 13 tasks, 2 phases, manifest spec + codegen + FQCN resolver + CLI command. Eliminates ~3,400 LOC (cmd/, mcpconfig/, cursor wrapper).
2026-02-28 22:00 — G5 (scorer registry + evaluation engine) split into dedicated `scorer-pattern` contract. Deconstruction identified 10 generic batch patterns covering all 21 metrics. Phase 3 G5 task updated to reference the new contract.
2026-02-28 20:00 — Phase 4[Leftovers]+5 sweep complete (B1-B7). B1: G1 extractor DSL in Origami (resolveExtractor, circuit-level ExtractorDef). B2: G3 dispatch marble — StaticDispatcher, adapt/ package eliminated, heuristics.yaml extracted, consumers updated. B3: G4 sqlite-exec hook in Origami, StoreHooks integration preserved. B4: G6 report marble — report_data.go data-prep functions for YAML templates, runtime template loading. B5: G11 calibration simplification — deleted parallel.go (753 LOC), cluster.go (165 LOC), briefing.go (129 LOC), briefing_test.go, cluster_test.go; replaced with simple errgroup parallelism in cal_runner.go. B6: G9 ingest migration — adapters/ingest/ package eliminated, inlined into cmd/asterisk/ingest.go. B7: internal/demo/ and internal/dataset/ packages eliminated, inlined into cmd/asterisk/. B8 (G8 origami fold) cancelled — requires building code-gen feature in Origami, marked Large effort. Total: ~2,500 LOC deleted across 15+ files. All builds green, all tests green.
2026-02-28 16:30 — Phase 4 partial. Metrics: fully migrated to ScorerRegistry (21 scorers, YAML scorer: declarations, computeMetrics via ScoreCard.ScoreCase). Reports: 4 YAML templates created (calibration, briefing, rca, transcript); Go renderers kept — data-prep deferred. Dispatch: shared buildDispatcher helper extracted; BuildRouter deferred (overkill for current single-mode selection). Orchestration: RunAnalysis rewired to WalkCase with StoreHooks and per-step artifact collection; bridgeNode/passthroughNode/buildNodeRegistry deleted; BuildRunner defaults to real NodeRegistry. RunStep/SaveArtifactAndAdvance kept for manual cmd_cursor/cmd_save mode. Origami gained: ScorerFunc detail strings, ReportDef repeat sections, graph loop auto-increment for expression edges. Remaining for Phase 5: adapt/ replacement (1,278 LOC), rp_source move (blocked by circular dep), full persistence absorption, cal_runner/parallel.go migration.
2026-02-28 03:00 — Phase 1 complete. Marble catalog (6 marbles), framework gaps inventory (11 gaps), and RCA decomposition analysis (62 files, 14,600 LOC classified) produced. Achilles cross-validated. Phase 2-5 tasks now concrete. Migration order: G7 (NodeDef meta) → G1+G2 (extractor+transformer DSL) → G3 (dispatch) → G4 (persist) → G5 (score) → G6 (report) → G9 (generic transformers) → G8 (origami fold) → G11 (calibration-as-circuit).
2026-02-28 01:00 — Expanded Phase 1 to catalog the 22% gap (ingest, cmd, mcpconfig, demo, dataset). Added framework gaps: NodeDef `meta:` field, `origami fold` concept. Phase 1 now produces the complete Origami gaps inventory for achieving zero Go across all 28K LOC.
2026-02-27 22:30 — Reframed as marble discovery. The 6K lines of Go in rca/ are prototypes for Origami's first user-discovered marbles: llm-extract, context-builder, persist, score, report, dispatch. Phase 1 produces the marble catalog, cross-validated against Achilles. This is the most strategically important contract — it defines the reusable building blocks for any analysis tool on Origami.
2026-02-27 22:15 — Contract drafted. Gate-tier: this is the final step to zero-Go Asterisk. Phase 1 is design-only — no code changes. Achilles validates the abstraction.
