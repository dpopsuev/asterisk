# Contract — Mock Wiring (skeleton)

**Status:** complete  
**Goal:** Wire the three mock phases (pre-investigation, investigation, post-investigation) into one runnable flow: fetch → save → analyze → push, all with mocks. BDD/TDD, red–green–blue.

## Contract rules

- No real RP, Cursor, or Jira. All components use mocks/stubs from the three phase contracts.
- Depends on: **Mock Pre-investigation**, **Mock Investigation**, **Mock Post-investigation** completed. Run **last**.
- Execution order: **1 → 2 → 3 → 4** (pre-inv → investigation → post-inv → wiring). See `contracts/index.mdc`.

## Context

- **Full flow:** Pre-investigation (fetch + save) → Investigation (analyze → artifact) → Post-investigation (push). `notes/three-phases-manual.mdc`, `notes/poc-flow.mdc`.
- **Wiring:** Same interfaces and stores used across phases; one entry point or script runs: fetch(launchID) → analyze(launchID, workspace) → push(artifactPath).

## Execution strategy

1. **Red** — Write failing BDD test: Given launch ID and workspace path, When full flow runs (fetch → analyze → push), Then envelope is stored, artifact is written, and push is recorded (e.g. defect type in mock store).
2. **Green** — Wire the three phases: call pre-investigation fetch/save, then investigate (read from store, write artifact), then push (read artifact, update mock). Use same mock store and artifact shape as in phase contracts. Make test **pass**.
3. **Blue** — Single entry point or CLI skeleton (e.g. `run-mock-flow --launch 33195 --workspace .`); no behavior change.
4. **Validate** — Full flow test passes; acceptance criteria met.

## Tasks

- [x] **Red: BDD scenario — full flow** — Add scenario: Given a launch ID (e.g. 33195), a fixture envelope, and a workspace path, When the full mock flow runs (fetch → save → analyze → push), Then (1) envelope is in store, (2) artifact file exists, (3) mock push store contains defect type (and optional Jira fields) for the case(s). Test **fails**.
- [x] **Green: Wire phases** — Invoke pre-investigation (stub fetcher + store), then investigation (read from store, write artifact), then post-investigation (read artifact, push to mock). Shared store and artifact format. Make test **pass**.
- [x] **Blue: Entry point** — Expose one entry point (function or CLI stub) that accepts launch ID and workspace and runs the three phases in order; no behavior change.
- [x] **Validate (green)** — Full flow test passes; acceptance criteria met.
- [x] **Validate (green) after blue** — Re-run full flow after refactor; still pass.

## Acceptance criteria (BDD)

- **Given** a launch ID, a mock fetcher (returning fixture envelope), and a workspace path,
- **When** the wired mock flow runs (pre-investigation → investigation → post-investigation),
- **Then** the envelope is stored (pre-inv), an artifact is written (investigation), and the push is recorded in the mock (post-inv).
- **And** the flow is runnable in one go (single entry point or script).

## Notes

(Running log, newest first. YYYY-MM-DD HH:MM — decision or finding.)

- 2026-02-09 — Implemented in `internal/wiring/`: Run(fetcher, envelopeStore, launchID, artifactPath, pushStore, jiraTicketID, jiraLink). Test TestRun_FullFlowStoresEnvelopeWritesArtifactRecordsPush passes. Entry point: `cmd/run-mock-flow` (go run ./cmd/run-mock-flow -launch 33195 -artifact <path>). Contract complete.
