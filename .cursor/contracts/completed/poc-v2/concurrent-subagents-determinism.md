# Contract — Concurrent Cursor Subagents Determinism Assertion

**Status:** complete  
**Goal:** Prove that 4 concurrent Cursor subagents can deterministically drain the full MCP calibration pipeline with zero dispatch collisions, no starvation, and a complete final report.  
**Serves:** PoC completion

## Contract rules

- Test-only: zero production code changes.
- Tests live in `internal/mcp/server_test.go` alongside existing MCP parallel tests.
- Subagent goroutines simulate Cursor Task tool behavior: `get_next_step` -> `submit_artifact` loop.
- Artifact responses must be valid JSON that the runner can parse per step type.

## Current Architecture

Existing tests (`TestServer_Parallel_*`) exercise `parallel: 2` with at most 2 steps consumed. No test drains the pipeline, and no test proves 4 concurrent consumers can operate without starvation or routing errors.

## Desired Architecture

Two new integration tests in `server_test.go`:

1. `TestServer_FourSubagents_FullDrain` -- 4 goroutines drain `ptp-mock` (12 cases) through the full F0-F6 pipeline via MCP tool calls. Asserts all 4 got work, all dispatch IDs unique, report complete.
2. `TestServer_FourSubagents_NoDuplicateDispatch` -- 4 goroutines drain `daemon-mock` (8 cases). Asserts no `(case_id, step)` pair dispatched twice.

## Context

- `internal/mcp/server.go` — MCP server with `start_calibration`, `get_next_step`, `submit_artifact`, `get_report`
- `internal/mcp/session.go` — `Session` wraps `MuxDispatcher` + `RunCalibration` goroutine
- `internal/calibrate/dispatch/mux.go` — `MuxDispatcher` routes dispatch IDs under concurrency
- `internal/mcp/server_test.go` — existing parallel tests at `parallel: 2`
- `rules/agent-bus.mdc` — delegation mandate: every step dispatched to a subagent

## Tasks

- [x] Add `artifactForStep(step string, subagentID int) string` helper to `server_test.go`
- [x] Add `TestServer_FourSubagents_FullDrain` — 4 goroutines, ptp-mock, full drain, 5 assertions
- [x] Add `TestServer_FourSubagents_NoDuplicateDispatch` — 4 goroutines, daemon-mock, no duplicate dispatch
- [x] Validate — `go test ./internal/mcp/...`, full suite, lint

## Acceptance criteria

- **Given** a calibration started with `parallel: 4`, `adapter: cursor`, `scenario: ptp-mock`,
- **When** 4 concurrent goroutines loop `get_next_step` -> `submit_artifact` until `done=true`,
- **Then** all 4 goroutines processed >= 1 step, all dispatch IDs are unique, `get_report` returns `status=done` with all case results.

- **Given** a calibration started with `parallel: 4`, `adapter: cursor`, `scenario: daemon-mock`,
- **When** 4 concurrent goroutines drain the pipeline,
- **Then** no `(case_id, step)` pair appears twice in the collected work log.

## Notes

- 2026-02-19 — Contract created. Motivated by gap in MCP test coverage: existing tests stop at `parallel: 2` and do not drain the pipeline. The `agent-bus.mdc` delegation mandate requires every step to be dispatched to a subagent, but no test proves this works at 4-way concurrency.
- 2026-02-19 — Contract complete. All 4 tasks done. `callToolE` + `artifactForStep` helpers, both tests pass: FullDrain (28 steps, 12 cases, 4x7 distribution) and NoDuplicateDispatch (20 steps, 8 cases, 0 duplicates). Full suite green, zero lints.
- 2026-02-20 — **Follow-up proposed: `TestServer_FourPositions_ZoneAffinity`**. The existing tests prove the transport layer (4 goroutines can drain without collisions). The next layer is **position-aware routing**: 4 goroutines typed as PG/SG/PF/C, each preferring steps in their home zone. Assertions: PG handles >= 80% of F0+F1, PF+C handle >= 80% of F2+F3, SG handles >= 60% of F4+F5+F6. This requires either a `preferred_phases` hint in `get_next_step` (server-side filtering) or client-side step acceptance logic where workers release non-home steps back to the queue. Design source: subagent position system (agent-bus.mdc court zones). Depends on: `agent-adapter-overloading.md` Phase 1 (Position type and AffinityScheduler).
