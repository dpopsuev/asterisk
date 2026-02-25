package dataset

import (
	"asterisk/internal/calibrate"
	"github.com/dpopsuev/origami/curate"
	"fmt"
)

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

// GroundTruthCaseToRecord converts a calibrate.GroundTruthCase (with its
// matching RCA, if found) into a domain-agnostic curate.Record.
func GroundTruthCaseToRecord(c calibrate.GroundTruthCase, rcas []calibrate.GroundTruthRCA) curate.Record {
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

	rca := findRCA(rcas, c.RCAID)
	if rca != nil {
		set("rca_defect_type", rca.DefectType, "rca")
		set("rca_category", rca.Category, "rca")
		set("rca_component", rca.Component, "rca")
		set("rca_smoking_gun", rca.SmokingGun, "rca")
	}

	return r
}

// RecordToGroundTruthCase converts a curate.Record back to a
// calibrate.GroundTruthCase. Only string/primitive fields are mapped;
// complex nested types (ExpectedTriage, etc.) are not reconstructed.
func RecordToGroundTruthCase(r curate.Record) calibrate.GroundTruthCase {
	c := calibrate.GroundTruthCase{
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
		if et, ok := f.Value.(*calibrate.ExpectedTriage); ok {
			c.ExpectedTriage = et
		}
	}

	return c
}

// ScenarioToDataset converts a calibrate.Scenario to a curate.Dataset.
func ScenarioToDataset(s *calibrate.Scenario) curate.Dataset {
	records := make([]curate.Record, 0, len(s.Cases))
	for _, c := range s.Cases {
		records = append(records, GroundTruthCaseToRecord(c, s.RCAs))
	}
	return curate.Dataset{
		Name:    s.Name,
		Records: records,
	}
}

// DatasetToScenario converts a curate.Dataset to a calibrate.Scenario.
// Only primitive case fields are reconstructed. RCAs are not recovered
// because they are not stored as separate records in the generic dataset.
func DatasetToScenario(d *curate.Dataset) *calibrate.Scenario {
	cases := make([]calibrate.GroundTruthCase, 0, len(d.Records))
	for _, r := range d.Records {
		cases = append(cases, RecordToGroundTruthCase(r))
	}
	return &calibrate.Scenario{
		Name:  d.Name,
		Cases: cases,
	}
}

func findRCA(rcas []calibrate.GroundTruthRCA, id string) *calibrate.GroundTruthRCA {
	for i := range rcas {
		if rcas[i].ID == id {
			return &rcas[i]
		}
	}
	return nil
}

// scenarioName extracts the scenario name from a calibrate.Scenario.
// Used internally when delegating to curate.FileStore.
func scenarioName(s *calibrate.Scenario) string {
	if s.Name != "" {
		return s.Name
	}
	return fmt.Sprintf("unnamed-%p", s)
}
