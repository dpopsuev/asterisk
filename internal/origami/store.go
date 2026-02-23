package origami

import (
	"asterisk/internal/calibrate"
	"github.com/dpopsuev/origami/curate"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// DatasetStore is the Asterisk-specific interface for ground truth persistence.
// It operates on calibrate.Scenario, which is the domain type that Asterisk's
// calibration pipeline consumes.
type DatasetStore interface {
	List(ctx context.Context) ([]string, error)
	Load(ctx context.Context, name string) (*calibrate.Scenario, error)
	Save(ctx context.Context, s *calibrate.Scenario) error
}

// FileStore implements DatasetStore using JSON files in a directory.
// It stores calibrate.Scenario directly for backward compatibility with
// existing datasets, while the curate.FileStore can be used for generic
// curation datasets.
type FileStore struct {
	Dir string
}

func NewFileStore(dir string) *FileStore {
	return &FileStore{Dir: dir}
}

func (fs *FileStore) List(_ context.Context) ([]string, error) {
	entries, err := os.ReadDir(fs.Dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("list datasets: %w", err)
	}

	var names []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		names = append(names, strings.TrimSuffix(e.Name(), ".json"))
	}
	return names, nil
}

func (fs *FileStore) Load(_ context.Context, name string) (*calibrate.Scenario, error) {
	path := filepath.Join(fs.Dir, name+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("load dataset %q: %w", name, err)
	}

	var s calibrate.Scenario
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("parse dataset %q: %w", name, err)
	}
	return &s, nil
}

func (fs *FileStore) Save(_ context.Context, s *calibrate.Scenario) error {
	if err := os.MkdirAll(fs.Dir, 0o755); err != nil {
		return fmt.Errorf("create dataset dir: %w", err)
	}

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal dataset %q: %w", s.Name, err)
	}

	path := filepath.Join(fs.Dir, s.Name+".json")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write dataset %q: %w", s.Name, err)
	}

	return nil
}

// CurationStore returns a generic curate.Store that persists curate.Dataset
// objects. This is the bridge between Asterisk's origami adapter and the
// generic curation layer.
func CurationStore(dir string) (curate.Store, error) {
	return curate.NewFileStore(dir)
}
