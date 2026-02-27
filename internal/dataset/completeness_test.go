package dataset

import (
	"asterisk/adapters/rca"
	"testing"
)

func TestCheckCase_FullyComplete(t *testing.T) {
	gtRCA := rca.GroundTruthRCA{
		ID: "R01", DefectType: "product_bug", Category: "pb001",
		Component: "linuxptp-daemon", SmokingGun: "commit abc123",
	}
	c := rca.GroundTruthCase{
		ID: "C01", TestName: "test", ErrorMessage: "fail", LogSnippet: "log",
		SymptomID: "S01", RCAID: "R01", ExpectedPath: []string{"F0", "F1"},
		ExpectedTriage: &rca.ExpectedTriage{DefectTypeHypothesis: "product_bug"},
	}
	r := CheckCase(c, []rca.GroundTruthRCA{gtRCA})
	if !r.Promotable {
		t.Errorf("expected promotable, missing: %v", r.Missing)
	}
	if r.Score != 1.0 {
		t.Errorf("Score = %f, want 1.0", r.Score)
	}
	if len(r.Missing) != 0 {
		t.Errorf("Missing = %v, want empty", r.Missing)
	}
}

func TestCheckCase_MissingFields(t *testing.T) {
	c := rca.GroundTruthCase{
		ID:       "C01",
		TestName: "test",
	}
	r := CheckCase(c, nil)
	if r.Promotable {
		t.Error("should not be promotable with missing fields")
	}
	if r.Score >= 1.0 {
		t.Errorf("Score = %f, should be less than 1.0", r.Score)
	}
	if len(r.Missing) == 0 {
		t.Error("expected some missing fields")
	}
}

func TestCheckCase_MissingRCA(t *testing.T) {
	c := rca.GroundTruthCase{
		ID: "C01", TestName: "test", ErrorMessage: "fail", LogSnippet: "log",
		SymptomID: "S01", RCAID: "R99", ExpectedPath: []string{"F0"},
		ExpectedTriage: &rca.ExpectedTriage{},
	}
	r := CheckCase(c, nil)
	if r.Promotable {
		t.Error("should not be promotable without matching RCA")
	}
	found := false
	for _, m := range r.Missing {
		if m == "rca_record" {
			found = true
		}
	}
	if !found {
		t.Error("expected 'rca_record' in missing list")
	}
}

func TestCheckScenario(t *testing.T) {
	s := &rca.Scenario{
		Cases: []rca.GroundTruthCase{
			{ID: "C01"},
			{ID: "C02"},
			{ID: "C03"},
		},
	}
	results := CheckScenario(s)
	if len(results) != 3 {
		t.Errorf("expected 3 results, got %d", len(results))
	}
}
