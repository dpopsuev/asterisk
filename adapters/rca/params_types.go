package rca

// TemplateParams holds all parameter groups injected into prompt templates.
// Templates use {{.Group.Field}} to access values.
type TemplateParams struct {
	LaunchID string
	CaseID   int64
	StepName string

	Envelope *EnvelopeParams

	Env map[string]string

	Git *GitParams

	Failure *FailureParams

	Siblings []SiblingParams

	Workspace *WorkspaceParams

	URLs *URLParams

	AlwaysReadSources []AlwaysReadSource

	Prior *PriorParams

	History *HistoryParams

	// Recall digest: all RCAs discovered so far in the current run.
	// Populated at F0_RECALL to enable cross-case recall in parallel mode.
	RecallDigest []RecallDigestEntry

	Taxonomy *TaxonomyParams

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
	ID                int64
	Title             string
	DefectType        string
	Status            string
	AffectedVersions  string
	JiraLink          string
	ResolvedAt        string
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
	DefectTypes string
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
