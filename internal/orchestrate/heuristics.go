package orchestrate

import "fmt"

// Thresholds holds configurable threshold values for heuristic evaluation.
type Thresholds struct {
	RecallHit             float64 // when to short-circuit on prior RCA (default 0.80)
	RecallUncertain       float64 // below this = definite miss (default 0.40)
	ConvergenceSufficient float64 // when to stop investigating (default 0.70)
	MaxInvestigateLoops   int     // cap on F3→F2→F3 iterations (default 2)
	CorrelateDup          float64 // when to auto-link cases to same RCA (default 0.80)
}

// DefaultThresholds returns conservative default thresholds.
func DefaultThresholds() Thresholds {
	return Thresholds{
		RecallHit:             0.80,
		RecallUncertain:       0.40,
		ConvergenceSufficient: 0.70,
		MaxInvestigateLoops:   2,
		CorrelateDup:          0.80,
	}
}

// DefaultHeuristics returns the 17 heuristic rules from the prompt-families contract §4.1.
func DefaultHeuristics(th Thresholds) []HeuristicRule {
	return []HeuristicRule{
		// --- F0 Recall stage ---
		{
			ID: "H1", Name: "recall-hit", Stage: StepF0Recall,
			SignalField: "confidence",
			Evaluate: func(artifact any, state *CaseState) *HeuristicAction {
				r, ok := artifact.(*RecallResult)
				if !ok || r == nil || !r.Match || r.Confidence < th.RecallHit {
					return nil
				}
				return &HeuristicAction{
					NextStep:    StepF5Review,
					Explanation: fmt.Sprintf("recall match confidence %.2f >= %.2f; skip to review with prior RCA #%d", r.Confidence, th.RecallHit, r.PriorRCAID),
				}
			},
		},
		{
			ID: "H3", Name: "recall-uncertain", Stage: StepF0Recall,
			SignalField: "confidence",
			Evaluate: func(artifact any, state *CaseState) *HeuristicAction {
				r, ok := artifact.(*RecallResult)
				if !ok || r == nil || !r.Match || r.Confidence >= th.RecallHit || r.Confidence < th.RecallUncertain {
					return nil
				}
				return &HeuristicAction{
					NextStep:    StepF1Triage,
					Explanation: fmt.Sprintf("recall uncertain: confidence %.2f in [%.2f, %.2f); proceed to triage with recall candidate as context", r.Confidence, th.RecallUncertain, th.RecallHit),
					ContextAdditions: map[string]any{"recall_candidate": r},
				}
			},
		},
		{
			ID: "H2", Name: "recall-miss", Stage: StepF0Recall,
			SignalField: "match",
			Evaluate: func(artifact any, state *CaseState) *HeuristicAction {
				r, ok := artifact.(*RecallResult)
				if !ok || r == nil {
					// No recall result = miss
					return &HeuristicAction{
						NextStep:    StepF1Triage,
						Explanation: "no recall result; proceed to triage",
					}
				}
				if r.Match {
					return nil // matched, let H1/H3 handle
				}
				return &HeuristicAction{
					NextStep:    StepF1Triage,
					Explanation: "recall miss; proceed to triage",
				}
			},
		},

		// --- F1 Triage stage ---
		{
			ID: "H4", Name: "triage-skip-infra", Stage: StepF1Triage,
			SignalField: "symptom_category",
			Evaluate: func(artifact any, state *CaseState) *HeuristicAction {
				r, ok := artifact.(*TriageResult)
				if !ok || r == nil || r.SymptomCategory != "infra" {
					return nil
				}
				return &HeuristicAction{
					NextStep:    StepF5Review,
					Explanation: "infra symptom: skip repo investigation, go to review",
				}
			},
		},
		{
			ID: "H5", Name: "triage-skip-flake", Stage: StepF1Triage,
			SignalField: "symptom_category",
			Evaluate: func(artifact any, state *CaseState) *HeuristicAction {
				r, ok := artifact.(*TriageResult)
				if !ok || r == nil || r.SymptomCategory != "flake" {
					return nil
				}
				return &HeuristicAction{
					NextStep:    StepF5Review,
					Explanation: "flaky test: skip repo investigation, confirm with human",
				}
			},
		},
		{
			ID: "H17", Name: "triage-clock-skew", Stage: StepF1Triage,
			SignalField: "clock_skew_suspected",
			Evaluate: func(artifact any, state *CaseState) *HeuristicAction {
				r, ok := artifact.(*TriageResult)
				if !ok || r == nil || !r.ClockSkewSuspected {
					return nil
				}
				// Clock skew is an advisory; don't change routing, just add context.
				return nil // handled as context addition by the runner, not a routing change
			},
		},
		{
			ID: "H7", Name: "triage-single-repo", Stage: StepF1Triage,
			SignalField: "candidate_repos",
			Evaluate: func(artifact any, state *CaseState) *HeuristicAction {
				r, ok := artifact.(*TriageResult)
				if !ok || r == nil || r.SkipInvestigation || len(r.CandidateRepos) != 1 {
					return nil
				}
				return &HeuristicAction{
					NextStep:    StepF3Invest,
					Explanation: fmt.Sprintf("single candidate repo %q; skip F2, go directly to investigate", r.CandidateRepos[0]),
					ContextAdditions: map[string]any{
						"selected_repos": []RepoSelection{{Name: r.CandidateRepos[0], Reason: "only candidate from triage"}},
					},
				}
			},
		},
		{
			ID: "H6", Name: "triage-investigate", Stage: StepF1Triage,
			SignalField: "skip_investigation",
			Evaluate: func(artifact any, state *CaseState) *HeuristicAction {
				r, ok := artifact.(*TriageResult)
				if !ok || r == nil || r.SkipInvestigation {
					return nil
				}
				return &HeuristicAction{
					NextStep:    StepF2Resolve,
					Explanation: "triage recommends investigation; proceed to repo resolution",
				}
			},
		},

		// --- F2 Resolve stage ---
		{
			ID: "H8", Name: "resolve-multi", Stage: StepF2Resolve,
			SignalField: "selected_repos",
			Evaluate: func(artifact any, state *CaseState) *HeuristicAction {
				r, ok := artifact.(*ResolveResult)
				if !ok || r == nil {
					return nil
				}
				// Always proceed to F3 after resolve (multi-repo handled by runner)
				return &HeuristicAction{
					NextStep:    StepF3Invest,
					Explanation: fmt.Sprintf("resolved %d repo(s); proceed to investigate", len(r.SelectedRepos)),
				}
			},
		},

		// --- F3 Investigate stage ---
		{
			ID: "H9", Name: "investigate-converged", Stage: StepF3Invest,
			SignalField: "convergence_score",
			Evaluate: func(artifact any, state *CaseState) *HeuristicAction {
				r, ok := artifact.(*InvestigateArtifact)
				if !ok || r == nil || r.ConvergenceScore < th.ConvergenceSufficient {
					return nil
				}
				return &HeuristicAction{
					NextStep:    StepF4Correlate,
					Explanation: fmt.Sprintf("convergence %.2f >= %.2f; proceed to correlate", r.ConvergenceScore, th.ConvergenceSufficient),
				}
			},
		},
		{
			ID: "H10", Name: "investigate-low", Stage: StepF3Invest,
			SignalField: "convergence_score",
			Evaluate: func(artifact any, state *CaseState) *HeuristicAction {
				r, ok := artifact.(*InvestigateArtifact)
				if !ok || r == nil || r.ConvergenceScore >= th.ConvergenceSufficient {
					return nil
				}
				if IsLoopExhausted(state, "investigate", th.MaxInvestigateLoops) {
					return nil // let H11 handle exhaustion
				}
				return &HeuristicAction{
					NextStep:    StepF2Resolve,
					Explanation: fmt.Sprintf("convergence %.2f < %.2f and loop %d < %d; retry with broader scope", r.ConvergenceScore, th.ConvergenceSufficient, LoopCount(state, "investigate"), th.MaxInvestigateLoops),
				}
			},
		},
		{
			ID: "H11", Name: "investigate-exhausted", Stage: StepF3Invest,
			SignalField: "convergence_score",
			Evaluate: func(artifact any, state *CaseState) *HeuristicAction {
				r, ok := artifact.(*InvestigateArtifact)
				if !ok || r == nil || r.ConvergenceScore >= th.ConvergenceSufficient {
					return nil
				}
				if !IsLoopExhausted(state, "investigate", th.MaxInvestigateLoops) {
					return nil
				}
				return &HeuristicAction{
					NextStep:    StepF5Review,
					Explanation: fmt.Sprintf("convergence %.2f < %.2f and loops exhausted (%d); proceed to review with ti001", r.ConvergenceScore, th.ConvergenceSufficient, LoopCount(state, "investigate")),
				}
			},
		},

		// --- F4 Correlate stage ---
		{
			ID: "H15", Name: "correlate-dup", Stage: StepF4Correlate,
			SignalField: "is_duplicate",
			Evaluate: func(artifact any, state *CaseState) *HeuristicAction {
				r, ok := artifact.(*CorrelateResult)
				if !ok || r == nil || !r.IsDuplicate || r.Confidence < th.CorrelateDup {
					return nil
				}
				return &HeuristicAction{
					NextStep:    StepDone,
					Explanation: fmt.Sprintf("duplicate with confidence %.2f >= %.2f; link to RCA #%d, skip report", r.Confidence, th.CorrelateDup, r.LinkedRCAID),
				}
			},
		},
		{
			ID: "H15b", Name: "correlate-proceed", Stage: StepF4Correlate,
			SignalField: "is_duplicate",
			Evaluate: func(artifact any, state *CaseState) *HeuristicAction {
				// Fallback: if not a high-confidence dup, proceed to review
				return &HeuristicAction{
					NextStep:    StepF5Review,
					Explanation: "correlation complete; proceed to review",
				}
			},
		},

		// --- F5 Review stage ---
		{
			ID: "H12", Name: "review-approve", Stage: StepF5Review,
			SignalField: "decision",
			Evaluate: func(artifact any, state *CaseState) *HeuristicAction {
				r, ok := artifact.(*ReviewDecision)
				if !ok || r == nil || r.Decision != "approve" {
					return nil
				}
				return &HeuristicAction{
					NextStep:    StepF6Report,
					Explanation: "human approved; proceed to report",
				}
			},
		},
		{
			ID: "H13", Name: "review-reassess", Stage: StepF5Review,
			SignalField: "decision",
			Evaluate: func(artifact any, state *CaseState) *HeuristicAction {
				r, ok := artifact.(*ReviewDecision)
				if !ok || r == nil || r.Decision != "reassess" {
					return nil
				}
				target := r.LoopTarget
				if target == "" {
					target = StepF2Resolve // default reassess target
				}
				return &HeuristicAction{
					NextStep:    target,
					Explanation: fmt.Sprintf("human requested reassessment; loop back to %s", target),
				}
			},
		},
		{
			ID: "H14", Name: "review-overturn", Stage: StepF5Review,
			SignalField: "decision",
			Evaluate: func(artifact any, state *CaseState) *HeuristicAction {
				r, ok := artifact.(*ReviewDecision)
				if !ok || r == nil || r.Decision != "overturn" {
					return nil
				}
				return &HeuristicAction{
					NextStep:    StepF6Report,
					Explanation: "human overturned with correct answer; proceed to report with override",
					ContextAdditions: map[string]any{"human_override": r.HumanOverride},
				}
			},
		},
	}
}

// EvaluateHeuristics runs the heuristic rules for the given stage against
// the current artifact, returning the first matching action.
// Rules are evaluated in order (most specific first per §4.2).
func EvaluateHeuristics(
	rules []HeuristicRule,
	stage PipelineStep,
	artifact any,
	state *CaseState,
) (action *HeuristicAction, matchedRule string) {
	for _, rule := range rules {
		if rule.Stage != stage {
			continue
		}
		if result := rule.Evaluate(artifact, state); result != nil {
			return result, rule.ID
		}
	}
	// Fallback: advance to next step in default pipeline
	return defaultFallback(stage), "FALLBACK"
}

// defaultFallback returns the next step in the happy path pipeline.
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
