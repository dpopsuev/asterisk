package dataset

import (
	"asterisk/adapters/rca"
	"github.com/dpopsuev/origami/curate"
	"testing"
)

func TestAsteriskSchema(t *testing.T) {
	s := AsteriskSchema()
	if s.Name != "asterisk-ground-truth" {
		t.Errorf("Name = %q", s.Name)
	}

	req := s.RequiredFields()
	if len(req) < 12 {
		t.Errorf("expected at least 12 required fields, got %d", len(req))
	}

	names := make(map[string]bool)
	for _, f := range req {
		names[f.Name] = true
	}
	for _, want := range []string{"id", "test_name", "error_message", "rca_id", "rca_defect_type", "rca_smoking_gun"} {
		if !names[want] {
			t.Errorf("missing required field %q", want)
		}
	}
}

func TestGroundTruthCaseToRecord(t *testing.T) {
	gtRCA := rca.GroundTruthRCA{
		ID: "R01", DefectType: "product_bug", Category: "pb001",
		Component: "linuxptp-daemon", SmokingGun: "commit abc123",
	}
	c := rca.GroundTruthCase{
		ID: "C01", TestName: "test_sync", ErrorMessage: "timeout",
		LogSnippet: "ptp4l[123]", SymptomID: "S01", RCAID: "R01",
		ExpectedPath:   []string{"F0", "F1", "F3"},
		ExpectedTriage: &rca.ExpectedTriage{DefectTypeHypothesis: "product_bug"},
		Version:        "4.20",
		Job:            "[T-TSC]",
	}

	r := GroundTruthCaseToRecord(c, []rca.GroundTruthRCA{gtRCA})

	if r.ID != "C01" {
		t.Errorf("ID = %q", r.ID)
	}

	assertField := func(name string, want any) {
		t.Helper()
		f, ok := r.Get(name)
		if !ok {
			t.Errorf("field %q missing", name)
			return
		}
		switch w := want.(type) {
		case string:
			if v, ok := f.Value.(string); !ok || v != w {
				t.Errorf("field %q = %v, want %q", name, f.Value, w)
			}
		}
	}

	assertField("test_name", "test_sync")
	assertField("error_message", "timeout")
	assertField("rca_defect_type", "product_bug")
	assertField("rca_component", "linuxptp-daemon")
	assertField("rca_smoking_gun", "commit abc123")
	assertField("version", "4.20")
	assertField("job", "[T-TSC]")

	if f, ok := r.Get("expected_path"); !ok {
		t.Error("expected_path missing")
	} else if paths, ok := f.Value.([]string); !ok || len(paths) != 3 {
		t.Errorf("expected_path = %v", f.Value)
	}

	if f, ok := r.Get("expected_triage"); !ok {
		t.Error("expected_triage missing")
	} else if f.Value == nil {
		t.Error("expected_triage should not be nil")
	}
}

func TestGroundTruthCaseToRecord_NoRCA(t *testing.T) {
	c := rca.GroundTruthCase{
		ID: "C01", TestName: "test", RCAID: "R99",
	}
	r := GroundTruthCaseToRecord(c, nil)

	if r.Has("rca_defect_type") {
		t.Error("rca_defect_type should not be present without matching RCA")
	}
}

func TestRecordToGroundTruthCase(t *testing.T) {
	r := curate.NewRecord("C01")
	r.Set(curate.Field{Name: "test_name", Value: "test_sync"})
	r.Set(curate.Field{Name: "error_message", Value: "timeout"})
	r.Set(curate.Field{Name: "symptom_id", Value: "S01"})
	r.Set(curate.Field{Name: "rca_id", Value: "R01"})
	r.Set(curate.Field{Name: "version", Value: "4.20"})
	r.Set(curate.Field{Name: "expected_path", Value: []string{"F0", "F1"}})

	c := RecordToGroundTruthCase(r)

	if c.ID != "C01" {
		t.Errorf("ID = %q", c.ID)
	}
	if c.TestName != "test_sync" {
		t.Errorf("TestName = %q", c.TestName)
	}
	if c.ErrorMessage != "timeout" {
		t.Errorf("ErrorMessage = %q", c.ErrorMessage)
	}
	if c.Version != "4.20" {
		t.Errorf("Version = %q", c.Version)
	}
	if len(c.ExpectedPath) != 2 {
		t.Errorf("ExpectedPath = %v", c.ExpectedPath)
	}
}

