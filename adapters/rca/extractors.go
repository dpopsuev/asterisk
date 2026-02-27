package rca

import (
	"context"
	"encoding/json"
	"fmt"
)

// StepExtractor is a generic extractor that parses JSON input into a typed
// RCA artifact. Each pipeline step has its own extractor instance.
type StepExtractor[T any] struct {
	name string
}

// NewStepExtractor creates a typed extractor for a specific RCA step.
func NewStepExtractor[T any](name string) *StepExtractor[T] {
	return &StepExtractor[T]{name: name}
}

func (e *StepExtractor[T]) Name() string { return e.name }

func (e *StepExtractor[T]) Extract(_ context.Context, input any) (any, error) {
	switch v := input.(type) {
	case T:
		return v, nil
	case *T:
		return *v, nil
	case []byte:
		var result T
		if err := json.Unmarshal(v, &result); err != nil {
			return nil, fmt.Errorf("extractor %s: unmarshal: %w", e.name, err)
		}
		return result, nil
	case string:
		var result T
		if err := json.Unmarshal([]byte(v), &result); err != nil {
			return nil, fmt.Errorf("extractor %s: unmarshal string: %w", e.name, err)
		}
		return result, nil
	case json.RawMessage:
		var result T
		if err := json.Unmarshal(v, &result); err != nil {
			return nil, fmt.Errorf("extractor %s: unmarshal raw: %w", e.name, err)
		}
		return result, nil
	default:
		data, err := json.Marshal(v)
		if err != nil {
			return nil, fmt.Errorf("extractor %s: marshal input (%T): %w", e.name, input, err)
		}
		var result T
		if err := json.Unmarshal(data, &result); err != nil {
			return nil, fmt.Errorf("extractor %s: re-unmarshal: %w", e.name, err)
		}
		return result, nil
	}
}
