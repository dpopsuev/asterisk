package rp

import (
	"context"
	"fmt"
	"strconv"

	"asterisk/internal/preinvest"
)

// FetchEnvelope fetches a launch and its failed test items, mapping them
// into a preinvest.Envelope. This is the convenience method that replaces
// rpfetch.Client.FetchEnvelope.
func (p *ProjectScope) FetchEnvelope(ctx context.Context, launchID int) (*preinvest.Envelope, error) {
	launch, err := p.Launches().Get(ctx, launchID)
	if err != nil {
		return nil, fmt.Errorf("fetch envelope: get launch: %w", err)
	}

	items, err := p.Items().ListAll(ctx,
		WithLaunchID(launchID),
		WithStatus("FAILED"),
	)
	if err != nil {
		return nil, fmt.Errorf("fetch envelope: list items: %w", err)
	}

	env := &preinvest.Envelope{
		RunID:       strconv.Itoa(launch.ID),
		LaunchUUID:  launch.UUID,
		Name:        launch.Name,
		FailureList: make([]preinvest.FailureItem, 0, len(items)),
	}

	for _, it := range items {
		path := it.Path
		if path == "" {
			path = strconv.Itoa(it.ID)
		}

		fi := preinvest.FailureItem{
			ID:     it.ID,
			UUID:   it.UUID,
			Name:   it.Name,
			Type:   it.Type,
			Status: it.Status,
			Path:   path,
			// Enriched fields from TestItemResource
			CodeRef:     it.CodeRef,
			Description: it.Description,
			ParentID:    it.Parent,
		}

		if it.Issue != nil {
			fi.IssueType = it.Issue.IssueType
			fi.IssueComment = it.Issue.Comment
			fi.AutoAnalyzed = it.Issue.AutoAnalyzed
		}

		env.FailureList = append(env.FailureList, fi)
	}

	return env, nil
}
