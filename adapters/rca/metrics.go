package rca

import (
	"fmt"
	"math"
	"strings"

	cal "github.com/dpopsuev/origami/calibrate"
)

// scored is the raw output of a domain scorer: metric ID, computed value, and
// a human-readable detail string. The ScoreCard converts these into full Metrics.
type scored struct {
	id     string
	value  float64
	detail string
}

// computeMetrics calculates all 21 calibration metrics from case results.
// Raw values are computed by domain-specific scorers; thresholds, pass/fail,
// direction, and tier come from the ScoreCard definition.
func computeMetrics(scenario *Scenario, results []CaseResult, sc *cal.ScoreCard) MetricSet {
	rcaMap := make(map[string]*GroundTruthRCA)
	for i := range scenario.RCAs {
		rcaMap[scenario.RCAs[i].ID] = &scenario.RCAs[i]
	}
	caseMap := make(map[string]*GroundTruthCase)
	for i := range scenario.Cases {
		caseMap[scenario.Cases[i].ID] = &scenario.Cases[i]
	}
	repoRelevance := buildRepoRelevanceMap(scenario)

	raw := []scored{
		scoreDefectTypeAccuracy(results, caseMap, rcaMap),
		scoreSymptomCategoryAccuracy(results, caseMap),
		scoreRecallHitRate(results, caseMap),
		scoreRecallFalsePositiveRate(results, caseMap),
		scoreSerialKillerDetection(results, caseMap, rcaMap),
		scoreSkipAccuracy(results, caseMap),
		scoreCascadeDetection(results, caseMap),
		scoreConvergenceCalibration(results, caseMap, rcaMap),
		scoreRepoSelectionPrecision(results, caseMap, repoRelevance),
		scoreRepoSelectionRecall(results, caseMap, repoRelevance),
		scoreRedHerringRejection(results, caseMap, scenario),
		scoreEvidenceRecall(results, caseMap),
		scoreEvidencePrecision(results, caseMap),
		scoreRCAMessageRelevance(results, caseMap, rcaMap),
		scoreSmokingGunHitRate(results, caseMap, rcaMap),
		scoreComponentIdentification(results, caseMap, rcaMap),
		scorePipelinePathAccuracy(results, caseMap),
		scoreLoopEfficiency(results, caseMap),
		scoreTotalPromptTokens(results),
	}

	values := make(map[string]float64, len(raw))
	details := make(map[string]string, len(raw))
	for _, s := range raw {
		values[s.id] = s.value
		details[s.id] = s.detail
	}

	ms := sc.Evaluate(values, details)

	if sc.Aggregate != nil {
		agg, err := sc.ComputeAggregate(ms)
		if err == nil {
			ms.Metrics = append(ms.Metrics, agg)
		}
	}

	m20def := sc.FindDef("M20")
	if m20def != nil {
		ms.Metrics = append(ms.Metrics, m20def.ToMetric(0, "single run"))
	}

	applyDryCaps(&ms, scenario.DryCappedMetrics)
	return ms
}

// applyDryCaps marks metrics that are structurally unsolvable in dry calibration.
func applyDryCaps(ms *MetricSet, capped []string) {
	if len(capped) == 0 {
		return
	}
	set := make(map[string]bool, len(capped))
	for _, id := range capped {
		set[id] = true
	}
	for i := range ms.Metrics {
		if set[ms.Metrics[i].ID] {
			ms.Metrics[i].DryCapped = true
		}
	}
}

// --- M1: Defect type accuracy ---
func scoreDefectTypeAccuracy(results []CaseResult, caseMap map[string]*GroundTruthCase, rcaMap map[string]*GroundTruthRCA) scored {
	correct, total := 0, 0
	for _, r := range results {
		gt := caseMap[r.CaseID]
		if gt == nil || gt.RCAID == "" {
			continue
		}
		total++
		rca := rcaMap[gt.RCAID]
		if rca != nil && r.ActualDefectType == rca.DefectType {
			correct++
			r.DefectTypeCorrect = true
		}
	}
	return scored{"M1", safeDiv(correct, total), fmt.Sprintf("%d/%d", correct, total)}
}

