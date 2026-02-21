# Contract — Agentic Framework III.3: Shadow

**Status:** complete
**Goal:** Define the adversarial pipeline pattern where Light and Shadow agents contest uncertain cases, using the Wuxing destructive cycle for challenger selection. Absorb scope of `defect-court.md`.
**Serves:** Architecture evolution (Framework identity)

## Contract rules

- The Shadow path is **optional** and **additive** -- it activates only for uncertain cases and does not modify the Light path (F0-F6) behavior.
- Role separation is mandatory: prosecution, defense, and judge must use different agent identities with different Alignments.
- Fast-track (plea deal) must exist so easy cases bypass full adversarial review.
- Challenger selection uses the destructive cycle from II.2-cycles: the Shadow agent's element challenges the Light agent's element.
- The Shadow path reuses the Framework ontology (Nodes, Edges, Walkers, Graph) -- it IS a second pipeline definition, not a special case.
- This contract absorbs the scope of `defect-court.md`. All D0-D4 steps, heuristics, and artifact types are preserved.
- Inspired by: Warhammer (Loyal vs Traitor Primarchs), Inside Out (Joy needs Sadness), Jungian Shadow (the hidden self), Wuxing destructive cycle (ke).

## Context

- `contracts/draft/defect-court.md` -- defines D0-D4 pipeline, HD1-HD12 heuristics, prosecution/defense/judge roles, remand feedback, TTL/handoff limits. All absorbed here.
- `contracts/draft/agentic-framework-I.1-ontology.md` -- Node, Edge, Walker, Graph interfaces.
- `contracts/draft/agentic-framework-I.2-characteristica.md` -- DSL for declaring pipelines (the Shadow path IS a second pipeline).
- `contracts/draft/agentic-framework-II.2-cycles.md` -- destructive cycle for challenger selection.
- `contracts/draft/agentic-framework-III.1-personae.md` -- Shadow personas (Challenger, Abyss, Bulwark, Specter).
- `contracts/draft/agentic-framework-III.2-masks.md` -- Shadow masks (Indictment, Discovery).
- Plan reference: `agentic_framework_contracts_2daf3e14.plan.md` -- Tome III: Personae.

## Architecture

### Pipeline paths

```
Light path: F0 -> F1 -> F2 -> F3 -> F4 -> F5 -> F6
Shadow path:                   D0 -> D1 -> D2 -> D3
                               (indict, discover, defend, hear, verdict)
```

The Shadow path activates when a Light agent's F5 Review confidence is in the "uncertain" range -- neither high enough to accept nor low enough to reject. This is the Wuxing destructive cycle in action: Shadow agents tear down weak conclusions so Light agents can rebuild stronger ones.

### Shadow persona to court role mapping

| Shadow Persona | Court Role | Element | Destructive Target |
|----------------|------------|---------|-------------------|
| Challenger (Scarlet) | Prosecutor | Fire | Challenges Water (deep evidence) |
| Abyss (Sapphire) | Devil's Advocate | Water | Challenges Earth (stable conclusions) |
| Bulwark (Iron) | Forensic Expert | Diamond | Challenges Air (vague synthesis) |
| Specter (Obsidian) | Summary Judgment | Lightning | Challenges Diamond (precise but brittle) |

### Activation trigger

The Shadow path activates based on the Light path's confidence:
- Confidence >= 0.85: affirm (no Shadow needed)
- 0.50 <= Confidence < 0.85: UNCERTAIN -- activate Shadow
- Confidence < 0.50: reject (Light agent already uncertain)

### Shadow pipeline definition (DSL)

