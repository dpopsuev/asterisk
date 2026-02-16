# Contract — Mock Investigation (skeleton)

**Status:** complete  
**Goal:** Implement a mock/skeleton for the investigation phase so that “analyze” reads envelope from DB/workspace, creates cases (one per failure), and produces an artifact; all with mocks. BDD/TDD, red–green–blue.

## Contract rules

- Mock only: envelope source is from store (filled by pre-investigation mock); no real Cursor/AI. Artifact is a real file with a defined shape.
- Depends on: **Mock Pre-investigation** (envelope available in store). Run after `mock-pre-investigation.md`.
- Glossary: `docs/cli-data-model.mdc` (case, RCA, storage adapter); `glossary/glossary.mdc` (Investigation case).

## Context

- **Investigation flow:** Get failure list from envelope → one case per failure → RCA (stub) → artifact on FS. `notes/three-phases-manual.mdc`, `notes/ci-analysis-flow.mdc`.
- **Data model:** One investigation case per failure (Step); cases reference launch/envelope. Storage adapter (interface); for this contract use in-memory or file-based mock.
- **Artifact:** Output of analyze: RCA message, convergence score, suggested defect type, evidence refs. `goals/poc.mdc`.

## Execution strategy

1. **Red** — Write failing BDD/TDD tests: Given envelope in store, When analyze runs, Then artifact exists at given path and contains expected shape (e.g. launch_id, case_id, defect_type).
2. **Green** — Minimal implementation: read envelope from mock store; create one case per failure (mock); write artifact file (JSON or similar).
3. **Blue** — Refactor (interfaces, naming); no behavior change.
4. **Validate** — All tests pass; acceptance criteria met.

## Tasks

- [x] **Red: BDD scenario — analyze produces artifact** — Add scenario: Given an envelope in the store (from pre-investigation mock), When analyze runs (no real AI), Then an artifact file is written to the given path and contains at least: launch/run identifier, failure/case identifiers, and placeholder fields (e.g. defect_type, convergence_score). Test **fails**.
- [x] **Red: TDD unit** — Unit test(s): (1) load envelope from store by launch ID; (2) build failure list from envelope; (3) write artifact with required shape. Tests **fail**.
- [x] **Green: Read envelope from store** — Use same store interface as pre-investigation; read envelope by launch ID.
- [x] **Green: Cases and artifact** — For each failure in envelope, create a case record (in-memory or mock DB); write one artifact file (e.g. JSON) with launch_id, case_ids, and stub defect_type/convergence_score. Make tests **pass**.
- [x] **Blue: Tune** — Clear boundaries (Analyzer interface, ArtifactWriter); no behavior change.
- [x] **Validate (green)** — All tests pass; acceptance criteria met.
- [x] **Validate (green) after blue** — Re-run after refactor; still pass.

## Acceptance criteria (BDD)

- **Given** an envelope stored (e.g. by mock pre-investigation) for a launch ID,
- **When** investigation “analyze” runs (mock, no real model),
- **Then** an artifact file exists at the specified path,
- **And** the artifact contains: launch/run id, references to cases/failures, and placeholder RCA fields (defect_type, convergence_score, evidence refs).
- **And** one investigation case exists per failure (step) in the envelope.

## Notes

(Running log, newest first. YYYY-MM-DD HH:MM — decision or finding.)

- 2026-02-09 — Implemented in `internal/investigate/`: EnvelopeSource, Artifact, Analyze, CaseIDsFromEnvelope; DefaultAnalyzer interface. Test TestAnalyze_ProducesArtifactWithRequiredShape passes. Contract complete.
