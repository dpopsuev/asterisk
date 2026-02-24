# Contract — marshaller-step-schema-factory

**Status:** complete  
**Goal:** Enrich Asterisk step schemas with `FieldDef` entries for runtime validation and update calibrate skill for `submit_step`.  
**Serves:** PoC completion

## Contract rules

Global rules only.

## Context

Companion to `origami/.cursor/contracts/active/marshaller-step-schema-factory.md`. The framework-side contract adds `submit_step` to Origami's MCP Marshaller. This contract updates Asterisk's domain-specific schemas and skill to use the new tool.

## FSC artifacts

Code only — no FSC artifacts.

## Execution strategy

1. Enrich `asteriskStepSchemas()` with `FieldDef` entries (required markers per field).
2. Update `asterisk-calibrate` SKILL.md to reference `submit_step`.
3. Validate compilation and tests.

## Coverage matrix

| Layer | Applies | Rationale |
|-------|---------|-----------|
| **Unit** | no | Schema definitions are data; validation logic is tested in Origami |
| **Integration** | yes | Stub calibration round-trip verifies no regressions |
| **Contract** | no | Schema contract tested in Origami |
| **E2E** | yes | `just calibrate-stub` confirms baseline match |
| **Concurrency** | no | No new shared state |
| **Security** | no | No trust boundaries affected in Asterisk |

## Tasks

- [x] Enrich `asteriskStepSchemas()` with `FieldDef` entries
- [x] Update `asterisk-calibrate` SKILL.md for `submit_step`
- [x] Validate (green) — `go build ./...` and `go test ./...` pass
- [x] Tune (blue) — refactor for quality
- [x] Validate (green) — all tests still pass after tuning

## Acceptance criteria

- **Given** enriched step schemas, **when** `go build ./...` runs, **then** compilation succeeds.
- **Given** all changes applied, **when** `go test ./...` runs, **then** all tests pass.

## Security assessment

No trust boundaries affected.

## Notes

2026-02-24 — Contract created. Schemas enriched, skill updated. Validation pending.
2026-02-24 — All tasks complete. Build and tests pass in both repos.
