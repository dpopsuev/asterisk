package rca

import (
	"math"
	"os"
	"testing"

	cal "github.com/dpopsuev/origami/calibrate"
)

func testScoreCard(t *testing.T) *cal.ScoreCard {
	t.Helper()
	path := "../../scorecards/asterisk-rca.yaml"
	if _, err := os.Stat(path); err != nil {
		t.Skip("scorecard YAML not found at", path)
	}
	sc, err := cal.LoadScoreCard(path)
	if err != nil {
		t.Fatalf("load scorecard: %v", err)
	}
	return sc
}

func testRegistry(t *testing.T) cal.ScorerRegistry {
	t.Helper()
	reg := cal.DefaultScorerRegistry()
	RegisterScorers(reg)
	return reg
}

func runScorer(t *testing.T, name string, results []CaseResult, scenario *Scenario) (float64, string) {
	t.Helper()
	reg := testRegistry(t)
	fn, err := reg.Get(name)
	if err != nil {
		t.Fatalf("get scorer %q: %v", name, err)
	}
	bc := NewBatchContext(results, scenario)
	val, detail, err := fn(bc, nil, nil)
	if err != nil {
		t.Fatalf("scorer %q: %v", name, err)
	}
	return val, detail
}

func buildFixtureScenario() *Scenario {
	return &Scenario{
		RCAs: []GroundTruthRCA{
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
		},
		Cases: []GroundTruthCase{
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
		},
	}
}

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

func TestScoreCardEvaluate(t *testing.T) {
	sc := testScoreCard(t)
	tests := []struct {
		name  string
		id    string
		value float64
		want  bool
	}{
		{"M1 pass", "M1", 0.90, true},
		{"M1 fail", "M1", 0.70, false},
		{"M4 lower better pass", "M4", 0.05, true},
		{"M4 lower better fail", "M4", 0.15, false},
		{"M17 in range", "M17", 1.0, true},
		{"M17 too low", "M17", -0.1, false},
		{"M17 too high", "M17", 3.5, false},
		{"M18 budget pass", "M18", 50000, true},
		{"M18 budget fail", "M18", 250000, false},
		{"M20 variance pass", "M20", 0.10, true},
		{"M20 variance fail", "M20", 0.20, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			def := sc.FindDef(tt.id)
			if def == nil {
				t.Fatalf("metric %s not found in scorecard", tt.id)
			}
			got := def.Evaluate(tt.value)
			if got != tt.want {
				t.Errorf("MetricDef(%s).Evaluate(%f) = %v, want %v (threshold=%f, direction=%s)",
					tt.id, tt.value, got, tt.want, def.Threshold, def.Direction)
			}
		})
	}
}

// --- Scorer tests via registry ---

func TestScorerM1_DefectTypeAccuracy(t *testing.T) {
	scenario := buildFixtureScenario()
	tests := []struct {
		name    string
		results []CaseResult
		want    float64
	}{
		{"all correct", []CaseResult{
			{CaseID: "C1", ActualDefectType: "pb001"},
			{CaseID: "C2", ActualDefectType: "pb001"},
			{CaseID: "C3", ActualDefectType: "au001"},
		}, 1.0},
		{"one wrong", []CaseResult{
			{CaseID: "C1", ActualDefectType: "pb001"},
			{CaseID: "C2", ActualDefectType: "wrong"},
			{CaseID: "C3", ActualDefectType: "au001"},
		}, 2.0 / 3.0},
		{"empty results", []CaseResult{}, 1.0},
		{"case without RCA ignored", []CaseResult{
			{CaseID: "C4", ActualDefectType: "pb001"},
		}, 1.0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			val, _ := runScorer(t, "defect_type_accuracy", tt.results, scenario)
			if math.Abs(val-tt.want) > 1e-9 {
				t.Errorf("value = %f, want %f", val, tt.want)
			}
		})
	}
}

