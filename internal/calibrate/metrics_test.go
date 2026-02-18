package calibrate

import (
	"math"
	"testing"
)

// --- Helper tests ---

func TestSafeDiv(t *testing.T) {
	tests := []struct {
		name     string
		num, den int
		want     float64
	}{
		{"normal", 3, 4, 0.75},
		{"all correct", 5, 5, 1.0},
		{"none correct", 0, 5, 0.0},
		{"zero/zero perfect", 0, 0, 1.0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := safeDiv(tt.num, tt.den)
			if math.Abs(got-tt.want) > 1e-9 {
				t.Errorf("safeDiv(%d, %d) = %f, want %f", tt.num, tt.den, got, tt.want)
			}
		})
	}
}

func TestSafeDiv2(t *testing.T) {
	tests := []struct {
		name     string
		num, den float64
		want     float64
	}{
		{"normal", 3.0, 4.0, 0.75},
		{"zero denom", 3.0, 0.0, 1.0},
		{"zero both", 0.0, 0.0, 1.0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := safeDiv2(tt.num, tt.den)
			if math.Abs(got-tt.want) > 1e-9 {
				t.Errorf("safeDiv2(%f, %f) = %f, want %f", tt.num, tt.den, got, tt.want)
			}
		})
	}
}

func TestMean(t *testing.T) {
	tests := []struct {
		name string
		vals []float64
		want float64
	}{
		{"empty", nil, 0},
		{"single", []float64{5.0}, 5.0},
		{"multiple", []float64{1.0, 2.0, 3.0}, 2.0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mean(tt.vals)
			if math.Abs(got-tt.want) > 1e-9 {
				t.Errorf("mean(%v) = %f, want %f", tt.vals, got, tt.want)
			}
		})
	}
}

func TestStddev(t *testing.T) {
	tests := []struct {
		name string
		vals []float64
		want float64
	}{
		{"single", []float64{5.0}, 0.0},
		{"identical", []float64{3.0, 3.0, 3.0}, 0.0},
		{"varied", []float64{2.0, 4.0, 4.0, 4.0, 5.0, 5.0, 7.0, 9.0}, 2.14},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stddev(tt.vals)
			if math.Abs(got-tt.want) > 0.01 {
				t.Errorf("stddev(%v) = %f, want %f", tt.vals, got, tt.want)
			}
		})
	}
}

func TestPearsonCorrelation(t *testing.T) {
	tests := []struct {
		name string
		x, y []float64
		want float64
	}{
		{"too few", []float64{1.0}, []float64{1.0}, 0.0},
		{"mismatched length", []float64{1.0, 2.0}, []float64{1.0}, 0.0},
		{"perfect positive", []float64{1.0, 2.0, 3.0}, []float64{10.0, 20.0, 30.0}, 1.0},
		{"perfect negative", []float64{1.0, 2.0, 3.0}, []float64{30.0, 20.0, 10.0}, -1.0},
		{"zero variance all correct", []float64{0.8, 0.8}, []float64{1.0, 1.0}, 1.0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := pearsonCorrelation(tt.x, tt.y)
			if math.Abs(got-tt.want) > 0.01 {
				t.Errorf("pearsonCorrelation(%v, %v) = %f, want %f", tt.x, tt.y, got, tt.want)
			}
		})
	}
}

