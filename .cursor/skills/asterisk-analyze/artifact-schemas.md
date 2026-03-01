# Artifact Schemas

JSON schemas for each circuit step artifact, derived from `internal/orchestrate/types.go`. Write these exact field names and types.

## F0 Recall -- `recall-result.json`

**Go type:** `orchestrate.RecallResult`

| Field | JSON key | Type | Required | Description |
|-------|----------|------|----------|-------------|
| Match | `match` | boolean | yes | Whether a prior symptom matches this failure |
| PriorRCAID | `prior_rca_id` | integer | no | Store ID of the matching RCA (0 if no match) |
| SymptomID | `symptom_id` | integer | no | Store ID of the matching symptom (0 if no match) |
| Confidence | `confidence` | float | yes | Confidence in the match (0.0 to 1.0) |
| Reasoning | `reasoning` | string | yes | Explanation of why this is/isn't a match |
| IsRegression | `is_regression` | boolean | no | Whether this appears to be a regression |

**Example (no match):**

```json
{
  "match": false,
  "prior_rca_id": 0,
  "symptom_id": 0,
  "confidence": 0.1,
  "reasoning": "No prior symptom in the store matches this error pattern. The 'holdover exceeded' message has not been seen before.",
  "is_regression": false
}
```

**Example (match):**

```json
{
  "match": true,
  "prior_rca_id": 1,
  "symptom_id": 1,
  "confidence": 0.95,
  "reasoning": "Error pattern 'ptp4l FREERUN holdover exceeded' matches symptom S1 (ptp4l sync timeout). Same test, same error, different job.",
  "is_regression": false
}
```

## F1 Triage -- `triage-result.json`

**Go type:** `orchestrate.TriageResult`

| Field | JSON key | Type | Required | Description |
|-------|----------|------|----------|-------------|
| SymptomCategory | `symptom_category` | string | yes | One of: `product`, `automation`, `infra`, `flake` |
| Severity | `severity` | string | no | `critical`, `high`, `medium`, `low` |
| DefectTypeHypothesis | `defect_type_hypothesis` | string | yes | Hypothesized RP defect type (e.g. `pb001`, `ab001`, `si001`, `nd001`) |
| CandidateRepos | `candidate_repos` | string[] | yes | List of repo names that might contain the root cause |
| SkipInvestigation | `skip_investigation` | boolean | yes | True for infra/flake cases that don't need deep investigation |
| ClockSkewSuspected | `clock_skew_suspected` | boolean | no | Whether clock skew may be a factor |
| CascadeSuspected | `cascade_suspected` | boolean | no | Whether this failure is a cascade from a shared setup |
| DataQualityNotes | `data_quality_notes` | string | no | Notes about data quality issues |

**Defect type codes:**

| Code | Meaning |
|------|---------|
| `pb001` | Product Bug |
| `ab001` | Automation Bug |
| `si001` | System Issue (infra) |
| `nd001` | No Defect (flake) |
| `ti001` | To Investigate (default) |

**Example (product bug):**

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

**Example (infra skip):**

```json
{
  "symptom_category": "infra",
  "severity": "medium",
  "defect_type_hypothesis": "si001",
  "candidate_repos": [],
  "skip_investigation": true,
  "cascade_suspected": false,
  "data_quality_notes": "NTP server unreachable - infrastructure issue, not code."
}
```

## F2 Resolve -- `resolve-result.json`

**Go type:** `orchestrate.ResolveResult`

| Field | JSON key | Type | Required | Description |
|-------|----------|------|----------|-------------|
| SelectedRepos | `selected_repos` | RepoSelection[] | yes | Repos selected for investigation |
| CrossRefStrategy | `cross_ref_strategy` | string | no | Strategy for cross-referencing repos |

**RepoSelection:**

| Field | JSON key | Type | Required | Description |
|-------|----------|------|----------|-------------|
| Name | `name` | string | yes | Repository name |
| Path | `path` | string | no | Local path to the repo |
| FocusPaths | `focus_paths` | string[] | no | Specific directories/files to focus on |
| Branch | `branch` | string | no | Branch to examine |
| Reason | `reason` | string | yes | Why this repo was selected |

**Example:**

```json
{
  "selected_repos": [
    {
      "name": "linuxptp-daemon-operator",
      "reason": "Product code with holdover configuration and ptp4l management"
    }
  ]
}
```

## F3 Investigate -- `artifact.json`

**Go type:** `orchestrate.InvestigateArtifact`