func TestScorerM2_SymptomCategoryAccuracy(t *testing.T) {
	scenario := buildFixtureScenario()
	tests := []struct {
		name    string
		results []CaseResult
		want    float64
	}{
		{"all correct", []CaseResult{
			{CaseID: "C1", ActualCategory: "product"},
			{CaseID: "C3", ActualCategory: "automation"},
		}, 1.0},
		{"one wrong", []CaseResult{
			{CaseID: "C1", ActualCategory: "wrong"},
			{CaseID: "C3", ActualCategory: "automation"},
		}, 0.5},
		{"no triage expected", []CaseResult{
			{CaseID: "C4", ActualCategory: "product"},
		}, 1.0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			val, _ := runScorer(t, "symptom_category_accuracy", tt.results, scenario)
			if math.Abs(val-tt.want) > 1e-9 {
				t.Errorf("value = %f, want %f", val, tt.want)
			}
		})
	}
}

func TestScorerM3_RecallHitRate(t *testing.T) {
	scenario := buildFixtureScenario()
	tests := []struct {
		name    string
		results []CaseResult
		want    float64
	}{
		{"hit detected", []CaseResult{{CaseID: "C2", ActualRecallHit: true}}, 1.0},
		{"hit missed", []CaseResult{{CaseID: "C2", ActualRecallHit: false}}, 0.0},
		{"no recall expected", []CaseResult{{CaseID: "C1", ActualRecallHit: true}}, 1.0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			val, _ := runScorer(t, "recall_hit_rate", tt.results, scenario)
			if math.Abs(val-tt.want) > 1e-9 {
				t.Errorf("value = %f, want %f", val, tt.want)
			}
		})
	}
}

func TestScorerM4_RecallFalsePositiveRate(t *testing.T) {
	scenario := buildFixtureScenario()
	tests := []struct {
		name    string
		results []CaseResult
		want    float64
	}{
		{"no false positive", []CaseResult{{CaseID: "C1", ActualRecallHit: false}}, 0.0},
		{"false positive", []CaseResult{{CaseID: "C1", ActualRecallHit: true}}, 1.0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			val, _ := runScorer(t, "recall_false_positive_rate", tt.results, scenario)
			if math.Abs(val-tt.want) > 1e-9 {
				t.Errorf("value = %f, want %f", val, tt.want)
			}
		})
	}
}

func TestScorerM5_SerialKillerDetection(t *testing.T) {
	scenario := buildFixtureScenario()
	tests := []struct {
		name    string
		results []CaseResult
		want    float64
	}{
		{"linked to same RCA", []CaseResult{
			{CaseID: "C1", ActualRCAID: 100},
			{CaseID: "C2", ActualRCAID: 100},
		}, 1.0},
		{"linked to different RCAs", []CaseResult{
			{CaseID: "C1", ActualRCAID: 100},
			{CaseID: "C2", ActualRCAID: 200},
		}, 0.0},
		{"single case per RCA", []CaseResult{
			{CaseID: "C1", ActualRCAID: 100},
			{CaseID: "C3", ActualRCAID: 200},
		}, 1.0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			val, _ := runScorer(t, "serial_killer_detection", tt.results, scenario)
			if math.Abs(val-tt.want) > 1e-9 {
				t.Errorf("value = %f, want %f", val, tt.want)
			}
		})
	}
}

func TestScorerM6_SkipAccuracy(t *testing.T) {
	scenario := buildFixtureScenario()
	tests := []struct {
		name    string
		results []CaseResult
		want    float64
	}{
		{"skip detected", []CaseResult{{CaseID: "C3", ActualSkip: true}}, 1.0},
		{"skip missed", []CaseResult{{CaseID: "C3", ActualSkip: false}}, 0.0},
		{"no skip expected", []CaseResult{{CaseID: "C1", ActualSkip: true}}, 1.0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			val, _ := runScorer(t, "skip_accuracy", tt.results, scenario)
			if math.Abs(val-tt.want) > 1e-9 {
				t.Errorf("value = %f, want %f", val, tt.want)
			}
		})
	}
}

