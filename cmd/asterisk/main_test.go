package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"asterisk/internal/preinvest"
)

func TestAnalyzeAndPush_FileEnvelope(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, "envelope.json")
	artifactPath := filepath.Join(dir, "artifact.json")
	env := &preinvest.Envelope{
		RunID:  "99",
		Name:   "test",
		FailureList: []preinvest.FailureItem{{ID: 1, Name: "fail1", Status: "FAILED"}},
	}
	data, _ := json.MarshalIndent(env, "", "  ")
	if err := os.WriteFile(envPath, data, 0644); err != nil {
		t.Fatal(err)
	}
	root := filepath.Join("..", "..")
	cmd := exec.Command("go", "run", "./cmd/asterisk", "analyze", "--launch="+envPath, "-o", artifactPath)
	cmd.Dir = root
	cmd.Env = os.Environ()
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("analyze: %v\n%s", err, out)
	}
	if _, err := os.Stat(artifactPath); err != nil {
		t.Fatalf("artifact not created: %v", err)
	}
	cmd2 := exec.Command("go", "run", "./cmd/asterisk", "push", "-f", artifactPath)
	cmd2.Dir = root
	cmd2.Env = os.Environ()
	out2, err := cmd2.CombinedOutput()
	if err != nil {
		t.Fatalf("push: %v\n%s", err, out2)
	}
}
