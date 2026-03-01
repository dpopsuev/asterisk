package rca

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/dpopsuev/origami/logging"
	"github.com/dpopsuev/origami/adapters/rp"
	"asterisk/adapters/store"
	"github.com/dpopsuev/origami/knowledge"
)

// RunnerConfig holds configuration for the interactive circuit runner.
// Used by cmd_cursor and cmd_save for human-in-the-loop RCA.
type RunnerConfig struct {
	PromptDir  string
	Thresholds Thresholds
	BasePath   string
}

// DefaultRunnerConfig returns a RunnerConfig with sensible defaults.
func DefaultRunnerConfig() RunnerConfig {
	return RunnerConfig{
		PromptDir:  ".cursor/prompts",
		Thresholds: DefaultThresholds(),
	}
}

// StepResult is returned by RunStep to the CLI caller.
type StepResult struct {
	PromptPath  string
	NextStep    CircuitStep
	IsDone      bool
	Explanation string
}

// RunStep is the interactive circuit driver for cmd_cursor. It generates prompts
// for the user to paste into Cursor. The user runs `asterisk save` to ingest
// the artifact, then runs `asterisk cursor` again to advance.
func RunStep(
	st store.Store,
	caseData *store.Case,
	env *rp.Envelope,
	catalog *knowledge.KnowledgeSourceCatalog,
	cfg RunnerConfig,
) (*StepResult, error) {
	suiteID := int64(1)
	if caseData.JobID != 0 {
		job, err := st.GetJob(caseData.JobID)
		if err == nil && job != nil {
			launch, err := st.GetLaunch(job.LaunchID)
			if err == nil && launch != nil {
				pipe, err := st.GetCircuit(launch.CircuitID)
				if err == nil && pipe != nil {
					suiteID = pipe.SuiteID
				}
			}
		}
	}

	basePath := cfg.BasePath
	if basePath == "" {
		basePath = DefaultBasePath
	}
	caseDir, err := EnsureCaseDir(basePath, suiteID, caseData.ID)
	if err != nil {
		return nil, fmt.Errorf("ensure case dir: %w", err)
	}

	state, err := LoadState(caseDir)
	if err != nil {
		return nil, fmt.Errorf("load state: %w", err)
	}
	if state == nil {
		state = InitState(caseData.ID, suiteID)
	}

	if state.Status == "done" || state.CurrentStep == StepDone {
		return &StepResult{IsDone: true, Explanation: "circuit complete"}, nil
	}

	if state.CurrentStep == StepInit {
		AdvanceStep(state, StepF0Recall, "INIT", "start circuit")
		if err := SaveState(caseDir, state); err != nil {
			return nil, fmt.Errorf("save state: %w", err)
		}
	}

	for {
		artifact := loadCurrentArtifact(caseDir, state.CurrentStep)
		if artifact == nil {
			break
		}

		action, ruleID, evalErr := cfg.evaluateStep(state.CurrentStep, artifact, state)
		if evalErr != nil {
			return nil, fmt.Errorf("evaluate step %s: %w", state.CurrentStep, evalErr)
		}

		logging.New("orchestrate").Info("heuristic evaluated",
			"step", string(state.CurrentStep), "rule", ruleID, "next", string(action.NextStep), "explanation", action.Explanation)

		if state.CurrentStep == StepF3Invest && action.NextStep == StepF2Resolve {
			IncrementLoop(state, "investigate")
		}
		if state.CurrentStep == StepF5Review && action.NextStep != StepF6Report && action.NextStep != StepDone {
			IncrementLoop(state, "reassess")
		}

		if err := ApplyStoreEffects(st, caseData, state.CurrentStep, artifact); err != nil {
			logging.New("orchestrate").Warn("store side-effect error", "step", string(state.CurrentStep), "error", err)
		}

		AdvanceStep(state, action.NextStep, ruleID, action.Explanation)
		if err := SaveState(caseDir, state); err != nil {
			return nil, fmt.Errorf("save state: %w", err)
		}

		if state.CurrentStep == StepDone {
			return &StepResult{IsDone: true, Explanation: action.Explanation}, nil
		}
	}

	step := state.CurrentStep
	loopIter := 0
	if step == StepF3Invest {
		loopIter = LoopCount(state, "investigate")
	}

	params := BuildParams(st, caseData, env, catalog, step, caseDir)
	templatePath := TemplatePathForStep(cfg.PromptDir, step)
	if templatePath == "" {
		return nil, fmt.Errorf("no template for step %s", step)
	}

	prompt, err := FillTemplate(templatePath, params)
	if err != nil {
		return nil, fmt.Errorf("fill template for %s: %w", step, err)
	}

	promptPath, err := WritePrompt(caseDir, step, loopIter, prompt)
	if err != nil {
		return nil, fmt.Errorf("write prompt for %s: %w", step, err)
	}

	state.Status = "paused"
	if err := SaveState(caseDir, state); err != nil {
		return nil, fmt.Errorf("save state: %w", err)
	}

	return &StepResult{
		PromptPath:  promptPath,
		NextStep:    step,
		Explanation: fmt.Sprintf("generated prompt for %s (paste into Cursor)", step),
	}, nil
}

