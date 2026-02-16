package store

import (
	"database/sql"
	"errors"
	"fmt"
)

// --- Suite ---

func (s *SqlStore) CreateSuite(suite *InvestigationSuite) (int64, error) {
	if suite == nil {
		return 0, errors.New("suite is nil")
	}
	now := nowUTC()
	if suite.Status == "" {
		suite.Status = "open"
	}
	if suite.CreatedAt == "" {
		suite.CreatedAt = now
	}
	res, err := s.db.Exec(
		`INSERT INTO investigation_suites(name, description, status, created_at, closed_at)
		 VALUES(?, ?, ?, ?, ?)`,
		suite.Name, suite.Description, suite.Status, suite.CreatedAt, nilIfEmpty(suite.ClosedAt),
	)
	if err != nil {
		return 0, fmt.Errorf("insert suite: %w", err)
	}
	return res.LastInsertId()
}

func (s *SqlStore) GetSuite(id int64) (*InvestigationSuite, error) {
	var v InvestigationSuite
	var closedAt sql.NullString
	err := s.db.QueryRow(
		`SELECT id, name, description, status, created_at, closed_at
		 FROM investigation_suites WHERE id = ?`, id,
	).Scan(&v.ID, &v.Name, &v.Description, &v.Status, &v.CreatedAt, &closedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get suite: %w", err)
	}
	v.ClosedAt = nullStr(closedAt)
	return &v, nil
}

func (s *SqlStore) ListSuites() ([]*InvestigationSuite, error) {
	rows, err := s.db.Query(
		`SELECT id, name, description, status, created_at, closed_at
		 FROM investigation_suites ORDER BY id`,
	)
	if err != nil {
		return nil, fmt.Errorf("list suites: %w", err)
	}
	defer rows.Close()
	var out []*InvestigationSuite
	for rows.Next() {
		var v InvestigationSuite
		var closedAt sql.NullString
		if err := rows.Scan(&v.ID, &v.Name, &v.Description, &v.Status, &v.CreatedAt, &closedAt); err != nil {
			return nil, fmt.Errorf("scan suite: %w", err)
		}
		v.ClosedAt = nullStr(closedAt)
		out = append(out, &v)
	}
	return out, rows.Err()
}

func (s *SqlStore) CloseSuite(id int64) error {
	now := nowUTC()
	res, err := s.db.Exec(
		`UPDATE investigation_suites SET status = 'closed', closed_at = ? WHERE id = ?`,
		now, id,
	)
	if err != nil {
		return fmt.Errorf("close suite: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("suite %d not found", id)
	}
	return nil
}

// --- Version ---

func (s *SqlStore) CreateVersion(v *Version) (int64, error) {
	if v == nil {
		return 0, errors.New("version is nil")
	}
	res, err := s.db.Exec(
		"INSERT INTO versions(label, ocp_build) VALUES(?, ?)",
		v.Label, nilIfEmpty(v.OCPBuild),
	)
	if err != nil {
		return 0, fmt.Errorf("insert version: %w", err)
	}
	return res.LastInsertId()
}

func (s *SqlStore) GetVersion(id int64) (*Version, error) {
	var v Version
	var ocp sql.NullString
	err := s.db.QueryRow(
		"SELECT id, label, ocp_build FROM versions WHERE id = ?", id,
	).Scan(&v.ID, &v.Label, &ocp)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get version: %w", err)
	}
	v.OCPBuild = nullStr(ocp)
	return &v, nil
}

func (s *SqlStore) GetVersionByLabel(label string) (*Version, error) {
	var v Version
	var ocp sql.NullString
	err := s.db.QueryRow(
		"SELECT id, label, ocp_build FROM versions WHERE label = ?", label,
	).Scan(&v.ID, &v.Label, &ocp)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get version by label: %w", err)
	}
	v.OCPBuild = nullStr(ocp)
	return &v, nil
}

func (s *SqlStore) ListVersions() ([]*Version, error) {
	rows, err := s.db.Query("SELECT id, label, ocp_build FROM versions ORDER BY id")
	if err != nil {
		return nil, fmt.Errorf("list versions: %w", err)
	}
	defer rows.Close()
	var out []*Version
	for rows.Next() {
		var v Version
		var ocp sql.NullString
		if err := rows.Scan(&v.ID, &v.Label, &ocp); err != nil {
			return nil, fmt.Errorf("scan version: %w", err)
		}
		v.OCPBuild = nullStr(ocp)
		out = append(out, &v)
	}
	return out, rows.Err()
}

// --- Pipeline ---

