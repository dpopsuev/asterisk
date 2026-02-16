package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"asterisk/internal/preinvest"

	_ "modernc.org/sqlite"
)

// nowUTC returns the current UTC time as an ISO 8601 string.
func nowUTC() string { return time.Now().UTC().Format(time.RFC3339) }

// nullStr converts a sql.NullString to a plain string (empty if null).
func nullStr(ns sql.NullString) string {
	if ns.Valid {
		return ns.String
	}
	return ""
}

// nullFloat converts a sql.NullFloat64 to a plain float64 (0 if null).
func nullFloat(nf sql.NullFloat64) float64 {
	if nf.Valid {
		return nf.Float64
	}
	return 0
}

// currentSchemaVersion is the target schema version for this build.
const currentSchemaVersion = schemaVersionV2

// SqlStore implements Store with SQLite.
type SqlStore struct {
	db *sql.DB
}

// Open opens or creates a SQLite DB at path and runs migrations.
// Creates the parent directory (e.g. .asterisk) if it does not exist.
func Open(path string) (*SqlStore, error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create store dir: %w", err)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}
	s := &SqlStore{db: db}
	if err := s.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

func (s *SqlStore) migrate() error {
	// Check if schema_version table exists to detect database state.
	var tableCount int
	err := s.db.QueryRow(
		"SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='schema_version'",
	).Scan(&tableCount)
	if err != nil {
		return fmt.Errorf("check schema_version table: %w", err)
	}

	if tableCount == 0 {
		// Fresh database — create v2 schema directly.
		return s.freshInstallV2()
	}

	// Existing database — check version and migrate if needed.
	var v int
	err = s.db.QueryRow("SELECT version FROM schema_version LIMIT 1").Scan(&v)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("read schema version: %w", err)
	}
	if errors.Is(err, sql.ErrNoRows) {
		// schema_version exists but is empty — treat as v1.
		v = schemaVersionV1
		if _, err := s.db.Exec("INSERT INTO schema_version(version) VALUES(?)", v); err != nil {
			return fmt.Errorf("set schema version: %w", err)
		}
	}

	switch v {
	case schemaVersionV2:
		return nil // already at target
	case schemaVersionV1:
		return s.migrateV1ToV2()
	default:
		return fmt.Errorf("unknown schema version %d", v)
	}
}

// freshInstallV2 creates the v2 schema from scratch on an empty database.
func (s *SqlStore) freshInstallV2() error {
	if _, err := s.db.Exec(schemaV2); err != nil {
		return fmt.Errorf("create v2 schema: %w", err)
	}
	if _, err := s.db.Exec("INSERT INTO schema_version(version) VALUES(?)", schemaVersionV2); err != nil {
		return fmt.Errorf("set schema version: %w", err)
	}
	return nil
}

// migrateV1ToV2 migrates an existing v1 database to the v2 schema.
// Runs inside a transaction to ensure atomicity.
func (s *SqlStore) migrateV1ToV2() error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin migration tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Enable foreign keys for the migration.
	if _, err := tx.Exec("PRAGMA foreign_keys = OFF"); err != nil {
		return fmt.Errorf("disable fkeys for migration: %w", err)
	}

	// Execute the migration DDL + data migration.
	if _, err := tx.Exec(migrationV1ToV2); err != nil {
		return fmt.Errorf("v1→v2 migration: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit migration tx: %w", err)
	}
	return nil
}

// Close closes the database connection.
func (s *SqlStore) Close() error {
	return s.db.Close()
}

