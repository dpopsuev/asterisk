# Contract — yaml-playbook-docs

**Status:** draft  
**Goal:** Rewrite README.md, CONTRIBUTING.md, and architecture docs to reflect Asterisk's YAML-only playbook identity.  
**Serves:** 100% DSL — Zero Go; onboarding; external visibility

## Contract rules

- README must position Asterisk as a YAML playbook (the "Ansible for RCA" pitch), not a Go project.
- No broken links — every reference must resolve to an existing file.
- Architecture diagrams must match the current repo structure (zero Go, circuits/, scorecards/, origami.yaml).

## Context

Asterisk achieved 100% YAML — zero `.go` files. But `README.md` still says `go build -o bin/asterisk ./cmd/asterisk/` and `CONTRIBUTING.md` lists `cmd/`, `internal/`, `adapters/` as the project structure. `.cursor/docs/architecture.md` describes the old Go layout. A visitor to the repo would be thoroughly confused.

The Ansible analogy is the guiding metaphor: there is no Python in an Ansible repo — only YAML playbooks. The Python lives in Ansible (the engine) and in collections (the plugins). Asterisk follows the same model.

Depends on `housekeeping` contract — stale refs should be cleaned first so docs don't reference things that are about to be removed.

## FSC artifacts

| Artifact | Target | Compartment |
|----------|--------|-------------|
| Architecture diagram (Mermaid) | `.cursor/docs/architecture.md` | domain |

## Execution strategy

### Phase 1: README.md rewrite

- Position as YAML-only playbook for evidence-based RCA
- Quick start: install `origami` CLI, `just build`, set RP credentials, `bin/asterisk analyze`
- Prerequisites: `origami` CLI (installed via `just install` in Origami repo), `just` task runner
- Project structure: `circuits/`, `scorecards/`, `examples/`, `origami.yaml`, `.cursor/`
- Fix paths: `circuits/` not `pipelines/`
- Remove: `make` references, `go build`, Go 1.24+ prerequisite
- Remove: broken links to `docs/framework-guide.md` and `README.md.post`
- Keep: analyze example with `examples/pre-investigation-33195-4.21/`, calibration example, Cursor skill reference, pipeline diagram (F0-F6), framework overview

### Phase 2: CONTRIBUTING.md rewrite

- Prerequisites: `origami` CLI, `just`
- Build: `just build` (runs `origami fold`)
- Tests: "All tests live in Origami — run `go test ./...` in the Origami repo"
- Lint: `origami lint --profile strict` on circuit YAMLs
- Project structure: current layout (circuits/, scorecards/, examples/, .cursor/)
- Keep: conventional commits, Red-Orange-Green-Yellow-Blue cycle, commit convention

### Phase 3: Architecture docs rewrite

- `.cursor/docs/architecture.md` — rewrite for YAML-only era
- Remove all `cmd/`, `internal/`, `adapters/` references
- Mermaid diagram: Asterisk (YAML) <-> Origami (Go engine + modules) relationship
- Reference the end-state architecture from `current-goal.mdc`
- Document the Ansible model: playbook repo (Asterisk) vs engine repo (Origami) vs module repo (also Origami, `modules/rca/`)

## Coverage matrix

| Layer | Applies | Rationale |
|-------|---------|-----------|
| **Unit** | no | Documentation only |
| **Integration** | no | No code changes |
| **Contract** | no | No API changes |
| **E2E** | no | No behavior changes |
| **Concurrency** | no | N/A |
| **Security** | no | No trust boundaries |

## Tasks

- [ ] Phase 1 — Rewrite README.md
- [ ] Phase 2 — Rewrite CONTRIBUTING.md
- [ ] Phase 3 — Rewrite `.cursor/docs/architecture.md`
- [ ] Validate — all links resolve, no broken references, `just build` command documented correctly
- [ ] Tune (blue) — polish language, ensure Ansible analogy is consistent
- [ ] Validate (green) — final review for broken links

## Acceptance criteria

- **Given** a new developer visiting the repo, **when** reading README.md, **then** they understand Asterisk is YAML-only, know how to build (`just build`), and can run their first analysis.
- **Given** README.md, **when** checking all links, **then** zero broken links.
- **Given** CONTRIBUTING.md, **when** reading the project structure section, **then** it matches the actual directory layout (no `cmd/`, `internal/`, `adapters/`).
- **Given** `.cursor/docs/architecture.md`, **when** viewing the Mermaid diagram, **then** it shows Asterisk as YAML-only with Origami as the Go engine.
- **Given** a search for `make build` or `go build` in README.md or CONTRIBUTING.md, **then** zero matches.

## Security assessment

No trust boundaries affected. Documentation only.

## Notes

2026-03-02 00:00 — Contract drafted. The repo's public face must match its reality: Asterisk is a YAML playbook, not a Go project.