func (s *SqlStore) CreatePipeline(p *Pipeline) (int64, error) {
	if p == nil {
		return 0, errors.New("pipeline is nil")
	}
	res, err := s.db.Exec(
		`INSERT INTO pipelines(suite_id, version_id, name, rp_launch_id, status, started_at, ended_at)
		 VALUES(?, ?, ?, ?, ?, ?, ?)`,
		p.SuiteID, p.VersionID, p.Name, nilIfZero(p.RPLaunchID), p.Status,
		nilIfEmpty(p.StartedAt), nilIfEmpty(p.EndedAt),
	)
	if err != nil {
		return 0, fmt.Errorf("insert pipeline: %w", err)
	}
	return res.LastInsertId()
}

func (s *SqlStore) GetPipeline(id int64) (*Pipeline, error) {
	var p Pipeline
	var rpLaunch sql.NullInt64
	var startedAt, endedAt sql.NullString
	err := s.db.QueryRow(
		`SELECT id, suite_id, version_id, name, rp_launch_id, status, started_at, ended_at
		 FROM pipelines WHERE id = ?`, id,
	).Scan(&p.ID, &p.SuiteID, &p.VersionID, &p.Name, &rpLaunch, &p.Status, &startedAt, &endedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get pipeline: %w", err)
	}
	if rpLaunch.Valid {
		p.RPLaunchID = int(rpLaunch.Int64)
	}
	p.StartedAt = nullStr(startedAt)
	p.EndedAt = nullStr(endedAt)
	return &p, nil
}

func (s *SqlStore) ListPipelinesBySuite(suiteID int64) ([]*Pipeline, error) {
	rows, err := s.db.Query(
		`SELECT id, suite_id, version_id, name, rp_launch_id, status, started_at, ended_at
		 FROM pipelines WHERE suite_id = ? ORDER BY id`, suiteID,
	)
	if err != nil {
		return nil, fmt.Errorf("list pipelines: %w", err)
	}
	defer rows.Close()
	var out []*Pipeline
	for rows.Next() {
		var p Pipeline
		var rpLaunch sql.NullInt64
		var startedAt, endedAt sql.NullString
		if err := rows.Scan(&p.ID, &p.SuiteID, &p.VersionID, &p.Name, &rpLaunch, &p.Status, &startedAt, &endedAt); err != nil {
			return nil, fmt.Errorf("scan pipeline: %w", err)
		}
		if rpLaunch.Valid {
			p.RPLaunchID = int(rpLaunch.Int64)
		}
		p.StartedAt = nullStr(startedAt)
		p.EndedAt = nullStr(endedAt)
		out = append(out, &p)
	}
	return out, rows.Err()
}

// --- Launch ---

