package store

import (
	"database/sql"
	"path/filepath"
	"testing"
)

// TestSqlStoreV2_FullHierarchy tests the complete v2 entity tree:
// Suite → Pipeline (+ Version) → Launch → Job → Case → Triage
// Plus global knowledge: Symptom, RCA, SymptomRCA.
func TestSqlStoreV2_FullHierarchy(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "v2.db")
	s, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer s.Close()

	// --- Suite ---
	suiteID, err := s.CreateSuite(&InvestigationSuite{Name: "PTP Feb 2026", Description: "Regression analysis"})
	if err != nil {
		t.Fatalf("CreateSuite: %v", err)
	}
	suite, err := s.GetSuite(suiteID)
	if err != nil || suite == nil || suite.Name != "PTP Feb 2026" || suite.Status != "open" {
		t.Fatalf("GetSuite: got %+v err %v", suite, err)
	}
	suites, err := s.ListSuites()
	if err != nil || len(suites) != 1 {
		t.Fatalf("ListSuites: got %d err %v", len(suites), err)
	}

	// --- Version ---
	verID, err := s.CreateVersion(&Version{Label: "4.21", OCPBuild: "4.21.2"})
	if err != nil {
		t.Fatalf("CreateVersion: %v", err)
	}
	ver, err := s.GetVersion(verID)
	if err != nil || ver == nil || ver.Label != "4.21" {
		t.Fatalf("GetVersion: got %+v err %v", ver, err)
	}
	verByLabel, err := s.GetVersionByLabel("4.21")
	if err != nil || verByLabel == nil || verByLabel.ID != verID {
		t.Fatalf("GetVersionByLabel: got %+v err %v", verByLabel, err)
	}

	// --- Pipeline ---
	pipID, err := s.CreatePipeline(&Pipeline{
		SuiteID: suiteID, VersionID: verID,
		Name: "telco-ft-ran-ptp-4.21", RPLaunchID: 33195, Status: "FAILED",
	})
	if err != nil {
		t.Fatalf("CreatePipeline: %v", err)
	}
	pip, err := s.GetPipeline(pipID)
	if err != nil || pip == nil || pip.Name != "telco-ft-ran-ptp-4.21" {
		t.Fatalf("GetPipeline: got %+v err %v", pip, err)
	}
	pips, err := s.ListPipelinesBySuite(suiteID)
	if err != nil || len(pips) != 1 {
		t.Fatalf("ListPipelinesBySuite: got %d err %v", len(pips), err)
	}

	// --- Launch ---
	launchID, err := s.CreateLaunch(&Launch{
		PipelineID: pipID, RPLaunchID: 33195, Name: "test-launch", Status: "FAILED",
	})
	if err != nil {
		t.Fatalf("CreateLaunch: %v", err)
	}
	launch, err := s.GetLaunch(launchID)
	if err != nil || launch == nil || launch.RPLaunchID != 33195 {
		t.Fatalf("GetLaunch: got %+v err %v", launch, err)
	}
	launchByRP, err := s.GetLaunchByRPID(pipID, 33195)
	if err != nil || launchByRP == nil || launchByRP.ID != launchID {
		t.Fatalf("GetLaunchByRPID: got %+v err %v", launchByRP, err)
	}

	// --- Job ---
	jobID, err := s.CreateJob(&Job{
		LaunchID: launchID, RPItemID: 100, Name: "[T-TSC] RAN PTP tests",
		ClockType: "T-TSC", Status: "FAILED",
		StatsTotal: 20, StatsFailed: 5, StatsPassed: 12, StatsSkipped: 3,
	})
	if err != nil {
		t.Fatalf("CreateJob: %v", err)
	}
	job, err := s.GetJob(jobID)
	if err != nil || job == nil || job.Name != "[T-TSC] RAN PTP tests" || job.StatsTotal != 20 {
		t.Fatalf("GetJob: got %+v err %v", job, err)
	}
	jobs, err := s.ListJobsByLaunch(launchID)
	if err != nil || len(jobs) != 1 {
		t.Fatalf("ListJobsByLaunch: got %d err %v", len(jobs), err)
	}

	// --- Case v2 ---
	caseID, err := s.CreateCaseV2(&Case{
		JobID: jobID, LaunchID: launchID, RPItemID: 200,
		Name: "PTP Recovery ptp process restart", Status: "open",
		ErrorMessage: "context deadline exceeded",
	})
	if err != nil {
		t.Fatalf("CreateCaseV2: %v", err)
	}
	c, err := s.GetCaseV2(caseID)
	if err != nil || c == nil || c.Name != "PTP Recovery ptp process restart" {
		t.Fatalf("GetCaseV2: got %+v err %v", c, err)
	}
	if c.Status != "open" || c.ErrorMessage != "context deadline exceeded" {
		t.Fatalf("GetCaseV2 fields: status=%q err_msg=%q", c.Status, c.ErrorMessage)
	}

	cases, err := s.ListCasesByJob(jobID)
	if err != nil || len(cases) != 1 {
		t.Fatalf("ListCasesByJob: got %d err %v", len(cases), err)
	}

	// Update case status
	if err := s.UpdateCaseStatus(caseID, "triaged"); err != nil {
		t.Fatalf("UpdateCaseStatus: %v", err)
	}
	c, _ = s.GetCaseV2(caseID)
	if c.Status != "triaged" {
		t.Errorf("case status after update: got %q want %q", c.Status, "triaged")
	}

	// --- Symptom ---
	symID, err := s.CreateSymptom(&Symptom{
		Fingerprint: "fp-ptp-sync-timeout",
		Name:        "PTP sync timeout",
		ErrorPattern: "context deadline exceeded",
		Component:   "ptp-operator",
	})
	if err != nil {
		t.Fatalf("CreateSymptom: %v", err)
	}
	sym, err := s.GetSymptom(symID)
	if err != nil || sym == nil || sym.Name != "PTP sync timeout" || sym.OccurrenceCount != 1 {
		t.Fatalf("GetSymptom: got %+v err %v", sym, err)
	}
	if sym.Status != "active" {
		t.Errorf("symptom status: got %q want %q", sym.Status, "active")
	}

	symByFP, err := s.GetSymptomByFingerprint("fp-ptp-sync-timeout")
	if err != nil || symByFP == nil || symByFP.ID != symID {
		t.Fatalf("GetSymptomByFingerprint: got %+v err %v", symByFP, err)
	}

	// Update symptom seen
	if err := s.UpdateSymptomSeen(symID); err != nil {
		t.Fatalf("UpdateSymptomSeen: %v", err)
	}
	sym, _ = s.GetSymptom(symID)
	if sym.OccurrenceCount != 2 {
		t.Errorf("symptom occurrence after update: got %d want 2", sym.OccurrenceCount)
	}

	// Link case to symptom
	if err := s.LinkCaseToSymptom(caseID, symID); err != nil {
		t.Fatalf("LinkCaseToSymptom: %v", err)
	}
	c, _ = s.GetCaseV2(caseID)
	if c.SymptomID != symID {
		t.Errorf("case symptom_id: got %d want %d", c.SymptomID, symID)
	}

	// ListCasesBySymptom
	casesBySym, err := s.ListCasesBySymptom(symID)
	if err != nil || len(casesBySym) != 1 {
		t.Fatalf("ListCasesBySymptom: got %d err %v", len(casesBySym), err)
	}

	// --- Triage ---
	triageID, err := s.CreateTriage(&Triage{
		CaseID:               caseID,
		SymptomCategory:      "timeout",
		DefectTypeHypothesis: "pb001",
		CandidateRepos:       `["ptp-operator","linuxptp-daemon"]`,
	})
	if err != nil {
		t.Fatalf("CreateTriage: %v", err)
	}
	triage, err := s.GetTriageByCase(caseID)
	if err != nil || triage == nil || triage.ID != triageID || triage.SymptomCategory != "timeout" {
		t.Fatalf("GetTriageByCase: got %+v err %v", triage, err)
	}

	// --- RCA v2 ---
	rcaID, err := s.SaveRCAV2(&RCA{
		Title:            "PTP holdover timeout reduced",
		Description:      "ptp4l fails to acquire lock because holdover timeout was reduced",
		DefectType:       "pb001",
		Category:         "product",
		Component:        "linuxptp-daemon",
		AffectedVersions: `["4.21"]`,
		ConvergenceScore: 0.85,
	})
	if err != nil {
		t.Fatalf("SaveRCAV2: %v", err)
	}
	rca, err := s.GetRCAV2(rcaID)
	if err != nil || rca == nil || rca.Title != "PTP holdover timeout reduced" || rca.Status != "open" {
		t.Fatalf("GetRCAV2: got %+v err %v", rca, err)
	}
	if rca.ConvergenceScore != 0.85 {
		t.Errorf("rca convergence: got %f want 0.85", rca.ConvergenceScore)
	}

	// RCA status lifecycle
	if err := s.UpdateRCAStatus(rcaID, "resolved"); err != nil {
		t.Fatalf("UpdateRCAStatus resolved: %v", err)
	}
	rca, _ = s.GetRCAV2(rcaID)
	if rca.Status != "resolved" || rca.ResolvedAt == "" {
		t.Errorf("rca resolved: status=%q resolvedAt=%q", rca.Status, rca.ResolvedAt)
	}

	openRCAs, err := s.ListRCAsByStatus("open")
	if err != nil {
		t.Fatalf("ListRCAsByStatus: %v", err)
	}
	resolvedRCAs, err := s.ListRCAsByStatus("resolved")
	if err != nil || len(resolvedRCAs) != 1 {
		t.Fatalf("ListRCAsByStatus resolved: got %d err %v", len(resolvedRCAs), err)
	}
	_ = openRCAs

	// --- SymptomRCA ---
	linkID, err := s.LinkSymptomToRCA(&SymptomRCA{
		SymptomID: symID, RCAID: rcaID, Confidence: 0.9,
		Notes: "High confidence match",
	})
	if err != nil {
		t.Fatalf("LinkSymptomToRCA: %v", err)
	}
	if linkID == 0 {
		t.Error("LinkSymptomToRCA returned 0 id")
	}

	rcasForSym, err := s.GetRCAsForSymptom(symID)
	if err != nil || len(rcasForSym) != 1 || rcasForSym[0].RCAID != rcaID {
		t.Fatalf("GetRCAsForSymptom: got %+v err %v", rcasForSym, err)
	}
	symsForRCA, err := s.GetSymptomsForRCA(rcaID)
	if err != nil || len(symsForRCA) != 1 || symsForRCA[0].SymptomID != symID {
		t.Fatalf("GetSymptomsForRCA: got %+v err %v", symsForRCA, err)
	}

	// --- Close suite ---
	if err := s.CloseSuite(suiteID); err != nil {
		t.Fatalf("CloseSuite: %v", err)
	}
	suite, _ = s.GetSuite(suiteID)
	if suite.Status != "closed" || suite.ClosedAt == "" {
		t.Errorf("closed suite: status=%q closedAt=%q", suite.Status, suite.ClosedAt)
	}
}

