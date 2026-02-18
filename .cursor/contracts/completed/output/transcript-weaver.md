# Contract — Transcript Weaver

**Status:** complete  
**Goal:** Post-hoc weaver that consolidates per-step prompt/response artifacts into one Markdown transcript per Root Cause, usable with all adapters (stub, basic, cursor).

## Contract rules

- Weaver is a pure reader of on-disk artifacts + in-memory `CalibrationReport`; zero changes to the pipeline loop or adapter interface.
- Gate: `--transcript` CLI flag (opt-in). When off, no weaving occurs. No filesystem detection heuristics.
- Works with stubs (decoupled from AI agent); testable in integration tests.
- Prompt content is optional enrichment: included when `prompt-{family}.md` exists on disk (CursorAdapter), omitted otherwise. Transcript still has step + response + heuristic decision without prompt.
- One transcript file per Root Cause (grouped by `ActualRCAID`). Cases with `ActualRCAID == 0` get standalone transcripts.
- Reverse chronological order: conclusion (F6) first, evidence (F0) last.

## Context

- Pipeline artifacts: `state.json`, `prompt-{family}.md`, `{artifact}.json` per case in `{basePath}/{suiteID}/{caseID}/`.
- `orchestrate.ArtifactFilename(step)` and `orchestrate.PromptFilename(step, 0)` provide filename conventions.
- `CaseResult.ActualRCAID` provides RCA grouping.
- `display` package provides human-readable names for steps and defect types.
- `format` package provides table rendering (Markdown mode).

## Execution strategy

1. Expose `SuiteID` from the calibration run so the weaver can locate case directories.
2. Create transcript types and the weaver function in `internal/calibrate/transcript.go`.
3. Add `--transcript` flag to CLI; call weaver after calibration completes.
4. Write integration tests using the `StubAdapter` to verify weaving without AI.
5. Validate (green) — all tests pass.
6. Tune (blue) — refine Markdown output quality.

## Tasks

- [x] **Expose SuiteID** — Add `SuiteID int64` and `BasePath string` to `CalibrationReport`; set in `RunCalibration`. `runSingleCalibration` now returns `([]CaseResult, int64, error)`.
- [x] **Transcript types** — `TranscriptEntry`, `CaseTranscript`, `RCATranscript` structs in `transcript.go`.
- [x] **WeaveTranscripts** — Group `CaseResult`s by `ActualRCAID`, read `state.json` + optional prompt + artifact from disk, build entries in reverse timestamp order.
- [x] **RenderRCATranscript** — Markdown renderer: RCA header table, primary case full dialog (reverse order), correlated cases abbreviated.
- [x] **CLI integration** — `--transcript` flag in `cmd_calibrate.go`; write `rca-transcript-{slug}.md` files to `{basePath}/transcripts/`.
- [x] **Tests** — 9 tests: nil/empty guard, stub adapter integration, group-by-RCA, reverse order, prompt inclusion/omission, slug generation, write-to-disk E2E.
- [x] Validate (green) — all 15 packages pass with `-race`, `go vet` clean.

## Acceptance criteria

- **Given** a calibration run with `--transcript` enabled and any adapter (stub, basic, cursor),
- **When** the run completes,
- **Then** one Markdown transcript file is produced per distinct Root Cause, containing the full Asterisk-agent dialog in reverse chronological order, with prompt content included when available on disk.

- **Given** a calibration run without `--transcript`,
- **When** the run completes,
- **Then** no transcript files are produced and no weaving logic runs.

## Notes

(Running log, newest first.)

- 2026-02-17 22:42 — All tests pass (9 transcript tests + full suite with -race). Contract complete.
- 2026-02-17 — Contract created. Design: post-hoc weaver, flag-gated, adapter-agnostic, one file per RCA.