func (s *SqlStore) CreateLaunch(l *Launch) (int64, error) {
	if l == nil {
		return 0, errors.New("launch is nil")
	}
	res, err := s.db.Exec(
		`INSERT INTO launches(pipeline_id, rp_launch_id, rp_launch_uuid, name, status,
		        started_at, ended_at, env_attributes, git_branch, git_commit, envelope_payload)
		 VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		l.PipelineID, l.RPLaunchID, nilIfEmpty(l.RPLaunchUUID),
		nilIfEmpty(l.Name), nilIfEmpty(l.Status),
		nilIfEmpty(l.StartedAt), nilIfEmpty(l.EndedAt),
		nilIfEmpty(l.EnvAttributes), nilIfEmpty(l.GitBranch), nilIfEmpty(l.GitCommit),
		l.EnvelopePayload,
	)
	if err != nil {
		return 0, fmt.Errorf("insert launch: %w", err)
	}
	return res.LastInsertId()
}

func (s *SqlStore) GetLaunch(id int64) (*Launch, error) {
	var l Launch
	var uuid, name, status, startedAt, endedAt sql.NullString
	var envAttr, gitBranch, gitCommit sql.NullString
	err := s.db.QueryRow(
		`SELECT id, pipeline_id, rp_launch_id, rp_launch_uuid, name, status,
		        started_at, ended_at, env_attributes, git_branch, git_commit, envelope_payload
		 FROM launches WHERE id = ?`, id,
	).Scan(&l.ID, &l.PipelineID, &l.RPLaunchID, &uuid, &name, &status,
		&startedAt, &endedAt, &envAttr, &gitBranch, &gitCommit, &l.EnvelopePayload)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get launch: %w", err)
	}
	l.RPLaunchUUID = nullStr(uuid)
	l.Name = nullStr(name)
	l.Status = nullStr(status)
	l.StartedAt = nullStr(startedAt)
	l.EndedAt = nullStr(endedAt)
	l.EnvAttributes = nullStr(envAttr)
	l.GitBranch = nullStr(gitBranch)
	l.GitCommit = nullStr(gitCommit)
	return &l, nil
}

func (s *SqlStore) GetLaunchByRPID(pipelineID int64, rpLaunchID int) (*Launch, error) {
	var l Launch
	var uuid, name, status, startedAt, endedAt sql.NullString
	var envAttr, gitBranch, gitCommit sql.NullString
	err := s.db.QueryRow(
		`SELECT id, pipeline_id, rp_launch_id, rp_launch_uuid, name, status,
		        started_at, ended_at, env_attributes, git_branch, git_commit, envelope_payload
		 FROM launches WHERE pipeline_id = ? AND rp_launch_id = ?`, pipelineID, rpLaunchID,
	).Scan(&l.ID, &l.PipelineID, &l.RPLaunchID, &uuid, &name, &status,
		&startedAt, &endedAt, &envAttr, &gitBranch, &gitCommit, &l.EnvelopePayload)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get launch by rp id: %w", err)
	}
	l.RPLaunchUUID = nullStr(uuid)
	l.Name = nullStr(name)
	l.Status = nullStr(status)
	l.StartedAt = nullStr(startedAt)
	l.EndedAt = nullStr(endedAt)
	l.EnvAttributes = nullStr(envAttr)
	l.GitBranch = nullStr(gitBranch)
	l.GitCommit = nullStr(gitCommit)
	return &l, nil
}

func (s *SqlStore) ListLaunchesByPipeline(pipelineID int64) ([]*Launch, error) {
	rows, err := s.db.Query(
		`SELECT id, pipeline_id, rp_launch_id, rp_launch_uuid, name, status,
		        started_at, ended_at, env_attributes, git_branch, git_commit, envelope_payload
		 FROM launches WHERE pipeline_id = ? ORDER BY id`, pipelineID,
	)
	if err != nil {
		return nil, fmt.Errorf("list launches: %w", err)
	}
	defer rows.Close()
	var out []*Launch
	for rows.Next() {
		var l Launch
		var uuid, name, status, startedAt, endedAt sql.NullString
		var envAttr, gitBranch, gitCommit sql.NullString
		if err := rows.Scan(&l.ID, &l.PipelineID, &l.RPLaunchID, &uuid, &name, &status,
			&startedAt, &endedAt, &envAttr, &gitBranch, &gitCommit, &l.EnvelopePayload); err != nil {
			return nil, fmt.Errorf("scan launch: %w", err)
		}
		l.RPLaunchUUID = nullStr(uuid)
		l.Name = nullStr(name)
		l.Status = nullStr(status)
		l.StartedAt = nullStr(startedAt)
		l.EndedAt = nullStr(endedAt)
		l.EnvAttributes = nullStr(envAttr)
		l.GitBranch = nullStr(gitBranch)
		l.GitCommit = nullStr(gitCommit)
		out = append(out, &l)
	}
	return out, rows.Err()
}

// --- Job ---

func (s *SqlStore) CreateJob(j *Job) (int64, error) {
	if j == nil {
		return 0, errors.New("job is nil")
	}
	res, err := s.db.Exec(
		`INSERT INTO jobs(launch_id, rp_item_id, name, clock_type, status,
		        stats_total, stats_failed, stats_passed, stats_skipped, started_at, ended_at)
		 VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		j.LaunchID, j.RPItemID, j.Name, nilIfEmpty(j.ClockType), nilIfEmpty(j.Status),
		nilIfZero(j.StatsTotal), nilIfZero(j.StatsFailed), nilIfZero(j.StatsPassed), nilIfZero(j.StatsSkipped),
		nilIfEmpty(j.StartedAt), nilIfEmpty(j.EndedAt),
	)
	if err != nil {
		return 0, fmt.Errorf("insert job: %w", err)
	}
	return res.LastInsertId()
}

func (s *SqlStore) GetJob(id int64) (*Job, error) {
	var j Job
	var clockType, status, startedAt, endedAt sql.NullString
	var total, failed, passed, skipped sql.NullInt64
	err := s.db.QueryRow(
		`SELECT id, launch_id, rp_item_id, name, clock_type, status,
		        stats_total, stats_failed, stats_passed, stats_skipped, started_at, ended_at
		 FROM jobs WHERE id = ?`, id,
	).Scan(&j.ID, &j.LaunchID, &j.RPItemID, &j.Name, &clockType, &status,
		&total, &failed, &passed, &skipped, &startedAt, &endedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get job: %w", err)
	}
	j.ClockType = nullStr(clockType)
	j.Status = nullStr(status)
	if total.Valid {
		j.StatsTotal = int(total.Int64)
	}
	if failed.Valid {
		j.StatsFailed = int(failed.Int64)
	}
	if passed.Valid {
		j.StatsPassed = int(passed.Int64)
	}
	if skipped.Valid {
		j.StatsSkipped = int(skipped.Int64)
	}
	j.StartedAt = nullStr(startedAt)
	j.EndedAt = nullStr(endedAt)
	return &j, nil
}

