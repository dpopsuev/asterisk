# Contract — parallel-investigation

**Status:** complete (2026-02-17)  
**Goal:** Enable parallel case processing via a job-queue pipeline with bounded worker pools for >= 3x wall-clock speedup on 30+ case scenarios without regressing M19.

## Contract rules

- Do not start until `calibration-victory.md` is complete (M19 >= 0.95 is the prerequisite; parallelism must not regress accuracy).
- Architecture changes must be backward-compatible: `--parallel=1` (default) must behave identically to the current serial runner.
- Token budget constraint: parallel execution must not increase total token usage. Symptom dedup should reduce it.
- BDD-TDD: write concurrency tests (race detector enabled) before implementing parallel paths.

## Context

- Current bottleneck analysis: serial case loop in `internal/calibrate/runner.go` line 149 (`for i, gtCase := range cfg.Scenario.Cases`). Each case processes F0-F6 sequentially. 30 cases x ~6 steps x ~5s/step = ~15 minutes.
- Dispatcher: `internal/calibrate/dispatcher.go` — `FileDispatcher` is 1:1 (one signal.json per dispatch). Needs multiplexing for parallel workers.
- Store concurrency: `MemStore` has a single `sync.Mutex`. `SqlStore` serializes via SQLite. Both need concurrent-safe access patterns.

### Concurrency model: three-stage pipeline

The architecture is a **fan-out / barrier / fan-out pipeline** using standard Go concurrency primitives — not a distributed message bus. All execution happens in a single process.

| Stage | Pattern | Go primitives | What runs |
|-------|---------|---------------|-----------|
| **Stage 1: Triage** | Bounded worker pool (fan-out) | `chan TriageJob`, N goroutine workers, `sync.WaitGroup` | F0 + F1 per case, concurrently |
| **Barrier** | Synchronization point | `wg.Wait()` | All triage must complete before clustering |
| **Stage 2: Cluster** | Pure function (single goroutine) | — | Group cases by symptom fingerprint, elect representatives |
| **Stage 3: Investigate** | Bounded worker pool (fan-out) | `chan ClusterJob`, M goroutine workers, `sync.WaitGroup` | F2-F6 per cluster representative, concurrently |

A **token-budget semaphore** (`chan struct{}` of capacity T) gates LLM calls across both worker pools, preventing cost overruns regardless of pool sizes.

### Terminology

| Term used in this contract | Meaning |
|---|---|
| **Job queue** | A buffered `chan` of work items consumed by a pool of goroutine workers |
| **Worker pool** | N goroutines reading from a shared job channel; each processes one job at a time |
| **Token semaphore** | A `chan struct{}` of fixed capacity; acquired before each LLM dispatch, released after response. Bounds concurrent API calls independently of pool size. |
| **Barrier** | A `sync.WaitGroup` synchronization point where all Stage 1 jobs must complete before Stage 2 begins |
| **Symptom cluster** | A group of cases with identical `{symptom_category, component, defect_type_hypothesis}`. Only the representative is investigated; members are linked via F4 dedup. |

## Execution strategy

Four phases. Phase 1 enables parallel triage. Phase 2 adds symptom clustering to eliminate redundant investigation. Phase 3 parallelizes investigation across clusters. Phase 4 validates speedup and accuracy.

### Phase 1 — Triage worker pool (F0-F1 fan-out)

- [ ] **P1.1** Add `--parallel=N` flag to `asterisk calibrate`. Default: 1 (serial, backward-compatible). N > 1 enables concurrent case processing.
- [ ] **P1.2** Refactor `runSingleCalibration` to split into two stages:
  - **Triage stage**: send all cases into a `chan TriageJob`. N goroutine workers consume from the channel, each running F0 (Recall) + F1 (Triage) for one case at a time. A `sync.WaitGroup` signals completion.
  - **Investigation stage**: process F2-F6 sequentially (initially; Phase 3 parallelizes this).
- [ ] **P1.3** Snapshot-isolated recall: take a read-only snapshot of the symptom table before starting Stage 1. All concurrent F0 queries run against this snapshot, preventing mid-batch cross-contamination. Implementation: copy `MemStore.symptoms` slice under lock before fan-out.
- [ ] **P1.4** Token-budget semaphore: `make(chan struct{}, T)` where T defaults to N. Each worker acquires a slot (`sem <- struct{}{}`) before calling `adapter.SendPrompt`, releases (`<-sem`) after receiving the response. This caps concurrent LLM calls independently of pool size.
- [ ] **P1.5** Write concurrency tests with `go test -race`:
  - `TestTriagePool_NoRace`: 10 cases, N=5, race detector clean.
  - `TestTriagePool_ResultsMatch`: parallel N=5 produces identical triage results as serial N=1.
- [ ] **P1.6** Validate: calibration with `--parallel=4` produces same M19 as `--parallel=1`.

### Phase 2 — Symptom clustering (barrier + grouping)

- [ ] **P2.1** After `wg.Wait()` (all triage jobs complete), run a synchronous clustering function:
  - Primary key: `{symptom_category, component, defect_type_hypothesis}` from each case's `TriageResult`.
  - Secondary key: error message similarity (Jaccard on tokenized error text).
  - Output: `[]SymptomCluster` where each cluster has a `Representative CaseID` and `[]MemberCaseID`.
- [ ] **P2.2** Elect one representative case per cluster for F2-F6 investigation. Other cluster members skip F2-F3 and link to the representative's RCA via F4 (automatic dedup).
- [ ] **P2.3** Write tests:
  - `TestSymptomClustering_Groups`: given 10 cases with 3 distinct symptom patterns, clustering produces 3 groups.
  - `TestClusterDedup_ReducesSteps`: clustered execution uses fewer total pipeline steps than serial.
