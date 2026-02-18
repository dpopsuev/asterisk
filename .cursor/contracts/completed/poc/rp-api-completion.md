# Contract — RP API research completion (5.11)

**Status:** complete  
**Goal:** Complete the remaining RP API research tasks so fetch (launch by ID) and push (defect-type update) can be implemented unambiguously; document in FSC.

## Contract rules

- Same as `rp-api-research.md`: stick to service-api 5.11 / RP 24.1; CI-agnostic docs; example launch 33195.
- This contract closes the open tasks in the research contract and produces the FSC doc.

## Context

- **Existing contract:** `rp-api-research.md` — version/info done; launch, test-item, logs, defect-type update, FSC doc, rules still open.
- **Canonical instance:** `notes/rp-instance-and-version.mdc` (base URL, `/api/info`).
- **Need:** Launch by ID (and UUID if used), test items (list/filter by launch, status=FAILED), defect-type update endpoint and payload. Document in `notes/rp-api-usage.mdc` (or equivalent).

## Execution strategy

1. Complete each open task in `rp-api-research.md`: launch endpoints, test-item endpoints, logs/attachments (optional for PoC), defect-type update, cross-check with report-portal-cli, RP → envelope mapping, FSC doc, rules if needed.
2. Write `notes/rp-api-usage.mdc` with: endpoints, auth (`.rp-api-key`, Bearer), base URL, example launch 33195, request/response essentials for fetch and push.
3. Resolve open check in `poc-constraints.mdc` (defect-type endpoint/permissions).
4. Mark tasks complete in `rp-api-research.md` or leave as reference; this contract’s completion = research sufficient for implementation.

## Tasks

- [x] **Launch endpoints** — Get launch by ID (and UUID). Path, method, response shape; project/launch ID handling. Document in FSC.
- [x] **Test-item endpoints** — List items for launch; filter by status (e.g. failed). Item shape (id, name, status, path, etc.). Document in FSC.
- [x] **Logs/attachments (optional)** — How to get log content or attachment URLs for a test item; note for RCA context. Can be minimal for PoC.
- [x] **Defect-type update** — Endpoint and payload to update defect type (and related fields) for a test item; verify 5.11/24.1. Document; address open check in poc-constraints.
- [x] **Cross-check report-portal-cli** — Map client to service-api; list gaps (e.g. defect-type write).
- [x] **RP → envelope mapping** — Document required fields and failure-list derivation from test items. Link from execution-envelope or new section.
- [x] **Write notes/rp-api-usage.mdc** — Endpoints, auth, base URL, example launch 33195; enough for implementer to do fetch and push.
- [x] **Add rules if needed** — If API usage should constrain implementation, add/update `.cursor/rules/`.
- [x] **Validate** — Implementer can build fetch and push from the doc without guessing.

## Acceptance criteria

- **Given** the need to implement fetch (launch by ID) and push (defect type),
- **When** this contract is complete,
- **Then** `notes/rp-api-usage.mdc` (or equivalent) exists with clear, version-pinned (5.11) description of launch endpoint, test-item endpoint, defect-type update, and auth; and open checks in poc-constraints are addressed where possible.

## Notes

(Running log, newest first. YYYY-MM-DD HH:MM — decision or finding.)

- 2026-02-17 — Completed. Created `notes/rp-api-usage.mdc` with launch (GET …/launch/{id}), test-item (GET …/item?filter.eq.launchId=…, filter.eq.status=FAILED, pagination), defect-type update (PUT …/item bulk and PUT …/item/{itemId}/update), auth (Bearer from .rp-api-key), and RP→envelope mapping. Resolved open check in poc-constraints.mdc. No new rules added; doc is sufficient for implementer.
