package store

import "asterisk/internal/preinvest"

// DefaultDBPath is the default relative path for the SQLite DB (per-workspace).
// Resolve against cwd or workspace root; Open() creates the parent dir (e.g. .asterisk).
const DefaultDBPath = ".asterisk/asterisk.db"

// Case is one investigation case (one failed test). Keyed by launch + item.
type Case struct {
	ID       int64
	LaunchID int
	ItemID   int   // failure (test item) id from envelope
	RCAID    int64 // nullable: 0 means not linked
}

// RCA is a root-cause analysis record. Many cases can link to one RCA.
type RCA struct {
	ID            int64
	Title         string
	Description   string
	DefectType    string
	JiraTicketID  string
	JiraLink      string
}

// Store is the persistence facade: cases, RCAs, and envelope storage.
// Domain and CLI use only this interface; implementation is SQLite or in-memory.
type Store interface {
	// Cases
	CreateCase(launchID, itemID int) (caseID int64, err error)
	GetCase(caseID int64) (*Case, error)
	ListCasesByLaunch(launchID int) ([]*Case, error)
	// RCAs
	SaveRCA(rca *RCA) (rcaID int64, err error)
	LinkCaseToRCA(caseID, rcaID int64) error
	GetRCA(rcaID int64) (*RCA, error)
	ListRCAs() ([]*RCA, error)
	// Envelope (for pre-investigation; same DB can hold envelope by launch ID)
	SaveEnvelope(launchID int, env *preinvest.Envelope) error
	GetEnvelope(launchID int) (*preinvest.Envelope, error)
}
