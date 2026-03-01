# Contract: Calibrate Package Decomposition

**Priority:** HIGH — architectural  
**Status:** Complete  
**Depends on:** None (can proceed independently)

## Goal

Split the `calibrate` God package (7,307 LOC, 18+ files, 6 internal deps, 8 mixed concerns) into focused sub-packages. The `calibrate/` root remains as orchestration-only.

## Current State

The `calibrate` package mixes 8 distinct concerns in one flat namespace:

| Concern | Files | LOC |
|---------|-------|-----|
| Data types | `types.go` | ~230 |
| Circuit runner | `runner.go`, `analysis.go` | ~820 |
| Parallel execution | `parallel.go` | ~530 |
| Metrics scoring | `metrics.go` | ~710 |
| Formatting | `report.go`, `briefing.go`, `tokimeter.go` | ~450 |
| Dispatchers | `dispatcher.go`, `batch_dispatcher.go`, `token_dispatcher.go`, `batch_manifest.go` | ~760 |
| Adapters | `adapter.go`, `basic_adapter.go`, `cursor_adapter.go` | ~670 |
| Lifecycle | `lifecycle.go`, `cluster.go`, `tokens.go` | ~440 |

## Target State

```
internal/calibrate/
├── calibrate.go          # Public API: RunCalibration, RunAnalysis (orchestration only)
├── types.go              # Shared types: Scenario, GroundTruth*, CaseResult, CalibrationReport
├── runner.go             # Core circuit sequencing
├── parallel.go           # Parallel execution (errgroup-based after concurrency contract)
├── lifecycle.go          # Pre/post-run lifecycle hooks
├── cluster.go            # Symptom clustering
├── tokens.go             # Token cost summary
├── dispatch/
│   ├── dispatch.go       # Dispatcher interface + StdinDispatcher
│   ├── file.go           # FileDispatcher
│   ├── batch.go          # BatchFileDispatcher + ManifestWriter
│   ├── token.go          # TokenDispatcher
│   └── manifest.go       # Batch manifest schema
├── metrics/
│   ├── metrics.go        # Metric interface + registry
│   ├── structural.go     # M1–M5: defect type, category, component, evidence, RCA
│   ├── workspace.go      # M6–M8: repo selection, file relevance, commit relevance
│   ├── evidence.go       # M9–M11: evidence quality, chain completeness, cited lines
│   ├── semantic.go       # M12–M14: keyword, message quality, confidence calibration
│   ├── circuit.go       # M15–M17: completion, path accuracy, step count
│   └── aggregate.go      # M18–M20: token efficiency, overall accuracy, consistency
├── adapt/
│   ├── adapt.go          # ModelAdapter interface
│   ├── basic.go          # BasicAdapter (keyword classifier)
│   └── cursor.go         # CursorAdapter (external agent)
└── scenarios/            # (unchanged) ptp_mock.go, daemon_mock.go, ptp_real.go, etc.
```

## Tasks

### Phase 1: Extract dispatchers → `calibrate/dispatch/`
1. Create `internal/calibrate/dispatch/` package
2. Move `Dispatcher` interface + `StdinDispatcher` → `dispatch.go`
3. Move `FileDispatcher` → `file.go`
4. Move `BatchFileDispatcher` + `ManifestWriter` → `batch.go`
5. Move `TokenDispatcher` → `token.go`
6. Move batch manifest types → `manifest.go`
7. Update all import paths in `calibrate/`, `cmd/asterisk/`
8. Run tests — all green

### Phase 2: Extract metrics → `calibrate/metrics/`
1. Create `internal/calibrate/metrics/` package
2. Define `Metric` interface: `Score(scenario, results) float64`
3. Split 20 scorers into 6 thematic files (structural, workspace, evidence, semantic, circuit, aggregate)
4. Create scorer registry for iteration
5. Update `runner.go` and `report.go` to use `metrics.ScoreAll()`
6. Run tests — all green

### Phase 3: Extract adapters → `calibrate/adapt/`
1. Create `internal/calibrate/adapt/` package
2. Move `ModelAdapter` interface → `adapt.go`
3. Move `BasicAdapter` → `basic.go`
4. Move `CursorAdapter` → `cursor.go`
5. Update all import paths
6. Run tests — all green

### Phase 4: Clean `calibrate/` root
1. Remove now-empty source files
2. Ensure `calibrate/` root only contains: types, runner, parallel, lifecycle, cluster, tokens
3. Verify no circular imports
4. Run full test suite
5. Measure LOC: target <= 2,500 in root (down from 7,307)

## Completion Criteria
- `calibrate/` root <= 2,500 LOC
- Three new sub-packages: `dispatch/`, `metrics/`, `adapt/`
- All existing tests pass
- No circular imports
- No public API changes (types still exported from `calibrate`)

## Cross-Reference
- Architecture: `.cursor/docs/architecture.md`
- Concurrency modernization: `.cursor/contracts/concurrency-modernization.md` (parallel.go changes)
- Test coverage: `.cursor/contracts/critical-test-coverage.md` (metrics tests)
