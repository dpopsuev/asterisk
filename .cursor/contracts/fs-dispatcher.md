# Contract — FS Dispatcher

**Status:** complete (2026-02-17) — Dispatcher interface, StdinDispatcher, FileDispatcher, BatchFileDispatcher all implemented  
**Goal:** Extract the prompt-delivery and artifact-collection transport from `CursorAdapter.SendPrompt` into a `Dispatcher` interface, implement `StdinDispatcher` (current behavior) and `FileDispatcher` (auto-mode via signal file polling), and wire a `--dispatch` CLI flag — without changing any other adapter (Stub, Basic) or breaking existing calibration.

## Contract rules

- Global rules only.
- The Dispatcher is a **transport concern** only. It knows *how* to deliver a prompt and collect an artifact. It does **not** know what the prompt contains, how to score the result, or anything about the pipeline. That remains `ModelAdapter`'s job.
- `CursorAdapter` is the only consumer of `Dispatcher`. Other adapters (Stub, Basic) never dispatch — they compute responses internally.
- The refactor must be **surgical**: extract the 15-line stdin block from `CursorAdapter.SendPrompt`, replace it with `a.dispatcher.Dispatch(...)`, and keep all other logic (template filling, preamble, case registration, store wiring) untouched.
- Every dispatcher must be safe for sequential use (one prompt at a time) but need not be thread-safe.
- Signal files and artifact polling must have configurable timeouts with sane defaults.

## Context