```yaml
pipeline: defect-court
description: "D0-D4 adversarial court pipeline"

zones:
  prosecution:
    nodes: [indict]
    element: fire
    stickiness: 0
  discovery:
    nodes: [discover, defend]
    element: water
    stickiness: 2
  verdict:
    nodes: [hearing, verdict]
    element: diamond
    stickiness: 3

nodes:
  - name: indict
    element: fire
    family: indict
  - name: discover
    element: earth
    family: discover
  - name: defend
    element: water
    family: defend
  - name: hearing
    element: air
    family: hearing
  - name: verdict
    element: diamond
    family: verdict

edges:
  - id: HD1
    name: fast-track
    from: indict
    to: defend
    shortcut: true
    condition: "prosecution confidence >= 0.95"
  - id: HD2
    name: plea-deal
    from: defend
    to: verdict
    shortcut: true
    condition: "defense concedes"
  - id: HD3
    name: motion-to-dismiss
    from: defend
    to: hearing
    condition: "defense has motion to dismiss"
  - id: HD4
    name: alternative-hypothesis
    from: defend
    to: hearing
    condition: "defense has alternative hypothesis"
  - id: HD5
    name: hearing-complete
    from: hearing
    to: verdict
    condition: "max rounds reached OR convergence"
  - id: HD6
    name: affirm
    from: verdict
    to: _done
    condition: "verdict == affirm"
  - id: HD7
    name: amend
    from: verdict
    to: _done
    condition: "verdict == amend"
  - id: HD8
    name: remand
    from: verdict
    to: _remand
    loop: true
    condition: "verdict == remand AND remands < max_remands"
  - id: HD9
    name: acquit
    from: verdict
    to: _gap_brief
    condition: "verdict == acquit"
  - id: HD10
    name: ttl-exceeded
    from: _any
    to: _mistrial
    condition: "wall-clock TTL exceeded"
  - id: HD11
    name: handoff-exceeded
    from: _any
    to: _mistrial
    condition: "handoff counter exceeded"
  - id: HD12
    name: judge-mistrial
    from: verdict
    to: _mistrial
    condition: "verdict == mistrial"

start: indict
done: _done
```

### New artifact types (from defect-court.md)

- **Indictment** (D0) -- charged defect type, prosecution narrative, itemized evidence with weights
- **DefenseBrief** (D2) -- challenges to evidence items, alternative hypothesis, plea deal flag
- **HearingRecord** (D3) -- rounds of prosecution argument + defense rebuttal + judge notes
- **Verdict** (D4) -- decision (affirm/amend/acquit/remand), final classification, confidence, reasoning

### Remand feedback (structured, not blind)

A court remand provides structured feedback to the Light path's F2/F3:
- Which evidence items were challenged and why
- The defense's alternative hypothesis with supporting evidence
- Specific questions the reinvestigation must address

### TTL and handoff limits

```go
type CourtConfig struct {
    Enabled     bool          `json:"enabled"`
    TTL         time.Duration `json:"ttl"`
    MaxHandoffs int           `json:"max_handoffs"`
    MaxRemands  int           `json:"max_remands"`
    ActivationThreshold float64 `json:"activation_threshold"` // confidence below which Shadow activates
}
```

## Execution strategy

Four phases matching `defect-court.md` but reframed as Framework implementations:

### Phase 1 -- Data structures and plumbing (Framework types)
Define court artifact types and heuristic rules using Framework ontology.

### Phase 2 -- BasicAdapter court roles (heuristic baseline)
Implement Shadow personas as BasicAdapter variants with specialized heuristics.

### Phase 3 -- Calibration metrics for court
Add verdict flip rate, defense challenge accuracy, remand effectiveness.

### Phase 4 -- LLM-based court (requires MCP)
Shadow personas with distinct system prompts via MCP.

## Tasks

### Phase 1 -- Framework types

- [x] Define court artifact types: `Indictment`, `DefenseBrief`, `HearingRecord`, `Verdict`
- [x] Define `CourtConfig` with TTL, MaxHandoffs, MaxRemands, ActivationThreshold
- [x] Create `pipelines/defect-court.yaml` -- D0-D4 pipeline in Framework DSL (created in I.2)
- [x] Define court Edge evaluators (HD1-HD12) as Framework Edge implementations
- [x] Define `CourtEvidenceGap` extending shared `EvidenceGap` type
- [x] Wire Shadow activation trigger: Light F5 confidence in uncertain range (`ShouldActivate`)

### Phase 2 -- BasicAdapter court roles

