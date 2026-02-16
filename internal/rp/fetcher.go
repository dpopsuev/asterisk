package rp

import (
	"context"

	"asterisk/internal/preinvest"
)

// Fetcher implements preinvest.Fetcher by calling the RP API via the
// scope-based client. Replaces rpfetch.Fetcher.
type Fetcher struct {
	client  *Client
	project string
}

// NewFetcher returns a Fetcher that uses the given client and project.
func NewFetcher(client *Client, project string) *Fetcher {
	return &Fetcher{client: client, project: project}
}

// Fetch implements preinvest.Fetcher.
func (f *Fetcher) Fetch(launchID int) (*preinvest.Envelope, error) {
	return f.client.Project(f.project).FetchEnvelope(context.Background(), launchID)
}