func (s *SqlStore) ListJobsByLaunch(launchID int64) ([]*Job, error) {
	rows, err := s.db.Query(
		`SELECT id, launch_id, rp_item_id, name, clock_type, status,
		        stats_total, stats_failed, stats_passed, stats_skipped, started_at, ended_at
		 FROM jobs WHERE launch_id = ? ORDER BY id`, launchID,
	)
	if err != nil {
		return nil, fmt.Errorf("list jobs: %w", err)
	}
	defer rows.Close()
	var out []*Job
	for rows.Next() {
		var j Job
		var clockType, status, startedAt, endedAt sql.NullString
		var total, failed, passed, skipped sql.NullInt64
		if err := rows.Scan(&j.ID, &j.LaunchID, &j.RPItemID, &j.Name, &clockType, &status,
			&total, &failed, &passed, &skipped, &startedAt, &endedAt); err != nil {
			return nil, fmt.Errorf("scan job: %w", err)
		}
		j.ClockType = nullStr(clockType)
		j.Status = nullStr(status)
		if total.Valid {
			j.StatsTotal = int(total.Int64)
		}
		if failed.Valid {
			j.StatsFailed = int(failed.Int64)
		}
		if passed.Valid {
			j.StatsPassed = int(passed.Int64)
		}
		if skipped.Valid {
			j.StatsSkipped = int(skipped.Int64)
		}
		j.StartedAt = nullStr(startedAt)
		j.EndedAt = nullStr(endedAt)
		out = append(out, &j)
	}
	return out, rows.Err()
}

// --- Case v2 ---

func (s *SqlStore) CreateCaseV2(c *Case) (int64, error) {
	if c == nil {
		return 0, errors.New("case is nil")
	}
	now := nowUTC()
	if c.Status == "" {
		c.Status = "open"
	}
	if c.CreatedAt == "" {
		c.CreatedAt = now
	}
	c.UpdatedAt = now
	logTrunc := 0
	if c.LogTruncated {
		logTrunc = 1
	}
	res, err := s.db.Exec(
		`INSERT INTO cases(job_id, launch_id, rp_item_id, name, polarion_id, status,
		        symptom_id, rca_id, error_message, log_snippet, log_truncated,
		        started_at, ended_at, created_at, updated_at)
		 VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		c.JobID, c.LaunchID, c.RPItemID, c.Name, nilIfEmpty(c.PolarionID), c.Status,
		nilIfZero64(c.SymptomID), nilIfZero64(c.RCAID),
		nilIfEmpty(c.ErrorMessage), nilIfEmpty(c.LogSnippet), logTrunc,
		nilIfEmpty(c.StartedAt), nilIfEmpty(c.EndedAt), c.CreatedAt, c.UpdatedAt,
	)
	if err != nil {
		return 0, fmt.Errorf("insert case v2: %w", err)
	}
	return res.LastInsertId()
}

func (s *SqlStore) GetCaseV2(id int64) (*Case, error) {
	// Reuse GetCase — it already reads all v2 fields.
	return s.GetCase(id)
}

func (s *SqlStore) ListCasesByJob(jobID int64) ([]*Case, error) {
	rows, err := s.db.Query(
		`SELECT id, job_id, launch_id, rp_item_id, name, polarion_id, status,
		        symptom_id, rca_id, error_message, log_snippet, log_truncated,
		        started_at, ended_at, created_at, updated_at
		 FROM cases WHERE job_id = ? ORDER BY id`, jobID,
	)
	if err != nil {
		return nil, fmt.Errorf("list cases by job: %w", err)
	}
	defer rows.Close()
	return scanCases(rows)
}

func (s *SqlStore) ListCasesBySymptom(symptomID int64) ([]*Case, error) {
	rows, err := s.db.Query(
		`SELECT id, job_id, launch_id, rp_item_id, name, polarion_id, status,
		        symptom_id, rca_id, error_message, log_snippet, log_truncated,
		        started_at, ended_at, created_at, updated_at
		 FROM cases WHERE symptom_id = ? ORDER BY id`, symptomID,
	)
	if err != nil {
		return nil, fmt.Errorf("list cases by symptom: %w", err)
	}
	defer rows.Close()
	return scanCases(rows)
}

func (s *SqlStore) UpdateCaseStatus(caseID int64, status string) error {
	now := nowUTC()
	res, err := s.db.Exec(
		"UPDATE cases SET status = ?, updated_at = ? WHERE id = ?",
		status, now, caseID,
	)
	if err != nil {
		return fmt.Errorf("update case status: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("case %d not found", caseID)
	}
	return nil
}

func (s *SqlStore) LinkCaseToSymptom(caseID, symptomID int64) error {
	now := nowUTC()
	res, err := s.db.Exec(
		"UPDATE cases SET symptom_id = ?, updated_at = ? WHERE id = ?",
		symptomID, now, caseID,
	)
	if err != nil {
		return fmt.Errorf("link case to symptom: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("case %d not found", caseID)
	}
	return nil
}

// --- Triage ---

func (s *SqlStore) CreateTriage(t *Triage) (int64, error) {
	if t == nil {
		return 0, errors.New("triage is nil")
	}
	now := nowUTC()
	if t.CreatedAt == "" {
		t.CreatedAt = now
	}
	skipInv, clockSkew, cascade := boolToInt(t.SkipInvestigation), boolToInt(t.ClockSkewSuspected), boolToInt(t.CascadeSuspected)
	res, err := s.db.Exec(
		`INSERT INTO triages(case_id, symptom_category, severity, defect_type_hypothesis,
		        skip_investigation, clock_skew_suspected, cascade_suspected,
		        candidate_repos, data_quality_notes, created_at)
		 VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		t.CaseID, t.SymptomCategory, nilIfEmpty(t.Severity), nilIfEmpty(t.DefectTypeHypothesis),
		skipInv, clockSkew, cascade,
		nilIfEmpty(t.CandidateRepos), nilIfEmpty(t.DataQualityNotes), t.CreatedAt,
	)
	if err != nil {
		return 0, fmt.Errorf("insert triage: %w", err)
	}
	return res.LastInsertId()
}

