# Worked Examples

Prompt-to-artifact examples for each pipeline step. These show the reasoning process and expected output format.

---

## F0 Recall -- Miss (no prior match)

**Prompt excerpt:**

> Case C1: `[T-TSC] RAN PTP ptp4l FREERUN holdover exceeded`
>
> Known symptoms:
> (none in store)
>
> Analyze: does this failure match any known symptom?

**Reasoning:** The symptom store is empty. No prior patterns to match. This is a new failure.

**Artifact (`recall-result.json`):**

```json
{
  "match": false,
  "prior_rca_id": 0,
  "symptom_id": 0,
  "confidence": 0.05,
  "reasoning": "No symptoms in the store to compare against. This is a fresh failure requiring full investigation.",
  "is_regression": false
}
```

---

## F0 Recall -- Hit (prior match found)

**Prompt excerpt:**

> Case C5: `[T-BC] RAN PTP ptp4l FREERUN holdover exceeded`
>
> Known symptoms:
> - S1 (symptom_id=1): "ptp4l holdover timeout" — linked to RCA #1 (product bug, linuxptp-daemon config change)
>
> Analyze: does this failure match any known symptom?

**Reasoning:** The error message "ptp4l FREERUN holdover exceeded" strongly matches S1. Same root cause suspected.

**Artifact (`recall-result.json`):**

```json
{
  "match": true,
  "prior_rca_id": 1,
  "symptom_id": 1,
  "confidence": 0.92,
  "reasoning": "Error pattern 'ptp4l FREERUN holdover exceeded' matches S1 (ptp4l holdover timeout). Both involve ptp4l entering FREERUN state due to holdover config. The BC profile variant shows the same root cause as the TSC variant.",
  "is_regression": false
}
```

---

## F1 Triage -- Product bug with investigation

**Prompt excerpt:**

> Case C1: `[T-TSC] RAN PTP ptp4l FREERUN holdover exceeded`
> Error: `ptp4l[1234]: FREERUN holdover 60s exceeded`
> Workspace repos: linuxptp-daemon-operator, cluster-infra-config
> Classify and decide next steps.

**Reasoning:** The holdover value (60s) suggests a configuration change in the operator. This is a product code issue, not infra. Both repos are candidates.

**Artifact (`triage-result.json`):**

```json
{
  "symptom_category": "product",
  "severity": "critical",
  "defect_type_hypothesis": "pb001",
  "candidate_repos": ["linuxptp-daemon-operator", "cluster-infra-config"],
  "skip_investigation": false,
  "cascade_suspected": false
}
```

---

## F1 Triage -- Infra skip

**Prompt excerpt:**

> Case C3: `[T-DUALNIC] RAN PTP NIC firmware timeout`
> Error: `NIC i810 firmware: initialization timeout after 30s`
> Classify and decide next steps.

**Reasoning:** NIC firmware timeout is a hardware/infrastructure issue. No code investigation needed.

**Artifact (`triage-result.json`):**

```json
{
  "symptom_category": "infra",
  "severity": "medium",
  "defect_type_hypothesis": "si001",
  "candidate_repos": [],
  "skip_investigation": true,
  "cascade_suspected": false,
  "data_quality_notes": "NIC firmware timeout indicates hardware issue, not a code defect."
}
```

---

## F2 Resolve -- Repo selection

**Prompt excerpt:**

> Case C1: triage says product bug, candidates: [linuxptp-daemon-operator, cluster-infra-config]
> Workspace repos:
> - linuxptp-daemon-operator: PTP daemon operator code
> - cluster-infra-config: Cluster infrastructure configuration
> Select repos for investigation.

**Reasoning:** The holdover config is managed by the daemon operator. cluster-infra-config is less likely to contain PTP-specific logic.

**Artifact (`resolve-result.json`):**

```json
{
  "selected_repos": [
    {
      "name": "linuxptp-daemon-operator",
      "reason": "Product code managing ptp4l daemon configuration including holdover timeout"
    }
  ]
}
```

---

## F3 Investigate -- Deep RCA with evidence

**Prompt excerpt:**

> Case C1: PTP holdover exceeded.
> Error: `ptp4l[1234]: FREERUN holdover 60s exceeded`
> Repo: linuxptp-daemon-operator
> Prior step context: product bug hypothesis, holdover config change suspected.
> Produce a root cause analysis.

