# Contract — Architecture Snapshots

**Status:** complete  
**Goal:** Architecture diagrams from Plan Mode are preserved as first-class artifacts in the knowledge store, with current/desired state snapshots in contracts and architectural integrity validation on every contract lifecycle transition.  
**Serves:** PoC completion

## Contract rules

- `rules/architecture-snapshots.mdc` defines the protocol. This contract implements it.
- Retrofitting existing contracts is additive — no existing content is removed or reworded, only new sections are added.
- `docs/architecture.md` is the canonical current-state reference. All `Current Architecture` sections in contracts must be consistent with it.

## Context

- `rules/architecture-snapshots.mdc` — the new rule defining the snapshot protocol, plan-to-contract transfer, diagram taxonomy, and integrity checks.
- `docs/architecture.md` — existing canonical architecture doc with one mermaid dependency graph. Needs extension with component interaction diagrams.
- `contracts/draft/agent-adapter-overloading.md` — describes three-layer architecture (AdapterPool, Scheduler, ResultCollector) in prose; plan file had two mermaid diagrams that were not carried forward.
- `contracts/draft/defect-court.md` — describes D0-D4 adversarial flow with tables and prose; no diagrams.
- `contracts/draft/workspace-revisited.md` — describes artifact taxonomy and resolution chain in prose; no diagrams.
- 933 plan files in `~/.cursor/plans/` — ephemeral, unversioned, accumulating noise. Architecture content there is at risk of loss.

## Execution strategy

1. Create the rule (done — `architecture-snapshots.mdc`).
2. Update the knowledge-store persistence protocol to include architecture diagrams as a trigger.
3. Retrofit the three draft contracts that need Current/Desired Architecture sections.
4. Extend `docs/architecture.md` with component interaction diagrams beyond the existing dependency graph.
5. Validate: every architecture-touching draft contract has both sections with mermaid diagrams; `docs/architecture.md` is consistent with all `Current Architecture` references.

## Tasks

- [x] Create `rules/architecture-snapshots.mdc` — snapshot protocol, plan-to-contract transfer, integrity checks, diagram taxonomy
- [x] Update `rules/knowledge-store.mdc` — add architecture diagrams as persistence trigger
- [x] Update indexes — `rules/index.mdc`, `contracts/index.mdc`, `contracts/current-goal.mdc`
- [x] Retrofit `agent-adapter-overloading.md` — added Current Architecture (single-adapter path, 1 flowchart) and Desired Architecture (three-layer routing, 1 flowchart)
- [x] Retrofit `defect-court.md` — added Current Architecture (linear F0-F6, 1 flowchart) and Desired Architecture (D0-D4 court phase, 1 flowchart + 1 sequenceDiagram for role interaction)
- [x] Retrofit `workspace-revisited.md` — added Current Architecture (flat repo list, 1 flowchart) and Desired Architecture (layered catalog with resolution chain, 1 flowchart)
- [x] Extend `docs/architecture.md` — added 3 diagrams: pipeline flow (flowchart), adapter dispatch (sequenceDiagram), MCP session lifecycle (sequenceDiagram). Total now 4 mermaid diagrams.
- [x] Validate — all 3 contracts have Current/Desired sections with mermaid diagrams; `docs/architecture.md` has 4 diagrams; pipeline flow in architecture.md matches F0-F6 referenced in contract Current Architecture sections
- [x] Tune — 11 diagrams across 4 files reviewed; taxonomy-compliant (flowchart, sequenceDiagram); no redundancy
- [x] Validate — all diagrams present, no orphaned references, 21/21 cross-contract checks pass

## Acceptance criteria

- **Given** the rule `architecture-snapshots.mdc` is active,
- **When** a new contract is created that modifies component boundaries,
- **Then** it MUST include `## Current Architecture` and `## Desired Architecture` sections, each with at least one mermaid diagram.

- **Given** the three retrofitted contracts (`agent-adapter-overloading`, `defect-court`, `workspace-revisited`),
- **When** an agent reads any of them,
- **Then** it can understand the architectural delta from diagrams alone, without reading prose.

- **Given** `docs/architecture.md` is updated,
- **When** a new contract references `Current Architecture`,
- **Then** its diagram is consistent with the canonical doc (no drift).

- **Given** a Plan Mode conversation that produces mermaid diagrams,
- **When** the plan becomes a contract,
- **Then** all architecture diagrams are carried into the contract (not left in the plan file).

## Notes

- 2026-02-19 02:00 — Retrofit complete. All 3 contracts now have Current/Desired Architecture sections with mermaid diagrams. `docs/architecture.md` extended with 3 new diagrams (pipeline flow, adapter dispatch, MCP session). Total: 7 new mermaid diagrams across 4 files + 1 existing. Diagram count before: 1. After: 11.
- 2026-02-19 01:30 — Contract created. Rule `architecture-snapshots.mdc` written. Knowledge-store persistence protocol updated. Three draft contracts identified for retrofit: `agent-adapter-overloading`, `defect-court`, `workspace-revisited`. 933 plan files in `~/.cursor/plans/` identified as the source of architectural knowledge loss.
- 2026-02-19 03:30 — Contract closed. All validation checks pass. 11 mermaid diagrams across 4 files, taxonomy-compliant, no orphaned references. Moved to completed/knowledge-store/.
