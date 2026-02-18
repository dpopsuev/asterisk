# Contract — Mock Pre-investigation (skeleton)

**Status:** complete  
**Goal:** Implement a mock/skeleton for the pre-investigation phase so that “fetch by launch ID” and “save to DB or workspace” are testable with fakes; no real RP calls. BDD/TDD, red–green–blue.

## Contract rules

- Mock only: no live RP API. Use in-memory or file-based fakes.
- Follow execution order: this contract runs **first** (see `contracts/index.mdc` — Mock execution order).
- Glossary: `glossary/glossary.mdc` (Envelope, Launch, Job, Step); `notes/three-phases-manual.mdc` (Pre-investigation).

## Context

- **Pre-investigation flow:** Get launch ID → fetch from RP → store launch data locally (envelope + failure list). `notes/three-phases-manual.mdc`, `notes/ci-notification-and-fetch.mdc`.
- **Execution DB:** RP is source; for this contract we **stub** the fetcher (return envelope from fixture, e.g. `examples/pre-investigation-33195-4.21/envelope_33195_4.21.json`).
- **Asterik workspace:** Launch data stored under a known path or in a mock DB. `docs/examples.mdc`, `notes/pre-investigation-33195-4.21.mdc`.

## Execution strategy

1. **Red** — Write failing BDD/TDD tests for fetch + save (Given launch ID, When fetch, Then envelope stored).
2. **Green** — Minimal implementation: stub fetcher (returns fixture envelope), stub or minimal store (file or in-memory).
3. **Blue** — Refactor for clarity; no behavior change.
4. **Validate** — All tests pass; acceptance criteria met.

## Tasks

- [x] **Red: BDD scenario — fetch and save** — Add scenario: Given a launch ID (e.g. 33195) and a fixture envelope, When pre-investigation fetch runs (mock fetcher returns fixture), Then envelope (and optionally raw launch/items) is stored in the target (DB or workspace path). Write the test; it must **fail** (no impl yet).
- [x] **Red: TDD unit** — Add unit test(s) for: (1) stub fetcher returns envelope from fixture; (2) store persists envelope to target. Tests **fail**.
- [x] **Green: Stub fetcher** — Implement a mock fetcher that reads envelope from fixture (e.g. `examples/pre-investigation-33195-4.21/envelope_33195_4.21.json`) given launch ID; no HTTP.
- [x] **Green: Store envelope** — Implement minimal store: write envelope (and optional launch/items) to a target (in-memory map or file under workspace dir). Make BDD + unit tests **pass**.
- [x] **Blue: Tune** — Extract interfaces (Fetcher, Store) if not already; clarify names; no behavior change.
- [x] **Validate (green)** — All tests pass; acceptance criteria met.
- [x] **Validate (green) after blue** — Re-run tests after refactor; still pass.

## Acceptance criteria (BDD)

- **Given** a launch ID (e.g. 33195) and a mock that returns the example envelope from `examples/pre-investigation-33195-4.21/`,
- **When** pre-investigation “fetch and save” runs,
- **Then** the envelope is stored in the chosen target (DB or workspace directory), and can be read back by launch ID.
- **And** no real RP API is called (mock/stub only).

## Notes

(Running log, newest first. YYYY-MM-DD HH:MM — decision or finding.)

- 2026-02-09 — Executed Red: added `internal/preinvest/` (Fetcher, Store, Envelope, MemStore, FetchAndSave stub); test `TestFetchAndSave_StoresEnvelopeByLaunchID` fails (envelope not stored). Green: implemented `FetchAndSave` to call fetcher.Fetch and store.Save; test passes. Blue and final validate pending.
- 2026-02-09 — Blue: added `doc.go` (package doc + contract link); exported `StubFetcher` and `NewStubFetcher` for reuse; simplified `MemStore.Save` (removed redundant nil check; create via `NewMemStore`); added `TestMemStore_GetUnknownLaunchIDReturnsNil`; doc link in `FetchAndSave`. All tests pass (Validate after blue). Contract 1 complete.
