package rca

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	framework "github.com/dpopsuev/origami"
	"github.com/dpopsuev/origami/dispatch"
	"github.com/dpopsuev/origami/knowledge"

	"github.com/dpopsuev/origami/adapters/rp"
	"asterisk/adapters/store"
)

const calibrationPreamble = `> **CALIBRATION MODE — BLIND EVALUATION**
>
> You are participating in a calibration run. Your responses at each circuit
> step will be **scored against known ground truth** using 20 metrics including
> defect type accuracy, component identification, evidence quality, circuit
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

// LLMAdapter is an interactive adapter for real calibration.
// It generates a filled prompt, delivers it via a dispatch.Dispatcher,
// waits for the external agent to produce an artifact, and reads it back.
type LLMAdapter struct {
	st         store.Store
	promptDir  string
	catalog    *knowledge.KnowledgeSourceCatalog
	suiteID    int64
	basePath   string
	cases      map[string]*llmCaseCtx
	dispatcher dispatch.Dispatcher
}

type LLMAdapterOption func(*LLMAdapter)

func WithDispatcher(d dispatch.Dispatcher) LLMAdapterOption {
	return func(a *LLMAdapter) { a.dispatcher = d }
}

func WithBasePath(p string) LLMAdapterOption {
	return func(a *LLMAdapter) { a.basePath = p }
}

type llmCaseCtx struct {
	storeCase *store.Case
	env       *rp.Envelope
}

func NewLLMAdapter(promptDir string, opts ...LLMAdapterOption) *LLMAdapter {
	a := &LLMAdapter{
		promptDir:  promptDir,
		basePath:   DefaultBasePath,
		cases:      make(map[string]*llmCaseCtx),
		dispatcher: dispatch.NewStdinDispatcherWithTemplate(AsteriskStdinTemplate()),
	}
	for _, opt := range opts {
		opt(a)
	}
	return a
}

func (a *LLMAdapter) Name() string { return "llm" }

const identityProbePrompt = `You are being asked to self-identify. Respond with ONLY a JSON object.
No markdown, no code fences, no explanation — just raw JSON.

IMPORTANT: Report your FOUNDATION model, not the wrapper or IDE hosting you.
If you are Claude running inside Cursor, model_name is "claude-sonnet-4-20250514", NOT "composer".
If you are GPT-4o running inside Copilot, model_name is "gpt-4o", NOT "copilot".

