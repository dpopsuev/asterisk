package main

import (
	"asterisk/adapters/rca"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dpopsuev/origami/curate"
)

// CompletenessResult scores a single case's readiness for verification.
// It wraps curate.CompletenessResult with Asterisk-specific fields.
type CompletenessResult struct {
	CaseID     string   `json:"case_id"`
	RCAID      string   `json:"rca_id"`
	Score      float64  `json:"score"`
	Present    []string `json:"present"`
	Missing    []string `json:"missing"`
	Promotable bool     `json:"promotable"`
}

// CheckCase evaluates a GroundTruthCase for completeness using the
// Asterisk ground truth schema. All required fields must be present
// for a case to be promotable.
func CheckCase(c rca.GroundTruthCase, rcas []rca.GroundTruthRCA) CompletenessResult {
	record := GroundTruthCaseToRecord(c, rcas)
	schema := AsteriskSchema()
	cr := curate.CheckCompleteness(record, schema)

	rcaRec := findRCA(rcas, c.RCAID)

	result := CompletenessResult{
		CaseID:     c.ID,
		RCAID:      c.RCAID,
		Score:      cr.Score,
		Present:    cr.Present,
		Missing:    cr.Missing,
		Promotable: cr.Promotable,
	}

	if rcaRec == nil && c.RCAID != "" {
		result.Missing = append(result.Missing, "rca_record")
		result.Promotable = false
		if len(result.Present)+len(result.Missing) > 0 {
			total := len(result.Present) + len(result.Missing)
			result.Score = float64(len(result.Present)) / float64(total)
		}
	}

	return result
}

// CheckScenario evaluates all cases in a scenario.
func CheckScenario(s *rca.Scenario) []CompletenessResult {
	results := make([]CompletenessResult, 0, len(s.Cases))
	for _, c := range s.Cases {
		results = append(results, CheckCase(c, s.RCAs))
	}
	return results
}

// AsteriskSchema returns the curate.Schema that defines which fields a
// ground truth case needs for promotion. This replaces the hardcoded
// field checks in the old CheckCase implementation.
func AsteriskSchema() curate.Schema {
	nonEmpty := func(v any) bool {
		s, ok := v.(string)
		return ok && s != ""
	}
	nonEmptySlice := func(v any) bool {
		switch s := v.(type) {
		case []string:
			return len(s) > 0
		case []any:
			return len(s) > 0
		default:
			return false
		}
	}
	notNil := func(v any) bool { return v != nil }

	return curate.Schema{
		Name: "asterisk-ground-truth",
		Fields: []curate.FieldSpec{
			{Name: "id", Requirement: curate.Required, Validate: nonEmpty},
			{Name: "test_name", Requirement: curate.Required, Validate: nonEmpty},
			{Name: "error_message", Requirement: curate.Required, Validate: nonEmpty},
			{Name: "log_snippet", Requirement: curate.Required, Validate: nonEmpty},
			{Name: "symptom_id", Requirement: curate.Required, Validate: nonEmpty},
			{Name: "rca_id", Requirement: curate.Required, Validate: nonEmpty},
			{Name: "expected_path", Requirement: curate.Required, Validate: nonEmptySlice},
			{Name: "expected_triage", Requirement: curate.Required, Validate: notNil},
			{Name: "rca_defect_type", Requirement: curate.Required, Validate: nonEmpty},
			{Name: "rca_category", Requirement: curate.Required, Validate: nonEmpty},
			{Name: "rca_component", Requirement: curate.Required, Validate: nonEmpty},
			{Name: "rca_smoking_gun", Requirement: curate.Required, Validate: nonEmpty},
			{Name: "version", Requirement: curate.Optional},
			{Name: "job", Requirement: curate.Optional},
		},
	}
}

// GroundTruthCaseToRecord converts a rca.GroundTruthCase (with its
// matching RCA, if found) into a domain-agnostic curate.Record.
func GroundTruthCaseToRecord(c rca.GroundTruthCase, rcas []rca.GroundTruthRCA) curate.Record {
	r := curate.NewRecord(c.ID)

	set := func(name string, value any, source string) {
		r.Set(curate.Field{Name: name, Value: value, Source: source})
	}

	set("id", c.ID, "case")
	set("test_name", c.TestName, "case")
	set("error_message", c.ErrorMessage, "case")
	set("log_snippet", c.LogSnippet, "case")
	set("symptom_id", c.SymptomID, "case")
	set("rca_id", c.RCAID, "case")
	set("version", c.Version, "case")
	set("job", c.Job, "case")

	if len(c.ExpectedPath) > 0 {
		set("expected_path", c.ExpectedPath, "case")
	}
	if c.ExpectedTriage != nil {
		set("expected_triage", c.ExpectedTriage, "case")
	}

	rcaRec := findRCA(rcas, c.RCAID)
	if rcaRec != nil {
		set("rca_defect_type", rcaRec.DefectType, "rca")
		set("rca_category", rcaRec.Category, "rca")
		set("rca_component", rcaRec.Component, "rca")
		set("rca_smoking_gun", rcaRec.SmokingGun, "rca")
	}

	return r
}

