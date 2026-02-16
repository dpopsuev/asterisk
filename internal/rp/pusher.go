package rp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"asterisk/internal/investigate"
	"asterisk/internal/postinvest"
)

// Pusher implements postinvest.Pusher by calling the RP API to update defect
// types, then recording to the push store. Replaces rppush.RPPusher.
type Pusher struct {
	client  *Client
	project string
}

// NewPusher returns a Pusher that uses the given client and project.
func NewPusher(client *Client, project string) *Pusher {
	return &Pusher{client: client, project: project}
}

// Push reads the artifact, updates defect types in RP for each case, then
// records to the push store.
func (p *Pusher) Push(artifactPath string, store postinvest.PushStore, jiraTicketID, jiraLink string) error {
	data, err := os.ReadFile(artifactPath)
	if err != nil {
		return err
	}
	var a investigate.Artifact
	if err := json.Unmarshal(data, &a); err != nil {
		return err
	}

	ctx := context.Background()
	items := p.client.Project(p.project).Items()

	for _, itemID := range a.CaseIDs {
		if err := items.UpdateDefect(ctx, itemID, a.DefectType); err != nil {
			return fmt.Errorf("update item %d: %w", itemID, err)
		}
	}

	return store.RecordPushed(postinvest.PushedRecord{
		LaunchID:     a.LaunchID,
		CaseIDs:      a.CaseIDs,
		DefectType:   a.DefectType,
		JiraTicketID: jiraTicketID,
		JiraLink:     jiraLink,
	})
}
