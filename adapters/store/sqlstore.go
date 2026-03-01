package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/dpopsuev/origami/adapters/rp"
	"github.com/dpopsuev/origami/adapters/sqlite"
)

func nowUTC() string { return time.Now().UTC().Format(time.RFC3339) }

func nullStr(ns sql.NullString) string {
	if ns.Valid {
		return ns.String
	}
	return ""
}

func nullFloat(nf sql.NullFloat64) float64 {
	if nf.Valid {
		return nf.Float64
	}
	return 0
}

// SqlStore implements Store with SQLite via the Origami sqlite adapter.
type SqlStore struct {
	db *sqlite.DB
}

// Open opens or creates a SQLite DB at path with YAML-defined schema and migrations.
func Open(path string) (*SqlStore, error) {
	schema, err := LoadSchema()
	if err != nil {
		return nil, fmt.Errorf("load schema: %w", err)
	}
	migrations, err := LoadMigrations()
	if err != nil {
		return nil, fmt.Errorf("load migrations: %w", err)
	}

	db, err := sqlite.Open(path, schema)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	if err := db.Migrate(migrations); err != nil {
		db.Close()
		return nil, fmt.Errorf("run migrations: %w", err)
	}

	return &SqlStore{db: db}, nil
}

// OpenMemory opens an in-memory SQLite DB for testing.
func OpenMemory() (*SqlStore, error) {
	schema, err := LoadSchema()
	if err != nil {
		return nil, fmt.Errorf("load schema: %w", err)
	}
	db, err := sqlite.OpenMemory(schema)
	if err != nil {
		return nil, fmt.Errorf("open memory sqlite: %w", err)
	}
	return &SqlStore{db: db}, nil
}

func (s *SqlStore) Close() error {
	return s.db.Close()
}

// RawDB returns the underlying *sqlite.DB for direct access.
func (s *SqlStore) RawDB() *sqlite.DB {
	return s.db
}

func (s *SqlStore) getCase(caseID int64) (*Case, error) {
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

func (s *SqlStore) LinkCaseToRCA(caseID, rcaID int64) error {
	_, err := s.db.Exec("UPDATE cases SET rca_id = ? WHERE id = ?", rcaID, caseID)
	if err != nil {
		return fmt.Errorf("link case to rca: %w", err)
	}
	return nil
}

func (s *SqlStore) getRCA(rcaID int64) (*RCA, error) {
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

func (s *SqlStore) SaveEnvelope(launchID int, env *rp.Envelope) error {
	if env == nil {
		return errors.New("envelope is nil")
	}
	payload, err := json.Marshal(env)
	if err != nil {
		return fmt.Errorf("marshal envelope: %w", err)
	}

	var existingID int64
	err = s.db.QueryRow(
		"SELECT id FROM launches WHERE rp_launch_id = ? LIMIT 1", launchID,
	).Scan(&existingID)
	if err == nil {
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

	now := nowUTC()

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

	res, err := s.db.Exec(
		"INSERT INTO circuits(suite_id, version_id, name, rp_launch_id, status) VALUES(?, ?, ?, ?, 'UNKNOWN')",
		suiteID, versionID, fmt.Sprintf("auto-circuit-%d", launchID), launchID,
	)
	if err != nil {
		return fmt.Errorf("create circuit: %w", err)
	}
	circuitID, _ := res.LastInsertId()

	res, err = s.db.Exec(
		`INSERT INTO launches(circuit_id, rp_launch_id, name, envelope_payload)
		 VALUES(?, ?, ?, ?)`,
		circuitID, launchID, env.Name, payload,
	)
	if err != nil {
		return fmt.Errorf("create launch: %w", err)
	}
	dbLaunchID, _ := res.LastInsertId()

	_, err = s.db.Exec(
		"INSERT INTO jobs(launch_id, rp_item_id, name) VALUES(?, 0, 'default-job')",
		dbLaunchID,
	)
	if err != nil {
		return fmt.Errorf("create default job: %w", err)
	}

	return nil
}

func (s *SqlStore) GetEnvelope(launchID int) (*rp.Envelope, error) {
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
	var env rp.Envelope
	if err := json.Unmarshal(payload, &env); err != nil {
		return nil, fmt.Errorf("unmarshal envelope: %w", err)
	}
	return &env, nil
}
