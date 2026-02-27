package rp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

)

// Pusher implements DefectPusher by calling the RP API to update defect
// types with attribution, then recording to the push store.
type Pusher struct {
	client      *Client
	project     string
	submittedBy string
}

// NewPusher returns a Pusher that uses the given client, project, and submitter name.
func NewPusher(client *Client, project string, submittedBy string) *Pusher {
	return &Pusher{client: client, project: project, submittedBy: submittedBy}
}

// Push reads the artifact, updates defect types in RP for each case with an
// attribution comment, then records to the push store.
func (p *Pusher) Push(artifactPath string, store PushStore, jiraTicketID, jiraLink string) error {
	data, err := os.ReadFile(artifactPath)
	if err != nil {
		return err
	}
	var a pushArtifact
	if err := json.Unmarshal(data, &a); err != nil {
		return err
	}

	ctx := context.Background()
	items := p.client.Project(p.project).Items()

	comment := p.buildComment(a.RCAMessage, a.EvidenceRefs)

	defs := make([]IssueDefinition, 0, len(a.CaseIDs))
	for _, itemID := range a.CaseIDs {
		defs = append(defs, IssueDefinition{
			TestItemID: itemID,
			Issue: Issue{
				IssueType: a.DefectType,
				Comment:   comment,
			},
		})
	}

	if err := items.UpdateDefectBulk(ctx, defs); err != nil {
		return fmt.Errorf("update defects: %w", err)
	}

	return store.RecordPushed(PushedRecord{
		LaunchID:     a.LaunchID,
		CaseIDs:      a.CaseIDs,
		DefectType:   a.DefectType,
		JiraTicketID: jiraTicketID,
		JiraLink:     jiraLink,
	})
}

func (p *Pusher) buildComment(rcaMessage string, evidenceRefs []string) string {
	var parts []string

	if rcaMessage != "" {
		parts = append(parts, rcaMessage)
	}

	commitLinks := filterCommitLinks(evidenceRefs)
	if len(commitLinks) > 0 {
		parts = append(parts, "**Suspected commit(s):**\n"+strings.Join(commitLinks, "\n"))
	}

	attribution := "Analysis was submitted"
	if p.submittedBy != "" {
		attribution += " by " + p.submittedBy
	}
	attribution += " (via Asterisk)"
	parts = append(parts, attribution)

	return strings.Join(parts, "\n\n---\n")
}

func filterCommitLinks(refs []string) []string {
	var links []string
	for _, ref := range refs {
		if isCommitLink(ref) {
			links = append(links, ref)
		}
	}
	return links
}

func isCommitLink(ref string) bool {
	if !strings.HasPrefix(ref, "http://") && !strings.HasPrefix(ref, "https://") {
		return false
	}
	lower := strings.ToLower(ref)
	return strings.Contains(lower, "/commit/") ||
		strings.Contains(lower, "/commits/") ||
		strings.Contains(lower, "/-/commit/")
}
