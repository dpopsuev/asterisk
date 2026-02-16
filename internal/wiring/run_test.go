package wiring

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"asterisk/internal/postinvest"
	"asterisk/internal/preinvest"
)

// BDD: Given launch ID and fixture envelope, When full flow runs, Then envelope stored, artifact exists, push recorded.
func TestRun_FullFlowStoresEnvelopeWritesArtifactRecordsPush(t *testing.T) {
	launchID := 33195
	env := loadFixtureEnvelope(t)
	fetcher := preinvest.NewStubFetcher(env)
	envelopeStore := preinvest.NewMemStore()
	dir := t.TempDir()
	artifactPath := filepath.Join(dir, "artifact.json")
	pushStore := postinvest.NewMemPushStore()

	err := Run(fetcher, envelopeStore, launchID, artifactPath, pushStore, "PROJ-456", "https://jira.example.com/PROJ-456")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	// (1) Envelope in store
	gotEnv, err := envelopeStore.Get(launchID)
	if err != nil || gotEnv == nil {
		t.Fatalf("envelope not in store: err=%v", err)
	}
	if gotEnv.RunID != env.RunID {
		t.Errorf("envelope RunID: got %q want %q", gotEnv.RunID, env.RunID)
	}

	// (2) Artifact file exists and has shape
	data, err := os.ReadFile(artifactPath)
	if err != nil {
		t.Fatalf("artifact file: %v", err)
	}
	var a struct {
		LaunchID   string `json:"launch_id"`
		CaseIDs    []int  `json:"case_ids"`
		DefectType string `json:"defect_type"`
	}
	if err := json.Unmarshal(data, &a); err != nil {
		t.Fatalf("artifact unmarshal: %v", err)
	}
	if a.LaunchID != env.RunID {
		t.Errorf("artifact LaunchID: got %q want %q", a.LaunchID, env.RunID)
	}

	// (3) Push recorded in mock
	gotPush := pushStore.LastPushed()
	if gotPush == nil {
		t.Fatal("push not recorded")
	}
	if gotPush.DefectType == "" {
		t.Error("push DefectType: empty")
	}
	if gotPush.JiraTicketID != "PROJ-456" {
		t.Errorf("push JiraTicketID: got %q want PROJ-456", gotPush.JiraTicketID)
	}
}

func loadFixtureEnvelope(t *testing.T) *preinvest.Envelope {
	t.Helper()
	path := filepath.Join("..", "..", "examples", "pre-investigation-33195-4.21", "envelope_33195_4.21.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("fixture not found: %v", err)
	}
	var env preinvest.Envelope
	if err := json.Unmarshal(data, &env); err != nil {
		t.Fatalf("unmarshal fixture: %v", err)
	}
	return &env
}
