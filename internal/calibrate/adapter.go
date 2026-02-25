package calibrate

import (
	"encoding/json"

	cal "github.com/dpopsuev/origami/calibrate"
	"github.com/dpopsuev/origami/knowledge"

	"asterisk/internal/store"
)

// ModelAdapter is the interface for sending prompts and receiving responses.
// Step is a plain string so the interface matches cal.ModelAdapter.
type ModelAdapter interface {
	Name() string
	SendPrompt(caseID string, step string, prompt string) (json.RawMessage, error)
}

// Compile-time check: local ModelAdapter is compatible with cal.ModelAdapter.
var _ cal.ModelAdapter = (ModelAdapter)(nil)

// Identifiable is an alias for the generic calibrate.Identifiable interface.
type Identifiable = cal.Identifiable

// StoreAware is an optional interface implemented by adapters that need
// per-run store injection (e.g. LLMAdapter).
type StoreAware interface {
	SetStore(st store.Store)
	SetSuiteID(id int64)
	SetCatalog(cat *knowledge.KnowledgeSourceCatalog)
	RegisterCase(gtCaseID string, storeCase *store.Case)
}

// IDMappable is an optional interface implemented by adapters that track
// ground-truth-to-store ID mappings (e.g. StubAdapter).
type IDMappable interface {
	SetRCAID(gtID string, storeID int64)
	SetSymptomID(gtID string, storeID int64)
}