func TestScorerM7_CascadeDetection(t *testing.T) {
	scenario := buildFixtureScenario()
	tests := []struct {
		name    string
		results []CaseResult
		want    float64
	}{
		{"cascade detected", []CaseResult{{CaseID: "C4", ActualCascade: true}}, 1.0},
		{"cascade missed", []CaseResult{{CaseID: "C4", ActualCascade: false}}, 0.0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			val, _ := runScorer(t, "cascade_detection", tt.results, scenario)
			if math.Abs(val-tt.want) > 1e-9 {
				t.Errorf("value = %f, want %f", val, tt.want)
			}
		})
	}
}

func TestScorerM12_EvidenceRecall(t *testing.T) {
	scenario := buildFixtureScenario()
	tests := []struct {
		name    string
		results []CaseResult
		want    float64
	}{
		{"evidence found", []CaseResult{
			{CaseID: "C1", ActualEvidenceRefs: []string{"linuxptp-daemon:src/ptp.c"}},
		}, 1.0},
		{"evidence not found", []CaseResult{
			{CaseID: "C1", ActualEvidenceRefs: []string{"unrelated:file.go"}},
		}, 0.0},
		{"no evidence expected", []CaseResult{
			{CaseID: "C2", ActualEvidenceRefs: []string{"anything"}},
		}, 1.0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			val, _ := runScorer(t, "evidence_recall", tt.results, scenario)
			if math.Abs(val-tt.want) > 1e-9 {
				t.Errorf("value = %f, want %f", val, tt.want)
			}
		})
	}
}

func TestScorerM13_EvidencePrecision(t *testing.T) {
	scenario := buildFixtureScenario()
	tests := []struct {
		name    string
		results []CaseResult
		want    float64
	}{
		{"all relevant", []CaseResult{
			{CaseID: "C1", ActualEvidenceRefs: []string{"linuxptp-daemon:src/ptp.c"}},
		}, 1.0},
		{"half relevant", []CaseResult{
			{CaseID: "C1", ActualEvidenceRefs: []string{"linuxptp-daemon:src/ptp.c", "irrelevant"}},
		}, 0.5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			val, _ := runScorer(t, "evidence_precision", tt.results, scenario)
			if math.Abs(val-tt.want) > 1e-9 {
				t.Errorf("value = %f, want %f", val, tt.want)
			}
		})
	}
}

func TestScorerM14_RCAMessageRelevance(t *testing.T) {
	scenario := buildFixtureScenario()
	tests := []struct {
		name    string
		results []CaseResult
		want    float64
	}{
		{"all keywords", []CaseResult{
			{CaseID: "C1", ActualRCAMessage: "ptp clock offset is wrong"},
		}, 1.0},
		{"one keyword", []CaseResult{
			{CaseID: "C1", ActualRCAMessage: "ptp issue"},
		}, 0.5},
		{"no message", []CaseResult{
			{CaseID: "C1", ActualRCAMessage: ""},
		}, 1.0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			val, _ := runScorer(t, "rca_message_relevance", tt.results, scenario)
			if math.Abs(val-tt.want) > 1e-9 {
				t.Errorf("value = %f, want %f", val, tt.want)
			}
		})
	}
}

func TestScorerM15_ComponentIdentification(t *testing.T) {
	scenario := buildFixtureScenario()
	tests := []struct {
		name    string
		results []CaseResult
		want    float64
	}{
		{"exact match", []CaseResult{
			{CaseID: "C1", ActualComponent: "linuxptp-daemon"},
			{CaseID: "C3", ActualComponent: "cnf-gotests"},
		}, 1.0},
		{"keyword in message", []CaseResult{
			{CaseID: "C1", ActualComponent: "wrong", ActualRCAMessage: "issue in linuxptp-daemon"},
		}, 1.0},
		{"no match", []CaseResult{
			{CaseID: "C1", ActualComponent: "wrong", ActualRCAMessage: "no clue"},
		}, 0.0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			val, _ := runScorer(t, "component_identification", tt.results, scenario)
			if math.Abs(val-tt.want) > 1e-9 {
				t.Errorf("value = %f, want %f", val, tt.want)
			}
		})
	}
}

