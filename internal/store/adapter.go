package store

import "asterisk/internal/preinvest"

// PreinvestStoreAdapter adapts a Store (with SaveEnvelope/GetEnvelope) to preinvest.Store.
// Use so preinvest.FetchAndSave and investigate.Analyze can use the same SQLite store.
type PreinvestStoreAdapter struct {
	Store Store
}

// Save implements preinvest.Store.
func (a *PreinvestStoreAdapter) Save(launchID int, envelope *preinvest.Envelope) error {
	return a.Store.SaveEnvelope(launchID, envelope)
}

// Get implements preinvest.Store.
func (a *PreinvestStoreAdapter) Get(launchID int) (*preinvest.Envelope, error) {
	return a.Store.GetEnvelope(launchID)
}
