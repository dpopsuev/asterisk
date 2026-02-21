package metacal

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// RunStore persists and retrieves discovery run reports.
// Implementations must be append-only — never overwrite or delete a run.
type RunStore interface {
	SaveRun(report RunReport) error
	LoadRun(runID string) (RunReport, error)
	ListRuns() ([]string, error)
}

// FileRunStore implements RunStore using one JSON file per run in a directory.
type FileRunStore struct {
	Dir string
}

// NewFileRunStore creates a FileRunStore, ensuring the directory exists.
func NewFileRunStore(dir string) (*FileRunStore, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create store dir: %w", err)
	}
	return &FileRunStore{Dir: dir}, nil
}

// SaveRun writes a run report as pretty-printed JSON. The filename is
// derived from the RunID. Returns an error if a file with that ID
// already exists (append-only guarantee).
func (s *FileRunStore) SaveRun(report RunReport) error {
	path := s.pathFor(report.RunID)

	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("run %q already exists at %s (append-only: refusing to overwrite)", report.RunID, path)
	}

	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal report: %w", err)
	}

	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write report: %w", err)
	}

	return nil
}

// LoadRun reads a run report by ID.
func (s *FileRunStore) LoadRun(runID string) (RunReport, error) {
	data, err := os.ReadFile(s.pathFor(runID))
	if err != nil {
		return RunReport{}, fmt.Errorf("read run %q: %w", runID, err)
	}

	var report RunReport
	if err := json.Unmarshal(data, &report); err != nil {
		return RunReport{}, fmt.Errorf("parse run %q: %w", runID, err)
	}

	return report, nil
}

// ListRuns returns all run IDs in the store, sorted alphabetically.
func (s *FileRunStore) ListRuns() ([]string, error) {
	entries, err := os.ReadDir(s.Dir)
	if err != nil {
		return nil, fmt.Errorf("list runs: %w", err)
	}

	var ids []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		ids = append(ids, strings.TrimSuffix(e.Name(), ".json"))
	}
	sort.Strings(ids)
	return ids, nil
}

func (s *FileRunStore) pathFor(runID string) string {
	return filepath.Join(s.Dir, runID+".json")
}
