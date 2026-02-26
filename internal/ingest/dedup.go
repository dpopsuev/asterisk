package ingest

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// DedupIndex tracks known dedup keys to prevent duplicate ingestion.
// Keys follow the format: {rp_project}:{launch_id}:{test_item_id}
type DedupIndex struct {
	known map[string]bool
}

// NewDedupIndex creates an empty dedup index.
func NewDedupIndex() *DedupIndex {
	return &DedupIndex{known: make(map[string]bool)}
}

// LoadDedupIndex scans a dataset directory and candidate directory for
// existing dedup keys. It reads JSON files looking for "dedup_key" fields.
func LoadDedupIndex(dirs ...string) (*DedupIndex, error) {
	idx := NewDedupIndex()
	for _, dir := range dirs {
		if err := idx.scanDir(dir); err != nil {
			return nil, err
		}
	}
	return idx, nil
}

func (d *DedupIndex) scanDir(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		var doc struct {
			DedupKey string `json:"dedup_key"`
		}
		if json.Unmarshal(data, &doc) == nil && doc.DedupKey != "" {
			d.known[doc.DedupKey] = true
		}
	}
	return nil
}

// Contains returns true if the key is already known.
func (d *DedupIndex) Contains(key string) bool {
	return d.known[key]
}

// Add marks a key as known.
func (d *DedupIndex) Add(key string) {
	d.known[key] = true
}

// Size returns the number of known keys.
func (d *DedupIndex) Size() int {
	return len(d.known)
}

// Filter returns only the matches whose dedup key is NOT in the index.
func (d *DedupIndex) Filter(project string, matches []SymptomMatch) (newCases []SymptomMatch, dupes int) {
	for _, m := range matches {
		key := m.DedupKey(project)
		if d.Contains(key) {
			dupes++
			continue
		}
		d.Add(key)
		newCases = append(newCases, m)
	}
	return
}
