package store

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

// v2 fields for MemStore. Extends the base MemStore with v2 entity storage.
// These will be initialized lazily on first v2 method call.

// memStoreData holds v2 entity maps. Embedded into MemStore via initV2().
type memStoreData struct {
	once            sync.Once
	suites          map[int64]*InvestigationSuite
	nextSuite       int64
	versions        map[int64]*Version
	versionsByLabel map[string]int64
	nextVersion     int64
	circuits       map[int64]*Circuit
	nextCircuit    int64
	launches        map[int64]*Launch
	nextLaunch      int64
	jobs            map[int64]*Job
	nextJob         int64
	cases         map[int64]*Case
	nextCase      int64
	triages         map[int64]*Triage // keyed by case_id
	nextTriage      int64
	symptoms        map[int64]*Symptom
	symptomsByFP    map[string]int64 // fingerprint -> symptom id
	nextSymptom     int64
	rcas          map[int64]*RCA
	nextRCA       int64
	symptomRCAs     map[int64]*SymptomRCA
	nextSymptomRCA  int64
}

func (s *MemStore) ensureData() *memStoreData {
	if s.data == nil {
		s.data = &memStoreData{}
	}
	s.data.once.Do(func() {
		s.data.suites = make(map[int64]*InvestigationSuite)
		s.data.versions = make(map[int64]*Version)
		s.data.versionsByLabel = make(map[string]int64)
		s.data.circuits = make(map[int64]*Circuit)
		s.data.launches = make(map[int64]*Launch)
		s.data.jobs = make(map[int64]*Job)
		s.data.cases = make(map[int64]*Case)
		s.data.triages = make(map[int64]*Triage)
		s.data.symptoms = make(map[int64]*Symptom)
		s.data.symptomsByFP = make(map[string]int64)
		s.data.rcas = make(map[int64]*RCA)
		s.data.symptomRCAs = make(map[int64]*SymptomRCA)
	})
	return s.data
}

func now() string { return time.Now().UTC().Format(time.RFC3339) }

// --- Suite ---

func (s *MemStore) CreateSuite(suite *InvestigationSuite) (int64, error) {
	if suite == nil {
		return 0, errors.New("suite is nil")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	d := s.ensureData()
	d.nextSuite++
	cp := *suite
	cp.ID = d.nextSuite
	if cp.Status == "" {
		cp.Status = "open"
	}
	if cp.CreatedAt == "" {
		cp.CreatedAt = now()
	}
	d.suites[cp.ID] = &cp
	return cp.ID, nil
}

func (s *MemStore) GetSuite(id int64) (*InvestigationSuite, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.ensureData().suites[id]
	if !ok {
		return nil, nil
	}
	cp := *v
	return &cp, nil
}

func (s *MemStore) ListSuites() ([]*InvestigationSuite, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]*InvestigationSuite, 0, len(s.ensureData().suites))
	for _, v := range s.ensureData().suites {
		cp := *v
		out = append(out, &cp)
	}
	return out, nil
}

func (s *MemStore) CloseSuite(id int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.ensureData().suites[id]
	if !ok {
		return errors.New("suite not found")
	}
	v.Status = "closed"
	v.ClosedAt = now()
	return nil
}

// --- Version ---

func (s *MemStore) CreateVersion(ver *Version) (int64, error) {
	if ver == nil {
		return 0, errors.New("version is nil")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	d := s.ensureData()
	if _, exists := d.versionsByLabel[ver.Label]; exists {
		return 0, fmt.Errorf("version label %q already exists", ver.Label)
	}
	d.nextVersion++
	cp := *ver
	cp.ID = d.nextVersion
	d.versions[cp.ID] = &cp
	d.versionsByLabel[cp.Label] = cp.ID
	return cp.ID, nil
}

func (s *MemStore) GetVersion(id int64) (*Version, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.ensureData().versions[id]
	if !ok {
		return nil, nil
	}
	cp := *v
	return &cp, nil
}

func (s *MemStore) GetVersionByLabel(label string) (*Version, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d := s.ensureData()
	id, ok := d.versionsByLabel[label]
	if !ok {
		return nil, nil
	}
	cp := *d.versions[id]
	return &cp, nil
}

func (s *MemStore) ListVersions() ([]*Version, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]*Version, 0, len(s.ensureData().versions))
	for _, v := range s.ensureData().versions {
		cp := *v
		out = append(out, &cp)
	}
	return out, nil
}