- [x] Implement Challenger persona as prosecution heuristic adapter
- [x] Implement Abyss persona as defense heuristic adapter
- [x] Implement Bulwark persona as forensic expert heuristic adapter
- [x] Implement Specter persona as summary judgment adapter
- [x] Wire Shadow pipeline into calibrate runner as post-F6 phase

### Phase 3 -- Metrics

- [x] Add verdict flip rate metric
- [x] Add defense challenge accuracy metric (verdict accuracy against ground truth)
- [x] Add remand effectiveness metric
- [x] Add `ExpectedVerdict` to `GroundTruthCase`

### Phase 4 -- LLM-based court (future)

- [x] Prosecution, defense, and judge system prompts
- [x] Multi-round hearing with structured JSON exchange
- [x] Remand feedback integration with Light F2/F3

### Validation

- [x] Validate (green) -- `go build ./...`, all tests pass, Light pipeline unchanged
- [x] Tune (blue) -- review activation thresholds, court heuristic rules
- [x] Validate (green) -- all tests still pass after tuning

## Acceptance criteria

- **Given** the Light pipeline produces a classification with confidence 0.65 (uncertain range),
- **When** the Shadow path activates,
- **Then** Shadow agents (Challenger, Abyss) contest the classification using the destructive cycle.

- **Given** a case where prosecution confidence is >= 0.95,
- **When** defense evaluates the case,
- **Then** the plea deal fast-track (HD2) skips to verdict with near-zero overhead.

- **Given** a court remand back to F2/F3,
- **When** the Light path reinvestigates with structured feedback,
- **Then** the second-pass classification addresses the defense's specific challenges.

- **Given** the court's TTL or handoff counter is exceeded,
- **When** the case cannot reach a verdict,
- **Then** a mistrial is declared with an Evidence Gap Brief.

- **Given** the Shadow pipeline YAML definition,
- **When** it is loaded into the Framework DSL,
- **Then** it produces a valid Graph with 5 nodes and 12 edges.

## Security assessment

| OWASP | Finding | Mitigation |
|-------|---------|------------|
| A03 | Prosecution, defense, and judge exchange structured data. Prompt injection between roles is possible. | All inter-role data is treated as untrusted. Structured JSON only. Validate schema before injection. |
| A04 | TTL and handoff limits prevent DoS from buggy defense adapter forcing infinite remand loops. | Mitigated by HD10-HD12. Deliberate security design. |
| A08 | Court artifacts are unsigned. Tampered verdict could alter RCA. | Acceptable risk for PoC. MVP: add artifact HMAC signing. |

## Notes

- 2026-02-22 -- Phase 1 complete: added BuildCourtEdgeFactory with HD1-HD12 court edge evaluators, CourtEvidenceGap type, updated defect-court.yaml to HD1-HD12 IDs. Fixed Artifact interface compliance: renamed Confidence struct fields to ConfidenceScore to resolve method/field clash (Confidence_() -> Confidence()). 15 court tests passing, compile-time interface assertions added. All Phase 1 tasks done.
- 2026-02-21 20:30 -- Phase 1 partial: court artifact types (Indictment, DefenseBrief, HearingRecord, Verdict), CourtConfig with ShouldActivate, VerdictDecision constants, RemandFeedback. 9 court tests passing. Edge evaluators (HD1-HD12) and CourtEvidenceGap deferred to Phase 1 completion. Phases 2-4 require adapter/calibration/LLM integration outside framework layer. Contract moved to active.
- 2026-02-20 -- Contract created. Absorbs `defect-court.md` scope. Reframed as a Framework pipeline instance (second Graph) rather than a special extension to F0-F6. Shadow personas (from III.1) serve as court roles. Destructive cycle (from II.2) determines challenger selection. Court pipeline expressed in Framework DSL (from I.2).
- The key insight from Jung: the Shadow is not evil -- it is the hidden, unacknowledged aspect that must be integrated for wholeness. Similarly, Shadow agents are not trying to break the pipeline -- they are trying to make it honest.
- Depends on I.1-ontology, I.2-characteristica, II.2-cycles, III.1-personae, III.2-masks.
