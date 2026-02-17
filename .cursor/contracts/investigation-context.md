# Contract — Investigation Context Data Model

**Status:** complete (2026-02-17) — design consumed by storage-adapter-v2  
**Goal:** Replace the flat data model (cases/rcas/envelopes) with a two-tier entity model — investigation-scoped entities forming the execution tree and global cross-version knowledge entities (symptoms, RCAs) with temporal context — so the prompt system can inject structured, time-aware, cross-version knowledge into templates.

## Contract rules

- Global rules only.
- All entities are accessed through the storage adapter interface; no raw SQL in domain or CLI code. See `docs/cli-data-model.mdc`.
- Schema changes must be backward-compatible via migrations (v1 → v2). Existing data preserved.
- Global knowledge entities (Symptom, RCA) must never carry pipeline-specific or suite-specific foreign keys — they exist independently and are linked via junction tables or nullable FKs on the investigation-scoped side.
- Temporal transitions (active → dormant, open → resolved) must be explainable: log the trigger and timestamp.

## Context

- **Current model:** `internal/store/store.go` — flat `cases`, `rcas`, `envelopes` tables. No hierarchy, no versioning, no symptom tracking, no temporal context.
- **Prompt families contract:** `contracts/prompt-families.md` — F0 Recall needs symptom fingerprint matching + RCA status/age. F1 Triage needs symptom occurrence count + staleness. F4 Correlate needs cross-suite symptom matching. F6 Report needs aggregation by RCA across versions.
- **Envelope hierarchy:** `docs/envelope-mental-model.mdc`, `glossary/glossary.mdc` — recursive hierarchy (step → job → launch → envelope of envelope). Domain naming: Step, Job, Launch, Envelope.
- **Existing data model doc:** `docs/cli-data-model.mdc` — documents the flat model; must be updated to reflect the two-tier design.

---

## 0  Core relational model — the three pillars

A failed **Test Case** is like a witness reporting a crime. The **Symptom** is the story the witness tells — the observable pattern (error message, behavior, timing). The **Root Cause** is the actual criminal — the underlying bug, misconfiguration, or infra issue.

```
Test Case ──reports──▶ Symptom ──caused by──▶ Root Cause (RCA)
```

The canonical path is **always through the Symptom**:

- **Same story, different criminal:** Multiple cases report the same symptom (e.g. "PTP sync timeout"), but in version 4.20 the root cause is a firmware bug, while in 4.22 it's an operator config change. Two different RCAs explain the same symptom in different contexts.
- **Different stories, same criminal:** Different cases report different symptoms (e.g. "sync timeout" and "holdover exceeded"), but both are caused by the same operator regression. One RCA explains multiple symptoms.
- **Same story, same criminal (serial killer):** The same symptom appears across 4.20, 4.21, 4.22 — all pointing to the same unfixed bug. One RCA, one symptom, many cases across versions.

### The three tables

| Table | Role | Analogy |
|-------|------|---------|
| **Case** | Failed test instance — the leaf of the execution tree. Reports what it observed. | Witness |
| **Symptom** | Recognized failure pattern — identified by fingerprint. Cross-version, accumulates over time. | The story |
| **RCA** | Root cause analysis — the actual bug/issue. Cross-version, has lifecycle (open → resolved → verified). | The criminal |

### Relationships

- **Case → Symptom** (many-to-one): A case reports one primary symptom (`case.symptom_id`). Many cases can report the same symptom.
- **Symptom ↔ RCA** (many-to-many via `symptom_rca`): Global knowledge graph. One symptom can be caused by different RCAs in different contexts; one RCA can manifest as different symptoms.
- **Case → RCA** (`case.rca_id`): The **verdict** — which specific RCA from the symptom's possible causes was determined for this case during investigation. This is a denormalized convenience; the canonical path is always `Case → Symptom → SymptomRCA → RCA`. The verdict is set after F3 Investigate or F4 Correlate determines the specific cause.

---

## 1  Entity definitions

### Tier 1 — Investigation-scoped entities (the execution tree)

Bound to specific executions. Form the tree the user navigates. Created when a new investigation is opened.

#### InvestigationSuite

Top-level grouping. An analyst opens a suite when starting a regression analysis (e.g. "PTP Feb 2026 regression"). Spans versions.

