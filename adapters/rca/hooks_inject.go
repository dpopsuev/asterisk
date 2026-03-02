package rca

import (
	"context"

	framework "github.com/dpopsuev/origami"
	"github.com/dpopsuev/origami/adapters/rp"
	"asterisk/adapters/store"
	"github.com/dpopsuev/origami/knowledge"
)

// Context keys used by inject hooks to store assembled template data.
const (
	KeyParamsEnvelope  = "params.envelope"
	KeyParamsFailure   = "params.failure"
	KeyParamsWorkspace = "params.workspace"
	KeyParamsHistory   = "params.history"
	KeyParamsDigest    = "params.recall_digest"
	KeyParamsSources   = "params.sources"
	KeyParamsPrior     = "params.prior"
	KeyParamsTaxonomy  = "params.taxonomy"
)

// InjectHooks creates a HookRegistry with the inject.* before-hooks
// that populate walker.Context with per-concern template data.
// Each hook uses WalkerStateFromContext to write into walker.Context.
func InjectHooks(st store.Store, caseData *store.Case, env *rp.Envelope, catalog *knowledge.KnowledgeSourceCatalog, caseDir string) framework.HookRegistry {
	reg := framework.HookRegistry{}

	reg.Register(newInjectEnvelopeHook(env))
	reg.Register(newInjectFailureHook(caseData))
	reg.Register(newInjectWorkspaceHook(env, catalog))
	reg.Register(newInjectHistoryHook(st, caseData))
	reg.Register(newInjectRecallDigestHook(st))
	reg.Register(newInjectSourcesHook(catalog))
	reg.Register(newInjectPriorHook(caseDir))
	reg.Register(newInjectTaxonomyHook())

	return reg
}

func newInjectEnvelopeHook(env *rp.Envelope) framework.Hook {
	return framework.NewHookFunc("inject.envelope", func(ctx context.Context, _ string, _ framework.Artifact) error {
		ws := framework.WalkerStateFromContext(ctx)
		if ws == nil {
			return nil
		}
		injectEnvelopeData(env, ws.Context)
		return nil
	})
}

func newInjectFailureHook(caseData *store.Case) framework.Hook {
	return framework.NewHookFunc("inject.failure", func(ctx context.Context, _ string, _ framework.Artifact) error {
		ws := framework.WalkerStateFromContext(ctx)
		if ws == nil {
			return nil
		}
		injectFailureData(caseData, ws.Context)
		return nil
	})
}

func newInjectWorkspaceHook(env *rp.Envelope, catalog *knowledge.KnowledgeSourceCatalog) framework.Hook {
	return framework.NewHookFunc("inject.workspace", func(ctx context.Context, _ string, _ framework.Artifact) error {
		ws := framework.WalkerStateFromContext(ctx)
		if ws == nil {
			return nil
		}
		injectWorkspaceData(env, catalog, ws.Context)
		return nil
	})
}

func newInjectHistoryHook(st store.Store, caseData *store.Case) framework.Hook {
	return framework.NewHookFunc("inject.history", func(ctx context.Context, _ string, _ framework.Artifact) error {
		ws := framework.WalkerStateFromContext(ctx)
		if ws == nil {
			return nil
		}
		injectHistoryData(st, caseData, ws.Context)
		return nil
	})
}

func newInjectRecallDigestHook(st store.Store) framework.Hook {
	return framework.NewHookFunc("inject.recall-digest", func(ctx context.Context, _ string, _ framework.Artifact) error {
		ws := framework.WalkerStateFromContext(ctx)
		if ws == nil {
			return nil
		}
		injectRecallDigestData(st, ws.Context)
		return nil
	})
}

func newInjectSourcesHook(catalog *knowledge.KnowledgeSourceCatalog) framework.Hook {
	return framework.NewHookFunc("inject.sources", func(ctx context.Context, _ string, _ framework.Artifact) error {
		ws := framework.WalkerStateFromContext(ctx)
		if ws == nil {
			return nil
		}
		injectSourcesData(catalog, ws.Context)
		return nil
	})
}

func newInjectPriorHook(caseDir string) framework.Hook {
	return framework.NewHookFunc("inject.prior", func(ctx context.Context, _ string, _ framework.Artifact) error {
		ws := framework.WalkerStateFromContext(ctx)
		if ws == nil {
			return nil
		}
		injectPriorData(caseDir, ws.Context)
		return nil
	})
}

func newInjectTaxonomyHook() framework.Hook {
	return framework.NewHookFunc("inject.taxonomy", func(ctx context.Context, _ string, _ framework.Artifact) error {
		ws := framework.WalkerStateFromContext(ctx)
		if ws == nil {
			return nil
		}
		injectTaxonomyData(ws.Context)
		return nil
	})
}

