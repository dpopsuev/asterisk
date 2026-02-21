package origami

import "asterisk/internal/calibrate"

// CompletenessResult scores a single case's readiness for verification.
type CompletenessResult struct {
	CaseID     string   `json:"case_id"`
	RCAID      string   `json:"rca_id"`
	Score      float64  `json:"score"`
	Present    []string `json:"present"`
	Missing    []string `json:"missing"`
	Promotable bool     `json:"promotable"`
}

// CheckCase evaluates a GroundTruthCase for completeness. All required fields
// must be present for a case to be promotable.
func CheckCase(c calibrate.GroundTruthCase, rcas []calibrate.GroundTruthRCA) CompletenessResult {
	r := CompletenessResult{
		CaseID: c.ID,
		RCAID:  c.RCAID,
	}

	checks := []struct {
		name    string
		present bool
	}{
		{"id", c.ID != ""},
		{"test_name", c.TestName != ""},
		{"error_message", c.ErrorMessage != ""},
		{"log_snippet", c.LogSnippet != ""},
		{"symptom_id", c.SymptomID != ""},
		{"rca_id", c.RCAID != ""},
		{"expected_path", len(c.ExpectedPath) > 0},
		{"expected_triage", c.ExpectedTriage != nil},
	}

	rca := findRCA(rcas, c.RCAID)
	if rca != nil {
		checks = append(checks, []struct {
			name    string
			present bool
		}{
			{"rca_defect_type", rca.DefectType != ""},
			{"rca_category", rca.Category != ""},
			{"rca_component", rca.Component != ""},
			{"rca_smoking_gun", rca.SmokingGun != ""},
		}...)
	} else {
		r.Missing = append(r.Missing, "rca_record")
	}

	total := len(checks)
	present := 0
	for _, ch := range checks {
		if ch.present {
			present++
			r.Present = append(r.Present, ch.name)
		} else {
			r.Missing = append(r.Missing, ch.name)
		}
	}

	if total > 0 {
		r.Score = float64(present) / float64(total)
	}
	r.Promotable = len(r.Missing) == 0

	return r
}

// CheckScenario evaluates all cases in a scenario.
func CheckScenario(s *calibrate.Scenario) []CompletenessResult {
	results := make([]CompletenessResult, 0, len(s.Cases))
	for _, c := range s.Cases {
		results = append(results, CheckCase(c, s.RCAs))
	}
	return results
}

func findRCA(rcas []calibrate.GroundTruthRCA, id string) *calibrate.GroundTruthRCA {
	for i := range rcas {
		if rcas[i].ID == id {
			return &rcas[i]
		}
	}
	return nil
}