func TestEvaluatePass(t *testing.T) {
	tests := []struct {
		name string
		m    Metric
		want bool
	}{
		{"M1 pass", Metric{ID: "M1", Value: 0.85, Threshold: 0.80}, true},
		{"M1 fail", Metric{ID: "M1", Value: 0.70, Threshold: 0.80}, false},
		{"M4 lower better pass", Metric{ID: "M4", Value: 0.05, Threshold: 0.10}, true},
		{"M4 lower better fail", Metric{ID: "M4", Value: 0.15, Threshold: 0.10}, false},
		{"M17 in range", Metric{ID: "M17", Value: 1.0, Threshold: 1.0}, true},
		{"M17 too low", Metric{ID: "M17", Value: 0.3, Threshold: 1.0}, false},
		{"M17 too high", Metric{ID: "M17", Value: 2.5, Threshold: 1.0}, false},
		{"M18 budget pass", Metric{ID: "M18", Value: 50000, Threshold: 60000}, true},
		{"M18 budget fail", Metric{ID: "M18", Value: 70000, Threshold: 60000}, false},
		{"M20 variance pass", Metric{ID: "M20", Value: 0.10, Threshold: 0.15}, true},
		{"M20 variance fail", Metric{ID: "M20", Value: 0.20, Threshold: 0.15}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := evaluatePass(tt.m)
			if got != tt.want {
				t.Errorf("evaluatePass(%+v) = %v, want %v", tt.m, got, tt.want)
			}
		})
	}
}

// --- Scorer tests with fixture data ---

// buildFixtureCaseMaps returns caseMap and rcaMap from a minimal scenario.
func buildFixtureCaseMaps() (map[string]*GroundTruthCase, map[string]*GroundTruthRCA) {
	rcas := []GroundTruthRCA{
		{
			ID: "R1", DefectType: "pb001", Component: "linuxptp-daemon",
			RequiredKeywords: []string{"ptp", "clock", "offset"},
			KeywordThreshold: 2, RelevantRepos: []string{"linuxptp-daemon"},
		},
		{
			ID: "R2", DefectType: "au001", Component: "cnf-gotests",
			RequiredKeywords: []string{"automation", "skip"},
			KeywordThreshold: 1, RelevantRepos: []string{"cnf-gotests"},
		},
	}
	cases := []GroundTruthCase{
		{
			ID: "C1", RCAID: "R1",
			ExpectedTriage:  &ExpectedTriage{SymptomCategory: "product"},
			ExpectedInvest:  &ExpectedInvest{EvidenceRefs: []string{"linuxptp-daemon:src/ptp.c"}},
			ExpectedResolve: &ExpectedResolve{SelectedRepos: []ExpectedResolveRepo{{Name: "linuxptp-daemon"}}},
			ExpectedPath:    []string{"F0", "F1", "F2", "F3", "F4", "F5", "F6"},
			ExpectedLoops:   0,
		},
		{
			ID: "C2", RCAID: "R1", ExpectRecallHit: true,
			ExpectedTriage: &ExpectedTriage{SymptomCategory: "product"},
			ExpectedPath:   []string{"F0", "F1", "F2", "F3", "F4", "F5", "F6"},
			ExpectedLoops:  0,
		},
		{
			ID: "C3", RCAID: "R2", ExpectSkip: true,
			ExpectedTriage: &ExpectedTriage{SymptomCategory: "automation"},
			ExpectedPath:   []string{"F0", "F1"},
			ExpectedLoops:  0,
		},
		{
			ID: "C4", ExpectCascade: true,
			ExpectedTriage: &ExpectedTriage{SymptomCategory: "product"},
			ExpectedPath:   []string{"F0", "F1", "F2", "F3", "F4", "F5", "F6"},
			ExpectedLoops:  0,
		},
	}

	caseMap := make(map[string]*GroundTruthCase)
	for i := range cases {
		caseMap[cases[i].ID] = &cases[i]
	}
	rcaMap := make(map[string]*GroundTruthRCA)
	for i := range rcas {
		rcaMap[rcas[i].ID] = &rcas[i]
	}
	return caseMap, rcaMap
}

