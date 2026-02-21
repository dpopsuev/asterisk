# Contract — Domain-Specific Calibration

**Status:** draft  
**Goal:** Explicitly scope calibration as a per-domain concern and establish Origami's domain calibration alongside Asterisk's existing one.  
**Serves:** Architecture evolution (vision)

## Contract rules

- Each problem domain owns its calibration: ground truth, metrics, runner, and acceptance thresholds.
- Domain calibration measures the **pipeline's** accuracy on the **problem**, not the model's generic capabilities (that is meta-calibration's job).
- Domain calibration may consume `ModelProfile` data from meta-calibration for routing optimization, but must not depend on it for correctness.
- New domain calibrations follow the same pattern: ground truth, metrics, runner, scorer.

## Context

The project currently has one calibration system (`internal/calibrate/`) which is implicitly Asterisk-specific. It measures F0-F6 pipeline accuracy against RCA ground truth (M1-M20 metrics, `calibrate.Scenario`, `GroundTruthCase`, `GroundTruthRCA`).

With Origami emerging as a second pipeline (curation: fetch-extract-validate-enrich-promote), there is no calibration system for information extraction quality. This contract scopes the distinction clearly and lays groundwork for Origami's domain calibration.

### Domain calibration taxonomy

| Domain | Product | Pipeline | Ground truth | Metrics | Package |
|--------|---------|----------|-------------|---------|---------|
| **Asterisk** | Root Cause Analysis | F0-F6 (Light) + D0-D4 (Shadow) | `calibrate.Scenario` (GroundTruthCase, GroundTruthRCA, GroundTruthSymptom) | M1-M20 (defect type accuracy, evidence recall, convergence, etc.) | `internal/calibrate/` |
| **Origami** | Information Extraction | Curation pipeline (fetch-extract-validate-enrich-promote) | `curate.Dataset` + `curate.Schema` (known-correct records with verified fields) | Extraction recall, field accuracy, completeness score, source coverage | `internal/curate/eval/` (new) |

### Key insight: domain calibration consumes meta-calibration

When running domain calibration, the system can optionally:
1. Look up the `ModelProfile` for the adapter's model (from meta-calibration)
2. Use the model's `ElementMatch` and `CostProfile` to optimize routing (e.g. assign a Water-affinity model to investigation-heavy steps)
3. Domain calibration then measures pipeline output quality, not model capability

Dependency: `domain calibration --optionally uses--> ModelProfile --from--> meta-calibration`

### Current architecture (Asterisk only)

```
internal/calibrate/
├── types.go          # Scenario, GroundTruthCase, MetricSet, CaseResult
├── runner.go         # RunCalibration, runSingleCalibration, runCasePipeline
├── metrics.go        # computeMetrics (M1-M20)
├── court_runner.go   # RunCourt (D0-D4 shadow pipeline)
├── court_metrics.go  # CourtMetrics
├── tuning.go         # TuningRunner, QuickWin
├── adapt/            # StubAdapter, BasicAdapter, CursorAdapter
├── dispatch/         # MuxDispatcher, BatchFileDispatcher
└── scenarios/        # ptp-real-ingest, mock scenarios
```

### Desired architecture (domain-scoped)

```
internal/calibrate/         # Asterisk domain calibration (unchanged)
├── doc.go                  # Explicitly: "Asterisk RCA domain calibration"
├── types.go                # Scenario, GroundTruthCase, MetricSet
├── runner.go               # RunCalibration
├── metrics.go              # M1-M20
└── ...

internal/curate/eval/       # Origami domain calibration (new)
├── types.go                # ExtractionScenario, ExtractionCase, ExtractionMetrics
├── runner.go               # RunEvaluation
├── metrics.go              # Extraction recall, field accuracy, completeness
└── *_test.go
```

## FSC artifacts

| Artifact | Target | Compartment |
|----------|--------|-------------|
| Domain calibration taxonomy | `docs/domain-calibration.md` | domain |
| Calibration glossary entries | `glossary/` | domain |

## Execution strategy

Phase 1 scopes and documents the distinction (no code changes to existing calibrate package). Phase 2 defines Origami evaluation types. Phase 3 implements the evaluation runner. Phase 4 wires the optional meta-calibration consumption.

## Tasks

### Phase 1 — Scope and document

- [ ] Add explicit doc.go comment to `internal/calibrate/` scoping it as Asterisk RCA domain calibration
- [ ] Add doc.go to `internal/curate/` scoping it as generic curation (domain-agnostic)
- [ ] Document the domain calibration taxonomy in `docs/domain-calibration.md`

### Phase 2 — Origami evaluation types (`internal/curate/eval/`)

- [ ] Define `ExtractionScenario` — a dataset with known-correct records as ground truth
- [ ] Define `ExtractionCase` — a single record with expected fields and their correct values
- [ ] Define `ExtractionMetrics` — extraction recall, field accuracy, completeness score, source coverage, promotion rate
- [ ] Define `EvaluationResult` — per-case scoring (present/missing/incorrect fields, completeness score)
- [ ] Unit tests for all types

### Phase 3 — Origami evaluation runner

- [ ] Implement `RunEvaluation(ctx, scenario, walker, graph) EvaluationReport`
- [ ] Walk each case through the curation pipeline, compare extracted fields to ground truth
- [ ] Compute aggregate metrics: mean extraction recall, field accuracy, completeness
- [ ] Score source coverage: did the pipeline find all available sources?
- [ ] Score promotion rate: what fraction of records were promoted?
- [ ] Integration test with stub source/extractor walking the curation graph

### Phase 4 — Meta-calibration consumption (optional)

- [ ] Define `ModelProfileProvider` interface in `internal/framework/metacal/`
- [ ] Add optional `ModelProfileProvider` to `calibrate.RunConfig`
- [ ] When present, log model profile alongside calibration results
- [ ] When present, use `CostProfile` for token budget estimation
- [ ] Do NOT change routing or adapter selection yet (that is a Phase 4 of meta-calibration)

### Phase 5 — Validate and tune

- [ ] Validate (green) — all existing calibrate tests still pass, new curate/eval tests pass
- [ ] Tune (blue) — review naming, ensure packages remain cleanly scoped
- [ ] Validate (green) — all tests still pass after tuning

## Acceptance criteria

**Given** the existing `internal/calibrate/` package,  
**When** this contract is complete,  
**Then** it is explicitly documented as "Asterisk RCA domain calibration" and its behavior is unchanged.

**Given** a `curate.Dataset` with known-correct records,  
**When** `RunEvaluation` is executed with the curation graph and a stub walker,  
**Then** an `EvaluationReport` is produced with:
- Per-case field accuracy scores
- Aggregate extraction recall and completeness metrics
- Promotion rate

**Given** a domain calibration run (Asterisk or Origami),  
**When** a `ModelProfileProvider` is configured,  
**Then** the model's profile is logged alongside calibration results (informational only, no behavioral change).

## Security assessment

No trust boundaries affected. Domain calibration operates on ground truth datasets (synthetic or curated) with no external API calls beyond what existing calibration already does.

## Notes

2026-02-21 12:00 — Contract created to formally distinguish meta-calibration (model assessment) from domain calibration (pipeline quality). Asterisk's existing `internal/calibrate/` is the reference implementation for domain calibration. Origami's `internal/curate/eval/` is the second instance.
