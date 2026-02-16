package workspace

import (
	"path/filepath"
	"runtime"
	"testing"
)

func testdataPath(name string) string {
	_, f, _, _ := runtime.Caller(0)
	dir := filepath.Dir(f)
	return filepath.Join(dir, "testdata", name)
}

func TestLoadFromPath_JSON(t *testing.T) {
	path := testdataPath("workspace.json")
	w, err := LoadFromPath(path)
	if err != nil {
		t.Fatalf("LoadFromPath: %v", err)
	}
	if w == nil || len(w.Repos) != 2 {
		t.Errorf("want 2 repos, got %+v", w)
	}
	if w.Repos[0].Name != "tests" || w.Repos[0].Path != "../../cnf-gotests" {
		t.Errorf("first repo: got %+v", w.Repos[0])
	}
	if w.Repos[1].Purpose != "SUT: lifecycle" {
		t.Errorf("second repo purpose: got %q", w.Repos[1].Purpose)
	}
}

func TestLoadFromPath_YAML(t *testing.T) {
	path := testdataPath("workspace.yaml")
	w, err := LoadFromPath(path)
	if err != nil {
		t.Fatalf("LoadFromPath: %v", err)
	}
	if w == nil || len(w.Repos) != 2 {
		t.Errorf("want 2 repos, got %+v", w)
	}
	if w.Repos[0].Name != "backend" || w.Repos[0].Branch != "main" {
		t.Errorf("first repo: got %+v", w.Repos[0])
	}
}

func TestLoad_DetectJSON(t *testing.T) {
	data := []byte(`{"repos":[{"name":"a","path":"/a"}]}`)
	w, err := Load(data, "")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(w.Repos) != 1 || w.Repos[0].Name != "a" {
		t.Errorf("got %+v", w)
	}
}

func TestLoad_DetectYAML(t *testing.T) {
	data := []byte("repos:\n  - name: x\n    path: /x\n")
	w, err := Load(data, "")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(w.Repos) != 1 || w.Repos[0].Name != "x" {
		t.Errorf("got %+v", w)
	}
}