| Column | Type | Description |
|--------|------|-------------|
| `id` | INTEGER PK | Auto-increment. |
| `name` | TEXT NOT NULL | Human-readable name. |
| `description` | TEXT | Optional longer description. |
| `status` | TEXT NOT NULL | `open` / `closed`. Default `open`. |
| `created_at` | TEXT NOT NULL | ISO 8601 timestamp. |
| `closed_at` | TEXT | When status changed to `closed`. |

#### Version

Product/OCP version. Shared across suites — a global reference table. Multiple suites can reference the same version.

| Column | Type | Description |
|--------|------|-------------|
| `id` | INTEGER PK | Auto-increment. |
| `label` | TEXT NOT NULL UNIQUE | Version label, e.g. `4.21`. |
| `ocp_build` | TEXT | Specific build, e.g. `4.21.2`. |

#### Pipeline

One CI pipeline run for one version. Bound to a suite + version.

| Column | Type | Description |
|--------|------|-------------|
| `id` | INTEGER PK | Auto-increment. |
| `suite_id` | INTEGER NOT NULL | FK → `investigation_suites.id`. |
| `version_id` | INTEGER NOT NULL | FK → `versions.id`. |
| `name` | TEXT NOT NULL | Pipeline name, e.g. `telco-ft-ran-ptp-4.21`. |
| `rp_launch_id` | INTEGER | RP launch ID (for quick reference; denormalized from Launch). |
| `status` | TEXT NOT NULL | Pipeline status, e.g. `FAILED`, `PASSED`. |
| `started_at` | TEXT | ISO 8601. |
| `ended_at` | TEXT | ISO 8601. |

#### Launch

One RP launch. Evolves from the current `envelopes` blob table into a proper entity. 1:1 with Pipeline for now; schema allows N launches per pipeline for future multi-launch pipelines.

| Column | Type | Description |
|--------|------|-------------|
| `id` | INTEGER PK | Auto-increment. |
| `pipeline_id` | INTEGER NOT NULL | FK → `pipelines.id`. |
| `rp_launch_id` | INTEGER NOT NULL | RP launch ID (e.g. 33195). |
| `rp_launch_uuid` | TEXT | RP launch UUID. |
| `name` | TEXT | Launch name from RP. |
| `status` | TEXT | Overall launch status (`FAILED`, `PASSED`, etc.). |
| `started_at` | TEXT | ISO 8601. |
| `ended_at` | TEXT | ISO 8601. |
| `env_attributes` | TEXT | JSON blob of all environment attributes (operator versions, cluster info, etc.). |
| `git_branch` | TEXT | Branch from envelope git metadata (may be null). |
| `git_commit` | TEXT | Commit SHA from envelope git metadata (may be null). |
| `envelope_payload` | BLOB | Full envelope JSON for backward compatibility and raw access. |
| UNIQUE | | `(pipeline_id, rp_launch_id)` |

#### Job

One test execution group within a launch. Maps to RP TEST-level items (e.g. `[T-TSC] RAN PTP tests`).

| Column | Type | Description |
|--------|------|-------------|
| `id` | INTEGER PK | Auto-increment. |
| `launch_id` | INTEGER NOT NULL | FK → `launches.id`. |
| `rp_item_id` | INTEGER NOT NULL | RP item ID for this TEST-level item. |
| `name` | TEXT NOT NULL | Job name, e.g. `[T-TSC] RAN PTP tests`. |
| `clock_type` | TEXT | Extracted from name/attributes, e.g. `T-TSC`. |
| `status` | TEXT | `FAILED`, `PASSED`, etc. |
| `stats_total` | INTEGER | Total test count. |
| `stats_failed` | INTEGER | Failed count. |
| `stats_passed` | INTEGER | Passed count. |
| `stats_skipped` | INTEGER | Skipped count. |
| `started_at` | TEXT | ISO 8601. |
| `ended_at` | TEXT | ISO 8601. |
| UNIQUE | | `(launch_id, rp_item_id)` |

#### Case (the witness)

One failure. Leaf of the execution tree. The unit of agent work. A Case **reports** a Symptom (the story it observed). The canonical path to Root Cause is always through the Symptom: `Case → Symptom → SymptomRCA → RCA`.

