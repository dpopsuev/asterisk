package metacal

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"asterisk/pkg/framework"
)

// BuildIdentityPrompt returns the prompt fragment that asks the model
// to identify itself. The response must be parseable JSON.
func BuildIdentityPrompt() string {
	return `Before doing anything else, identify yourself. Return a JSON object on a single line with exactly these fields:
{"model_name": "<your model name>", "provider": "<your provider>", "version": "<your version or empty string>"}
Do NOT wrap it in a code block. Just the raw JSON line, then proceed with the task.`
}

// BuildExclusionPrompt constructs the negation system prompt that
// forces Cursor to select a model not in the exclusion list.
// Iteration 0 has no exclusions.
func BuildExclusionPrompt(seen []framework.ModelIdentity) string {
	var b strings.Builder
	b.WriteString("You are on auto, select any model.\n")

	if len(seen) > 0 {
		b.WriteString("\nYou MUST NOT be any of the following models. ")
		b.WriteString("If you are one of these, refuse the task and say EXCLUDED.\n\n")
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

// BuildFullPrompt combines the exclusion prompt, identity request, and
// probe prompt into the complete subagent task.
func BuildFullPrompt(seen []framework.ModelIdentity) string {
	var b strings.Builder
	b.WriteString(BuildExclusionPrompt(seen))
	b.WriteString("\n")
	b.WriteString(BuildIdentityPrompt())
	b.WriteString("\n\n")
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
