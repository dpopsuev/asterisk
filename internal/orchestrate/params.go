package orchestrate

import (
	"asterisk/internal/preinvest"
	"asterisk/internal/store"
	"asterisk/internal/workspace"
)

// TemplateParams holds all parameter groups injected into prompt templates.
// Templates use {{.Group.Field}} to access values.
type TemplateParams struct {
	// Identity
	LaunchID string
	CaseID   int64
	StepName string

	// Envelope context
	Envelope *EnvelopeParams

	// Environment attributes
	Env map[string]string

	// Git context
	Git *GitParams

	// Failure context
	Failure *FailureParams

	// Sibling failures
	Siblings []SiblingParams

	// Workspace repos
	Workspace *WorkspaceParams

	// URLs
	URLs *URLParams

	// Prior stage context
	Prior *PriorParams

	// Historical context
	History *HistoryParams

	// Taxonomy reference
	Taxonomy *TaxonomyParams

	// Timestamps
	Timestamps *TimestampParams
}

// EnvelopeParams holds envelope-level context.
type EnvelopeParams struct {
	Name   string
	RunID  string
	Status string
}

// GitParams holds git metadata from the envelope.
type GitParams struct {
	Branch string
	Commit string
}

// FailureParams holds the failure under investigation.
type FailureParams struct {
	TestName     string
	ErrorMessage string
	LogSnippet   string
	LogTruncated bool
	Status       string
	Path         string
}

// SiblingParams holds a sibling failure for context.
type SiblingParams struct {
	ID     int
	Name   string
	Status string
}

// WorkspaceParams holds repo list from the context workspace.
type WorkspaceParams struct {
	Repos []RepoParams
}

// RepoParams holds one repo's metadata.
type RepoParams struct {
	Name    string
	Path    string
	Purpose string
	Branch  string
}

// URLParams holds pre-built navigable links.
type URLParams struct {
	RPLaunch string
	RPItem   string
}

// PriorParams holds prior stage artifacts for context injection.
type PriorParams struct {
	RecallResult      *RecallResult
	TriageResult      *TriageResult
	ResolveResult     *ResolveResult
	InvestigateResult *InvestigateArtifact
	CorrelateResult   *CorrelateResult
}

// HistoryParams holds historical data from the Store.
type HistoryParams struct {
	SymptomInfo *SymptomInfoParams
	PriorRCAs   []PriorRCAParams
}

// SymptomInfoParams holds cross-version symptom knowledge.
type SymptomInfoParams struct {
	Name                  string
	OccurrenceCount       int
	FirstSeen             string
	LastSeen              string
	Status                string
	IsDormantReactivation bool
}

// PriorRCAParams holds a prior RCA for history injection.
type PriorRCAParams struct {
	ID               int64
	Title            string
	DefectType       string
	Status           string
	AffectedVersions string
	JiraLink         string
	ResolvedAt       string
	DaysSinceResolved int
}

// TaxonomyParams holds defect type vocabulary.
type TaxonomyParams struct {
	DefectTypes string // pre-formatted defect type list for injection
}

// TimestampParams holds clock plane warnings.
type TimestampParams struct {
	ClockPlaneNote   string
	ClockSkewWarning string
}

// DefaultTaxonomy returns the standard defect type taxonomy.
func DefaultTaxonomy() *TaxonomyParams {
	return &TaxonomyParams{
		DefectTypes: `Defect types:
- pb001: Product Bug — defect in the product code (operator, daemon, proxy, etc.)
- ab001: Automation Bug — defect in test code, CI config, or test infrastructure
- si001: System Issue — infrastructure/environment issue (node, network, cluster, NTP, etc.)
- nd001: No Defect — test is correct, product is correct, flaky/transient/expected behavior
- ti001: To Investigate — insufficient data to classify; needs manual investigation`,
	}
}

