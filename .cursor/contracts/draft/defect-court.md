# Contract — Defect Court

**Status:** draft  
**Goal:** Extend the F0-F6 investigation pipeline with an adversarial "Defect Court" phase (D0-D4) where a prosecution brief, defense consul, and judicial verdict replace the single-perspective review, feeding contested findings back to investigation for reassessment. Future phase — post-MCP integration.

## Contract rules

- Global rules only.
- This is a **future-phase** contract. Do not implement until MCP integration is operational.
- The court phase is **optional** and **additive** — it follows F6 and does not modify F0-F6 behavior.
- Fast-track (plea deal) must exist so easy cases bypass full adversarial review.
- Role separation is mandatory: prosecution, defense, and judge must use different adapter instances with different objectives.

## Context

- **Current pipeline:** F0-F6 (Recall, Triage, Resolve, Investigate, Correlate, Review, Report)
- **Current weakness:** Single-perspective analysis with confirmation bias. F5 Review is a rubber stamp (BasicAdapter always approves). No alternative hypotheses are considered.
- **Inspiration:** US federal criminal trial process — investigation, indictment, discovery, defense preparation, hearing, verdict, appeal/remand.
- **Baseline accuracy:** BasicAdapter M19 = 0.93 on ptp-real-ingest. The ~7% error rate is concentrated in ambiguous cases (product vs automation, component A vs component B).

## Metaphor mapping

The current F0-F6 pipeline is the **police and prosecutor** — it investigates, gathers evidence, and charges the defect. The court phase adds:

