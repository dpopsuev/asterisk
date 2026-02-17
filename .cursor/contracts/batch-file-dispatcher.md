# Contract — batch-file-dispatcher

**Status:** complete (2026-02-17)  
**Goal:** Implement `BatchFileDispatcher` in Go — write N signals concurrently, generate batch manifest and briefing, poll N artifact paths in parallel — so the parallel pipeline can drive multiple Cursor subagents simultaneously.

## Contract rules

- BDD-TDD **Red-Orange-Green-Blue** per `rules/test-coverage-checklist.mdc`.
- `--dispatch=file --parallel=1` must produce identical results to today (backward compatibility).
- `BatchFileDispatcher` wraps N `FileDispatcher` instances — reuse, don't rewrite, the existing polling logic.
- The `Dispatcher` interface must not change; `BatchFileDispatcher` is a new implementation alongside `FileDispatcher`.
- All new concurrency must pass `go test -race`.
- Token semaphore from `parallel.go` continues to gate actual dispatches; `BatchFileDispatcher` respects it.

## Context

- **Batch dispatch protocol**: `contracts/batch-dispatch-protocol.md` — defines manifest schema, briefing format, concurrent semantics.
- **Existing FileDispatcher**: `internal/calibrate/dispatcher.go` — single-signal write + poll. One `Dispatch()` call blocks until artifact appears.
- **Parallel runner**: `internal/calibrate/parallel.go` — worker pools call `cfg.Adapter.SendPrompt()` which calls `dispatcher.Dispatch()`. Currently sequential per worker.
- **Token tracking**: `internal/calibrate/token_dispatcher.go` — decorator wrapping any `Dispatcher`. `BatchFileDispatcher` must be compatible (wrappable by `TokenTrackingDispatcher`).
- **CursorAdapter**: `internal/calibrate/cursor_adapter.go` — calls `dispatcher.Dispatch()` per case+step. Must be wirable to `BatchFileDispatcher`.
- **MemStore**: `internal/store/memstore_v2.go` — has `SnapshotSymptoms()` for read isolation. Briefing generator will read from store.
- **CLI**: `cmd/asterisk/main.go` — `--dispatch` flag currently accepts `stdin` or `file`.

## Design

### BatchFileDispatcher

```go
type BatchFileDispatcher struct {
    cfg       FileDispatcherConfig
    log       *slog.Logger
    suiteDir  string           // .asterisk/calibrate/{suiteID}
    batchID   int64            // monotonic batch counter
    store     store.Store      // for briefing generation
    scenario  *Scenario        // for briefing generation
}
```

The `BatchFileDispatcher` does not implement `Dispatcher` directly (which is per-case). Instead, it provides a **batch-level API** used by the parallel runner:

```go
// DispatchBatch writes N signals, generates manifest + briefing,
// then polls all N artifact paths concurrently.
// Returns results in the same order as the input contexts.
func (d *BatchFileDispatcher) DispatchBatch(ctxs []DispatchContext) ([][]byte, []error)
```

Internally, each slot uses a `FileDispatcher` instance for the actual polling logic.

### ManifestWriter

```go
func WriteManifest(path string, batch *BatchManifest) error
```

Writes `batch-manifest.json` atomically (temp + rename, same pattern as `writeSignal`).

### BriefingGenerator

```go
func GenerateBriefing(st store.Store, scenario *Scenario, suiteID int64, phase string, batchID int64) (string, error)
```

Reads from the store to produce a markdown briefing:
- Run context (scenario name, suite ID, phase, case counts)
- Known symptoms (from `st.ListSymptoms()`)
- Cluster assignments (from the latest clustering result, if investigation phase)
- Prior RCAs (from `st.ListRCAs()`)
- Common error patterns (frequency analysis of error messages)

Returns the briefing as a string; caller writes to disk.

### Wiring into parallel runner

In `runParallelCalibration`, when `--dispatch=batch-file`:

**Triage phase**: Instead of N workers each calling `Dispatch()` sequentially, collect all triage `DispatchContext`s for a batch (up to batch size), call `DispatchBatch()`, distribute results.

**Investigation phase**: Same pattern — collect investigation `DispatchContext`s per batch, call `DispatchBatch()`.

### CLI changes

- `--dispatch` flag accepts new value: `batch-file` (alongside existing `stdin`, `file`).
- When `--dispatch=batch-file`, instantiate `BatchFileDispatcher` and wire into `CursorAdapter`.
- `--batch-size` flag (default 4): maximum signals per batch manifest. Matches Cursor's subagent cap.

## Execution strategy

Four phases. Phase 1 builds the manifest and briefing writers. Phase 2 implements `BatchFileDispatcher`. Phase 3 wires into the parallel runner. Phase 4 validates end-to-end.

### Phase 1 — Manifest and briefing (Red-Green)

