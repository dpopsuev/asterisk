package store

import (
	"os"
	"path/filepath"
	"testing"

	"asterisk/adapters/rp"
)

func TestSqlStore_Integration(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")
	s, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer s.Close()

	// Envelope
	env := &rp.Envelope{
		RunID:  "33195",
		Name:   "test-launch",
		FailureList: []rp.FailureItem{
			{ID: 1, Name: "fail1", Status: "FAILED"},
			{ID: 2, Name: "fail2", Status: "FAILED"},
		},
	}
	if err := s.SaveEnvelope(33195, env); err != nil {
		t.Fatalf("SaveEnvelope: %v", err)
	}
	got, err := s.GetEnvelope(33195)
	if err != nil {
		t.Fatalf("GetEnvelope: %v", err)
	}
	if got == nil || got.RunID != "33195" || len(got.FailureList) != 2 {
		t.Errorf("GetEnvelope: got %+v", got)
	}

	// Cases
	id1, err := s.CreateCase(33195, 1)
	if err != nil {
		t.Fatalf("CreateCase 1: %v", err)
	}
	_, err = s.CreateCase(33195, 2)
	if err != nil {
		t.Fatalf("CreateCase 2: %v", err)
	}
	list, err := s.ListCasesByLaunch(33195)
	if err != nil {
		t.Fatalf("ListCasesByLaunch: %v", err)
	}
	if len(list) != 2 {
		t.Errorf("ListCasesByLaunch: want 2, got %d", len(list))
	}
	c, err := s.GetCase(id1)
	if err != nil || c == nil || c.RPItemID != 1 || c.LaunchID == 0 {
		t.Errorf("GetCase: got %+v err %v", c, err)
	}
	if c.Status != "open" {
		t.Errorf("GetCase status: got %q want %q", c.Status, "open")
	}

	// RCA
	rcaID, err := s.SaveRCA(&RCA{Title: "R1", Description: "desc", DefectType: "ti001"})
	if err != nil {
		t.Fatalf("SaveRCA: %v", err)
	}
	if err := s.LinkCaseToRCA(id1, rcaID); err != nil {
		t.Fatalf("LinkCaseToRCA: %v", err)
	}
	r, err := s.GetRCA(rcaID)
	if err != nil || r == nil || r.Title != "R1" {
		t.Errorf("GetRCA: got %+v err %v", r, err)
	}
	rcas, err := s.ListRCAs()
	if err != nil || len(rcas) != 1 {
		t.Errorf("ListRCAs: got %d err %v", len(rcas), err)
	}
}

func TestSqlStore_OpenCreatesDir(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sub", "asterisk.db")
	s, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer s.Close()
	if _, err := os.Stat(filepath.Dir(path)); err != nil {
		t.Errorf("parent dir not created: %v", err)
	}
}