- **D0 Indictment** — Package the prosecution's case (from F6 output) into a formal charge with itemized evidence
- **D1 Discovery** — Make all raw failure data available to the defense (same data the prosecution saw, NOT the prosecution's conclusions)
- **D2 Defense** — Defense consul independently analyzes raw data, then reviews prosecution case, builds challenges and alternative hypotheses
- **D3 Hearing** — Structured adversarial exchange: prosecution argues, defense rebuts, possibly multiple rounds
- **D4 Verdict** — Judge weighs both sides, issues ruling (affirm, amend, acquit, or remand)

The **remand** path is the key feedback loop: if the judge finds the evidence insufficient, the case goes back to F2/F3 with the defense's specific challenges as structured feedback, transforming blind retry into targeted inquiry.

The **plea deal** fast-path handles easy cases: when prosecution confidence is very high and the defense concedes, skip directly to verdict.

## Architecture

### New pipeline steps

```
StepD0Indict   = "D0_INDICT"
StepD1Discover = "D1_DISCOVER"
StepD2Defend   = "D2_DEFEND"
StepD3Hearing  = "D3_HEARING"
StepD4Verdict  = "D4_VERDICT"
```

### New artifact types

- **Indictment** (D0) — charged defect type, component, prosecution narrative, itemized evidence with weights (primary/corroborating/circumstantial), prosecution confidence
- **DefenseBrief** (D2) — challenges to specific evidence items (with severity: fatal/weakening/minor), alternative hypothesis (if any), mitigating factors, plea deal flag, motion to dismiss flag
- **HearingRecord** (D3) — rounds of prosecution argument + defense rebuttal + judge notes
- **Verdict** (D4) — decision (affirm/amend/acquit/remand), final defect type, final component, confidence, reasoning, dissent (if multiple judges), remand reason and target step

### New heuristic rules

| Rule | Stage | Condition | Action |
|------|-------|-----------|--------|
| HD1 | D0 | Prosecution confidence >= 0.95 | Fast-track to D2 (defense may plea-deal) |
| HD2 | D2 | Defense plea-deal (concedes) | Skip to D4 with affirm |
| HD3 | D2 | Defense motion to dismiss | D3 Hearing (prosecution must respond) |
| HD4 | D2 | Defense has alternative hypothesis | D3 Hearing |
| HD5 | D3 | Hearing complete (max rounds or convergence) | D4 Verdict |
| HD6 | D4 | Verdict = affirm | Done (prosecution's classification stands) |
| HD7 | D4 | Verdict = amend | Done (judge's amended classification) |
| HD8 | D4 | Verdict = remand | Back to F2/F3 with defense feedback |
| HD9 | D4 | Verdict = acquit | Done (mark unclassified, needs human) |

### Multi-adapter architecture

The court requires role-separated adapters:

- **ProsecutionAdapter** — the existing pipeline adapter (reused); presents the case
- **DefenseAdapter** — separate adapter with adversarial objective; challenges the prosecution
- **JudgeAdapter** — separate adapter with neutral objective; weighs both sides

For BasicAdapter: `BasicDefenseAdapter` (skeptical keyword rules) and `BasicJudgeAdapter` (confidence comparison). For MCP/CursorAdapter: different system prompts per role.

### Remand feedback (structured, not blind)

The existing F5 reassess loop says "try again." A court remand provides structured feedback:

- Which evidence items were challenged and why
- The defense's alternative hypothesis with supporting evidence
- Specific questions the reinvestigation must address

This transforms reinvestigation from blind retry to targeted inquiry.

## Benefit assessment

**Real benefit, not just novelty.** Adversarial systems are proven in AI alignment (Constitutional AI, debate-based reasoning, red-teaming). The ~7% error rate is concentrated in ambiguous cases — exactly where adversarial review helps most. The plea-deal mechanism keeps overhead proportional to difficulty.

Estimated impact:
- BasicAdapter (heuristic): modest improvement (M19 0.93 -> 0.94-0.95)
- LLM-based adapters (MCP): significant improvement (M19 -> 0.96-0.98)
- Cost: 2-3x tokens for contested cases, near-zero for plea-deal cases

## Implementation phases

### Phase 1 — Data structures and plumbing
- [ ] Add D0-D4 step constants and artifact types to `internal/orchestrate/types.go`
- [ ] Add court heuristic rules (HD1-HD9) to `internal/orchestrate/heuristics.go`
- [ ] Extend state machine for D0-D4 transitions and remand paths
- [ ] Add `CourtConfig` to `internal/calibrate/types.go`

### Phase 2 — BasicAdapter court roles (heuristic baseline)
- [ ] Implement `BasicDefenseAdapter` (skeptical keyword rules)
- [ ] Implement `BasicJudgeAdapter` (confidence comparison)
- [ ] Wire into calibrate runner as post-F6 phase

### Phase 3 — Calibration metrics for court
- [ ] New metrics: verdict flip rate, defense challenge accuracy, remand effectiveness
- [ ] Ground truth extension: `ExpectedVerdict` on `GroundTruthCase`

### Phase 4 — LLM-based court (requires MCP)
- [ ] Prosecution, defense, and judge system prompts
- [ ] Multi-round hearing with structured JSON exchange
- [ ] Remand feedback integration with F2/F3

## Acceptance criteria

- **Given** the F0-F6 pipeline produces a defect classification with low confidence,
- **When** the Defect Court phase (D0-D4) runs with adversarial defense and judicial review,
- **Then** the final verdict has higher accuracy than the prosecution's original classification.

- **Given** a case where the prosecution correctly classifies the defect with high confidence,
- **When** the Defect Court fast-tracks via plea deal,
- **Then** overhead is near-zero (no full hearing).

- **Given** a court remand back to F2/F3,
- **When** the reinvestigation runs with the defense's structured feedback,
- **Then** the second-pass classification addresses the defense's specific challenges.

## Dependencies

| Contract | Status | Required for |
|----------|--------|--------------|
| `mcp-server-foundation.md` | Pending | LLM-based adapters need MCP transport |
| `mcp-pipeline-tools.md` | Pending | Court phase reuses pipeline tool infrastructure |
| `rp-e2e-launch.md` | Active | PoC gate must pass before court phase |

## Files affected

- `internal/orchestrate/types.go` — new step constants and artifact types
- `internal/orchestrate/heuristics.go` — new court heuristic rules (HD1-HD9)
- `internal/orchestrate/state.go` — state machine extension for D0-D4
- `internal/calibrate/types.go` — CourtConfig, court-related CaseResult fields
- `internal/calibrate/runner.go` — post-F6 court phase execution
- `internal/calibrate/metrics.go` — new court metrics
- `internal/calibrate/adapt/` — new defense and judge adapter implementations
- New: `internal/court/` — court-specific orchestration logic

## Notes

(Running log, newest first.)

- 2026-02-16 — Contract drafted. Adversarial RCA inspired by US federal trial process. Assessed as real benefit (not just novelty) based on proven adversarial techniques in AI alignment and the concentration of errors in ambiguous cases. Deferred to post-MCP phase.