| Column | Type | Description |
|--------|------|-------------|
| `id` | INTEGER PK | Auto-increment. |
| `job_id` | INTEGER NOT NULL | FK → `jobs.id`. |
| `launch_id` | INTEGER NOT NULL | FK → `launches.id` (denormalized for queries). |
| `rp_item_id` | INTEGER NOT NULL | RP item ID for this STEP-level item. |
| `name` | TEXT NOT NULL | Full test name from RP. |
| `polarion_id` | TEXT | Polarion test case ID (e.g. `OCP-83297`). |
| `status` | TEXT NOT NULL | Investigation status: `open` / `triaged` / `investigated` / `reviewed` / `closed`. Default `open`. |
| `symptom_id` | INTEGER | FK → `symptoms.id`. **"Reports" link** — which symptom (story) this case observed. Set after F0 Recall match or F1 Triage. Nullable until matched. |
| `rca_id` | INTEGER | FK → `rcas.id`. **Verdict** (denormalized) — which specific RCA was determined for this case. The canonical path is Case → Symptom → SymptomRCA → RCA; this FK caches the verdict for fast queries. Set after F3 Investigate or F4 Correlate. Nullable until resolved. |
| `error_message` | TEXT | Error message from RP item logs. |
| `log_snippet` | TEXT | Truncated log excerpt. |
| `log_truncated` | INTEGER | 1 if log was truncated, 0 otherwise. |
| `started_at` | TEXT | ISO 8601 (failure start time). |
| `ended_at` | TEXT | ISO 8601 (failure end time). |
| `created_at` | TEXT NOT NULL | When the case was created in the DB. |
| `updated_at` | TEXT NOT NULL | Last update. |
| UNIQUE | | `(launch_id, rp_item_id)` |

#### Triage

F1 output per case. One per case. Investigation-scoped but points into global Symptom via the parent Case.

| Column | Type | Description |
|--------|------|-------------|
| `id` | INTEGER PK | Auto-increment. |
| `case_id` | INTEGER NOT NULL UNIQUE | FK → `cases.id`. One triage per case. |
| `symptom_category` | TEXT NOT NULL | Category from F1: `timeout`, `assertion`, `crash`, `infra`, `config`, `flake`, `unknown`. |
| `severity` | TEXT | Severity estimate. |
| `defect_type_hypothesis` | TEXT | Initial defect type guess (e.g. `pb001`). |
| `skip_investigation` | INTEGER | 1 if F1 recommends skipping repo investigation. |
| `clock_skew_suspected` | INTEGER | 1 if clock skew detected. |
| `cascade_suspected` | INTEGER | 1 if BeforeSuite/ordered cascade detected. |
| `candidate_repos` | TEXT | JSON array of repo names ranked by relevance. |
| `data_quality_notes` | TEXT | Notes about data quality issues (truncated logs, missing data, etc.). |
| `created_at` | TEXT NOT NULL | When triage was performed. |

---

### Tier 2 — Global knowledge entities (institutional memory)

Cross-version, cross-suite. Accumulate over time. Power F0 Recall, F1 pattern matching, F4 Correlate, F6 Report.

#### Symptom (the story)

A recognized failure pattern — the observable "story" that test cases report. Identified by a fingerprint (error pattern + test name pattern + component). Cross-version: the same symptom can appear in 4.20, 4.21, 4.22. Multiple cases can tell the same story; the story persists across investigations.

| Column | Type | Description |
|--------|------|-------------|
| `id` | INTEGER PK | Auto-increment. |
| `fingerprint` | TEXT NOT NULL UNIQUE | Normalized hash for matching (deterministic: test_name_pattern + error_pattern + component). |
| `name` | TEXT NOT NULL | Human-readable symptom name. |
| `description` | TEXT | Longer description of the symptom pattern. |
| `error_pattern` | TEXT | Normalized error snippet or regex for matching. |
| `test_name_pattern` | TEXT | Test name pattern (may include wildcards for variants). |
| `component` | TEXT | Associated component (e.g. `ptp-operator`, `linuxptp-daemon`). |
| `severity` | TEXT | Overall severity of this symptom. |
| `first_seen_at` | TEXT NOT NULL | ISO 8601 — when this symptom was first observed. |
| `last_seen_at` | TEXT NOT NULL | ISO 8601 — when this symptom was last observed. Updated on each new match. |
| `occurrence_count` | INTEGER NOT NULL | How many times this symptom has been observed. Incremented on each new match. Default 1. |
| `status` | TEXT NOT NULL | `active` / `dormant` / `resolved`. Default `active`. |

