package calibrate

import (
	"encoding/json"

	"asterisk/pkg/framework"
	"asterisk/internal/orchestrate"
	"asterisk/internal/store"
	"asterisk/internal/workspace"
)

// ModelAdapter is the interface for sending prompts and receiving responses.
// Different adapters support different calibration modes.
type ModelAdapter interface {
	// Name returns the adapter identifier (e.g. "stub", "cursor", "llm-api").
	Name() string

	// SendPrompt sends a prompt and returns a raw JSON response that can be
	// parsed into the appropriate artifact type for the given step.
	SendPrompt(caseID string, step orchestrate.PipelineStep, prompt string) (json.RawMessage, error)
}

// StoreAware is an optional interface implemented by adapters that need
// per-run store injection (e.g. CursorAdapter).
type StoreAware interface {
	SetStore(st store.Store)
	SetSuiteID(id int64)
	SetWorkspace(ws *workspace.Workspace)
	RegisterCase(gtCaseID string, storeCase *store.Case)
}

// IDMappable is an optional interface implemented by adapters that track
// ground-truth-to-store ID mappings (e.g. StubAdapter).
type IDMappable interface {
	SetRCAID(gtID string, storeID int64)
	SetSymptomID(gtID string, storeID int64)
}

// Identifiable is an optional interface for adapters that can report
// which LLM model ("ghost") is behind the adapter ("shell").
// Called once at session start to populate session metadata.
type Identifiable interface {
	Identify() (framework.ModelIdentity, error)
}
