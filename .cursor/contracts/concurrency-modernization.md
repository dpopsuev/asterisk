# Contract: Concurrency Modernization

**Priority:** MEDIUM — technical debt  
**Status:** Complete  
**Depends on:** `calibrate-decomposition.md` (parallel.go stays in calibrate root after decomposition)

## Goal

Replace hand-rolled goroutine pool management with `golang.org/x/sync/errgroup`, optionally replace `FileDispatcher` polling with `fsnotify/fsnotify`, and eliminate remaining mutable globals.

## Current State

### Hand-rolled goroutine pools (`parallel.go`, ~530 LOC)
- `RunTriagePool`: manual semaphore + goroutine + channel pattern
- `RunInvestigationPool`: similar pattern with different concurrency limit
- Error propagation via channels, manual WaitGroup-like logic
- Race condition risk from hand-managed state

### FileDispatcher polling (`dispatcher.go`, ~340 LOC)
- Timer-based polling loop checking for signal file existence
- CPU waste during idle periods
- Complex timeout logic interleaved with poll logic
- Stale detection via file modification time

### Mutable global
- `orchestrate.BasePath`: set from `main.go`, read in multiple functions
- Should become a field on a config struct passed to functions

## Target State

### errgroup-based parallel execution
```go
g, ctx := errgroup.WithContext(ctx)
g.SetLimit(concurrencyLimit)
for _, c := range cases {
    c := c
    g.Go(func() error {
        return runSingleCalibration(ctx, c, adapter, store)
    })
}
if err := g.Wait(); err != nil {
    return fmt.Errorf("calibration failed: %w", err)
}
```

### fsnotify-based file dispatch (optional)
```go
watcher, _ := fsnotify.NewWatcher()
watcher.Add(signalDir)
for {
    select {
    case event := <-watcher.Events:
        if event.Op&fsnotify.Create != 0 {
            handleSignal(event.Name)
        }
    case err := <-watcher.Errors:
        handleError(err)
    case <-ctx.Done():
        return ctx.Err()
    }
}
```

### Config struct (replace global)
```go
type OrchestrateConfig struct {
    BasePath    string
    TemplateDir string
    // ... other settings
}
```

## Tasks

### Phase 1: errgroup migration
1. Verify `golang.org/x/sync` is in `go.mod` (it's transitive via other deps)
2. If not: `go get golang.org/x/sync`
3. Rewrite `RunTriagePool` using `errgroup.SetLimit()`
4. Rewrite `RunInvestigationPool` using `errgroup.SetLimit()`
5. Remove manual semaphore, channel, and WaitGroup code
6. Add `context.Context` propagation for cancellation
7. Run parallel tests — verify same results
8. Measure: target ~200 LOC reduction in `parallel.go`

### Phase 2: fsnotify for FileDispatcher (optional)
1. `go get github.com/fsnotify/fsnotify`
2. Replace polling loop in `FileDispatcher.Dispatch` with `fsnotify.Watcher`
3. Keep timeout logic (deadline-based, not poll-based)
4. Keep stale detection via file mod time
5. Run dispatcher tests — verify same behavior
6. Measure: target ~150 LOC reduction, near-zero CPU idle usage

### Phase 3: Eliminate mutable globals
1. Create `OrchestrateConfig` struct
2. Replace `orchestrate.BasePath` with `config.BasePath`
3. Pass config struct through function signatures
4. Update `cmd/asterisk/` to construct config and pass it
5. Verify no package-level `var` statements remain (except registries)

## Estimated Impact

| Area | Before | After | Reduction |
|------|--------|-------|-----------|
| `parallel.go` | ~530 LOC | ~330 LOC | ~200 (~38%) |
| `dispatcher.go` | ~340 LOC | ~190 LOC | ~150 (~44%) |
| Mutable globals | 1 (`BasePath`) | 0 | Eliminated |
| Race condition risk | Medium | Low | Reduced by errgroup |

## Completion Criteria
- `parallel.go` uses `errgroup.SetLimit()` — no manual goroutine/channel management
- (Optional) `FileDispatcher` uses `fsnotify` — no polling loop
- `orchestrate.BasePath` global eliminated — config struct instead
- All tests pass
- No race conditions: `go test -race ./...` clean

## Cross-Reference
- Architecture: `.cursor/docs/architecture.md`
- Off-the-shelf rule: `.cursor/rules/off-the-shelf.mdc`
- Calibrate decomposition: `.cursor/contracts/calibrate-decomposition.md`
- Cobra CLI: `.cursor/contracts/cobra-cli.md` (also eliminates globals)
