package preinvest

// Store persists and retrieves envelopes by launch ID.
type Store interface {
	Save(launchID int, envelope *Envelope) error
	Get(launchID int) (*Envelope, error)
}

// MemStore is an in-memory store for tests. Create with NewMemStore.
type MemStore struct {
	envelopes map[int]*Envelope
}

// NewMemStore returns a new in-memory store (ready for Save/Get).
func NewMemStore() *MemStore {
	return &MemStore{envelopes: make(map[int]*Envelope)}
}

// Save stores the envelope by launch ID.
func (s *MemStore) Save(launchID int, envelope *Envelope) error {
	s.envelopes[launchID] = envelope
	return nil
}

// Get returns the envelope for the launch ID, or nil if not found.
func (s *MemStore) Get(launchID int) (*Envelope, error) {
	return s.envelopes[launchID], nil
}