- [ ] **P2.4** Validate: with clustering, total pipeline steps drop from ~180 (30 x 6) to ~90 (fewer F2-F6 runs).

### Phase 3 — Investigation worker pool (F2-F6 fan-out)

- [ ] **P3.1** Investigation job queue: after clustering, send each `SymptomCluster` into a `chan ClusterJob`. M goroutine workers (M <= N) consume clusters. Each worker runs the full F2-F3-F4-F5-F6 pipeline for its cluster's representative case. The same token semaphore from Phase 1 gates LLM calls.
- [ ] **P3.2** After all investigation workers complete (`wg.Wait()`), link cluster members to the representative's RCA in the store (batch F4 dedup).
- [ ] **P3.3** Multiplexed dispatcher: extend `FileDispatcher` to support per-worker signal directories (e.g., `.asterisk/calibrate/worker-0/signal.json`, `.asterisk/calibrate/worker-1/signal.json`). Each worker gets its own `FileDispatcher` instance pointed at its directory. Alternative: `MCPDispatcher` with JSON-RPC multiplexing.
- [ ] **P3.4** Write tests:
  - `TestInvestigationPool_AllClustersComplete`: 5 clusters, 3 workers, all clusters investigated.
  - `TestInvestigationPool_TokenSemaphore`: with semaphore capacity=2, never more than 2 concurrent LLM dispatches.
- [ ] **P3.5** Validate: full pipeline with `--parallel=4` completes in < 5 minutes for 30 cases.

### Phase 4 — Validate and benchmark

- [ ] **P4.1** Accuracy: `--parallel=4` produces M19 >= 0.95 on `ptp-real-ingest` (same as serial).
- [ ] **P4.2** Speedup: measure wall-clock time for serial vs parallel. Target: >= 3x speedup for 30 cases.
- [ ] **P4.3** Token parity: total token usage with parallel + clustering <= serial token usage (dedup saves redundant investigations).
- [ ] **P4.4** All 4 stub scenarios still pass 20/20.
- [ ] **P4.5** Tune (blue) — document the parallel architecture in `.cursor/docs/parallel-architecture.mdc`.
- [ ] **P4.6** Validate (green) — all tests pass, race detector clean.

## Acceptance criteria

- **Given** the `ptp-real-ingest` scenario with 30 cases,
- **When** `asterisk calibrate --scenario=ptp-real-ingest --adapter=cursor --dispatch=file --responder=auto --clean --parallel=4` completes,
- **Then** M19 >= 0.95 (no accuracy regression), wall-clock time is >= 3x faster than `--parallel=1`, and total token usage is <= serial token usage.
- **And** `--parallel=1` produces identical results to the current serial runner (backward compatibility).
- **And** all concurrency tests pass with `-race` flag.
- **And** all 4 stub scenarios pass 20/20.

## Dependencies

| Contract | Status | Required for |
|----------|--------|--------------|
| `calibration-victory.md` | Must be complete | M19 >= 0.95 baseline to protect |
| `token-perf-tracking.md` | Should be complete | Accurate token measurement for parity validation |
| `fs-dispatcher.md` | Complete | Dispatcher interface (extend for per-worker multiplexing) |
| `e2e-calibration.md` | Complete (stub) | Metric framework |

## Architecture

```
                     ┌──────────────────────────────────────────────┐
                     │            Calibration Runner                │
                     │  --parallel=N  --token-budget=T              │
                     └──────────────────┬───────────────────────────┘
                                        │
             ┌──────────────────────────▼───────────────────────────┐
             │  STAGE 1: Triage Worker Pool                         │
             │  chan TriageJob (buffered, all cases enqueued)        │
             │  N goroutine workers: F0 + F1 per case               │
             │  Token semaphore gates each SendPrompt call           │
             │  Snapshot-isolated recall (read-only symptom copy)    │
             │  sync.WaitGroup barrier at end                        │
             └──────────────────────────┬───────────────────────────┘
                                        │ wg.Wait()
             ┌──────────────────────────▼───────────────────────────┐
             │  BARRIER: Symptom Clustering (single goroutine)       │
             │  Group by {category, component, defect_hypothesis}    │
             │  Elect representative per cluster                     │
             └──────────────────────────┬───────────────────────────┘
                                        │
             ┌──────────────────────────▼───────────────────────────┐
             │  STAGE 3: Investigation Worker Pool                   │
             │  chan ClusterJob (one per unique symptom cluster)      │
             │  M goroutine workers: F2→F3→F4→F5→F6 per cluster     │
             │  Same token semaphore gates LLM calls                 │
             │  Per-worker FileDispatcher (own signal directory)      │
             │  sync.WaitGroup barrier at end                        │
             └──────────────────────────┬───────────────────────────┘
                                        │ wg.Wait()
             ┌──────────────────────────▼───────────────────────────┐
             │  FINALIZE: Link cluster members to representative RCA │
             │  Batch store updates (automatic F4 dedup)             │
             └──────────────────────────────────────────────────────┘
```

## Notes

(Running log, newest first.)

- 2026-02-17 02:00 — Rewrote contract with precise Go concurrency terminology. Replaced "message bus" with job queue + bounded worker pool, "map-reduce" with fan-out/barrier/fan-out pipeline, "work-stealing" with channel-based job dispatch. Added terminology table and Go primitives mapping. Added `token-perf-tracking.md` as dependency.
- 2026-02-17 01:30 — Contract created. Future phase; requires calibration-victory (M19 >= 0.95) as prerequisite.
