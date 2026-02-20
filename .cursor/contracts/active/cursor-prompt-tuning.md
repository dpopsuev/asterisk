# Contract — Cursor Prompt Tuning

**Status:** active  
**Goal:** Improve CursorAdapter accuracy on Phase 5a (18 verified cases) through targeted prompt edits. Measured by M19 delta against baseline (0.50).  
**Serves:** PoC completion (gate: rp-e2e-launch Phase 5a)

## Contract rules

- Prompt-only changes unless a metric requires code.
- Each fix: implement, rebuild, re-run Phase 5a via MCP, measure delta.
- Do NOT modify ground truth.
- Iterative: stop when M19 >= 0.85 or diminishing returns.

## Context

- **Baseline:** Phase 5a (2026-02-18) M19=0.50, 8/21 metrics pass
- **Analysis:** `.cursor/notes/phase-5a-post-run-analysis.md`
- **Gate contract:** `.cursor/contracts/active/rp-e2e-launch.md`

## Tasks

- [ ] Fix F1 taxonomy in `classify-symptoms.md` — align categories with ground truth (product/automation/environment)
- [ ] Add component frequency priors to `deep-rca.md` — linuxptp-daemon 78%, cloud-event-proxy 11%
- [ ] Rebuild, run Phase 5a via MCP (parallel=4), record M19 delta
- [ ] Assess results, apply next fix if needed (convergence tuning, defect type guidance)

## Acceptance criteria

- **Given** the F1 prompt taxonomy is aligned with ground truth categories,
- **When** Phase 5a runs via MCP with the cursor adapter,
- **Then** M2 (symptom category accuracy) improves from 0.00 to >= 0.50.

- **Given** component frequency priors are injected into the F3 prompt,
- **When** Phase 5a runs,
- **Then** M15 (component identification) improves from 0.44 to >= 0.60.

## Notes

- 2026-02-19 — Contract created. Motivated by Phase 5a FAIL (M19=0.50). Top two fixes: taxonomy mismatch (M2=0.00, est. +0.10) and component blindness (M15=0.44, est. +0.05). No existing contract covers cursor-adapter prompt fixes — poc-tuning-loop targets BasicAdapter and activates after gate passes.
