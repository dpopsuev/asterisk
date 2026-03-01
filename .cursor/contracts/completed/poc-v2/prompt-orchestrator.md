# Contract — Prompt orchestrator (F0–F6 circuit engine)

**Status:** complete  
**Goal:** Implement the prompt orchestrator in Go — the engine that runs the F0–F6 prompt circuit per case: evaluates heuristics, fills templates, persists intermediate artifacts, controls loops, and manages per-case state — replacing the current single-prompt `cursor` subcommand with a multi-step, evidence-first investigation flow.

## Contract rules

- Global rules only.
- The orchestrator uses the Store interface only (no raw SQL). Depends on storage-adapter-v2 for the two-tier schema.
- Prompt templates are file-based (Go `text/template`); the orchestrator fills and outputs them but does **not** call an AI model — in PoC (`asterisk --cursor`), the user pastes the prompt into Cursor. The orchestrator is a state machine, not an AI caller.
- Every heuristic decision is logged with the signal that triggered it (explainability invariant).
- Intermediate artifacts are JSON files on disk. The orchestrator reads the previous step's artifact to decide the next step. No reliance on chat context surviving between steps.
- Loop bounds must be configurable and enforced (no infinite loops).

## Context

- **Design contract:** `contracts/prompt-families.md` — F0–F6 taxonomy, 5 circuits, 3 loop types, 17 heuristics, 34 guards, configurable thresholds.
- **Current state:** `asterisk cursor` subcommand generates one prompt from `rca.md` template. No multi-step circuit, no heuristics, no intermediate artifacts, no per-case directory.
- **Store v2:** `contracts/storage-adapter-v2.md` — provides InvestigationSuite, Circuit, Launch, Job, Case, Triage, Symptom, RCA, SymptomRCA. The orchestrator reads/writes these entities at each circuit step.
- **Template params:** `docs/prompts.mdc` — 9 parameter groups (Timestamps, Env, Git, Failure, Siblings, Workspace, URLs, Prior/History, Taxonomy). The orchestrator populates these from Store + envelope + workspace data.
- **Cursor handoff:** `contracts/cursor-handoff.md` — PoC flow is manual: orchestrator outputs prompt file → user pastes in Cursor → user runs `asterisk save` → orchestrator reads artifact and advances state. The orchestrator drives this loop.

## Architecture

### Core components

```
┌──────────────────────────────────────────────────────┐
│ Orchestrator (internal/orchestrate/)                   │
│                                                        │
│  ┌──────────┐   ┌──────────────┐   ┌──────────────┐  │
│  │ Circuit  │──▶│  Heuristic   │──▶│  Template    │  │
│  │ Runner    │   │  Engine      │   │  Filler      │  │
│  └──────────┘   └──────────────┘   └──────────────┘  │
│       │                │                   │           │
│       ▼                ▼                   ▼           │
│  ┌──────────┐   ┌──────────────┐   ┌──────────────┐  │
│  │ Artifact  │   │  Store       │   │  Case State  │  │
│  │ I/O       │   │  (v2)       │   │  Manager     │  │
│  └──────────┘   └──────────────┘   └──────────────┘  │
└──────────────────────────────────────────────────────┘
```

1. **Circuit Runner** — Executes the prompt circuit per case. Knows the circuit topology (§2 of prompt-families). Manages the step sequence and loop re-entry.
2. **Heuristic Engine** — Table-driven decision engine. Evaluates heuristics (H1–H17) against the current artifact to determine the next step. Returns `(next_family, context_additions)`.
3. **Template Filler** — Loads a prompt template file, populates parameter groups from Store/envelope/workspace/prior artifacts, writes the filled prompt to the per-case directory.
4. **Artifact I/O** — Reads and writes typed JSON artifacts per family per case. Handles the per-case directory structure.
5. **Case State Manager** — Tracks per-case progress: which families have run, loop iteration counts, current status. Persisted to disk (JSON state file) so the orchestrator can resume across CLI invocations.

### Per-case directory structure

```
.asterisk/investigations/{suite_id}/{case_id}/
├── state.json              # Case state: current_step, loop_counts, status
├── recall-result.json      # F0 output
├── triage-result.json      # F1 output
├── resolve-result.json     # F2 output
├── artifact.json           # F3 output (main investigation artifact)
├── correlate-result.json   # F4 output
├── review-decision.json    # F5 output
├── jira-draft.json         # F6 output
├── regression-report.md    # F6 output
├── prompt-f0.md            # Generated prompt for F0
├── prompt-f1.md            # Generated prompt for F1
├── ...                     # etc.
└── prompt-f3-loop-2.md     # Loop iteration prompt
```

