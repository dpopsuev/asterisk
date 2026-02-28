package rca

import (
	_ "embed"
	"fmt"

	framework "github.com/dpopsuev/origami"
	"github.com/dpopsuev/origami/logging"
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
func BuildRunner(th Thresholds, hooks ...framework.HookRegistry) (*framework.Runner, error) {
	return BuildRunnerWith(th, nil, hooks...)
}

// BuildRunnerWith constructs a framework.Runner using the provided node registry
// and hooks. When nodes is nil, defaults to the real processing NodeRegistry.
func BuildRunnerWith(th Thresholds, nodes framework.NodeRegistry, hooks ...framework.HookRegistry) (*framework.Runner, error) {
	def, err := AsteriskPipelineDef(th)
	if err != nil {
		return nil, err
	}
	if nodes == nil {
		nodes = NodeRegistry()
	}
	reg := framework.GraphRegistries{Nodes: nodes}
	if len(hooks) > 0 {
		reg.Hooks = hooks[0]
	}
	return framework.NewRunnerWith(def, reg)
}

// EvaluateGraphEdge evaluates the YAML-defined expression edges for the given
// pipeline step. The runner should be built once (via BuildRunner) and reused
// across evaluations. This is the single routing path — all callers use YAML
// edges, no Go closures.
func EvaluateGraphEdge(runner *framework.Runner, step PipelineStep, artifact any, state *CaseState) (*HeuristicAction, string) {
	nodeName := StepToNodeName(step)
	edges := runner.Graph.EdgesFrom(nodeName)
	wrappedArtifact := WrapArtifact(step, artifact)
	wrappedState := caseStateToWalkerState(state)

	for _, e := range edges {
		t := e.Evaluate(wrappedArtifact, wrappedState)
		if t != nil {
			return &HeuristicAction{
				NextStep:         NodeNameToStep(t.NextNode),
				ContextAdditions: t.ContextAdditions,
				Explanation:      t.Explanation,
			}, e.ID()
		}
	}

	logging.New("pipeline").Debug("no edge matched, using fallback",
		"step", string(step), "node", nodeName)
	return defaultFallback(step), "FALLBACK"
}

// defaultFallback returns the next step in the happy-path pipeline when no
// edge matches. This should rarely fire — the YAML edges are exhaustive.
func defaultFallback(current PipelineStep) *HeuristicAction {
	next := map[PipelineStep]PipelineStep{
		StepInit:        StepF0Recall,
		StepF0Recall:    StepF1Triage,
		StepF1Triage:    StepF2Resolve,
		StepF2Resolve:   StepF3Invest,
		StepF3Invest:    StepF4Correlate,
		StepF4Correlate: StepF5Review,
		StepF5Review:    StepF6Report,
		StepF6Report:    StepDone,
	}
	if n, ok := next[current]; ok {
		return &HeuristicAction{
			NextStep:    n,
			Explanation: fmt.Sprintf("fallback: default pipeline progression from %s to %s", current, n),
		}
	}
	return &HeuristicAction{NextStep: StepDone, Explanation: "fallback: end of pipeline"}
}
