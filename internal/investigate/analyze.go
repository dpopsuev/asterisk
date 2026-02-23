package investigate

import (
	"encoding/json"
	"os"

	"github.com/dpopsuev/origami/workspace"
)

// Analyzer runs investigation: envelope → cases + artifact. Mock implementation only.
type Analyzer interface {
	Analyze(src EnvelopeSource, launchID int, artifactPath string) error
}

// DefaultAnalyzer is the mock analyzer used in tests and wiring.
type DefaultAnalyzer struct{}

// Analyze implements Analyzer: read envelope from source, one case per failure, write artifact.
// Contract: .cursor/contracts/mock-investigation.md
func (DefaultAnalyzer) Analyze(src EnvelopeSource, launchID int, artifactPath string) error {
	return Analyze(src, launchID, artifactPath)
}

// Analyze reads envelope from source, creates one case per failure, and writes artifact to path.
// Contract: mock-investigation — no real AI; artifact has launch_id, case_ids, placeholder RCA fields.
func Analyze(src EnvelopeSource, launchID int, artifactPath string) error {
	return AnalyzeWithWorkspace(src, launchID, artifactPath, nil)
}

// AnalyzeWithWorkspace is like Analyze but accepts an optional context workspace (repos, purpose).
// When non-nil, workspace is available for downstream (e.g. prompts). Caller may load via workspace.LoadFromPath.
func AnalyzeWithWorkspace(src EnvelopeSource, launchID int, artifactPath string, ws *workspace.Workspace) error {
	env, err := src.Get(launchID)
	if err != nil {
		return err
	}
	if env == nil {
		return nil
	}
	_ = ws // used by prompts when building context; artifact unchanged for PoC
	artifact := Artifact{
		LaunchID:         env.RunID,
		CaseIDs:          CaseIDsFromEnvelope(env),
		RCAMessage:       "",
		DefectType:       "ti001",
		ConvergenceScore: 0.85,
		EvidenceRefs:     []string{},
	}
	data, err := json.MarshalIndent(artifact, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(artifactPath, data, 0644)
}