func TestScoreDefectTypeAccuracy(t *testing.T) {
	caseMap, rcaMap := buildFixtureCaseMaps()

	tests := []struct {
		name    string
		results []CaseResult
		want    float64
	}{
		{
			"all correct",
			[]CaseResult{
				{CaseID: "C1", ActualDefectType: "pb001"},
				{CaseID: "C2", ActualDefectType: "pb001"},
				{CaseID: "C3", ActualDefectType: "au001"},
			},
			1.0,
		},
		{
			"one wrong",
			[]CaseResult{
				{CaseID: "C1", ActualDefectType: "pb001"},
				{CaseID: "C2", ActualDefectType: "wrong"},
				{CaseID: "C3", ActualDefectType: "au001"},
			},
			2.0 / 3.0,
		},
		{
			"empty results",
			[]CaseResult{},
			1.0, // 0/0 = perfect
		},
		{
			"case without RCA ignored",
			[]CaseResult{
				{CaseID: "C4", ActualDefectType: "pb001"}, // C4 has no RCAID
			},
			1.0, // 0/0
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := scoreDefectTypeAccuracy(tt.results, caseMap, rcaMap)
			if m.ID != "M1" {
				t.Errorf("expected ID=M1, got %s", m.ID)
			}
			if math.Abs(m.Value-tt.want) > 1e-9 {
				t.Errorf("value = %f, want %f", m.Value, tt.want)
			}
		})
	}
}

func TestScoreSymptomCategoryAccuracy(t *testing.T) {
	caseMap, _ := buildFixtureCaseMaps()

	tests := []struct {
		name    string
		results []CaseResult
		want    float64
	}{
		{
			"all correct",
			[]CaseResult{
				{CaseID: "C1", ActualCategory: "product"},
				{CaseID: "C3", ActualCategory: "automation"},
			},
			1.0,
		},
		{
			"one wrong",
			[]CaseResult{
				{CaseID: "C1", ActualCategory: "wrong"},
				{CaseID: "C3", ActualCategory: "automation"},
			},
			0.5,
		},
		{
			"no triage expected",
			[]CaseResult{
				{CaseID: "C4", ActualCategory: "product"}, // C4 has no ExpectedTriage
			},
			1.0, // 0/0
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := scoreSymptomCategoryAccuracy(tt.results, caseMap)
			if m.ID != "M2" {
				t.Errorf("expected ID=M2, got %s", m.ID)
			}
			if math.Abs(m.Value-tt.want) > 1e-9 {
				t.Errorf("value = %f, want %f", m.Value, tt.want)
			}
		})
	}
}

func TestScoreRecallHitRate(t *testing.T) {
	caseMap, _ := buildFixtureCaseMaps()

	tests := []struct {
		name    string
		results []CaseResult
		want    float64
	}{
		{
			"hit detected",
			[]CaseResult{
				{CaseID: "C2", ActualRecallHit: true},
			},
			1.0,
		},
		{
			"hit missed",
			[]CaseResult{
				{CaseID: "C2", ActualRecallHit: false},
			},
			0.0,
		},
		{
			"no recall expected",
			[]CaseResult{
				{CaseID: "C1", ActualRecallHit: true}, // C1 doesn't expect recall
			},
			1.0, // 0/0
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := scoreRecallHitRate(tt.results, caseMap)
			if m.ID != "M3" {
				t.Errorf("expected ID=M3, got %s", m.ID)
			}
			if math.Abs(m.Value-tt.want) > 1e-9 {
				t.Errorf("value = %f, want %f", m.Value, tt.want)
			}
		})
	}
}

func TestScoreRecallFalsePositiveRate(t *testing.T) {
	caseMap, _ := buildFixtureCaseMaps()

	tests := []struct {
		name    string
		results []CaseResult
		want    float64
	}{
		{
			"no false positive",
			[]CaseResult{
				{CaseID: "C1", ActualRecallHit: false}, // C1 doesn't expect recall
			},
			0.0,
		},
		{
			"false positive",
			[]CaseResult{
				{CaseID: "C1", ActualRecallHit: true}, // C1 doesn't expect recall but got one
			},
			1.0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := scoreRecallFalsePositiveRate(tt.results, caseMap)
			if m.ID != "M4" {
				t.Errorf("expected ID=M4, got %s", m.ID)
			}
			if math.Abs(m.Value-tt.want) > 1e-9 {
				t.Errorf("value = %f, want %f", m.Value, tt.want)
			}
			// M4 threshold is <=0.10; 0.0 should pass, 1.0 should fail
			if tt.want == 0.0 && !m.Pass {
				t.Error("expected Pass for FP rate 0.0")
			}
			if tt.want == 1.0 && m.Pass {
				t.Error("expected Fail for FP rate 1.0")
			}
		})
	}
}