- [ ] **P1.1** Define `BatchManifest` struct in `internal/calibrate/batch_manifest.go`. Fields match the schema in `batch-dispatch-protocol.md`.
- [ ] **P1.2** Implement `WriteManifest` and `ReadManifest` — atomic JSON write/read with temp+rename.
- [ ] **P1.3** Write tests: `TestWriteManifest_RoundTrip`, `TestWriteManifest_Atomic`.
- [ ] **P1.4** Implement `GenerateBriefing` in `internal/calibrate/briefing.go`. Read from `MemStore`, produce markdown string.
- [ ] **P1.5** Write tests: `TestGenerateBriefing_TriagePhase`, `TestGenerateBriefing_InvestigationPhase` (with cluster data), `TestGenerateBriefing_Empty` (no prior data).

### Phase 2 — BatchFileDispatcher (Red-Green)

- [ ] **P2.1** Implement `BatchFileDispatcher` in `internal/calibrate/batch_dispatcher.go`.
- [ ] **P2.2** `DispatchBatch`: write all signals, write manifest, write briefing, then poll all artifact paths concurrently using one goroutine per signal (bounded by batch size). Collect results via channel.
- [ ] **P2.3** Error handling: if a signal errors (timeout, invalid JSON), record the error for that slot and continue polling others. Return partial results with per-slot errors.
- [ ] **P2.4** Manifest lifecycle: update manifest status to `in_progress` when polling starts, update per-signal status as artifacts arrive, set to `done` when all complete.
- [ ] **P2.5** Write tests with `-race`:
  - `TestBatchDispatch_AllComplete`: 4 signals, mock artifacts written by test goroutines, all 4 collected.
  - `TestBatchDispatch_PartialFailure`: 4 signals, 1 times out, other 3 succeed. Verify partial results.
  - `TestBatchDispatch_ManifestLifecycle`: verify manifest transitions from `pending` -> `in_progress` -> `done`.
  - `TestBatchDispatch_BriefingGenerated`: verify briefing.md is written before polling starts.

### Phase 3 — Wire into parallel runner (Green)

- [ ] **P3.1** Add `--dispatch=batch-file` and `--batch-size=N` flags to CLI.
- [ ] **P3.2** Refactor `runParallelCalibration` to detect `BatchFileDispatcher`:
  - Triage phase: collect `DispatchContext`s in batches of `batch-size`, call `DispatchBatch` per batch.
  - Investigation phase: same pattern for cluster representatives.
- [ ] **P3.3** Ensure `TokenTrackingDispatcher` can wrap per-slot dispatches within `BatchFileDispatcher` (record tokens per signal, not per batch).
- [ ] **P3.4** Add `just calibrate-batch` recipe: `asterisk calibrate --scenario=ptp-mock --adapter=stub --dispatch=batch-file --parallel=4 --batch-size=4`.
- [ ] **P3.5** Write integration test: `TestParallelCalibration_BatchDispatch` — 12 cases, batch-size=4, stub adapter, verify same results as serial.

### Phase 4 — Validate and tune (Blue)

- [ ] **P4.1** Backward compatibility: `--dispatch=file --parallel=4` produces same results as before (no regressions).
- [ ] **P4.2** `--dispatch=batch-file --parallel=4 --batch-size=4` with stub adapter on ptp-mock: 20/20 metrics, race detector clean.
- [ ] **P4.3** Verify manifest and briefing files are generated correctly by inspecting `.asterisk/calibrate/` after a run.
- [ ] **P4.4** Tune (blue) — refactor for clarity, document batch dispatcher in `docs/parallel-architecture.mdc`.
- [ ] **P4.5** Validate (green) — all tests pass, race detector clean.

## Acceptance criteria

- **Given** `asterisk calibrate --scenario=ptp-mock --adapter=stub --dispatch=batch-file --parallel=4 --batch-size=4`,
- **When** calibration completes,
- **Then** 20/20 metrics pass, `batch-manifest.json` shows `status: "done"`, and `briefing.md` exists with populated content.

- **Given** `--dispatch=file --parallel=4` (old mode),
- **When** calibration completes,
- **Then** results are identical to before this contract (no `batch-manifest.json` generated, no behavior change).

- **Given** a batch of 4 signals where 1 times out,
- **When** `DispatchBatch` completes,
- **Then** 3 results are returned successfully, 1 has an error, and the manifest shows 3 `done` + 1 `error`.

- **Given** `go test -race ./internal/calibrate/...`,
- **When** all batch dispatch tests run,
- **Then** no data races are detected.

## Dependencies

| Contract | Status | Required for |
|----------|--------|--------------|
| `batch-dispatch-protocol.md` | Draft | Schema definitions for manifest and briefing |
| `parallel-investigation.md` | Complete | Worker pool architecture to extend |
| `token-perf-tracking.md` | Complete | Token tracking decorator compatibility |
| `fs-dispatcher.md` | Complete | `FileDispatcher` to wrap |

## Notes

(Running log, newest first.)

- 2026-02-17 23:30 — Contract complete. Implemented `batch_manifest.go`, `briefing.go`, `batch_dispatcher.go` with tests. CLI wired with `--dispatch=batch-file` and `--batch-size`. `calibrate-batch` recipe added to justfile. All tests pass with race detector.
- 2026-02-17 22:00 — Contract created. Go-side implementation of batch dispatch protocol. Introduces `BatchFileDispatcher`, `ManifestWriter`, `BriefingGenerator`. Wires into parallel runner with `--dispatch=batch-file` flag.
