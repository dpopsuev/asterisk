package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"asterisk/internal/preinvest"

	_ "modernc.org/sqlite"
)

const schemaVersion = 1

var schema = `
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
	if _, err := s.db.Exec(schema); err != nil {
		return fmt.Errorf("create schema: %w", err)
	}
	var v int
	err := s.db.QueryRow("SELECT version FROM schema_version LIMIT 1").Scan(&v)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("read schema version: %w", err)
	}
	if err == sql.ErrNoRows {
		if _, err := s.db.Exec("INSERT INTO schema_version(version) VALUES(?)", schemaVersion); err != nil {
			return fmt.Errorf("set schema version: %w", err)
		}
	}
	return nil
}

// Close closes the database connection.
func (s *SqlStore) Close() error {
	return s.db.Close()
}

// CreateCase creates a case for the given launch and item (failure) id.
func (s *SqlStore) CreateCase(launchID, itemID int) (int64, error) {
	res, err := s.db.Exec(
		"INSERT INTO cases(launch_id, item_id, rca_id) VALUES(?, ?, NULL)",
		launchID, itemID,
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

// GetCase returns the case by id.
func (s *SqlStore) GetCase(caseID int64) (*Case, error) {
	var c Case
	var rcaID sql.NullInt64
	err := s.db.QueryRow(
		"SELECT id, launch_id, item_id, rca_id FROM cases WHERE id = ?",
		caseID,
	).Scan(&c.ID, &c.LaunchID, &c.ItemID, &rcaID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get case: %w", err)
	}
	if rcaID.Valid {
		c.RCAID = rcaID.Int64
	}
	return &c, nil
}

// ListCasesByLaunch returns all cases for the launch.
func (s *SqlStore) ListCasesByLaunch(launchID int) ([]*Case, error) {
	rows, err := s.db.Query(
		"SELECT id, launch_id, item_id, rca_id FROM cases WHERE launch_id = ? ORDER BY id",
		launchID,
	)
	if err != nil {
		return nil, fmt.Errorf("list cases: %w", err)
	}
	defer rows.Close()
	var list []*Case
	for rows.Next() {
		var c Case
		var rcaID sql.NullInt64
		if err := rows.Scan(&c.ID, &c.LaunchID, &c.ItemID, &rcaID); err != nil {
			return nil, fmt.Errorf("scan case: %w", err)
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

// SaveRCA inserts or updates an RCA and returns its id.
// If rca.ID is 0, insert; otherwise update (and return same id).
func (s *SqlStore) SaveRCA(rca *RCA) (int64, error) {
	if rca == nil {
		return 0, errors.New("rca is nil")
	}
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
		"INSERT INTO rcas(title, description, defect_type, jira_ticket_id, jira_link) VALUES(?, ?, ?, ?, ?)",
		rca.Title, rca.Description, rca.DefectType, rca.JiraTicketID, rca.JiraLink,
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

// GetRCA returns the RCA by id.
func (s *SqlStore) GetRCA(rcaID int64) (*RCA, error) {
	var r RCA
	err := s.db.QueryRow(
		"SELECT id, title, description, defect_type, jira_ticket_id, jira_link FROM rcas WHERE id = ?",
		rcaID,
	).Scan(&r.ID, &r.Title, &r.Description, &r.DefectType, &r.JiraTicketID, &r.JiraLink)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get rca: %w", err)
	}
	return &r, nil
}

// ListRCAs returns all RCAs.
func (s *SqlStore) ListRCAs() ([]*RCA, error) {
	rows, err := s.db.Query(
		"SELECT id, title, description, defect_type, jira_ticket_id, jira_link FROM rcas ORDER BY id",
	)
	if err != nil {
		return nil, fmt.Errorf("list rcas: %w", err)
	}
	defer rows.Close()
	var list []*RCA
	for rows.Next() {
		var r RCA
		if err := rows.Scan(&r.ID, &r.Title, &r.Description, &r.DefectType, &r.JiraTicketID, &r.JiraLink); err != nil {
			return nil, fmt.Errorf("scan rca: %w", err)
		}
		list = append(list, &r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list rcas: %w", err)
	}
	return list, nil
}

// SaveEnvelope stores the envelope by launch ID (JSON blob).
func (s *SqlStore) SaveEnvelope(launchID int, env *preinvest.Envelope) error {
	if env == nil {
		return errors.New("envelope is nil")
	}
	payload, err := json.Marshal(env)
	if err != nil {
		return fmt.Errorf("marshal envelope: %w", err)
	}
	_, err = s.db.Exec(
		"INSERT INTO envelopes(launch_id, payload) VALUES(?, ?) ON CONFLICT(launch_id) DO UPDATE SET payload=excluded.payload",
		launchID, payload,
	)
	if err != nil {
		return fmt.Errorf("save envelope: %w", err)
	}
	return nil
}

// GetEnvelope returns the envelope for the launch ID, or nil if not found.
func (s *SqlStore) GetEnvelope(launchID int) (*preinvest.Envelope, error) {
	var payload []byte
	err := s.db.QueryRow("SELECT payload FROM envelopes WHERE launch_id = ?", launchID).Scan(&payload)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get envelope: %w", err)
	}
	var env preinvest.Envelope
	if err := json.Unmarshal(payload, &env); err != nil {
		return nil, fmt.Errorf("unmarshal envelope: %w", err)
	}
	return &env, nil
}