func TestScoreSkipAccuracy(t *testing.T) {
	caseMap, _ := buildFixtureCaseMaps()

	tests := []struct {
		name    string
		results []CaseResult
		want    float64
	}{
		{
			"skip detected",
			[]CaseResult{
				{CaseID: "C3", ActualSkip: true},
			},
			1.0,
		},
		{
			"skip missed",
			[]CaseResult{
				{CaseID: "C3", ActualSkip: false},
			},
			0.0,
		},
		{
			"no skip expected",
			[]CaseResult{
				{CaseID: "C1", ActualSkip: true}, // C1 doesn't expect skip
			},
			1.0, // 0/0
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := scoreSkipAccuracy(tt.results, caseMap)
			if m.ID != "M6" {
				t.Errorf("expected ID=M6, got %s", m.ID)
			}
			if math.Abs(m.Value-tt.want) > 1e-9 {
				t.Errorf("value = %f, want %f", m.Value, tt.want)
			}
		})
	}
}

func TestScoreCascadeDetection(t *testing.T) {
	caseMap, _ := buildFixtureCaseMaps()

	tests := []struct {
		name    string
		results []CaseResult
		want    float64
	}{
		{
			"cascade detected",
			[]CaseResult{
				{CaseID: "C4", ActualCascade: true},
			},
			1.0,
		},
		{
			"cascade missed",
			[]CaseResult{
				{CaseID: "C4", ActualCascade: false},
			},
			0.0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := scoreCascadeDetection(tt.results, caseMap)
			if m.ID != "M7" {
				t.Errorf("expected ID=M7, got %s", m.ID)
			}
			if math.Abs(m.Value-tt.want) > 1e-9 {
				t.Errorf("value = %f, want %f", m.Value, tt.want)
			}
		})
	}
}

func TestScoreSerialKillerDetection(t *testing.T) {
	caseMap, rcaMap := buildFixtureCaseMaps()

	tests := []struct {
		name    string
		results []CaseResult
		want    float64
	}{
		{
			"linked to same RCA",
			[]CaseResult{
				{CaseID: "C1", ActualRCAID: 100},
				{CaseID: "C2", ActualRCAID: 100},
			},
			1.0,
		},
		{
			"linked to different RCAs",
			[]CaseResult{
				{CaseID: "C1", ActualRCAID: 100},
				{CaseID: "C2", ActualRCAID: 200},
			},
			0.0,
		},
		{
			"single case per RCA",
			[]CaseResult{
				{CaseID: "C1", ActualRCAID: 100},
				{CaseID: "C3", ActualRCAID: 200},
			},
			1.0, // 0/0: no pairs expected
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := scoreSerialKillerDetection(tt.results, caseMap, rcaMap)
			if m.ID != "M5" {
				t.Errorf("expected ID=M5, got %s", m.ID)
			}
			if math.Abs(m.Value-tt.want) > 1e-9 {
				t.Errorf("value = %f, want %f", m.Value, tt.want)
			}
		})
	}
}

func TestScoreEvidenceRecall(t *testing.T) {
	caseMap, _ := buildFixtureCaseMaps()

	tests := []struct {
		name    string
		results []CaseResult
		want    float64
	}{
		{
			"evidence found",
			[]CaseResult{
				{CaseID: "C1", ActualEvidenceRefs: []string{"linuxptp-daemon:src/ptp.c"}},
			},
			1.0,
		},
		{
			"evidence not found",
			[]CaseResult{
				{CaseID: "C1", ActualEvidenceRefs: []string{"unrelated:file.go"}},
			},
			0.0,
		},
		{
			"no evidence expected",
			[]CaseResult{
				{CaseID: "C2", ActualEvidenceRefs: []string{"anything"}},
			},
			1.0, // 0/0
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := scoreEvidenceRecall(tt.results, caseMap)
			if m.ID != "M12" {
				t.Errorf("expected ID=M12, got %s", m.ID)
			}
			if math.Abs(m.Value-tt.want) > 1e-9 {
				t.Errorf("value = %f, want %f", m.Value, tt.want)
			}
		})
	}
}

