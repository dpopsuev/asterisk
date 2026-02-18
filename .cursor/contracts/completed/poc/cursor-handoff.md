# Contract — Cursor handoff (minimal asterisk --cursor)

**Status:** complete  
**Goal:** Define and implement the minimal `asterisk --cursor` flow: how the user gets the next prompt (file or text), pastes or opens it in Cursor, and how the user saves/ingests the result (artifact) into Asterik so state persists; no Cursor API.

## Contract rules

- PoC = no Cursor API/MCP; handoff is manual: Asterik produces prompt, user pastes/opens in Cursor; user runs Asterik command to save result. See `notes/pre-dev-decisions.mdc`, `notes/poc-flow.mdc`.
- Depends on **asterisk-cli** (commands exist) and **prompts** (templates and params).

## Context

- **Pre-dev:** Asterik generates prompt text or prompt file; user pastes into Cursor or opens file; Cursor runs the model. After each step, user runs an Asterik command (e.g. `asterisk save -f <artifact-path>` or `asterisk ingest <path>`) that reads the artifact and writes to Asterik DB. No Cursor→Asterik API. `notes/pre-dev-decisions.mdc`.
- **PoC flow:** `asterisk --cursor` → get next prompt/file → user pastes/opens in Cursor → user continues → user runs save/ingest with artifact path. `notes/poc-flow.mdc`.
- **Current:** No --cursor mode; no prompt delivery; no save/ingest command.

## Execution strategy

1. **--cursor mode:** When user runs `asterisk --cursor` (or `asterisk cursor` subcommand), Asterik outputs the next prompt (to stdout or writes to a file in workspace, e.g. `.asterisk/current_prompt.md`). Document: user copies to Cursor or opens the file in Cursor.
2. **Prompt content:** From template + params (launch_id, case_id, workspace_path, etc.); template from prompts contract. One prompt per “step” (e.g. one case assessment or RCA).
3. **Save/ingest:** Add command (e.g. `asterisk save -f <artifact-path>` or `asterisk ingest <path>`) that reads the artifact file and writes to Store (case update, RCA if present). Document in poc-flow.
4. **State:** Store holds cases and RCAs; “next” prompt might be “next case” or “same case, next step”. Define minimally: e.g. list cases, pick one, generate prompt for that case; after ingest, mark progress. Keep scope minimal for PoC.
5. Document full flow in `notes/poc-flow.mdc` or a new “Cursor handoff” section so agents and users can follow it.
6. Validate with a manual run-through; blue if needed.

## Tasks

- [x] **--cursor behavior** — When `asterisk --cursor` (or `asterisk cursor`): generate next prompt from template + params; output to stdout or write to file (e.g. `.asterisk/current_prompt.md`). Document: user pastes or opens in Cursor.
- [x] **Next-prompt logic** — Define “next”: e.g. list open cases, pick one (first or by choice), load envelope and workspace, fill template params for that case, output prompt. Minimal: one case per prompt for PoC.
- [x] **Save/ingest command** — Add `asterisk save -f <path>` (or ingest): read artifact from path, parse; update Store (case, RCA if present). Document in FSC.
- [x] **Document flow** — Update `notes/poc-flow.mdc` or add `.cursor/docs/cursor-handoff.mdc`: step-by-step (run asterisk --cursor → get prompt → paste/open in Cursor → run model → save artifact to file → run asterisk save -f <path>). Link from goals/poc.
- [x] **Validate** — Manual run-through: generate prompt, simulate paste, produce artifact file, run save -f; verify Store updated.
- [x] **Tune (blue)** — Clarity of docs and CLI help; no behavior change.
- [x] **Validate** — Flow still works.

## Acceptance criteria

- **Given** the PoC constraint of no Cursor API,
- **When** this contract is complete,
- **Then** the user can run `asterisk --cursor` (or equivalent) to get the next prompt (text or file), use it in Cursor manually, and run an Asterik command to save the artifact so state persists in the Asterik DB; and the flow is documented so an agent or user can follow it.

## Notes

(Running log, newest first. YYYY-MM-DD HH:MM — decision or finding.)

- 2026-02-17 — Completed. Added `asterisk cursor` subcommand: --launch (path or id), --workspace, --case-id, -o (prompt file or stdout); loads envelope, picks case, fills template (.cursor/prompts/rca.md), outputs prompt. Added `asterisk save -f <path>`: reads artifact, creates RCA in Store, links cases (creates cases from artifact if not present). Updated notes/poc-flow.mdc and created docs/cursor-handoff.mdc with step-by-step flow. No Cursor API; handoff is manual.
