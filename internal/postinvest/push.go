package postinvest

import (
	"asterisk/internal/investigate"
	"encoding/json"
	"os"
)

// Pusher pushes artifact content to a store (mock or real RP). Mock implementation only.
type Pusher interface {
	Push(artifactPath string, store PushStore, jiraTicketID, jiraLink string) error
}

// DefaultPusher is the mock pusher used in tests and wiring.
type DefaultPusher struct{}

// Push implements Pusher.
func (DefaultPusher) Push(artifactPath string, store PushStore, jiraTicketID, jiraLink string) error {
	return Push(artifactPath, store, jiraTicketID, jiraLink)
}

// Push reads the artifact at path and records defect type (and optional Jira fields) to the store.
// Artifact format: same as investigate.Artifact (JSON from mock investigation).
// Contract: .cursor/contracts/mock-post-investigation.md
func Push(artifactPath string, store PushStore, jiraTicketID, jiraLink string) error {
	data, err := os.ReadFile(artifactPath)
	if err != nil {
		return err
	}
	var a investigate.Artifact
	if err := json.Unmarshal(data, &a); err != nil {
		return err
	}
	return store.RecordPushed(PushedRecord{
		LaunchID:     a.LaunchID,
		CaseIDs:      a.CaseIDs,
		DefectType:   a.DefectType,
		JiraTicketID: jiraTicketID,
		JiraLink:     jiraLink,
	})
}