func TestScoreEvidencePrecision(t *testing.T) {
	caseMap, _ := buildFixtureCaseMaps()

	tests := []struct {
		name    string
		results []CaseResult
		want    float64
	}{
		{
			"all relevant",
			[]CaseResult{
				{CaseID: "C1", ActualEvidenceRefs: []string{"linuxptp-daemon:src/ptp.c"}},
			},
			1.0,
		},
		{
			"half relevant",
			[]CaseResult{
				{CaseID: "C1", ActualEvidenceRefs: []string{"linuxptp-daemon:src/ptp.c", "irrelevant"}},
			},
			0.5,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := scoreEvidencePrecision(tt.results, caseMap)
			if m.ID != "M13" {
				t.Errorf("expected ID=M13, got %s", m.ID)
			}
			if math.Abs(m.Value-tt.want) > 1e-9 {
				t.Errorf("value = %f, want %f", m.Value, tt.want)
			}
		})
	}
}

func TestScoreRCAMessageRelevance(t *testing.T) {
	caseMap, rcaMap := buildFixtureCaseMaps()

	tests := []struct {
		name    string
		results []CaseResult
		want    float64
	}{
		{
			"all keywords",
			[]CaseResult{
				{CaseID: "C1", ActualRCAMessage: "ptp clock offset is wrong"},
			},
			1.0, // 3 keywords matched, threshold=2, min(3/2, 1) = 1
		},
		{
			"one keyword",
			[]CaseResult{
				{CaseID: "C1", ActualRCAMessage: "ptp issue"},
			},
			0.5, // 1/2 = 0.5
		},
		{
			"no message",
			[]CaseResult{
				{CaseID: "C1", ActualRCAMessage: ""},
			},
			1.0, // skipped, 0/0
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := scoreRCAMessageRelevance(tt.results, caseMap, rcaMap)
			if m.ID != "M14" {
				t.Errorf("expected ID=M14, got %s", m.ID)
			}
			if math.Abs(m.Value-tt.want) > 1e-9 {
				t.Errorf("value = %f, want %f", m.Value, tt.want)
			}
		})
	}
}

func TestScoreComponentIdentification(t *testing.T) {
	caseMap, rcaMap := buildFixtureCaseMaps()

	tests := []struct {
		name    string
		results []CaseResult
		want    float64
	}{
		{
			"exact match",
			[]CaseResult{
				{CaseID: "C1", ActualComponent: "linuxptp-daemon"},
				{CaseID: "C3", ActualComponent: "cnf-gotests"},
			},
			1.0,
		},
		{
			"keyword in message",
			[]CaseResult{
				{CaseID: "C1", ActualComponent: "wrong", ActualRCAMessage: "issue in linuxptp-daemon"},
			},
			1.0,
		},
		{
			"no match",
			[]CaseResult{
				{CaseID: "C1", ActualComponent: "wrong", ActualRCAMessage: "no clue"},
			},
			0.0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := scoreComponentIdentification(tt.results, caseMap, rcaMap)
			if m.ID != "M15" {
				t.Errorf("expected ID=M15, got %s", m.ID)
			}
			if math.Abs(m.Value-tt.want) > 1e-9 {
				t.Errorf("value = %f, want %f", m.Value, tt.want)
			}
		})
	}
}

func TestScorePipelinePathAccuracy(t *testing.T) {
	caseMap, _ := buildFixtureCaseMaps()

	tests := []struct {
		name    string
		results []CaseResult
		want    float64
	}{
		{
			"correct path",
			[]CaseResult{
				{CaseID: "C1", ActualPath: []string{"F0", "F1", "F2", "F3", "F4", "F5", "F6"}},
				{CaseID: "C3", ActualPath: []string{"F0", "F1"}},
			},
			1.0,
		},
		{
			"wrong path",
			[]CaseResult{
				{CaseID: "C1", ActualPath: []string{"F0", "F1"}},
			},
			0.0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := scorePipelinePathAccuracy(tt.results, caseMap)
			if m.ID != "M16" {
				t.Errorf("expected ID=M16, got %s", m.ID)
			}
			if math.Abs(m.Value-tt.want) > 1e-9 {
				t.Errorf("value = %f, want %f", m.Value, tt.want)
			}
		})
	}
}

