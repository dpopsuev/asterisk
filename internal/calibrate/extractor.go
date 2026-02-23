package calibrate

import (
	"context"
	"encoding/json"
	"fmt"
)

// StepExtractor wraps parseJSON[T] as a framework.Extractor implementation.
// Demonstrates that the Extractor interface handles the existing calibration
// pattern: json.RawMessage -> typed struct per pipeline step.
//
// Input: json.RawMessage or []byte. Output: *T.
type StepExtractor[T any] struct {
	name string
}

// NewStepExtractor creates an Extractor that unmarshals JSON into *T.
func NewStepExtractor[T any](name string) *StepExtractor[T] {
	return &StepExtractor[T]{name: name}
}

func (e *StepExtractor[T]) Name() string { return e.name }

func (e *StepExtractor[T]) Extract(_ context.Context, input any) (any, error) {
	var data []byte
	switch v := input.(type) {
	case json.RawMessage:
		data = v
	case []byte:
		data = v
	default:
		return nil, fmt.Errorf("StepExtractor %q: expected json.RawMessage or []byte, got %T", e.name, input)
	}
	var result T
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("StepExtractor %q: %w", e.name, err)
	}
	return &result, nil
}
