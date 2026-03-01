# Contract — Prompt templates and params

**Status:** complete  
**Goal:** Add prompt templates under `.cursor/prompts/` (dev) and document template parameters so the CLI and Cursor handoff can inject launch_id, case_id, workspace_path, etc.; prepare for shipment to CLI prompt dir.

## Contract rules

- Global rules only. Prompts are file-based; during development live in `.cursor/prompts/`; when stable, copy to CLI prompt directory. See `goals/poc.mdc`, `notes/pre-dev-decisions.mdc`.
- Template parameters must be listed in FSC (`docs/prompts.mdc` or equivalent) so templates and CLI stay in sync.

## Context

- **Pre-dev:** Template params at least: launch_id (or run_id), case_id (or failure id), workspace_path, failed_test_name, artifact_path, job_id, circuit_id (optional). Prompt loading: CLI loads from prompt dir (e.g. `./.asterisk/prompts/` or `~/.config/asterisk/prompts/` or flag); dev uses `.cursor/prompts/`. `notes/pre-dev-decisions.mdc`.
- **PoC:** Analyze uses one external context source; prompt instructs model to emit RCA and convergence score in structured form (artifact). `goals/poc.mdc`.
- **Current:** No prompt templates in repo; no docs/prompts.mdc listing params.

## Execution strategy

1. Create `.cursor/prompts/` with at least one template (e.g. assessment or RCA template) that uses placeholders for the documented params.
2. Document template parameters in `docs/prompts.mdc`: name, description, example; link from CLI and cursor-handoff contract.
3. Define format for placeholders (e.g. `{{.LaunchID}}` Go template, or simple {{param}} substitution). Prefer Go text/template for flexibility.
4. CLI (or handoff) will load template and substitute params; document how so implementers know the contract.
5. Validate: template loads and params substitute; docs accurate.
6. Blue if needed.

## Tasks

- [x] **Create docs/prompts.mdc** — List all template parameters: launch_id, run_id, case_id, failure_id, workspace_path, failed_test_name, artifact_path, job_id, circuit_id; description and example each. Note: convergence score and RCA output format (so model knows what to emit).
- [x] **Add .cursor/prompts/** — At least one template file (e.g. `rca.md` or `assessment.md`) with placeholders for the params above. Instruct model to produce RCA summary and convergence score (and defect type) for artifact.
- [x] **Placeholder format** — Choose and document: e.g. Go text/template `{{.LaunchID}}` or `{{ launch_id }}`. Document in prompts.mdc.
- [x] **Loader contract** — Document how CLI (or handoff) loads template: path resolution (prompts dir), file name, substitution. No Cursor API in PoC; output is text or file for user to paste/open.
- [x] **Validate** — Template file exists; params doc matches placeholders; loader (or manual sub) produces valid prompt for one example.
- [x] **Tune (blue)** — Wording, structure; no behavior change.
- [x] **Validate** — Still valid.

## Acceptance criteria

- **Given** a template in `.cursor/prompts/` and a set of parameter values,
- **When** the template is loaded and parameters are substituted,
- **Then** the result is a valid prompt that includes launch/case/workspace context and instructs the model to produce RCA and convergence score for the artifact.
- **And** `docs/prompts.mdc` (or equivalent) lists all parameters so CLI and templates stay in sync.

## Notes

(Running log, newest first. YYYY-MM-DD HH:MM — decision or finding.)

- 2026-02-17 — Completed. Expanded docs/prompts.mdc with template parameters table (LaunchID, RunID, CaseID, FailureID, WorkspacePath, FailedTestName, ArtifactPath, JobID, CircuitID), placeholder format (Go text/template {{.FieldName}}), output format (artifact), and loader contract. Added .cursor/prompts/rca.md with placeholders and instructions for RCA message, convergence score, defect type, evidence refs, and artifact output.
