package rpfetch

import "asterisk/internal/preinvest"

// Fetcher implements preinvest.Fetcher by calling the RP API.
type Fetcher struct {
	Client *Client
}

// NewFetcher returns a Fetcher that uses the given client.
func NewFetcher(client *Client) *Fetcher {
	return &Fetcher{Client: client}
}

// Fetch implements preinvest.Fetcher: fetches launch and failed items from RP and returns an Envelope.
func (f *Fetcher) Fetch(launchID int) (*preinvest.Envelope, error) {
	return f.Client.FetchEnvelope(launchID)
}
