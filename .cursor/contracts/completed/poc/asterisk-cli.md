# Contract — asterisk CLI (analyze and push -f)

**Status:** complete  
**Goal:** Implement the main CLI binary `asterisk` with subcommands `analyze` and `push -f`, wired to envelope source (file or store), context workspace, storage adapter, and artifact; optional integration with real RP fetch/push when available.

## Contract rules

- Follow PoC scope: `asterisk --cursor` is the invocation mode for the dialog; CLI commands `analyze` and `push -f` are the primary actions. See `goals/poc.mdc`, `notes/poc-flow.mdc`.
- Depends on **storage-adapter** (Store facade), **context-workspace** (loader), **artifact-schema** (rca_message). Can start with file-based envelope; **rp-fetch** and **rp-push** can be integrated when their contracts are done.

## Context

- **analyze:** Input: launch (path to envelope file or launch ID if fetch available), context workspace path. Output: artifact on FS (RCA message, convergence score, defect type, evidence refs). Uses Store for cases; may create cases from envelope. No RP write. `goals/poc.mdc`, `notes/pre-dev-decisions.mdc`.
- **push:** Input: path to artifact (`push -f <path>`). Reads artifact, pushes defect type to RP (real or mock). `notes/poc-flow.mdc`.
- **Current:** Only `cmd/run-mock-flow` exists; no `asterisk` binary with subcommands.

## Execution strategy

1. Add `cmd/asterisk`: main that delegates to subcommands (e.g. cobra or flag-based). Subcommands: `analyze`, `push`, optionally `fetch` (when rp-fetch done).
2. **analyze:** Flags: `--launch=<path|id>`, `--workspace=<path>`, `-o <artifact-path>`. Load envelope (from file or from store after fetch); load context workspace; ensure cases exist in Store (create from envelope if needed); run analyze path (current investigate.Analyze or extended version that uses workspace and produces rca_message placeholder); write artifact.
3. **push:** Flag: `-f <artifact-path>`. Read artifact; call push (real RP or mock per config).
4. Config: DB path, RP base URL, API key path (e.g. .rp-api-key), prompts dir. Env or config file.
5. Keep run-mock-flow for demo; Makefile can build both.
6. Tests: CLI tests (invoke binary or main); integration with file-based envelope and mock push.
7. Validate; blue if needed.

## Tasks

- [x] **cmd/asterisk structure** — Main entry; subcommands analyze and push; flags and config (DB path, RP base URL, api key, workspace paths).
- [x] **analyze subcommand** — --launch (file path or launch ID), --workspace, -o artifact. Load envelope (file or store); load workspace; ensure cases in Store; run analyze; write artifact (include rca_message when schema ready).
- [x] **push subcommand** — -f <path>. Read artifact; call push (RP or mock); exit code and errors.
- [x] **Config** — Resolve config (cwd, .asterisk, env); DB path, RP base URL, .rp-api-key path; document in FSC.
- [x] **Wire storage and workspace** — Analyze uses Store (create/list cases) and workspace loader; push uses real or mock pusher based on config.
- [x] **Tests** — Test analyze with file envelope and workspace file; test push with artifact file; mock RP where needed.
- [x] **Validate (green)** — `asterisk analyze` and `asterisk push -f` work with file-based envelope and mock push; artifact format stable.
- [x] **Tune (blue)** — Structure, help text; no behavior change.
- [x] **Validate (green)** — Tests still pass.

## Acceptance criteria

- **Given** an envelope file (or launch ID when fetch is available) and a context workspace file,
- **When** the user runs `asterisk analyze --launch=<path|id> --workspace=<path> -o <artifact>`,
- **Then** an artifact file is written at the specified path with launch_id, case_ids, defect_type, convergence_score, evidence_refs, rca_message (or placeholder).
- **Given** an artifact file,
- **When** the user runs `asterisk push -f <path>`,
- **Then** the artifact is read and defect type is pushed to RP (or mock when configured).
- **And** the CLI is the single entry point for PoC analyze and push.

## Notes

(Running log, newest first. YYYY-MM-DD HH:MM — decision or finding.)

- 2026-02-17 — Completed. Added cmd/asterisk: flag-based subcommands analyze and push. analyze: --launch (file path or launch ID), --workspace, -o artifact; load envelope from file or store (optional fetch from RP with --rp-base-url); load workspace; AnalyzeWithWorkspace; write artifact. push: -f artifact path; mock or RP per --rp-base-url. Config: --db, --rp-base-url, --rp-api-key. Test: analyze + push with file envelope. run-mock-flow kept. All tests pass.