// SaveArtifactAndAdvance is called after the user runs `asterisk save` with the
// artifact produced by Cursor. It evaluates heuristics and advances state.
func SaveArtifactAndAdvance(
	st store.Store,
	caseData *store.Case,
	caseDir string,
	cfg RunnerConfig,
) (*StepResult, error) {
	state, err := LoadState(caseDir)
	if err != nil || state == nil {
		return nil, fmt.Errorf("load state from %s: %w", caseDir, err)
	}

	artifact := loadCurrentArtifact(caseDir, state.CurrentStep)
	if artifact == nil {
		return nil, fmt.Errorf("no artifact found for step %s in %s", state.CurrentStep, caseDir)
	}

	action, ruleID, evalErr := cfg.evaluateStep(state.CurrentStep, artifact, state)
	if evalErr != nil {
		return nil, fmt.Errorf("evaluate step %s: %w", state.CurrentStep, evalErr)
	}

	logging.New("orchestrate").Info("save: heuristic evaluated",
		"step", string(state.CurrentStep), "rule", ruleID, "next", string(action.NextStep), "explanation", action.Explanation)

	if state.CurrentStep == StepF3Invest && action.NextStep == StepF2Resolve {
		IncrementLoop(state, "investigate")
	}
	if state.CurrentStep == StepF5Review && action.NextStep != StepF6Report && action.NextStep != StepDone {
		IncrementLoop(state, "reassess")
	}

	if err := ApplyStoreEffects(st, caseData, state.CurrentStep, artifact); err != nil {
		logging.New("orchestrate").Warn("store side-effect error", "step", string(state.CurrentStep), "error", err)
	}

	AdvanceStep(state, action.NextStep, ruleID, action.Explanation)

	state.Status = "running"
	if state.CurrentStep == StepDone {
		state.Status = "done"
	}

	if err := SaveState(caseDir, state); err != nil {
		return nil, fmt.Errorf("save state: %w", err)
	}

	return &StepResult{
		NextStep:    state.CurrentStep,
		IsDone:      state.CurrentStep == StepDone,
		Explanation: action.Explanation,
	}, nil
}

func loadCurrentArtifact(caseDir string, step CircuitStep) any {
	path := filepath.Join(caseDir, ArtifactFilename(step))
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	artifact, err := parseTypedArtifact(step, data)
	if err != nil {
		return nil
	}
	return artifact
}

func (cfg RunnerConfig) evaluateStep(step CircuitStep, artifact any, state *CaseState) (*HeuristicAction, string, error) {
	runner, err := BuildRunner(cfg.Thresholds)
	if err != nil {
		return nil, "", fmt.Errorf("build runner: %w", err)
	}
	action, edgeID := EvaluateGraphEdge(runner, step, artifact, state)
	return action, edgeID, nil
}
