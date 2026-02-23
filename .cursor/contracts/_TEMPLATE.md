# Contract — {slug}

**Status:** draft | active | complete | abandoned  
**Goal:** (One sentence. Binary — true or false.)  
**Serves:** (Goal name from current-goal.mdc, e.g. "PoC completion")

## Contract rules

(Additions to global rules for this work. Or: "Global rules only.")

## Context

(Links to notes, docs, design decisions.)

### Current architecture
(Optional. Mermaid diagram of affected components before this contract.)

### Desired architecture
(Optional. Mermaid diagram of target state after this contract.)

## FSC artifacts

(What reusable knowledge will this contract produce? Route to the correct FSC target.)

| Artifact | Target | Compartment |
|----------|--------|-------------|
| (e.g. "design reference") | `docs/` | domain |
| (e.g. "new glossary term") | `glossary/` | domain |
| (e.g. "infra credentials") | `sensitive/` | sensitive |

(If this contract produces no reusable artifacts beyond code, write: "Code only — no FSC artifacts.")

## Execution strategy

(Approach and ordering.)

## Coverage matrix

Declare which test layers apply (per `rules/universal/testing-methodology.mdc`). Mark N/A with rationale.

| Layer | Applies | Rationale |
|-------|---------|-----------|
| **Unit** | (yes/no) | (what functions/branches are tested, or why N/A) |
| **Integration** | (yes/no) | (what cross-boundary interactions, or why N/A) |
| **Contract** | (yes/no) | (what API schemas/interfaces, or why N/A) |
| **E2E** | (yes/no) | (what pipeline validation, or why N/A) |
| **Concurrency** | (yes/no) | (what shared state/parallel paths, or why N/A) |
| **Security** | (yes/no) | (see Security assessment section below, or why N/A) |

## Tasks

- [ ] (Task 1)
- [ ] (Task 2)
- [ ] Validate (green) — all tests pass, acceptance criteria met.
- [ ] Tune (blue) — refactor for quality. No behavior changes.
- [ ] Validate (green) — all tests still pass after tuning.

## Acceptance criteria

(Given/When/Then or structural invariants.)

## Security assessment

(OWASP spot-check for trust boundaries touched by this contract. Or: "No trust boundaries affected.")

| OWASP | Finding | Mitigation |
|-------|---------|------------|

## Notes

(Running log, newest first. Use `YYYY-MM-DD HH:MM` — e.g. `2026-02-15 14:32 — Decision or finding.`
On completion: extract reusable knowledge to FSC targets declared above.)
