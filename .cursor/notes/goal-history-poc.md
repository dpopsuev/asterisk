---
description: Active goal manifest — single-read orientation for agents to know what matters now
---

# Current Goal

**Goal:** PoC completion (resumed)  
**Summary:** Prove Asterisk can do real blind RCA on RP data with both BasicAdapter (heuristic) and CursorAdapter (AI via MCP). Framework showcase contracts (A, B, C) moved to `github.com/dpopsuev/origami`.

## Asterisk Domain — Urgency Map

| Tier | Contract | Status | Notes |
|------|----------|--------|-------|
| ~~gate~~ | ~~`distillation-endgame`~~ | **complete** | All 3 phases done. Pipeline uses Runner.Walk(), dispatch via origami/dispatch. |
| ~~gate~~ | ~~`asterisk-origami-distillation`~~ | **complete** | Boundary validated. Achilles built. Moved to `completed/distillation/`. |
| **next-milestone** | Origami DSL (5 contracts) | active (C1) | Evolves Origami into declarative pipeline DSL. Asterisk shrinks from ~18k to ~3k lines Go. See `origami/.cursor/contracts/current-goal.mdc`. |

Framework contracts (playground, developer guide, OSS polish, Tomes I-IV, MCP, multi-subagent) moved to `github.com/dpopsuev/origami/.cursor/contracts/`.

---

## Suspended Goal: PoC completion

**Resumes:** Sunday 2026-02-23  
**Reference:** `goals/poc.mdc`  
**Summary:** Prove Asterisk can do real blind RCA on RP data with both BasicAdapter (heuristic) and CursorAdapter (AI via MCP), push results back to ReportPortal, then tune accuracy through targeted quick wins.

## PoC Urgency Map (suspended)

Ordered by priority within each tier. Agents should work top-down: complete gates before quick-wins; quick-wins before should-haves; should-haves before next-milestone. Re-tiered per Phase 5a mitigation urgency audit (2026-02-19).

### MUST — Blocks PoC gate or pitch

| Tier | Contract | Status | Notes |
|------|----------|--------|-------|
| ~~gate~~ | ~~`asterisk-papercup-v2-hardening`~~ | **complete** | Embedded v2 choreography into MCP server + skill. Companion: `origami-papercup-v2-hardening`. Moved to `completed/calibration/`. |
| ~~gate~~ | ~~`asterisk-calibrate-skill`~~ | **complete** | Skill shipped (v2 choreography, 4 workers). Wet validation = Phase 5a run. Moved to `completed/calibration/`. |
| ~~gate~~ | ~~`phase-5a-v2-analysis`~~ | **complete** | CursorAdapter M19=0.58 baseline. Root causes documented. PoC proved. Moved to `completed/calibration/`. |
| ~~gate~~ | ~~`rp-e2e-launch`~~ | **complete** | BasicAdapter M19=0.83. CursorAdapter M19=0.58. Final PoC gate reached. Moved to `completed/calibration/`. |
| ~~gate~~ | ~~`domain-cursor-prompt-tuning`~~ | **abandoned** | Superseded by `phase-5a-v2-analysis`. Moved to `completed/calibration/`. |
| ~~gate~~ | ~~`wet-calibration-tuning`~~ | **complete** | Dry M19: 0.49 → 0.83. Moved to `completed/calibration/`. |
| ~~gate~~ | ~~`m3-m9-tuning`~~ | **complete** | M3: 0.33→0.83 (Phase 3.5 recall inference). M9 superseded by m9-m10-four-pain-points. Moved to `completed/calibration/`. |
| ~~gate~~ | ~~`efficiency-m16-m17-m18-tuning`~~ | **complete** | M16: 0.42→0.67, M17: 8.00→1.00, M18: 91k→55.6k. All passing. Moved to `completed/calibration/`. |
| ~~gate~~ | ~~`m9-m10-four-pain-points`~~ | **complete** | M9: 0.20→1.00, M10: 0.00→1.00. Deterministic hypothesis-based repo routing. Moved to `completed/calibration/`. |
| ~~gate~~ | ~~`poc-scope-precision`~~ | **complete** | All 7 tasks done. Moved to `completed/poc-v2/`. |
| ~~gate~~ | ~~`phase-5a-mitigation`~~ | **complete** | Umbrella closed. All 9 tasks done. Moved to `completed/poc-v2/`. |
| ~~gate~~ | ~~`workspace-mvp`~~ | **complete** | 11 tasks done, 5 tests. Moved to `completed/poc-v2/`. |

### SHOULD — Significant PoC quality, pitch polish

Execution order assessed 2026-02-19. Ordered by: dependencies resolved, metric impact, effort. Work top-down.