// --- Circuit ---

func (s *MemStore) CreateCircuit(p *Circuit) (int64, error) {
	if p == nil {
		return 0, errors.New("circuit is nil")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	d := s.ensureData()
	d.nextCircuit++
	cp := *p
	cp.ID = d.nextCircuit
	d.circuits[cp.ID] = &cp
	return cp.ID, nil
}

func (s *MemStore) GetCircuit(id int64) (*Circuit, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.ensureData().circuits[id]
	if !ok {
		return nil, nil
	}
	cp := *v
	return &cp, nil
}

func (s *MemStore) ListCircuitsBySuite(suiteID int64) ([]*Circuit, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []*Circuit
	for _, p := range s.ensureData().circuits {
		if p.SuiteID == suiteID {
			cp := *p
			out = append(out, &cp)
		}
	}
	return out, nil
}

// --- Launch ---

func (s *MemStore) CreateLaunch(l *Launch) (int64, error) {
	if l == nil {
		return 0, errors.New("launch is nil")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	d := s.ensureData()
	d.nextLaunch++
	cp := *l
	cp.ID = d.nextLaunch
	d.launches[cp.ID] = &cp
	return cp.ID, nil
}

func (s *MemStore) GetLaunch(id int64) (*Launch, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.ensureData().launches[id]
	if !ok {
		return nil, nil
	}
	cp := *v
	return &cp, nil
}

func (s *MemStore) GetLaunchByRPID(circuitID int64, rpLaunchID int) (*Launch, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, l := range s.ensureData().launches {
		if l.CircuitID == circuitID && l.RPLaunchID == rpLaunchID {
			cp := *l
			return &cp, nil
		}
	}
	return nil, nil
}

func (s *MemStore) ListLaunchesByCircuit(circuitID int64) ([]*Launch, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []*Launch
	for _, l := range s.ensureData().launches {
		if l.CircuitID == circuitID {
			cp := *l
			out = append(out, &cp)
		}
	}
	return out, nil
}

// --- Job ---

func (s *MemStore) CreateJob(j *Job) (int64, error) {
	if j == nil {
		return 0, errors.New("job is nil")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	d := s.ensureData()
	d.nextJob++
	cp := *j
	cp.ID = d.nextJob
	d.jobs[cp.ID] = &cp
	return cp.ID, nil
}

func (s *MemStore) GetJob(id int64) (*Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.ensureData().jobs[id]
	if !ok {
		return nil, nil
	}
	cp := *v
	return &cp, nil
}

func (s *MemStore) ListJobsByLaunch(launchID int64) ([]*Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []*Job
	for _, j := range s.ensureData().jobs {
		if j.LaunchID == launchID {
			cp := *j
			out = append(out, &cp)
		}
	}
	return out, nil
}

// --- Case v2 ---

func (s *MemStore) CreateCase(c *Case) (int64, error) {
	if c == nil {
		return 0, errors.New("case is nil")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	d := s.ensureData()
	d.nextCase++
	cp := *c
	cp.ID = d.nextCase
	if cp.Status == "" {
		cp.Status = "open"
	}
	if cp.CreatedAt == "" {
		cp.CreatedAt = now()
	}
	cp.UpdatedAt = now()
	d.cases[cp.ID] = &cp
	return cp.ID, nil
}

func (s *MemStore) GetCase(id int64) (*Case, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.ensureData().cases[id]
	if !ok {
		return nil, nil
	}
	cp := *v
	return &cp, nil
}

func (s *MemStore) ListCasesByJob(jobID int64) ([]*Case, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []*Case
	for _, c := range s.ensureData().cases {
		if c.JobID == jobID {
			cp := *c
			out = append(out, &cp)
		}
	}
	return out, nil
}

func (s *MemStore) ListCasesBySymptom(symptomID int64) ([]*Case, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []*Case
	for _, c := range s.ensureData().cases {
		if c.SymptomID == symptomID {
			cp := *c
			out = append(out, &cp)
		}
	}
	return out, nil
}

func (s *MemStore) UpdateCaseStatus(caseID int64, status string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.ensureData().cases[caseID]
	if !ok {
		return errors.New("case not found")
	}
	c.Status = status
	c.UpdatedAt = now()
	return nil
}

func (s *MemStore) LinkCaseToSymptom(caseID, symptomID int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.ensureData().cases[caseID]
	if !ok {
		return errors.New("case not found")
	}
	c.SymptomID = symptomID
	c.UpdatedAt = now()
	return nil
}

// --- Triage ---

func (s *MemStore) CreateTriage(t *Triage) (int64, error) {
	if t == nil {
		return 0, errors.New("triage is nil")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	d := s.ensureData()
	d.nextTriage++
	cp := *t
	cp.ID = d.nextTriage
	if cp.CreatedAt == "" {
		cp.CreatedAt = now()
	}
	d.triages[cp.CaseID] = &cp
	return cp.ID, nil
}

func (s *MemStore) GetTriageByCase(caseID int64) (*Triage, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.ensureData().triages[caseID]
	if !ok {
		return nil, nil
	}
	cp := *v
	return &cp, nil
}

// --- Symptom ---

func (s *MemStore) CreateSymptom(sym *Symptom) (int64, error) {
	if sym == nil {
		return 0, errors.New("symptom is nil")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	d := s.ensureData()
	if _, exists := d.symptomsByFP[sym.Fingerprint]; exists {
		return 0, fmt.Errorf("symptom with fingerprint %q already exists", sym.Fingerprint)
	}
	d.nextSymptom++
	cp := *sym
	cp.ID = d.nextSymptom
	if cp.Status == "" {
		cp.Status = "active"
	}
	if cp.OccurrenceCount == 0 {
		cp.OccurrenceCount = 1
	}
	if cp.FirstSeenAt == "" {
		cp.FirstSeenAt = now()
	}
	if cp.LastSeenAt == "" {
		cp.LastSeenAt = cp.FirstSeenAt
	}
	d.symptoms[cp.ID] = &cp
	d.symptomsByFP[cp.Fingerprint] = cp.ID
	return cp.ID, nil
}

func (s *MemStore) GetSymptom(id int64) (*Symptom, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.ensureData().symptoms[id]
	if !ok {
		return nil, nil
	}
	cp := *v
	return &cp, nil
}

func (s *MemStore) GetSymptomByFingerprint(fingerprint string) (*Symptom, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d := s.ensureData()
	id, ok := d.symptomsByFP[fingerprint]
	if !ok {
		return nil, nil
	}
	cp := *d.symptoms[id]
	return &cp, nil
}

func (s *MemStore) FindSymptomCandidates(testName string) ([]*Symptom, error) {
	// Do not match on empty test names — this would return all symptoms with
	// empty names, causing false recall hits during calibration.
	if testName == "" {
		return nil, nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []*Symptom
	for _, sym := range s.ensureData().symptoms {
		if sym.Name == testName {
			cp := *sym
			out = append(out, &cp)
		}
	}
	return out, nil
}

func (s *MemStore) UpdateSymptomSeen(id int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	sym, ok := s.ensureData().symptoms[id]
	if !ok {
		return errors.New("symptom not found")
	}
	sym.OccurrenceCount++
	sym.LastSeenAt = now()
	if sym.Status == "dormant" {
		sym.Status = "active"
	}
	return nil
}

func (s *MemStore) ListSymptoms() ([]*Symptom, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]*Symptom, 0, len(s.ensureData().symptoms))
	for _, v := range s.ensureData().symptoms {
		cp := *v
		out = append(out, &cp)
	}
	return out, nil
}

// SnapshotSymptoms returns a copy of all current symptoms for isolation
// during parallel triage. The caller gets a point-in-time snapshot that
// won't be affected by concurrent writes.
func (s *MemStore) SnapshotSymptoms() []*Symptom {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]*Symptom, 0, len(s.ensureData().symptoms))
	for _, v := range s.ensureData().symptoms {
		cp := *v
		out = append(out, &cp)
	}
	return out
}

func (s *MemStore) MarkDormantSymptoms(staleDays int) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cutoff := time.Now().UTC().AddDate(0, 0, -staleDays).Format(time.RFC3339)
	var count int64
	for _, sym := range s.ensureData().symptoms {
		if sym.Status == "active" && sym.LastSeenAt < cutoff {
			sym.Status = "dormant"
			count++
		}
	}
	return count, nil
}

// --- RCA v2 ---

func (s *MemStore) SaveRCA(rca *RCA) (int64, error) {
	if rca == nil {
		return 0, errors.New("rca is nil")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	d := s.ensureData()
	if rca.ID != 0 {
		if _, ok := d.rcas[rca.ID]; ok {
			cp := *rca
			d.rcas[rca.ID] = &cp
			return rca.ID, nil
		}
	}
	d.nextRCA++
	cp := *rca
	cp.ID = d.nextRCA
	if cp.Status == "" {
		cp.Status = "open"
	}
	if cp.CreatedAt == "" {
		cp.CreatedAt = now()
	}
	d.rcas[cp.ID] = &cp
	return cp.ID, nil
}

func (s *MemStore) GetRCA(id int64) (*RCA, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.ensureData().rcas[id]
	if !ok {
		return nil, nil
	}
	cp := *v
	return &cp, nil
}

func (s *MemStore) ListRCAsByStatus(status string) ([]*RCA, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []*RCA
	for _, r := range s.ensureData().rcas {
		if r.Status == status {
			cp := *r
			out = append(out, &cp)
		}
	}
	return out, nil
}

func (s *MemStore) UpdateRCAStatus(id int64, status string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.ensureData().rcas[id]
	if !ok {
		return errors.New("rca not found")
	}
	r.Status = status
	switch status {
	case "resolved":
		r.ResolvedAt = now()
	case "verified":
		r.VerifiedAt = now()
	case "archived":
		r.ArchivedAt = now()
	case "open":
		r.ResolvedAt = ""
		r.VerifiedAt = ""
	}
	return nil
}

// --- SymptomRCA ---

func (s *MemStore) LinkSymptomToRCA(link *SymptomRCA) (int64, error) {
	if link == nil {
		return 0, errors.New("link is nil")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	d := s.ensureData()
	// Check for duplicate
	for _, existing := range d.symptomRCAs {
		if existing.SymptomID == link.SymptomID && existing.RCAID == link.RCAID {
			return 0, errors.New("symptom-rca link already exists")
		}
	}
	d.nextSymptomRCA++
	cp := *link
	cp.ID = d.nextSymptomRCA
	if cp.LinkedAt == "" {
		cp.LinkedAt = now()
	}
	d.symptomRCAs[cp.ID] = &cp
	return cp.ID, nil
}

func (s *MemStore) GetRCAsForSymptom(symptomID int64) ([]*SymptomRCA, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []*SymptomRCA
	for _, link := range s.ensureData().symptomRCAs {
		if link.SymptomID == symptomID {
			cp := *link
			out = append(out, &cp)
		}
	}
	return out, nil
}

func (s *MemStore) GetSymptomsForRCA(rcaID int64) ([]*SymptomRCA, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []*SymptomRCA
	for _, link := range s.ensureData().symptomRCAs {
		if link.RCAID == rcaID {
			cp := *link
			out = append(out, &cp)
		}
	}
	return out, nil
}
