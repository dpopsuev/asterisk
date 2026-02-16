package rppush

import (
	"asterisk/internal/investigate"
	"asterisk/internal/postinvest"
	"encoding/json"
	"os"
)

// RPPusher implements postinvest.Pusher by calling the RP API to update defect type for each case, then recording to the push store.
type RPPusher struct {
	Client *Client
}

// NewRPPusher returns an RPPusher that uses the given client.
func NewRPPusher(client *Client) *RPPusher {
	return &RPPusher{Client: client}
}

// Push implements postinvest.Pusher: reads artifact, updates defect type in RP for each case_id, then records to store.
func (p *RPPusher) Push(artifactPath string, store postinvest.PushStore, jiraTicketID, jiraLink string) error {
	data, err := os.ReadFile(artifactPath)
	if err != nil {
		return err
	}
	var a investigate.Artifact
	if err := json.Unmarshal(data, &a); err != nil {
		return err
	}
	if len(a.CaseIDs) > 0 && p.Client != nil {
		if err := p.Client.UpdateItemsDefectType(a.CaseIDs, a.DefectType); err != nil {
			return err
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
