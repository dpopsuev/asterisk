# Contract — Real push to RP (defect-type update)

**Status:** complete  
**Goal:** Implement push of artifact data to the RP API: read artifact, update defect type (and related fields) for the relevant test item(s) in RP so the UI reflects the result.

## Contract rules

- Depends on **rp-api-completion**: defect-type update endpoint and payload documented in FSC.
- Use RP API 5.11 only; auth from `.rp-api-key`; base URL from config or env. No push during analyze; explicit push only (e.g. `asterisk push -f <path>`).

## Context

- **PoC flow:** User/agent runs analyze → gets artifact → reviews → runs push -f <path> to send defect type to RP. `notes/poc-flow.mdc`, `goals/poc.mdc`.
- **Current:** postinvest.Push writes to a mock store only; no HTTP call to RP.
- **Artifact:** Contains launch_id, case_ids, defect_type, convergence_score, evidence_refs, rca_message (after artifact-schema contract). Push must map artifact to RP test item(s) and call update endpoint.

## Execution strategy

1. Implement RP client call for defect-type update: endpoint and payload from FSC doc (rp-api-completion).
2. Push flow: read artifact from path; for each case (or for the launch’s failed items) update defect type in RP. Clarify: one artifact per case or per launch? Pre-dev suggests artifact can have case_id, launch_id; if one artifact per case, one update; if per launch, possibly multiple updates (one per case in artifact).
3. Replace or wrap mock: real Pusher implementation that calls RP; keep mock for tests.
4. Tests: unit test with mock HTTP; optional integration test with real RP (guarded).
5. Validate; blue if needed.

## Tasks

- [x] **Defect-type API call** — Implement HTTP request to RP defect-type update endpoint (method, path, body) per FSC doc. Auth and base URL as in rp-fetch.
- [x] **Map artifact to RP** — From artifact (launch_id, case_ids, defect_type) determine which RP test item(s) to update; call update for each or for the scope documented in API.
- [x] **Push implementation** — postinvest: add RealPusher or similar that reads artifact and calls RP; or extend Push to accept a PusherTarget (mock vs RP). Keep existing mock store for tests.
- [x] **Error handling** — Network errors, 4xx/5xx; report to user; do not overwrite artifact.
- [x] **Tests** — Unit test with mock HTTP (expect correct endpoint and payload); existing mock tests still pass.
- [x] **Validate (green)** — Push to mock still works; push to RP (or mock HTTP) succeeds in test.
- [x] **Tune (blue)** — Interfaces, naming; no behavior change.
- [x] **Validate (green)** — Tests still pass.

## Acceptance criteria

- **Given** an artifact file path and RP base URL + API key,
- **When** push runs (e.g. `asterisk push -f <path>`),
- **Then** the artifact is read and the defect type (and any related fields) are sent to the RP API for the relevant test item(s), and the RP UI can reflect the update.
- **And** tests can still use a mock (no real RP call).

## Notes

(Running log, newest first. YYYY-MM-DD HH:MM — decision or finding.)

- 2026-02-17 — Completed. Added internal/rppush: Client (Config BaseURL, APIKey, Project); UpdateItemDefectType(itemID, defectType) PUT .../item/{id}/update with issues payload; UpdateItemsDefectType(itemIDs, defectType). RPPusher implements postinvest.Pusher: read artifact, call RP for each case_id, then store.RecordPushed. Tests with httptest.Server. Mock (DefaultPusher) unchanged. All tests pass.
