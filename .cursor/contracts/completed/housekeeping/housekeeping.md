# Contract — housekeeping

**Status:** complete  
**Goal:** Remove all Go-era leftover artifacts from Asterisk now that the repo is 100% YAML.  
**Serves:** 100% DSL — Zero Go; repository hygiene; onboarding clarity

## Contract rules

- Mechanical changes only — no behavior changes, no new features.
- Every file touched must still be valid after the change (no broken references).
- Skills and rules must reflect the current build/test workflow: `origami fold` for build, tests in Origami, `origami lint` for pipeline linting.

## Context

After `origami-fold` completed, Asterisk has zero `.go` files, no `go.mod`, no `cmd/`, no `internal/`, no `adapters/`. But many supporting files still reference the old Go workflow (`go build`, `make`, Go test commands, removed directories). A new developer or agent reading these files would be confused.

## FSC artifacts

Code only — no FSC artifacts.

## Execution strategy

All changes are independent and can be done in any order. Group by file type for reviewability.

### Phase 1: Build and config files

- `.gitignore` — remove Go-specific entries (`*.test`, `*.out`, `go.work`). Keep binary entries (`/bin/`, `/asterisk`) since `origami fold` still produces them.
- `justfile` — remove stale `generate-scenario` recipe (references nonexistent `internal/calibrate/` path). Fix comment on line 25 referencing `go test ./...`.

### Phase 2: Skills

- `.cursor/skills/asterisk-analyze/SKILL.md` — replace `go build -o bin/asterisk ./cmd/asterisk/` with `just build` / `origami fold`. Remove `cmd/asterisk-analyze-rp-cursor/` references (binary was deleted). Update prerequisite section.
- `.cursor/skills/asterisk-calibrate/SKILL.md` — replace `go build` references with `just build`.

### Phase 3: Rules

- `.cursor/rules/domain/agent-operations.mdc` — update local pipeline table: Build stage is `origami fold` (or `just build`), Unit test stage notes tests live in Origami, Integration stage uses `just calibrate-stub`.
- `.cursor/rules/universal/go-test-conventions.mdc` — add note that this rule applies to Origami only; Asterisk has no `.go` files. Or move to Origami repo.
- `.cursor/rules/domain/rule-router.mdc` — update go-test-conventions routing if the rule is scoped differently.

### Phase 4: Dev scripts

- `.dev/scripts/generate_scenario.py` — remove (generates to nonexistent `internal/calibrate/scenarios/` path).
- `.dev/README.md` — remove `internal/*/testdata/` reference and other stale Go-era content.

## Coverage matrix

| Layer | Applies | Rationale |
|-------|---------|-----------|
| **Unit** | no | No code changes |
| **Integration** | yes | `just build` + `just calibrate-stub` must still work after changes |
| **Contract** | no | No API changes |
| **E2E** | no | No behavior changes |
| **Concurrency** | no | N/A |
| **Security** | no | No trust boundaries |

## Tasks

- [x] Phase 1 — Clean `.gitignore` and `justfile`
- [x] Phase 2 — Update `asterisk-analyze` and `asterisk-calibrate` skills
- [x] Phase 3 — Update `agent-operations.mdc`, `go-test-conventions.mdc`
- [x] Phase 4 — Clean `.dev/` stale scripts (deleted `generate_scenario.py`)
- [x] Validate (green) — `just build` + `just calibrate-stub` PASS 21/21
- [x] Tune (blue) — verified zero `go build` matches outside contracts/docs/README
- [x] Validate (green) — all still works after tuning

## Acceptance criteria

- **Given** a search for `go build` in Asterisk, **when** excluding `.cursor/contracts/completed/` and `.cursor/docs/`, **then** zero matches.
- **Given** a search for `make build` or `make test` in Asterisk, **when** excluding completed contracts, **then** zero matches.
- **Given** `.gitignore`, **when** listing Go-specific entries, **then** only binary output entries remain (no `*.test`, `*.out`, `go.work`).
- **Given** `just build`, **when** run in Asterisk, **then** succeeds via `origami fold`.
- **Given** `just calibrate-stub`, **when** run in Asterisk, **then** PASS 21/21.

## Security assessment

No trust boundaries affected. Documentation and configuration changes only.

## Notes

2026-03-02 00:00 — Contract drafted. Asterisk achieved zero Go; these are the leftovers from the bygone era.

2026-03-02 14:30 — Contract complete. All 4 phases executed. `just build` + `just calibrate-stub` PASS 21/21. Zero `go build` matches in non-contract/non-doc files. `generate_scenario.py` deleted. Skills, rules, .gitignore, justfile all updated.
