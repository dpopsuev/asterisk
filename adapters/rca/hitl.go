package rca

import (
	"context"
	"encoding/json"
	"fmt"

	framework "github.com/dpopsuev/origami"
	"github.com/dpopsuev/origami/adapters/rp"
	"asterisk/adapters/store"
	"github.com/dpopsuev/origami/knowledge"
)

// HITLConfig holds configuration for the interactive HITL circuit mode.
type HITLConfig struct {
	Store     store.Store
	CaseData  *store.Case
	Envelope  *rp.Envelope
	Catalog   *knowledge.KnowledgeSourceCatalog
	PromptDir string
	CaseDir   string
}

// HITLResult is returned by RunHITLStep and ResumeHITLStep to the CLI caller.
type HITLResult struct {
	PromptPath  string
	CurrentStep string
	IsDone      bool
	Explanation string
}

// RunHITLStep runs (or resumes) the circuit until it either pauses for
// human input (Interrupt) or completes. If a checkpoint exists, the walk
// resumes from the last interrupted node.
func RunHITLStep(ctx context.Context, cfg HITLConfig) (*HITLResult, error) {
	th := DefaultThresholds()
	walkerID := fmt.Sprintf("case-%d", cfg.CaseData.ID)

	cp, err := framework.NewJSONCheckpointer(cfg.CaseDir)
	if err != nil {
		return nil, fmt.Errorf("create checkpointer: %w", err)
	}

	hitlAdapter := HITLAdapter()
	storeAdapter := &framework.Adapter{
		Namespace: "store",
		Name:      "rca-store-hooks",
		Hooks:     StoreHooks(cfg.Store, cfg.CaseData),
	}
	runner, err := BuildRunner(th, hitlAdapter, storeAdapter)
	if err != nil {
		return nil, fmt.Errorf("build runner: %w", err)
	}

	walker, startNode, err := prepareWalker(cp, walkerID, cfg)
	if err != nil {
		return nil, err
	}

	wrapped := framework.WrapWithCheckpointer(walker, cp)
	walkErr := runner.Walk(ctx, wrapped, startNode)
	return buildResult(walker, walkErr)
}

// ResumeHITLStep reads a saved artifact and resumes the walk from the
// last checkpointed node.
func ResumeHITLStep(ctx context.Context, cfg HITLConfig, artifactData []byte) (*HITLResult, error) {
	th := DefaultThresholds()
	walkerID := fmt.Sprintf("case-%d", cfg.CaseData.ID)

	cp, err := framework.NewJSONCheckpointer(cfg.CaseDir)
	if err != nil {
		return nil, fmt.Errorf("create checkpointer: %w", err)
	}

	hitlAdapter := HITLAdapter()
	storeAdapter := &framework.Adapter{
		Namespace: "store",
		Name:      "rca-store-hooks",
		Hooks:     StoreHooks(cfg.Store, cfg.CaseData),
	}
	runner, err := BuildRunner(th, hitlAdapter, storeAdapter)
	if err != nil {
		return nil, fmt.Errorf("build runner: %w", err)
	}

	walker, startNode, err := prepareWalker(cp, walkerID, cfg)
	if err != nil {
		return nil, err
	}

	var artifact any
	if err := json.Unmarshal(artifactData, &artifact); err != nil {
		return nil, fmt.Errorf("parse artifact: %w", err)
	}
	walker.State().Context["resume_input"] = artifact

	wrapped := framework.WrapWithCheckpointer(walker, cp)
	walkErr := runner.Walk(ctx, wrapped, startNode)
	return buildResult(walker, walkErr)
}

// LoadCheckpointState loads the WalkerState from the checkpoint directory.
// Returns nil, nil if no checkpoint exists.
func LoadCheckpointState(caseDir string, caseID int64) (*framework.WalkerState, error) {
	cp, err := framework.NewJSONCheckpointer(caseDir)
	if err != nil {
		return nil, err
	}
	walkerID := fmt.Sprintf("case-%d", caseID)
	state, err := cp.Load(walkerID)
	if err != nil {
		return nil, nil
	}
	return state, nil
}

func prepareWalker(cp framework.Checkpointer, walkerID string, cfg HITLConfig) (framework.Walker, string, error) {
	loaded, _ := cp.Load(walkerID)

	walker := framework.NewProcessWalker(walkerID)
	injectHITLContext(walker.State(), cfg)

	startNode := "recall"
	if loaded != nil {
		for k, v := range loaded.LoopCounts {
			walker.State().LoopCounts[k] = v
		}
		walker.State().Status = loaded.Status
		walker.State().CurrentNode = loaded.CurrentNode
		walker.State().History = loaded.History
		walker.State().Outputs = loaded.Outputs
		startNode = loaded.CurrentNode
	}

	return walker, startNode, nil
}

func injectHITLContext(state *framework.WalkerState, cfg HITLConfig) {
	state.Context[KeyStore] = cfg.Store
	state.Context[KeyCaseData] = cfg.CaseData
	state.Context[KeyEnvelope] = cfg.Envelope
	state.Context[KeyCatalog] = cfg.Catalog
	state.Context[KeyCaseDir] = cfg.CaseDir
	state.Context[KeyPromptDir] = cfg.PromptDir
}

func buildResult(walker framework.Walker, walkErr error) (*HITLResult, error) {
	state := walker.State()

	if state.Status == "interrupted" {
		step := state.CurrentNode
		promptPath := ""
		if data, ok := state.Context["interrupt_data"].(map[string]any); ok {
			promptPath, _ = data["prompt_path"].(string)
			if s, ok := data["step"].(string); ok {
				step = s
			}
		}
		return &HITLResult{
			PromptPath:  promptPath,
			CurrentStep: step,
			Explanation: fmt.Sprintf("generated prompt for %s (paste into Cursor)", step),
		}, nil
	}

	if walkErr != nil {
		return nil, fmt.Errorf("walk: %w", walkErr)
	}

	return &HITLResult{
		IsDone:      true,
		CurrentStep: "DONE",
		Explanation: "circuit complete",
	}, nil
}
