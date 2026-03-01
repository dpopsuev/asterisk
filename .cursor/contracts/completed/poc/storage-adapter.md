# Contract — Storage adapter (SQLite + Store facade)

**Status:** complete  
**Goal:** Implement the persistence layer for PoC: a Store facade backed by SQLite, per-workspace DB, so cases and RCAs can be created, listed, and linked.

## Contract rules

- Global rules only. Follow `docs/cli-data-model.mdc` and `notes/pre-dev-decisions.mdc` (storage adapter, per-workspace DB, when to create cases/RCA).
- Access domain and CLI only via the adapter interface; no raw SQL in domain code.

## Context

- **Data model:** One investigation case per failure; cases reference circuit/job; many cases can share one RCA. RCA: title, description, defect_type, evidence; optional jira_ticket_id, jira_link. See `docs/cli-data-model.mdc`.
- **Pre-dev:** Single Store facade with CreateCase, GetCase, ListCasesByCircuit/ByJob, SaveRCA, LinkCaseToRCA, GetRCA, ListRCAs. DB placement: per-workspace (e.g. `./.asterisk/asterisk.db` or `./asterisk.db`). Create cases up front when opening investigation; create RCA row only after agent produces RCA (or "same as case X"). `notes/pre-dev-decisions.mdc`.
- **Current:** Only in-memory mocks (preinvest.MemStore, postinvest.MemPushStore). No SQLite or case/RCA persistence.

## Execution strategy

1. Define Store interface (facade) with the methods above; keep envelope storage separate or extend as needed for "open investigation" (store envelope ref by launch ID).
2. Implement SQLite-backed Store (schema: cases, rcas; case.rca_id nullable).
3. Add placement: per-workspace dir (flag or env for DB path); create dir if needed.
4. Wire into existing flow: e.g. investigate creates case records; postinvest can record to Store instead of only mock push store. Tests can use in-memory impl or temp SQLite DB.
5. Validate with tests; blue refactor.

## Tasks

- [x] **Define Store interface** — CreateCase, GetCase, ListCasesByLaunch/ByJob (or ByCircuit), SaveRCA, LinkCaseToRCA, GetRCA, ListRCAs. Types: Case, RCA (slim keys per pre-dev). Document in code and optionally in `docs/cli-data-model.mdc`.
- [x] **SQLite schema** — Tables for cases and rcas; migrations or single schema version for PoC.
- [x] **Implement SQLite Store** — All facade methods; DB path from config (e.g. `./.asterisk/asterisk.db` or flag).
- [x] **Per-workspace DB** — Resolve DB path (cwd, flag, or env); ensure `.asterisk` (or chosen dir) exists when opening Store.
- [x] **Wire into flow** — Investigation phase creates cases when envelope is loaded (or when "open investigation" runs); push phase can update RCA/link. Keep mock stores for existing tests; add integration test with SQLite Store.
- [x] **Validate (green)** — Tests pass; at least one test uses real SQLite Store (e.g. temp file).
- [x] **Tune (blue)** — Extract interfaces cleanly; no behavior change.
- [x] **Validate (green)** — Tests still pass.

## Acceptance criteria

- **Given** a workspace path (or cwd),
- **When** the Store is opened,
- **Then** a SQLite DB exists at the configured path and the Store facade can create/list cases and save/list/link RCAs.
- **And** domain and CLI code use only the Store interface, not raw SQL.

## Notes

(Running log, newest first. YYYY-MM-DD HH:MM — decision or finding.)

- 2026-02-17 — Completed. Added `internal/store`: Store interface (CreateCase, GetCase, ListCasesByLaunch, SaveRCA, LinkCaseToRCA, GetRCA, ListRCAs, SaveEnvelope, GetEnvelope); Case and RCA types; SqlStore (Open with MkdirAll, schema: cases, rcas, envelopes); MemStore for tests; PreinvestStoreAdapter for preinvest.Store. DefaultDBPath = `.asterisk/asterisk.db`. Integration test with temp SQLite; adapter test. All tests pass.