func (s *SqlStore) GetTriageByCase(caseID int64) (*Triage, error) {
	var t Triage
	var sev, defHyp, repos, notes sql.NullString
	var skipInv, clockSkew, cascade sql.NullInt64
	err := s.db.QueryRow(
		`SELECT id, case_id, symptom_category, severity, defect_type_hypothesis,
		        skip_investigation, clock_skew_suspected, cascade_suspected,
		        candidate_repos, data_quality_notes, created_at
		 FROM triages WHERE case_id = ?`, caseID,
	).Scan(&t.ID, &t.CaseID, &t.SymptomCategory, &sev, &defHyp,
		&skipInv, &clockSkew, &cascade, &repos, &notes, &t.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get triage: %w", err)
	}
	t.Severity = nullStr(sev)
	t.DefectTypeHypothesis = nullStr(defHyp)
	t.SkipInvestigation = skipInv.Valid && skipInv.Int64 == 1
	t.ClockSkewSuspected = clockSkew.Valid && clockSkew.Int64 == 1
	t.CascadeSuspected = cascade.Valid && cascade.Int64 == 1
	t.CandidateRepos = nullStr(repos)
	t.DataQualityNotes = nullStr(notes)
	return &t, nil
}

// --- Symptom ---

func (s *SqlStore) CreateSymptom(sym *Symptom) (int64, error) {
	if sym == nil {
		return 0, errors.New("symptom is nil")
	}
	now := nowUTC()
	if sym.Status == "" {
		sym.Status = "active"
	}
	if sym.OccurrenceCount == 0 {
		sym.OccurrenceCount = 1
	}
	if sym.FirstSeenAt == "" {
		sym.FirstSeenAt = now
	}
	if sym.LastSeenAt == "" {
		sym.LastSeenAt = sym.FirstSeenAt
	}
	res, err := s.db.Exec(
		`INSERT INTO symptoms(fingerprint, name, description, error_pattern, test_name_pattern,
		        component, severity, first_seen_at, last_seen_at, occurrence_count, status)
		 VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		sym.Fingerprint, sym.Name, nilIfEmpty(sym.Description),
		nilIfEmpty(sym.ErrorPattern), nilIfEmpty(sym.TestNamePattern),
		nilIfEmpty(sym.Component), nilIfEmpty(sym.Severity),
		sym.FirstSeenAt, sym.LastSeenAt, sym.OccurrenceCount, sym.Status,
	)
	if err != nil {
		return 0, fmt.Errorf("insert symptom: %w", err)
	}
	return res.LastInsertId()
}

func (s *SqlStore) GetSymptom(id int64) (*Symptom, error) {
	var sym Symptom
	var desc, errPat, testPat, comp, sev sql.NullString
	err := s.db.QueryRow(
		`SELECT id, fingerprint, name, description, error_pattern, test_name_pattern,
		        component, severity, first_seen_at, last_seen_at, occurrence_count, status
		 FROM symptoms WHERE id = ?`, id,
	).Scan(&sym.ID, &sym.Fingerprint, &sym.Name, &desc, &errPat, &testPat,
		&comp, &sev, &sym.FirstSeenAt, &sym.LastSeenAt, &sym.OccurrenceCount, &sym.Status)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get symptom: %w", err)
	}
	sym.Description = nullStr(desc)
	sym.ErrorPattern = nullStr(errPat)
	sym.TestNamePattern = nullStr(testPat)
	sym.Component = nullStr(comp)
	sym.Severity = nullStr(sev)
	return &sym, nil
}

func (s *SqlStore) GetSymptomByFingerprint(fingerprint string) (*Symptom, error) {
	var sym Symptom
	var desc, errPat, testPat, comp, sev sql.NullString
	err := s.db.QueryRow(
		`SELECT id, fingerprint, name, description, error_pattern, test_name_pattern,
		        component, severity, first_seen_at, last_seen_at, occurrence_count, status
		 FROM symptoms WHERE fingerprint = ?`, fingerprint,
	).Scan(&sym.ID, &sym.Fingerprint, &sym.Name, &desc, &errPat, &testPat,
		&comp, &sev, &sym.FirstSeenAt, &sym.LastSeenAt, &sym.OccurrenceCount, &sym.Status)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get symptom by fingerprint: %w", err)
	}
	sym.Description = nullStr(desc)
	sym.ErrorPattern = nullStr(errPat)
	sym.TestNamePattern = nullStr(testPat)
	sym.Component = nullStr(comp)
	sym.Severity = nullStr(sev)
	return &sym, nil
}

func (s *SqlStore) FindSymptomCandidates(testName string) ([]*Symptom, error) {
	rows, err := s.db.Query(
		`SELECT id, fingerprint, name, description, error_pattern, test_name_pattern,
		        component, severity, first_seen_at, last_seen_at, occurrence_count, status
		 FROM symptoms WHERE name = ?`, testName,
	)
	if err != nil {
		return nil, fmt.Errorf("find symptom candidates: %w", err)
	}
	defer rows.Close()
	var out []*Symptom
	for rows.Next() {
		var sym Symptom
		var desc, errPat, testPat, comp, sev sql.NullString
		if err := rows.Scan(&sym.ID, &sym.Fingerprint, &sym.Name, &desc, &errPat, &testPat,
			&comp, &sev, &sym.FirstSeenAt, &sym.LastSeenAt, &sym.OccurrenceCount, &sym.Status); err != nil {
			return nil, fmt.Errorf("scan symptom candidate: %w", err)
		}
		sym.Description = nullStr(desc)
		sym.ErrorPattern = nullStr(errPat)
		sym.TestNamePattern = nullStr(testPat)
		sym.Component = nullStr(comp)
		sym.Severity = nullStr(sev)
		out = append(out, &sym)
	}
	return out, rows.Err()
}

func (s *SqlStore) UpdateSymptomSeen(id int64) error {
	now := nowUTC()
	res, err := s.db.Exec(
		`UPDATE symptoms SET occurrence_count = occurrence_count + 1, last_seen_at = ?,
		        status = CASE WHEN status = 'dormant' THEN 'active' ELSE status END
		 WHERE id = ?`,
		now, id,
	)
	if err != nil {
		return fmt.Errorf("update symptom seen: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("symptom %d not found", id)
	}
	return nil
}

func (s *SqlStore) ListSymptoms() ([]*Symptom, error) {
	rows, err := s.db.Query(
		`SELECT id, fingerprint, name, description, error_pattern, test_name_pattern,
		        component, severity, first_seen_at, last_seen_at, occurrence_count, status
		 FROM symptoms ORDER BY id`,
	)
	if err != nil {
		return nil, fmt.Errorf("list symptoms: %w", err)
	}
	defer rows.Close()
	var out []*Symptom
	for rows.Next() {
		var sym Symptom
		var desc, errPat, testPat, comp, sev sql.NullString
		if err := rows.Scan(&sym.ID, &sym.Fingerprint, &sym.Name, &desc, &errPat, &testPat,
			&comp, &sev, &sym.FirstSeenAt, &sym.LastSeenAt, &sym.OccurrenceCount, &sym.Status); err != nil {
			return nil, fmt.Errorf("scan symptom: %w", err)
		}
		sym.Description = nullStr(desc)
		sym.ErrorPattern = nullStr(errPat)
		sym.TestNamePattern = nullStr(testPat)
		sym.Component = nullStr(comp)
		sym.Severity = nullStr(sev)
		out = append(out, &sym)
	}
	return out, rows.Err()
}

func (s *SqlStore) MarkDormantSymptoms(staleDays int) (int64, error) {
	res, err := s.db.Exec(
		`UPDATE symptoms SET status = 'dormant'
		 WHERE status = 'active'
		   AND last_seen_at < datetime('now', '-' || ? || ' days')`,
		staleDays,
	)
	if err != nil {
		return 0, fmt.Errorf("mark dormant symptoms: %w", err)
	}
	return res.RowsAffected()
}

// --- RCA v2 ---

func (s *SqlStore) SaveRCAV2(rca *RCA) (int64, error) {
	if rca == nil {
		return 0, errors.New("rca is nil")
	}
	now := nowUTC()
	if rca.Status == "" {
		rca.Status = "open"
	}
	if rca.CreatedAt == "" {
		rca.CreatedAt = now
	}
	if rca.ID != 0 {
		_, err := s.db.Exec(
			`UPDATE rcas SET title=?, description=?, defect_type=?, category=?, component=?,
			        affected_versions=?, evidence_refs=?, convergence_score=?,
			        jira_ticket_id=?, jira_link=?, status=?,
			        resolved_at=?, verified_at=?, archived_at=?
			 WHERE id=?`,
			rca.Title, rca.Description, rca.DefectType, nilIfEmpty(rca.Category), nilIfEmpty(rca.Component),
			nilIfEmpty(rca.AffectedVersions), nilIfEmpty(rca.EvidenceRefs), rca.ConvergenceScore,
			nilIfEmpty(rca.JiraTicketID), nilIfEmpty(rca.JiraLink), rca.Status,
			nilIfEmpty(rca.ResolvedAt), nilIfEmpty(rca.VerifiedAt), nilIfEmpty(rca.ArchivedAt),
			rca.ID,
		)
		if err != nil {
			return 0, fmt.Errorf("update rca v2: %w", err)
		}
		return rca.ID, nil
	}
	res, err := s.db.Exec(
		`INSERT INTO rcas(title, description, defect_type, category, component,
		        affected_versions, evidence_refs, convergence_score,
		        jira_ticket_id, jira_link, status, created_at,
		        resolved_at, verified_at, archived_at)
		 VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		rca.Title, rca.Description, rca.DefectType, nilIfEmpty(rca.Category), nilIfEmpty(rca.Component),
		nilIfEmpty(rca.AffectedVersions), nilIfEmpty(rca.EvidenceRefs), rca.ConvergenceScore,
		nilIfEmpty(rca.JiraTicketID), nilIfEmpty(rca.JiraLink), rca.Status, rca.CreatedAt,
		nilIfEmpty(rca.ResolvedAt), nilIfEmpty(rca.VerifiedAt), nilIfEmpty(rca.ArchivedAt),
	)
	if err != nil {
		return 0, fmt.Errorf("insert rca v2: %w", err)
	}
	return res.LastInsertId()
}