**Staleness rule:** If `last_seen_at` is older than a configurable window (default 90 days), status auto-transitions to `dormant`. Dormant symptoms get lower recall confidence in F0. If a dormant symptom reappears, it transitions back to `active` and the occurrence count increments — this is a strong signal of a regression (fix reverted or backport missing).

**Fingerprint algorithm (PoC):** Deterministic hash of `normalize(test_name_pattern) + normalize(error_pattern) + component`. Normalization: lowercase, collapse whitespace, strip ANSI, strip numeric IDs (item IDs, timestamps). Fuzzy/embedding matching is a post-PoC enhancement.

#### RCA — the criminal

Root cause analysis — the actual bug, misconfiguration, or infra issue that caused the symptoms. Cross-version, cross-suite. Independent of any pipeline or envelope. One criminal can cause many stories; one story can (in different contexts) be caused by different criminals.

| Column | Type | Description |
|--------|------|-------------|
| `id` | INTEGER PK | Auto-increment. |
| `title` | TEXT NOT NULL | Short title. |
| `description` | TEXT NOT NULL | Full RCA description. |
| `defect_type` | TEXT NOT NULL | Defect type code (e.g. `pb001`, `ab001`). |
| `category` | TEXT | High-level category: `product` / `automation` / `system` / `infra` / `config`. |
| `component` | TEXT | Primary component (e.g. `linuxptp-daemon`). |
| `affected_versions` | TEXT | JSON array of version labels (e.g. `["4.20","4.21","4.22"]`). |
| `evidence_refs` | TEXT | JSON array of evidence references (paths, links, commit SHAs). |
| `convergence_score` | REAL | Confidence score 0–1 from the last investigation. |
| `jira_ticket_id` | TEXT | Jira ticket ID (e.g. `PROJ-123`). |
| `jira_link` | TEXT | Jira ticket URL. |
| `status` | TEXT NOT NULL | `open` / `resolved` / `verified` / `archived`. Default `open`. |
| `created_at` | TEXT NOT NULL | ISO 8601. |
| `resolved_at` | TEXT | When a fix was identified/merged. |
| `verified_at` | TEXT | When the fix was confirmed in CI. |
| `archived_at` | TEXT | When the RCA became irrelevant (version EOL, etc.). |

