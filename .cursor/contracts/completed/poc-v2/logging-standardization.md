# Contract — logging-standardization

**Status:** complete  
**Goal:** All production log statements use `log/slog` with a per-module `component` attribute and level-gated output. Zero `log.Printf` calls remain in `internal/` or `cmd/`.  
**Serves:** PoC completion (SHOULD)

## Contract rules

- Follow Red-Orange-Green-Yellow-Blue cycle per `rules/testing-methodology.mdc`.
- Every migrated module must compile and pass its tests before moving to the next.
- No behavioral changes — logging migration only. Output content may change format but must preserve equivalent information.

## Context

- **Library decision:** `log/slog` (stdlib). Assessed 6 candidates (slog, zerolog, zap, charmbracelet/log, tint, logrus). slog wins on: zero deps, 8 files already adopted, 9 files to migrate (not 17), stdlib long-term maintenance. Full assessment in plan file `logging_standardization_contract_56426ec9.plan.md`.
- **Current state:** 9 files use `log.Printf` with `[module]` prefix + inline severity. 8 files use `log/slog` without `component` attribute. 1 file uses `fmt.Printf`. No centralized logger setup, no level gating.
- **Module prefix inventory (10 prefixes):**

| Prefix | Package | Current logger |
|---|---|---|
| `[signal-bus]` | `internal/mcp/server.go` | `log` |
| `[mcp-session]` | `internal/mcp/session.go` | `log` |
| `[mcp]` | `internal/mcp/stdin_watch.go`, `cmd/asterisk/cmd_serve.go` | `log` |
| `[calibrate]` | `internal/calibrate/runner.go` | `log` |
| `[parallel]` | `internal/calibrate/parallel.go` | `log` |
| `[analyze]` | `internal/calibrate/analysis.go` | `log` |
| `[rp-source]` | `internal/calibrate/rp_source.go` | `log` |
| `[orchestrate]` | `internal/orchestrate/runner.go` | `log` |
| `[file-dispatch]` | `internal/calibrate/dispatch/file.go` | `fmt` |
| (none) | `dispatch/mux.go`, `dispatch/batch.go`, `adapt/routing.go`, `rp/client.go` | `slog` |

- **Optional follow-up:** Add `lmittmann/tint` as colored stderr handler for `--log-format=dev`. 1-file, 1-dep change. Deferred — not in scope for this contract.

## Execution strategy

1. Create `internal/logging/logging.go` — factory + init. Test it.
2. Migrate `log.Printf` files one-by-one (9 files), building after each.
3. Standardize existing `slog` files with `component` attribute (8 files).
4. Wire `--log-level` and `--log-format` CLI flags.
5. Orange/Yellow alignment pass across all log statements.
6. Validate + tune + validate.

## Tasks

- [ ] Design and create `internal/logging/logging.go` with `New(component)` and `Init(level, format)`.
- [ ] Migrate `internal/mcp/server.go` — replace `log.Printf("[signal-bus] ...")` with slog.
- [ ] Migrate `internal/mcp/session.go` — replace `log.Printf("[mcp-session] ...")` with slog.
- [ ] Migrate `internal/mcp/stdin_watch.go` — replace `log.Printf("[mcp] ...")` with slog.
- [ ] Migrate `cmd/asterisk/cmd_serve.go` — replace `log.Printf("[mcp] ...")` with slog.
- [ ] Migrate `internal/calibrate/runner.go` — replace `log.Printf("[calibrate] ...")` with slog.
- [ ] Migrate `internal/calibrate/parallel.go` — replace `log.Printf("[parallel] ...")` with slog.
- [ ] Migrate `internal/calibrate/analysis.go` — replace `log.Printf("[analyze] ...")` with slog.
- [ ] Migrate `internal/calibrate/rp_source.go` — replace `log.Printf("[rp-source] ...")` with slog.
- [ ] Migrate `internal/orchestrate/runner.go` — replace `log.Printf("[orchestrate] ...")` with slog.
- [ ] Migrate `internal/calibrate/dispatch/file.go` — replace `fmt.Printf("[file-dispatch] ...")` with slog.
- [ ] Standardize `internal/calibrate/dispatch/mux.go` — add `component` via `logging.New("mux-dispatch")`.
- [ ] Standardize `internal/calibrate/dispatch/batch.go` — add `component` via `logging.New("batch-dispatch")`.
- [ ] Standardize `internal/calibrate/adapt/routing.go` — add `component` via `logging.New("routing")`.
- [ ] Standardize `internal/rp/client.go` and `internal/rp/project.go` — add `component` via `logging.New("rp-client")`.
- [ ] Wire `--log-level` and `--log-format` flags in `cmd/asterisk/root.go`, call `logging.Init()` in `PersistentPreRun`.
- [ ] Orange/Yellow alignment pass — review all log statements for correct severity level.
- [ ] Validate (green) — all tests pass, zero `log.Printf` in `internal/` or `cmd/`, acceptance criteria met.
- [ ] Tune (blue) — refactor for quality, review log levels, no behavior changes.
- [ ] Validate (green) — all tests still pass after tuning.

## Acceptance criteria

```gherkin
Given the asterisk binary is built
When I run `asterisk calibrate --scenario ptp-mock --adapter stub --log-level debug --log-format text`
Then all log lines contain a `component=` attribute
And severity is expressed via slog level (not inline text)
And --log-level=warn suppresses Info/Debug lines

Given any Go file in internal/ or cmd/
When I search for `log.Printf` or `log.Println`
Then zero matches are found (all migrated to slog)

Given any slog usage in internal/
When I inspect the logger construction
Then it uses `logging.New("component-name")` with a non-empty component
```

## Notes

2026-02-19 22:30 — Contract created. Library assessment: slog (stdlib) selected over zerolog, zap, charmbracelet/log, tint, logrus. Key factor: 8 files already use slog, migration is 9 files not 17. Zero new deps. Optional tint enhancement deferred.
