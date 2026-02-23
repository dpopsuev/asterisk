package calibrate

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/dpopsuev/origami"
)

type testArtifact struct {
	Status string `json:"status"`
	Score  int    `json:"score"`
}

func TestStepExtractor_ImplementsExtractor(t *testing.T) {
	var ext framework.Extractor = NewStepExtractor[testArtifact]("test-step")
	if ext.Name() != "test-step" {
		t.Errorf("Name() = %q, want %q", ext.Name(), "test-step")
	}
}

func TestStepExtractor_RawMessage(t *testing.T) {
	ext := NewStepExtractor[testArtifact]("step-raw")
	input := json.RawMessage(`{"status":"ok","score":99}`)
	result, err := ext.Extract(context.Background(), input)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	art, ok := result.(*testArtifact)
	if !ok {
		t.Fatalf("result type = %T, want *testArtifact", result)
	}
	if art.Status != "ok" || art.Score != 99 {
		t.Errorf("got {%q, %d}, want {ok, 99}", art.Status, art.Score)
	}
}

func TestStepExtractor_Bytes(t *testing.T) {
	ext := NewStepExtractor[testArtifact]("step-bytes")
	result, err := ext.Extract(context.Background(), []byte(`{"status":"done","score":1}`))
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	art := result.(*testArtifact)
	if art.Status != "done" {
		t.Errorf("Status = %q, want %q", art.Status, "done")
	}
}

func TestStepExtractor_MalformedJSON(t *testing.T) {
	ext := NewStepExtractor[testArtifact]("step-bad")
	_, err := ext.Extract(context.Background(), json.RawMessage(`{broken`))
	if err == nil {
		t.Fatal("expected error for malformed JSON")
	}
}

func TestStepExtractor_WrongType(t *testing.T) {
	ext := NewStepExtractor[testArtifact]("step-type")
	_, err := ext.Extract(context.Background(), "not bytes")
	if err == nil {
		t.Fatal("expected error for string input")
	}
}

func TestStepExtractor_MatchesParseJSON(t *testing.T) {
	data := json.RawMessage(`{"status":"match","score":42}`)

	oldResult, err := parseJSON[testArtifact](data)
	if err != nil {
		t.Fatalf("parseJSON: %v", err)
	}

	ext := NewStepExtractor[testArtifact]("match-test")
	newResult, err := ext.Extract(context.Background(), data)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	art := newResult.(*testArtifact)

	if art.Status != oldResult.Status || art.Score != oldResult.Score {
		t.Errorf("mismatch: extractor={%q,%d} vs parseJSON={%q,%d}",
			art.Status, art.Score, oldResult.Status, oldResult.Score)
	}
}
