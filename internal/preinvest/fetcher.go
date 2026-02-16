package preinvest

// Fetcher returns an envelope for a launch ID (e.g. from RP API or stub).
type Fetcher interface {
	Fetch(launchID int) (*Envelope, error)
}

// StubFetcher returns a fixed envelope for any launch ID (mock; no HTTP).
// Use in tests or when wiring with fixture data.
type StubFetcher struct {
	Env *Envelope
}

// Fetch implements Fetcher by returning the fixed envelope.
func (f *StubFetcher) Fetch(launchID int) (*Envelope, error) {
	return f.Env, nil
}

// NewStubFetcher returns a Fetcher that always returns env.
func NewStubFetcher(env *Envelope) *StubFetcher {
	return &StubFetcher{Env: env}
}