// TestSqlStoreV2_FreshInstall verifies that a fresh DB gets v2 schema directly.
func TestSqlStoreV2_FreshInstall(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "fresh.db")
	s, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer s.Close()

	// Verify v2 tables exist by running a v2 query.
	_, err = s.CreateSuite(&InvestigationSuite{Name: "test"})
	if err != nil {
		t.Fatalf("CreateSuite on fresh db: %v", err)
	}
	_, err = s.CreateSymptom(&Symptom{Fingerprint: "test-fp", Name: "test"})
	if err != nil {
		t.Fatalf("CreateSymptom on fresh db: %v", err)
	}
}

// TestSqlStoreV2_Migration tests v1 → v2 migration with data preservation.
func TestSqlStoreV2_Migration(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "migrate.db")

	// Step 1: Create a v1 database manually.
	createV1DB(t, path)

	// Step 2: Open with v2 code — migration should run automatically.
	s, err := Open(path)
	if err != nil {
		t.Fatalf("Open (migration): %v", err)
	}
	defer s.Close()

	// Step 3: Verify v2 tables exist and v1 data was migrated.

	// Check schema version.
	var v int
	if err := s.db.QueryRow("SELECT version FROM schema_version LIMIT 1").Scan(&v); err != nil {
		t.Fatalf("read schema version: %v", err)
	}
	if v != schemaVersionV2 {
		t.Errorf("schema version: got %d want %d", v, schemaVersionV2)
	}

	// Check that migration suite was created.
	suites, err := s.ListSuites()
	if err != nil || len(suites) == 0 {
		t.Fatalf("ListSuites after migration: got %d err %v", len(suites), err)
	}

	// Check that the v1 RCA was migrated.
	rcas, err := s.ListRCAs()
	if err != nil {
		t.Fatalf("ListRCAs after migration: %v", err)
	}
	if len(rcas) != 1 || rcas[0].Title != "v1-rca" {
		t.Errorf("migrated RCAs: got %+v", rcas)
	}

	// Check that the v1 envelope was migrated to a launch.
	env, err := s.GetEnvelope(33195)
	if err != nil {
		t.Fatalf("GetEnvelope after migration: %v", err)
	}
	if env == nil || env.RunID != "33195" {
		t.Errorf("migrated envelope: got %+v", env)
	}

	// Check that v1 cases were migrated.
	cases, err := s.ListCasesByLaunch(33195)
	if err != nil {
		t.Fatalf("ListCasesByLaunch after migration: %v", err)
	}
	if len(cases) != 2 {
		t.Errorf("migrated cases: got %d want 2", len(cases))
	}

	// v2 operations should work.
	_, err = s.CreateSymptom(&Symptom{Fingerprint: "new-fp", Name: "new symptom"})
	if err != nil {
		t.Fatalf("CreateSymptom after migration: %v", err)
	}
}

