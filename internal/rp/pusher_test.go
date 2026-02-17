package rp

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"asterisk/internal/investigate"
	"asterisk/internal/postinvest"
)

func TestPusher_Push_Success(t *testing.T) {
	var updateCalls []int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Accept PUT requests for item update
		if r.Method == "PUT" {
			updateCalls = append(updateCalls, 1)
			w.WriteHeader(http.StatusOK)
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	client, err := New(server.URL, "test-token", WithHTTPClient(server.Client()))
	if err != nil {
		t.Fatal(err)
	}

	pusher := NewPusher(client, "ecosystem-qe")

	// Write artifact to temp file
	artifact := investigate.Artifact{
		LaunchID:   "12345",
		CaseIDs:    []int{100, 101},
		DefectType: "pb001",
		RCAMessage: "PTP clock sync failure",
	}
	data, _ := json.Marshal(artifact)
	tmpDir := t.TempDir()
	artifactPath := filepath.Join(tmpDir, "artifact.json")
	if err := os.WriteFile(artifactPath, data, 0644); err != nil {
		t.Fatal(err)
	}

	store := postinvest.NewMemPushStore()
	err = pusher.Push(artifactPath, store, "JIRA-123", "https://jira.example.com/JIRA-123")
	if err != nil {
		t.Fatalf("Push: %v", err)
	}

	// Verify UpdateDefect was called for each case
	if len(updateCalls) != 2 {
		t.Errorf("expected 2 update calls, got %d", len(updateCalls))
	}

	// Verify store record
	last := store.LastPushed()
	if last == nil {
		t.Fatal("expected pushed record")
	}
	if last.DefectType != "pb001" {
		t.Errorf("defect type = %q, want pb001", last.DefectType)
	}
	if last.JiraTicketID != "JIRA-123" {
		t.Errorf("jira ticket = %q, want JIRA-123", last.JiraTicketID)
	}
	if len(last.CaseIDs) != 2 {
		t.Errorf("expected 2 case IDs, got %d", len(last.CaseIDs))
	}
}

func TestPusher_Push_MissingFile(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client, _ := New(server.URL, "test-token", WithHTTPClient(server.Client()))
	pusher := NewPusher(client, "ecosystem-qe")
	store := postinvest.NewMemPushStore()

	err := pusher.Push("/nonexistent/path.json", store, "", "")
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestPusher_Push_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client, _ := New(server.URL, "test-token", WithHTTPClient(server.Client()))
	pusher := NewPusher(client, "ecosystem-qe")
	store := postinvest.NewMemPushStore()

	tmpDir := t.TempDir()
	artifactPath := filepath.Join(tmpDir, "bad.json")
	if err := os.WriteFile(artifactPath, []byte("not json"), 0644); err != nil {
		t.Fatal(err)
	}

	err := pusher.Push(artifactPath, store, "", "")
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestPusher_Push_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(ErrorRS{ErrorCode: 50000, Message: "Internal Server Error"})
	}))
	defer server.Close()

	client, _ := New(server.URL, "test-token", WithHTTPClient(server.Client()))
	pusher := NewPusher(client, "ecosystem-qe")

	artifact := investigate.Artifact{
		LaunchID:   "12345",
		CaseIDs:    []int{100},
		DefectType: "pb001",
	}
	data, _ := json.Marshal(artifact)
	tmpDir := t.TempDir()
	artifactPath := filepath.Join(tmpDir, "artifact.json")
	os.WriteFile(artifactPath, data, 0644)

	store := postinvest.NewMemPushStore()
	err := pusher.Push(artifactPath, store, "", "")
	if err == nil {
		t.Error("expected error for API failure")
	}
}