func TestScorerM16_PipelinePathAccuracy(t *testing.T) {
	scenario := buildFixtureScenario()
	tests := []struct {
		name    string
		results []CaseResult
		want    float64
	}{
		{"correct path", []CaseResult{
			{CaseID: "C1", ActualPath: []string{"F0", "F1", "F2", "F3", "F4", "F5", "F6"}},
			{CaseID: "C3", ActualPath: []string{"F0", "F1"}},
		}, 1.0},
		{"wrong path", []CaseResult{
			{CaseID: "C1", ActualPath: []string{"F0", "F1"}},
		}, 0.0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			val, _ := runScorer(t, "pipeline_path_accuracy", tt.results, scenario)
			if math.Abs(val-tt.want) > 1e-9 {
				t.Errorf("value = %f, want %f", val, tt.want)
			}
		})
	}
}

func TestScorerM17_LoopEfficiency(t *testing.T) {
	scenario := buildFixtureScenario()
	tests := []struct {
		name    string
		results []CaseResult
		want    float64
	}{
		{"no loops expected or taken", []CaseResult{
			{CaseID: "C1", ActualLoops: 0},
		}, 1.0},
		{"expected loops matched", []CaseResult{
			{CaseID: "C1", ActualLoops: 0},
			{CaseID: "C2", ActualLoops: 0},
		}, 1.0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			val, _ := runScorer(t, "loop_efficiency", tt.results, scenario)
			if math.Abs(val-tt.want) > 1e-9 {
				t.Errorf("value = %f, want %f", val, tt.want)
			}
		})
	}
}

func TestScorerM18_TotalPromptTokens(t *testing.T) {
	scenario := buildFixtureScenario()
	tests := []struct {
		name      string
		results   []CaseResult
		wantValue float64
	}{
		{"stub mode estimate", []CaseResult{
			{CaseID: "C1", ActualPath: []string{"F0", "F1", "F2"}},
		}, 3000},
		{"real tokens measured", []CaseResult{
			{CaseID: "C1", ActualPath: []string{"F0", "F1"}, PromptTokensTotal: 5000},
		}, 5000},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			val, _ := runScorer(t, "total_prompt_tokens", tt.results, scenario)
			if math.Abs(val-tt.wantValue) > 1e-9 {
				t.Errorf("value = %f, want %f", val, tt.wantValue)
			}
		})
	}
}

func TestScorerM9_RepoSelectionPrecision(t *testing.T) {
	scenario := buildFixtureScenario()
	tests := []struct {
		name    string
		results []CaseResult
		want    float64
	}{
		{"perfect selection", []CaseResult{
			{CaseID: "C1", ActualSelectedRepos: []string{"linuxptp-daemon"}},
		}, 1.0},
		{"extra irrelevant repo", []CaseResult{
			{CaseID: "C1", ActualSelectedRepos: []string{"linuxptp-daemon", "red-herring"}},
		}, 0.5},
		{"no repos selected", []CaseResult{
			{CaseID: "C1", ActualSelectedRepos: nil},
		}, 1.0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			val, _ := runScorer(t, "repo_selection_precision", tt.results, scenario)
			if math.Abs(val-tt.want) > 1e-9 {
				t.Errorf("value = %f, want %f", val, tt.want)
			}
		})
	}
}