func TestScoreLoopEfficiency(t *testing.T) {
	caseMap, _ := buildFixtureCaseMaps()

	tests := []struct {
		name    string
		results []CaseResult
		want    float64
		pass    bool
	}{
		{
			"no loops expected or taken",
			[]CaseResult{
				{CaseID: "C1", ActualLoops: 0},
			},
			1.0, true,
		},
		{
			"expected loops matched",
			[]CaseResult{
				{CaseID: "C1", ActualLoops: 0},
				{CaseID: "C2", ActualLoops: 0},
			},
			1.0, true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := scoreLoopEfficiency(tt.results, caseMap)
			if m.ID != "M17" {
				t.Errorf("expected ID=M17, got %s", m.ID)
			}
			if math.Abs(m.Value-tt.want) > 1e-9 {
				t.Errorf("value = %f, want %f", m.Value, tt.want)
			}
			if m.Pass != tt.pass {
				t.Errorf("pass = %v, want %v", m.Pass, tt.pass)
			}
		})
	}
}

func TestScoreTotalPromptTokens(t *testing.T) {
	tests := []struct {
		name    string
		results []CaseResult
		pass    bool
	}{
		{
			"stub mode estimate",
			[]CaseResult{
				{ActualPath: []string{"F0", "F1", "F2"}},
			},
			true, // estimated, always passes
		},
		{
			"real tokens under budget",
			[]CaseResult{
				{ActualPath: []string{"F0", "F1"}, PromptTokensTotal: 5000},
			},
			true,
		},
		{
			"real tokens over budget",
			[]CaseResult{
				{ActualPath: []string{"F0"}, PromptTokensTotal: 70000},
			},
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := scoreTotalPromptTokens(tt.results)
			if m.ID != "M18" {
				t.Errorf("expected ID=M18, got %s", m.ID)
			}
			if m.Pass != tt.pass {
				t.Errorf("pass = %v, want %v (value=%f)", m.Pass, tt.pass, m.Value)
			}
		})
	}
}

func TestScoreOverallAccuracy(t *testing.T) {
	// Build a MetricSet where all contributing metrics are 1.0
	ms := MetricSet{
		Structured: []Metric{
			{ID: "M1", Value: 1.0}, {ID: "M2", Value: 1.0},
			{ID: "M3", Value: 1.0}, {ID: "M4", Value: 0.0},
			{ID: "M5", Value: 1.0}, {ID: "M6", Value: 1.0},
			{ID: "M7", Value: 1.0}, {ID: "M8", Value: 1.0},
		},
		Workspace: []Metric{
			{ID: "M9", Value: 1.0}, {ID: "M10", Value: 1.0}, {ID: "M11", Value: 1.0},
		},
		Evidence: []Metric{
			{ID: "M12", Value: 1.0}, {ID: "M13", Value: 1.0},
		},
		Semantic: []Metric{
			{ID: "M14", Value: 1.0}, {ID: "M15", Value: 1.0},
		},
		Pipeline: []Metric{
			{ID: "M16", Value: 1.0}, {ID: "M17", Value: 1.0}, {ID: "M18", Value: 1000},
		},
	}

	m := scoreOverallAccuracy(ms)
	if m.ID != "M19" {
		t.Errorf("expected ID=M19, got %s", m.ID)
	}
	if math.Abs(m.Value-1.0) > 1e-9 {
		t.Errorf("expected overall accuracy 1.0 when all metrics perfect, got %f", m.Value)
	}
	if !m.Pass {
		t.Error("expected Pass for perfect metrics")
	}
}

