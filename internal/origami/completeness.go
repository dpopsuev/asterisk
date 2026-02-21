package origami

import (
	"asterisk/internal/calibrate"
	"asterisk/internal/curate"
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
func CheckCase(c calibrate.GroundTruthCase, rcas []calibrate.GroundTruthRCA) CompletenessResult {
	record := GroundTruthCaseToRecord(c, rcas)
	schema := AsteriskSchema()
	cr := curate.CheckCompleteness(record, schema)

	rca := findRCA(rcas, c.RCAID)

	result := CompletenessResult{
		CaseID:     c.ID,
		RCAID:      c.RCAID,
		Score:      cr.Score,
		Present:    cr.Present,
		Missing:    cr.Missing,
		Promotable: cr.Promotable,
	}

	if rca == nil && c.RCAID != "" {
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
func CheckScenario(s *calibrate.Scenario) []CompletenessResult {
	results := make([]CompletenessResult, 0, len(s.Cases))
	for _, c := range s.Cases {
		results = append(results, CheckCase(c, s.RCAs))
	}
	return results
}
