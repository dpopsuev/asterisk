// Package rca provides an Origami adapter that bundles the RCA circuit's
// hooks, transformers, and extractors under the "rca" namespace.
package rca

import (
	"github.com/dpopsuev/origami/adapters/rp"
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

// Adapter returns an Origami Adapter bundling all RCA circuit plumbing
// (store hooks, context-builder transformer, prompt-filler transformer,
// and per-step extractors) under the "rca" namespace.
func Adapter(cfg AdapterConfig) *framework.Adapter {
	return &framework.Adapter{
		Namespace:    "rca",
		Name:         "asterisk-rca",
		Version:      "1.0.0",
		Description:  "RCA circuit plumbing for CI root-cause analysis",
		Transformers: buildTransformers(cfg),
		Extractors:   buildExtractors(),
		Hooks:        buildHooks(cfg),
	}
}

// allNodeNames lists every RCA circuit node name. Used when registering
// a single monolithic transformer under each node for backwards compat.
var allNodeNames = []string{"recall", "triage", "resolve", "investigate", "correlate", "review", "report"}

// HeuristicAdapter returns an Adapter with per-node heuristic transformers
// that implement deterministic, keyword-based RCA logic.
func HeuristicAdapter(st store.Store, repos []string) *framework.Adapter {
	ht := NewHeuristicTransformer(st, repos)
	return &framework.Adapter{
		Namespace: "rca",
		Name:      "rca-heuristic",
		Transformers: framework.TransformerRegistry{
			"recall":      &recallHeuristic{ht: ht},
			"triage":      &triageHeuristic{ht: ht},
			"resolve":     &resolveHeuristic{ht: ht},
			"investigate": &investigateHeuristic{ht: ht},
			"correlate":   &correlateHeuristic{ht: ht},
			"review":      &reviewHeuristic{},
			"report":      &reportHeuristic{},
		},
	}
}

// TransformerAdapter wraps a monolithic framework.Transformer (e.g. stub, rca)
// and registers it under every node name so that DSL transformer: resolution
// can find it. The transformer's Transform() dispatches on tc.NodeName.
func TransformerAdapter(t framework.Transformer) *framework.Adapter {
	reg := framework.TransformerRegistry{}
	for _, name := range allNodeNames {
		reg[name] = t
	}
	return &framework.Adapter{
		Namespace:    "rca",
		Name:         "rca-transformer",
		Transformers: reg,
	}
}

// HITLAdapter returns an Adapter with per-node HITL transformers that
// fill prompt templates and return framework.Interrupt for human input.
func HITLAdapter() *framework.Adapter {
	reg := framework.TransformerRegistry{}
	steps := map[string]CircuitStep{
		"recall": StepF0Recall, "triage": StepF1Triage, "resolve": StepF2Resolve,
		"investigate": StepF3Invest, "correlate": StepF4Correlate, "review": StepF5Review,
		"report": StepF6Report,
	}
	for name, step := range steps {
		reg[name] = &hitlTransformerNode{step: step}
	}
	return &framework.Adapter{
		Namespace:    "rca",
		Name:         "rca-hitl",
		Transformers: reg,
	}
}

func buildTransformers(_ AdapterConfig) framework.TransformerRegistry {
	return framework.TransformerRegistry{}
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

	inject := InjectHooks(cfg.Store, cfg.CaseData, cfg.Envelope, cfg.Catalog, cfg.CaseDir)
	for name, h := range inject {
		reg[name] = h
	}

	if cfg.Store != nil && cfg.CaseData != nil {
		hooks := StoreHooks(cfg.Store, cfg.CaseData)
		for name, h := range hooks {
			reg[name] = h
		}
	}
	return reg
}
