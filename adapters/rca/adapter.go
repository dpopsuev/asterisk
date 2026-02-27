// Package rca provides an Origami adapter that bundles the RCA pipeline's
// hooks, transformers, and extractors under the "rca" namespace.
package rca

import (
	"asterisk/adapters/rp"
	"asterisk/adapters/store"

	framework "github.com/dpopsuev/origami"
	"github.com/dpopsuev/origami/knowledge"
)

// AdapterConfig holds runtime dependencies injected into the RCA adapter.
type AdapterConfig struct {
	Store     store.Store
	CaseData  *store.Case
	Envelope  *rp.Envelope
	Catalog   *knowledge.KnowledgeSourceCatalog
	PromptDir string
	CaseDir   string
}

// Adapter returns an Origami Adapter bundling all RCA pipeline plumbing
// (store hooks, context-builder transformer, prompt-filler transformer,
// and per-step extractors) under the "rca" namespace.
func Adapter(cfg AdapterConfig) *framework.Adapter {
	return &framework.Adapter{
		Namespace:    "rca",
		Name:         "asterisk-rca",
		Version:      "1.0.0",
		Description:  "RCA pipeline plumbing for CI root-cause analysis",
		Transformers: buildTransformers(cfg),
		Extractors:   buildExtractors(),
		Hooks:        buildHooks(cfg),
	}
}

func buildTransformers(cfg AdapterConfig) framework.TransformerRegistry {
	reg := framework.TransformerRegistry{}
	reg["context-builder"] = NewContextBuilder(cfg.Store, cfg.CaseData, cfg.Envelope, cfg.Catalog, cfg.CaseDir)
	reg["prompt-filler"] = NewPromptFiller(cfg.PromptDir)
	return reg
}

func buildExtractors() framework.ExtractorRegistry {
	reg := framework.ExtractorRegistry{}
	reg["recall"] = NewStepExtractor[RecallResult]("recall")
	reg["triage"] = NewStepExtractor[TriageResult]("triage")
	reg["resolve"] = NewStepExtractor[ResolveResult]("resolve")
	reg["investigate"] = NewStepExtractor[InvestigateArtifact]("investigate")
	reg["correlate"] = NewStepExtractor[CorrelateResult]("correlate")
	reg["review"] = NewStepExtractor[ReviewDecision]("review")
	reg["report"] = NewStepExtractor[InvestigateArtifact]("report")
	return reg
}

func buildHooks(cfg AdapterConfig) framework.HookRegistry {
	reg := framework.HookRegistry{}
	if cfg.Store != nil && cfg.CaseData != nil {
		hooks := StoreHooks(cfg.Store, cfg.CaseData)
		for name, h := range hooks {
			reg[name] = h
		}
	}
	return reg
}