// RecordToGroundTruthCase converts a curate.Record back to a
// rca.GroundTruthCase. Only string/primitive fields are mapped;
// complex nested types (ExpectedTriage, etc.) are not reconstructed.
func RecordToGroundTruthCase(r curate.Record) rca.GroundTruthCase {
	c := rca.GroundTruthCase{
		ID: r.ID,
	}
	if f, ok := r.Get("test_name"); ok {
		c.TestName, _ = f.Value.(string)
	}
	if f, ok := r.Get("error_message"); ok {
		c.ErrorMessage, _ = f.Value.(string)
	}
	if f, ok := r.Get("log_snippet"); ok {
		c.LogSnippet, _ = f.Value.(string)
	}
	if f, ok := r.Get("symptom_id"); ok {
		c.SymptomID, _ = f.Value.(string)
	}
	if f, ok := r.Get("rca_id"); ok {
		c.RCAID, _ = f.Value.(string)
	}
	if f, ok := r.Get("version"); ok {
		c.Version, _ = f.Value.(string)
	}
	if f, ok := r.Get("job"); ok {
		c.Job, _ = f.Value.(string)
	}
	if f, ok := r.Get("expected_path"); ok {
		if paths, ok := f.Value.([]string); ok {
			c.ExpectedPath = paths
		}
	}
	if f, ok := r.Get("expected_triage"); ok {
		if et, ok := f.Value.(*rca.ExpectedTriage); ok {
			c.ExpectedTriage = et
		}
	}

	return c
}

// ScenarioToDataset converts a rca.Scenario to a curate.Dataset.
func ScenarioToDataset(s *rca.Scenario) curate.Dataset {
	records := make([]curate.Record, 0, len(s.Cases))
	for _, c := range s.Cases {
		records = append(records, GroundTruthCaseToRecord(c, s.RCAs))
	}
	return curate.Dataset{
		Name:    s.Name,
		Records: records,
	}
}

// DatasetToScenario converts a curate.Dataset to a rca.Scenario.
// Only primitive case fields are reconstructed. RCAs are not recovered
// because they are not stored as separate records in the generic dataset.
func DatasetToScenario(d *curate.Dataset) *rca.Scenario {
	cases := make([]rca.GroundTruthCase, 0, len(d.Records))
	for _, r := range d.Records {
		cases = append(cases, RecordToGroundTruthCase(r))
	}
	return &rca.Scenario{
		Name:  d.Name,
		Cases: cases,
	}
}

func findRCA(rcas []rca.GroundTruthRCA, id string) *rca.GroundTruthRCA {
	for i := range rcas {
		if rcas[i].ID == id {
			return &rcas[i]
		}
	}
	return nil
}

// scenarioName extracts the scenario name from a rca.Scenario.
// Used internally when delegating to curate.FileStore.
func scenarioName(s *rca.Scenario) string {
	if s.Name != "" {
		return s.Name
	}
	return fmt.Sprintf("unnamed-%p", s)
}

// DatasetStore is the Asterisk-specific interface for ground truth persistence.
// It operates on rca.Scenario, which is the domain type that Asterisk's
// calibration circuit consumes.
type DatasetStore interface {
	List(ctx context.Context) ([]string, error)
	Load(ctx context.Context, name string) (*rca.Scenario, error)
	Save(ctx context.Context, s *rca.Scenario) error
}

// FileStore implements DatasetStore using JSON files in a directory.
// It stores rca.Scenario directly for backward compatibility with
// existing datasets, while the curate.FileStore can be used for generic
// curation datasets.
type FileStore struct {
	Dir string
}

func NewFileStore(dir string) *FileStore {
	return &FileStore{Dir: dir}
}

func (fs *FileStore) List(_ context.Context) ([]string, error) {
	entries, err := os.ReadDir(fs.Dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("list datasets: %w", err)
	}

	var names []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		names = append(names, strings.TrimSuffix(e.Name(), ".json"))
	}
	return names, nil
}

func (fs *FileStore) Load(_ context.Context, name string) (*rca.Scenario, error) {
	path := filepath.Join(fs.Dir, name+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("load dataset %q: %w", name, err)
	}

	var s rca.Scenario
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("parse dataset %q: %w", name, err)
	}
	return &s, nil
}

func (fs *FileStore) Save(_ context.Context, s *rca.Scenario) error {
	if err := os.MkdirAll(fs.Dir, 0o755); err != nil {
		return fmt.Errorf("create dataset dir: %w", err)
	}

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal dataset %q: %w", s.Name, err)
	}

	path := filepath.Join(fs.Dir, s.Name+".json")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write dataset %q: %w", s.Name, err)
	}

	return nil
}

// CurationStore returns a generic curate.Store that persists curate.Dataset
// objects. This is the bridge between Asterisk's origami adapter and the
// generic curation layer.
func CurationStore(dir string) (curate.Store, error) {
	return curate.NewFileStore(dir)
}