func (s *SqlStore) GetRCAV2(id int64) (*RCA, error) {
	// Reuse GetRCA — it already reads all v2 fields.
	return s.GetRCA(id)
}

func (s *SqlStore) ListRCAsByStatus(status string) ([]*RCA, error) {
	rows, err := s.db.Query(
		`SELECT id, title, description, defect_type, category, component,
		        affected_versions, evidence_refs, convergence_score,
		        jira_ticket_id, jira_link, status, created_at,
		        resolved_at, verified_at, archived_at
		 FROM rcas WHERE status = ? ORDER BY id`, status,
	)
	if err != nil {
		return nil, fmt.Errorf("list rcas by status: %w", err)
	}
	defer rows.Close()
	return scanRCAs(rows)
}

func (s *SqlStore) UpdateRCAStatus(id int64, status string) error {
	now := nowUTC()
	var setExtra string
	switch status {
	case "resolved":
		setExtra = ", resolved_at = '" + now + "'"
	case "verified":
		setExtra = ", verified_at = '" + now + "'"
	case "archived":
		setExtra = ", archived_at = '" + now + "'"
	case "open":
		setExtra = ", resolved_at = NULL, verified_at = NULL"
	}
	res, err := s.db.Exec(
		fmt.Sprintf("UPDATE rcas SET status = ?%s WHERE id = ?", setExtra),
		status, id,
	)
	if err != nil {
		return fmt.Errorf("update rca status: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("rca %d not found", id)
	}
	return nil
}

// --- SymptomRCA ---

func (s *SqlStore) LinkSymptomToRCA(link *SymptomRCA) (int64, error) {
	if link == nil {
		return 0, errors.New("link is nil")
	}
	now := nowUTC()
	if link.LinkedAt == "" {
		link.LinkedAt = now
	}
	res, err := s.db.Exec(
		`INSERT INTO symptom_rca(symptom_id, rca_id, confidence, notes, linked_at)
		 VALUES(?, ?, ?, ?, ?)`,
		link.SymptomID, link.RCAID, link.Confidence, nilIfEmpty(link.Notes), link.LinkedAt,
	)
	if err != nil {
		return 0, fmt.Errorf("insert symptom_rca: %w", err)
	}
	return res.LastInsertId()
}

func (s *SqlStore) GetRCAsForSymptom(symptomID int64) ([]*SymptomRCA, error) {
	rows, err := s.db.Query(
		`SELECT id, symptom_id, rca_id, confidence, notes, linked_at
		 FROM symptom_rca WHERE symptom_id = ? ORDER BY id`, symptomID,
	)
	if err != nil {
		return nil, fmt.Errorf("get rcas for symptom: %w", err)
	}
	defer rows.Close()
	return scanSymptomRCAs(rows)
}

func (s *SqlStore) GetSymptomsForRCA(rcaID int64) ([]*SymptomRCA, error) {
	rows, err := s.db.Query(
		`SELECT id, symptom_id, rca_id, confidence, notes, linked_at
		 FROM symptom_rca WHERE rca_id = ? ORDER BY id`, rcaID,
	)
	if err != nil {
		return nil, fmt.Errorf("get symptoms for rca: %w", err)
	}
	defer rows.Close()
	return scanSymptomRCAs(rows)
}

// --- Scan helpers ---

// scanCases scans a rows result set into Case slices.
func scanCases(rows *sql.Rows) ([]*Case, error) {
	var out []*Case
	for rows.Next() {
		var c Case
		var rcaID, symptomID, jobID, logTrunc sql.NullInt64
		var polarionID, errMsg, logSnip, startedAt, endedAt sql.NullString
		if err := rows.Scan(&c.ID, &jobID, &c.LaunchID, &c.RPItemID,
			&c.Name, &polarionID, &c.Status,
			&symptomID, &rcaID, &errMsg, &logSnip, &logTrunc,
			&startedAt, &endedAt, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan case: %w", err)
		}
		c.JobID = jobID.Int64
		c.RCAID = rcaID.Int64
		c.SymptomID = symptomID.Int64
		c.PolarionID = nullStr(polarionID)
		c.ErrorMessage = nullStr(errMsg)
		c.LogSnippet = nullStr(logSnip)
		c.StartedAt = nullStr(startedAt)
		c.EndedAt = nullStr(endedAt)
		c.LogTruncated = logTrunc.Valid && logTrunc.Int64 == 1
		out = append(out, &c)
	}
	return out, rows.Err()
}

// scanRCAs scans a rows result set into RCA slices.
func scanRCAs(rows *sql.Rows) ([]*RCA, error) {
	var out []*RCA
	for rows.Next() {
		var r RCA
		var cat, comp, affVer, evRefs, jiraID, jiraLink sql.NullString
		var resolvedAt, verifiedAt, archivedAt sql.NullString
		var convScore sql.NullFloat64
		if err := rows.Scan(&r.ID, &r.Title, &r.Description, &r.DefectType,
			&cat, &comp, &affVer, &evRefs, &convScore,
			&jiraID, &jiraLink, &r.Status, &r.CreatedAt,
			&resolvedAt, &verifiedAt, &archivedAt); err != nil {
			return nil, fmt.Errorf("scan rca: %w", err)
		}
		r.Category = nullStr(cat)
		r.Component = nullStr(comp)
		r.AffectedVersions = nullStr(affVer)
		r.EvidenceRefs = nullStr(evRefs)
		r.ConvergenceScore = nullFloat(convScore)
		r.JiraTicketID = nullStr(jiraID)
		r.JiraLink = nullStr(jiraLink)
		r.ResolvedAt = nullStr(resolvedAt)
		r.VerifiedAt = nullStr(verifiedAt)
		r.ArchivedAt = nullStr(archivedAt)
		out = append(out, &r)
	}
	return out, rows.Err()
}

// scanSymptomRCAs scans a rows result set into SymptomRCA slices.
func scanSymptomRCAs(rows *sql.Rows) ([]*SymptomRCA, error) {
	var out []*SymptomRCA
	for rows.Next() {
		var link SymptomRCA
		var conf sql.NullFloat64
		var notes sql.NullString
		if err := rows.Scan(&link.ID, &link.SymptomID, &link.RCAID, &conf, &notes, &link.LinkedAt); err != nil {
			return nil, fmt.Errorf("scan symptom_rca: %w", err)
		}
		link.Confidence = nullFloat(conf)
		link.Notes = nullStr(notes)
		out = append(out, &link)
	}
	return out, rows.Err()
}

// --- nil helpers for optional SQL params ---

func nilIfEmpty(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

func nilIfZero(n int) interface{} {
	if n == 0 {
		return nil
	}
	return n
}

func nilIfZero64(n int64) interface{} {
	if n == 0 {
		return nil
	}
	return n
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
