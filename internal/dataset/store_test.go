package dataset

import (
	"asterisk/adapters/rca"
	"context"
	"os"
	"testing"
)

var _ DatasetStore = (*FileStore)(nil)

func TestFileStore_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	store := NewFileStore(dir)
	ctx := context.Background()

	scenario := &rca.Scenario{
		Name: "test-scenario",
		Cases: []rca.GroundTruthCase{
			{ID: "C01", TestName: "test_one", ErrorMessage: "fail"},
			{ID: "C02", TestName: "test_two", ErrorMessage: "error"},
		},
		RCAs: []rca.GroundTruthRCA{
			{ID: "R01", DefectType: "product_bug"},
		},
	}

	if err := store.Save(ctx, scenario); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := store.Load(ctx, "test-scenario")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if loaded.Name != scenario.Name {
		t.Errorf("Name = %q, want %q", loaded.Name, scenario.Name)
	}
	if len(loaded.Cases) != 2 {
		t.Errorf("len(Cases) = %d, want 2", len(loaded.Cases))
	}
	if len(loaded.RCAs) != 1 {
		t.Errorf("len(RCAs) = %d, want 1", len(loaded.RCAs))
	}
}

func TestFileStore_List(t *testing.T) {
	dir := t.TempDir()
	store := NewFileStore(dir)
	ctx := context.Background()

	names, err := store.List(ctx)
	if err != nil {
		t.Fatalf("List empty: %v", err)
	}
	if len(names) != 0 {
		t.Errorf("expected empty list, got %v", names)
	}

	s1 := &rca.Scenario{Name: "alpha"}
	s2 := &rca.Scenario{Name: "beta"}
	_ = store.Save(ctx, s1)
	_ = store.Save(ctx, s2)

	names, err = store.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(names) != 2 {
		t.Errorf("len(names) = %d, want 2", len(names))
	}
}

func TestFileStore_ListNonexistent(t *testing.T) {
	store := NewFileStore("/tmp/nonexistent-origami-test-dir")
	ctx := context.Background()

	names, err := store.List(ctx)
	if err != nil {
		t.Fatalf("List should not error for nonexistent dir: %v", err)
	}
	if names != nil {
		t.Errorf("expected nil, got %v", names)
	}
}

func TestFileStore_LoadNotFound(t *testing.T) {
	dir := t.TempDir()
	store := NewFileStore(dir)
	ctx := context.Background()

	_, err := store.Load(ctx, "missing")
	if err == nil {
		t.Fatal("expected error for missing dataset")
	}
}

func TestFileStore_SaveCreatesDir(t *testing.T) {
	dir := t.TempDir() + "/sub/deep"
	store := NewFileStore(dir)
	ctx := context.Background()

	s := &rca.Scenario{Name: "nested"}
	if err := store.Save(ctx, s); err != nil {
		t.Fatalf("Save should create nested dirs: %v", err)
	}

	if _, err := os.Stat(dir); err != nil {
		t.Errorf("dir should exist: %v", err)
	}
}
