package calibrate

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"asterisk/internal/orchestrate"
	"asterisk/internal/preinvest"
	"asterisk/internal/store"
	"asterisk/internal/workspace"
)

// calibrationPreamble is prepended to every prompt during cursor-based calibration.
// It informs the AI about the calibration context and the integrity rules.
const calibrationPreamble = `> **CALIBRATION MODE — BLIND EVALUATION**
>
> You are participating in a calibration run. Your responses at each pipeline
> step will be **scored against known ground truth** using 20 metrics including
> defect type accuracy, component identification, evidence quality, pipeline
> path efficiency, and semantic relevance.
>
> **Rules:**
> 1. Respond ONLY based on the information provided in this prompt.
> 2. Do NOT read scenario definition files, ground truth files, expected
>    results, or any calibration/test code in the repository. This includes
>    any file under ` + "`internal/calibrate/scenarios/`" + `, any ` + "`*_test.go`" + ` file,
>    and the ` + "`.cursor/contracts/`" + ` directory.
> 3. Do NOT inspect prior calibration artifacts or reports.
> 4. Produce your best independent analysis based solely on the failure data,
>    error messages, logs, and code context given below.
> 5. Your final score depends on the quality of your reasoning, not on
>    matching a specific expected answer.
>
> The calibration report will be shown after all cases complete.

---

`

// CursorAdapter is an interactive adapter for real calibration.
// Instead of returning canned answers, it generates a filled prompt from the
// template, delivers it via a Dispatcher, waits for the external agent to
// produce an artifact, and reads it back. The calibrate runner then scores
// the AI's real output against ground truth.
type CursorAdapter struct {
	st         store.Store
	promptDir  string
	ws         *workspace.Workspace
	suiteID    int64
	cases      map[string]*cursorCaseCtx
	dispatcher Dispatcher
}

// CursorAdapterOption configures the CursorAdapter.
type CursorAdapterOption func(*CursorAdapter)

// WithDispatcher sets the transport dispatcher for prompt delivery and
// artifact collection. Defaults to StdinDispatcher if not set.
func WithDispatcher(d Dispatcher) CursorAdapterOption {
	return func(a *CursorAdapter) { a.dispatcher = d }
}

type cursorCaseCtx struct {
	storeCase *store.Case
	env       *preinvest.Envelope
}

// NewCursorAdapter creates an interactive adapter.
// st and ws will be overwritten by the runner via SetStore; pass nil initially.
// Default dispatcher is StdinDispatcher (interactive terminal).
func NewCursorAdapter(promptDir string, opts ...CursorAdapterOption) *CursorAdapter {
	a := &CursorAdapter{
		promptDir:  promptDir,
		cases:      make(map[string]*cursorCaseCtx),
		dispatcher: NewStdinDispatcher(),
	}
	for _, opt := range opts {
		opt(a)
	}
	return a
}

// Name returns the adapter identifier.
func (a *CursorAdapter) Name() string { return "cursor" }

// SetStore is called by the calibrate runner to inject the per-run MemStore.
func (a *CursorAdapter) SetStore(st store.Store) { a.st = st }

// SetSuiteID is called by the calibrate runner after creating the suite.
func (a *CursorAdapter) SetSuiteID(id int64) { a.suiteID = id }

// SetWorkspace sets the workspace for prompt param building.
func (a *CursorAdapter) SetWorkspace(ws *workspace.Workspace) { a.ws = ws }

// RegisterCase registers a store case mapped to a ground truth case ID,
// so the adapter can look it up when SendPrompt is called.
func (a *CursorAdapter) RegisterCase(gtCaseID string, storeCase *store.Case) {
	a.cases[gtCaseID] = &cursorCaseCtx{
		storeCase: storeCase,
		env: &preinvest.Envelope{
			Name: storeCase.Name,
			FailureList: []preinvest.FailureItem{{
				Name: storeCase.Name,
			}},
		},
	}
}

// SendPrompt generates a filled prompt, presents it to the user, waits for the
// artifact to be saved, and returns the raw JSON.
func (a *CursorAdapter) SendPrompt(caseID string, step orchestrate.PipelineStep, _ string) (json.RawMessage, error) {
	ctx := a.cases[caseID]
	if ctx == nil {
		return nil, fmt.Errorf("cursor: unknown case %q", caseID)
	}

	// Refresh case from store to get latest symptom/RCA links
	updated, err := a.st.GetCaseV2(ctx.storeCase.ID)
	if err == nil && updated != nil {
		ctx.storeCase = updated
	}

	// Build case dir
	caseDir, err := orchestrate.EnsureCaseDir(a.suiteID, ctx.storeCase.ID)
	if err != nil {
		return nil, fmt.Errorf("cursor: ensure case dir: %w", err)
	}

	// Build prompt params and fill template
	params := orchestrate.BuildParams(a.st, ctx.storeCase, ctx.env, a.ws, step, caseDir)
	templatePath := orchestrate.TemplatePathForStep(a.promptDir, step)
	if templatePath == "" {
		return nil, fmt.Errorf("cursor: no template for step %s", step)
	}

	prompt, err := orchestrate.FillTemplate(templatePath, params)
	if err != nil {
		return nil, fmt.Errorf("cursor: fill template for %s: %w", step, err)
	}

	// Prepend calibration notice so the AI knows the rules
	prompt = calibrationPreamble + prompt

	// Write prompt file
	promptFile := filepath.Join(caseDir, fmt.Sprintf("prompt-%s.md", step.Family()))
	if err := os.WriteFile(promptFile, []byte(prompt), 0644); err != nil {
		return nil, fmt.Errorf("cursor: write prompt: %w", err)
	}

	// Expected artifact path
	artifactFile := filepath.Join(caseDir, orchestrate.ArtifactFilename(step))

	// Dispatch: deliver prompt and collect artifact via the configured dispatcher
	data, err := a.dispatcher.Dispatch(DispatchContext{
		CaseID:       caseID,
		Step:         string(step),
		PromptPath:   promptFile,
		ArtifactPath: artifactFile,
	})
	if err != nil {
		return nil, fmt.Errorf("cursor: dispatch %s/%s: %w", caseID, step, err)
	}

	// Mark done if the dispatcher supports it (FileDispatcher).
	// Unwrap TokenTrackingDispatcher if present.
	disp := a.dispatcher
	if td, ok := disp.(*TokenTrackingDispatcher); ok {
		disp = td.Inner()
	}
	if fd, ok := disp.(*FileDispatcher); ok {
		fd.MarkDone(artifactFile)
	}

	return json.RawMessage(data), nil
}

// ScenarioToWorkspace converts a calibrate.WorkspaceConfig to a workspace.Workspace
// so the CursorAdapter can use it for BuildParams.
func ScenarioToWorkspace(wc WorkspaceConfig) *workspace.Workspace {
	ws := &workspace.Workspace{}
	for _, r := range wc.Repos {
		ws.Repos = append(ws.Repos, workspace.Repo{
			Name:    r.Name,
			Path:    r.Path,
			Purpose: r.Purpose,
			Branch:  r.Branch,
		})
	}
	return ws
}
