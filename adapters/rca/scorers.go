package rca

import (
	"fmt"
	"math"
	"strings"

	cal "github.com/dpopsuev/origami/calibrate"
)

// BatchContext wraps the full batch of case results and scenario data.
// Passed as the caseResult argument to batch-level ScorerFunc implementations.
type BatchContext struct {
	Results       []CaseResult
	Scenario      *Scenario
	CaseMap       map[string]*GroundTruthCase
	RCAMap        map[string]*GroundTruthRCA
	RepoRelevance map[string]map[string]bool
}

// NewBatchContext builds lookup maps once for all scorers to share.
func NewBatchContext(results []CaseResult, scenario *Scenario) *BatchContext {
	caseMap := make(map[string]*GroundTruthCase, len(scenario.Cases))
	for i := range scenario.Cases {
		caseMap[scenario.Cases[i].ID] = &scenario.Cases[i]
	}
	rcaMap := make(map[string]*GroundTruthRCA, len(scenario.RCAs))
	for i := range scenario.RCAs {
		rcaMap[scenario.RCAs[i].ID] = &scenario.RCAs[i]
	}
	return &BatchContext{
		Results:       results,
		Scenario:      scenario,
		CaseMap:       caseMap,
		RCAMap:        rcaMap,
		RepoRelevance: buildRepoRelevanceMap(scenario),
	}
}

// RegisterScorers adds all 21 Asterisk RCA metrics to the registry.
func RegisterScorers(reg cal.ScorerRegistry) {
	reg.Register("defect_type_accuracy", scorerM1)
	reg.Register("symptom_category_accuracy", scorerM2)
	reg.Register("recall_hit_rate", scorerM3)
	reg.Register("recall_false_positive_rate", scorerM4)
	reg.Register("serial_killer_detection", scorerM5)
	reg.Register("skip_accuracy", scorerM6)
	reg.Register("cascade_detection", scorerM7)
	reg.Register("convergence_calibration", scorerM8)
	reg.Register("repo_selection_precision", scorerM9)
	reg.Register("repo_selection_recall", scorerM10)
	reg.Register("red_herring_rejection", scorerM11)
	reg.Register("evidence_recall", scorerM12)
	reg.Register("evidence_precision", scorerM13)
	reg.Register("rca_message_relevance", scorerM14)
	reg.Register("smoking_gun_hit_rate", scorerM14b)
	reg.Register("component_identification", scorerM15)
	reg.Register("pipeline_path_accuracy", scorerM16)
	reg.Register("loop_efficiency", scorerM17)
	reg.Register("total_prompt_tokens", scorerM18)
	reg.Register("gap_precision", scorerM21)
	reg.Register("gap_recall", scorerM22)
}

func batchCtx(caseResult any) (*BatchContext, error) {
	bc, ok := caseResult.(*BatchContext)
	if !ok {
		return nil, fmt.Errorf("expected *BatchContext, got %T", caseResult)
	}
	return bc, nil
}

func scorerM1(caseResult, _ any, _ map[string]any) (float64, string, error) {
	bc, err := batchCtx(caseResult)
	if err != nil {
		return 0, "", err
	}
	correct, total := 0, 0
	for _, r := range bc.Results {
		gt := bc.CaseMap[r.CaseID]
		if gt == nil || gt.RCAID == "" {
			continue
		}
		total++
		rca := bc.RCAMap[gt.RCAID]
		if rca != nil && r.ActualDefectType == rca.DefectType {
			correct++
		}
	}
	return safeDiv(correct, total), fmt.Sprintf("%d/%d", correct, total), nil
}

func scorerM2(caseResult, _ any, _ map[string]any) (float64, string, error) {
	bc, err := batchCtx(caseResult)
	if err != nil {
		return 0, "", err
	}
	correct, total := 0, 0
	for _, r := range bc.Results {
		gt := bc.CaseMap[r.CaseID]
		if gt == nil || gt.ExpectedTriage == nil {
			continue
		}
		total++
		if r.ActualCategory == gt.ExpectedTriage.SymptomCategory {
			correct++
		}
	}
	return safeDiv(correct, total), fmt.Sprintf("%d/%d", correct, total), nil
}

