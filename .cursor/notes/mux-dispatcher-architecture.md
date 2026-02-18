# MuxDispatcher Architecture — Current vs Desired State

## Current State

### MCPDispatcher (`internal/calibrate/dispatch/mcp.go`)

Single-channel bridge between the calibration runner and MCP tool handlers.

```
promptCh   chan DispatchContext   (unbuffered)
artifactCh chan []byte            (unbuffered)
errCh      chan error             (buffered 1)
```

**Parallel bug:** When `Parallel > 1`, multiple runner goroutines call `Dispatch()` concurrently. Each blocks on `<-artifactCh` after sending its prompt. When `SubmitArtifact` sends data, **any** waiting goroutine can receive it — delivering the wrong artifact to the wrong case.

### CursorAdapter coupling (`internal/calibrate/adapt/cursor.go`)

Lines 176-182 have type assertions for `*TokenTrackingDispatcher` and `*FileDispatcher` to call `MarkDone()`. This couples the adapter to specific dispatcher implementations.

### DispatchContext lacks identity

`DispatchContext` has `CaseID` and `Step` but no `DispatchID`. There is no way to correlate a `submit_artifact` with the specific `Dispatch()` call it should route to.

## Desired State

### MuxDispatcher (`internal/calibrate/dispatch/mux.go`)

Per-dispatch response routing via a `pending` map of `dispatchID → responseCh`.

- Each `Dispatch()` call gets a unique `DispatchID`, registers a buffered(1) response channel, sends on `promptCh`, and blocks on its own response channel.
- `SubmitArtifact(ctx, dispatchID, data)` looks up the response channel by ID, sends data, and removes the entry.
- Unknown/duplicate dispatch IDs return errors with Orange-level logging.

### ExternalDispatcher interface

Agent-facing surface decoupled from any specific agent runtime:

```go
type ExternalDispatcher interface {
    GetNextStep(ctx context.Context) (DispatchContext, error)
    SubmitArtifact(ctx context.Context, dispatchID int64, data []byte) error
}
```

Works for Cursor MCP, CLI AI, HTTP API, or any future agent.

### Finalizer interface

Replaces type assertions in CursorAdapter:

```go
type Finalizer interface {
    MarkDone(artifactPath string)
}
```

`FileDispatcher` implements `Finalizer`. `CursorAdapter` uses interface check instead of naming concrete types.

## Key Files

| File | Role |
|------|------|
| `internal/calibrate/dispatch/dispatch.go` | `Dispatcher`, `DispatchContext`, `ExternalDispatcher`, `Finalizer` interfaces |
| `internal/calibrate/dispatch/mux.go` | `MuxDispatcher` implementation (replaces `mcp.go`) |
| `internal/calibrate/dispatch/token.go` | `TokenTrackingDispatcher` decorator (unchanged interface) |
| `internal/calibrate/adapt/cursor.go` | `CursorAdapter` — uses `Finalizer` instead of type assertions |
| `internal/mcp/session.go` | Session wires `MuxDispatcher`; `GetNextStep`/`SubmitArtifact` use `ExternalDispatcher` |
| `internal/mcp/server.go` | MCP tool handlers pass `dispatch_id` through `get_next_step`/`submit_artifact` |
