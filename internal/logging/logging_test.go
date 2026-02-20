package logging

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
)

func TestNew_HasComponent(t *testing.T) {
	var buf bytes.Buffer
	Init(slog.LevelDebug, "text", &buf)

	logger := New("test-component")
	logger.Info("hello")

	output := buf.String()
	if !strings.Contains(output, "component=test-component") {
		t.Errorf("expected component=test-component in output, got: %s", output)
	}
	if !strings.Contains(output, "hello") {
		t.Errorf("expected 'hello' in output, got: %s", output)
	}
}

func TestInit_TextFormat(t *testing.T) {
	var buf bytes.Buffer
	Init(slog.LevelInfo, "text", &buf)

	logger := New("fmt-test")
	logger.Info("text check")

	output := buf.String()
	if !strings.Contains(output, "level=INFO") {
		t.Errorf("expected level=INFO in text output, got: %s", output)
	}
}

func TestInit_JSONFormat(t *testing.T) {
	var buf bytes.Buffer
	Init(slog.LevelInfo, "json", &buf)

	logger := New("json-test")
	logger.Info("json check")

	output := buf.String()
	if !strings.Contains(output, `"level":"INFO"`) {
		t.Errorf("expected JSON level field, got: %s", output)
	}
	if !strings.Contains(output, `"component":"json-test"`) {
		t.Errorf("expected JSON component field, got: %s", output)
	}
}

func TestInit_LevelGating(t *testing.T) {
	var buf bytes.Buffer
	Init(slog.LevelWarn, "text", &buf)

	logger := New("gate-test")
	logger.Info("should be suppressed")
	logger.Warn("should appear")

	output := buf.String()
	if strings.Contains(output, "should be suppressed") {
		t.Error("Info message should be suppressed at Warn level")
	}
	if !strings.Contains(output, "should appear") {
		t.Error("Warn message should appear at Warn level")
	}
}