func scorerM3(caseResult, _ any, _ map[string]any) (float64, string, error) {
	bc, err := batchCtx(caseResult)
	if err != nil {
		return 0, "", err
	}
	tp, expected := 0, 0
	for _, r := range bc.Results {
		gt := bc.CaseMap[r.CaseID]
		if gt == nil || !gt.ExpectRecallHit {
			continue
		}
		expected++
		if r.ActualRecallHit {
			tp++
		}
	}
	return safeDiv(tp, expected), fmt.Sprintf("%d/%d", tp, expected), nil
}

func scorerM4(caseResult, _ any, _ map[string]any) (float64, string, error) {
	bc, err := batchCtx(caseResult)
	if err != nil {
		return 0, "", err
	}
	fp, expectedMisses := 0, 0
	for _, r := range bc.Results {
		gt := bc.CaseMap[r.CaseID]
		if gt == nil || gt.ExpectRecallHit {
			continue
		}
		expectedMisses++
		if r.ActualRecallHit {
			fp++
		}
	}
	return safeDiv(fp, expectedMisses), fmt.Sprintf("%d/%d", fp, expectedMisses), nil
}

func scorerM5(caseResult, _ any, _ map[string]any) (float64, string, error) {
	bc, err := batchCtx(caseResult)
	if err != nil {
		return 0, "", err
	}
	rcaCases := make(map[string][]CaseResult)
	for _, r := range bc.Results {
		gt := bc.CaseMap[r.CaseID]
		if gt == nil || gt.RCAID == "" {
			continue
		}
		rcaCases[gt.RCAID] = append(rcaCases[gt.RCAID], r)
	}
	correctLinks, expectedLinks := 0, 0
	for _, cases := range rcaCases {
		if len(cases) < 2 {
			continue
		}
		firstRCA := cases[0].ActualRCAID
		for i := 1; i < len(cases); i++ {
			expectedLinks++
			if cases[i].ActualRCAID != 0 && cases[i].ActualRCAID == firstRCA {
				correctLinks++
			}
		}
	}
	return safeDiv(correctLinks, expectedLinks), fmt.Sprintf("%d/%d", correctLinks, expectedLinks), nil
}

func scorerM6(caseResult, _ any, _ map[string]any) (float64, string, error) {
	bc, err := batchCtx(caseResult)
	if err != nil {
		return 0, "", err
	}
	correct, expected := 0, 0
	for _, r := range bc.Results {
		gt := bc.CaseMap[r.CaseID]
		if gt == nil || !gt.ExpectSkip {
			continue
		}
		expected++
		if r.ActualSkip {
			correct++
		}
	}
	return safeDiv(correct, expected), fmt.Sprintf("%d/%d", correct, expected), nil
}

func scorerM7(caseResult, _ any, _ map[string]any) (float64, string, error) {
	bc, err := batchCtx(caseResult)
	if err != nil {
		return 0, "", err
	}
	detected, expected := 0, 0
	for _, r := range bc.Results {
		gt := bc.CaseMap[r.CaseID]
		if gt == nil || !gt.ExpectCascade {
			continue
		}
		expected++
		if r.ActualCascade {
			detected++
		}
	}
	return safeDiv(detected, expected), fmt.Sprintf("%d/%d", detected, expected), nil
}

func scorerM8(caseResult, _ any, _ map[string]any) (float64, string, error) {
	bc, err := batchCtx(caseResult)
	if err != nil {
		return 0, "", err
	}
	var convergences, correctness []float64
	for _, r := range bc.Results {
		if r.ActualConvergence == 0 {
			continue
		}
		gt := bc.CaseMap[r.CaseID]
		if gt == nil || gt.RCAID == "" {
			continue
		}
		rca := bc.RCAMap[gt.RCAID]
		correct := 0.0
		if rca != nil && r.ActualDefectType == rca.DefectType {
			correct = 1.0
		}
		convergences = append(convergences, r.ActualConvergence)
		correctness = append(correctness, correct)
	}
	corr := pearsonCorrelation(convergences, correctness)
	return corr, fmt.Sprintf("r=%.2f (n=%d)", corr, len(convergences)), nil
}

