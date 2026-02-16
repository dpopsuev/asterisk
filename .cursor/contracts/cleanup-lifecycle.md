# Contract — cleanup-lifecycle

**Status:** active  
**Goal:** Wet calibration runs are self-contained: pre-run cleanup, subprocess lifecycle management, and post-run teardown are automatic — no manual intervention between iterations.

## Contract rules

- Global rules only.
- All cleanup is opt-out, not opt-in (safe defaults).
- Subprocess management must handle both normal exit and signals (SIGINT/SIGTERM).

## Context

- Audit: `.asterisk/calibrate/` directory, `signal.json` files, artifact JSON files, and `signal-responder` process are left behind after every wet calibration run.
- Today the Cursor agent manually runs `rm -rf .asterisk/calibrate` and `pkill signal-responder` between runs.
- `writeSignal` uses a write-rename pattern; `.tmp` file may orphan on rename failure.
- Stub adapter already cleans up via `os.MkdirTemp` + `defer os.RemoveAll`.

## Execution strategy

Implement in this order:

1. **Pre-run cleanup** — `--clean` flag (default `true` for `--adapter=cursor --dispatch=file`). Removes `.asterisk/calibrate/` before creating it fresh.
2. **Responder subprocess** — `--responder=auto|external|none`. When `auto`, `asterisk calibrate` builds and spawns `signal-responder` as a child process, pipes its output, and kills it on exit/signal.
3. **writeSignal .tmp fix** — remove orphaned `.tmp` in the fallback path.
4. **Post-run finalization** — walk `.asterisk/calibrate/`, set every `signal.json` to `status: "complete"`, log summary of files/dirs created.
5. **Tests** — unit tests for cleanup, subprocess lifecycle, .tmp fix.

## Tasks

- [ ] Add `--clean` flag to `runCalibrate`; remove `.asterisk/calibrate/` when true before `os.MkdirAll`.
- [ ] Add `--responder` flag (`auto`, `external`, `none`); when `auto`, build + spawn `cmd/signal-responder` as child.
- [ ] Wire subprocess to `defer proc.Kill()` + signal forwarding (SIGINT/SIGTERM).
- [ ] Capture responder stdout/stderr and prefix-print to asterisk output.
- [ ] Fix `writeSignal` to clean up `.tmp` on rename-fail fallback path.
- [ ] Add `FinalizeSignals(dir)` to walk + set all `signal.json` to `"complete"`.
- [ ] Call `FinalizeSignals` in `runCalibrate` after `RunCalibration` returns (success or error).
- [ ] Add unit tests: pre-run cleanup, .tmp orphan fix, signal finalization.
- [ ] Integration test: spawn responder, run calibrate, verify clean shutdown.
- [ ] Validate (green) — all tests pass, acceptance criteria met.
- [ ] Tune (blue) — refactor for quality. No behavior changes.
- [ ] Validate (green) — all tests still pass after tuning.

## Acceptance criteria

- **Given** a dirty `.asterisk/calibrate/` from a previous run, **when** `asterisk calibrate --adapter=cursor --dispatch=file` starts, **then** the directory is removed and recreated fresh (no stale signals or artifacts).
- **Given** `--responder=auto`, **when** calibration starts, **then** `signal-responder` is spawned as a child process and its output is prefixed on asterisk's stdout.
- **Given** a running calibration with `--responder=auto`, **when** calibration completes (or is interrupted via SIGINT), **then** the signal-responder process is killed and waited on.
- **Given** a completed calibration run, **when** the report is printed, **then** all `signal.json` files have `status: "complete"`.
- **Given** `writeSignal` where `os.Rename` fails, **when** the fallback write succeeds, **then** the `.tmp` file is removed.
- **Given** `--clean=false`, **when** calibration starts, **then** the existing `.asterisk/calibrate/` is preserved (append mode).

## Notes

2026-02-16 — Contract created from artifact/process cleanup audit.
