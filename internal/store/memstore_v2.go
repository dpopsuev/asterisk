package store

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

// v2 fields for MemStore. Extends the base MemStore with v2 entity storage.
// These will be initialized lazily on first v2 method call.

// memStoreV2 holds v2 entity maps. Embedded into MemStore via initV2().
type memStoreV2 struct {
	once            sync.Once
	suites          map[int64]*InvestigationSuite
	nextSuite       int64
	versions        map[int64]*Version
	versionsByLabel map[string]int64
	nextVersion     int64
	pipelines       map[int64]*Pipeline
	nextPipeline    int64
	launches        map[int64]*Launch
	nextLaunch      int64
	jobs            map[int64]*Job
	nextJob         int64
	casesV2         map[int64]*Case
	nextCaseV2      int64
	triages         map[int64]*Triage // keyed by case_id
	nextTriage      int64
	symptoms        map[int64]*Symptom
	symptomsByFP    map[string]int64 // fingerprint -> symptom id
	nextSymptom     int64
	rcasV2          map[int64]*RCA
	nextRCAV2       int64
	symptomRCAs     map[int64]*SymptomRCA
	nextSymptomRCA  int64
}

func (s *MemStore) v2() *memStoreV2 {
	if s.v2data == nil {
		s.v2data = &memStoreV2{}
	}
	s.v2data.once.Do(func() {
		s.v2data.suites = make(map[int64]*InvestigationSuite)
		s.v2data.versions = make(map[int64]*Version)
		s.v2data.versionsByLabel = make(map[string]int64)
		s.v2data.pipelines = make(map[int64]*Pipeline)
		s.v2data.launches = make(map[int64]*Launch)
		s.v2data.jobs = make(map[int64]*Job)
		s.v2data.casesV2 = make(map[int64]*Case)
		s.v2data.triages = make(map[int64]*Triage)
		s.v2data.symptoms = make(map[int64]*Symptom)
		s.v2data.symptomsByFP = make(map[string]int64)
		s.v2data.rcasV2 = make(map[int64]*RCA)
		s.v2data.symptomRCAs = make(map[int64]*SymptomRCA)
	})
	return s.v2data
}

func now() string { return time.Now().UTC().Format(time.RFC3339) }

// --- Suite ---

func (s *MemStore) CreateSuite(suite *InvestigationSuite) (int64, error) {
	if suite == nil {
		return 0, errors.New("suite is nil")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	d := s.v2()
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
	v, ok := s.v2().suites[id]
	if !ok {
		return nil, nil
	}
	cp := *v
	return &cp, nil
}

func (s *MemStore) ListSuites() ([]*InvestigationSuite, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]*InvestigationSuite, 0, len(s.v2().suites))
	for _, v := range s.v2().suites {
		cp := *v
		out = append(out, &cp)
	}
	return out, nil
}

func (s *MemStore) CloseSuite(id int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.v2().suites[id]
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
	d := s.v2()
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
	v, ok := s.v2().versions[id]
	if !ok {
		return nil, nil
	}
	cp := *v
	return &cp, nil
}

func (s *MemStore) GetVersionByLabel(label string) (*Version, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d := s.v2()
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
	out := make([]*Version, 0, len(s.v2().versions))
	for _, v := range s.v2().versions {
		cp := *v
		out = append(out, &cp)
	}
	return out, nil
}

// --- Pipeline ---

