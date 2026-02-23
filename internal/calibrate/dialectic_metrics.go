package calibrate

// DialecticMetrics aggregates Shadow dialectic pipeline statistics.
type DialecticMetrics struct {
	CasesActivated        int     `json:"cases_activated"`
	SynthesisFlipRate     float64 `json:"synthesis_flip_rate"`
	AffirmCount           int     `json:"affirm_count"`
	AmendCount            int     `json:"amend_count"`
	AcquitCount           int     `json:"acquit_count"`
	NegationCount         int     `json:"negation_count"`
	UnresolvedCount       int     `json:"unresolved_count"`
	NegationEffectiveness float64 `json:"negation_effectiveness"`
	SynthesisAccuracy     float64 `json:"synthesis_accuracy"`
}

// ComputeDialecticMetrics calculates dialectic-specific metrics from case results.
func ComputeDialecticMetrics(results []CaseResult, gtCases []GroundTruthCase) DialecticMetrics {
	m := DialecticMetrics{}

	gtMap := make(map[string]GroundTruthCase, len(gtCases))
	for _, gt := range gtCases {
		gtMap[gt.ID] = gt
	}

	var dialecticCases int
	var flips int
	var synthesisCorrect int
	var totalNegations int
	var negationsImproved int

	for _, r := range results {
		if !r.DialecticActivated {
			continue
		}
		dialecticCases++

		switch r.DialecticSynthesis {
		case "affirm":
			m.AffirmCount++
		case "amend":
			m.AmendCount++
		case "acquit":
			m.AcquitCount++
		case "remand":
			m.NegationCount++
		case "unresolved":
			m.UnresolvedCount++
		}

		if r.DialecticFlipped {
			flips++
		}

		if r.DialecticNegations > 0 {
			totalNegations += r.DialecticNegations
			if r.DialecticFlipped && r.DialecticFinalDefect != "" {
				gt, ok := gtMap[r.CaseID]
				if ok && gt.ExpectedSynthesis != "" {
					if r.DialecticSynthesis == gt.ExpectedSynthesis {
						negationsImproved++
					}
				}
			}
		}

		gt, ok := gtMap[r.CaseID]
		if ok && gt.ExpectedSynthesis != "" && r.DialecticSynthesis == gt.ExpectedSynthesis {
			synthesisCorrect++
		}
	}

	m.CasesActivated = dialecticCases

	if dialecticCases > 0 {
		m.SynthesisFlipRate = float64(flips) / float64(dialecticCases)
	}

	if totalNegations > 0 {
		m.NegationEffectiveness = float64(negationsImproved) / float64(totalNegations)
	}

	casesWithExpected := 0
	for _, r := range results {
		if !r.DialecticActivated {
			continue
		}
		gt, ok := gtMap[r.CaseID]
		if ok && gt.ExpectedSynthesis != "" {
			casesWithExpected++
		}
	}
	if casesWithExpected > 0 {
		m.SynthesisAccuracy = float64(synthesisCorrect) / float64(casesWithExpected)
	}

	return m
}
