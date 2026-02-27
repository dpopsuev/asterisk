package store

import "github.com/dpopsuev/origami/adapters/rp"

// EnvelopeStoreAdapter adapts a Store (with SaveEnvelope/GetEnvelope) to rp.EnvelopeStore.
type EnvelopeStoreAdapter struct {
	Store Store
}

// Save implements rp.EnvelopeStore.
func (a *EnvelopeStoreAdapter) Save(launchID int, envelope *rp.Envelope) error {
	return a.Store.SaveEnvelope(launchID, envelope)
}

// Get implements rp.EnvelopeStore.
func (a *EnvelopeStoreAdapter) Get(launchID int) (*rp.Envelope, error) {
	return a.Store.GetEnvelope(launchID)
}