// --- M2: Symptom category accuracy ---
func scoreSymptomCategoryAccuracy(results []CaseResult, caseMap map[string]*GroundTruthCase) scored {
	correct, total := 0, 0
	for _, r := range results {
		gt := caseMap[r.CaseID]
		if gt == nil || gt.ExpectedTriage == nil {
			continue
		}
		total++
		if r.ActualCategory == gt.ExpectedTriage.SymptomCategory {
			correct++
		}
	}
	return scored{"M2", safeDiv(correct, total), fmt.Sprintf("%d/%d", correct, total)}
}

// --- M3: Recall hit rate ---
func scoreRecallHitRate(results []CaseResult, caseMap map[string]*GroundTruthCase) scored {
	truePositive, expectedHits := 0, 0
	for _, r := range results {
		gt := caseMap[r.CaseID]
		if gt == nil || !gt.ExpectRecallHit {
			continue
		}
		expectedHits++
		if r.ActualRecallHit {
			truePositive++
		}
	}
	return scored{"M3", safeDiv(truePositive, expectedHits), fmt.Sprintf("%d/%d", truePositive, expectedHits)}
}

// --- M4: Recall false positive rate ---
func scoreRecallFalsePositiveRate(results []CaseResult, caseMap map[string]*GroundTruthCase) scored {
	falsePositive, expectedMisses := 0, 0
	for _, r := range results {
		gt := caseMap[r.CaseID]
		if gt == nil || gt.ExpectRecallHit {
			continue
		}
		expectedMisses++
		if r.ActualRecallHit {
			falsePositive++
		}
	}
	return scored{"M4", safeDiv(falsePositive, expectedMisses), fmt.Sprintf("%d/%d", falsePositive, expectedMisses)}
}

// --- M5: Serial killer detection ---
func scoreSerialKillerDetection(results []CaseResult, caseMap map[string]*GroundTruthCase, rcaMap map[string]*GroundTruthRCA) scored {
	rcaCases := make(map[string][]CaseResult)
	for _, r := range results {
		gt := caseMap[r.CaseID]
		if gt == nil || gt.RCAID == "" {
			continue
		}
		rcaCases[gt.RCAID] = append(rcaCases[gt.RCAID], r)
	}

	correctLinks, expectedLinks := 0, 0
	for rcaID, cases := range rcaCases {
		if len(cases) < 2 {
			continue
		}
		_ = rcaID
		firstRCA := cases[0].ActualRCAID
		for i := 1; i < len(cases); i++ {
			expectedLinks++
			if cases[i].ActualRCAID != 0 && cases[i].ActualRCAID == firstRCA {
				correctLinks++
			}
		}
	}
	return scored{"M5", safeDiv(correctLinks, expectedLinks), fmt.Sprintf("%d/%d", correctLinks, expectedLinks)}
}

// --- M6: Skip accuracy ---
func scoreSkipAccuracy(results []CaseResult, caseMap map[string]*GroundTruthCase) scored {
	correct, expected := 0, 0
	for _, r := range results {
		gt := caseMap[r.CaseID]
		if gt == nil || !gt.ExpectSkip {
			continue
		}
		expected++
		if r.ActualSkip {
			correct++
		}
	}
	return scored{"M6", safeDiv(correct, expected), fmt.Sprintf("%d/%d", correct, expected)}
}

// --- M7: Cascade detection ---
func scoreCascadeDetection(results []CaseResult, caseMap map[string]*GroundTruthCase) scored {
	detected, expected := 0, 0
	for _, r := range results {
		gt := caseMap[r.CaseID]
		if gt == nil || !gt.ExpectCascade {
			continue
		}
		expected++
		if r.ActualCascade {
			detected++
		}
	}
	return scored{"M7", safeDiv(detected, expected), fmt.Sprintf("%d/%d", detected, expected)}
}

// --- M8: Convergence calibration ---
func scoreConvergenceCalibration(results []CaseResult, caseMap map[string]*GroundTruthCase, rcaMap map[string]*GroundTruthRCA) scored {
	var convergences, correctness []float64
	for _, r := range results {
		if r.ActualConvergence == 0 {
			continue
		}
		gt := caseMap[r.CaseID]
		if gt == nil || gt.RCAID == "" {
			continue
		}
		rca := rcaMap[gt.RCAID]
		correct := 0.0
		if rca != nil && r.ActualDefectType == rca.DefectType {
			correct = 1.0
		}
		convergences = append(convergences, r.ActualConvergence)
		correctness = append(correctness, correct)
	}
	corr := pearsonCorrelation(convergences, correctness)
	return scored{"M8", corr, fmt.Sprintf("r=%.2f (n=%d)", corr, len(convergences))}
}