// CreateCase creates a case for the given RP launch ID and item (failure) ID (v1 style).
// On a v2 schema, this finds or creates the necessary launch/job scaffolding
// to satisfy the foreign key constraints. For full v2 creation, use CreateCaseV2.
func (s *SqlStore) CreateCase(launchID, itemID int) (int64, error) {
	// Resolve the launch.id from rp_launch_id. If not found, the v1 scaffolding
	// hasn't been created yet — return an error directing the caller to use v2 methods.
	var dbLaunchID int64
	err := s.db.QueryRow(
		"SELECT id FROM launches WHERE rp_launch_id = ? LIMIT 1", launchID,
	).Scan(&dbLaunchID)
	if errors.Is(err, sql.ErrNoRows) {
		// Fallback: if no launch exists for this RP ID, we can't create a v2 case
		// without full scaffolding. Return an error.
		return 0, fmt.Errorf("no launch found for rp_launch_id=%d; use v2 methods to create the full hierarchy", launchID)
	}
	if err != nil {
		return 0, fmt.Errorf("resolve launch: %w", err)
	}

	// Find the first job for this launch (placeholder job from migration).
	var jobID int64
	err = s.db.QueryRow(
		"SELECT id FROM jobs WHERE launch_id = ? LIMIT 1", dbLaunchID,
	).Scan(&jobID)
	if err != nil {
		return 0, fmt.Errorf("resolve job: %w", err)
	}

	now := nowUTC()
	res, err := s.db.Exec(
		`INSERT INTO cases(job_id, launch_id, rp_item_id, name, status, rca_id, created_at, updated_at)
		 VALUES(?, ?, ?, '', 'open', NULL, ?, ?)`,
		jobID, dbLaunchID, itemID, now, now,
	)
	if err != nil {
		return 0, fmt.Errorf("insert case: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("last insert id: %w", err)
	}
	return id, nil
}

// GetCase returns the case by id. Returns all v2 fields if the schema is v2.
func (s *SqlStore) GetCase(caseID int64) (*Case, error) {
	var c Case
	var rcaID, symptomID, jobID, logTrunc sql.NullInt64
	var polarionID, errMsg, logSnip, startedAt, endedAt sql.NullString
	err := s.db.QueryRow(
		`SELECT id, job_id, launch_id, rp_item_id, name, polarion_id, status,
		        symptom_id, rca_id, error_message, log_snippet, log_truncated,
		        started_at, ended_at, created_at, updated_at
		 FROM cases WHERE id = ?`,
		caseID,
	).Scan(&c.ID, &jobID, &c.LaunchID, &c.RPItemID,
		&c.Name, &polarionID, &c.Status,
		&symptomID, &rcaID, &errMsg, &logSnip, &logTrunc,
		&startedAt, &endedAt, &c.CreatedAt, &c.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get case: %w", err)
	}
	c.JobID = jobID.Int64
	c.RCAID = rcaID.Int64
	c.SymptomID = symptomID.Int64
	c.PolarionID = nullStr(polarionID)
	c.ErrorMessage = nullStr(errMsg)
	c.LogSnippet = nullStr(logSnip)
	c.StartedAt = nullStr(startedAt)
	c.EndedAt = nullStr(endedAt)
	c.LogTruncated = logTrunc.Valid && logTrunc.Int64 == 1
	return &c, nil
}

// ListCasesByLaunch returns all cases for the RP launch ID (v1 key).
// Resolves via launches.rp_launch_id → cases.launch_id.
func (s *SqlStore) ListCasesByLaunch(launchID int) ([]*Case, error) {
	rows, err := s.db.Query(
		`SELECT c.id, c.job_id, c.launch_id, c.rp_item_id, c.name, c.status, c.rca_id
		 FROM cases c
		 JOIN launches l ON c.launch_id = l.id
		 WHERE l.rp_launch_id = ?
		 ORDER BY c.id`,
		launchID,
	)
	if err != nil {
		return nil, fmt.Errorf("list cases: %w", err)
	}
	defer rows.Close()
	var list []*Case
	for rows.Next() {
		var c Case
		var rcaID, jobID sql.NullInt64
		if err := rows.Scan(&c.ID, &jobID, &c.LaunchID, &c.RPItemID, &c.Name, &c.Status, &rcaID); err != nil {
			return nil, fmt.Errorf("scan case: %w", err)
		}
		if jobID.Valid {
			c.JobID = jobID.Int64
		}
		if rcaID.Valid {
			c.RCAID = rcaID.Int64
		}
		list = append(list, &c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list cases: %w", err)
	}
	return list, nil
}

// SaveRCA inserts or updates an RCA and returns its id (v1 style — basic fields).
// If rca.ID is 0, insert; otherwise update (and return same id).
func (s *SqlStore) SaveRCA(rca *RCA) (int64, error) {
	if rca == nil {
		return 0, errors.New("rca is nil")
	}
	now := nowUTC()
	if rca.ID != 0 {
		_, err := s.db.Exec(
			"UPDATE rcas SET title=?, description=?, defect_type=?, jira_ticket_id=?, jira_link=? WHERE id=?",
			rca.Title, rca.Description, rca.DefectType, rca.JiraTicketID, rca.JiraLink, rca.ID,
		)
		if err != nil {
			return 0, fmt.Errorf("update rca: %w", err)
		}
		return rca.ID, nil
	}
	res, err := s.db.Exec(
		`INSERT INTO rcas(title, description, defect_type, jira_ticket_id, jira_link, status, created_at)
		 VALUES(?, ?, ?, ?, ?, 'open', ?)`,
		rca.Title, rca.Description, rca.DefectType, rca.JiraTicketID, rca.JiraLink, now,
	)
	if err != nil {
		return 0, fmt.Errorf("insert rca: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("last insert id: %w", err)
	}
	return id, nil
}

// LinkCaseToRCA sets case.rca_id = rcaID.
func (s *SqlStore) LinkCaseToRCA(caseID, rcaID int64) error {
	_, err := s.db.Exec("UPDATE cases SET rca_id = ? WHERE id = ?", rcaID, caseID)
	if err != nil {
		return fmt.Errorf("link case to rca: %w", err)
	}
	return nil
}

// GetRCA returns the RCA by id (all available fields).
func (s *SqlStore) GetRCA(rcaID int64) (*RCA, error) {
	var r RCA
	var cat, comp, affVer, evRefs, jiraID, jiraLink sql.NullString
	var resolvedAt, verifiedAt, archivedAt sql.NullString
	var convScore sql.NullFloat64
	err := s.db.QueryRow(
		`SELECT id, title, description, defect_type, category, component,
		        affected_versions, evidence_refs, convergence_score,
		        jira_ticket_id, jira_link, status, created_at,
		        resolved_at, verified_at, archived_at
		 FROM rcas WHERE id = ?`,
		rcaID,
	).Scan(&r.ID, &r.Title, &r.Description, &r.DefectType,
		&cat, &comp, &affVer, &evRefs, &convScore,
		&jiraID, &jiraLink, &r.Status, &r.CreatedAt,
		&resolvedAt, &verifiedAt, &archivedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get rca: %w", err)
	}
	r.Category = nullStr(cat)
	r.Component = nullStr(comp)
	r.AffectedVersions = nullStr(affVer)
	r.EvidenceRefs = nullStr(evRefs)
	r.ConvergenceScore = nullFloat(convScore)
	r.JiraTicketID = nullStr(jiraID)
	r.JiraLink = nullStr(jiraLink)
	r.ResolvedAt = nullStr(resolvedAt)
	r.VerifiedAt = nullStr(verifiedAt)
	r.ArchivedAt = nullStr(archivedAt)
	return &r, nil
}

// ListRCAs returns all RCAs (all available fields).
func (s *SqlStore) ListRCAs() ([]*RCA, error) {
	rows, err := s.db.Query(
		`SELECT id, title, description, defect_type, category, component,
		        affected_versions, evidence_refs, convergence_score,
		        jira_ticket_id, jira_link, status, created_at,
		        resolved_at, verified_at, archived_at
		 FROM rcas ORDER BY id`,
	)
	if err != nil {
		return nil, fmt.Errorf("list rcas: %w", err)
	}
	defer rows.Close()
	var list []*RCA
	for rows.Next() {
		var r RCA
		var cat, comp, affVer, evRefs, jiraID, jiraLink sql.NullString
		var resolvedAt, verifiedAt, archivedAt sql.NullString
		var convScore sql.NullFloat64
		if err := rows.Scan(&r.ID, &r.Title, &r.Description, &r.DefectType,
			&cat, &comp, &affVer, &evRefs, &convScore,
			&jiraID, &jiraLink, &r.Status, &r.CreatedAt,
			&resolvedAt, &verifiedAt, &archivedAt); err != nil {
			return nil, fmt.Errorf("scan rca: %w", err)
		}
		r.Category = nullStr(cat)
		r.Component = nullStr(comp)
		r.AffectedVersions = nullStr(affVer)
		r.EvidenceRefs = nullStr(evRefs)
		r.ConvergenceScore = nullFloat(convScore)
		r.JiraTicketID = nullStr(jiraID)
		r.JiraLink = nullStr(jiraLink)
		r.ResolvedAt = nullStr(resolvedAt)
		r.VerifiedAt = nullStr(verifiedAt)
		r.ArchivedAt = nullStr(archivedAt)
		list = append(list, &r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list rcas: %w", err)
	}
	return list, nil
}

// SaveEnvelope stores the envelope by RP launch ID. In v2, this creates the
// necessary scaffolding (suite/version/pipeline/launch) if not already present,
// and stores the envelope payload in launches.envelope_payload.
func (s *SqlStore) SaveEnvelope(launchID int, env *preinvest.Envelope) error {
	if env == nil {
		return errors.New("envelope is nil")
	}
	payload, err := json.Marshal(env)
	if err != nil {
		return fmt.Errorf("marshal envelope: %w", err)
	}

	// Check if a launch already exists for this RP launch ID.
	var existingID int64
	err = s.db.QueryRow(
		"SELECT id FROM launches WHERE rp_launch_id = ? LIMIT 1", launchID,
	).Scan(&existingID)
	if err == nil {
		// Update existing launch's envelope payload.
		_, err = s.db.Exec(
			"UPDATE launches SET envelope_payload = ? WHERE id = ?",
			payload, existingID,
		)
		if err != nil {
			return fmt.Errorf("update envelope: %w", err)
		}
		return nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("check existing launch: %w", err)
	}

	// No launch exists — create minimal scaffolding (auto-suite, auto-pipeline).
	now := nowUTC()

	// Ensure a default suite exists.
	var suiteID int64
	err = s.db.QueryRow(
		"SELECT id FROM investigation_suites WHERE name = 'Default Suite' LIMIT 1",
	).Scan(&suiteID)
	if errors.Is(err, sql.ErrNoRows) {
		res, err := s.db.Exec(
			"INSERT INTO investigation_suites(name, description, status, created_at) VALUES(?, ?, 'open', ?)",
			"Default Suite", "Auto-created for v1-style envelope save", now,
		)
		if err != nil {
			return fmt.Errorf("create default suite: %w", err)
		}
		suiteID, _ = res.LastInsertId()
	} else if err != nil {
		return fmt.Errorf("check default suite: %w", err)
	}

	// Ensure a default version exists.
	var versionID int64
	err = s.db.QueryRow("SELECT id FROM versions WHERE label = 'unknown' LIMIT 1").Scan(&versionID)
	if errors.Is(err, sql.ErrNoRows) {
		res, err := s.db.Exec("INSERT INTO versions(label) VALUES('unknown')")
		if err != nil {
			return fmt.Errorf("create unknown version: %w", err)
		}
		versionID, _ = res.LastInsertId()
	} else if err != nil {
		return fmt.Errorf("check unknown version: %w", err)
	}

	// Create pipeline.
	res, err := s.db.Exec(
		"INSERT INTO pipelines(suite_id, version_id, name, rp_launch_id, status) VALUES(?, ?, ?, ?, 'UNKNOWN')",
		suiteID, versionID, fmt.Sprintf("auto-pipeline-%d", launchID), launchID,
	)
	if err != nil {
		return fmt.Errorf("create pipeline: %w", err)
	}
	pipelineID, _ := res.LastInsertId()

	// Create launch with envelope payload.
	res, err = s.db.Exec(
		`INSERT INTO launches(pipeline_id, rp_launch_id, name, envelope_payload)
		 VALUES(?, ?, ?, ?)`,
		pipelineID, launchID, env.Name, payload,
	)
	if err != nil {
		return fmt.Errorf("create launch: %w", err)
	}
	dbLaunchID, _ := res.LastInsertId()

	// Create a placeholder job for the launch (v1 cases need a job_id).
	_, err = s.db.Exec(
		"INSERT INTO jobs(launch_id, rp_item_id, name) VALUES(?, 0, 'default-job')",
		dbLaunchID,
	)
	if err != nil {
		return fmt.Errorf("create default job: %w", err)
	}

	return nil
}

// GetEnvelope returns the envelope for the RP launch ID, or nil if not found.
// In v2, reads from launches.envelope_payload.
func (s *SqlStore) GetEnvelope(launchID int) (*preinvest.Envelope, error) {
	var payload []byte
	err := s.db.QueryRow(
		"SELECT envelope_payload FROM launches WHERE rp_launch_id = ? LIMIT 1",
		launchID,
	).Scan(&payload)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get envelope: %w", err)
	}
	if payload == nil {
		return nil, nil
	}
	var env preinvest.Envelope
	if err := json.Unmarshal(payload, &env); err != nil {
		return nil, fmt.Errorf("unmarshal envelope: %w", err)
	}
	return &env, nil
}
