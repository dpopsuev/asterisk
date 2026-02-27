package rca

import (
	"context"
	"fmt"

	"asterisk/internal/orchestrate"
	"asterisk/adapters/rp"
	"asterisk/adapters/store"

	framework "github.com/dpopsuev/origami"
	"github.com/dpopsuev/origami/knowledge"
)

// ContextBuilder is a transformer that gathers RCA context (store, envelope,
// workspace, knowledge catalog, prior artifacts) and produces TemplateParams
// for prompt generation.
type ContextBuilder struct {
	store    store.Store
	caseData *store.Case
	envelope *rp.Envelope
	catalog  *knowledge.KnowledgeSourceCatalog
	caseDir  string
}

// NewContextBuilder creates a context-builder transformer bound to the given
// runtime dependencies.
func NewContextBuilder(
	st store.Store,
	caseData *store.Case,
	env *rp.Envelope,
	catalog *knowledge.KnowledgeSourceCatalog,
	caseDir string,
) *ContextBuilder {
	return &ContextBuilder{
		store:    st,
		caseData: caseData,
		envelope: env,
		catalog:  catalog,
		caseDir:  caseDir,
	}
}

func (t *ContextBuilder) Name() string { return "context-builder" }

func (t *ContextBuilder) Transform(_ context.Context, tc *framework.TransformerContext) (any, error) {
	stepName, ok := tc.Meta["step"].(string)
	if !ok {
		stepName = tc.NodeName
	}
	step := orchestrate.NodeNameToStep(stepName)
	if step == orchestrate.StepDone {
		return nil, fmt.Errorf("context-builder: unknown step %q", stepName)
	}
	params := orchestrate.BuildParams(t.store, t.caseData, t.envelope, t.catalog, step, t.caseDir)
	return params, nil
}

// PromptFiller is a transformer that fills a Go text/template with TemplateParams.
type PromptFiller struct {
	promptDir string
}

// NewPromptFiller creates a prompt-filler transformer that resolves templates
// from the given directory.
func NewPromptFiller(promptDir string) *PromptFiller {
	return &PromptFiller{promptDir: promptDir}
}

func (t *PromptFiller) Name() string { return "prompt-filler" }

func (t *PromptFiller) Transform(_ context.Context, tc *framework.TransformerContext) (any, error) {
	params, ok := tc.Input.(*orchestrate.TemplateParams)
	if !ok {
		return nil, fmt.Errorf("prompt-filler: expected *TemplateParams input, got %T", tc.Input)
	}

	stepName, ok := tc.Meta["step"].(string)
	if !ok {
		stepName = tc.NodeName
	}
	step := orchestrate.NodeNameToStep(stepName)
	if step == orchestrate.StepDone {
		return nil, fmt.Errorf("prompt-filler: unknown step %q", stepName)
	}

	templatePath := orchestrate.TemplatePathForStep(t.promptDir, step)
	if templatePath == "" {
		return nil, fmt.Errorf("prompt-filler: no template for step %q", stepName)
	}

	filled, err := orchestrate.FillTemplate(templatePath, params)
	if err != nil {
		return nil, fmt.Errorf("prompt-filler: %w", err)
	}
	return filled, nil
}
