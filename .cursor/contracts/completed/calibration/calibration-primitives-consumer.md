# Contract — calibration-primitives-consumer

**Status:** complete  
**Goal:** Refactor Asterisk's `internal/calibrate/` to import generic types from `origami/calibrate` instead of maintaining local copies.  
**Serves:** PoC completion (architecture evolution)

## Contract rules

- Depends on Origami `calibration-primitives` being complete first.
- No behavioral changes — metric values, report output, and CLI behavior must remain identical.
- `go test ./...` must pass before and after each task.

## Context

Origami's `calibrate/` package (see companion contract `origami/calibration-primitives`) provides generic `Metric`, `MetricSet`, `CaseResult`, `CalibrationReport`, `ModelAdapter`, and aggregation functions. This contract refactors Asterisk to consume those types.

Split from: `deadcode-dedup-architecture.md` boundary map P1 candidates.

### Current architecture

Asterisk defines its own `Metric`, `MetricSet`, `CaseResult`, `CalibrationReport`, `ModelAdapter`, and `aggregateRunMetrics` locally in `internal/calibrate/`. These are generic patterns with RCA-specific extensions mixed in.

### Desired architecture

Asterisk imports `origami/calibrate.Metric`, `origami/calibrate.MetricSet`, etc. Domain-specific types (`Scenario`, `GroundTruthCase`, M1-M20 score functions) remain local. `ModelAdapter.SendPrompt` uses `string` step type instead of `PipelineStep` to satisfy the generic interface.

## FSC artifacts

Code only — no FSC artifacts.

## Execution strategy

1. Import `origami/calibrate` types, alias or embed where needed.
2. Update `ModelAdapter` interface signature (`PipelineStep` → `string`).
3. Replace local aggregation with `calibrate.AggregateRunMetrics`.
4. Replace or wrap local `FormatReport` with `calibrate.FormatReport`.
5. Validate all tests still pass.

## Coverage matrix

| Layer | Applies | Rationale |
|-------|---------|-----------|
| **Unit** | yes | Verify all existing tests pass unchanged |
| **Integration** | no | No new cross-boundary interactions |
| **Contract** | yes | Compile-time interface satisfaction checks |
| **E2E** | yes | Stub calibration must produce identical output |
| **Concurrency** | no | No shared state changes |
| **Security** | no | No trust boundaries affected |

## Tasks

- [x] Import `origami/calibrate.Metric` and `origami/calibrate.MetricSet`, replace local types (type aliases)
- [x] Import `origami/calibrate.CalibrationReport`, embed for domain extensions (SuiteID, BasePath, CaseResults, Dataset)
- [x] Update `ModelAdapter.SendPrompt` signature: `step PipelineStep` → `step string`; updated 4 adapters + 5 callers + tests
- [x] Replace local `aggregateRunMetrics` with `cal.AggregateRunMetrics` + domain M19/M20 post-processing
- [x] Replace local math helpers (mean, stddev, safeDiv, safeDiv2) with `cal.*` aliases
- [x] Wrap `FormatReport` with `cal.FormatReport` + domain sections (dataset health, per-case breakdown)
- [x] Validate (green) — `go build ./...` and `go test ./...` pass; TestFormatReport output identical.
- [x] Validate (green) — all tests pass after tuning.

## Acceptance criteria

- **Given** Asterisk imports `origami/calibrate`, **when** `go build ./...` runs, **then** it compiles without local `Metric`/`MetricSet`/`CalibrationReport` type definitions.
- **Given** `ModelAdapter.SendPrompt` uses `string` step type, **when** all adapters (Stub, Basic, LLM) are compiled, **then** they satisfy both `calibrate.ModelAdapter` and the local interface.
- **Given** a stub calibration run, **when** comparing output before and after refactor, **then** metric values and report formatting are identical.

## Security assessment

No trust boundaries affected.

## Notes

2026-02-25 — Contract drafted. Companion to Origami `calibration-primitives`. Blocked until Origami contract is complete.
2026-02-25 — Contract complete. Type aliases for zero-disruption migration, CalibrationReport embedding, SendPrompt `string` step, `cal.AggregateRunMetrics` wrapper, `cal.FormatReport` delegation. All existing tests pass unchanged.