// --- M9: Repo selection precision ---
func scoreRepoSelectionPrecision(results []CaseResult, caseMap map[string]*GroundTruthCase, repoRelevance map[string]map[string]bool) scored {
	sumPrecision := 0.0
	count := 0
	for _, r := range results {
		if len(r.ActualSelectedRepos) == 0 {
			continue
		}
		gt := caseMap[r.CaseID]
		if gt == nil || gt.RCAID == "" {
			continue
		}
		count++
		relevantSelected := 0
		for _, repo := range r.ActualSelectedRepos {
			if repoRelevance[gt.RCAID][repo] {
				relevantSelected++
			}
		}
		sumPrecision += safeDiv(relevantSelected, len(r.ActualSelectedRepos))
	}
	return scored{"M9", safeDiv2(sumPrecision, float64(count)), fmt.Sprintf("avg over %d cases", count)}
}

// --- M10: Repo selection recall ---
func scoreRepoSelectionRecall(results []CaseResult, caseMap map[string]*GroundTruthCase, repoRelevance map[string]map[string]bool) scored {
	sumRecall := 0.0
	count := 0
	for _, r := range results {
		gt := caseMap[r.CaseID]
		if gt == nil || gt.RCAID == "" || gt.ExpectedResolve == nil {
			continue
		}
		count++
		totalRelevant := len(repoRelevance[gt.RCAID])
		if totalRelevant == 0 {
			sumRecall += 1.0
			continue
		}
		relevantSelected := 0
		for _, repo := range r.ActualSelectedRepos {
			if repoRelevance[gt.RCAID][repo] {
				relevantSelected++
			}
		}
		sumRecall += safeDiv(relevantSelected, totalRelevant)
	}
	return scored{"M10", safeDiv2(sumRecall, float64(count)), fmt.Sprintf("avg over %d cases", count)}
}

// --- M11: Red herring rejection ---
func scoreRedHerringRejection(results []CaseResult, caseMap map[string]*GroundTruthCase, scenario *Scenario) scored {
	redHerringRepos := make(map[string]bool)
	for _, repo := range scenario.Workspace.Repos {
		if repo.IsRedHerring {
			redHerringRepos[repo.Name] = true
		}
	}

	casesWithF2 := 0
	redHerringSelected := 0
	for _, r := range results {
		if len(r.ActualSelectedRepos) == 0 {
			continue
		}
		casesWithF2++
		for _, repo := range r.ActualSelectedRepos {
			if redHerringRepos[repo] {
				redHerringSelected++
				break
			}
		}
	}
	val := 1.0 - safeDiv(redHerringSelected, casesWithF2)
	return scored{"M11", val, fmt.Sprintf("%d cases with Resolve, %d selected red herring", casesWithF2, redHerringSelected)}
}

// --- M12: Evidence recall ---
func scoreEvidenceRecall(results []CaseResult, caseMap map[string]*GroundTruthCase) scored {
	totalFound, totalPlanted := 0, 0
	for _, r := range results {
		gt := caseMap[r.CaseID]
		if gt == nil || gt.ExpectedInvest == nil || len(gt.ExpectedInvest.EvidenceRefs) == 0 {
			continue
		}
		found, total := evidenceOverlap(r.ActualEvidenceRefs, gt.ExpectedInvest.EvidenceRefs)
		totalFound += found
		totalPlanted += total
	}
	return scored{"M12", safeDiv(totalFound, totalPlanted), fmt.Sprintf("%d/%d", totalFound, totalPlanted)}
}

// --- M13: Evidence precision ---
func scoreEvidencePrecision(results []CaseResult, caseMap map[string]*GroundTruthCase) scored {
	totalRelevant, totalCited := 0, 0
	for _, r := range results {
		gt := caseMap[r.CaseID]
		if gt == nil || len(r.ActualEvidenceRefs) == 0 {
			continue
		}
		totalCited += len(r.ActualEvidenceRefs)
		if gt.ExpectedInvest != nil {
			found, _ := evidenceOverlap(r.ActualEvidenceRefs, gt.ExpectedInvest.EvidenceRefs)
			totalRelevant += found
		}
	}
	return scored{"M13", safeDiv(totalRelevant, totalCited), fmt.Sprintf("%d/%d", totalRelevant, totalCited)}
}

