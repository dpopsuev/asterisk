package rca

import (
	_ "embed"
	"fmt"

	framework "github.com/dpopsuev/origami"
)

//go:embed circuit_rca.yaml
var circuitRCAYAML []byte

// ThresholdsToVars converts typed Thresholds to a map for circuit vars / expression config.
func ThresholdsToVars(th Thresholds) map[string]any {
	return map[string]any{
		"recall_hit":             th.RecallHit,
		"recall_uncertain":       th.RecallUncertain,
		"convergence_sufficient": th.ConvergenceSufficient,
		"max_investigate_loops":  th.MaxInvestigateLoops,
		"correlate_dup":          th.CorrelateDup,
	}
}

// AsteriskCircuitDef loads the RCA circuit from the embedded YAML and
// overrides vars with the provided thresholds.
func AsteriskCircuitDef(th Thresholds) (*framework.CircuitDef, error) {
	def, err := framework.LoadCircuit(circuitRCAYAML)
	if err != nil {
		return nil, fmt.Errorf("load embedded circuit YAML: %w", err)
	}
	def.Vars = ThresholdsToVars(th)
	return def, nil
}

// BuildRunner constructs a framework.Runner from the Asterisk circuit
// definition with the given thresholds and adapters. The adapters provide
// transformers, hooks, and extractors to the graph build.
func BuildRunner(th Thresholds, adapters ...*framework.Adapter) (*framework.Runner, error) {
	def, err := AsteriskCircuitDef(th)
	if err != nil {
		return nil, err
	}
	reg := framework.GraphRegistries{}
	if len(adapters) > 0 {
		reg, err = framework.MergeAdapters(reg, adapters...)
		if err != nil {
			return nil, fmt.Errorf("merge adapters: %w", err)
		}
	}
	return framework.NewRunnerWith(def, reg)
}

