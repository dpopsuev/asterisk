# Contract — RP API deep research (locked version 5.11)

**Status:** draft  
**Goal:** Learn to interact with the RP API at our locked version (service-api 5.11 / RP 24.1); document endpoints, payloads, and usage in FSC; add rules if needed so implementation (fetch launch, failure list, push defect type) is unambiguous.

## Contract rules

- Global rules only. Follow `rules/abstraction-boundaries.mdc`: document API in a CI-agnostic way; PTP/ecosystem-qe is the example, not the only scenario.
- **Stick to 5.11:** All research targets `report-portal/service-api` at version 5.11 (and report-portal-cli aligned to same). Do not rely on newer API or MCP server.

## Context

- **PoC constraints:** `notes/poc-constraints.mdc` — RP 24.1 / 5.11; CLI-first; fetch launch manually, push via `push -f`.
- **Execution DB:** RP is the Execution DB; envelope and failure list from RP. `docs/envelope-mental-model.mdc`, `notes/pre-dev-decisions.mdc`.
- **Workspace structure:** `notes/cursor-workspace-structure.mdc` — Cursor workspace folders; **service-api** and **report-portal-cli** are primary sources for API research.
- **Workspace references:** `notes/workspace-references.mdc` — report-portal-cli client (GetByID, GetByUUID, List); gap: defect-type update not implemented.
- **Canonical RP instance (saved in FSC):** **Base URL:** https://your-reportportal.example.com — See `notes/rp-instance-and-version.mdc`.
- **Example test execution (pipeline launch):**  
  **URL:** https://your-reportportal.example.com/ui/#ecosystem-qe/launches/all/33195  
  **Launch ID:** 33195 (project: `ecosystem-qe`). Use this launch to learn launch structure, test items, failed items, and logs; verify API behavior and map to Execution Envelope.

## Execution strategy

1. **Discover endpoints** — In `report-portal/service-api`: find launch (get by ID/UUID), test items (list/filter by launch, status), logs, and defect-type update. Note path, method, request/response shape.
2. **Align with report-portal-cli** — See how `report-portal-cli` calls the API (client, config, auth). Identify gaps (e.g. defect-type update).
3. **Map to Execution Envelope** — Document how RP launch + test items map to our envelope type (run_id, items with failure info, git metadata). See `docs/execution-envelope.mdc` and pre-dev decisions.
4. **Document in FSC** — Add or update a note or doc (e.g. `notes/rp-api-usage.mdc` or section in `docs/execution-envelope.mdc`) with: endpoints used for PoC, request/response essentials, auth (e.g. `.rp-api-key`), base URL.
5. **Rules if needed** — If API usage should constrain how agents implement fetch/push, add or update rules under `.cursor/rules/` (e.g. "use only 5.11 launch/test-item endpoints"; "defect-type update: endpoint X, payload Y").

## Tasks

- [x] **Discover version/info endpoint** — **GET `{base}/api/info`** (Bearer token required). Response includes `build.version` (5.11.2), `build.branch` (git hash in branch string, e.g. `47c854e97...`). See `notes/rp-instance-and-version.mdc` (version from service fetched).
- [ ] **Discover launch endpoints** — Get launch by ID (and UUID if used). Request/response; project/launch ID handling.
- [ ] **Discover test-item endpoints** — List items for a launch; filter by status (e.g. failed). Item shape (id, name, status, logs link).
- [ ] **Discover logs / attachments** — How to get log content or attachment URLs for a test item (for RCA context).
- [ ] **Discover defect-type update** — Endpoint and payload to update defect type (and related fields) for a test item; verify against 5.11/24.1. Note in contract or `poc-constraints.mdc`.
- [ ] **Cross-check with report-portal-cli** — Map client calls to service-api endpoints; list what’s missing for PoC (e.g. defect-type write).
- [ ] **Map to Execution Envelope** — Document RP → envelope mapping (required fields, failure list from test items).
- [ ] **Write FSC doc** — Add `notes/rp-api-usage.mdc` (or equivalent) with endpoints, auth, base URL, example launch 33195.
- [ ] **Add rules if needed** — If implementation should be constrained by API usage, add or update `.cursor/rules/`.
- [ ] **Validate (green)** — Documented API usage for PoC (fetch launch, failure list, push defect type); FSC updated; open checks in poc-constraints addressed where possible.
- [ ] **Tune (blue)** — Clarify wording; link from goals/poc and pre-dev-decisions to the new doc.
- [ ] **Validate (green)** — Checklist still complete after tune.

## Acceptance criteria

- Given the PoC need to fetch launch and test items from RP and push defect type,
- When this contract is complete,
- Then the FSC contains a clear, version-pinned (5.11) description of: which endpoints to use, how to authenticate, how launch/test items map to Execution Envelope, and how to update defect type; and rules are added only where they reduce ambiguity for implementation.

## Notes

(Running log, newest first. Use `YYYY-MM-DD HH:MM` — e.g. `2026-02-15 14:32 — Decision or finding`.)

- 2026-02-16 — Contacted canonical instance GET `/api/info` (Bearer from `.rp-api-key`). Path is **/api/info** (not /api/v1/info). API Service version **5.11.2**, branch with git hash `47c854e9739dab476c1bf7bc778cec5b67067a91`. Jobs 5.11.1, analyzer 5.11.0. Result recorded in `notes/rp-instance-and-version.mdc`.