| Field | JSON key | Type | Required | Description |
|-------|----------|------|----------|-------------|
| LaunchID | `launch_id` | string | no | RP launch ID (empty in calibration) |
| CaseIDs | `case_ids` | int[] | no | RP item IDs (empty in calibration) |
| RCAMessage | `rca_message` | string | yes | Full root cause analysis explanation |
| DefectType | `defect_type` | string | yes | Defect type code (e.g. `pb001`) |
| Component | `component` | string | no | Affected component name |
| ConvergenceScore | `convergence_score` | float | yes | Confidence in the RCA (0.0 to 1.0) |
| EvidenceRefs | `evidence_refs` | string[] | yes | Evidence references (`repo:file:detail`) |

**Example:**

```json
{
  "launch_id": "",
  "case_ids": [],
  "rca_message": "Holdover timeout reduced from 300s to 60s in linuxptp-daemon config (commit abc1234 on release-4.21). The ptp4l process enters FREERUN state and cannot recover because the holdover period is too short.",
  "defect_type": "pb001",
  "component": "linuxptp-daemon",
  "convergence_score": 0.85,
  "evidence_refs": [
    "linuxptp-daemon-operator:pkg/daemon/config.go:abc1234"
  ]
}
```

## F4 Correlate -- `correlate-result.json`

**Go type:** `orchestrate.CorrelateResult`

| Field | JSON key | Type | Required | Description |
|-------|----------|------|----------|-------------|
| IsDuplicate | `is_duplicate` | boolean | yes | Whether this case is a duplicate of a prior RCA |
| LinkedRCAID | `linked_rca_id` | integer | no | Store ID of the linked RCA (0 if not duplicate) |
| Confidence | `confidence` | float | yes | Confidence in the correlation (0.0 to 1.0) |
| Reasoning | `reasoning` | string | yes | Explanation of correlation decision |
| CrossVersionMatch | `cross_version_match` | boolean | no | Whether the duplicate spans multiple versions |
| AffectedVersions | `affected_versions` | string[] | no | List of affected version labels |

**Example (not duplicate):**

```json
{
  "is_duplicate": false,
  "linked_rca_id": 0,
  "confidence": 0.3,
  "reasoning": "This is the first RCA for this symptom pattern. No prior cases to correlate with."
}
```

**Example (duplicate):**

```json
{
  "is_duplicate": true,
  "linked_rca_id": 1,
  "confidence": 0.90,
  "reasoning": "Same stale PtpConfig CRD issue as the prior case. Both point to missing AfterSuite cleanup.",
  "cross_version_match": false
}
```

## F5 Review -- `review-decision.json`

**Go type:** `orchestrate.ReviewDecision`

| Field | JSON key | Type | Required | Description |
|-------|----------|------|----------|-------------|
| Decision | `decision` | string | yes | `approve`, `reassess`, or `overturn` |
| HumanOverride | `human_override` | object | no | Override data (only for `overturn`) |
| LoopTarget | `loop_target` | string | no | Step to loop back to (only for `reassess`) |

**HumanOverride fields:**

| Field | JSON key | Type | Description |
|-------|----------|------|-------------|
| DefectType | `defect_type` | string | Corrected defect type |
| RCAMessage | `rca_message` | string | Corrected RCA message |

**Example (approve):**

```json
{
  "decision": "approve"
}
```

**Example (reassess):**

```json
{
  "decision": "reassess",
  "loop_target": "F3_INVESTIGATE"
}
```

## F6 Report -- `jira-draft.json`

**Go type:** `map[string]any` (free-form)

The report step produces a structured summary. Common fields:

| Field | Type | Description |
|-------|------|-------------|
| `case_id` | string | Case identifier |
| `test_name` | string | Test name from the failure |
| `summary` | string | Brief summary of the investigation |
| `defect_type` | string | Final defect type code |
| `component` | string | Affected component |
| `jira_id` | string | Jira bug ID (if known) |
| `evidence` | string[] | Key evidence references |

**Example:**

```json
{
  "case_id": "C1",
  "test_name": "OCP-83297 PTP sync stability",
  "summary": "Holdover timeout reduced from 300s to 60s in operator 4.21.0 causing ptp4l FREERUN state.",
  "defect_type": "pb001",
  "component": "linuxptp-daemon",
  "jira_id": "OCPBUGS-1001",
  "evidence": ["linuxptp-daemon-operator:pkg/daemon/config.go:abc1234"]
}
```