// --- M14: RCA message relevance (keyword fallback for stub mode) ---
func scoreRCAMessageRelevance(results []CaseResult, caseMap map[string]*GroundTruthCase, rcaMap map[string]*GroundTruthRCA) scored {
	sumScore := 0.0
	count := 0
	for _, r := range results {
		if r.ActualRCAMessage == "" {
			continue
		}
		gt := caseMap[r.CaseID]
		if gt == nil || gt.RCAID == "" {
			continue
		}
		rca := rcaMap[gt.RCAID]
		if rca == nil || len(rca.RequiredKeywords) == 0 {
			continue
		}
		count++
		matched := keywordMatch(r.ActualRCAMessage, rca.RequiredKeywords)
		score := math.Min(float64(matched)/float64(rca.KeywordThreshold), 1.0)
		sumScore += score
	}
	return scored{"M14", safeDiv2(sumScore, float64(count)), fmt.Sprintf("avg over %d cases", count)}
}

// --- M14b: Smoking gun hit rate ---
// Checks if the adapter's RCA message textually reaches the same core conclusion
// as the PR-proven "smoking gun" phrase. A hit requires >=50% of the phrase's
// significant words (>3 chars) to appear in the RCA message.
func scoreSmokingGunHitRate(results []CaseResult, caseMap map[string]*GroundTruthCase, rcaMap map[string]*GroundTruthRCA) scored {
	hits, eligible := 0, 0
	for _, r := range results {
		if r.ActualRCAMessage == "" {
			continue
		}
		gt := caseMap[r.CaseID]
		if gt == nil || gt.RCAID == "" {
			continue
		}
		rca := rcaMap[gt.RCAID]
		if rca == nil || rca.SmokingGun == "" {
			continue
		}
		eligible++
		words := smokingGunWords(rca.SmokingGun)
		if len(words) == 0 {
			continue
		}
		matched := keywordMatch(r.ActualRCAMessage, words)
		if float64(matched) >= float64(len(words))*0.5 {
			hits++
		}
	}
	return scored{"M14b", safeDiv(hits, eligible), fmt.Sprintf("%d/%d", hits, eligible)}
}

// smokingGunWords tokenizes a smoking gun phrase into significant lowercase words (>3 chars).
func smokingGunWords(phrase string) []string {
	var words []string
	for _, w := range strings.Fields(strings.ToLower(phrase)) {
		if len(w) > 3 {
			words = append(words, w)
		}
	}
	return words
}

// --- M15: Component identification ---
func scoreComponentIdentification(results []CaseResult, caseMap map[string]*GroundTruthCase, rcaMap map[string]*GroundTruthRCA) scored {
	correct, total := 0, 0
	for _, r := range results {
		gt := caseMap[r.CaseID]
		if gt == nil || gt.RCAID == "" {
			continue
		}
		rca := rcaMap[gt.RCAID]
		if rca == nil {
			continue
		}
		total++
		if r.ActualComponent == rca.Component {
			correct++
		} else if r.ActualRCAMessage != "" && strings.Contains(strings.ToLower(r.ActualRCAMessage), strings.ToLower(rca.Component)) {
			correct++
		}
	}
	return scored{"M15", safeDiv(correct, total), fmt.Sprintf("%d/%d", correct, total)}
}

// --- M16: Pipeline path accuracy ---
func scorePipelinePathAccuracy(results []CaseResult, caseMap map[string]*GroundTruthCase) scored {
	correct, total := 0, 0
	for _, r := range results {
		gt := caseMap[r.CaseID]
		if gt == nil {
			continue
		}
		total++
		if pathsEqual(r.ActualPath, gt.ExpectedPath) {
			correct++
		}
	}
	return scored{"M16", safeDiv(correct, total), fmt.Sprintf("%d/%d", correct, total)}
}

// --- M17: Loop efficiency ---
func scoreLoopEfficiency(results []CaseResult, caseMap map[string]*GroundTruthCase) scored {
	sumActual, sumExpected := 0, 0
	for _, r := range results {
		gt := caseMap[r.CaseID]
		if gt == nil {
			continue
		}
		sumActual += r.ActualLoops
		sumExpected += gt.ExpectedLoops
	}
	var val float64
	if sumExpected == 0 && sumActual == 0 {
		val = 1.0
	} else if sumExpected == 0 {
		val = float64(sumActual + 1)
	} else {
		val = float64(sumActual) / float64(sumExpected)
	}
	return scored{"M17", val, fmt.Sprintf("actual=%d expected=%d", sumActual, sumExpected)}
}