func TestScenarioToDataset(t *testing.T) {
	s := &rca.Scenario{
		Name: "test-scenario",
		Cases: []rca.GroundTruthCase{
			{ID: "C01", TestName: "test_one"},
			{ID: "C02", TestName: "test_two"},
		},
		RCAs: []rca.GroundTruthRCA{
			{ID: "R01", DefectType: "product_bug"},
		},
	}

	ds := ScenarioToDataset(s)
	if ds.Name != "test-scenario" {
		t.Errorf("Name = %q", ds.Name)
	}
	if len(ds.Records) != 2 {
		t.Fatalf("len(Records) = %d, want 2", len(ds.Records))
	}
	if ds.Records[0].ID != "C01" {
		t.Errorf("Records[0].ID = %q", ds.Records[0].ID)
	}
}

func TestDatasetToScenario(t *testing.T) {
	r1 := curate.NewRecord("C01")
	r1.Set(curate.Field{Name: "test_name", Value: "test_one"})
	r2 := curate.NewRecord("C02")
	r2.Set(curate.Field{Name: "test_name", Value: "test_two"})

	ds := &curate.Dataset{
		Name:    "test-dataset",
		Records: []curate.Record{r1, r2},
	}

	s := DatasetToScenario(ds)
	if s.Name != "test-dataset" {
		t.Errorf("Name = %q", s.Name)
	}
	if len(s.Cases) != 2 {
		t.Fatalf("len(Cases) = %d, want 2", len(s.Cases))
	}
	if s.Cases[0].ID != "C01" {
		t.Errorf("Cases[0].ID = %q", s.Cases[0].ID)
	}
	if s.Cases[0].TestName != "test_one" {
		t.Errorf("Cases[0].TestName = %q", s.Cases[0].TestName)
	}
}

func TestRoundTrip_CaseToRecordAndBack(t *testing.T) {
	original := rca.GroundTruthCase{
		ID: "C01", TestName: "test_sync", ErrorMessage: "timeout",
		LogSnippet: "log", SymptomID: "S01", RCAID: "R01",
		Version: "4.20", Job: "[T-TSC]",
		ExpectedPath: []string{"F0", "F1"},
	}

	record := GroundTruthCaseToRecord(original, nil)
	recovered := RecordToGroundTruthCase(record)

	if recovered.ID != original.ID {
		t.Errorf("ID: %q != %q", recovered.ID, original.ID)
	}
	if recovered.TestName != original.TestName {
		t.Errorf("TestName: %q != %q", recovered.TestName, original.TestName)
	}
	if recovered.ErrorMessage != original.ErrorMessage {
		t.Errorf("ErrorMessage: %q != %q", recovered.ErrorMessage, original.ErrorMessage)
	}
	if recovered.Version != original.Version {
		t.Errorf("Version: %q != %q", recovered.Version, original.Version)
	}
	if len(recovered.ExpectedPath) != len(original.ExpectedPath) {
		t.Errorf("ExpectedPath len: %d != %d", len(recovered.ExpectedPath), len(original.ExpectedPath))
	}
}

func TestSchemaCompleteness_ViaMapper(t *testing.T) {
	gtRCA := rca.GroundTruthRCA{
		ID: "R01", DefectType: "product_bug", Category: "pb001",
		Component: "linuxptp-daemon", SmokingGun: "commit abc123",
	}
	c := rca.GroundTruthCase{
		ID: "C01", TestName: "test", ErrorMessage: "fail", LogSnippet: "log",
		SymptomID: "S01", RCAID: "R01", ExpectedPath: []string{"F0", "F1"},
		ExpectedTriage: &rca.ExpectedTriage{DefectTypeHypothesis: "product_bug"},
	}

	record := GroundTruthCaseToRecord(c, []rca.GroundTruthRCA{gtRCA})
	schema := AsteriskSchema()
	result := curate.CheckCompleteness(record, schema)

	if !result.Promotable {
		t.Errorf("should be promotable, missing: %v, invalid: %v", result.Missing, result.Invalid)
	}
	if result.Score != 1.0 {
		t.Errorf("Score = %.2f, want 1.0", result.Score)
	}
}
