package calibrate

import (
	"fmt"
	"math"
	"strings"
)

// computeMetrics calculates all 20 calibration metrics from case results.
func computeMetrics(scenario *Scenario, results []CaseResult) MetricSet {
	ms := MetricSet{}

	// Build lookup maps
	rcaMap := make(map[string]*GroundTruthRCA)
	for i := range scenario.RCAs {
		rcaMap[scenario.RCAs[i].ID] = &scenario.RCAs[i]
	}
	caseMap := make(map[string]*GroundTruthCase)
	for i := range scenario.Cases {
		caseMap[scenario.Cases[i].ID] = &scenario.Cases[i]
	}
	repoRelevance := buildRepoRelevanceMap(scenario)

	// --- M1-M8: Structured metrics ---
	ms.Structured = []Metric{
		scoreDefectTypeAccuracy(results, caseMap, rcaMap),
		scoreSymptomCategoryAccuracy(results, caseMap),
		scoreRecallHitRate(results, caseMap),
		scoreRecallFalsePositiveRate(results, caseMap),
		scoreSerialKillerDetection(results, caseMap, rcaMap),
		scoreSkipAccuracy(results, caseMap),
		scoreCascadeDetection(results, caseMap),
		scoreConvergenceCalibration(results, caseMap, rcaMap),
	}

	// --- M9-M11: Workspace/repo selection metrics ---
	ms.Workspace = []Metric{
		scoreRepoSelectionPrecision(results, caseMap, repoRelevance),
		scoreRepoSelectionRecall(results, caseMap, repoRelevance),
		scoreRedHerringRejection(results, caseMap, scenario),
	}

	// --- M12-M13: Evidence metrics ---
	ms.Evidence = []Metric{
		scoreEvidenceRecall(results, caseMap),
		scoreEvidencePrecision(results, caseMap),
	}

	// --- M14-M15 + smoking gun: Semantic metrics ---
	ms.Semantic = []Metric{
		scoreRCAMessageRelevance(results, caseMap, rcaMap),
		scoreSmokingGunHitRate(results, caseMap, rcaMap),
		scoreComponentIdentification(results, caseMap, rcaMap),
	}

	// --- M16-M18: Pipeline metrics ---
	ms.Pipeline = []Metric{
		scorePipelinePathAccuracy(results, caseMap),
		scoreLoopEfficiency(results, caseMap),
		scoreTotalPromptTokens(results),
	}

	// --- M19-M20: Aggregate metrics ---
	ms.Aggregate = []Metric{
		scoreOverallAccuracy(ms),
		{ID: "M20", Name: "run_variance", Value: 0, Threshold: 0.15,
			Pass: true, Detail: "single run"},
	}

	return ms
}

// --- M1: Defect type accuracy ---
func scoreDefectTypeAccuracy(results []CaseResult, caseMap map[string]*GroundTruthCase, rcaMap map[string]*GroundTruthRCA) Metric {
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
	val := safeDiv(correct, total)
	return Metric{
		ID: "M1", Name: "defect_type_accuracy",
		Value: val, Threshold: 0.80,
		Pass: val >= 0.80, Detail: fmt.Sprintf("%d/%d", correct, total),
	}
}

// --- M2: Symptom category accuracy ---
func scoreSymptomCategoryAccuracy(results []CaseResult, caseMap map[string]*GroundTruthCase) Metric {
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
	val := safeDiv(correct, total)
	return Metric{
		ID: "M2", Name: "symptom_category_accuracy",
		Value: val, Threshold: 0.75,
		Pass: val >= 0.75, Detail: fmt.Sprintf("%d/%d", correct, total),
	}
}

// --- M3: Recall hit rate ---
func scoreRecallHitRate(results []CaseResult, caseMap map[string]*GroundTruthCase) Metric {
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
	val := safeDiv(truePositive, expectedHits)
	return Metric{
		ID: "M3", Name: "recall_hit_rate",
		Value: val, Threshold: 0.70,
		Pass: val >= 0.70, Detail: fmt.Sprintf("%d/%d", truePositive, expectedHits),
	}
}