func scorerM9(caseResult, _ any, _ map[string]any) (float64, string, error) {
	bc, err := batchCtx(caseResult)
	if err != nil {
		return 0, "", err
	}
	sumPrecision := 0.0
	count := 0
	for _, r := range bc.Results {
		if len(r.ActualSelectedRepos) == 0 {
			continue
		}
		gt := bc.CaseMap[r.CaseID]
		if gt == nil || gt.RCAID == "" {
			continue
		}
		count++
		relevantSelected := 0
		for _, repo := range r.ActualSelectedRepos {
			if bc.RepoRelevance[gt.RCAID][repo] {
				relevantSelected++
			}
		}
		sumPrecision += safeDiv(relevantSelected, len(r.ActualSelectedRepos))
	}
	return safeDiv2(sumPrecision, float64(count)), fmt.Sprintf("avg over %d cases", count), nil
}

func scorerM10(caseResult, _ any, _ map[string]any) (float64, string, error) {
	bc, err := batchCtx(caseResult)
	if err != nil {
		return 0, "", err
	}
	sumRecall := 0.0
	count := 0
	for _, r := range bc.Results {
		gt := bc.CaseMap[r.CaseID]
		if gt == nil || gt.RCAID == "" || gt.ExpectedResolve == nil {
			continue
		}
		count++
		totalRelevant := len(bc.RepoRelevance[gt.RCAID])
		if totalRelevant == 0 {
			sumRecall += 1.0
			continue
		}
		relevantSelected := 0
		for _, repo := range r.ActualSelectedRepos {
			if bc.RepoRelevance[gt.RCAID][repo] {
				relevantSelected++
			}
		}
		sumRecall += safeDiv(relevantSelected, totalRelevant)
	}
	return safeDiv2(sumRecall, float64(count)), fmt.Sprintf("avg over %d cases", count), nil
}