// createV1DB creates a v1 database with sample data for migration testing.
func createV1DB(t *testing.T, path string) {
	t.Helper()
	// We need to create a raw v1 DB. Use the sqlite driver directly.
	// Import is already available via the sqlstore.go blank import.
	import_db, err := openRawDB(path)
	if err != nil {
		t.Fatalf("open raw db: %v", err)
	}
	defer import_db.Close()

	if _, err := import_db.Exec(schemaV1); err != nil {
		t.Fatalf("create v1 schema: %v", err)
	}
	if _, err := import_db.Exec("INSERT INTO schema_version(version) VALUES(1)"); err != nil {
		t.Fatalf("insert v1 version: %v", err)
	}
	// Insert sample RCA.
	if _, err := import_db.Exec(
		"INSERT INTO rcas(title, description, defect_type) VALUES('v1-rca', 'v1 desc', 'ti001')",
	); err != nil {
		t.Fatalf("insert v1 rca: %v", err)
	}
	// Insert sample envelope.
	envJSON := `{"run_id":"33195","name":"test","failure_list":[{"id":1,"name":"f1","status":"FAILED"},{"id":2,"name":"f2","status":"FAILED"}]}`
	if _, err := import_db.Exec(
		"INSERT INTO envelopes(launch_id, payload) VALUES(33195, ?)", []byte(envJSON),
	); err != nil {
		t.Fatalf("insert v1 envelope: %v", err)
	}
	// Insert sample cases.
	if _, err := import_db.Exec("INSERT INTO cases(launch_id, item_id) VALUES(33195, 1)"); err != nil {
		t.Fatalf("insert v1 case 1: %v", err)
	}
	if _, err := import_db.Exec("INSERT INTO cases(launch_id, item_id, rca_id) VALUES(33195, 2, 1)"); err != nil {
		t.Fatalf("insert v1 case 2: %v", err)
	}
}

// openRawDB opens a raw SQLite DB connection (no migration).
func openRawDB(path string) (*sqlRawDB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	return &sqlRawDB{db: db}, nil
}

type sqlRawDB struct{ db *sql.DB }

func (r *sqlRawDB) Exec(q string, args ...interface{}) (sql.Result, error) {
	return r.db.Exec(q, args...)
}
func (r *sqlRawDB) Close() error { return r.db.Close() }