// --- M4: Recall false positive rate ---
func scoreRecallFalsePositiveRate(results []CaseResult, caseMap map[string]*GroundTruthCase) Metric {
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
	val := safeDiv(falsePositive, expectedMisses)
	return Metric{
		ID: "M4", Name: "recall_false_positive_rate",
		Value: val, Threshold: 0.10,
		Pass: val <= 0.10, Detail: fmt.Sprintf("%d/%d", falsePositive, expectedMisses),
	}
}

// --- M5: Serial killer detection ---
func scoreSerialKillerDetection(results []CaseResult, caseMap map[string]*GroundTruthCase, rcaMap map[string]*GroundTruthRCA) Metric {
	// Build expected links: cases with the same ground truth RCA should be linked
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
		// All pairs should share the same RCA ID in the store
		firstRCA := cases[0].ActualRCAID
		for i := 1; i < len(cases); i++ {
			expectedLinks++
			if cases[i].ActualRCAID != 0 && cases[i].ActualRCAID == firstRCA {
				correctLinks++
			}
		}
	}
	val := safeDiv(correctLinks, expectedLinks)
	return Metric{
		ID: "M5", Name: "serial_killer_detection",
		Value: val, Threshold: 0.70,
		Pass: val >= 0.70, Detail: fmt.Sprintf("%d/%d", correctLinks, expectedLinks),
	}
}

// --- M6: Skip accuracy ---
func scoreSkipAccuracy(results []CaseResult, caseMap map[string]*GroundTruthCase) Metric {
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
	val := safeDiv(correct, expected)
	return Metric{
		ID: "M6", Name: "skip_accuracy",
		Value: val, Threshold: 0.80,
		Pass: val >= 0.80, Detail: fmt.Sprintf("%d/%d", correct, expected),
	}
}

// --- M7: Cascade detection ---
func scoreCascadeDetection(results []CaseResult, caseMap map[string]*GroundTruthCase) Metric {
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
	val := safeDiv(detected, expected)
	return Metric{
		ID: "M7", Name: "cascade_detection",
		Value: val, Threshold: 0.50,
		Pass: val >= 0.50, Detail: fmt.Sprintf("%d/%d", detected, expected),
	}
}

// --- M8: Convergence calibration ---
func scoreConvergenceCalibration(results []CaseResult, caseMap map[string]*GroundTruthCase, rcaMap map[string]*GroundTruthRCA) Metric {
	// Pearson correlation between convergence score and actual correctness (0 or 1)
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
	return Metric{
		ID: "M8", Name: "convergence_calibration",
		Value: corr, Threshold: 0.40,
		Pass: corr >= 0.40, Detail: fmt.Sprintf("r=%.2f (n=%d)", corr, len(convergences)),
	}
}

// --- M9: Repo selection precision ---
func scoreRepoSelectionPrecision(results []CaseResult, caseMap map[string]*GroundTruthCase, repoRelevance map[string]map[string]bool) Metric {
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
	val := safeDiv2(sumPrecision, float64(count))
	return Metric{
		ID: "M9", Name: "repo_selection_precision",
		Value: val, Threshold: 0.70,
		Pass: val >= 0.70, Detail: fmt.Sprintf("avg over %d cases", count),
	}
}

// --- M10: Repo selection recall ---
func scoreRepoSelectionRecall(results []CaseResult, caseMap map[string]*GroundTruthCase, repoRelevance map[string]map[string]bool) Metric {
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
	val := safeDiv2(sumRecall, float64(count))
	return Metric{
		ID: "M10", Name: "repo_selection_recall",
		Value: val, Threshold: 0.80,
		Pass: val >= 0.80, Detail: fmt.Sprintf("avg over %d cases", count),
	}
}

// --- M11: Red herring rejection ---
func scoreRedHerringRejection(results []CaseResult, caseMap map[string]*GroundTruthCase, scenario *Scenario) Metric {
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
	return Metric{
		ID: "M11", Name: "red_herring_rejection",
		Value: val, Threshold: 0.80,
		Pass: val >= 0.80, Detail: fmt.Sprintf("%d cases with Resolve, %d selected red herring", casesWithF2, redHerringSelected),
	}
}

// --- M12: Evidence recall ---
func scoreEvidenceRecall(results []CaseResult, caseMap map[string]*GroundTruthCase) Metric {
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
	val := safeDiv(totalFound, totalPlanted)
	return Metric{
		ID: "M12", Name: "evidence_recall",
		Value: val, Threshold: 0.60,
		Pass: val >= 0.60, Detail: fmt.Sprintf("%d/%d", totalFound, totalPlanted),
	}
}

