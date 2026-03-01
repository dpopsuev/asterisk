package store

import "github.com/dpopsuev/origami/adapters/rp"

// DefaultDBPath is the default relative path for the SQLite DB (per-workspace).
// Resolve against cwd or workspace root; Open() creates the parent dir (e.g. .asterisk).
const DefaultDBPath = ".asterisk/asterisk.db"

// Store is the persistence facade for the two-tier data model.
// Tier 1: investigation-scoped entities (suite, circuit, launch, job, case, triage).
// Tier 2: global knowledge entities (symptom, rca, symptom_rca).
// Domain and CLI code use only this interface; implementation is SQLite or in-memory.
type Store interface {
	// ---------------------------------------------------------------
	// v1 methods (backward-compatible, kept for existing CLI commands)
	// ---------------------------------------------------------------

	// CreateCase creates a case from RP launch ID and item ID (v1 style).
	CreateCase(launchID, itemID int) (caseID int64, err error)
	// GetCase returns the case by its DB id (populates all available fields).
	GetCase(caseID int64) (*Case, error)
	// ListCasesByLaunch returns all cases for the RP launch (v1 key).
	ListCasesByLaunch(launchID int) ([]*Case, error)
	// SaveRCA inserts or updates an RCA; if rca.ID != 0, updates.
	SaveRCA(rca *RCA) (rcaID int64, err error)
	// LinkCaseToRCA sets case.rca_id (the verdict shortcut).
	LinkCaseToRCA(caseID, rcaID int64) error
	// GetRCA returns the RCA by id.
	GetRCA(rcaID int64) (*RCA, error)
	// ListRCAs returns all RCAs.
	ListRCAs() ([]*RCA, error)
	// SaveEnvelope stores an envelope blob by RP launch ID.
	SaveEnvelope(launchID int, env *rp.Envelope) error
	// GetEnvelope returns the envelope for the RP launch ID.
	GetEnvelope(launchID int) (*rp.Envelope, error)

	// ---------------------------------------------------------------
	// v2 methods — Investigation-scoped entities
	// ---------------------------------------------------------------

	// Suite operations
	CreateSuite(suite *InvestigationSuite) (int64, error)
	GetSuite(id int64) (*InvestigationSuite, error)
	ListSuites() ([]*InvestigationSuite, error)
	CloseSuite(id int64) error

	// Version operations
	CreateVersion(v *Version) (int64, error)
	GetVersion(id int64) (*Version, error)
	GetVersionByLabel(label string) (*Version, error)
	ListVersions() ([]*Version, error)

	// Circuit operations
	CreateCircuit(p *Circuit) (int64, error)
	GetCircuit(id int64) (*Circuit, error)
	ListCircuitsBySuite(suiteID int64) ([]*Circuit, error)

	// Launch operations
	CreateLaunch(l *Launch) (int64, error)
	GetLaunch(id int64) (*Launch, error)
	GetLaunchByRPID(circuitID int64, rpLaunchID int) (*Launch, error)
	ListLaunchesByCircuit(circuitID int64) ([]*Launch, error)

	// Job operations
	CreateJob(j *Job) (int64, error)
	GetJob(id int64) (*Job, error)
	ListJobsByLaunch(launchID int64) ([]*Job, error)

	// Case v2 operations (expanded fields)
	CreateCaseV2(c *Case) (int64, error)
	GetCaseV2(id int64) (*Case, error)
	ListCasesByJob(jobID int64) ([]*Case, error)
	ListCasesBySymptom(symptomID int64) ([]*Case, error)
	UpdateCaseStatus(caseID int64, status string) error
	LinkCaseToSymptom(caseID, symptomID int64) error

	// Triage operations
	CreateTriage(t *Triage) (int64, error)
	GetTriageByCase(caseID int64) (*Triage, error)

	// ---------------------------------------------------------------
	// v2 methods — Global knowledge entities
	// ---------------------------------------------------------------

	// Symptom operations
	CreateSymptom(s *Symptom) (int64, error)
	GetSymptom(id int64) (*Symptom, error)
	GetSymptomByFingerprint(fingerprint string) (*Symptom, error)
	// FindSymptomCandidates returns symptoms whose Name matches testName exactly.
	// Used at F0_RECALL to find prior data before a fingerprint (which requires
	// the triage category) is available. Returns nil slice if no match.
	FindSymptomCandidates(testName string) ([]*Symptom, error)
	// UpdateSymptomSeen increments occurrence_count and updates last_seen_at.
	UpdateSymptomSeen(id int64) error
	ListSymptoms() ([]*Symptom, error)
	// MarkDormantSymptoms transitions active symptoms older than staleDays to dormant.
	// Returns the number of rows affected.
	MarkDormantSymptoms(staleDays int) (int64, error)

	// RCA v2 operations (expanded fields)
	SaveRCAV2(rca *RCA) (int64, error)
	GetRCAV2(id int64) (*RCA, error)
	ListRCAsByStatus(status string) ([]*RCA, error)
	UpdateRCAStatus(id int64, status string) error

	// SymptomRCA operations (junction table)
	LinkSymptomToRCA(link *SymptomRCA) (int64, error)
	GetRCAsForSymptom(symptomID int64) ([]*SymptomRCA, error)
	GetSymptomsForRCA(rcaID int64) ([]*SymptomRCA, error)
}
