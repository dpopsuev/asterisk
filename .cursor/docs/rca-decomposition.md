# RCA Decomposition Analysis

Per-file classification of `adapters/rca/` (62 files, ~14,600 LOC) mapping every production Go file to its marble or framework destination. Test files migrate with their production counterparts.

## Production files — `adapters/rca/`

| File | LOC | Category | Destination | Marble/Gap |
|------|-----|----------|-------------|------------|
| `adapter.go` | 66 | Framework adapters | Delete — wiring absorbed by YAML `adapter.yaml` + `origami fold` | G8 |
| `analysis.go` | 318 | Orchestration | Delete — replaced by circuit walk with marble-configured nodes | G7, G11 |
| `artifact.go` | 122 | Persistence | `persist` marble — file I/O becomes YAML-declared hook actions | G4 |
| `briefing.go` | 129 | Report/Output | `report` marble — briefing becomes a report template | G6 |
| `cal_runner.go` | 684 | Calibration | Framework `calibrate/` — calibration runner becomes meta-circuit | G11 |
| `cal_types.go` | 236 | Types | Framework types — `Scenario`, `GroundTruthCase`, `CaseResult` move to `calibrate/` | — |
| `catalog_convert.go` | 57 | Transformers | `context-builder` marble — workspace-to-catalog conversion | G2 |
| `cluster.go` | 165 | Domain logic | `score` marble — clustering for M5 serial killer detection scorer | G5 |
| `evidence_gap.go` | 61 | Domain logic | `score` marble — gap classification becomes a built-in scorer | G5 |
| `extractor.go` | 40 | LLM extraction | `llm-extract` marble — generic JSON extractor becomes built-in | G1 |
| `framework_adapters.go` | 136 | Framework adapters | Partially delete — `StepToNodeName`/`NodeNameToStep` become framework internals, `EvaluateGraphEdge` stays in framework | G7 |
| `hooks.go` | 32 | Persistence hooks | `persist` marble — 5 hooks become YAML-declared actions | G4 |
| `metrics.go` | 646 | Domain logic | `score` marble — 21 scorers registered in scorer registry | G5 |
| `model_adapter.go` | 37 | Types | `dispatch` marble — `ModelAdapter` interface absorbed by provider chain | G3 |
| `nodes.go` | 129 | Orchestration | Delete — passthrough nodes absorbed by marble-configured graph walk | G7 |
| `parallel.go` | 752 | Orchestration | Framework `calibrate/` — parallel fan-out becomes framework feature | G11 |
| `params.go` | 512 | Transformers | `context-builder` marble — `BuildParams` becomes YAML-configured source chain | G2 |
| `circuit_def.go` | 108 | Circuit DSL | Keep (thin) — loads `circuit_rca.yaml`, already DSL | — |
| `report.go` | 126 | Report/Output | `report` marble — calibration report becomes a template | G6 |
| `rca_report.go` | 257 | Report/Output | `report` marble — RCA report becomes a template | G6 |
| `rp_source.go` | 92 | Domain logic | RP adapter — `ResolveRPCases` moves to `adapters/rp/` | — |
| `runner.go` | 482 | Orchestration | Delete — procedural step runner absorbed by framework graph walk + marbles | G1, G2, G7 |
| `state.go` | 67 | Orchestration | Framework — `CaseState` absorbed by `framework.WalkerState` | — |
| `template.go` | 77 | Transformers | `context-builder` marble — `FillTemplate` becomes built-in transformer | G2 |
| `tokimeter.go` | 48 | Report/Output | `report` marble — cost bill becomes a report section | G6 |
| `transcript.go` | 266 | Report/Output | `report` marble — transcript becomes a report template | G6 |
| `transformers.go` | 96 | Transformers | `context-builder` marble — `ContextBuilder`/`PromptFiller` become YAML-configured | G2 |
| `tuning.go` | 184 | Calibration | Framework `calibrate/` — tuning becomes declarative quick-win config | G11 |
| `types.go` | 189 | Types | Partially framework — `CircuitStep` enum → framework, domain types → YAML schema | — |
| `vocab.go` | 29 | Report/Output | Delete — vocabulary already in `vocabulary.yaml` | — |
| `walk.go` | 77 | Orchestration | Keep (thin) — `WalkCase` becomes a thin YAML circuit invoker | — |
| `walker.go` | 118 | Orchestration | Delete — `calibrationWalker` absorbed by framework walker + marble hooks | G4, G7 |

