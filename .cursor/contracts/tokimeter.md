# Contract — tokimeter

**Status:** complete (2026-02-17)  
**Goal:** Markdown-formatted cost bill ("TokiMeter" — taximeter for tokens) printed at the end of every calibration/investigation run showing per-case cost, per-step cost, and total launch cost.

## Contract rules

- TokiMeter output is always printed when token tracking is active (no extra flag required).
- Output format is Cursor-friendly Markdown (tables, bold, headers) so it renders nicely in the IDE terminal.
- A `tokimeter.md` file is written alongside the existing `token-report.json` for easy review.
- Pricing uses the same `CostConfig` as `TokenSummary` ($3/M input, $15/M output by default).
- Token counts are estimated at ~4 chars/token (same as `EstimateTokens`).

## Context

- Extends the token tracking system from `token-perf-tracking.md` (complete).
- `TokenSummary` already has `PerCase` and `PerStep` breakdowns — TokiMeter joins those with `CaseResult` metadata (test name, version, job) for a richer bill.
- The existing `FormatTokenSummary` produces a plain-text summary; TokiMeter replaces it as the primary cost display with a structured markdown document.

## Implementation

### Types (`internal/calibrate/tokimeter.go`)

- `TokiMeterBill` — top-level bill struct with scenario, adapter, timestamp, totals, wall clock.
- `TokiMeterCaseLine` — per-case row: case ID, test name, version/job, steps, in/out tokens, cost, wall time.
- `TokiMeterStepLine` — per-step row: step name, invocations, in/out tokens, cost.
- `BuildTokiMeterBill(report *CalibrationReport) *TokiMeterBill` — joins CaseResult + TokenSummary data.
- `FormatTokiMeter(bill *TokiMeterBill) string` — renders markdown bill with 3 sections:
  1. **Summary** — total cases, steps, tokens, cost, wall clock, per-case average.
  2. **Per-case costs** — table sorted by case ID.
  3. **Per-step costs** — table in pipeline order (F0-F6) with TOTAL footer row.

### CLI wiring (`cmd/asterisk/main.go`)

- After `FormatReport`, build and print TokiMeter bill.
- Write `tokimeter.md` to calibration directory.

### Tests (`internal/calibrate/tokimeter_test.go`)

- `TestBuildTokiMeterBill_NilTokens` — nil safety.
- `TestBuildTokiMeterBill_Basic` — 2-case scenario, verify cost calculation and step ordering.
- `TestFormatTokiMeter_Nil` — empty output for nil bill.
- `TestFormatTokiMeter_Markdown` — verify markdown structure, table headers, data presence, truncation.
- `TestFmtTokens` — K-suffix formatting.
- `TestFmtDuration` — minute/second formatting.

## Acceptance criteria

- **Given** a calibration run with token tracking active,
- **When** the run completes,
- **Then** a markdown TokiMeter bill is printed to stdout with per-case and per-step cost tables.
- **And** a `tokimeter.md` file is written to the calibration directory.
- **And** all TokiMeter tests pass.
- **And** the bill totals match the existing `TokenSummary` totals.

## Dependencies

| Contract | Status | Required for |
|----------|--------|--------------|
| `token-perf-tracking.md` | Complete | TokenTracker, TokenSummary, CostConfig |

## Notes

- 2026-02-17 — Implementation complete: `tokimeter.go`, `tokimeter_test.go`, `main.go` wiring. All tests pass. Validated with wet calibration run.