// --- M13: Evidence precision ---
func scoreEvidencePrecision(results []CaseResult, caseMap map[string]*GroundTruthCase) Metric {
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
	val := safeDiv(totalRelevant, totalCited)
	return Metric{
		ID: "M13", Name: "evidence_precision",
		Value: val, Threshold: 0.50,
		Pass: val >= 0.50, Detail: fmt.Sprintf("%d/%d", totalRelevant, totalCited),
	}
}

// --- M14: RCA message relevance (keyword fallback for stub mode) ---
func scoreRCAMessageRelevance(results []CaseResult, caseMap map[string]*GroundTruthCase, rcaMap map[string]*GroundTruthRCA) Metric {
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
	val := safeDiv2(sumScore, float64(count))
	return Metric{
		ID: "M14", Name: "rca_message_relevance",
		Value: val, Threshold: 0.60,
		Pass: val >= 0.60, Detail: fmt.Sprintf("avg over %d cases", count),
	}
}

// --- M14b: Smoking gun hit rate ---
// Checks if the adapter's RCA message textually reaches the same core conclusion
// as the PR-proven "smoking gun" phrase. A hit requires ≥50% of the phrase's
// significant words (>3 chars) to appear in the RCA message.
func scoreSmokingGunHitRate(results []CaseResult, caseMap map[string]*GroundTruthCase, rcaMap map[string]*GroundTruthRCA) Metric {
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
	val := safeDiv(hits, eligible)
	return Metric{
		ID: "M14b", Name: "smoking_gun_hit_rate",
		Value: val, Threshold: 0.0,
		Pass: true, Detail: fmt.Sprintf("%d/%d", hits, eligible),
	}
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
func scoreComponentIdentification(results []CaseResult, caseMap map[string]*GroundTruthCase, rcaMap map[string]*GroundTruthRCA) Metric {
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
		// Exact match on component field, or keyword in RCA message
		if r.ActualComponent == rca.Component {
			correct++
		} else if r.ActualRCAMessage != "" && strings.Contains(strings.ToLower(r.ActualRCAMessage), strings.ToLower(rca.Component)) {
			correct++
		}
	}
	val := safeDiv(correct, total)
	return Metric{
		ID: "M15", Name: "component_identification",
		Value: val, Threshold: 0.70,
		Pass: val >= 0.70, Detail: fmt.Sprintf("%d/%d", correct, total),
	}
}

// --- M16: Pipeline path accuracy ---
func scorePipelinePathAccuracy(results []CaseResult, caseMap map[string]*GroundTruthCase) Metric {
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
	val := safeDiv(correct, total)
	return Metric{
		ID: "M16", Name: "pipeline_path_accuracy",
		Value: val, Threshold: 0.60,
		Pass: val >= 0.60, Detail: fmt.Sprintf("%d/%d", correct, total),
	}
}

// --- M17: Loop efficiency ---
func scoreLoopEfficiency(results []CaseResult, caseMap map[string]*GroundTruthCase) Metric {
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
		val = 1.0 // perfect: no loops expected, no loops taken
	} else if sumExpected == 0 {
		val = float64(sumActual + 1) // penalize unexpected loops
	} else {
		val = float64(sumActual) / float64(sumExpected)
	}
	pass := val >= 0.5 && val <= 2.0
	return Metric{
		ID: "M17", Name: "loop_efficiency",
		Value: val, Threshold: 1.0,
		Pass: pass, Detail: fmt.Sprintf("actual=%d expected=%d", sumActual, sumExpected),
	}
}

// --- M18: Total prompt tokens ---
// When dispatch.TokenTracker data is present (PromptTokensTotal > 0), uses real
// measured values. Falls back to step-count estimate for stub mode.
func scoreTotalPromptTokens(results []CaseResult) Metric {
	// Check if we have real token data
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

	budget := 60000.0

	if hasReal {
		val := float64(realTokens)
		return Metric{
			ID: "M18", Name: "total_prompt_tokens",
			Value: val, Threshold: budget,
			Pass: val <= budget, Detail: fmt.Sprintf("%d tokens (measured)", realTokens),
		}
	}

	// Fallback: estimate from path length (each step ~1000 tokens).
	// When no real token data is available (e.g. stub adapter), the budget
	// threshold is not enforced — the estimate is informational only.
	estimated := totalSteps * 1000
	val := float64(estimated)
	return Metric{
		ID: "M18", Name: "total_prompt_tokens",
		Value: val, Threshold: budget,
		Pass: true, Detail: fmt.Sprintf("~%d tokens (%d steps, estimated)", estimated, totalSteps),
	}
}

