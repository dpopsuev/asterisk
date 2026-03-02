package rca

import (
	"encoding/json"
	"fmt"
)

// WalkerContextKeys used by RCA transformers and hooks to read runtime
// dependencies from the walker's context map.
const (
	KeyCaseLabel = "rca.case_label"
	KeyStore     = "rca.store"
	KeyCaseData  = "rca.case_data"
	KeyEnvelope  = "rca.envelope"
	KeyCatalog   = "rca.catalog"
	KeyCaseDir   = "rca.case_dir"
	KeyPromptDir = "rca.prompt_dir"
)

// parseTypedArtifact parses a JSON response into the appropriate typed artifact
// based on the circuit step. Uses parseJSON (from cal_runner.go) which handles
// JSON cleaning (stripping markdown fences, etc.).
func parseTypedArtifact(step CircuitStep, data json.RawMessage) (any, error) {
	switch step {
	case StepF0Recall:
		return parseJSON[RecallResult](data)
	case StepF1Triage:
		return parseJSON[TriageResult](data)
	case StepF2Resolve:
		return parseJSON[ResolveResult](data)
	case StepF3Invest:
		return parseJSON[InvestigateArtifact](data)
	case StepF4Correlate:
		return parseJSON[CorrelateResult](data)
	case StepF5Review:
		return parseJSON[ReviewDecision](data)
	case StepF6Report:
		return parseJSON[map[string]any](data)
	default:
		return nil, fmt.Errorf("unknown step %s", step)
	}
}
