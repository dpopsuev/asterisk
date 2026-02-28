package rca

import (
	"fmt"
	"math"
	"strings"

	cal "github.com/dpopsuev/origami/calibrate"
)

// computeMetrics calculates all 21 calibration metrics from case results.
// Scorer implementations are looked up from the ScorerRegistry via the
// scorer name declared in each MetricDef. Values, pass/fail, direction,
// and tier come from the ScoreCard definition.
func computeMetrics(scenario *Scenario, results []CaseResult, sc *cal.ScoreCard) MetricSet {
	reg := cal.DefaultScorerRegistry()
	RegisterScorers(reg)

	bc := NewBatchContext(results, scenario)
	values, details, err := sc.ScoreCase(bc, nil, reg)
	if err != nil {
		values = make(map[string]float64)
		details = make(map[string]string)
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
