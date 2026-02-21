package metacal

import (
	"strings"
	"testing"
)

func TestBuildProbePrompt_ContainsMessyInput(t *testing.T) {
	prompt := BuildProbePrompt()

	if !strings.Contains(prompt, "func d(") {
		t.Error("prompt should contain the messy function")
	}
	if !strings.Contains(prompt, "Refactor") {
		t.Error("prompt should ask for refactoring")
	}
	if !strings.Contains(prompt, "```go") {
		t.Error("prompt should wrap code in fenced block")
	}
}

func TestScoreRefactorOutput_NoChanges(t *testing.T) {
	score := ScoreRefactorOutput(MessyInput)

	if score.Renames != 0 {
		t.Errorf("renames: got %d, want 0 (no changes)", score.Renames)
	}
	if score.FunctionSplits != 0 {
		t.Errorf("splits: got %d, want 0", score.FunctionSplits)
	}
	if score.CommentsAdded != 0 {
		t.Errorf("comments: got %d, want 0", score.CommentsAdded)
	}
}

func TestScoreRefactorOutput_GoodRefactor(t *testing.T) {
	refactored := `// sumAbsolute computes the absolute sum of integers and builds a formatted label.
func sumAbsolute(values []int, label string, verbose bool) (int, string, error) {
	total := 0
	var builder strings.Builder
	for _, value := range values {
		if value > 0 {
			total += value
			if verbose {
				fmt.Fprintf(&builder, "%d,", value)
			}
		} else if value < 0 {
			total -= value
			if verbose {
				fmt.Fprintf(&builder, "(%d),", value)
			}
		}
	}
	if total == 0 {
		return 0, "", fmt.Errorf("empty result for %s", label)
	}
	result := builder.String()
	if label != "" {
		result = label + ": " + result
	}
	return total, result, nil
}`

	score := ScoreRefactorOutput(refactored)

	if score.Renames < 2 {
		t.Errorf("renames: got %d, want >= 2 (d→sumAbsolute, r→total, s→builder, etc.)", score.Renames)
	}
	if score.CommentsAdded < 1 {
		t.Errorf("comments: got %d, want >= 1", score.CommentsAdded)
	}
	if score.StructuralChanges < 1 {
		t.Errorf("structural: got %d, want >= 1 (range, strings.Builder)", score.StructuralChanges)
	}
	if score.TotalScore <= 0 {
		t.Errorf("total: got %f, want > 0", score.TotalScore)
	}

	t.Logf("Score: renames=%d splits=%d comments=%d structural=%d total=%.2f",
		score.Renames, score.FunctionSplits, score.CommentsAdded, score.StructuralChanges, score.TotalScore)
}

func TestScoreRefactorOutput_SplitFunctions(t *testing.T) {
	refactored := `func processItems(items []int, label string, verbose bool) (int, string, error) {
	total := computeTotal(items)
	summary := buildSummary(items, verbose)
	if total == 0 {
		return 0, "", fmt.Errorf("empty result for %s", label)
	}
	if label != "" {
		summary = label + ": " + summary
	}
	return total, summary, nil
}

func computeTotal(items []int) int {
	total := 0
	for _, v := range items {
		if v < 0 {
			total -= v
		} else {
			total += v
		}
	}
	return total
}

func buildSummary(items []int, verbose bool) string {
	if !verbose {
		return ""
	}
	var parts []string
	for _, v := range items {
		if v > 0 {
			parts = append(parts, fmt.Sprintf("%d", v))
		} else if v < 0 {
			parts = append(parts, fmt.Sprintf("(%d)", v))
		}
	}
	return strings.Join(parts, ",")
}`

	score := ScoreRefactorOutput(refactored)

	if score.FunctionSplits < 2 {
		t.Errorf("splits: got %d, want >= 2 (3 funcs - 1 original)", score.FunctionSplits)
	}

	t.Logf("Score: renames=%d splits=%d comments=%d structural=%d total=%.2f",
		score.Renames, score.FunctionSplits, score.CommentsAdded, score.StructuralChanges, score.TotalScore)
}

func TestScoreRefactorOutput_Deterministic(t *testing.T) {
	refactored := `func calculate(numbers []int, name string, log bool) (int, string, error) {
	result := 0
	output := ""
	for _, n := range numbers {
		if n > 0 { result += n } else if n < 0 { result -= n }
	}
	return result, output, nil
}`

	score1 := ScoreRefactorOutput(refactored)
	score2 := ScoreRefactorOutput(refactored)

	if score1 != score2 {
		t.Errorf("non-deterministic: %+v != %+v", score1, score2)
	}
}

func TestCountFunctionDecls(t *testing.T) {
	src := "func a() {}\nfunc b() {}\nfunc c() {}"
	if got := countFunctionDecls(src); got != 3 {
		t.Errorf("got %d, want 3", got)
	}
}

func TestCountComments(t *testing.T) {
	src := "// comment 1\ncode\n// comment 2\n  // indented"
	if got := countComments(src); got != 3 {
		t.Errorf("got %d, want 3", got)
	}
}

func TestExtractIdentifiers(t *testing.T) {
	ids := extractIdentifiers("r := 0\n s = s + b\n c {")
	for _, expected := range []string{"r", "s", "b", "c"} {
		if !ids[expected] {
			t.Errorf("expected identifier %q not found in %v", expected, ids)
		}
	}
}
