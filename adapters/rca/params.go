package rca

import (
	"os"

	"github.com/dpopsuev/origami/adapters/rp"
	"asterisk/adapters/store"
	"github.com/dpopsuev/origami/knowledge"
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

	// Always-read knowledge sources (from ReadAlways policy)
	AlwaysReadSources []AlwaysReadSource

	// Prior stage context
	Prior *PriorParams

	// Historical context
	History *HistoryParams

	// Recall digest: all RCAs discovered so far in the current run.
	// Populated at F0_RECALL to enable cross-case recall in parallel mode.
	RecallDigest []RecallDigestEntry

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

// ResolutionStatus indicates whether a workspace field was successfully resolved.
type ResolutionStatus string

const (
	Resolved    ResolutionStatus = "resolved"
	Unavailable ResolutionStatus = "unavailable"
)

// WorkspaceParams holds repo list, launch attributes, and Jira links.
type WorkspaceParams struct {
	Repos            []RepoParams
	LaunchAttributes []AttributeParams
	JiraLinks        []JiraLinkParams
	AttrsStatus      ResolutionStatus
	JiraStatus       ResolutionStatus
	ReposStatus      ResolutionStatus
}

// RepoParams holds one repo's metadata.
type RepoParams struct {
	Name    string
	Path    string
	Purpose string
	Branch  string
}

// AttributeParams holds a key-value launch attribute from RP.
type AttributeParams struct {
	Key    string
	Value  string
	System bool
}

// JiraLinkParams holds an external issue link from RP test items.
type JiraLinkParams struct {
	TicketID string
	URL      string
}

// URLParams holds pre-built navigable links.
type URLParams struct {
	RPLaunch string
	RPItem   string
}

// AlwaysReadSource holds the content of a knowledge source that is always
// loaded regardless of routing rules (ReadPolicy == ReadAlways).
type AlwaysReadSource struct {
	Name    string
	Purpose string
	Content string
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

// RecallDigestEntry summarizes one RCA for the recall digest.
type RecallDigestEntry struct {
	ID         int64
	Component  string
	DefectType string
	Summary    string
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
- au001: Automation Bug — defect in test code, CI config, or test infrastructure
- en001: Environment Issue — infrastructure/environment issue (node, network, cluster, NTP, etc.)
- fw001: Firmware Issue — defect in firmware or hardware-adjacent code (NIC, FPGA, PHC)
- nd001: No Defect — test is correct, product is correct, flaky/transient/expected behavior
- ti001: To Investigate — insufficient data to classify; needs manual investigation`,
	}
}

// BuildParams constructs the full TemplateParams from available data.
func BuildParams(
	st store.Store,
	caseData *store.Case,
	env *rp.Envelope,
	catalog *knowledge.KnowledgeSourceCatalog,
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

	// Workspace: repos, launch attributes, Jira links
	wsp := &WorkspaceParams{}

	if catalog != nil && len(catalog.Sources) > 0 {
		wsp.ReposStatus = Resolved
		for _, s := range catalog.Sources {
			wsp.Repos = append(wsp.Repos, RepoParams{
				Name:    s.Name,
				Path:    s.URI,
				Purpose: s.Purpose,
				Branch:  s.Branch,
			})
		}
	} else {
		wsp.ReposStatus = Unavailable
	}

	if env != nil && len(env.LaunchAttributes) > 0 {
		wsp.AttrsStatus = Resolved
		for _, a := range env.LaunchAttributes {
			wsp.LaunchAttributes = append(wsp.LaunchAttributes, AttributeParams{
				Key:    a.Key,
				Value:  a.Value,
				System: a.System,
			})
		}
	} else {
		wsp.AttrsStatus = Unavailable
	}

	if env != nil {
		seen := map[string]bool{}
		for _, f := range env.FailureList {
			for _, ext := range f.ExternalIssues {
				if ext.TicketID != "" && !seen[ext.TicketID] {
					seen[ext.TicketID] = true
					wsp.JiraLinks = append(wsp.JiraLinks, JiraLinkParams{
						TicketID: ext.TicketID,
						URL:      ext.URL,
					})
				}
			}
		}
	}
	if len(wsp.JiraLinks) > 0 {
		wsp.JiraStatus = Resolved
	} else {
		wsp.JiraStatus = Unavailable
	}

	params.Workspace = wsp

	// Load always-read knowledge sources
	if catalog != nil {
		params.AlwaysReadSources = loadAlwaysReadSources(catalog)
	}

	// Load prior artifacts from case directory
	params.Prior = loadPriorArtifacts(caseDir)

	// Load history from store
	if st != nil && caseData.SymptomID != 0 {
		params.History = loadHistory(st, caseData.SymptomID)
	}

	// At F0_RECALL with no linked symptom, search for candidate symptoms by test name.
	// This enables the recall prompt to see prior symptom/RCA data from earlier cases
	// that share the same test name, even before triage assigns a symptom to this case.
	if st != nil && step == StepF0Recall && caseData.SymptomID == 0 && params.History == nil {
		params.History = findRecallCandidates(st, caseData.Name)
	}

	// At F0_RECALL, also load a digest of ALL RCAs discovered so far.
	// In parallel mode, symptom-based recall may miss cases that haven't been
	// triaged yet. The digest provides a fallback: the model can match the
	// current failure against any known RCA, not just symptom-linked ones.
	if st != nil && step == StepF0Recall {
		params.RecallDigest = buildRecallDigest(st)
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

// loadAlwaysReadSources loads content from sources with ReadPolicy == ReadAlways.
func loadAlwaysReadSources(catalog *knowledge.KnowledgeSourceCatalog) []AlwaysReadSource {
	alwaysSources := catalog.AlwaysReadSources()
	if len(alwaysSources) == 0 {
		return nil
	}
	var result []AlwaysReadSource
	for _, s := range alwaysSources {
		if s.LocalPath == "" {
			continue
		}
		content, err := os.ReadFile(s.LocalPath)
		if err != nil || len(content) == 0 {
			continue
		}
		result = append(result, AlwaysReadSource{
			Name:    s.Name,
			Purpose: s.Purpose,
			Content: string(content),
		})
	}
	return result
}

// findRecallCandidates searches the store for symptoms matching the test name.
// At F0_RECALL the case hasn't been triaged yet (SymptomID == 0), so we can't
// use the fingerprint (which includes the triage category). Instead we match
// on test name — the most reliable attribute available before triage.
// Returns nil if no candidates are found.
func findRecallCandidates(st store.Store, testName string) *HistoryParams {
	// Guard: never search for recall candidates with an empty test name.
	// Empty names would match generic symptoms from unrelated cases,
	// causing false positives in the recall step.
	if testName == "" {
		return nil
	}
	candidates, err := st.FindSymptomCandidates(testName)
	if err != nil || len(candidates) == 0 {
		return nil
	}

	// Pick the best candidate: prefer the one with the most occurrences
	// (most established), breaking ties by most recently seen.
	best := candidates[0]
	for _, c := range candidates[1:] {
		if c.OccurrenceCount > best.OccurrenceCount {
			best = c
		} else if c.OccurrenceCount == best.OccurrenceCount && c.LastSeenAt > best.LastSeenAt {
			best = c
		}
	}

	history := loadHistory(st, best.ID)
	if history == nil {
		return nil
	}

	// Flag dormant reactivation: if the best candidate symptom is dormant,
	// this failure may be a regression.
	if best.Status == "dormant" && history.SymptomInfo != nil {
		history.SymptomInfo.IsDormantReactivation = true
	}

	return history
}

// buildRecallDigest loads all RCAs from the store as a flat digest.
func buildRecallDigest(st store.Store) []RecallDigestEntry {
	rcas, err := st.ListRCAs()
	if err != nil || len(rcas) == 0 {
		return nil
	}
	digest := make([]RecallDigestEntry, 0, len(rcas))
	for _, rca := range rcas {
		summary := rca.Description
		if len(summary) > 200 {
			summary = summary[:200] + "..."
		}
		digest = append(digest, RecallDigestEntry{
			ID:         rca.ID,
			Component:  rca.Component,
			DefectType: rca.DefectType,
			Summary:    summary,
		})
	}
	return digest
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

// LayeredCatalog filters a catalog using layered routing: base → version →
// investigation. Returns a new catalog containing only the matched sources
// in dependency-resolved order. If no layer tags are populated, returns
// the original catalog unchanged (safe default).
func LayeredCatalog(
	catalog *knowledge.KnowledgeSourceCatalog,
	version string,
	component string,
	deps *knowledge.DepGraph,
) *knowledge.KnowledgeSourceCatalog {
	if catalog == nil {
		return nil
	}

	baseTags := map[string]string{knowledge.LayerTagKey: knowledge.LayerBase}
	versionTags := map[string]string{knowledge.LayerTagKey: knowledge.LayerVersion}
	investigationTags := map[string]string{knowledge.LayerTagKey: knowledge.LayerInvestigation}

	if component != "" {
		baseTags["role"] = "sut"
	}
	if version != "" {
		versionTags["version"] = version
	}

	router := knowledge.NewRouter(catalog, knowledge.RequestTagMatchRule{})
	sources := router.LayeredRoute(baseTags, versionTags, investigationTags)

	if deps != nil {
		ordered, err := deps.OrderSources(sources)
		if err == nil {
			sources = ordered
		}
	}

	return &knowledge.KnowledgeSourceCatalog{Sources: sources}
}