func TestScoreRepoSelectionPrecision(t *testing.T) {
	caseMap, _ := buildFixtureCaseMaps()
	repoRelevance := map[string]map[string]bool{
		"R1": {"linuxptp-daemon": true},
		"R2": {"cnf-gotests": true},
	}

	tests := []struct {
		name    string
		results []CaseResult
		want    float64
	}{
		{
			"perfect selection",
			[]CaseResult{
				{CaseID: "C1", ActualSelectedRepos: []string{"linuxptp-daemon"}},
			},
			1.0,
		},
		{
			"extra irrelevant repo",
			[]CaseResult{
				{CaseID: "C1", ActualSelectedRepos: []string{"linuxptp-daemon", "red-herring"}},
			},
			0.5,
		},
		{
			"no repos selected",
			[]CaseResult{
				{CaseID: "C1", ActualSelectedRepos: nil},
			},
			1.0, // 0/0
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := scoreRepoSelectionPrecision(tt.results, caseMap, repoRelevance)
			if m.ID != "M9" {
				t.Errorf("expected ID=M9, got %s", m.ID)
			}
			if math.Abs(m.Value-tt.want) > 1e-9 {
				t.Errorf("value = %f, want %f", m.Value, tt.want)
			}
		})
	}
}

func TestScoreRepoSelectionRecall(t *testing.T) {
	caseMap, _ := buildFixtureCaseMaps()
	repoRelevance := map[string]map[string]bool{
		"R1": {"linuxptp-daemon": true},
		"R2": {"cnf-gotests": true},
	}

	tests := []struct {
		name    string
		results []CaseResult
		want    float64
	}{
		{
			"all relevant selected",
			[]CaseResult{
				{CaseID: "C1", ActualSelectedRepos: []string{"linuxptp-daemon"}},
			},
			1.0,
		},
		{
			"relevant missing",
			[]CaseResult{
				{CaseID: "C1", ActualSelectedRepos: []string{"wrong-repo"}},
			},
			0.0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := scoreRepoSelectionRecall(tt.results, caseMap, repoRelevance)
			if m.ID != "M10" {
				t.Errorf("expected ID=M10, got %s", m.ID)
			}
			if math.Abs(m.Value-tt.want) > 1e-9 {
				t.Errorf("value = %f, want %f", m.Value, tt.want)
			}
		})
	}
}

func TestScoreRedHerringRejection(t *testing.T) {
	scenario := &Scenario{
		Workspace: WorkspaceConfig{
			Repos: []RepoConfig{
				{Name: "linuxptp-daemon"},
				{Name: "red-herring", IsRedHerring: true},
			},
		},
	}
	caseMap, _ := buildFixtureCaseMaps()

	tests := []struct {
		name    string
		results []CaseResult
		want    float64
	}{
		{
			"red herring rejected",
			[]CaseResult{
				{CaseID: "C1", ActualSelectedRepos: []string{"linuxptp-daemon"}},
			},
			1.0,
		},
		{
			"red herring selected",
			[]CaseResult{
				{CaseID: "C1", ActualSelectedRepos: []string{"red-herring"}},
			},
			0.0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := scoreRedHerringRejection(tt.results, caseMap, scenario)
			if m.ID != "M11" {
				t.Errorf("expected ID=M11, got %s", m.ID)
			}
			if math.Abs(m.Value-tt.want) > 1e-9 {
				t.Errorf("value = %f, want %f", m.Value, tt.want)
			}
		})
	}
}

// --- aggregateRunMetrics ---

