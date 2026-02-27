package rca

import (
	_ "embed"
	"fmt"

	framework "github.com/dpopsuev/origami"
)

//go:embed pipeline_rca.yaml
var pipelineRCAYAML []byte

// ThresholdsToVars converts typed Thresholds to a map for pipeline vars / expression config.
func ThresholdsToVars(th Thresholds) map[string]any {
	return map[string]any{
		"recall_hit":             th.RecallHit,
		"recall_uncertain":       th.RecallUncertain,
		"convergence_sufficient": th.ConvergenceSufficient,
		"max_investigate_loops":  th.MaxInvestigateLoops,
		"correlate_dup":          th.CorrelateDup,
	}
}

// AsteriskPipelineDef loads the RCA pipeline from the embedded YAML and
// overrides vars with the provided thresholds.
func AsteriskPipelineDef(th Thresholds) (*framework.PipelineDef, error) {
	def, err := framework.LoadPipeline(pipelineRCAYAML)
	if err != nil {
		return nil, fmt.Errorf("load embedded pipeline YAML: %w", err)
	}
	def.Vars = ThresholdsToVars(th)
	return def, nil
}

// BuildRunner constructs a framework.Runner from the Asterisk pipeline
// definition with the given thresholds. Expression edges are compiled at
// build time and evaluate against the config derived from thresholds.
// Accepts an optional NodeRegistry; if nil, uses the legacy passthrough registry.
func BuildRunner(th Thresholds, hooks ...framework.HookRegistry) (*framework.Runner, error) {
	return BuildRunnerWith(th, nil, hooks...)
}

// BuildRunnerWith constructs a framework.Runner using the provided node registry,
// marble registry, and hooks. When nodes is nil, falls back to the legacy
// passthrough node registry for backward compatibility.
func BuildRunnerWith(th Thresholds, nodes framework.NodeRegistry, hooks ...framework.HookRegistry) (*framework.Runner, error) {
	def, err := AsteriskPipelineDef(th)
	if err != nil {
		return nil, err
	}
	if nodes == nil {
		nodes = buildNodeRegistry()
	}
	reg := framework.GraphRegistries{Nodes: nodes}
	if len(hooks) > 0 {
		reg.Hooks = hooks[0]
	}
	return framework.NewRunnerWith(def, reg)
}
