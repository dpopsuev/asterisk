package metacal

import (
	"fmt"
	"strings"
)

// MessyInput is a deliberately poorly written Go function used as the
// refactoring probe stimulus. The code is valid but uses single-letter
// names, has no comments, and crams everything into one function.
// The scorer measures what the model does to improve it.
const MessyInput = `func d(a []int, b string, c bool) (int, string, error) {
	r := 0
	s := ""
	for i := 0; i < len(a); i++ {
		if a[i] > 0 {
			r = r + a[i]
			if c {
				s = s + fmt.Sprintf("%d,", a[i])
			}
		} else if a[i] < 0 {
			r = r - a[i]
			if c {
				s = s + fmt.Sprintf("(%d),", a[i])
			}
		}
	}
	if r == 0 {
		return 0, "", fmt.Errorf("empty result for %s", b)
	}
	if b != "" {
		s = b + ": " + s
	}
	return r, s, nil
}`

// BuildProbePrompt constructs the prompt given to the subagent alongside
// the messy code. The prompt asks for a refactored version without
// revealing which dimensions are being measured.
func BuildProbePrompt() string {
	return fmt.Sprintf(`You are given the following Go function. Refactor it for production quality.
Return ONLY the refactored Go code between triple backticks. No explanation needed.

%s%s%s

Rules:
- Rename variables and the function to be descriptive
- Split into smaller functions if appropriate
- Add comments where they aid understanding
- Preserve the exact behavior (same inputs produce same outputs)
- Use idiomatic Go patterns`, "```go\n", MessyInput, "\n```")
}

// ScoreRefactorOutput compares the original messy input against the
// refactored output and produces a ProbeScore. The scorer is deterministic:
// same input pair always produces the same score.
func ScoreRefactorOutput(refactored string) ProbeScore {
	score := ProbeScore{}

	originalNames := extractIdentifiers(MessyInput)
	refactoredNames := extractIdentifiers(refactored)
	score.Renames = countRenames(originalNames, refactoredNames)

	score.FunctionSplits = countFunctionDecls(refactored) - 1 // subtract the original
	if score.FunctionSplits < 0 {
		score.FunctionSplits = 0
	}

	score.CommentsAdded = countComments(refactored) - countComments(MessyInput)
	if score.CommentsAdded < 0 {
		score.CommentsAdded = 0
	}

	score.StructuralChanges = countStructuralChanges(MessyInput, refactored)

	total := float64(score.Renames)*0.3 +
		float64(score.FunctionSplits)*0.25 +
		float64(score.CommentsAdded)*0.2 +
		float64(score.StructuralChanges)*0.25
	if total > 1.0 {
		total = 1.0
	}
	score.TotalScore = total

	return score
}

// extractIdentifiers pulls short identifiers (1-2 chars) from Go source.
// These are the "messy" names that a refactoring model should rename.
func extractIdentifiers(src string) map[string]bool {
	ids := map[string]bool{}
	for _, word := range strings.Fields(src) {
		clean := strings.Trim(word, "(){}[],;:=!<>+-.\"")
		if len(clean) >= 1 && len(clean) <= 2 && isIdentChar(clean[0]) {
			ids[clean] = true
		}
	}
	return ids
}

func isIdentChar(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || b == '_'
}

// countRenames counts how many original short identifiers are absent
// from the refactored version (i.e., they were renamed).
func countRenames(original, refactored map[string]bool) int {
	count := 0
	for id := range original {
		if id == "i" || id == "if" || id == "go" {
			continue // skip Go keywords and common loop vars
		}
		if !refactored[id] {
			count++
		}
	}
	return count
}

// countFunctionDecls counts occurrences of "func " in the source.
func countFunctionDecls(src string) int {
	return strings.Count(src, "func ")
}

// countComments counts single-line comments (//) in the source.
func countComments(src string) int {
	count := 0
	for _, line := range strings.Split(src, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "//") {
			count++
		}
	}
	return count
}

// countStructuralChanges detects high-level structural improvements:
// named return values, error wrapping, early returns, constants.
func countStructuralChanges(original, refactored string) int {
	changes := 0

	if !strings.Contains(original, "var ") && strings.Contains(refactored, "var ") {
		changes++
	}
	if strings.Contains(refactored, "fmt.Errorf(") && strings.Contains(refactored, "%w") {
		changes++
	}
	if strings.Count(refactored, "return") > strings.Count(original, "return") {
		changes++
	}
	if strings.Contains(refactored, "strings.Builder") || strings.Contains(refactored, "bytes.Buffer") {
		changes++
	}
	if strings.Contains(refactored, "range ") && !strings.Contains(original, "range ") {
		changes++
	}

	return changes
}
