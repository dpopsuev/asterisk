# Contract — Mock Post-investigation (skeleton)

**Status:** complete  
**Goal:** Implement a mock/skeleton for the post-investigation phase so that “push” reads an artifact and updates a defect type (and optionally RCA/Jira fields) via a stub; no real RP or Jira. BDD/TDD, red–green–blue.

## Contract rules

- Mock only: no live RP API or Jira. Push goes to a mock (in-memory or file) that records “defect type updated” and optional Jira ticket ID/link.
- Depends on: **Mock Investigation** (artifact produced). Run after `mock-investigation.md`.
- Glossary: `docs/cli-data-model.mdc` (RCA → Jira; push); `goals/poc.mdc` (push -f).

## Context

- **Post-investigation flow:** Jira ticket (store ID/link on RCA), Slack (out of scope for mock), track bug status, archive. For this contract: **push** = read artifact → “push” defect type to stub; optionally update RCA with Jira ID/link (stub). `notes/three-phases-manual.mdc`.
- **Push command:** `asterisk push -f <artifact-path>`. Reads artifact; pushes defect type to RP (here: mock). `notes/poc-flow.mdc`.
- **RCA storage:** Per RCA: title, description, defect_type, jira_ticket_id, jira_link. Model in `docs/cli-data-model.mdc`.

## Execution strategy

1. **Red** — Write failing BDD/TDD tests: Given an artifact file, When push runs, Then mock receives defect type (and optional Jira ID/link) and stores it; no real HTTP.
2. **Green** — Minimal implementation: read artifact from path; call mock “RP” and mock “RCA store” to record defect type and optional Jira fields; tests pass.
3. **Blue** — Refactor (Pusher interface, artifact parser); no behavior change.
4. **Validate** — All tests pass; acceptance criteria met.

## Tasks

- [x] **Red: BDD scenario — push updates defect type** — Add scenario: Given an artifact file (as produced by mock investigation), When push runs (mock RP and mock RCA store), Then the mock records the defect type (and optional Jira ticket ID/link) for the case(s) in the artifact. Test **fails**.
- [x] **Red: TDD unit** — Unit test(s): (1) parse artifact file; (2) call mock pusher with defect_type; (3) mock store persists RCA fields. Tests **fail**.
- [x] **Green: Read artifact and push to mock** — Parse artifact (same shape as mock investigation output); pass defect_type (and optional jira_ticket_id, jira_link) to mock RP and mock RCA store. Make tests **pass**.
- [x] **Green: Mock store** — In-memory or file-based store that records “pushed” defect type and RCA fields; queryable for assertions.
- [x] **Blue: Tune** — Pusher interface, artifact format documented; no behavior change.
- [x] **Validate (green)** — All tests pass; acceptance criteria met.
- [x] **Validate (green) after blue** — Re-run after refactor; still pass.

## Acceptance criteria (BDD)

- **Given** an artifact file (output of mock investigation),
- **When** post-investigation “push” runs (mock RP, mock RCA store),
- **Then** the mock records the defect type (and optional Jira ticket ID and link) for the case(s) referenced in the artifact.
- **And** no real RP or Jira API is called.

## Notes

(Running log, newest first. YYYY-MM-DD HH:MM — decision or finding.)

- 2026-02-09 — Implemented in `internal/postinvest/`: PushStore, PushedRecord, MemPushStore, Push, DefaultPusher. Test TestPush_RecordsDefectTypeAndJiraInStore passes. Contract complete.
