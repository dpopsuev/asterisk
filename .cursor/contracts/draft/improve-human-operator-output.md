# Contract — Improve Human Operator Output

**Status:** draft  
**Goal:** The human operator watching a calibration run always knows what is happening, why, and how long they'll be waiting.  
**Serves:** PoC completion

## Contract rules

- `human-readable-output.mdc` applies: raw codes in human-facing text are bugs.
- `agent-bus.mdc` defines the dispatch loop structure — this contract extends it with narration requirements.
- No Go code changes required. This contract governs **agent chat narration** during MCP calibration.

## Context

### The problem

During the Phase 5a calibration run (18 cases, 130 steps, 25 minutes), the agent produced output like:

```
C05 complete. 2/18 done. Continuing with the next case.
```
```
Retry loop for C06. Broadening scope.
```
```
F2 again for the retry. Let me submit a stronger investigation with higher convergence this time.
```

These violate `human-readable-output.mdc` (raw codes like `C05`, `F2`, `F3`) and leave the human operator with three unanswered questions:

1. **What is happening right now?** The step name in human words, the case identity.
2. **Is it going well?** Convergence direction, defect type hypothesis, component under investigation.
3. **How much longer?** ETA based on elapsed time and remaining cases.

Silence is the worst output. A human watching a 25-minute process with no narration assumes it's broken.

### Vocabulary table

Map domain concepts to human language. The agent must never use the left column in chat output.

| Machine code | Human words | Example in narration |
|---|---|---|
| `C05`, `C13` | "case 5", "case 13" | "Case 5 of 18" |
| `F0_RECALL` | "checking for prior matches" | "Checking if we've seen this failure before" |
| `F1_TRIAGE` | "classifying symptoms" | "Classifying the symptoms — looks like an assertion failure" |
| `F2_RESOLVE` | "selecting repos" | "Selecting repos to investigate — focusing on linuxptp-daemon" |
| `F3_INVESTIGATE` | "investigating" | "Investigating — tracing the holdover timeout change" |
| `F4_CORRELATE` | "checking for duplicates" | "Checking if this matches a prior case" |
| `F5_REVIEW` | "final review" | "Reviewing the analysis before closing" |
| `F6_REPORT` | "writing up the finding" | "Writing up the finding" |
| `pb001` | "product bug" | "Hypothesis: product bug in linuxptp-daemon" |
| `ab001` | "automation bug" | "This looks like an automation bug in the test code" |
| `si001` | "system issue" | "Likely a system/infrastructure issue" |
| convergence loop | "retrying with broader scope" | "Convergence too low (0.45) — retrying with broader scope" |
| duplicate detected | "same root cause as case N" | "Same root cause as case 7 — fast-tracking" |

### Narration format

Every narration line the agent emits to the human must contain **at least three** of these five elements:

| Element | Purpose | Example |
|---|---|---|
| **Progress** | Where are we in the run | "Case 5 of 18" |
| **Activity** | What the agent is doing right now | "Investigating holdover recovery" |
| **Trajectory** | Is confidence increasing or decreasing | "Convergence: 0.45 → 0.70 (accepted)" |
| **Diagnosis** | Current hypothesis or finding | "Product bug in linuxptp-daemon" |
| **Time** | How long this is taking, how long remains | "~2min this case, ~18min remaining" |

### Output templates

**Step transition** (emit once per pipeline step, one line):

```
Case 5 of 18 — Classifying symptoms for "GNSS sync state mapping" test
Case 5 of 18 — Investigating — hypothesis: product bug in cloud-event-proxy
Case 5 of 18 — Investigation converged at 0.80 — moving to duplicate check
```

**Convergence retry** (emit when the pipeline loops back):

```
Case 5 of 18 — Convergence too low (0.45), retrying investigation with broader repo scope
```

**Case completion** (emit when a case closes):

```
Case 5 of 18 complete — product bug in linuxptp-daemon (GNSS sync state) — 1m42s
  ETA: ~19min for remaining 13 cases
```

**Duplicate fast-track** (emit when F4 Correlate detects a duplicate):

```
Case 8 of 18 — Same root cause as case 7 (holdover recovery) — fast-tracked
```

**Milestone summary** (emit every 5 cases or at notable events):

```
━━━ Progress: 9 of 18 cases complete ━━━
  Elapsed: 12m30s │ Avg: 1m23s/case │ ETA: ~12min remaining
  Findings so far: 7 product bugs, 1 automation bug, 1 system issue
  Smoking gun hits: 2 of 9 │ 3 duplicates fast-tracked
```

**Parallel execution** (future — when parallel > 1):

```
[Worker 1] Case 5 of 18 — Investigating "GNSS sync state" — convergence 0.70
[Worker 2] Case 6 of 18 — Classifying symptoms for "PTP clock state"
[Worker 3] Case 7 of 18 — Complete — product bug in ptp-operator — 1m15s
```

### ETA calculation

```
avg_per_case = total_elapsed / cases_completed
eta = avg_per_case * cases_remaining
```

Adjust dynamically: cases with convergence retries take ~2x average. If the current case has already looped, increase the ETA for that case by 50%.

Round ETA to nearest minute for display. Show "less than a minute" when ETA < 60s.

## Execution strategy

1. Update `agent-bus.mdc` to include narration requirements (the vocabulary table and output templates above).
2. On the next calibration run, the agent follows the updated rule.
3. No Go code changes — this is pure agent behavior.

## Tasks

- [ ] **Update agent-bus.mdc** — add narration section with vocabulary table, output templates, and ETA requirements
- [ ] Validate — run a calibration (stub or real) and verify output matches the templates
- [ ] Tune — adjust templates based on actual output readability

## Acceptance criteria

- **Given** the agent is processing a calibration run via MCP,
- **When** the agent transitions between pipeline steps,
- **Then** every narration line contains at least 3 of the 5 required elements (progress, activity, trajectory, diagnosis, time).

- **Given** the agent completes a case,
- **When** the completion message is emitted,
- **Then** it includes the case number, total, defect type in human words, component name, elapsed time, and ETA for remaining cases.

- **Given** the agent has completed 5, 10, or 15 cases,
- **When** the milestone boundary is reached,
- **Then** a milestone summary is emitted with running tallies and ETA.

- **Given** the vocabulary table above,
- **When** the agent narrates any calibration activity,
- **Then** no raw machine codes (`C05`, `F2_RESOLVE`, `pb001`) appear in the output. Only human words, optionally with dual-audience format ("Product Bug (pb001)") when disambiguation is needed.

## Notes

- 2026-02-18 19:00 — Contract created. Triggered by Phase 5a calibration run where the agent produced 25 minutes of terse machine-code output with no ETAs, no progress context, and no trajectory information. The `human-readable-output.mdc` rule was violated throughout.