- **Current state:** `CursorAdapter.SendPrompt` (in `internal/calibrate/cursor_adapter.go`) writes a prompt file, prints instructions to stdout, blocks on `bufio.Reader.ReadString('\n')` (stdin), then reads the artifact file. This couples the adapter to interactive terminal use.
- **Problem:** For automated/semi-automated calibration (Cursor agent skill, or Kirsten's harness), we need a non-blocking, file-based protocol so an external agent can discover prompts, produce artifacts, and signal completion without a human pressing Enter.
- **Design decision (from prior discussion):** Use the **Strategy pattern** — `Dispatcher` is the strategy interface, concrete dispatchers are interchangeable strategies. The `CursorAdapter` holds one `Dispatcher` and delegates transport to it. This is the Go-idiomatic approach (interface field, set at construction).
- **Naming:** "Dispatcher" was chosen over Controller (too broad/MVC), Bridge (GoF but not idiomatic Go), and Transport (implies raw bytes, not request-response). Dispatcher is the Go/systems idiom for "send work somewhere, get result back."
- Related repos:
  - `report-portal-cli` (`/home/dpopsuev/Repositories/report-portal-cli/`) — Kirsten's Go CLI for RP. Agent skill + CLI approach; validates our direction of file-based tool exchange over MCP.
  - `testo-resumado-agento` (`/home/dpopsuev/Repositories/testo-resumado-agento/`) — Kirsten's agent harness. A potential future dispatcher consumer (HarnessDispatcher).
- Contracts: `e2e-calibration.md` (calibration framework), `prompt-orchestrator.md` (F0-F6 pipeline).

## Design

### Layer separation

```
┌─────────────────────────────────────────────┐
│            CursorAdapter (logic)            │
│  template fill, preamble, case registry,   │
│  store wiring, prompt param building       │
├─────────────────────────────────────────────┤
│         Dispatcher (transport)              │
│  deliver prompt path, collect artifact path │
└──────────┬──────────────┬───────────────────┘
           │              │
    StdinDispatcher   FileDispatcher
    (interactive)     (auto/polling)
```

### Interface

```go
// Dispatcher abstracts how a prompt is delivered to an external agent
// and how the resulting artifact is collected back.
type Dispatcher interface {
    // Dispatch delivers the prompt at promptPath to the external agent and
    // blocks until the artifact appears at artifactPath.
    // caseID and step are metadata for logging/signal context.
    Dispatch(ctx DispatchContext) ([]byte, error)
}

// DispatchContext carries all the metadata a dispatcher needs.
type DispatchContext struct {
    CaseID       string
    Step         string   // pipeline step name, e.g. "F0-recall"
    PromptPath   string   // absolute path to the filled prompt file
    ArtifactPath string   // absolute path where artifact JSON should appear
}
```

### StdinDispatcher (extract current behavior)

```
1. Print banner with CaseID, Step, PromptPath, ArtifactPath
2. Print instructions: "paste into Cursor, save artifact, press Enter"
3. Block on stdin ReadString('\n')
4. Read artifact file, return bytes
```

This is a **zero-behavior-change extraction** of the existing `CursorAdapter.SendPrompt` transport block.

### FileDispatcher (new, auto mode)

**Protocol:**

```
1. Write signal.json next to prompt file:
   {
     "status": "waiting",
     "case_id": "C1",
     "step": "F0-recall",
     "prompt_path": "/abs/path/to/prompt-recall.md",
     "artifact_path": "/abs/path/to/recall.json",
     "timestamp": "2026-02-16T12:00:00Z"
   }
2. Poll for artifact file every PollInterval (default 500ms)
3. On artifact found:
   a. Validate it is valid JSON
   b. Update signal.json: { "status": "processing" }
   c. Return artifact bytes
4. On timeout (default 10min): return error
5. On done (after caller processes): signal.json updated to { "status": "done" }
```

**Agent side contract (documented, not implemented here):**

An external agent (Cursor skill, harness, script) watches signal.json. When `status: waiting`:
1. Read prompt from `prompt_path`
2. Produce response
3. Write JSON artifact to `artifact_path`
4. (Optional) Update signal.json `status: ready` — but FileDispatcher detects via artifact file existence, so this is optional.

**Configuration:**

```go
type FileDispatcherConfig struct {
    PollInterval time.Duration // default 500ms
    Timeout      time.Duration // default 10min
    SignalDir    string        // directory for signal.json; defaults to artifact dir
}
```

### Future dispatchers (not in scope, documented for extensibility)

| Dispatcher | Transport | When |
|------------|-----------|------|
| HTTPDispatcher | Tiny HTTP server; GET /prompt, POST /artifact | When file polling is too slow or cross-machine |
| MCPDispatcher | Expose as MCP tools; Cursor discovers natively | When MCP server infra is built |
| HarnessDispatcher | Spawn Kirsten's resumu agent as subprocess | When resumu is integrated |

These all implement the same `Dispatcher` interface. No other code changes needed.

### CursorAdapter changes

```go
type CursorAdapter struct {
    // existing fields unchanged
    dispatcher Dispatcher   // NEW: replaces inline stdin logic
}

func NewCursorAdapter(promptDir string, opts ...CursorAdapterOption) *CursorAdapter {
    a := &CursorAdapter{
        promptDir:  promptDir,
        cases:      make(map[string]*cursorCaseCtx),
        dispatcher: NewStdinDispatcher(), // default: interactive
    }
    for _, opt := range opts {
        opt(a)
    }
    return a
}

// CursorAdapterOption configures the CursorAdapter.
type CursorAdapterOption func(*CursorAdapter)

// WithDispatcher sets the transport dispatcher.
func WithDispatcher(d Dispatcher) CursorAdapterOption {
    return func(a *CursorAdapter) { a.dispatcher = d }
}
```

In `SendPrompt`, replace the 15-line stdin block with:

```go
data, err := a.dispatcher.Dispatch(DispatchContext{
    CaseID:       caseID,
    Step:         string(step),
    PromptPath:   promptFile,
    ArtifactPath: artifactFile,
})
```

### CLI integration

```
asterisk calibrate --scenario=ptp-mock --adapter=cursor --dispatch=stdin   (default, interactive)
asterisk calibrate --scenario=ptp-mock --adapter=cursor --dispatch=file    (auto, polling)
```

The `--dispatch` flag is only meaningful when `--adapter=cursor`. For other adapters it is ignored.

## Execution strategy

Implement in order. Each step must pass tests before proceeding.

1. Define the `Dispatcher` interface + `DispatchContext` type.
2. Extract `StdinDispatcher` from current `CursorAdapter.SendPrompt` — verify zero behavior change.
3. Implement `FileDispatcher` with signal file protocol + polling.
4. Refactor `CursorAdapter` to accept a `Dispatcher` via functional option.
5. Wire `--dispatch` flag in CLI.
6. Write tests for `FileDispatcher` (signal file lifecycle, timeout, invalid JSON).
7. Document agent-side contract (how an external agent interacts with signal.json).

## Tasks

- [ ] **Dispatcher interface** — Define `Dispatcher`, `DispatchContext` in `internal/calibrate/dispatcher.go`. Minimal: one method, one context struct.
- [ ] **StdinDispatcher** — Extract current stdin logic from `CursorAdapter.SendPrompt` into `StdinDispatcher.Dispatch()`. Must produce identical stdout output and stdin blocking.
- [ ] **FileDispatcher** — Implement `FileDispatcher` with signal.json write, artifact polling, timeout, and JSON validation. Configurable poll interval and timeout via `FileDispatcherConfig`.
- [ ] **CursorAdapter refactor** — Add `dispatcher` field, `CursorAdapterOption` type, `WithDispatcher` option. Replace inline transport in `SendPrompt` with `a.dispatcher.Dispatch(...)`. Default to `StdinDispatcher`.
- [ ] **CLI wiring** — Add `--dispatch` flag to `calibrate` subcommand. Map `stdin` → `StdinDispatcher`, `file` → `FileDispatcher`. Only active when `--adapter=cursor`.
- [ ] **FileDispatcher tests** — Unit tests: happy path (artifact appears), timeout (no artifact), invalid JSON (artifact exists but not valid JSON), signal file lifecycle (waiting → processing → done).
- [ ] **Agent-side documentation** — Document the signal file protocol in `.cursor/notes/dispatcher-protocol.mdc`: signal.json schema, polling contract, example agent-side watcher script.
- [ ] Validate (green) — all tests pass, `calibrate --adapter=cursor --dispatch=stdin` works identically to current behavior, `calibrate --adapter=cursor --dispatch=file` creates signal.json and polls.
- [ ] Tune (blue) — refactor for quality. No behavior changes.
- [ ] Validate (green) — all tests still pass after tuning.

## Acceptance criteria

- **Given** the existing `CursorAdapter` with `--dispatch=stdin` (default),
- **When** `asterisk calibrate --scenario=ptp-mock --adapter=cursor` is run,
- **Then** behavior is identical to the current implementation: banner printed, stdin blocks, artifact read from file.

- **Given** the `CursorAdapter` with `--dispatch=file`,
- **When** `asterisk calibrate --scenario=ptp-mock --adapter=cursor --dispatch=file` is run,
- **Then** a `signal.json` file is written with `status: waiting`, the process polls for the artifact file, and when found, reads and returns it.

- **Given** the `FileDispatcher` with a configured timeout of 5 seconds,
- **When** no artifact appears within the timeout,
- **Then** the dispatcher returns an error with a clear message including the expected artifact path.

- **Given** any adapter other than `cursor` (e.g. `stub`, `basic`),
- **When** `--dispatch` is provided,
- **Then** the flag is ignored and the adapter works normally.

- **Given** a future `HTTPDispatcher` or `MCPDispatcher` implementing the `Dispatcher` interface,
- **When** it is passed to `CursorAdapter` via `WithDispatcher`,
- **Then** no changes to `CursorAdapter`, runner, metrics, or any other code are required.

## Notes

(Running log, newest first. Use `YYYY-MM-DD HH:MM` — e.g. `2026-02-16 14:32 — Decision or finding.`)

- 2026-02-16 22:00 — Contract created. FS Dispatcher with Strategy pattern. PoC scope: StdinDispatcher (extract) + FileDispatcher (new). Future: HTTP, MCP, Harness dispatchers. Kirsten's report-portal-cli validates CLI+skill approach over MCP for PoC.
