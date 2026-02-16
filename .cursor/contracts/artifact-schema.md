# Contract — Artifact schema (rca_message and alignment)

**Status:** complete  
**Goal:** Add `rca_message` to the artifact schema and align with pre-dev and PoC success criteria so artifact is the single format for analyze output and push input.

## Contract rules

- Global rules only. Schema must stay backward-compatible for existing mock tests (add field, do not remove).
- Document in FSC (e.g. `docs/data-io.mdc` or a short artifact section).

## Context

- **Pre-dev:** Artifact schema JSON: `rca_message`, `convergence_score`, `defect_type`, `evidence_refs`; optional case_id, launch_id. Same format for analyze output and push -f input. `notes/pre-dev-decisions.mdc`.
- **PoC success criteria:** Artifact contains RCA message, convergence score, suggested defect type, evidence refs. `goals/poc.mdc`.
- **Current:** `internal/investigate/artifact.go` has LaunchID, CaseIDs, DefectType, ConvergenceScore, EvidenceRefs; no `rca_message`.

## Execution strategy

1. Add RCAMessage (or rca_message) to the Artifact struct; update JSON tag.
2. Ensure all producers (investigate, future analyze) and consumers (postinvest push, future push -f) use the same struct or documented shape.
3. Document artifact schema in FSC; reference from pre-dev and goals/poc.
4. Update mock tests if they assert on artifact shape; keep backward compatibility (new field optional for read).
5. Validate; blue if needed.

## Tasks

- [x] **Add rca_message** — Add field to Artifact in `internal/investigate/artifact.go`; JSON tag `rca_message`. Default empty string for existing mock flow.
- [x] **Producers/consumers** — Ensure push (postinvest) and any other reader can handle the new field; no breaking change.
- [x] **Document schema** — Add artifact section to `docs/data-io.mdc` or create short `docs/artifact-schema.mdc`: fields (launch_id, case_ids, rca_message, convergence_score, defect_type, evidence_refs), purpose, example.
- [x] **Tests** — Mock tests still pass; optional test that round-trips artifact with rca_message.
- [x] **Validate (green)** — All tests pass; schema documented.
- [x] **Tune (blue)** — Naming and docs clarity; no behavior change.
- [x] **Validate (green)** — Tests still pass.

## Acceptance criteria

- **Given** the PoC requirement for an artifact with RCA message, convergence score, defect type, evidence refs,
- **When** this contract is complete,
- **Then** the Artifact type and JSON include `rca_message` and the full schema is documented in FSC; existing mock flow and tests remain valid.

## Notes

(Running log, newest first. YYYY-MM-DD HH:MM — decision or finding.)

- 2026-02-17 — Completed. Added RCAMessage (json:"rca_message") to internal/investigate/artifact.go; mock flow sets empty string. Created docs/artifact-schema.mdc with fields, purpose, example; linked from data-io.mdc. Push/postinvest unmarshal into Artifact (new field optional). All tests pass.
