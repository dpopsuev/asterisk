package rca

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	framework "github.com/dpopsuev/origami"
	"github.com/dpopsuev/origami/dispatch"
)

const calibrationPreamble = `> **CALIBRATION MODE — BLIND EVALUATION**
>
> You are participating in a calibration run. Your responses at each circuit
> step will be **scored against known ground truth** using 20 metrics including
> defect type accuracy, component identification, evidence quality, circuit
> path efficiency, and semantic relevance.
>
> **Rules:**
> 1. Respond ONLY based on the information provided in this prompt.
> 2. Do NOT read scenario definition files, ground truth files, expected
>    results, or any calibration/test code in the repository. This includes
>    any file under ` + "`internal/calibrate/scenarios/`" + `, any ` + "`*_test.go`" + ` file,
>    and the ` + "`.cursor/contracts/`" + ` directory.
> 3. Do NOT look at previous artifact files for other cases unless
>    explicitly referenced in the prompt context.
> 4. Treat each step independently — base your output solely on the
>    provided context for THIS step.
>
> Violating these rules contaminates the calibration signal.

`

type rcaTransformer struct {
	dispatcher dispatch.Dispatcher
	promptDir  string
	basePath   string
}

type RCATransformerOption func(*rcaTransformer)

func WithRCABasePath(p string) RCATransformerOption {
	return func(t *rcaTransformer) { t.basePath = p }
}

func NewRCATransformer(d dispatch.Dispatcher, promptDir string, opts ...RCATransformerOption) framework.Transformer {
	t := &rcaTransformer{
		dispatcher: d,
		promptDir:  promptDir,
		basePath:   DefaultBasePath,
	}
	for _, opt := range opts {
		opt(t)
	}
	return t
}

func (t *rcaTransformer) Name() string { return "rca" }

func (t *rcaTransformer) Transform(ctx context.Context, tc *framework.TransformerContext) (any, error) {
	step := NodeNameToStep(tc.NodeName)
	if step == "" {
		return nil, fmt.Errorf("rca transformer: unknown node %q", tc.NodeName)
	}

	params := ParamsFromContext(tc.WalkerState.Context)
	params.StepName = string(step)

	templatePath := TemplatePathForStep(t.promptDir, step)
	if templatePath == "" {
		return nil, fmt.Errorf("rca transformer: no template for step %s", step)
	}

	prompt, err := FillTemplate(templatePath, params)
	if err != nil {
		return nil, fmt.Errorf("rca transformer: fill template for %s: %w", step, err)
	}
	prompt = calibrationPreamble + prompt

	caseDir, _ := tc.WalkerState.Context[KeyCaseDir].(string)
	if caseDir == "" {
		caseDir = os.TempDir()
	}

	promptFile := filepath.Join(caseDir, fmt.Sprintf("prompt-%s.md", step.Family()))
	if err := os.WriteFile(promptFile, []byte(prompt), 0644); err != nil {
		return nil, fmt.Errorf("rca transformer: write prompt: %w", err)
	}

	artifactFile := filepath.Join(caseDir, ArtifactFilename(step))

	caseLabel, _ := tc.WalkerState.Context[KeyCaseLabel].(string)
	if caseLabel == "" {
		caseLabel = tc.WalkerState.ID
	}

	data, err := t.dispatcher.Dispatch(dispatch.DispatchContext{
		CaseID: caseLabel, Step: string(step),
		PromptPath: promptFile, ArtifactPath: artifactFile,
	})
	if err != nil {
		return nil, fmt.Errorf("rca transformer: dispatch %s/%s: %w", caseLabel, step, err)
	}

	if f := dispatch.UnwrapFinalizer(t.dispatcher); f != nil {
		f.MarkDone(artifactFile)
	}

	return parseTypedArtifact(step, json.RawMessage(data))
}
