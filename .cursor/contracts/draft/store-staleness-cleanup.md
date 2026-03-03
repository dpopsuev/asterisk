# Contract — store-staleness-cleanup

**Status:** complete
**Goal:** Aborted or completed circuit sessions leave a clean Kami store (no frozen walkers); manual reset available via MCP tool, CLI flag, and standalone command.
**Serves:** 100% DSL — Zero Go

## Contract rules

Global rules only.

## Context

When a calibration session is aborted mid-flight, walkers remain frozen in Kami's `CircuitStore`. The next Sumi connection bootstraps from `/api/snapshot` and sees stale walkers from the dead session. The store is only replaced when a *new* `start_circuit` creates a fresh one — there is no cleanup on session teardown or manual reset.

- `modules/rca/mcpconfig/server.go` — `createSession` creates new store but old session teardown doesn't clean up
- `mcp/circuit_server.go` — `handleStartCircuit` cancels old session without notifying Kami
- `mcp/circuit_config.go` — has `OnCircuitDone` but no `OnSessionEnd`
- `kami/server.go` — `SetStore` closes old store; no standalone reset

### Current architecture

Session abort leaves walkers frozen; Sumi sees stale state.

### Desired architecture

Session teardown (done, abort, force-replace) clears walkers automatically. Three manual reset surfaces: `kami_reset_store` MCP tool, `origami sumi --watch --clean` flag, `origami kami reset` CLI subcommand — all backed by `POST /api/store/reset`.

## FSC artifacts

Code only — no FSC artifacts.

## Execution strategy

1. Add `POST /api/store/reset` HTTP endpoint to Kami server — resets or nils the store
2. Add `OnSessionEnd` callback to `CircuitConfig`
3. Wire it in `handleStartCircuit` on all teardown paths
4. Wire it in `mcpconfig/server.go` to emit `WalkComplete`
5. Add `kami_reset_store` MCP tool — calls the HTTP endpoint
6. Add `--clean` flag to `origami sumi --watch` — POSTs to `/api/store/reset` before bootstrapping
7. Add `origami kami reset <addr>` standalone CLI subcommand
8. Test: aborted session snapshot shows completed=true, zero walkers

## Coverage matrix

| Layer | Applies | Rationale |
|-------|---------|-----------|
| **Unit** | yes | `CircuitStore` state after `WalkComplete` on abort |
| **Integration** | yes | Session lifecycle: start → abort → snapshot shows clean state |
| **Contract** | no | No public API schema changes |
| **E2E** | yes | Sumi bootstrap after aborted session sees no stale walkers |
| **Concurrency** | yes | Session replacement while SSE clients are connected |
| **Security** | no | No trust boundaries affected |

## Tasks

- [x] Add `POST /api/store/reset` HTTP endpoint to Kami server (Origami `kami/server.go`)
- [x] Add `OnSessionEnd` callback to `CircuitConfig` (Origami `mcp/circuit_config.go`)
- [x] Call `OnSessionEnd` in `handleStartCircuit` on all teardown paths (done, abort, force)
- [x] Wire `OnSessionEnd` in `mcpconfig/server.go` to emit `WalkComplete`
- [x] Add `kami_reset_store` MCP tool backed by the HTTP endpoint (Origami `kami/mcp_tools.go`)
- [x] Add `--clean` flag to `origami sumi --watch` — POST reset before bootstrap (Origami `cmd/origami/main.go` + `sumi/run.go`)
- [x] Add `origami kami reset <addr>` CLI subcommand (Origami `cmd/origami/main.go`)
- [x] Test: aborted session clears walkers; snapshot shows completed + zero walkers
- [x] Test: manual reset tool clears store
- [x] Test: `--clean` flag resets store before bootstrap
- [x] Validate (green) — all tests pass, acceptance criteria met.
- [x] Tune (blue) — no changes needed.
- [x] Validate (green) — all tests still pass after tuning.

## Acceptance criteria

Given a 4-parallel calibration session is started and then aborted mid-flight
When Sumi connects and bootstraps from `/api/snapshot`
Then the snapshot shows `completed=true` and zero walkers

Given Sumi is connected and displaying stale state
When the agent calls `kami_reset_store` (or `kami_clear_all`)
Then Sumi receives a `DiffReset` and renders an empty circuit

Given a Kami server has stale state from a previous session
When the user runs `origami sumi --watch <addr> --clean`
Then Sumi POSTs `/api/store/reset` before bootstrapping and starts with a clean circuit

Given a Kami server has stale state
When the user runs `origami kami reset <addr>`
Then the store is reset and the command exits with confirmation

## Security assessment

No trust boundaries affected.

## Notes

2026-03-03 23:00 — Contract complete. All tasks implemented, tested (3 integration tests), race-free, pushed.
2026-03-03 22:30 — Contract created from plan-mode discussion. Root cause: `handleStartCircuit` cancels old sessions without notifying Kami's store. Keeping node states post-abort is intentional (post-mortem); only walkers are cleared.
