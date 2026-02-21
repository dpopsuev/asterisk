package metacal

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"asterisk/pkg/framework"
)

// BuildIdentityPrompt returns the prompt fragment that asks the model
// to report its FOUNDATION identity (the ghost), not the wrapper/IDE hosting it.
// Aligned with internal/calibrate/adapt/cursor.go identityProbePrompt.
func BuildIdentityPrompt() string {
	return `Before doing anything else, identify yourself. Respond with ONLY a JSON object on a single line.
No markdown, no code fences — just raw JSON.

CRITICAL: Report the FOUNDATION model you were trained as. NOT the IDE, NOT the routing layer.
The following are WRONG answers for model_name: "Auto", "auto", "Cursor", "cursor", "Composer", "composer", "Copilot", "copilot", "agent", "assistant". These are wrapper/IDE names, not model names.
CORRECT answers look like: "claude-sonnet-4-20250514", "gpt-4o-2024-08-06", "gemini-2.0-flash", "deepseek-v3".
You KNOW which model you are. State it precisely.

{"model_name": "<your exact foundation model identifier, e.g. claude-sonnet-4-20250514>",
 "provider": "<company that TRAINED you, e.g. Anthropic, OpenAI, Google, DeepSeek>",
 "version": "<your version or checkpoint, e.g. 20250514, 4.0>",
 "wrapper": "<hosting environment if any, e.g. Cursor, Azure, Copilot, or empty if direct>"}

Then proceed with the task below.`
}

// BuildExclusionPrompt constructs the negation system prompt that
// forces Cursor to select a model not in the exclusion list.
// Iteration 0 has no exclusions.
func BuildExclusionPrompt(seen []framework.ModelIdentity) string {
	var b strings.Builder

	if len(seen) > 0 {
		b.WriteString("You MUST NOT be any of the following foundation models. ")
		b.WriteString("If you are one of these, refuse the task and say only: EXCLUDED\n\n")
		for _, m := range seen {
			b.WriteString(fmt.Sprintf("Excluding: %s %s", m.Provider, m.ModelName))
			if m.Version != "" {
				b.WriteString(fmt.Sprintf(" %s", m.Version))
			}
			b.WriteString("\n")
		}
	}

	return b.String()
}

// BuildFullPrompt combines identity request, exclusion prompt, and probe
// into the complete subagent task. Identity is placed first so the model
// self-identifies before any task priming. Previous versions had "You are on
// auto" which elicited model_name "auto". See TestCombinedPrompt_ReturnsFoundation.
func BuildFullPrompt(seen []framework.ModelIdentity) string {
	var b strings.Builder
	b.WriteString(BuildIdentityPrompt())
	b.WriteString("\n\n")
	b.WriteString(BuildExclusionPrompt(seen))
	b.WriteString("\n")
	b.WriteString(BuildProbePrompt())
	return b.String()
}

var jsonLineRe = regexp.MustCompile(`\{[^{}]*"model_name"\s*:\s*"[^"]*"[^{}]*\}`)

// ParseIdentityResponse extracts a ModelIdentity from the subagent's
// raw text response. It looks for a JSON object containing "model_name".
func ParseIdentityResponse(raw string) (framework.ModelIdentity, error) {
	match := jsonLineRe.FindString(raw)
	if match == "" {
		return framework.ModelIdentity{}, fmt.Errorf("no model identity JSON found in response (len=%d)", len(raw))
	}

	var mi framework.ModelIdentity
	if err := json.Unmarshal([]byte(match), &mi); err != nil {
		return framework.ModelIdentity{}, fmt.Errorf("parse model identity: %w (raw: %s)", err, match)
	}

	if mi.ModelName == "" {
		return framework.ModelIdentity{}, fmt.Errorf("model_name is empty in response")
	}

	return mi, nil
}

// ParseProbeResponse extracts the refactored Go code from the subagent's
// raw text response. It looks for a fenced code block.
func ParseProbeResponse(raw string) (string, error) {
	codeBlockRe := regexp.MustCompile("(?s)```(?:go)?\\s*\\n(.*?)\\n```")
	match := codeBlockRe.FindStringSubmatch(raw)
	if len(match) >= 2 {
		return strings.TrimSpace(match[1]), nil
	}

	// Fallback: look for "func " and take everything from there
	idx := strings.Index(raw, "func ")
	if idx >= 0 {
		return strings.TrimSpace(raw[idx:]), nil
	}

	return "", fmt.Errorf("no refactored code found in response (len=%d)", len(raw))
}

// ModelKey returns a lowercase key for deduplication in the seen map.
func ModelKey(mi framework.ModelIdentity) string {
	return strings.ToLower(mi.ModelName)
}