// BuildParams constructs the full TemplateParams from available data.
func BuildParams(
	st store.Store,
	caseData *store.Case,
	env *preinvest.Envelope,
	ws *workspace.Workspace,
	step PipelineStep,
	caseDir string,
) *TemplateParams {
	params := &TemplateParams{
		CaseID:   caseData.ID,
		StepName: string(step),
		Taxonomy: DefaultTaxonomy(),
		Timestamps: &TimestampParams{
			ClockPlaneNote: "Note: Timestamps may originate from different clock planes (executor, test node, SUT). Cross-plane time comparisons may be unreliable.",
		},
	}

	// Envelope context
	if env != nil {
		params.LaunchID = env.RunID
		params.Envelope = &EnvelopeParams{
			Name:  env.Name,
			RunID: env.RunID,
		}
		// Sibling failures
		for _, f := range env.FailureList {
			params.Siblings = append(params.Siblings, SiblingParams{
				ID: f.ID, Name: f.Name, Status: f.Status,
			})
		}
	}

	// Failure context from case
	params.Failure = &FailureParams{
		TestName:     caseData.Name,
		ErrorMessage: caseData.ErrorMessage,
		LogSnippet:   caseData.LogSnippet,
		LogTruncated: caseData.LogTruncated,
		Status:       caseData.Status,
	}

	// Workspace repos
	if ws != nil {
		wsp := &WorkspaceParams{}
		for _, r := range ws.Repos {
			wsp.Repos = append(wsp.Repos, RepoParams{
				Name:    r.Name,
				Path:    r.Path,
				Purpose: r.Purpose,
				Branch:  r.Branch,
			})
		}
		params.Workspace = wsp
	}

	// Load prior artifacts from case directory
	params.Prior = loadPriorArtifacts(caseDir)

	// Load history from store
	if st != nil && caseData.SymptomID != 0 {
		params.History = loadHistory(st, caseData.SymptomID)
	}

	return params
}

// loadPriorArtifacts reads previously generated artifacts from the case directory.
func loadPriorArtifacts(caseDir string) *PriorParams {
	if caseDir == "" {
		return nil
	}
	prior := &PriorParams{}
	prior.RecallResult, _ = ReadArtifact[RecallResult](caseDir, ArtifactFilename(StepF0Recall))
	prior.TriageResult, _ = ReadArtifact[TriageResult](caseDir, ArtifactFilename(StepF1Triage))
	prior.ResolveResult, _ = ReadArtifact[ResolveResult](caseDir, ArtifactFilename(StepF2Resolve))
	prior.InvestigateResult, _ = ReadArtifact[InvestigateArtifact](caseDir, ArtifactFilename(StepF3Invest))
	prior.CorrelateResult, _ = ReadArtifact[CorrelateResult](caseDir, ArtifactFilename(StepF4Correlate))
	return prior
}

// loadHistory loads symptom and RCA history from the store.
func loadHistory(st store.Store, symptomID int64) *HistoryParams {
	history := &HistoryParams{}

	sym, err := st.GetSymptom(symptomID)
	if err == nil && sym != nil {
		history.SymptomInfo = &SymptomInfoParams{
			Name:            sym.Name,
			OccurrenceCount: sym.OccurrenceCount,
			FirstSeen:       sym.FirstSeenAt,
			LastSeen:        sym.LastSeenAt,
			Status:          sym.Status,
		}
	}

	links, err := st.GetRCAsForSymptom(symptomID)
	if err == nil {
		for _, link := range links {
			rca, err := st.GetRCAV2(link.RCAID)
			if err != nil || rca == nil {
				continue
			}
			history.PriorRCAs = append(history.PriorRCAs, PriorRCAParams{
				ID:               rca.ID,
				Title:            rca.Title,
				DefectType:       rca.DefectType,
				Status:           rca.Status,
				AffectedVersions: rca.AffectedVersions,
				JiraLink:         rca.JiraLink,
				ResolvedAt:       rca.ResolvedAt,
			})
		}
	}

	return history
}
