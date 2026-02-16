package store

// schemaVersionV1 is the original flat schema.
const schemaVersionV1 = 1

// schemaVersionV2 is the two-tier data model.
const schemaVersionV2 = 2

// schemaV1 is the original flat schema DDL (kept for reference and migration detection).
var schemaV1 = `
CREATE TABLE IF NOT EXISTS schema_version (version INTEGER NOT NULL);
CREATE TABLE IF NOT EXISTS cases (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	launch_id INTEGER NOT NULL,
	item_id INTEGER NOT NULL,
	rca_id INTEGER,
	UNIQUE(launch_id, item_id),
	FOREIGN KEY (rca_id) REFERENCES rcas(id)
);
CREATE TABLE IF NOT EXISTS rcas (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	title TEXT NOT NULL,
	description TEXT NOT NULL,
	defect_type TEXT NOT NULL,
	jira_ticket_id TEXT,
	jira_link TEXT
);
CREATE TABLE IF NOT EXISTS envelopes (
	launch_id INTEGER PRIMARY KEY,
	payload BLOB NOT NULL
);
`

// schemaV2 is the two-tier data model DDL (fresh install).
// Tier 1: Investigation-scoped entities (execution tree).
// Tier 2: Global knowledge entities (institutional memory).
var schemaV2 = `
CREATE TABLE IF NOT EXISTS schema_version (version INTEGER NOT NULL);

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
	name          TEXT NOT NULL DEFAULT '',
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
	description       TEXT NOT NULL DEFAULT '',
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
`

// migrationV1ToV2 contains the SQL statements to migrate from v1 to v2.
// Executed as a transaction in SqlStore.migrate().
var migrationV1ToV2 = `
-- Step 1: Rename v1 tables to _v1_backup
ALTER TABLE cases RENAME TO _v1_cases;
ALTER TABLE rcas RENAME TO _v1_rcas;
ALTER TABLE envelopes RENAME TO _v1_envelopes;

-- Step 2: Create all v2 tables (Tier 1 + Tier 2)
CREATE TABLE investigation_suites (
	id          INTEGER PRIMARY KEY AUTOINCREMENT,
	name        TEXT NOT NULL,
	description TEXT,
	status      TEXT NOT NULL DEFAULT 'open',
	created_at  TEXT NOT NULL,
	closed_at   TEXT
);

CREATE TABLE versions (
	id        INTEGER PRIMARY KEY AUTOINCREMENT,
	label     TEXT NOT NULL UNIQUE,
	ocp_build TEXT
);

CREATE TABLE pipelines (
	id            INTEGER PRIMARY KEY AUTOINCREMENT,
	suite_id      INTEGER NOT NULL REFERENCES investigation_suites(id),
	version_id    INTEGER NOT NULL REFERENCES versions(id),
	name          TEXT NOT NULL,
	rp_launch_id  INTEGER,
	status        TEXT NOT NULL,
	started_at    TEXT,
	ended_at      TEXT
);

CREATE TABLE launches (
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

CREATE TABLE jobs (
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

CREATE TABLE cases (
	id            INTEGER PRIMARY KEY AUTOINCREMENT,
	job_id        INTEGER NOT NULL REFERENCES jobs(id),
	launch_id     INTEGER NOT NULL REFERENCES launches(id),
	rp_item_id    INTEGER NOT NULL,
	name          TEXT NOT NULL DEFAULT '',
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

CREATE TABLE triages (
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

CREATE TABLE symptoms (
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

CREATE TABLE rcas (
	id                INTEGER PRIMARY KEY AUTOINCREMENT,
	title             TEXT NOT NULL,
	description       TEXT NOT NULL DEFAULT '',
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

CREATE TABLE symptom_rca (
	id          INTEGER PRIMARY KEY AUTOINCREMENT,
	symptom_id  INTEGER NOT NULL REFERENCES symptoms(id),
	rca_id      INTEGER NOT NULL REFERENCES rcas(id),
	confidence  REAL,
	notes       TEXT,
	linked_at   TEXT NOT NULL,
	UNIQUE(symptom_id, rca_id)
);

-- Step 3: Create a default migration suite and version for existing data
INSERT INTO investigation_suites(name, description, status, created_at)
	VALUES('Migrated from v1', 'Auto-created during v1→v2 migration', 'open', datetime('now'));

INSERT INTO versions(label)
	VALUES('unknown');

-- Step 4: Migrate envelopes → pipelines + launches
-- For each unique envelope, create one pipeline and one launch.
-- The pipeline references the migration suite (id=1) and unknown version (id=1).
INSERT INTO pipelines(suite_id, version_id, name, rp_launch_id, status)
	SELECT 1, 1, 'migrated-pipeline-' || e.launch_id, e.launch_id, 'UNKNOWN'
	FROM _v1_envelopes e;

INSERT INTO launches(pipeline_id, rp_launch_id, envelope_payload)
	SELECT p.id, p.rp_launch_id, e.payload
	FROM _v1_envelopes e
	JOIN pipelines p ON p.rp_launch_id = e.launch_id;

-- Step 5: Create a placeholder job per launch for migrated cases
INSERT INTO jobs(launch_id, rp_item_id, name)
	SELECT l.id, 0, 'migrated-job'
	FROM launches l;

-- Step 6: Migrate v1 rcas → v2 rcas (add new columns with defaults)
INSERT INTO rcas(title, description, defect_type, jira_ticket_id, jira_link, status, created_at)
	SELECT title, description, defect_type, jira_ticket_id, jira_link, 'open', datetime('now')
	FROM _v1_rcas;

-- Step 7: Migrate v1 cases → v2 cases
-- Map old launch_id (RP ID) to new launches.id via rp_launch_id.
-- Map old rca_id to new rcas.id via rowid ordering (IDs preserved since we INSERT in order).
INSERT INTO cases(job_id, launch_id, rp_item_id, name, status, rca_id, created_at, updated_at)
	SELECT
		j.id,
		l.id,
		oc.item_id,
		'',
		'open',
		CASE WHEN oc.rca_id IS NOT NULL THEN oc.rca_id ELSE NULL END,
		datetime('now'),
		datetime('now')
	FROM _v1_cases oc
	JOIN launches l ON l.rp_launch_id = oc.launch_id
	JOIN jobs j ON j.launch_id = l.id;

-- Step 8: Create indexes
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

-- Step 9: Update schema version
UPDATE schema_version SET version = 2;
`