func (s *MemStore) CreatePipeline(p *Pipeline) (int64, error) {
	if p == nil {
		return 0, errors.New("pipeline is nil")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	d := s.v2()
	d.nextPipeline++
	cp := *p
	cp.ID = d.nextPipeline
	d.pipelines[cp.ID] = &cp
	return cp.ID, nil
}

func (s *MemStore) GetPipeline(id int64) (*Pipeline, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.v2().pipelines[id]
	if !ok {
		return nil, nil
	}
	cp := *v
	return &cp, nil
}

func (s *MemStore) ListPipelinesBySuite(suiteID int64) ([]*Pipeline, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []*Pipeline
	for _, p := range s.v2().pipelines {
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
	d := s.v2()
	d.nextLaunch++
	cp := *l
	cp.ID = d.nextLaunch
	d.launches[cp.ID] = &cp
	return cp.ID, nil
}

func (s *MemStore) GetLaunch(id int64) (*Launch, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.v2().launches[id]
	if !ok {
		return nil, nil
	}
	cp := *v
	return &cp, nil
}

func (s *MemStore) GetLaunchByRPID(pipelineID int64, rpLaunchID int) (*Launch, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, l := range s.v2().launches {
		if l.PipelineID == pipelineID && l.RPLaunchID == rpLaunchID {
			cp := *l
			return &cp, nil
		}
	}
	return nil, nil
}

func (s *MemStore) ListLaunchesByPipeline(pipelineID int64) ([]*Launch, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []*Launch
	for _, l := range s.v2().launches {
		if l.PipelineID == pipelineID {
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
	d := s.v2()
	d.nextJob++
	cp := *j
	cp.ID = d.nextJob
	d.jobs[cp.ID] = &cp
	return cp.ID, nil
}

func (s *MemStore) GetJob(id int64) (*Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.v2().jobs[id]
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
	for _, j := range s.v2().jobs {
		if j.LaunchID == launchID {
			cp := *j
			out = append(out, &cp)
		}
	}
	return out, nil
}

// --- Case v2 ---

func (s *MemStore) CreateCaseV2(c *Case) (int64, error) {
	if c == nil {
		return 0, errors.New("case is nil")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	d := s.v2()
	d.nextCaseV2++
	cp := *c
	cp.ID = d.nextCaseV2
	if cp.Status == "" {
		cp.Status = "open"
	}
	if cp.CreatedAt == "" {
		cp.CreatedAt = now()
	}
	cp.UpdatedAt = now()
	d.casesV2[cp.ID] = &cp
	return cp.ID, nil
}

func (s *MemStore) GetCaseV2(id int64) (*Case, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.v2().casesV2[id]
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
	for _, c := range s.v2().casesV2 {
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
	for _, c := range s.v2().casesV2 {
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
	c, ok := s.v2().casesV2[caseID]
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
	c, ok := s.v2().casesV2[caseID]
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
	d := s.v2()
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
	v, ok := s.v2().triages[caseID]
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
	d := s.v2()
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
	v, ok := s.v2().symptoms[id]
	if !ok {
		return nil, nil
	}
	cp := *v
	return &cp, nil
}

func (s *MemStore) GetSymptomByFingerprint(fingerprint string) (*Symptom, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d := s.v2()
	id, ok := d.symptomsByFP[fingerprint]
	if !ok {
		return nil, nil
	}
	cp := *d.symptoms[id]
	return &cp, nil
}

func (s *MemStore) FindSymptomCandidates(testName string) ([]*Symptom, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []*Symptom
	for _, sym := range s.v2().symptoms {
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
	sym, ok := s.v2().symptoms[id]
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
	out := make([]*Symptom, 0, len(s.v2().symptoms))
	for _, v := range s.v2().symptoms {
		cp := *v
		out = append(out, &cp)
	}
	return out, nil
}

func (s *MemStore) MarkDormantSymptoms(staleDays int) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cutoff := time.Now().UTC().AddDate(0, 0, -staleDays).Format(time.RFC3339)
	var count int64
	for _, sym := range s.v2().symptoms {
		if sym.Status == "active" && sym.LastSeenAt < cutoff {
			sym.Status = "dormant"
			count++
		}
	}
	return count, nil
}

// --- RCA v2 ---

func (s *MemStore) SaveRCAV2(rca *RCA) (int64, error) {
	if rca == nil {
		return 0, errors.New("rca is nil")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	d := s.v2()
	if rca.ID != 0 {
		if _, ok := d.rcasV2[rca.ID]; ok {
			cp := *rca
			d.rcasV2[rca.ID] = &cp
			return rca.ID, nil
		}
	}
	d.nextRCAV2++
	cp := *rca
	cp.ID = d.nextRCAV2
	if cp.Status == "" {
		cp.Status = "open"
	}
	if cp.CreatedAt == "" {
		cp.CreatedAt = now()
	}
	d.rcasV2[cp.ID] = &cp
	return cp.ID, nil
}

func (s *MemStore) GetRCAV2(id int64) (*RCA, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.v2().rcasV2[id]
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
	for _, r := range s.v2().rcasV2 {
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
	r, ok := s.v2().rcasV2[id]
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
	d := s.v2()
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
	for _, link := range s.v2().symptomRCAs {
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
	for _, link := range s.v2().symptomRCAs {
		if link.RCAID == rcaID {
			cp := *link
			out = append(out, &cp)
		}
	}
	return out, nil
}
