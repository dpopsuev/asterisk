// Package ingest implements the data ingestion pipeline nodes for
// automatically discovering new CI failures from ReportPortal and
// creating candidate cases for human review.
package ingest

import (
	"fmt"
	"time"
)

// IngestConfig provides configuration for the ingestion pipeline,
// injected via walker context at key "config".
type IngestConfig struct {
	RPProject    string
	LookbackDays int
	ScenarioPath string
	DatasetDir   string
	CandidateDir string
}

// LaunchInfo summarizes an RP launch for the pipeline.
type LaunchInfo struct {
	ID         int       `json:"id"`
	UUID       string    `json:"uuid"`
	Name       string    `json:"name"`
	Number     int       `json:"number"`
	Status     string    `json:"status"`
	StartTime  time.Time `json:"start_time"`
	FailedCount int      `json:"failed_count"`
}

// FailureInfo represents a parsed test failure from an RP launch.
type FailureInfo struct {
	LaunchID     int    `json:"launch_id"`
	LaunchName   string `json:"launch_name"`
	ItemID       int    `json:"item_id"`
	ItemUUID     string `json:"item_uuid"`
	TestName     string `json:"test_name"`
	Status       string `json:"status"`
	ErrorMessage string `json:"error_message"`
	IssueType    string `json:"issue_type,omitempty"`
	AutoAnalyzed bool   `json:"auto_analyzed,omitempty"`
}

// DedupKey generates the deduplication key for a failure.
func (f *FailureInfo) DedupKey(project string) string {
	return fmt.Sprintf("%s:%d:%d", project, f.LaunchID, f.ItemID)
}

// SymptomMatch holds the result of matching a failure against the symptom catalog.
type SymptomMatch struct {
	FailureInfo
	SymptomID   string  `json:"symptom_id,omitempty"`
	SymptomName string  `json:"symptom_name,omitempty"`
	Confidence  float64 `json:"confidence"`
	Matched     bool    `json:"matched"`
}

// CandidateCase is a candidate case ready for human review.
type CandidateCase struct {
	ID           string    `json:"id"`
	LaunchID     int       `json:"launch_id"`
	ItemID       int       `json:"item_id"`
	TestName     string    `json:"test_name"`
	ErrorMessage string    `json:"error_message"`
	SymptomID    string    `json:"symptom_id,omitempty"`
	SymptomName  string    `json:"symptom_name,omitempty"`
	Status       string    `json:"status"` // "candidate" or "verified"
	CreatedAt    time.Time `json:"created_at"`
	DedupKey     string    `json:"dedup_key"`
}

// IngestSummary is the output of the notify node.
type IngestSummary struct {
	LaunchesFetched    int `json:"launches_fetched"`
	FailuresParsed     int `json:"failures_parsed"`
	SymptomsMatched    int `json:"symptoms_matched"`
	Deduplicated       int `json:"deduplicated"`
	CandidatesCreated  int `json:"candidates_created"`
}

// LaunchFetcher abstracts the RP API for listing launches.
type LaunchFetcher interface {
	FetchLaunches(project string, since time.Time) ([]LaunchInfo, error)
	FetchFailures(launchID int) ([]FailureInfo, error)
}
