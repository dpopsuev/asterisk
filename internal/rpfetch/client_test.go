package rpfetch

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"asterisk/internal/preinvest"
)

func TestClient_FetchEnvelope_MockHTTP(t *testing.T) {
	launchResp := map[string]interface{}{
		"id":        33195,
		"uuid":      "a1610fc6-8700-4814-9947-a5595d9b7f8c",
		"name":      "telco-ft-ran-ptp-4.21",
		"number":    13,
		"status":    "FAILED",
		"startTime": 1771104069000,
		"endTime":   1771133352804,
	}
	itemsResp := []map[string]interface{}{
		{"id": 1697136, "uuid": "c2b48976-9589-456b-bf87-fec5ac91838c", "name": "[T-TSC] RAN PTP tests", "type": "TEST", "status": "FAILED", "path": "1697136", "launchId": 33195},
		{"id": 1697139, "uuid": "bdc98492-3e85-4f45-b266-952db892d1d1", "name": "[T-BC] RAN PTP tests", "type": "TEST", "status": "FAILED", "path": "1697139", "launchId": 33195},
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/ecosystem-qe/launch/33195" {
			_ = json.NewEncoder(w).Encode(launchResp)
			return
		}
		if r.URL.Path == "/api/v1/ecosystem-qe/item" && r.URL.Query().Get("filter.eq.launchId") == "33195" {
			_ = json.NewEncoder(w).Encode(itemsResp)
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

	env, err := client.FetchEnvelope(33195)
	if err != nil {
		t.Fatalf("FetchEnvelope: %v", err)
	}
	if env.RunID != "33195" || env.Name != "telco-ft-ran-ptp-4.21" {
		t.Errorf("envelope: run_id=%q name=%q", env.RunID, env.Name)
	}
	if len(env.FailureList) != 2 {
		t.Errorf("failure_list: want 2, got %d", len(env.FailureList))
	}
	if env.FailureList[0].ID != 1697136 || env.FailureList[0].Name != "[T-TSC] RAN PTP tests" {
		t.Errorf("first failure: %+v", env.FailureList[0])
	}
}

func TestFetcher_ImplementsPreinvestFetcher(t *testing.T) {
	var _ preinvest.Fetcher = (*Fetcher)(nil)
}

func TestReadAPIKey(t *testing.T) {
	// ReadAPIKey reads from file; without a real file we get error
	_, err := ReadAPIKey("/nonexistent/rp-api-key")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}