### State machine

```
           ┌────────────────────────────────────────────┐
           │                                            │
INIT ──▶ F0 ──hit──▶ F5 ──approve──▶ F6 ──▶ DONE      │
           │                │                           │
           │miss            │overturn                   │
           ▼                ▼                           │
          F1 ──skip──▶ F5 ──reassess──▶ F1/F2/F3 ─────┘
           │                                            │
           │investigate                                 │
           ▼                                            │
          F2 ──▶ F3 ──converged──▶ F4 ──▶ F5 ──approve──▶ F6 ──▶ DONE
                  │                                     │
                  │low conf (loop < max)                │
                  └──▶ F2 ──▶ F3 ──────────────────────┘
                  │
                  │exhausted (loop >= max)
                  └──▶ F5 (with ti001)
```

## Execution strategy

1. Define the orchestrator package and core types (state, heuristic rule, circuit step).
2. Implement Artifact I/O (read/write typed JSON to per-case directory).
3. Implement Case State Manager (init, advance, persist, resume).
4. Implement Heuristic Engine (table-driven, evaluate in order, log decisions).
5. Implement Template Filler (load template, populate params from Store/envelope/workspace, write prompt).
6. Implement Circuit Runner (drive the loop: fill template → output → wait for user → read artifact → evaluate heuristic → advance).
7. Write prompt templates for each family (F0–F6).
8. Integrate with CLI: replace current `asterisk cursor` single-shot with orchestrated flow.
9. End-to-end dry-run with PTP scenario.

## Tasks

### Phase 1 — Core types and artifact I/O ✅

- [x] **Define core types** — `internal/orchestrate/types.go`: CircuitStep (enum: F0–F6), CaseState (current_step, loop_counts, status, case_id, suite_id), HeuristicRule (id, name, signal_field, condition, action, explanation).
- [x] **Artifact I/O** — `internal/orchestrate/artifact.go`: ReadArtifact[T](caseDir, filename) → T; WriteArtifact(caseDir, filename, data) → error. Typed structs for each artifact: RecallResult, TriageResult, ResolveResult, InvestigateArtifact, CorrelateResult, ReviewDecision.
- [x] **Per-case directory** — CreateCaseDir(suiteID, caseID) → path; EnsureCaseDir; ListCaseDirs.

### Phase 2 — State management ✅

- [x] **Case State Manager** — `internal/orchestrate/state.go`: InitState(caseID, suiteID) → CaseState; LoadState(caseDir) → CaseState; SaveState(caseDir, state); AdvanceStep(state, nextStep); IncrementLoop(state, loopName); IsLoopExhausted(state, loopName, maxIterations).
- [x] **Resume across invocations** — On `asterisk cursor`, detect existing state.json; resume from current_step. If no state, start at F0.

### Phase 3 — Heuristic engine ✅

- [x] **Heuristic table** — `internal/orchestrate/heuristics.go`: DefaultHeuristics() → []HeuristicRule. 17 rules from prompt-families §4.1.
- [x] **Evaluation** — EvaluateHeuristics(state, currentArtifact, heuristics) → (nextStep CircuitStep, contextAdditions map[string]any, matchedRule string). Evaluates in order per §4.2. Logs the matched rule and signal values.
- [x] **Configurable thresholds** — Load from workspace config or CLI flags. Defaults per §4.3.

### Phase 4 — Template filler ✅

- [x] **Parameter builder** — `internal/orchestrate/params.go`: BuildParams(store, caseID, envelope, workspace) → TemplateParams. Populates all 9 parameter groups from `docs/prompts.mdc`.
- [x] **Template execution** — FillTemplate(templatePath, params) → string. Uses Go `text/template`. Guards (G1–G34) are embedded in templates via conditional blocks.
- [x] **Prompt output** — WritePrompt(caseDir, familyName, content) → promptPath. Writes the filled prompt to per-case directory. Returns path for user to open in Cursor.

### Phase 5 — Circuit runner ✅

