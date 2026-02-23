package orchestrate

import framework "github.com/dpopsuev/origami"

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

// AsteriskPipelineDef returns the F0-F6 RCA pipeline definition with all
// heuristic edges expressed as when: expressions. Expression edges are evaluated
// by the framework's expr-lang engine at walk time against {output, state, config}.
func AsteriskPipelineDef(th Thresholds) *framework.PipelineDef {
	return &framework.PipelineDef{
		Pipeline:    "asterisk-rca",
		Description: "Root-cause analysis pipeline (F0 Recall through F6 Report)",
		Vars:        ThresholdsToVars(th),
		Nodes: []framework.NodeDef{
			{Name: "recall", Family: "recall", After: []string{"store.recall"}},
			{Name: "triage", Family: "triage", After: []string{"store.triage"}},
			{Name: "resolve", Family: "resolve"},
			{Name: "investigate", Family: "investigate", After: []string{"store.investigate"}},
			{Name: "correlate", Family: "correlate", After: []string{"store.correlate"}},
			{Name: "review", Family: "review", After: []string{"store.review"}},
			{Name: "report", Family: "report"},
		},
		Edges: []framework.EdgeDef{
			// F0 Recall
			{ID: "H1", Name: "recall-hit", From: "recall", To: "review", Shortcut: true,
				When: `output.match == true && output.confidence >= config.recall_hit`},
			{ID: "H3", Name: "recall-uncertain", From: "recall", To: "triage",
				When: `output.match == true && output.confidence >= config.recall_uncertain && output.confidence < config.recall_hit`},
			{ID: "H2", Name: "recall-miss", From: "recall", To: "triage",
				When: `output.match != true`},

			// F1 Triage
			{ID: "H4", Name: "triage-skip-infra", From: "triage", To: "review", Shortcut: true,
				When: `output.symptom_category == "infra"`},
			{ID: "H5", Name: "triage-skip-flake", From: "triage", To: "review", Shortcut: true,
				When: `output.symptom_category == "flake"`},
			{ID: "H18", Name: "triage-skip-investigation", From: "triage", To: "review", Shortcut: true,
				When: `output.skip_investigation == true`},
			{ID: "H7", Name: "triage-single-repo", From: "triage", To: "investigate", Shortcut: true,
				When: `output.skip_investigation != true && len(output.candidate_repos) == 1`},
			{ID: "H6", Name: "triage-investigate", From: "triage", To: "resolve",
				When: `output.skip_investigation != true`},

			// F2 Resolve
			{ID: "H8", Name: "resolve-multi", From: "resolve", To: "investigate",
				When: `true`},

			// F3 Investigate
			{ID: "H9", Name: "investigate-converged", From: "investigate", To: "correlate",
				When: `output.convergence_score >= config.convergence_sufficient`},
			{ID: "H10a", Name: "investigate-no-evidence-skip", From: "investigate", To: "correlate",
				When: `output.convergence_score < config.convergence_sufficient && len(output.evidence_refs) == 0`},
			{ID: "H10", Name: "investigate-low", From: "investigate", To: "resolve", Loop: true,
				When: `output.convergence_score < config.convergence_sufficient && len(output.evidence_refs) > 0 && state.loops.investigate < config.max_investigate_loops`},
			{ID: "H11", Name: "investigate-exhausted", From: "investigate", To: "review", Shortcut: true,
				When: `output.convergence_score < config.convergence_sufficient && state.loops.investigate >= config.max_investigate_loops`},

			// F4 Correlate
			{ID: "H15", Name: "correlate-dup", From: "correlate", To: DoneNodeName,
				When: `output.is_duplicate == true && output.confidence >= config.correlate_dup`},
			{ID: "H15b", Name: "correlate-proceed", From: "correlate", To: "review",
				When: `true`},

			// F5 Review
			{ID: "H12", Name: "review-approve", From: "review", To: "report",
				When: `output.decision == "approve"`},
			{ID: "H13", Name: "review-reassess", From: "review", To: "resolve", Loop: true,
				When: `output.decision == "reassess"`},
			{ID: "H14", Name: "review-overturn", From: "review", To: "report",
				When: `output.decision == "overturn"`},

			// F6 Report
			{ID: "FALLBACK", Name: "report-done", From: "report", To: DoneNodeName,
				When: `true`},
		},
		Start: "recall",
		Done:  DoneNodeName,
	}
}

// BuildRunner constructs a framework.Runner from the Asterisk pipeline
// definition with the given thresholds. Expression edges are compiled at
// build time and evaluate against the config derived from thresholds.
func BuildRunner(th Thresholds, hooks ...framework.HookRegistry) (*framework.Runner, error) {
	def := AsteriskPipelineDef(th)
	nodeReg := buildNodeRegistry()
	reg := framework.GraphRegistries{Nodes: nodeReg}
	if len(hooks) > 0 {
		reg.Hooks = hooks[0]
	}
	return framework.NewRunnerWith(def, reg)
}
