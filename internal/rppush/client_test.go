package rppush

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"asterisk/internal/postinvest"
)

func TestClient_UpdateItemDefectType_MockHTTP(t *testing.T) {
	var lastBody map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if r.URL.Path == "/api/v1/ecosystem-qe/item/1697136/update" {
			_ = json.NewDecoder(r.Body).Decode(&lastBody)
			r.Body.Close()
			w.WriteHeader(http.StatusOK)
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	client := NewClient(Config{
		BaseURL: server.URL,
		APIKey:  "test-token",
		Project: "ecosystem-qe",
	})
	client.HTTPClient = server.Client()

	err := client.UpdateItemDefectType(1697136, "ti001")
	if err != nil {
		t.Fatalf("UpdateItemDefectType: %v", err)
	}
	if lastBody == nil {
		t.Error("expected request body to be sent")
	}
	if issues, ok := lastBody["issues"].([]interface{}); !ok || len(issues) == 0 {
		t.Errorf("expected issues in body: %+v", lastBody)
	}
}

func TestRPPusher_ImplementsPusher(t *testing.T) {
	var _ postinvest.Pusher = (*RPPusher)(nil)
}

func TestRPPusher_Push_MockHTTP(t *testing.T) {
	var updateCalls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/ecosystem-qe/item/1697136/update" || r.URL.Path == "/api/v1/ecosystem-qe/item/1697139/update" {
			updateCalls++
			w.WriteHeader(http.StatusOK)
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	client := NewClient(Config{BaseURL: server.URL, APIKey: "x", Project: "ecosystem-qe"})
	client.HTTPClient = server.Client()
	pusher := NewRPPusher(client)
	store := postinvest.NewMemPushStore()

	// Write a temp artifact
	artifactPath := t.TempDir() + "/artifact.json"
	artifactJSON := `{"launch_id":"33195","case_ids":[1697136,1697139],"rca_message":"","defect_type":"ti001","convergence_score":0.85,"evidence_refs":[]}`
	if err := os.WriteFile(artifactPath, []byte(artifactJSON), 0644); err != nil {
		t.Fatal(err)
	}

	err := pusher.Push(artifactPath, store, "", "")
	if err != nil {
		t.Fatalf("Push: %v", err)
	}
	if updateCalls != 2 {
		t.Errorf("expected 2 update calls, got %d", updateCalls)
	}
	rec := store.LastPushed()
	if rec == nil || rec.LaunchID != "33195" || rec.DefectType != "ti001" {
		t.Errorf("LastPushed: %+v", rec)
	}
}