| Order | Contract | Status | PoC scope | Notes |
|-------|----------|--------|-----------|-------|
| ~~1st~~ | ~~`improve-human-operator-output`~~ | **complete** | Full | Narration rules in agent-bus.mdc. Moved to `completed/poc-v2/`. |
| ~~2nd~~ | ~~`logging-standardization`~~ | **complete** | Full | All migrated to `log/slog`. Moved to `completed/poc-v2/`. |
| 2nd | `evidence-gap-brief` | draft | Ph1-3 (types + adapter + output) | Foundational "I don't know because X" infrastructure. Enables tuning QW-4. |
| ~~4th~~ | ~~`poc-tuning-loop`~~ | **abandoned** | — | QW-1/2 done by workspace-mvp, QW-3 absorbed by phase-5a-v2-analysis. Moved to `completed/calibration/`. |
| 5th | `ground-truth-dataset` | draft | **Ph1-2 done; Ph7 remaining** (export + expand + PR) | Phases 1-2 implemented (`internal/dataset/`, `cmd_gt.go`). Phases 3-6 deferred. |
| ~~6th~~ | ~~`concurrent-subagents-determinism`~~ | **complete** | — | Moved to Origami repo. |
| ~~7th~~ | ~~`subagent-testing-framework`~~ | **complete** | — | Moved to Origami repo. |
| ~~6th~~ | ~~`agent-adapter-overloading`~~ | **abandoned** | — | Moved to Origami repo. |
| 6th | `knowledge-source-migration` | draft | **Defer** | Phase 1: migrate 11 files to `origami/knowledge`. Phase 2: artifact catalog. Not PoC-blocking. |
| ~~—~~ | ~~`investigate-pipeline-artifact-store`~~ | **complete** | v1 alive (5 callers), no extraction, no split. Investigation-only. Moved to `completed/architecture/`. |
| ~~—~~ | ~~`rename-origami-to-dataset`~~ | **complete** | Renamed `internal/origami/` → `internal/dataset/`. Moved to `completed/architecture/`. |
| ~~—~~ | ~~`deadcode-dedup-architecture`~~ | **complete** | Dead code removed, boundary map produced, stale framework files deleted. Moved to `completed/architecture/`. |
| ~~—~~ | ~~`marshaller-step-schema-factory`~~ | **complete** | Enriched step schemas with FieldDef; calibrate skill updated for submit_step. Moved to `completed/calibration/`. |

### Completed

| Tier | Contract | Status | Notes |
|------|----------|--------|-------|
| ~~quick-win~~ | ~~`data-swipe`~~ | **complete** | Scrubbed. Moved to `completed/poc-v2/`. |
| ~~quick-win~~ | ~~`architecture-snapshots`~~ | **complete** | 11 diagrams across 4 files. Moved to `completed/knowledge-store/`. |
| ~~quick-win~~ | ~~`fsc-knowledge-compartmentalization`~~ | **complete** | 4 compartments, 3 alwaysApply. Moved to `completed/knowledge-store/`. |
| ~~vision~~ | ~~`soul-red-hat-telco`~~ | **complete** | Red Hat identity, telco mission, OSS philosophy. Moved to `completed/knowledge-store/`. |
| ~~SHOULD~~ | ~~`concurrent-subagents-determinism`~~ | **complete** | Moved to Origami repo `completed/poc-v2/`. |
| ~~SHOULD~~ | ~~`subagent-testing-framework`~~ | **complete** | Moved to Origami repo `completed/poc-v2/`. |

### NICE — Architecture evolution, post-PoC

| Tier | Contract | Status | Notes |
|------|----------|--------|-------|
| **vision** | `knowledge-source-evolution` | draft | Layered composition via `Source.Tags`, artifact dependency graph, token-budget summarization. Reframed on `origami/knowledge`. |
| ~~next-milestone~~ | ~~`calibration-primitives-consumer`~~ | **complete** | Import `origami/calibrate` types; type aliases, embedded report, `cal.AggregateRunMetrics`, `cal.FormatReport`. Companion: Origami `calibration-primitives`. Moved to `completed/calibration/`. |
| ~~vision~~ | ~~`domain-calibration`~~ | **moved** | Moved to Origami `draft/domain-calibration.md` — evaluation types and runner belong in the framework. |

Framework contracts (Tomes I-IV, MCP, multi-subagent) moved to `github.com/dpopsuev/origami/.cursor/contracts/completed/`.

## Goal Transition Protocol

When this goal is met (rp-e2e-launch complete + tuning loop stop condition reached):

1. Archive this manifest in `notes/` as `goal-history-poc.md`
2. Create a new `current-goal.mdc` for the next goal (likely: MCP integration)
3. Reassess all draft contracts — re-tier relative to the new goal
4. Move completed contracts to `completed/` with appropriate phase subdirectory
