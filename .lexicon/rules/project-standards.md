---
id: project-standards
title: Project Standards
description: Asterisk product definition, methodology, scope
labels: [asterisk, domain]
---

# Project Standards

## Product

- Standalone Go CLI (potential sidecar). RP API client. Purpose: automated RCA on RP test failures via source repos, CI circuits, telemetry.
- Design: explainable, evidence-first RCA with confidence scores.

## Methodology

- Start stories with Gherkin (Given/When/Then).
- Red-Orange-Green-Yellow-Blue cycle. Run local circuit after every change.
- Deterministic first. LLM only for genuine reasoning. Stub validates deterministic; dry/wet validates stochastic.
- Every AI inference must cite evidence.

## Safety > Speed

- Outcome correctness first. When accuracy vs speed conflict, accuracy wins.
- Outcome metrics (M1, M15, M10) weighted highest. Efficiency (M16–M18) are health checks.

## Scope

- Focus on current goal. Defer future phases unless re-scoped.

## Origami API

- Breaking changes expected. Update Asterisk immediately; no shims.
- Delete over deprecate. No `// Deprecated:` markers.

## Calibration ordering

1. **Stub** — no LLM. After every code change.
2. **Dry** — LLM + synthetic. During tuning.
3. **Wet** — LLM + production. Only when contracts complete, code refactored, dry passes.

## Data

- External data partial/absent → partial results + clear errors.
- Redact secrets/PII. Timeouts + exponential backoff on external calls.
- `autoAnalyzed=true` on RP issues = low-accuracy; never ground truth. Surface as `[auto]`.

## Persistence

- Storage adapter for cases, RCAs, circuit hierarchy. No raw SQLite in domain/CLI.

## Output

- RCA: summary, suspected components/commits, category + confidence, next actions.
- Every conclusion links to evidence.

## Company

- Red Hat telco QE. Multi-team, multi-operator, multi-CI. Operator/CI logic in adapters, not core.