{"model_name": "<your foundation model name, e.g. claude-sonnet-4-20250514>",
 "provider": "<company that TRAINED you, e.g. Anthropic, OpenAI, Google>",
 "version": "<your version or checkpoint, e.g. 20250514, 4.0>",
 "wrapper": "<hosting environment if any, e.g. Cursor, Azure, Copilot, or null if direct>"}`

func (a *LLMAdapter) Identify() (framework.ModelIdentity, error) {
	tmpDir := os.TempDir()
	promptFile := filepath.Join(tmpDir, "asterisk-identity-probe.md")
	artifactFile := filepath.Join(tmpDir, "asterisk-identity-probe.json")

	if err := os.WriteFile(promptFile, []byte(identityProbePrompt), 0644); err != nil {
		return framework.ModelIdentity{}, fmt.Errorf("llm: write identity probe: %w", err)
	}
	defer os.Remove(promptFile)
	defer os.Remove(artifactFile)

	data, err := a.dispatcher.Dispatch(dispatch.DispatchContext{
		CaseID: "_probe", Step: "_identify",
		PromptPath: promptFile, ArtifactPath: artifactFile,
	})
	if err != nil {
		return framework.ModelIdentity{}, fmt.Errorf("llm: identity probe dispatch: %w", err)
	}
	return ParseModelIdentity(data)
}

// ParseModelIdentity extracts a ModelIdentity from raw JSON bytes.
func ParseModelIdentity(data []byte) (framework.ModelIdentity, error) {
	raw := strings.TrimSpace(string(data))
	var mi framework.ModelIdentity
	if err := json.Unmarshal([]byte(raw), &mi); err != nil {
		return framework.ModelIdentity{}, fmt.Errorf("identity probe: invalid JSON: %w (raw: %q)", err, truncate(raw, 120))
	}
	mi.Raw = raw
	if mi.ModelName == "" {
		return framework.ModelIdentity{}, fmt.Errorf("identity probe: model_name is empty (raw: %q)", truncate(raw, 120))
	}
	if mi.Provider == "" {
		return framework.ModelIdentity{}, fmt.Errorf("identity probe: provider is empty (raw: %q)", truncate(raw, 120))
	}
	if framework.IsWrapperName(mi.ModelName) {
		return mi, fmt.Errorf("identity probe: model_name %q is a wrapper, not a foundation model (raw: %q)", mi.ModelName, truncate(raw, 120))
	}
	return mi, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

func (a *LLMAdapter) SetStore(st store.Store)                           { a.st = st }
func (a *LLMAdapter) SetSuiteID(id int64)                              { a.suiteID = id }
func (a *LLMAdapter) SetCatalog(cat *knowledge.KnowledgeSourceCatalog) { a.catalog = cat }

func (a *LLMAdapter) RegisterCase(gtCaseID string, storeCase *store.Case) {
	a.cases[gtCaseID] = &llmCaseCtx{
		storeCase: storeCase,
		env: &rp.Envelope{
			Name:        storeCase.Name,
			FailureList: []rp.FailureItem{{Name: storeCase.Name}},
		},
	}
}

func (a *LLMAdapter) SendPrompt(caseID string, step string, _ string) (json.RawMessage, error) {
	ps := CircuitStep(step)
	ctx := a.cases[caseID]
	if ctx == nil {
		return nil, fmt.Errorf("llm: unknown case %q", caseID)
	}

	updated, err := a.st.GetCaseV2(ctx.storeCase.ID)
	if err == nil && updated != nil {
		ctx.storeCase = updated
	}

	caseDir, err := EnsureCaseDir(a.basePath, a.suiteID, ctx.storeCase.ID)
	if err != nil {
		return nil, fmt.Errorf("llm: ensure case dir: %w", err)
	}

	params := BuildParams(a.st, ctx.storeCase, ctx.env, a.catalog, ps, caseDir)
	templatePath := TemplatePathForStep(a.promptDir, ps)
	if templatePath == "" {
		return nil, fmt.Errorf("llm: no template for step %s", step)
	}

	prompt, err := FillTemplate(templatePath, params)
	if err != nil {
		return nil, fmt.Errorf("llm: fill template for %s: %w", step, err)
	}
	prompt = calibrationPreamble + prompt

	promptFile := filepath.Join(caseDir, fmt.Sprintf("prompt-%s.md", ps.Family()))
	if err := os.WriteFile(promptFile, []byte(prompt), 0644); err != nil {
		return nil, fmt.Errorf("llm: write prompt: %w", err)
	}

	artifactFile := filepath.Join(caseDir, ArtifactFilename(ps))

	data, err := a.dispatcher.Dispatch(dispatch.DispatchContext{
		CaseID: caseID, Step: step,
		PromptPath: promptFile, ArtifactPath: artifactFile,
	})
	if err != nil {
		return nil, fmt.Errorf("llm: dispatch %s/%s: %w", caseID, step, err)
	}

	if f := dispatch.UnwrapFinalizer(a.dispatcher); f != nil {
		f.MarkDone(artifactFile)
	}

	return json.RawMessage(data), nil
}

// AsteriskStdinTemplate returns the Asterisk-specific instructions for
// interactive stdin dispatch (Cursor agent workflow).
func AsteriskStdinTemplate() dispatch.StdinTemplate {
	return dispatch.StdinTemplate{
		Instructions: []string{
			"1. Open the prompt file and paste it into Cursor",
			"2. Save Cursor's JSON response to the artifact path above",
			"3. Press Enter to continue",
		},
	}
}
