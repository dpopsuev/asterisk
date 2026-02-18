# Contract — Storage adapter v2 (two-tier schema migration)

**Status:** complete  
**Goal:** Migrate the Store interface and SQLite schema from v1 (flat: cases, rcas, envelopes) to v2 (two-tier: investigation-scoped execution tree + global knowledge with symptoms) so the prompt orchestrator can query structured, cross-version data.

## Contract rules

- Global rules only. Follow `docs/cli-data-model.mdc` and `contracts/investigation-context.md`.
- Access domain and CLI only via the adapter interface; no raw SQL in domain code.
- v1 → v2 migration must preserve existing data. Existing cases, rcas, and envelopes are migrated into the new schema.
- All new entity types must have both SqlStore and MemStore implementations.

## Context

- **v1 (current):** `internal/store/store.go` — Store interface with CreateCase, GetCase, ListCasesByLaunch, SaveRCA, LinkCaseToRCA, GetRCA, ListRCAs, SaveEnvelope, GetEnvelope. Types: Case (id, launch_id, item_id, rca_id), RCA (id, title, description, defect_type, jira_ticket_id, jira_link). Schema: 3 tables (cases, rcas, envelopes). `schemaVersion = 1`.
- **v2 (target):** `contracts/investigation-context.md` — 10 tables: investigation_suites, versions, pipelines, launches, jobs, cases (expanded), triages, symptoms, rcas (expanded), symptom_rca. Full DDL in contract §5.
- **Prompt system needs:** F0 Recall needs fingerprint lookup on symptoms + prior RCA retrieval via symptom_rca. F1 Triage needs sibling cases + symptom upsert. F4 Correlate needs cross-case/cross-suite symptom queries. F6 Report needs suite-wide aggregation. See `contracts/prompt-families.md` §9.
- **PoC constraint:** Local CLI, single-agent, SQLite. No concurrency concerns.

## Execution strategy

1. Define new Go types for all v2 entities (alongside existing ones initially).
2. Expand the Store interface with new methods per entity (InvestigationSuite, Version, Pipeline, Launch, Job, expanded Case, Triage, Symptom, expanded RCA, SymptomRCA).
3. Write v2 DDL as a migration from v1 (CREATE new tables, ALTER existing, migrate data).
4. Implement SqlStore v2 methods.
5. Implement MemStore v2 methods (for tests).
6. Write migration logic: v1 data → v2 schema (preserve existing cases, rcas, envelopes).
7. Wire into existing flow: existing commands still work; new entities available for orchestrator.
8. Validate; blue refactor.

## Tasks

### Phase 1 — Types and interface

- [ ] **Define v2 Go types** — InvestigationSuite, Version, Pipeline, Launch (expanded from Envelope), Job, Case (expanded: symptom_id, rca_id, error_message, log_snippet, status, timestamps), Triage, Symptom, RCA (expanded: category, component, affected_versions, status, temporal timestamps), SymptomRCA. In `internal/store/types.go` or expand `store.go`.
- [ ] **Expand Store interface** — Add methods per entity group. Keep existing methods working (backward-compatible). New method groups:
  - Suite: CreateSuite, GetSuite, ListSuites, CloseSuite.
  - Version: CreateVersion, GetVersion, GetVersionByLabel, ListVersions.
  - Pipeline: CreatePipeline, GetPipeline, ListPipelinesBySuite.
  - Launch: CreateLaunch, GetLaunch, GetLaunchByRPID, ListLaunchesByPipeline.
  - Job: CreateJob, GetJob, ListJobsByLaunch.
  - Case (expanded): CreateCaseV2, GetCaseV2, ListCasesByJob, ListCasesByLaunch (expanded), ListCasesBySymptom, UpdateCaseStatus, LinkCaseToSymptom, LinkCaseToRCA.
  - Triage: CreateTriage, GetTriageByCase.
  - Symptom: CreateSymptom, GetSymptom, GetSymptomByFingerprint, UpdateSymptomSeen, ListSymptoms, MarkDormantSymptoms.
  - RCA (expanded): SaveRCAV2, GetRCAV2, ListRCAsByStatus, UpdateRCAStatus.
  - SymptomRCA: LinkSymptomToRCA, GetRCAsForSymptom, GetSymptomsForRCA.

### Phase 2 — Schema and migration

- [ ] **Write v2 DDL** — All 10 tables + indexes per `contracts/investigation-context.md` §5.
- [ ] **Migration logic** — Detect v1 schema (schema_version = 1), run migration:
  1. Create all new tables.
  2. Migrate `rcas` → expanded `rcas` (add columns with defaults; status='open', created_at=now).
  3. Migrate `envelopes` → `launches` (extract rp_launch_id from payload; create default suite + pipeline per launch for continuity).
  4. Migrate `cases` → expanded `cases` (add columns; create placeholder jobs from envelope data; status='open', created_at=now).
  5. Drop or rename old tables (_v1_backup).
  6. Set schema_version = 2.
- [ ] **Fresh install** — If no DB exists, create v2 schema directly (no migration needed).

### Phase 3 — SqlStore implementation

- [ ] **Implement all v2 SqlStore methods** — CRUD for each entity group. Follow existing patterns (parameterized queries, NullInt64 for nullable FKs, json.Marshal for JSON columns).
- [ ] **Symptom fingerprint matching** — GetSymptomByFingerprint returns exact match; UpdateSymptomSeen increments occurrence_count and updates last_seen_at.
- [ ] **Staleness sweep** — MarkDormantSymptoms: `UPDATE symptoms SET status='dormant' WHERE status='active' AND last_seen_at < datetime('now', '-' || ? || ' days')`.

### Phase 4 — MemStore and tests

- [ ] **MemStore v2** — In-memory implementation for all new methods. Used in unit tests and as mock.
- [ ] **Unit tests** — Each new method tested in isolation (MemStore).
- [ ] **Integration tests** — SqlStore with temp DB: create suite → pipeline → launch → job → case → triage; create symptom → link case → create RCA → link symptom to RCA; verify queries (cases by symptom, RCAs for symptom, suite aggregation).
- [ ] **Migration test** — Create v1 DB, populate with sample data, run migration, verify v2 schema and data integrity.

### Phase 5 — Wire and validate

- [ ] **Wire into CLI** — Existing `analyze`, `push`, `cursor`, `save` commands work with v2 store. New entity creation wired where appropriate (e.g. `cursor` creates suite/pipeline if needed).
- [ ] **Validate (green)** — All existing tests pass + new tests pass.
- [ ] **Tune (blue)** — Clean interface, extract helpers, consistent error handling.
- [ ] **Validate (green)** — All tests still pass.

## Acceptance criteria

- **Given** an existing v1 database with cases, rcas, and envelopes,
- **When** the store is opened with the v2 code,
- **Then** the migration runs automatically, existing data is preserved in the new schema, and all v2 query methods work (suite/pipeline/launch/job/case/triage/symptom/rca/symptom_rca CRUD).
- **And** a fresh install creates v2 schema directly.
- **And** all existing CLI commands continue to work.
- **And** symptom fingerprint matching, staleness sweep, and cross-case/cross-suite queries are functional.
- **And** domain and CLI code use only the Store interface, not raw SQL.

## Notes

(Running log, newest first. YYYY-MM-DD HH:MM — decision or finding.)

- 2026-02-17 02:30 — Contract created. v1 baseline: 3 tables (cases, rcas, envelopes), ~10 Store methods, schemaVersion=1. v2 target: 10 tables, ~30+ Store methods, schemaVersion=2. Migration preserves existing data.
