package main

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"testing"
)

// TestAnalyze_Stdin verifies that `metacal analyze --response-file -` reads
// from stdin and produces the same output as file-based analysis.
// RED: fails before stdin support is added (os.ReadFile("-") → no such file).
func TestAnalyze_Stdin(t *testing.T) {
	goldenResponse := `{"model_name": "claude-sonnet-4-20250514", "provider": "Anthropic", "version": "20250514", "wrapper": "Cursor"}

` + "```go\n" + `func sumAbsolute(numbers []int, label string, verbose bool) (int, string, error) {
	total := 0
	for _, num := range numbers {
		if num > 0 { total += num } else if num < 0 { total -= num }
	}
	if total == 0 { return 0, "", fmt.Errorf("empty result for %s", label) }
	return total, "", nil
}
` + "```\n"

	oldStdin := os.Stdin
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("create pipe: %v", err)
	}
	os.Stdin = r
	go func() {
		_, _ = w.Write([]byte(goldenResponse))
		w.Close()
	}()
	defer func() { os.Stdin = oldStdin }()

	oldStdout := os.Stdout
	outR, outW, err := os.Pipe()
	if err != nil {
		t.Fatalf("create stdout pipe: %v", err)
	}
	os.Stdout = outW
	defer func() { os.Stdout = oldStdout }()

	analyzeErr := cmdAnalyze([]string{"--response-file", "-"})

	outW.Close()
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, outR)
	os.Stdout = oldStdout

	if analyzeErr != nil {
		t.Fatalf("cmdAnalyze with stdin failed: %v", analyzeErr)
	}

	var result AnalyzeResult
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("parse output JSON: %v\nraw: %s", err, buf.String())
	}

	if result.Identity.ModelName != "claude-sonnet-4-20250514" {
		t.Errorf("model_name: got %q, want claude-sonnet-4-20250514", result.Identity.ModelName)
	}
	if result.Identity.Provider != "Anthropic" {
		t.Errorf("provider: got %q, want Anthropic", result.Identity.Provider)
	}
	if result.Key != "claude-sonnet-4-20250514" {
		t.Errorf("key: got %q, want claude-sonnet-4-20250514", result.Key)
	}
	if result.Wrapper {
		t.Error("wrapper should be false for foundation model")
	}
	if result.Code == "" {
		t.Error("code should not be empty")
	}
}

// TestSave_Stdin verifies that `metacal save --report-file -` reads from stdin.
// RED: fails before stdin support is added.
func TestSave_Stdin(t *testing.T) {
	report := `{
  "run_id": "test-stdin-save",
  "start_time": "2026-02-21T12:00:00Z",
  "end_time": "2026-02-21T12:01:00Z",
  "config": {"max_iterations": 15, "probe_id": "refactor-v1", "terminate_on_repeat": true},
  "results": [],
  "unique_models": [],
  "termination_reason": "stdin test"
}`

	runsDir := t.TempDir()

	oldStdin := os.Stdin
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("create pipe: %v", err)
	}
	os.Stdin = r
	go func() {
		_, _ = w.Write([]byte(report))
		w.Close()
	}()
	defer func() { os.Stdin = oldStdin }()

	saveErr := cmdSave([]string{"--report-file", "-", "--runs-dir", runsDir})
	if saveErr != nil {
		t.Fatalf("cmdSave with stdin failed: %v", saveErr)
	}

	entries, err := os.ReadDir(runsDir)
	if err != nil {
		t.Fatalf("read runs dir: %v", err)
	}

	found := false
	for _, e := range entries {
		if e.Name() == "test-stdin-save.json" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected test-stdin-save.json in %s, got %v", runsDir, entries)
	}
}