**Temporal context:**
- `open` — Under investigation or fix not yet identified.
- `resolved` — Fix identified/merged but not yet confirmed in CI. `resolved_at` set.
- `verified` — Fix confirmed passing in CI. `verified_at` set.
- `archived` — No longer relevant (old version EOL'd, obsolete). `archived_at` set.

F0 Recall uses status: a `resolved`+`verified` RCA from 3 months ago with the same symptom in a new version suggests a **backport regression** (fix didn't propagate to the new version).

#### SymptomRCA (junction table — story ↔ criminal)

Links symptoms to RCAs. Many-to-many: one story can point to different criminals in different contexts (same symptom, different root cause in different versions); one criminal can produce multiple stories (one RCA explains different symptoms).

| Column | Type | Description |
|--------|------|-------------|
| `id` | INTEGER PK | Auto-increment. |
| `symptom_id` | INTEGER NOT NULL | FK → `symptoms.id`. |
| `rca_id` | INTEGER NOT NULL | FK → `rcas.id`. |
| `confidence` | REAL | Confidence that this RCA explains this symptom (0–1). |
| `notes` | TEXT | Human or model notes about the link. |
| `linked_at` | TEXT NOT NULL | ISO 8601 — when the link was established. |
| UNIQUE | | `(symptom_id, rca_id)` |

---

## 2  Relationships

### The canonical chain

```
Case ──reports──▶ Symptom ──caused by──▶ RCA
 (witness)         (story)    (via SymptomRCA)   (criminal)
```

`Case.rca_id` is a **denormalized verdict shortcut**. The source of truth for "why did this case fail?" is always the chain: `Case → Symptom → SymptomRCA → RCA`. The verdict caches the specific RCA chosen from the symptom's possible causes for fast per-case queries.

### Entity relationship diagram

```
InvestigationSuite  1──*  Pipeline  *──1  Version
Pipeline            1──*  Launch
Launch              1──*  Job
Job                 1──*  Case (witness)
Case                1──0..1  Triage
Case                *──reports──0..1  Symptom (story)     [canonical link into global knowledge]
Case                *--.verdict.--0..1  RCA (criminal)    [denormalized; derived from symptom chain]
Symptom             *──caused by──*  RCA                  [via SymptomRCA junction — the knowledge graph]
```

### Cross-tier links

- **Case → Symptom** (`symptom_id`, nullable FK): The **"reports"** link — which story this witness told. Set when F0 Recall matches a fingerprint or F1 Triage identifies a pattern. This is the primary bridge from the investigation tree into global knowledge.
- **Symptom ↔ RCA** (via `symptom_rca` junction): The **"caused by"** link — the global knowledge graph. "This story has been explained by these criminals in the past." Many-to-many: same story can point to different criminals in different contexts; same criminal can produce different stories.
- **Case → RCA** (`rca_id`, nullable FK): The **verdict** — denormalized convenience. Which specific RCA (from the symptom's possible causes) was determined for this case. Set after F3 Investigate or F4 Correlate. Always consistent with the symptom chain: if `case.rca_id = R`, then `symptom_rca` must contain a row linking `case.symptom_id` to `R`.

### Navigational queries (what the UI/CLI/prompts need)

| Query | Path | Used by |
|-------|------|---------|
| All failures in a pipeline | Pipeline → Launch → Job → Case | CLI list, F6 Report |
| All failures for a version | Version → Pipeline → Launch → Job → Case | Cross-version report |
| All cases with the same symptom | Symptom → Case (reverse FK) | F4 Correlate |
| Prior RCAs for a symptom | Symptom → SymptomRCA → RCA | F0 Recall |
| All versions affected by an RCA | RCA.affected_versions JSON + Case → Launch → Pipeline → Version | F6 Report |
| Sibling cases in same job | Case WHERE job_id = X | F1 Triage (cascade detection) |
| Open cases in suite | Suite → Pipeline → Launch → Job → Case WHERE status != 'closed' | Suite dashboard |

---

## 3  Temporal rules

### Symptom staleness

| Condition | Transition | Effect on F0 Recall |
|-----------|-----------|---------------------|
| `last_seen_at` > 90 days ago, status = `active` | → `dormant` | Match confidence reduced by 50%. |
| Dormant symptom matched again | → `active`, `last_seen_at` updated, `occurrence_count` incremented | Strong regression signal — a fixed symptom returned. F0 flags this. |
| Symptom explicitly resolved by human | → `resolved` | No automatic recall match; only shown in history. |

### RCA lifecycle

| Transition | Trigger | Timestamp set |
|-----------|---------|---------------|
| `open` → `resolved` | Fix identified/merged (human or agent sets this) | `resolved_at` |
| `resolved` → `verified` | Fix confirmed passing in CI (human or automated check) | `verified_at` |
| `verified` → `archived` | Version EOL or RCA no longer relevant | `archived_at` |
| `resolved`/`verified` → `open` (reopen) | Same symptom reappears in a new version (backport regression) | `resolved_at` / `verified_at` cleared |

### Case lifecycle

```
open → triaged → investigated → reviewed → closed
         ↑           ↑             |
         └───────────└─────────────┘ (reassess loops)
```

- `open`: Case created from envelope failure list.
- `triaged`: F1 Triage completed; symptom_category set.
- `investigated`: F3 Investigate completed; RCA linked.
- `reviewed`: F5 Review completed; human approved/overturned.
- `closed`: Final state. Case fully resolved.
- Reassess: Review → triaged or investigated (loop back).

---

## 4  Prompt injection from this model

The data model feeds the template parameter groups defined in `docs/prompts.mdc`.

### History.SymptomInfo (new injection group)

Injected when a Case has a matched Symptom. Gives the model cross-version knowledge.

| Field | Type | Source | Example |
|-------|------|--------|---------|
| `SymptomName` | string | `symptoms.name` | `ptp4l fails to acquire lock within holdover timeout` |
| `OccurrenceCount` | int | `symptoms.occurrence_count` | `5` |
| `FirstSeen` | string | `symptoms.first_seen_at` | `2026-01-15T10:00:00Z` |
| `LastSeen` | string | `symptoms.last_seen_at` | `2026-02-15T00:32:22Z` |
| `Status` | string | `symptoms.status` | `active` |
| `AffectedVersions` | []string | Derived: SELECT DISTINCT v.label FROM cases c JOIN launches l ... WHERE c.symptom_id = ? | `["4.20","4.21","4.22"]` |
| `LinkedRCAs` | []struct | From SymptomRCA → RCA | `[{title, defect_type, status, jira_link}]` |
| `IsDormantReactivation` | bool | Was this symptom dormant before this match? | `true` (regression signal) |

### History.PriorRCAs (expanded)

Now includes temporal context and cross-version scope.

| Field | Type | Source |
|-------|------|--------|
| `ID` | int | `rcas.id` |
| `Title` | string | `rcas.title` |
| `DefectType` | string | `rcas.defect_type` |
| `Status` | string | `rcas.status` (open/resolved/verified/archived) |
| `AffectedVersions` | []string | `rcas.affected_versions` JSON |
| `JiraLink` | string | `rcas.jira_link` |
| `ResolvedAt` | string | `rcas.resolved_at` |
| `VerifiedAt` | string | `rcas.verified_at` |
| `DaysSinceResolved` | int | Computed: now - resolved_at |

### Temporal query examples for the orchestrator

```sql
-- F0 Recall: find matching symptom by fingerprint
SELECT * FROM symptoms WHERE fingerprint = ? AND status != 'resolved';

-- F0 Recall: get prior RCAs for a matched symptom, excluding archived
SELECT r.* FROM rcas r
  JOIN symptom_rca sr ON sr.rca_id = r.id
  WHERE sr.symptom_id = ? AND r.status != 'archived'
  ORDER BY r.created_at DESC;

-- F0 Recall: backport regression detection
-- (symptom is active/dormant, linked RCA is resolved/verified,
--  but symptom just appeared in a version NOT in affected_versions)
SELECT r.* FROM rcas r
  JOIN symptom_rca sr ON sr.rca_id = r.id
  WHERE sr.symptom_id = ?
    AND r.status IN ('resolved', 'verified')
    AND r.affected_versions NOT LIKE '%' || ? || '%';
-- (? = current version label; crude LIKE; real impl uses JSON functions)

-- F1 Triage: symptom recurrence info
SELECT occurrence_count, first_seen_at, last_seen_at, status
  FROM symptoms WHERE id = ?;

-- F4 Correlate: other open cases with same symptom across suites
SELECT c.*, j.name AS job_name, l.rp_launch_id, p.name AS pipeline_name
  FROM cases c
  JOIN jobs j ON c.job_id = j.id
  JOIN launches l ON c.launch_id = l.id
  JOIN pipelines p ON l.pipeline_id = p.id
  WHERE c.symptom_id = ? AND c.id != ? AND c.status != 'closed';

-- F6 Report: all product bugs in a suite, grouped by RCA
SELECT r.id, r.title, r.defect_type, r.jira_ticket_id, r.jira_link,
       r.affected_versions, COUNT(c.id) AS case_count
  FROM rcas r
  JOIN cases c ON c.rca_id = r.id
  JOIN launches l ON c.launch_id = l.id
  JOIN pipelines p ON l.pipeline_id = p.id
  WHERE p.suite_id = ? AND r.defect_type = 'pb001'
  GROUP BY r.id ORDER BY case_count DESC;

-- Staleness check: mark dormant symptoms
UPDATE symptoms SET status = 'dormant'
  WHERE status = 'active'
    AND last_seen_at < datetime('now', '-90 days');
```

---

## 5  Schema DDL (SQLite, schema version 2)

```sql
-- Tier 1: Investigation-scoped

CREATE TABLE IF NOT EXISTS investigation_suites (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    name        TEXT NOT NULL,
    description TEXT,
    status      TEXT NOT NULL DEFAULT 'open',
    created_at  TEXT NOT NULL,
    closed_at   TEXT
);

CREATE TABLE IF NOT EXISTS versions (
    id        INTEGER PRIMARY KEY AUTOINCREMENT,
    label     TEXT NOT NULL UNIQUE,
    ocp_build TEXT
);

CREATE TABLE IF NOT EXISTS pipelines (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    suite_id      INTEGER NOT NULL REFERENCES investigation_suites(id),
    version_id    INTEGER NOT NULL REFERENCES versions(id),
    name          TEXT NOT NULL,
    rp_launch_id  INTEGER,
    status        TEXT NOT NULL,
    started_at    TEXT,
    ended_at      TEXT
);

CREATE TABLE IF NOT EXISTS launches (
    id                INTEGER PRIMARY KEY AUTOINCREMENT,
    pipeline_id       INTEGER NOT NULL REFERENCES pipelines(id),
    rp_launch_id      INTEGER NOT NULL,
    rp_launch_uuid    TEXT,
    name              TEXT,
    status            TEXT,
    started_at        TEXT,
    ended_at          TEXT,
    env_attributes    TEXT,
    git_branch        TEXT,
    git_commit        TEXT,
    envelope_payload  BLOB,
    UNIQUE(pipeline_id, rp_launch_id)
);

CREATE TABLE IF NOT EXISTS jobs (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    launch_id     INTEGER NOT NULL REFERENCES launches(id),
    rp_item_id    INTEGER NOT NULL,
    name          TEXT NOT NULL,
    clock_type    TEXT,
    status        TEXT,
    stats_total   INTEGER,
    stats_failed  INTEGER,
    stats_passed  INTEGER,
    stats_skipped INTEGER,
    started_at    TEXT,
    ended_at      TEXT,
    UNIQUE(launch_id, rp_item_id)
);

CREATE TABLE IF NOT EXISTS cases (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    job_id        INTEGER NOT NULL REFERENCES jobs(id),
    launch_id     INTEGER NOT NULL REFERENCES launches(id),
    rp_item_id    INTEGER NOT NULL,
    name          TEXT NOT NULL,
    polarion_id   TEXT,
    status        TEXT NOT NULL DEFAULT 'open',
    symptom_id    INTEGER REFERENCES symptoms(id),
    rca_id        INTEGER REFERENCES rcas(id),
    error_message TEXT,
    log_snippet   TEXT,
    log_truncated INTEGER DEFAULT 0,
    started_at    TEXT,
    ended_at      TEXT,
    created_at    TEXT NOT NULL,
    updated_at    TEXT NOT NULL,
    UNIQUE(launch_id, rp_item_id)
);

CREATE TABLE IF NOT EXISTS triages (
    id                      INTEGER PRIMARY KEY AUTOINCREMENT,
    case_id                 INTEGER NOT NULL UNIQUE REFERENCES cases(id),
    symptom_category        TEXT NOT NULL,
    severity                TEXT,
    defect_type_hypothesis  TEXT,
    skip_investigation      INTEGER DEFAULT 0,
    clock_skew_suspected    INTEGER DEFAULT 0,
    cascade_suspected       INTEGER DEFAULT 0,
    candidate_repos         TEXT,
    data_quality_notes      TEXT,
    created_at              TEXT NOT NULL
);

-- Tier 2: Global knowledge

CREATE TABLE IF NOT EXISTS symptoms (
    id                INTEGER PRIMARY KEY AUTOINCREMENT,
    fingerprint       TEXT NOT NULL UNIQUE,
    name              TEXT NOT NULL,
    description       TEXT,
    error_pattern     TEXT,
    test_name_pattern TEXT,
    component         TEXT,
    severity          TEXT,
    first_seen_at     TEXT NOT NULL,
    last_seen_at      TEXT NOT NULL,
    occurrence_count  INTEGER NOT NULL DEFAULT 1,
    status            TEXT NOT NULL DEFAULT 'active'
);

CREATE TABLE IF NOT EXISTS rcas (
    id                INTEGER PRIMARY KEY AUTOINCREMENT,
    title             TEXT NOT NULL,
    description       TEXT NOT NULL,
    defect_type       TEXT NOT NULL,
    category          TEXT,
    component         TEXT,
    affected_versions TEXT,
    evidence_refs     TEXT,
    convergence_score REAL,
    jira_ticket_id    TEXT,
    jira_link         TEXT,
    status            TEXT NOT NULL DEFAULT 'open',
    created_at        TEXT NOT NULL,
    resolved_at       TEXT,
    verified_at       TEXT,
    archived_at       TEXT
);

CREATE TABLE IF NOT EXISTS symptom_rca (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    symptom_id  INTEGER NOT NULL REFERENCES symptoms(id),
    rca_id      INTEGER NOT NULL REFERENCES rcas(id),
    confidence  REAL,
    notes       TEXT,
    linked_at   TEXT NOT NULL,
    UNIQUE(symptom_id, rca_id)
);

-- Indexes for common queries
CREATE INDEX IF NOT EXISTS idx_cases_symptom ON cases(symptom_id);
CREATE INDEX IF NOT EXISTS idx_cases_rca ON cases(rca_id);
CREATE INDEX IF NOT EXISTS idx_cases_launch ON cases(launch_id);
CREATE INDEX IF NOT EXISTS idx_cases_job ON cases(job_id);
CREATE INDEX IF NOT EXISTS idx_pipelines_suite ON pipelines(suite_id);
CREATE INDEX IF NOT EXISTS idx_launches_pipeline ON launches(pipeline_id);
CREATE INDEX IF NOT EXISTS idx_jobs_launch ON jobs(launch_id);
CREATE INDEX IF NOT EXISTS idx_symptoms_fingerprint ON symptoms(fingerprint);
CREATE INDEX IF NOT EXISTS idx_symptom_rca_symptom ON symptom_rca(symptom_id);
CREATE INDEX IF NOT EXISTS idx_symptom_rca_rca ON symptom_rca(rca_id);
```

---

## 6  Migration from v1 to v2

The existing v1 schema has: `schema_version`, `cases` (id, launch_id, item_id, rca_id), `rcas` (id, title, description, defect_type, jira_ticket_id, jira_link), `envelopes` (launch_id, payload).

Migration strategy:

1. Create all new tables (Tier 1 + Tier 2).
2. Migrate `rcas` → new `rcas` (add new columns with defaults; preserve existing rows; set `status = 'open'`, `created_at = datetime('now')`).
3. Migrate `envelopes` → new `launches` (extract fields from payload blob where possible; create a default suite and pipeline per launch for continuity).
4. Migrate `cases` → new `cases` (add new columns; create placeholder `jobs` from envelope data; set `status = 'open'`, `created_at = datetime('now')`).
5. Drop old tables after migration (or keep as `_v1_*` backups).
6. Update `schema_version` to 2.

---

## Execution strategy

1. Write this contract (done).
2. Update `docs/cli-data-model.mdc` to reflect the two-tier model.
3. Update `glossary/glossary.mdc` with new terms.
4. Update `contracts/index.mdc` with this contract.
5. Update `contracts/prompt-families.md` to reference the new data model for F0/F1/F4 injection.
6. (Future, when implementing) Update `Store` interface, write migration, implement new entity operations.

## Tasks

- [ ] Write this contract document.
- [ ] Update `docs/cli-data-model.mdc` — rewrite to reflect two-tier model.
- [ ] Update `glossary/glossary.mdc` — add InvestigationSuite, Symptom, SymptomFingerprint, SymptomRCA, temporal status terms.
- [ ] Update `contracts/index.mdc` — add this contract.
- [ ] Update `contracts/prompt-families.md` — reference new data model for F0/F1/F4 injection.
- [ ] Validate — all cross-references consistent; schema DDL valid; prompt injection paths documented.

## Acceptance criteria

- **Given** a multi-version investigation (e.g. PTP across 4.20, 4.21, 4.22),
- **When** the data model is populated with suites, pipelines, launches, jobs, cases, symptoms, and RCAs,
- **Then** the following queries are expressible:
  - All failures for a specific version.
  - All cases linked to the same symptom across versions.
  - Prior RCAs for a symptom, with temporal context (status, age, affected versions).
  - Regression detection: dormant symptom reappearing in a new version.
  - Aggregation: all product bugs in a suite, grouped by RCA, with Jira links and affected versions.
- **And** investigation-scoped entities never leak into global knowledge (no suite_id on Symptom or RCA).
- **And** temporal transitions (staleness, lifecycle) are documented with rules and timestamps.
- **And** the prompt injection mapping (`History.SymptomInfo`, `History.PriorRCAs`) is fully specified so templates can consume it.

## Notes

(Running log, newest first. YYYY-MM-DD HH:MM — decision or finding.)

- 2026-02-17 02:00 — Clarified the three-pillar relational model with witness/story/criminal analogy. Case (witness) reports Symptom (story), Symptom is caused by RCA (criminal). Canonical path is always through the Symptom: Case → Symptom → SymptomRCA → RCA. Case.rca_id is documented as a denormalized verdict shortcut, not a primary relationship. Updated entity headers, relationship section, and diagrams.
- 2026-02-17 01:00 — Initial contract. Two-tier model designed: 7 investigation-scoped entities (suite, version, pipeline, launch, job, case, triage) + 3 global knowledge entities (symptom, rca, symptom_rca). Schema DDL for SQLite v2 included. Migration strategy from v1. Temporal rules for symptom staleness (90-day dormancy) and RCA lifecycle (open → resolved → verified → archived). Prompt injection mapping for History.SymptomInfo and expanded History.PriorRCAs.