**Subtotals:**

| Destination | Files | LOC | % of production |
|-------------|-------|-----|-----------------|
| `llm-extract` marble | 1 | 40 | 0.5% |
| `context-builder` marble | 4 | 742 | 9.8% |
| `persist` marble | 2 | 154 | 2.0% |
| `score` marble | 3 | 872 | 11.5% |
| `report` marble | 5 | 826 | 10.9% |
| `dispatch` marble (via adapt/) | 1 | 37 | 0.5% |
| Framework (`calibrate/`, types, walker) | 7 | 2,298 | 30.3% |
| Delete (orchestration glue) | 6 | 1,825 | 24.1% |
| Keep (thin DSL loaders) | 2 | 185 | 2.4% |
| Move to RP adapter | 1 | 92 | 1.2% |
| Delete (dead code) | 1 | 29 | 0.4% |
| **Total** | **33** | **7,100** | |

## Production files — `adapters/rca/adapt/`

| File | LOC | Category | Destination | Marble/Gap |
|------|-----|----------|-------------|------------|
| `basic.go` | 577 | Framework adapters | `dispatch` marble — heuristic rules become YAML data file | G3 |
| `llm.go` | 269 | LLM extraction | `dispatch` marble — LLM dispatch becomes YAML provider config | G3 |
| `routing.go` | 214 | Framework adapters | `dispatch` marble — routing log becomes built-in instrumentation | G3 |
| `stub.go` | 218 | Framework adapters | `dispatch` marble — stub provider config in YAML | G3 |

**Subtotal:** 4 files, 1,278 LOC → all `dispatch` marble (G3).

## Production files — `adapters/rca/adapt/testutil/`

| File | LOC | Category | Destination |
|------|-----|----------|-------------|
| `routing.go` | 47 | Test util | Moves with `dispatch` marble tests |

## Test files summary

| Directory | Test files | Test LOC | Migrates with |
|-----------|-----------|----------|---------------|
| `adapters/rca/` | 17 | ~5,260 | Production counterparts |
| `adapters/rca/adapt/` | 3 | ~1,149 | `dispatch` marble |
| `adapters/rca/adapt/testutil/` | 1 | 133 | `dispatch` marble |

## 22% Gap areas

| Area | LOC | Destination | Gap |
|------|-----|-------------|-----|
| `cmd/asterisk/` (10 files) | 2,032 | `origami fold` manifest — CLI commands become YAML entries | G8 |
| `internal/demo/` (5 files) | 1,436 | YAML extraction — Kabuki content becomes `demo.yaml` | — |
| `internal/mcpconfig/` (2 files) | 1,249 | `origami fold` + step schema DSL — server config becomes YAML | G8, G10 |
| `internal/dataset/` (4 files) | 649 | Framework `curate/` — schema + mapper absorbed by YAML dataset definition | — |
| `adapters/ingest/` (4 files) | 634 | Generic transformers — nodes become YAML-configured `match.pattern`, `dedup.by_key` | G9 |
| **Total** | **6,000** | | |

## Migration order

Based on gap dependencies (must resolve blockers first):

1. **G7** (`NodeDef meta:`) — smallest gap, highest leverage. Unblocks all marbles.
2. **G1 + G2** (extractor + transformer DSL) — unblocks `llm-extract` + `context-builder`.
3. **G3** (provider chains) — unblocks `dispatch`. Eliminates 1,278 LOC in `adapt/`.
4. **G4** (hook persistence DSL) — unblocks `persist`. Eliminates 154 LOC.
5. **G5** (scorer registry) — unblocks `score`. Eliminates 872 LOC.
6. **G6** (report templates) — unblocks `report`. Eliminates 826 LOC.
7. **G9** (generic transformers) — eliminates `adapters/ingest/` (634 LOC).
8. **G10** (step schema DSL) — partial `mcpconfig/` migration.
9. **G8** (`origami fold`) — eliminates `cmd/asterisk/` (2,032 LOC) + `internal/mcpconfig/` (1,249 LOC).
10. **G11** (calibration-as-circuit) — eliminates `cal_runner.go` (684 LOC) + `parallel.go` (752 LOC).

Total Go eliminated when all gaps resolved: ~14,600 LOC in `adapters/rca/` + ~6,000 LOC in gap areas = **~20,600 LOC → zero**.
