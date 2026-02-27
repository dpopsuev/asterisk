package store

import (
	"errors"
	"sync"

	"asterisk/adapters/rp"
)

// MemStore is an in-memory Store for tests. Implements Store.
type MemStore struct {
	mu        sync.Mutex
	nextCase  int64
	nextRCA   int64
	cases     map[int64]*Case
	casesBy   map[int][]int64 // launchID -> case IDs
	rcas      map[int64]*RCA
	envelopes map[int]*rp.Envelope
	v2data    *memStoreV2 // lazy-initialized v2 entity storage
}

// NewMemStore returns a new in-memory Store.
func NewMemStore() *MemStore {
	return &MemStore{
		cases:     make(map[int64]*Case),
		casesBy:   make(map[int][]int64),
		rcas:      make(map[int64]*RCA),
		envelopes: make(map[int]*rp.Envelope),
	}
}

// CreateCase implements Store.
func (s *MemStore) CreateCase(launchID, itemID int) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nextCase++
	id := s.nextCase
	c := &Case{ID: id, LaunchID: int64(launchID), RPItemID: itemID, RCAID: 0}
	s.cases[id] = c
	s.casesBy[launchID] = append(s.casesBy[launchID], id)
	return id, nil
}

// GetCase implements Store.
func (s *MemStore) GetCase(caseID int64) (*Case, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.cases[caseID]
	if !ok {
		return nil, nil
	}
	cp := *c
	return &cp, nil
}

// ListCasesByLaunch implements Store.
func (s *MemStore) ListCasesByLaunch(launchID int) ([]*Case, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ids := s.casesBy[launchID]
	out := make([]*Case, 0, len(ids))
	for _, id := range ids {
		if c, ok := s.cases[id]; ok {
			cp := *c
			out = append(out, &cp)
		}
	}
	return out, nil
}

// SaveRCA implements Store.
func (s *MemStore) SaveRCA(rca *RCA) (int64, error) {
	if rca == nil {
		return 0, errors.New("rca is nil")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if rca.ID != 0 {
		if _, ok := s.rcas[rca.ID]; ok {
			r := *rca
			s.rcas[rca.ID] = &r
			return rca.ID, nil
		}
	}
	s.nextRCA++
	id := s.nextRCA
	r := *rca
	r.ID = id
	s.rcas[id] = &r
	return id, nil
}

// LinkCaseToRCA implements Store. Updates both v1 and v2 case maps.
func (s *MemStore) LinkCaseToRCA(caseID, rcaID int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if c, ok := s.cases[caseID]; ok {
		c.RCAID = rcaID
	}
	if s.v2data != nil {
		if c, ok := s.v2data.casesV2[caseID]; ok {
			c.RCAID = rcaID
		}
	}
	return nil
}

// GetRCA implements Store.
func (s *MemStore) GetRCA(rcaID int64) (*RCA, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.rcas[rcaID]
	if !ok {
		return nil, nil
	}
	cp := *r
	return &cp, nil
}

// ListRCAs implements Store.
func (s *MemStore) ListRCAs() ([]*RCA, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]*RCA, 0, len(s.rcas))
	for _, r := range s.rcas {
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
