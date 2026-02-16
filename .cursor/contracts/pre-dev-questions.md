# Contract — pre-dev-questions

**Status:** complete  
**Goal:** All beneficial pre-development questions are listed and each has an answer (or a deferred/experiment decision with reason) so we can start implementation without blocking ambiguity.

## Contract rules

Global rules only. Follow `rules/scenario-vs-generic.mdc`: mark answers as generic vs PTP/current use case where it matters.

## Context

- **PoC scope:** `goals/poc.mdc` — `asterisk --cursor` only; analyze + push -f; storage adapter; artifact on FS.
- **Open design choices:** `docs/cli-data-model.mdc` (storage adapter interface, envelope identity, when to create cases, RCA creation).
- **Envelope / artifacts:** `docs/envelope-mental-model.mdc` (pure model vs PTP use case); `notes/test-output-and-ci-agnostic.mdc` (output format).
- **Constraints:** `notes/poc-constraints.mdc` (RP 24.1/5.11, CLI-first, artifact format).
- **Flow:** `notes/poc-flow.mdc`, `notes/three-phases-manual.mdc`.

## Execution strategy

1. **Make questions** — Draft a question bank from FSC open choices and obvious gaps (envelope, storage, Cursor integration, artifact schema, RP, prompts, test output). Review and add/trim.
2. **Answer them** — For each question: produce an answer or mark "deferred (reason)" / "experiment (what to try)". Record in this contract's Notes (or a linked doc).
3. **Validate** — Every question has a recorded outcome; no blocking ambiguity for PoC implementation start.
4. **Tune** — Optionally group answers into a short "Pre-dev decisions" note or doc section for handoff.
5. **Validate** — Checklist still complete after tune.

## Question bank (beneficial before development)

*Answer or mark "Deferred: …" / "Experiment: …" in Notes below. Add new questions as needed.*

### Envelope and investigation identity

1. **Envelope identity** — What exactly keys an investigation case (unique per failure)? Slim key (e.g. job ID + run path + failed test name) vs full Execution Envelope? Affects deduplication and storage.
2. **Execution Envelope from file** — For PoC tests/fixture only: what is the expected file format (JSON/YAML)? Which fields are required vs optional? Same shape as RP launch response?
3. **Execution Envelope from RP** — RP is the Execution DB; launch by ID is primary. Use report-portal-cli / RP API 5.11; which endpoint(s) and how do we map response to our envelope type?

### Storage adapter and DB

4. **Storage adapter interface shape** — Per-entity repositories (CaseRepo, RCARepo, PipelineRepo) vs single Store facade? Minimal method set for PoC (e.g. CreateCase, GetCase, ListCasesByJob, SaveRCA, LinkCaseToRCA)?
5. **DB placement** — Where does the SQLite file live? Per-workspace (e.g. `./asterisk.db` or `./.asterisk/db`) vs global (e.g. `~/.config/asterisk/`)? Affects "one DB per investigation" vs "one DB per user."

### Cursor integration (`asterisk --cursor`)

6. **Dialog mechanism** — How does Asterik "inject prompts" and "conduct dialog" with Cursor? (e.g. generate a prompt file / message that the user pastes into Cursor? Or is there an API / MCP / CLI that Cursor calls?) What is the handoff protocol?
7. **Saving to Asterik DB from the flow** — When and how does investigation data (case, RCA, assessment) get written to the Asterik DB during a Cursor session? (e.g. user runs a command after each step? Asterik watches a dir? Cursor sends via MCP?) Must be unambiguous for PoC.

### Artifact and output format

8. **Artifact schema** — Exact shape of the artifact file (RCA message, convergence score, defect type, evidence refs). JSON vs YAML? Field names and types? Needed for `analyze` output and `push -f` input.
9. **Failure list source (PoC)** — Failure list comes from **RP** (launch test items). No Jenkins dump or job log parsing. How do we map RP test items to our failure list (required fields, status filter)?

### RP API and push

10. **RP defect type update** — For `push -f`: which RP API 5.11 endpoint and payload updates defect type (and related fields)? Verify against 24.1/5.11; note in contract or `poc-constraints.mdc`.
11. **RP auth in PoC** — Use `.rp-api-key` for API key; base URL from config or flag? Where is RP base URL defined (e.g. env, config file, flag)?

### Prompts and templates

12. **Prompt loading** — When running as CLI (not dev in Cursor), where does the CLI load prompt templates from? Directory path convention; fallback if not found?
13. **Template parameters** — For file-based prompts: what parameters do we pass (e.g. launch_id, case_id, workspace_path, failed_test_name)? List so templates and CLI stay in sync.

### Cases and RCA lifecycle

14. **When to create cases** — On "open investigation": create one DB case per failure up front (from RP launch test items / envelope) vs create cases lazily when user/agent starts each? Impacts flow and storage.
15. **RCA creation** — Create RCA row only after agent/human produces an RCA vs create placeholder at case creation and fill later? PoC: manual "same as case X" — do we need placeholder RCAs?

### Optional / later

16. **Convergence score source** — Is convergence score produced by the model (in the prompt response) or by a separate heuristic? Affects artifact schema and prompt design.
17. **First scenario (PTP) scope** — For "at least one real launch + context workspace": use RP launch by ID + current workspace layout; for tests use minimal fixture envelope or mock RP response.

## Tasks

- [x] **Make questions** — Finalize question bank (add/trim from list above); ensure categories cover envelope, storage, Cursor, artifact, RP, prompts, lifecycle.
- [x] **Answer questions** — For each question in the bank: write answer or "Deferred: …" / "Experiment: …" with reason. Record in Notes below (or linked doc).
- [x] **Validate (green)** — Every question has a recorded outcome; no blocking ambiguity for starting PoC implementation.
- [x] **Tune (blue)** — Pre-dev decisions summary in `notes/pre-dev-decisions.mdc`; linked from this contract.
- [x] **Validate (green)** — Checklist complete; decisions discoverable.

## Acceptance criteria

- Given the PoC goal and open design choices in the FSC,
- When we complete this contract,
- Then every question in the bank has an answer or an explicit deferred/experiment decision with reason, and that outcome is recorded (Notes or linked doc). A developer can start implementation without guessing on these points.

## Notes

(Running log, newest first. Use `YYYY-MM-DD HH:MM` — e.g. `2026-02-15 14:32 — Decision or finding.`)

- 2026-02-15 12:00 — Contract executed. All 17 questions answered or deferred; full decisions in `notes/pre-dev-decisions.mdc`.
