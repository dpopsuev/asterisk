package rca

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dpopsuev/origami/knowledge"
)

func TestLoadAlwaysReadSources_HappyPath(t *testing.T) {
	dir := t.TempDir()
	docPath := filepath.Join(dir, "architecture.md")
	os.WriteFile(docPath, []byte("# PTP Architecture\nlinuxptp-daemon is a pod."), 0644)

	cat := &knowledge.KnowledgeSourceCatalog{
		Sources: []knowledge.Source{
			{
				Name:       "ptp-architecture",
				Kind:       knowledge.SourceKindDoc,
				Purpose:    "Disambiguation doc",
				ReadPolicy: knowledge.ReadAlways,
				LocalPath:  docPath,
			},
		},
	}

	result := loadAlwaysReadSources(cat)
	if len(result) != 1 {
		t.Fatalf("got %d sources, want 1", len(result))
	}
	if result[0].Name != "ptp-architecture" {
		t.Errorf("Name = %q, want %q", result[0].Name, "ptp-architecture")
	}
	if result[0].Purpose != "Disambiguation doc" {
		t.Errorf("Purpose = %q, want %q", result[0].Purpose, "Disambiguation doc")
	}
	if result[0].Content != "# PTP Architecture\nlinuxptp-daemon is a pod." {
		t.Errorf("Content = %q", result[0].Content)
	}
}

func TestLoadAlwaysReadSources_ConditionalOnly(t *testing.T) {
	cat := &knowledge.KnowledgeSourceCatalog{
		Sources: []knowledge.Source{
			{
				Name:       "repo-a",
				Kind:       knowledge.SourceKindRepo,
				ReadPolicy: knowledge.ReadConditional,
			},
		},
	}

	result := loadAlwaysReadSources(cat)
	if result != nil {
		t.Errorf("expected nil for conditional-only catalog, got %d sources", len(result))
	}
}

func TestLoadAlwaysReadSources_MissingLocalPath(t *testing.T) {
	cat := &knowledge.KnowledgeSourceCatalog{
		Sources: []knowledge.Source{
			{
				Name:       "no-path-doc",
				Kind:       knowledge.SourceKindDoc,
				ReadPolicy: knowledge.ReadAlways,
			},
		},
	}

	result := loadAlwaysReadSources(cat)
	if len(result) != 0 {
		t.Errorf("expected 0 sources for missing LocalPath, got %d", len(result))
	}
}

func TestLoadAlwaysReadSources_NonexistentFile(t *testing.T) {
	cat := &knowledge.KnowledgeSourceCatalog{
		Sources: []knowledge.Source{
			{
				Name:       "ghost-doc",
				Kind:       knowledge.SourceKindDoc,
				ReadPolicy: knowledge.ReadAlways,
				LocalPath:  "/tmp/nonexistent-doc-12345.md",
			},
		},
	}

	result := loadAlwaysReadSources(cat)
	if len(result) != 0 {
		t.Errorf("expected 0 sources for nonexistent file, got %d", len(result))
	}
}

func TestLoadAlwaysReadSources_NilCatalog(t *testing.T) {
	result := loadAlwaysReadSources(nil)
	if result != nil {
		t.Errorf("expected nil for nil catalog, got %d sources", len(result))
	}
}
