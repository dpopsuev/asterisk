package store

import (
	"embed"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/dpopsuev/origami/adapters/sqlite"
)

//go:embed schema.yaml
var schemaData []byte

//go:embed migrations
var migrationsFS embed.FS

// LoadSchema parses the embedded schema.yaml.
func LoadSchema() (*sqlite.Schema, error) {
	return sqlite.ParseSchema(schemaData)
}

// LoadMigrations parses all embedded migration YAML files.
func LoadMigrations() ([]*sqlite.Migration, error) {
	var migrations []*sqlite.Migration
	err := fs.WalkDir(migrationsFS, "migrations", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		ext := filepath.Ext(path)
		if ext != ".yaml" && ext != ".yml" {
			return nil
		}
		data, err := migrationsFS.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read %s: %w", path, err)
		}
		m, err := sqlite.ParseMigration(data)
		if err != nil {
			return fmt.Errorf("parse %s: %w", path, err)
		}
		migrations = append(migrations, m)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("load migrations: %w", err)
	}
	return migrations, nil
}

// schemaV1DDL is the V1 DDL kept only for migration tests.
// V1 databases in the wild use this schema; the v1-to-v2 migration
// (now in migrations/v1-to-v2.yaml) handles the upgrade.
var schemaV1DDL = strings.TrimSpace(`
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
`)