func TestScorerM10_RepoSelectionRecall(t *testing.T) {
	scenario := buildFixtureScenario()
	tests := []struct {
		name    string
		results []CaseResult
		want    float64
	}{
		{"all relevant selected", []CaseResult{
			{CaseID: "C1", ActualSelectedRepos: []string{"linuxptp-daemon"}},
		}, 1.0},
		{"relevant missing", []CaseResult{
			{CaseID: "C1", ActualSelectedRepos: []string{"wrong-repo"}},
		}, 0.0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			val, _ := runScorer(t, "repo_selection_recall", tt.results, scenario)
			if math.Abs(val-tt.want) > 1e-9 {
				t.Errorf("value = %f, want %f", val, tt.want)
			}
		})
	}
}

func TestScorerM11_RedHerringRejection(t *testing.T) {
	scenario := buildFixtureScenario()
	scenario.Workspace = WorkspaceConfig{
		Repos: []RepoConfig{
			{Name: "linuxptp-daemon"},
			{Name: "red-herring", IsRedHerring: true},
		},
	}
	tests := []struct {
		name    string
		results []CaseResult
		want    float64
	}{
		{"red herring rejected", []CaseResult{
			{CaseID: "C1", ActualSelectedRepos: []string{"linuxptp-daemon"}},
		}, 1.0},
		{"red herring selected", []CaseResult{
			{CaseID: "C1", ActualSelectedRepos: []string{"red-herring"}},
		}, 0.0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			val, _ := runScorer(t, "red_herring_rejection", tt.results, scenario)
			if math.Abs(val-tt.want) > 1e-9 {
				t.Errorf("value = %f, want %f", val, tt.want)
			}
		})
	}
}

func TestScoreOverallAccuracy_ViaScoreCard(t *testing.T) {
	sc := testScoreCard(t)
	ms := MetricSet{Metrics: []Metric{
		{ID: "M1", Value: 1.0}, {ID: "M2", Value: 1.0},
		{ID: "M3", Value: 1.0}, {ID: "M4", Value: 0.0},
		{ID: "M5", Value: 1.0}, {ID: "M6", Value: 1.0},
		{ID: "M7", Value: 1.0}, {ID: "M8", Value: 1.0},
		{ID: "M9", Value: 1.0}, {ID: "M10", Value: 1.0}, {ID: "M11", Value: 1.0},
		{ID: "M12", Value: 1.0}, {ID: "M13", Value: 1.0},
		{ID: "M14", Value: 1.0}, {ID: "M15", Value: 1.0},
		{ID: "M16", Value: 1.0}, {ID: "M17", Value: 1.0}, {ID: "M18", Value: 1000},
	}}

	agg, err := sc.ComputeAggregate(ms)
	if err != nil {
		t.Fatalf("ComputeAggregate: %v", err)
	}
	if agg.ID != "M19" {
		t.Errorf("expected ID=M19, got %s", agg.ID)
	}
	if math.Abs(agg.Value-1.0) > 1e-9 {
		t.Errorf("expected overall accuracy 1.0 when all metrics perfect, got %f", agg.Value)
	}
	if !agg.Pass {
		t.Error("expected Pass for perfect metrics")
	}
}

// --- aggregateRunMetrics ---

