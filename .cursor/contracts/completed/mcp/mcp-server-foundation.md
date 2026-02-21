# Contract — MCP Server Foundation

**Status:** complete  
**Goal:** `asterisk serve` starts a stateful MCP server over stdio that Cursor can connect to, with placeholder tools proving the integration works.  
**Serves:** MCP integration  
**Closure:** De facto implemented during calibration MCP work. `internal/mcp/server.go` (6 tools), `cmd/asterisk/cmd_serve.go`, `.cursor/mcp.json` all exist and functional. Tests passing.

## Contract rules

- Use the official Go MCP SDK: `github.com/modelcontextprotocol/go-sdk` (v1.3.0+).
- Stdio transport only (what Cursor uses by default for local MCP servers).
- No pipeline logic in this contract — placeholder tools only. Pipeline tools come in `mcp-pipeline-tools.md`.
- The existing `calibrate` command and all dispatchers remain untouched.

## Context

- Official Go MCP SDK: `github.com/modelcontextprotocol/go-sdk/mcp`. Typed handler signatures, struct-based input schemas via `jsonschema` tags.
- Reference implementation in workspace: `reportportal-mcp-server` (uses `mark3labs/mcp-go` — different SDK, same protocol).
- Cursor MCP config: `.cursor/mcp.json` at workspace root.
- Existing Cobra CLI: `cmd/asterisk/root.go` registers subcommands (`calibrate`, `analyze`, `push`, `save`, `status`, `cursor`).

## Execution strategy

1. Add `github.com/modelcontextprotocol/go-sdk` dependency.
2. Create `internal/mcp/server.go` with `NewServer()` returning a configured MCP server.
3. Register placeholder tools: `ping` (returns pong + version), `list_scenarios` (returns available scenario names).
4. Create `cmd/asterisk/cmd_serve.go` with `asterisk serve` Cobra command using stdio transport.
5. Create `.cursor/mcp.json` pointing Cursor to `asterisk serve`.
6. Write integration test: start server in-process, call `ping`, verify response.
7. Validate (green) — build passes, tests pass, Cursor can discover the server.

## Tasks

- [ ] **Add go-sdk dependency** — `go get github.com/modelcontextprotocol/go-sdk@latest`; verify `go.mod` updated.
- [ ] **Create `internal/mcp/server.go`** — `NewServer(version string)` function returning `*mcp.Server`; register `ping` and `list_scenarios` tools.
- [ ] **Create `cmd/asterisk/cmd_serve.go`** — Cobra `serve` command; stdio transport via `mcp.StdioTransport{}`; register in `root.go`.
- [ ] **Create `.cursor/mcp.json`** — Cursor workspace config pointing to `go run ./cmd/asterisk serve`.
- [ ] **Integration test** — `internal/mcp/server_test.go`; start server in-process, call `ping` tool, verify "pong" response.
- [ ] Validate (green) — `go build ./...`, `go test ./...`, `go vet ./...` all pass.
- [ ] Tune (blue) — refine error handling, logging, shutdown. No behavior changes.
- [ ] Validate (green) — all tests still pass after tuning.

## Acceptance criteria

- **Given** the `asterisk serve` command,
- **When** Cursor connects via stdio (as configured in `.cursor/mcp.json`),
- **Then** Cursor can list tools and call `ping`, receiving a valid response.

- **Given** a call to `list_scenarios`,
- **When** the server is running,
- **Then** it returns the names of available calibration scenarios (ptp-mock, daemon-mock, ptp-real, ptp-real-ingest).

## Security assessment

Implement these mitigations when executing this contract.

| OWASP | Finding | Mitigation |
|-------|---------|------------|
| A01 | MCP server over stdio: any process that can write to stdin can invoke tools. No authentication. | For PoC (stdio transport): acceptable — Cursor IDE is the only client. Document trust model: "MCP server trusts its stdio parent process." For MVP: add tool-level authorization. |
| A04 | MCP server is a long-running process — conflicts with the "CLI-first, no always-on service" principle from poc-constraints.mdc. | `asterisk serve` runs only during a Cursor session (started by Cursor, dies with the session). Not a persistent service. Document this distinction. |
| A07 | No session management, no token-based auth for MCP transport. | Acceptable for stdio (inherited trust from parent process). Required for future network transports (SSE, WebSocket). |

## Notes

(Running log, newest first.)

- 2026-02-17 — Contract created. SDK: official go-sdk. Transport: stdio. Placeholder tools: ping, list_scenarios.