// ParamsFromContext assembles a TemplateParams from walker context.
// Before-hooks inject their data into walker.Context with keys like
// "params.envelope", "params.failure", etc. This function collects
// them into the TemplateParams structure that templates expect.
func ParamsFromContext(walkerCtx map[string]any) *TemplateParams {
	params := &TemplateParams{}

	if v, ok := walkerCtx[KeyParamsEnvelope].(*EnvelopeParams); ok {
		params.Envelope = v
		params.LaunchID = v.RunID
	}

	if v, ok := walkerCtx[KeyParamsFailure].(*FailureParams); ok {
		params.Failure = v
	}

	if v, ok := walkerCtx[KeyParamsWorkspace].(*WorkspaceParams); ok {
		params.Workspace = v
	}

	if v, ok := walkerCtx[KeyParamsHistory].(*HistoryParams); ok {
		params.History = v
	}

	if v, ok := walkerCtx[KeyParamsDigest].([]RecallDigestEntry); ok {
		params.RecallDigest = v
	}

	if v, ok := walkerCtx[KeyParamsSources].([]AlwaysReadSource); ok {
		params.AlwaysReadSources = v
	}

	if v, ok := walkerCtx[KeyParamsPrior].(*PriorParams); ok {
		params.Prior = v
	}

	if v, ok := walkerCtx[KeyParamsTaxonomy].(*TaxonomyParams); ok {
		params.Taxonomy = v
	}

	if cd, ok := walkerCtx[KeyCaseData].(*store.Case); ok {
		params.CaseID = cd.ID
	}

	if _, ok := walkerCtx[KeyParamsEnvelope].(*EnvelopeParams); ok {
		if env, ok := walkerCtx[KeyEnvelope].(*rp.Envelope); ok {
			for _, f := range env.FailureList {
				params.Siblings = append(params.Siblings, SiblingParams{
					ID: f.ID, Name: f.Name, Status: f.Status,
				})
			}
		}
	}

	if params.Timestamps == nil {
		params.Timestamps = &TimestampParams{
			ClockPlaneNote: "Note: Timestamps may originate from different clock planes (executor, test node, SUT). Cross-plane time comparisons may be unreliable.",
		}
	}

	return params
}

// Concrete implementations that actually inject data into walker context.

func injectEnvelopeData(env *rp.Envelope, walkerCtx map[string]any) {
	if env == nil {
		return
	}
	walkerCtx[KeyParamsEnvelope] = &EnvelopeParams{
		Name:  env.Name,
		RunID: env.RunID,
	}
}

func injectFailureData(caseData *store.Case, walkerCtx map[string]any) {
	if caseData == nil {
		return
	}
	walkerCtx[KeyParamsFailure] = &FailureParams{
		TestName:     caseData.Name,
		ErrorMessage: caseData.ErrorMessage,
		LogSnippet:   caseData.LogSnippet,
		LogTruncated: caseData.LogTruncated,
		Status:       caseData.Status,
	}
}

func injectWorkspaceData(env *rp.Envelope, catalog *knowledge.KnowledgeSourceCatalog, walkerCtx map[string]any) {
	walkerCtx[KeyParamsWorkspace] = buildWorkspaceParams(env, catalog)
}

func injectHistoryData(st store.Store, caseData *store.Case, walkerCtx map[string]any) {
	if st == nil || caseData == nil {
		return
	}
	if caseData.SymptomID != 0 {
		walkerCtx[KeyParamsHistory] = loadHistory(st, caseData.SymptomID)
	} else {
		walkerCtx[KeyParamsHistory] = findRecallCandidates(st, caseData.Name)
	}
}

func injectRecallDigestData(st store.Store, walkerCtx map[string]any) {
	if st == nil {
		return
	}
	walkerCtx[KeyParamsDigest] = buildRecallDigest(st)
}

func injectSourcesData(catalog *knowledge.KnowledgeSourceCatalog, walkerCtx map[string]any) {
	if catalog == nil {
		return
	}
	walkerCtx[KeyParamsSources] = loadAlwaysReadSources(catalog)
}

func injectPriorData(caseDir string, walkerCtx map[string]any) {
	if caseDir == "" {
		return
	}
	walkerCtx[KeyParamsPrior] = loadPriorArtifacts(caseDir)
}

func injectTaxonomyData(walkerCtx map[string]any) {
	walkerCtx[KeyParamsTaxonomy] = DefaultTaxonomy()
}