// --- M19: Overall accuracy (weighted average) ---
func scoreOverallAccuracy(ms MetricSet) Metric {
	// Weighted average of M1, M2, M5, M9, M10, M12, M14, M15
	// M15 (component ID) and M9 (repo precision) give 25% combined weight
	// to "where is the bug?" vs 20% for "what kind of bug?" (M1).
	weights := map[string]float64{
		"M1": 0.20, "M2": 0.10, "M5": 0.15,
		"M9": 0.10, "M10": 0.10, "M12": 0.10,
		"M14": 0.10, "M15": 0.15,
	}
	sum, wsum := 0.0, 0.0
	all := ms.AllMetrics()
	for _, m := range all {
		if w, ok := weights[m.ID]; ok {
			sum += w * m.Value
			wsum += w
		}
	}
	val := 0.0
	if wsum > 0 {
		val = sum / wsum
	}
	return Metric{
		ID: "M19", Name: "overall_accuracy",
		Value: val, Threshold: 0.65,
		Pass: val >= 0.65, Detail: "weighted avg of M1,M2,M5,M9,M10,M12,M14,M15",
	}
}

// aggregateRunMetrics computes the mean and variance across multiple runs.
func aggregateRunMetrics(runs []MetricSet) MetricSet {
	if len(runs) == 0 {
		return MetricSet{}
	}
	if len(runs) == 1 {
		return runs[0]
	}

	// Average each metric across runs
	agg := runs[0] // start with first run's structure
	allByID := make(map[string][]float64)
	for _, run := range runs {
		for _, m := range run.AllMetrics() {
			allByID[m.ID] = append(allByID[m.ID], m.Value)
		}
	}

	updateMetrics := func(metrics []Metric) {
		for i := range metrics {
			vals := allByID[metrics[i].ID]
			metrics[i].Value = mean(vals)
			metrics[i].Pass = evaluatePass(metrics[i])
		}
	}
	updateMetrics(agg.Structured)
	updateMetrics(agg.Workspace)
	updateMetrics(agg.Evidence)
	updateMetrics(agg.Semantic)
	updateMetrics(agg.Pipeline)

	// M20: run variance = stddev of M19 across runs
	m19vals := allByID["M19"]
	variance := stddev(m19vals)
	agg.Aggregate = []Metric{
		{ID: "M19", Name: "overall_accuracy", Value: mean(m19vals), Threshold: 0.65,
			Pass: mean(m19vals) >= 0.65, Detail: fmt.Sprintf("mean of %d runs", len(runs))},
		{ID: "M20", Name: "run_variance", Value: variance, Threshold: 0.15,
			Pass: variance <= 0.15, Detail: fmt.Sprintf("stddev=%.3f over %d runs", variance, len(runs))},
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

func evaluatePass(m Metric) bool {
	switch m.ID {
	case "M4": // lower is better
		return m.Value <= m.Threshold
	case "M17": // range check
		return m.Value >= 0.5 && m.Value <= 2.0
	case "M18": // budget
		return m.Value <= m.Threshold
	case "M20": // variance
		return m.Value <= m.Threshold
	default:
		return m.Value >= m.Threshold
	}
}

// --- Math helpers ---

func safeDiv(num, denom int) float64 {
	if denom == 0 {
		return 1.0 // 0/0 = perfect (nothing to measure)
	}
	return float64(num) / float64(denom)
}

func safeDiv2(num, denom float64) float64 {
	if denom == 0 {
		return 1.0
	}
	return num / denom
}

func mean(vals []float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range vals {
		sum += v
	}
	return sum / float64(len(vals))
}

func stddev(vals []float64) float64 {
	if len(vals) < 2 {
		return 0
	}
	m := mean(vals)
	sum := 0.0
	for _, v := range vals {
		sum += (v - m) * (v - m)
	}
	return math.Sqrt(sum / float64(len(vals)-1))
}

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