func scorerM11(caseResult, _ any, _ map[string]any) (float64, string, error) {
	bc, err := batchCtx(caseResult)
	if err != nil {
		return 0, "", err
	}
	redHerringRepos := make(map[string]bool)
	for _, repo := range bc.Scenario.Workspace.Repos {
		if repo.IsRedHerring {
			redHerringRepos[repo.Name] = true
		}
	}
	casesWithF2 := 0
	redHerringSelected := 0
	for _, r := range bc.Results {
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
	return val, fmt.Sprintf("%d cases with Resolve, %d selected red herring", casesWithF2, redHerringSelected), nil
}

func scorerM12(caseResult, _ any, _ map[string]any) (float64, string, error) {
	bc, err := batchCtx(caseResult)
	if err != nil {
		return 0, "", err
	}
	totalFound, totalPlanted := 0, 0
	for _, r := range bc.Results {
		gt := bc.CaseMap[r.CaseID]
		if gt == nil || gt.ExpectedInvest == nil || len(gt.ExpectedInvest.EvidenceRefs) == 0 {
			continue
		}
		found, total := evidenceOverlap(r.ActualEvidenceRefs, gt.ExpectedInvest.EvidenceRefs)
		totalFound += found
		totalPlanted += total
	}
	return safeDiv(totalFound, totalPlanted), fmt.Sprintf("%d/%d", totalFound, totalPlanted), nil
}

func scorerM13(caseResult, _ any, _ map[string]any) (float64, string, error) {
	bc, err := batchCtx(caseResult)
	if err != nil {
		return 0, "", err
	}
	totalRelevant, totalCited := 0, 0
	for _, r := range bc.Results {
		gt := bc.CaseMap[r.CaseID]
		if gt == nil || len(r.ActualEvidenceRefs) == 0 {
			continue
		}
		totalCited += len(r.ActualEvidenceRefs)
		if gt.ExpectedInvest != nil {
			found, _ := evidenceOverlap(r.ActualEvidenceRefs, gt.ExpectedInvest.EvidenceRefs)
			totalRelevant += found
		}
	}
	return safeDiv(totalRelevant, totalCited), fmt.Sprintf("%d/%d", totalRelevant, totalCited), nil
}

func scorerM14(caseResult, _ any, _ map[string]any) (float64, string, error) {
	bc, err := batchCtx(caseResult)
	if err != nil {
		return 0, "", err
	}
	sumScore := 0.0
	count := 0
	for _, r := range bc.Results {
		if r.ActualRCAMessage == "" {
			continue
		}
		gt := bc.CaseMap[r.CaseID]
		if gt == nil || gt.RCAID == "" {
			continue
		}
		rca := bc.RCAMap[gt.RCAID]
		if rca == nil || len(rca.RequiredKeywords) == 0 {
			continue
		}
		count++
		matched := keywordMatch(r.ActualRCAMessage, rca.RequiredKeywords)
		score := math.Min(float64(matched)/float64(rca.KeywordThreshold), 1.0)
		sumScore += score
	}
	return safeDiv2(sumScore, float64(count)), fmt.Sprintf("avg over %d cases", count), nil
}

func scorerM14b(caseResult, _ any, _ map[string]any) (float64, string, error) {
	bc, err := batchCtx(caseResult)
	if err != nil {
		return 0, "", err
	}
	hits, eligible := 0, 0
	for _, r := range bc.Results {
		if r.ActualRCAMessage == "" {
			continue
		}
		gt := bc.CaseMap[r.CaseID]
		if gt == nil || gt.RCAID == "" {
			continue
		}
		rca := bc.RCAMap[gt.RCAID]
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
	return safeDiv(hits, eligible), fmt.Sprintf("%d/%d", hits, eligible), nil
}

func scorerM15(caseResult, _ any, _ map[string]any) (float64, string, error) {
	bc, err := batchCtx(caseResult)
	if err != nil {
		return 0, "", err
	}
	correct, total := 0, 0
	for _, r := range bc.Results {
		gt := bc.CaseMap[r.CaseID]
		if gt == nil || gt.RCAID == "" {
			continue
		}
		rca := bc.RCAMap[gt.RCAID]
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
	return safeDiv(correct, total), fmt.Sprintf("%d/%d", correct, total), nil
}

func scorerM16(caseResult, _ any, _ map[string]any) (float64, string, error) {
	bc, err := batchCtx(caseResult)
	if err != nil {
		return 0, "", err
	}
	correct, total := 0, 0
	for _, r := range bc.Results {
		gt := bc.CaseMap[r.CaseID]
		if gt == nil {
			continue
		}
		total++
		if pathsEqual(r.ActualPath, gt.ExpectedPath) {
			correct++
		}
	}
	return safeDiv(correct, total), fmt.Sprintf("%d/%d", correct, total), nil
}

func scorerM17(caseResult, _ any, _ map[string]any) (float64, string, error) {
	bc, err := batchCtx(caseResult)
	if err != nil {
		return 0, "", err
	}
	sumActual, sumExpected := 0, 0
	for _, r := range bc.Results {
		gt := bc.CaseMap[r.CaseID]
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
	return val, fmt.Sprintf("actual=%d expected=%d", sumActual, sumExpected), nil
}

func scorerM18(caseResult, _ any, _ map[string]any) (float64, string, error) {
	bc, err := batchCtx(caseResult)
	if err != nil {
		return 0, "", err
	}
	realTokens := 0
	hasReal := false
	totalSteps := 0
	for _, r := range bc.Results {
		totalSteps += len(r.ActualPath)
		if r.PromptTokensTotal > 0 {
			realTokens += r.PromptTokensTotal
			hasReal = true
		}
	}
	if hasReal {
		return float64(realTokens), fmt.Sprintf("%d tokens (measured)", realTokens), nil
	}
	estimated := totalSteps * 1000
	return float64(estimated), fmt.Sprintf("~%d tokens (%d steps, estimated)", estimated, totalSteps), nil
}

func scorerM21(caseResult, _ any, _ map[string]any) (float64, string, error) {
	bc, err := batchCtx(caseResult)
	if err != nil {
		return 0, "", err
	}
	totalGaps := 0
	for _, r := range bc.Results {
		totalGaps += len(r.EvidenceGaps)
	}
	return 0, fmt.Sprintf("stub — %d gap items emitted (manual verification required)", totalGaps), nil
}

func scorerM22(caseResult, _ any, _ map[string]any) (float64, string, error) {
	bc, err := batchCtx(caseResult)
	if err != nil {
		return 0, "", err
	}
	wrongWithGaps, wrongTotal := 0, 0
	for _, r := range bc.Results {
		if !r.DefectTypeCorrect && r.ActualDefectType != "" {
			wrongTotal++
			if len(r.EvidenceGaps) > 0 {
				wrongWithGaps++
			}
		}
	}
	return 0, fmt.Sprintf("stub — %d/%d wrong predictions have gap briefs (manual verification required)", wrongWithGaps, wrongTotal), nil
}