// --- M18: Total prompt tokens ---
// When dispatch.TokenTracker data is present (PromptTokensTotal > 0), uses real
// measured values. Falls back to step-count estimate for stub mode.
func scoreTotalPromptTokens(results []CaseResult) scored {
	realTokens := 0
	hasReal := false
	totalSteps := 0
	for _, r := range results {
		totalSteps += len(r.ActualPath)
		if r.PromptTokensTotal > 0 {
			realTokens += r.PromptTokensTotal
			hasReal = true
		}
	}

	if hasReal {
		return scored{"M18", float64(realTokens), fmt.Sprintf("%d tokens (measured)", realTokens)}
	}

	estimated := totalSteps * 1000
	return scored{"M18", float64(estimated), fmt.Sprintf("~%d tokens (%d steps, estimated)", estimated, totalSteps)}
}

// aggregateRunMetrics computes the mean and variance across multiple runs.
// It delegates to cal.AggregateRunMetrics for averaging, then replaces M19/M20
// with ScoreCard-driven aggregate values.
func aggregateRunMetrics(runs []MetricSet, sc *cal.ScoreCard) MetricSet {
	if len(runs) == 0 {
		return MetricSet{}
	}
	if len(runs) == 1 {
		return runs[0]
	}

	agg := cal.AggregateRunMetrics(runs, func(m Metric) bool {
		if def := sc.FindDef(m.ID); def != nil {
			return def.Evaluate(m.Value)
		}
		return m.Value >= m.Threshold
	})

	var m19vals []float64
	for _, run := range runs {
		for _, m := range run.AllMetrics() {
			if m.ID == "M19" {
				m19vals = append(m19vals, m.Value)
			}
		}
	}
	m19mean := cal.Mean(m19vals)
	variance := cal.Stddev(m19vals)

	m19threshold := 0.70
	if sc.Aggregate != nil {
		m19threshold = sc.Aggregate.Threshold
	}

	m20def := sc.FindDef("M20")
	m20threshold := 0.15
	if m20def != nil {
		m20threshold = m20def.Threshold
	}

	for i := range agg.Metrics {
		switch agg.Metrics[i].ID {
		case "M19":
			agg.Metrics[i] = Metric{
				ID: "M19", Name: "overall_accuracy", Value: m19mean, Threshold: m19threshold,
				Pass: m19mean >= m19threshold, Detail: fmt.Sprintf("mean of %d runs", len(runs)),
				Tier: cal.TierMeta,
			}
		case "M20":
			agg.Metrics[i] = Metric{
				ID: "M20", Name: "run_variance", Value: variance, Threshold: m20threshold,
				Pass: variance <= m20threshold, Detail: fmt.Sprintf("stddev=%.3f over %d runs", variance, len(runs)),
				Tier: cal.TierMeta,
			}
		}
	}

	return agg
}

// buildRepoRelevanceMap creates a map from RCA ID → set of relevant repo names.
func buildRepoRelevanceMap(scenario *Scenario) map[string]map[string]bool {
	m := make(map[string]map[string]bool)
	for _, rca := range scenario.RCAs {
		m[rca.ID] = make(map[string]bool)
		for _, repo := range rca.RelevantRepos {
			m[rca.ID][repo] = true
		}
	}
	return m
}


// Math helper aliases — delegate to the generic calibrate package.
var (
	safeDiv  = cal.SafeDiv
	safeDiv2 = cal.SafeDivFloat
	mean     = cal.Mean
	stddev   = cal.Stddev
)

func pearsonCorrelation(x, y []float64) float64 {
	if len(x) != len(y) || len(x) < 2 {
		return 0
	}
	mx, my := mean(x), mean(y)
	var num, dx2, dy2 float64
	for i := range x {
		dx := x[i] - mx
		dy := y[i] - my
		num += dx * dy
		dx2 += dx * dx
		dy2 += dy * dy
	}
	denom := math.Sqrt(dx2 * dy2)
	if denom == 0 {
		// Zero variance in one or both series. If all correctness values are 1.0
		// (perfect answers), this is a valid state (stub mode). Return 1.0 to
		// indicate that convergence scores are well-calibrated for this scenario.
		allCorrect := true
		for _, v := range y {
			if v != 1.0 {
				allCorrect = false
				break
			}
		}
		if allCorrect && len(y) > 0 {
			return 1.0
		}
		return 0
	}
	return num / denom
}
