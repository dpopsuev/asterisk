# Contract — token-perf-tracking

**Status:** complete (2026-02-17)  
**Goal:** Instrument real token usage tracking per step, per case, and per run so that every calibration and investigation run reports actual cost.

## Contract rules

- Token counting must be accurate for real (wet) runs; estimates are acceptable for stub runs.
- No external dependencies for counting (use byte-length heuristics or tiktoken-compatible Go library; avoid API calls just to count tokens).
- Tracking must not degrade circuit performance (< 1ms overhead per step).
- Cost data is never committed to the repo; lives in `.dev/` or `.asterisk/` (both git-ignored).
- All token data flows through a single `TokenTracker` interface so adapters (stub, cursor, future LLM-API) can report real or estimated counts uniformly.

## Context

- Current state: M18 (`total_prompt_tokens`) uses a rough estimate of `steps * 1000` tokens. No actual measurement exists. See `internal/calibrate/metrics.go` line 502.
- Dispatcher: `internal/calibrate/dispatcher.go` — `FileDispatcher.Dispatch()` has access to both the prompt bytes (via `PromptPath`) and the artifact bytes (via returned `[]byte`).
- CursorAdapter: `internal/calibrate/cursor_adapter.go` — calls `Dispatcher.Dispatch()` and receives raw artifact bytes.
- Runner: `internal/calibrate/runner.go` — iterates cases and steps; has access to `CaseResult` where per-step data can be accumulated.
- Report: `internal/calibrate/report.go` — prints the calibration summary.
- Pricing context: Cursor uses Claude/GPT under the hood. Rough pricing: ~$3/M input tokens, ~$15/M output tokens (varies by model). A token is ~4 chars for English text.

## Execution strategy

Four phases. Phase 1 defines the tracking types and interface. Phase 2 instruments the circuit. Phase 3 builds the cost report. Phase 4 validates and integrates with M18.

### Phase 1 — Token tracking types

- [ ] **P1.1** Define `TokenRecord` struct in `internal/calibrate/tokens.go`:
  ```
  type TokenRecord struct {
      CaseID       string
      Step         string
      PromptBytes  int      // raw byte count of prompt file
      ArtifactBytes int     // raw byte count of artifact response
      PromptTokens int      // estimated token count (bytes / 4)
      ArtifactTokens int    // estimated token count (bytes / 4)
      Timestamp    time.Time
      WallClockMs  int64    // dispatch round-trip time
  }
  ```
- [ ] **P1.2** Define `TokenTracker` interface:
  ```
  type TokenTracker interface {
      Record(r TokenRecord)
      Summary() TokenSummary
  }
  ```
- [ ] **P1.3** Define `TokenSummary` struct:
  ```
  type TokenSummary struct {
      TotalPromptTokens   int
      TotalArtifactTokens int
      TotalTokens         int
      TotalCost            float64  // estimated USD
      PerCase             map[string]CaseTokenSummary
      PerStep             map[string]StepTokenSummary
      TotalSteps          int
      TotalWallClockMs    int64
  }
  ```
- [ ] **P1.4** Implement `InMemoryTokenTracker` (thread-safe, accumulates records in a slice).
- [ ] **P1.5** Write unit tests for `InMemoryTokenTracker`: record 10 entries, verify summary totals, per-case, and per-step breakdowns.

### Phase 2 — Circuit instrumentation

- [ ] **P2.1** Extend `Dispatcher` interface or add a wrapper: after each `Dispatch()` call, measure prompt file size (from `PromptPath`) and artifact response size (from returned `[]byte`), and record a `TokenRecord`.
  - Option A: `TokenTrackingDispatcher` decorator that wraps any `Dispatcher`.
  - Option B: Recording in `CursorAdapter.SendPrompt()` after dispatch returns.
- [ ] **P2.2** Thread `TokenTracker` through the circuit:
  - `RunConfig` gets a `TokenTracker` field.
  - `runCaseCircuit` passes it to the adapter or records after each `SendPrompt`.
  - For stub adapter: estimate tokens from ground-truth response size.
- [ ] **P2.3** Extend `CaseResult` with per-case token fields:
  ```
  PromptTokensTotal   int
  ArtifactTokensTotal int
  StepCount           int
  WallClockMs         int64
  ```
- [ ] **P2.4** Write integration test: run `ptp-mock` stub calibration, verify every `CaseResult` has non-zero token counts and the `TokenTracker.Summary()` totals match.

### Phase 3 — Cost report

- [ ] **P3.1** Add cost estimation to `TokenSummary`: configurable pricing via `CostConfig{InputPricePerMToken, OutputPricePerMToken float64}`. Default: $3/M input, $15/M output.
- [ ] **P3.2** Extend `CalibrationReport` with a `TokenSummary` field.
- [ ] **P3.3** Update `FormatReport` (or the report printer) to include a "Token & Cost" section:
  ```
  === Token & Cost ===
  Total prompts:   45,230 tokens ($0.14)
  Total artifacts: 12,400 tokens ($0.19)
  Total:           57,630 tokens ($0.33)
  Per case avg:    1,921 tokens ($0.011)
  Per step avg:      320 tokens ($0.002)
  Wall clock:      4m 32s
  ```
- [ ] **P3.4** Add `--cost-report` flag (or always print when `--adapter=cursor`). Save detailed per-case breakdown to `.asterisk/calibrate/token-report.json`.
- [ ] **P3.5** Add `just` recipe: `just calibrate-cost` that runs wet calibration and pipes the token report to `.dev/calibration-runs/`.

### Phase 4 — M18 upgrade and validation

- [ ] **P4.1** Replace M18's `steps * 1000` estimate with actual `TokenTracker.Summary().TotalTokens` when available (wet runs). Keep the estimate as fallback for stub runs.
- [ ] **P4.2** Run a wet calibration round. Verify M18 reflects real token counts, not estimates.
- [ ] **P4.3** Compare real vs estimated: document the ratio so future stub estimates can be calibrated.
- [ ] **P4.4** Tune (blue) — clean up, ensure the tracker adds < 1ms overhead per step.
- [ ] **P4.5** Validate (green) — all existing tests pass, M18 still computes correctly in both stub and wet modes.

## Acceptance criteria

- **Given** a wet calibration run with `--adapter=cursor`,
- **When** the run completes,
- **Then** the report includes a Token & Cost section with accurate per-case and per-step token counts, estimated USD cost, and wall-clock time.
- **And** `token-report.json` is written to `.asterisk/calibrate/` with per-step detail.
- **And** M18 uses real token counts (not estimates) for wet runs.
- **And** stub calibration still works with estimated token counts and all tests pass.

## Dependencies

| Contract | Status | Required for |
|----------|--------|--------------|
| `e2e-calibration.md` | Complete (stub) | M18 metric, report format |
| `fs-dispatcher.md` | Active | Dispatcher interface (instrument here) |

## Notes

(Running log, newest first.)

- 2026-02-17 01:45 — Contract created. Current M18 uses `steps * 1000` estimate. Round 4 reported 156000 estimated tokens (156 steps). Real token count unknown. Goal: measure reality, compute cost, feed back into M18.
