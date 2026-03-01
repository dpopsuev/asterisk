package store

import (
	"errors"
	"sync"

	"github.com/dpopsuev/origami/adapters/rp"
)

// MemStore is an in-memory Store for tests. Implements Store.
type MemStore struct {
	mu        sync.Mutex
	envelopes map[int]*rp.Envelope
	v2data    *memStoreV2 // lazy-initialized v2 entity storage
}

// NewMemStore returns a new in-memory Store.
func NewMemStore() *MemStore {
	return &MemStore{
		envelopes: make(map[int]*rp.Envelope),
	}
}

// LinkCaseToRCA implements Store.
func (s *MemStore) LinkCaseToRCA(caseID, rcaID int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.v2data != nil {
		if c, ok := s.v2data.casesV2[caseID]; ok {
			c.RCAID = rcaID
		}
	}
	return nil
}

// ListRCAs implements Store.
func (s *MemStore) ListRCAs() ([]*RCA, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d := s.v2()
	out := make([]*RCA, 0, len(d.rcasV2))
	for _, r := range d.rcasV2 {
		cp := *r
		out = append(out, &cp)
	}
	return out, nil
}

// SaveEnvelope implements Store.
func (s *MemStore) SaveEnvelope(launchID int, env *rp.Envelope) error {
	if env == nil {
		return errors.New("envelope is nil")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.envelopes[launchID] = env
	return nil
}

// GetEnvelope implements Store.
func (s *MemStore) GetEnvelope(launchID int) (*rp.Envelope, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.envelopes[launchID], nil
}
