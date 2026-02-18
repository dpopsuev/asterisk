# Test Cards Assessment — Data Card

## Dataset identity

| Field | Value |
|-------|-------|
| **Name** | Asterisk Calibration Ground Truth |
| **Version** | 2026-02-18 |
| **Format** | Embedded Go structs (`GroundTruthCase`, `GroundTruthRCA`) |
| **Domain** | PTP/timing subsystem CI test failure root-cause analysis |
| **Source** | CI results (Jan 2025 – Feb 2026), Jira bugs, fix PRs |
| **Verification** | PR-proven (FixPRs + SmokingGun) for verified cases; Jira-only or disputed for candidates |

## Scenarios

| Scenario | RCAs | Symptoms | Cases | Candidates | Source |
|----------|------|----------|-------|------------|--------|
| ptp-mock | 3 | 4 | 12 | 0 | Synthetic (hand-written) |
| daemon-mock | 2 | 3 | 8 | 0 | Synthetic (hand-written) |
| ptp-real | 2 | 3 | 8 | 0 | Real bugs (embedded) |
| ptp-real-ingest | 18 verified + 12 candidate RCAs | 30 | 18 | 12 | Generated from CI data |

## Ground truth structure

Each case links to an RCA via `RCAID`. The RCA is the source of truth for:
- `DefectType` (e.g. pb001, au001, si001)
- `Component` (e.g. linuxptp-daemon, cnf-gotests)
- `Category` (product, automation, infra, firmware, environment)
- `RequiredKeywords` (semantic matching for M14)
- `SmokingGun` (specific phrase from fix PR for M14b)
- `RelevantRepos` (for repo selection metrics M9-M11)

Cases additionally carry per-step expected outcomes (F0-F6), metric flags, and optional RP source references.

## Verification model

Cases are either **verified** or **candidate**:

| Status | Criteria | Scored? |
|--------|----------|---------|
| **Verified** | RCA has `Verified: true`. Evidence: merged FixPR + SmokingGun phrase, or synthetic ground truth. | Yes — always included in M1-M20 scoring |
| **Candidate** | RCA has `Verified: false`. Evidence: Jira ticket exists but no fix PR, or disputed root cause. | No — tracked for dataset growth only |

Candidates are visible in the calibration report's dataset health summary. When a candidate gains a fix PR and smoking gun, it is promoted to verified by moving it from `Scenario.Candidates` to `Scenario.Cases`.

## Industry comparison

| Industry term | Asterisk equivalent | Status |
|---|---|---|
| Golden dataset / Gold set | Verified cases in `Scenario.Cases` | Strong |
| Benchmark dataset | `Scenario` + `MetricSet` (M1-M20+M14b) | Strong |
| Model Card (Google 2019) | `CalibrationReport` | Partial |
| Data Card (Google) | This document | New |
| Eval Factsheet (2024) | Contracts + this document | Partial |
| Data Readiness Level | `Verified` bool (binary: ready or candidate) | Simplified |

## Metrics (20 + M14b)

Scored against verified cases only:

| Group | Metrics | What they measure |
|-------|---------|-------------------|
| Structured (M1-M8) | defect_type, symptom_category, recall, FP rate, serial killer, skip, cascade, convergence | Classification accuracy |
| Workspace (M9-M11) | repo precision, repo recall, red herring rejection | Repo selection quality |
| Evidence (M12-M13) | evidence recall, evidence precision | Evidence citation quality |
| Semantic (M14-M15, M14b) | RCA message relevance, component ID, smoking gun hit | Explanation quality |
| Pipeline (M16-M18) | path accuracy, loop efficiency, token cost | Execution efficiency |
| Aggregate (M19-M20) | overall accuracy, run variance | System-level |

## Known limitations

1. **Domain-specific**: All cases are PTP/timing failures from one CI system. No generalization claim.
2. **Single annotator**: Ground truth curated by one engineer from Jira + PRs.
3. **No inter-annotator agreement**: No second-opinion verification process.
4. **Generated scenario**: `ptp-real-ingest` was produced by a script not in the repo; manual edits diverge from the generator.
5. **Coverage gaps**: Only 18 verified cases. Cases with rare defect types (firmware, environment) have 1-2 examples.
