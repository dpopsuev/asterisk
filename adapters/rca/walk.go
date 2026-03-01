package rca

import (
	"context"
	"fmt"

	"asterisk/adapters/store"

	framework "github.com/dpopsuev/origami"
)

// WalkConfig holds configuration for a walk-based RCA run.
type WalkConfig struct {
	Store      store.Store
	CaseData   *store.Case
	Adapter    ModelAdapter
	CaseLabel  string
	Thresholds Thresholds
	Hooks      framework.HookRegistry
}

// WalkResult captures the outcome of a walk-based RCA.
type WalkResult struct {
	Path          []string
	Artifact      framework.Artifact
	StepArtifacts map[string]framework.Artifact
}

// WalkCase runs a single case through the RCA circuit using a real graph walk
// with processing nodes instead of the procedural runner loop.
func WalkCase(ctx context.Context, cfg WalkConfig) (*WalkResult, error) {
	nodes := NodeRegistry()

	th := cfg.Thresholds
	if th == (Thresholds{}) {
		th = DefaultThresholds()
	}

	runner, err := BuildRunnerWith(th, nodes, cfg.Hooks)
	if err != nil {
		return nil, fmt.Errorf("build runner: %w", err)
	}

	walker := framework.NewProcessWalker(cfg.CaseLabel)
	walker.State().Context[KeyAdapter] = cfg.Adapter
	walker.State().Context[KeyCaseLabel] = cfg.CaseLabel

	var path []string
	var lastArtifact framework.Artifact
	stepArtifacts := map[string]framework.Artifact{}

	observer := framework.WalkObserverFunc(func(event framework.WalkEvent) {
		if event.Type == framework.EventNodeEnter {
			path = append(path, event.Node)
		}
		if event.Type == framework.EventNodeExit && event.Artifact != nil {
			lastArtifact = event.Artifact
			stepArtifacts[event.Node] = event.Artifact
		}
	})

	if dg, ok := runner.Graph.(*framework.DefaultGraph); ok {
		dg.SetObserver(observer)
	}

	def, err := AsteriskCircuitDef(th)
	if err != nil {
		return nil, fmt.Errorf("load circuit def: %w", err)
	}

	walkErr := runner.Walk(ctx, walker, def.Start)
	if walkErr != nil {
		return nil, fmt.Errorf("walk: %w", walkErr)
	}

	return &WalkResult{
		Path:          path,
		Artifact:      lastArtifact,
		StepArtifacts: stepArtifacts,
	}, nil
}
