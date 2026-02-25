# Contract — asterisk-calibrate-skill

**Status:** complete  
**Goal:** Ship an `asterisk-calibrate` Cursor Skill that drives wet LLM calibration via the existing MCP server — zero file writes, zero approval gates.  
**Serves:** PoC completion (gate)

## Contract rules

- The skill MUST use MCP tool calls (`start_calibration`, `get_next_step`, `submit_artifact`, `get_report`). FileDispatcher is banned — it was a dead end (see `goals/poc.mdc` CRISIS section).
- Every pipeline step MUST be delegated to a Task subagent per the agent bus protocol (`origami/.cursor/rules/domain/agent-bus.mdc`).
- The skill MUST reference the existing `asterisk-analyze` artifact schemas — no duplication.

## Context

- `goals/poc.mdc` — CRISIS section documents why FileDispatcher was abandoned.
- `internal/mcp/server.go` — MCP server with 6 tools.
- `internal/mcp/session.go` — MuxDispatcher-based session; capacity gate, TTL watchdog.
- `skills/asterisk-analyze/SKILL.md` — proven pattern for agentic skills.
- `origami/.cursor/rules/domain/agent-bus.mdc` — delegation mandate and signal protocol.
- `origami/.cursor/notes/parallel-subagent-test-results.md` — 4 concurrent subagents confirmed.

## FSC artifacts

| Artifact | Target | Compartment |
|----------|--------|-------------|
| `asterisk-calibrate` skill | `skills/asterisk-calibrate/SKILL.md` | domain |

## Execution strategy

Single-phase: write the SKILL.md, update the skills index, verify MCP server starts cleanly, run unit tests. Wet validation is a manual step (invoke the skill trigger in Cursor).

## Coverage matrix

| Layer | Applies | Rationale |
|-------|---------|-----------|
| **Unit** | yes | MCP server tests (`internal/mcp/`) verify tool handlers, session lifecycle, capacity gate. |
| **Integration** | yes | `just calibrate-stub` confirms the pipeline still passes with stub adapter. |
| **Contract** | N/A | No new API schemas; skill references existing MCP tool signatures. |
| **E2E** | yes | Manual: invoke `/asterisk-calibrate ptp-mock` in Cursor and verify M1-M21 scorecard. |
| **Concurrency** | yes | Parallel mode (4 subagents) exercises MuxDispatcher routing. |
| **Security** | N/A | No new trust boundaries; MCP server already validated. |

## Tasks

- [x] Create `skills/asterisk-calibrate/SKILL.md` with MCP-based pull loop, subagent delegation, batch mode.
- [x] Update `skills/index.mdc` with new skill entry.
- [x] Verify binary builds and MCP server starts cleanly.
- [x] Run `go test ./internal/mcp/...` — all pass.
- [x] Validate (green) — wet validation is Phase 5a of `rp-e2e-launch` (M19=0.58, 18 cases completed).
- [x] Tune (blue) — v1 -> v2 migration done via `papercup-v2-hardening`.
- [x] Validate (green) — R11 run confirmed skill works with 4 parallel workers.

## Acceptance criteria

```gherkin
Given the Asterisk MCP server is configured in .cursor/mcp.json
  And the binary is built (go build -o bin/asterisk ./cmd/asterisk/)
When the user triggers "/asterisk-calibrate ptp-mock"
Then the Cursor agent calls start_calibration(scenario="ptp-mock", adapter="cursor")
  And delegates each F0-F6 step to Task subagents via MCP tools
  And presents an M1-M21 metrics scorecard on completion
  And no file-based signal protocol is used (zero signal.json writes)
```

## Security assessment

No trust boundaries affected. MCP server already validated in prior contracts (`mcp-server-foundation`, `mcp-pipeline-tools`).

## Notes

2026-02-23 23:10 — Created. SKILL.md written with MCP-based architecture. Corrected from initial FileDispatcher approach after reviewing CRISIS documentation in poc.mdc.

2026-02-24 08:15 — **v1 → v2 migration.** The SKILL.md was rewritten as part of `papercup-v2-hardening` contract. The original skill implemented Papercup v1 orchestration (parent owns get_next_step/submit_artifact in a batch-pull loop), which caused "Weakest Link" and "Batching" anti-patterns during wet runs. The new skill implements Papercup v2 choreography: parent is a supervisor that launches worker Tasks with a server-generated `worker_prompt`. Workers own the full get_next_step/submit_artifact loop independently. Server-side changes include `WorkerPrompt()`, inline `prompt_content`, protocol-agnostic gate messages, and worker mode tracking via `worker_started` signals with `meta.mode="stream"`. See `papercup-v2-hardening.md` for full details.
2026-02-25 — **Contract complete.** All code tasks done. Skill shipped, v2 choreography working, 4 parallel workers confirmed in R11 wet run (10m 9s, $0.38, 65 steps). Wet validation merged into `rp-e2e-launch` Phase 5a — running the calibrate skill IS Phase 5a. Remaining accuracy work tracked in `phase-5a-v2-analysis`.
