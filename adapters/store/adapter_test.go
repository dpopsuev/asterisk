package store

import (
	"path/filepath"
	"testing"

	"asterisk/adapters/rp"
)

func TestEnvelopeStoreAdapter_WithSqlStore(t *testing.T) {
	path := filepath.Join(t.TempDir(), "store.db")
	s, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer s.Close()

	adapter := &EnvelopeStoreAdapter{Store: s}
	env := &rp.Envelope{RunID: "99", Name: "adapter-test", FailureList: nil}
	if err := adapter.Save(99, env); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := adapter.Get(99)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got == nil || got.RunID != "99" {
		t.Errorf("Get: got %+v", got)
	}
}
