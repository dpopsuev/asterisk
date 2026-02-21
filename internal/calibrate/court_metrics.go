package calibrate

// CourtMetrics aggregates Shadow court pipeline statistics.
type CourtMetrics struct {
	CasesActivated      int     `json:"cases_activated"`
	VerdictFlipRate     float64 `json:"verdict_flip_rate"`
	AffirmCount         int     `json:"affirm_count"`
	AmendCount          int     `json:"amend_count"`
	AcquitCount         int     `json:"acquit_count"`
	RemandCount         int     `json:"remand_count"`
	MistrialCount       int     `json:"mistrial_count"`
	RemandEffectiveness float64 `json:"remand_effectiveness"`
	VerdictAccuracy     float64 `json:"verdict_accuracy"`
}

// ComputeCourtMetrics calculates court-specific metrics from case results.
func ComputeCourtMetrics(results []CaseResult, gtCases []GroundTruthCase) CourtMetrics {
	m := CourtMetrics{}

	gtMap := make(map[string]GroundTruthCase, len(gtCases))
	for _, gt := range gtCases {
		gtMap[gt.ID] = gt
	}

	var courtCases int
	var flips int
	var verdictCorrect int
	var totalRemands int
	var remandsImproved int

	for _, r := range results {
		if !r.CourtActivated {
			continue
		}
		courtCases++

		switch r.CourtVerdict {
		case "affirm":
			m.AffirmCount++
		case "amend":
			m.AmendCount++
		case "acquit":
			m.AcquitCount++
		case "remand":
			m.RemandCount++
		case "mistrial":
			m.MistrialCount++
		}

		if r.CourtFlipped {
			flips++
		}

		if r.CourtRemands > 0 {
			totalRemands += r.CourtRemands
			if r.CourtFlipped && r.CourtFinalDefect != "" {
				gt, ok := gtMap[r.CaseID]
				if ok && gt.ExpectedVerdict != "" {
					if r.CourtVerdict == gt.ExpectedVerdict {
						remandsImproved++
					}
				}
			}
		}

		gt, ok := gtMap[r.CaseID]
		if ok && gt.ExpectedVerdict != "" && r.CourtVerdict == gt.ExpectedVerdict {
			verdictCorrect++
		}
	}

	m.CasesActivated = courtCases

	if courtCases > 0 {
		m.VerdictFlipRate = float64(flips) / float64(courtCases)
	}

	if totalRemands > 0 {
		m.RemandEffectiveness = float64(remandsImproved) / float64(totalRemands)
	}

	casesWithExpected := 0
	for _, r := range results {
		if !r.CourtActivated {
			continue
		}
		gt, ok := gtMap[r.CaseID]
		if ok && gt.ExpectedVerdict != "" {
			casesWithExpected++
		}
	}
	if casesWithExpected > 0 {
		m.VerdictAccuracy = float64(verdictCorrect) / float64(casesWithExpected)
	}

	return m
}
