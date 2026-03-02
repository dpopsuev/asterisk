package rca

import (
	"context"
	"encoding/json"
	"fmt"

	framework "github.com/dpopsuev/origami"
)

// hitlTransformerNode implements framework.Transformer for HITL mode.
// It fills a prompt template and returns framework.Interrupt to pause
// the walk for human input. On resume, the caller injects the artifact
// via the "resume_input" context key, and this transformer parses and
// returns it.
type hitlTransformerNode struct {
	step CircuitStep
}

func (t *hitlTransformerNode) Name() string { return "hitl-" + string(t.step) }

func (t *hitlTransformerNode) Transform(_ context.Context, tc *framework.TransformerContext) (any, error) {
	if input, ok := tc.WalkerState.Context["resume_input"]; ok {
		delete(tc.WalkerState.Context, "resume_input")
		data, err := json.Marshal(input)
		if err != nil {
			return nil, fmt.Errorf("hitl %s: marshal resume_input: %w", t.step, err)
		}
		return parseTypedArtifact(t.step, json.RawMessage(data))
	}

	promptDir, _ := tc.WalkerState.Context[KeyPromptDir].(string)
	caseDir, _ := tc.WalkerState.Context[KeyCaseDir].(string)

	if promptDir == "" {
		promptDir = ".cursor/prompts"
	}

	params := ParamsFromContext(tc.WalkerState.Context)
	params.StepName = string(t.step)

	templatePath := TemplatePathForStep(promptDir, t.step)
	if templatePath == "" {
		return nil, fmt.Errorf("hitl %s: no template for step", t.step)
	}

	prompt, err := FillTemplate(templatePath, params)
	if err != nil {
		return nil, fmt.Errorf("hitl %s: fill template: %w", t.step, err)
	}

	loopIter := tc.WalkerState.LoopCounts[tc.NodeName]
	promptPath, err := WritePrompt(caseDir, t.step, loopIter, prompt)
	if err != nil {
		return nil, fmt.Errorf("hitl %s: write prompt: %w", t.step, err)
	}

	return nil, framework.Interrupt{
		Reason: fmt.Sprintf("awaiting human input for %s", t.step),
		Data:   map[string]any{"prompt_path": promptPath, "step": string(t.step)},
	}
}