- [x] **Runner** — `internal/orchestrate/runner.go`: RunStep(store, state, caseDir) → (promptPath, error). The main loop driver.
- [ ] **F0 programmatic step** — F0 Recall is mostly programmatic (fingerprint match + DB query). RunF0(store, caseID, failure) → RecallResult. Only fires the judge-similarity prompt if candidates are found. *(Deferred: currently F0 always produces a prompt; programmatic pre-check can be added later.)*
- [x] **Store side effects** — After each step, update Store: F0 → set case.symptom_id; F1 → create triage row, upsert symptom; F3 → create/link RCA; F4 → link case to shared RCA, update symptom_rca; F5 approve → update case status.

### Phase 6 — Prompt templates ✅

- [x] **Write F0 template** — `recall/judge-similarity.md` — light similarity judge.
- [x] **Write F1 template** — `triage/classify-symptoms.md` — symptom classification from error output.
- [x] **Write F2 template** — `resolve/select-repo.md` — repo selection from workspace + triage.
- [x] **Write F3 template** — `investigate/deep-rca.md` — evolved from `rca.md` with full guards.
- [x] **Write F4 template** — `correlate/match-cases.md` — cross-case comparison.
- [x] **Write F5 template** — `review/present-findings.md` — human review presentation.
- [x] **Write F6 template** — `report/regression-table.md` — Jira draft + regression table.

### Phase 7 — CLI integration ✅

- [x] **Replace cursor subcommand** — `asterisk cursor` now drives the orchestrator: loads state, runs next step, outputs prompt path.
- [x] **Update save subcommand** — `asterisk save -f --case-id --suite-id` reads artifact, updates Store (via orchestrator's store side effects), advances state. Legacy save (without case-id/suite-id) preserved.
- [x] **New: status subcommand** — `asterisk status` shows current case state, which step is next, loop counts.
- [ ] **New: batch mode** — `asterisk cursor --batch` iterates all open cases in a suite (circuit 2.5 from prompt-families). *(Deferred to post-calibration.)*

### Phase 8 — Validate ✅

- [x] **Unit tests** — Heuristic engine (each rule: 19 tests), state management (advance, loop, resume), artifact I/O (read/write/typed), template filler (string + file + guards + siblings + prior), fingerprint.
- [x] **Integration test** — Full circuit dry-run (TestRunnerFullCircuit): mock artifacts at each step, verify heuristic routing F0→F1→F2→F3→F4→F5→F6→DONE, verify state transitions, verify store side effects.
- [x] **Short-circuit test** — TestRunnerRecallHitShortCircuit: F0→F5 via H1 recall-hit.
- [x] **Loop test** — TestRunnerInvestigateLoop: F3→F2 low-confidence loop with counter verification.
- [x] **Validate (green)** — All 30+ orchestrate tests pass. Full suite (11 packages) green.
- [ ] **End-to-end** — PTP scenario (launch 33195): run through F0→F6 manually with real data. *(Part of e2e-calibration contract.)*
- [ ] **Tune (blue)** — Clean interfaces, consistent error handling, template wording. *(Part of refactor step.)*

## Acceptance criteria

- **Given** a case in an investigation suite,
- **When** the user runs `asterisk cursor` repeatedly (with `asterisk save` between steps),
- **Then** the orchestrator advances through the F0–F6 circuit based on heuristic evaluation of intermediate artifacts,
- **And** each step produces a typed artifact persisted to the per-case directory,
- **And** the orchestrator can resume from any step across CLI invocations (state persisted),
- **And** loops are bounded by configurable max iterations,
- **And** every heuristic decision is logged with the signal and matched rule,
- **And** the human review gate (F5) always fires before any RP write,
- **And** Store entities are updated at each step (case status, symptom links, RCA links, triage records).

## Notes

(Running log, newest first. YYYY-MM-DD HH:MM — decision or finding.)

- 2026-02-16 16:10 — Contract complete. All 8 phases implemented: core types, artifact I/O, state management, heuristic engine (17 rules), template filler (9 parameter groups), circuit runner (with store side effects for F0-F5), F0-F6 prompt templates (with 34 guards), CLI integration (cursor/save/status subcommands). 30+ unit and integration tests passing. Deferred: F0 programmatic pre-check, batch mode, real-data e2e (delegated to e2e-calibration contract).
- 2026-02-17 02:30 — Contract created. Depends on storage-adapter-v2 (v2 Store interface) and prompt-families (design). Current baseline: single-shot `asterisk cursor` subcommand. Target: multi-step orchestrator with 7 families, 17 heuristics, 3 loop types, per-case state, and Store side effects.
