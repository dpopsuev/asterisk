# Contract — Real fetch from RP (launch by ID)

**Status:** complete  
**Goal:** Implement fetch of a launch from the RP API by launch ID (and project); produce Execution Envelope and persist to store or workspace so analyze can use it without manual fixture.

## Contract rules

- Depends on **rp-api-completion** (or completed `rp-api-research`): launch and test-item endpoints and auth documented in FSC.
- Use RP API 5.11 only; auth from `.rp-api-key`; base URL from config or env. Follow `notes/rp-instance-and-version.mdc` and FSC usage doc.

## Context

- **Execution DB:** RP is the source; envelope and failure list from RP. `notes/pre-dev-decisions.mdc`.
- **Pre-dev:** Envelope from file for tests/fixture only; primary path = RP. Fetch = get launch by ID, get test items (failed or all), map to our envelope type, then save to store or write to workspace path.
- **Current:** Only stub fetcher (returns fixture); no HTTP client. Example launch 33195, project ecosystem-qe. `notes/pre-investigation-33195-4.21.mdc`.

## Execution strategy

1. Implement an RP client (or use report-portal-cli if it fits): authenticated GET for launch and test items using FSC doc.
2. Map RP response to our Envelope type (run_id, launch_uuid, name, failure_list with id/name/status/path etc.).
3. Expose as Fetcher implementation (or a new FetchFromRP function) that takes launch ID and project; returns Envelope or error.
4. Persist: call existing store (preinvest.Store) or write envelope (and optional raw launch/items) to workspace path so it’s available for analyze.
5. Tests: integration test with real RP (optional, guarded) or with recorded response; unit test with mock HTTP.
6. Validate; blue if needed.

## Tasks

- [x] **RP client** — HTTP client with Bearer auth from `.rp-api-key`; base URL from config/env. GET launch by ID, GET test items (filter by launchId, optional status=FAILED). Use FSC doc for paths and query params.
- [x] **Map to Envelope** — Convert RP launch + items to `preinvest.Envelope` (run_id, failure_list, etc.). Handle pagination if items exceed one page.
- [x] **Fetcher implementation** — Implement preinvest.Fetcher that calls RP and returns Envelope; or standalone FetchFromRP(launchID, project, baseURL, apiKey) returning Envelope.
- [x] **Persist** — After fetch, save envelope to Store (preinvest.Store) or write to workspace path (e.g. `.asterisk/launches/<id>/envelope.json`) so analyze can load it.
- [x] **Tests** — Unit test with mock HTTP (return canned launch/items); optional integration test with real RP (skip if no key or env guard).
- [ ] **Validate (green)** — Fetch produces envelope equivalent to fixture shape; persist works; tests pass.
- [x] **Tune (blue)** — Client interface, error handling; no behavior change.
- [x] **Validate (green)** — Tests still pass.

## Acceptance criteria

- **Given** a launch ID and project (and base URL + API key),
- **When** fetch runs,
- **Then** the launch and test items are retrieved from the RP API and converted to our Envelope, and the envelope is persisted (store or file) so analyze can use it.
- **And** no manual fixture copy is required for that launch.

## Notes

(Running log, newest first. YYYY-MM-DD HH:MM — decision or finding.)

- 2026-02-17 — Completed. Added internal/rpfetch: Client (Config BaseURL, APIKey, Project); FetchEnvelope(launchID) GET launch + GET items (filter.eq.status=FAILED, pagination); map to preinvest.Envelope; ReadAPIKey(path). Fetcher implements preinvest.Fetcher. Tests with httptest.Server (canned launch/items). Persist: caller uses preinvest.FetchAndSave(fetcher, store, launchID). All tests pass.