func TestAggregateRunMetrics(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		agg := aggregateRunMetrics(nil)
		if len(agg.AllMetrics()) != 0 {
			t.Error("expected empty MetricSet")
		}
	})

	t.Run("single run passthrough", func(t *testing.T) {
		ms := MetricSet{
			Structured: []Metric{{ID: "M1", Value: 0.9}},
			Aggregate:  []Metric{{ID: "M19", Value: 0.85}},
		}
		agg := aggregateRunMetrics([]MetricSet{ms})
		if agg.Structured[0].Value != 0.9 {
			t.Errorf("expected 0.9, got %f", agg.Structured[0].Value)
		}
	})

	t.Run("two identical runs", func(t *testing.T) {
		ms := MetricSet{
			Structured: []Metric{{ID: "M1", Value: 0.8, Threshold: 0.80}},
			Workspace:  []Metric{{ID: "M9", Value: 0.7, Threshold: 0.70}},
			Evidence:   []Metric{{ID: "M12", Value: 0.6, Threshold: 0.60}},
			Semantic:   []Metric{{ID: "M14", Value: 0.7, Threshold: 0.60}},
			Pipeline:   []Metric{{ID: "M16", Value: 0.5, Threshold: 0.60}},
			Aggregate:  []Metric{{ID: "M19", Value: 0.75, Threshold: 0.65}, {ID: "M20", Value: 0, Threshold: 0.15}},
		}
		agg := aggregateRunMetrics([]MetricSet{ms, ms})
		// Means should be the same
		if math.Abs(agg.Structured[0].Value-0.8) > 1e-9 {
			t.Errorf("M1 mean = %f, want 0.8", agg.Structured[0].Value)
		}
		// M20 variance should be 0 for identical runs
		for _, m := range agg.Aggregate {
			if m.ID == "M20" && m.Value != 0 {
				t.Errorf("M20 variance = %f, want 0", m.Value)
			}
		}
	})
}

// --- pathsEqual ---

func TestPathsEqual(t *testing.T) {
	tests := []struct {
		a, b []string
		want bool
	}{
		{nil, nil, true},
		{[]string{"F0"}, []string{"F0"}, true},
		{[]string{"F0", "F1"}, []string{"F0"}, false},
		{[]string{"F0"}, []string{"F1"}, false},
	}
	for _, tt := range tests {
		got := pathsEqual(tt.a, tt.b)
		if got != tt.want {
			t.Errorf("pathsEqual(%v, %v) = %v, want %v", tt.a, tt.b, got, tt.want)
		}
	}
}

// --- keywordMatch ---

func TestKeywordMatch(t *testing.T) {
	tests := []struct {
		text     string
		keywords []string
		want     int
	}{
		{"ptp clock offset issue", []string{"ptp", "clock", "offset"}, 3},
		{"some random text", []string{"ptp", "clock"}, 0},
		{"PTP Clock", []string{"ptp", "clock"}, 2}, // case insensitive
	}
	for _, tt := range tests {
		got := keywordMatch(tt.text, tt.keywords)
		if got != tt.want {
			t.Errorf("keywordMatch(%q, %v) = %d, want %d", tt.text, tt.keywords, got, tt.want)
		}
	}
}

// --- evidenceOverlap ---

func TestEvidenceOverlap(t *testing.T) {
	tests := []struct {
		name              string
		actual, expected  []string
		wantFound, wantN  int
	}{
		{"empty expected", []string{"a"}, nil, 0, 0},
		{"exact match", []string{"file.go"}, []string{"file.go"}, 1, 1},
		{"partial path match", []string{"repo:src/file.go"}, []string{"file.go"}, 1, 1},
		{"no match", []string{"other.go"}, []string{"file.go"}, 0, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			found, total := evidenceOverlap(tt.actual, tt.expected)
			if found != tt.wantFound || total != tt.wantN {
				t.Errorf("evidenceOverlap = (%d, %d), want (%d, %d)", found, total, tt.wantFound, tt.wantN)
			}
		})
	}
}

// --- computeMetrics integration ---

func TestComputeMetrics_EmptyResults(t *testing.T) {
	scenario := &Scenario{
		RCAs:  []GroundTruthRCA{{ID: "R1", DefectType: "pb001"}},
		Cases: []GroundTruthCase{{ID: "C1", RCAID: "R1"}},
	}
	ms := computeMetrics(scenario, nil)
	all := ms.AllMetrics()
	if len(all) != 21 {
		t.Errorf("expected 21 metrics, got %d", len(all))
	}
}
