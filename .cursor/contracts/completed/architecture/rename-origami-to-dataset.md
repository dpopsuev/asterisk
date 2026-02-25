# Contract — rename-origami-to-dataset

**Status:** complete  
**Goal:** Rename `internal/origami/` to `internal/dataset/` so the package name reflects its purpose (ground truth dataset adapter) instead of leaking the framework name into the consumer.  
**Serves:** PoC completion

## Contract rules

- Mechanical rename only — no behavior changes, no new features.
- All references (imports, docs, contracts, diagrams) must be updated in the same pass.
- Absorbs task D10 from `ground-truth-dataset` contract.

## Context

`internal/origami/` (6 files, ~318 lines) is a domain adapter that bridges `calibrate.Scenario` to `curate.Record` via the Origami `curate/` primitives. The package name `origami` is misleading — it suggests framework code but contains only Asterisk-specific ground truth logic (`DatasetStore`, `FileStore`, `AsteriskSchema`, `CheckCase`, mappers).

The `ground-truth-dataset` contract (task D10) and the `deadcode-dedup-architecture` boundary map both identified this rename as needed. This contract extracts it as a standalone cleanup to unblock independently.

### Scope

| File | Change |
|------|--------|
| `internal/origami/*.go` (6 files) | Move to `internal/dataset/`, change `package origami` → `package dataset` |
| `cmd/asterisk/cmd_gt.go` | Import `asterisk/internal/dataset`, update `origami.` → `dataset.` call sites |
| `CONTRIBUTING.md` | Update package listing |
| `.cursor/contracts/draft/ground-truth-dataset.md` | Update references, mark D10 complete |
| `.cursor/contracts/current-goal.mdc` | Update reference |
| `.cursor/docs/distillation-manifest.md` | Update package name |

## FSC artifacts

Code only — no FSC artifacts.

## Execution strategy

Single pass: rename directory, update all package declarations, update all imports, update all docs. Build + test.

## Coverage matrix

| Layer | Applies | Rationale |
|-------|---------|-----------|
| **Unit** | yes | Existing tests in `internal/dataset/` must pass after rename |
| **Integration** | no | No cross-boundary changes |
| **Contract** | no | Internal package |
| **E2E** | no | No behavior change |
| **Concurrency** | no | No shared state |
| **Security** | no | No trust boundaries affected |

## Tasks

- [ ] Rename `internal/origami/` directory to `internal/dataset/`
- [ ] Change `package origami` to `package dataset` in all 6 files
- [ ] Update import in `cmd/asterisk/cmd_gt.go` from `asterisk/internal/origami` to `asterisk/internal/dataset`; update `origami.` call sites to `dataset.`
- [ ] Update `CONTRIBUTING.md` (line 89): `internal/origami/` → `internal/dataset/`
- [ ] Update `.cursor/docs/distillation-manifest.md` (line 31): `internal/origami/` → `internal/dataset/`
- [ ] Update `.cursor/contracts/draft/ground-truth-dataset.md`: all references + mark D10 complete
- [ ] Update `.cursor/contracts/current-goal.mdc`: reference in ground-truth-dataset notes
- [ ] Validate (green) — `go build ./...` and `go test ./...` pass
- [ ] Tune (blue) — verify no stale `origami` references remain (grep)
- [ ] Validate (green) — all tests still pass after tuning

## Acceptance criteria

- **Given** the rename is applied, **when** `rg "internal/origami" --type go` is run, **then** zero matches.
- **Given** the rename is applied, **when** `go build ./...` and `go test ./...` run, **then** all pass.
- **Given** an agent reads `internal/dataset/`, **when** it sees the package name, **then** it understands this is ground truth dataset logic, not Origami framework code.

## Security assessment

No trust boundaries affected.

## Notes

2026-02-25 — Contract created. Extracted from `ground-truth-dataset` task D10. Standalone cleanup to improve naming clarity. The user chose `dataset` over `groundtruth` as the target name.