**Reasoning:** The holdover timeout was reduced from 300s to 60s. Found in operator config. The daemon can't resync in 60s, enters FREERUN permanently.

**Artifact (`artifact.json`):**

```json
{
  "launch_id": "",
  "case_ids": [],
  "rca_message": "Holdover timeout was reduced from 300s to 60s in the linuxptp-daemon-operator release-4.21 branch. The ptp4l process cannot achieve PTP lock within 60s after a transient GPS outage, causing it to remain in FREERUN state until the test timeout is reached.",
  "defect_type": "pb001",
  "component": "linuxptp-daemon",
  "convergence_score": 0.85,
  "evidence_refs": [
    "linuxptp-daemon-operator:pkg/daemon/config.go:holdover_timeout_change"
  ]
}
```

---

## F4 Correlate -- Not a duplicate

**Prompt excerpt:**

> Case C1: RCA says product bug in holdover config.
> Prior RCAs in store: (none)
> Is this a duplicate of any prior investigation?

**Artifact (`correlate-result.json`):**

```json
{
  "is_duplicate": false,
  "linked_rca_id": 0,
  "confidence": 0.1,
  "reasoning": "No prior RCAs in the store. This is the first investigation for this failure pattern."
}
```

---

## F4 Correlate -- Duplicate found

**Prompt excerpt:**

> Case C5 ([T-BC] profile): RCA says product bug in holdover config.
> Prior RCAs:
> - RCA #1: "Holdover timeout reduced from 300s to 60s" (product bug, linuxptp-daemon)
> Is this a duplicate?

**Artifact (`correlate-result.json`):**

```json
{
  "is_duplicate": true,
  "linked_rca_id": 1,
  "confidence": 0.92,
  "reasoning": "Same root cause as RCA #1: holdover timeout regression affecting ptp4l sync. BC profile variant of the same issue.",
  "cross_version_match": false
}
```

---

## F5 Review -- Approve

**Prompt excerpt:**

> Case C1: RCA approved?
> Defect type: pb001 (product bug)
> Component: linuxptp-daemon
> Convergence: 0.85
> Evidence count: 1
> Review the analysis and decide.

**Artifact (`review-decision.json`):**

```json
{
  "decision": "approve"
}
```

---

## F5 Review -- Reassess (low confidence)

**Prompt excerpt:**

> Case C7: RCA for review.
> Defect type: ti001 (to investigate)
> Convergence: 0.3
> Evidence: none

**Reasoning:** Convergence is too low and defect type is still "to investigate". Needs more analysis.

**Artifact (`review-decision.json`):**

```json
{
  "decision": "reassess",
  "loop_target": "F3_INVESTIGATE"
}
```

---

## F6 Report -- Final summary

**Prompt excerpt:**

> Produce a final summary for case C1.
> Test: OCP-83297 PTP sync stability ([T-TSC])
> Defect: pb001, component: linuxptp-daemon
> RCA: Holdover timeout reduced...

**Artifact (`jira-draft.json`):**

```json
{
  "case_id": "C1",
  "test_name": "OCP-83297 PTP sync stability [T-TSC]",
  "summary": "Holdover timeout reduced from 300s to 60s in linuxptp-daemon-operator release-4.21, causing ptp4l FREERUN state during transient GPS outages.",
  "defect_type": "pb001",
  "component": "linuxptp-daemon",
  "evidence": [
    "linuxptp-daemon-operator:pkg/daemon/config.go:holdover_timeout_change"
  ]
}
```

---

## Cascade example (F1 with cascade flag)

**Prompt excerpt:**

> Case C2: `[T-TSC] RAN PTP ptp4l CLOCK_REALTIME offset`
> Error: `ptp4l CLOCK_REALTIME offset 999999ns > threshold`
> The previous case C1 failed first in the same suite.
> Classify and decide.

**Reasoning:** This failure occurred after C1 in the same test suite. The offset issue is likely a downstream effect of the holdover timeout issue in C1.

**Artifact (`triage-result.json`):**

```json
{
  "symptom_category": "product",
  "severity": "high",
  "defect_type_hypothesis": "pb001",
  "candidate_repos": ["linuxptp-daemon-operator"],
  "skip_investigation": false,
  "cascade_suspected": true,
  "data_quality_notes": "Likely cascade from C1 (holdover timeout). The offset violation is a downstream symptom."
}
```