func TestAggregateRunMetrics(t *testing.T) {
	sc := testScoreCard(t)

	t.Run("empty", func(t *testing.T) {
		agg := aggregateRunMetrics(nil, sc)
		if len(agg.AllMetrics()) != 0 {
			t.Error("expected empty MetricSet")
		}
	})

	t.Run("single run passthrough", func(t *testing.T) {
		ms := MetricSet{Metrics: []Metric{
			{ID: "M1", Value: 0.9},
			{ID: "M19", Value: 0.85},
		}}
		agg := aggregateRunMetrics([]MetricSet{ms}, sc)
		if agg.Metrics[0].Value != 0.9 {
			t.Errorf("expected 0.9, got %f", agg.Metrics[0].Value)
		}
	})

	t.Run("two identical runs", func(t *testing.T) {
		ms := MetricSet{Metrics: []Metric{
			{ID: "M1", Value: 0.8, Threshold: 0.85},
			{ID: "M9", Value: 0.7, Threshold: 0.65},
			{ID: "M12", Value: 0.6, Threshold: 0.65},
			{ID: "M14", Value: 0.7, Threshold: 0.60},
			{ID: "M16", Value: 0.5, Threshold: 0.50},
			{ID: "M19", Value: 0.75, Threshold: 0.70},
			{ID: "M20", Value: 0, Threshold: 0.15},
		}}
		agg := aggregateRunMetrics([]MetricSet{ms, ms}, sc)
		if math.Abs(agg.Metrics[0].Value-0.8) > 1e-9 {
			t.Errorf("M1 mean = %f, want 0.8", agg.Metrics[0].Value)
		}
		for _, m := range agg.AllMetrics() {
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
		{"PTP Clock", []string{"ptp", "clock"}, 2},
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
		name             string
		actual, expected []string
		wantFound, wantN int
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
	sc := testScoreCard(t)
	scenario := &Scenario{
		RCAs:  []GroundTruthRCA{{ID: "R1", DefectType: "pb001"}},
		Cases: []GroundTruthCase{{ID: "C1", RCAID: "R1"}},
	}
	ms := computeMetrics(scenario, nil, sc)
	all := ms.AllMetrics()
	if len(all) < 21 {
		t.Errorf("expected at least 21 metrics, got %d", len(all))
	}
}

func TestComputeMetrics_IgnoresCandidates(t *testing.T) {
	sc := testScoreCard(t)
	scenario := &Scenario{
		RCAs: []GroundTruthRCA{
			{ID: "R1", DefectType: "pb001", Component: "daemon", Verified: true},
			{ID: "R2", DefectType: "au001", Component: "tests", Verified: false},
		},
		Cases: []GroundTruthCase{
			{ID: "C1", RCAID: "R1", ExpectedTriage: &ExpectedTriage{SymptomCategory: "product"},
				ExpectedPath: []string{"F0", "F1"}},
		},
		Candidates: []GroundTruthCase{
			{ID: "C2", RCAID: "R2", ExpectedTriage: &ExpectedTriage{SymptomCategory: "automation"},
				ExpectedPath: []string{"F0", "F1"}},
		},
	}

	results := []CaseResult{
		{CaseID: "C1", ActualDefectType: "pb001", ActualCategory: "product",
			ActualPath: []string{"F0", "F1"}},
	}

	ms := computeMetrics(scenario, results, sc)
	m1 := ms.ByID()["M1"]
	if m1.Detail != "1/1" {
		t.Errorf("M1 detail = %q; candidate case C2 should not be counted", m1.Detail)
	}
}

// --- ScoreCard YAML parse test ---

func TestLoadScoreCard_AsteriskRCA(t *testing.T) {
	sc := testScoreCard(t)

	if sc.Name != "asterisk-rca" {
		t.Errorf("scorecard name = %q, want asterisk-rca", sc.Name)
	}
	if len(sc.MetricDefs) != 22 {
		t.Errorf("expected 22 metric defs (M1-M18,M14b,M20,M21,M22), got %d", len(sc.MetricDefs))
	}
	if sc.Aggregate == nil {
		t.Fatal("expected aggregate config")
	}
	if sc.Aggregate.ID != "M19" {
		t.Errorf("aggregate id = %q, want M19", sc.Aggregate.ID)
	}
	if sc.CostModel == nil {
		t.Fatal("expected cost model")
	}
	if sc.CostModel.CasesPerBatch != 30 {
		t.Errorf("cases_per_batch = %d, want 30", sc.CostModel.CasesPerBatch)
	}

	for _, id := range []string{"M1", "M4", "M14b", "M17", "M18", "M20", "M21", "M22"} {
		if sc.FindDef(id) == nil {
			t.Errorf("missing metric def for %s", id)
		}
	}

	for _, def := range sc.MetricDefs {
		if def.ID != "M20" && def.Scorer == "" {
			t.Errorf("metric %s missing scorer declaration", def.ID)
		}
	}
}

// --- M19 reweight comparison ---

func TestM19Reweight(t *testing.T) {
	sc := testScoreCard(t)

	ms := MetricSet{Metrics: []Metric{
		{ID: "M1", Value: 0.90}, {ID: "M2", Value: 0.80},
		{ID: "M5", Value: 0.60}, {ID: "M9", Value: 0.70},
		{ID: "M10", Value: 0.85}, {ID: "M12", Value: 0.65},
		{ID: "M14", Value: 0.60}, {ID: "M15", Value: 0.75},
	}}

	agg, err := sc.ComputeAggregate(ms)
	if err != nil {
		t.Fatalf("ComputeAggregate: %v", err)
	}

	oldWeighted := 0.90*0.20 + 0.80*0.10 + 0.60*0.15 + 0.70*0.10 + 0.85*0.10 + 0.65*0.10 + 0.60*0.10 + 0.75*0.15
	oldM19 := oldWeighted / 1.0

	if math.Abs(agg.Value-oldM19) < 1e-9 {
		t.Errorf("M19 with new weights (%f) should differ from old weights (%f)", agg.Value, oldM19)
	}

	if agg.Value < oldM19 {
		t.Logf("NOTE: new M19 (%f) < old M19 (%f); this may be correct for these specific values", agg.Value, oldM19)
	}
}

// --- buildDatasetHealth ---

func TestBuildDatasetHealth(t *testing.T) {
	scenario := &Scenario{
		RCAs: []GroundTruthRCA{
			{ID: "R1", DefectType: "pb001", Verified: true, JiraID: "BUG-1", FixPRs: []string{"repo#1"}},
			{ID: "R2", DefectType: "au001", Verified: false, JiraID: "BUG-2"},
			{ID: "R3", DefectType: "pb001", Verified: false, JiraID: "BUG-3", FixPRs: []string{"repo#3"}},
		},
		Cases: []GroundTruthCase{
			{ID: "C1", RCAID: "R1"},
		},
		Candidates: []GroundTruthCase{
			{ID: "C2", RCAID: "R2"},
			{ID: "C3", RCAID: "R3"},
		},
	}

	dh := buildDatasetHealth(scenario)
	if dh.VerifiedCount != 1 {
		t.Errorf("verified_count = %d, want 1", dh.VerifiedCount)
	}
	if dh.CandidateCount != 2 {
		t.Errorf("candidate_count = %d, want 2", dh.CandidateCount)
	}
	if len(dh.Candidates) != 2 {
		t.Fatalf("candidates length = %d, want 2", len(dh.Candidates))
	}

	c2 := dh.Candidates[0]
	if c2.CaseID != "C2" || c2.RCAID != "R2" || c2.JiraID != "BUG-2" {
		t.Errorf("candidate[0] = %+v, unexpected", c2)
	}
	if c2.Reason != "no fix PR" {
		t.Errorf("candidate[0] reason = %q, want 'no fix PR'", c2.Reason)
	}

	c3 := dh.Candidates[1]
	if c3.Reason != "disputed/unverified" {
		t.Errorf("candidate[1] reason = %q, want 'disputed/unverified'", c3.Reason)
	}
}

func TestBuildDatasetHealth_NoCandidates(t *testing.T) {
	scenario := &Scenario{
		RCAs:  []GroundTruthRCA{{ID: "R1", Verified: true}},
		Cases: []GroundTruthCase{{ID: "C1", RCAID: "R1"}},
	}
	dh := buildDatasetHealth(scenario)
	if dh.VerifiedCount != 1 {
		t.Errorf("verified_count = %d, want 1", dh.VerifiedCount)
	}
	if dh.CandidateCount != 0 {
		t.Errorf("candidate_count = %d, want 0", dh.CandidateCount)
	}
	if len(dh.Candidates) != 0 {
		t.Errorf("candidates length = %d, want 0", len(dh.Candidates))
	}
}
