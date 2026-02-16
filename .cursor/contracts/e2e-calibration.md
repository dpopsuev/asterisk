# Contract — E2E Calibration Test

**Status:** draft  
**Goal:** Build two calibration modes — **mock** (synthetic closed-world) and **real** (actual Jira bugs + RP launches + local repos) — run the full investigation pipeline blindfolded (`--agent --dev-calibrate`), and measure how closely the agent's conclusions match the known answers — with instrumented metrics that produce numeric pass/fail signals from natural language outputs.

## Contract rules

- Global rules only.
- The calibration scenario is a **closed world**: every RCA, every symptom, every failure, and every piece of evidence is pre-authored. The agent must discover the answers from the data, not from hints in the test harness.
- Metrics must be **numeric and deterministic** — no subjective "looks about right." Each metric has a threshold that gates pass/fail.
- The calibration loop is repeatable: same synthetic data, multiple runs, measure variance.
- Synthetic data must be realistic enough that the prompt system exercises all families (F0–F6), including recall hits, triage skips, low-confidence loops, and serial killer detection.
- **Execution order:** Mock calibration (Scenario A) is developed and iterated **during** implementation. Real calibration (Scenario B) is the **absolute last step** — it runs only after all implementation contracts are complete, the code is refactored and decoupled, and mock calibration consistently passes. See `rules/asterisk-development.mdc` § Calibration ordering.

## Context

- **Existing mocks:** `StubFetcher` (fixed envelope), `MemStore`, `MemPushStore`, `DefaultPusher`, `httptest.Server` mocks for rpfetch/rppush. These test individual components but not the full pipeline.
- **Existing fixture:** `examples/pre-investigation-33195-4.21/` — real PTP launch envelope with 12 failures. Good structure to model synthetic data after.
- **Pipeline:** F0 Recall → F1 Triage → F2 Resolve → F3 Investigate → F4 Correlate → F5 Review → F6 Report. See `contracts/prompt-families.md`.
- **Store v2:** `contracts/storage-adapter-v2.md` — all entities the pipeline reads/writes.
- **Orchestrator:** `contracts/prompt-orchestrator.md` — drives the pipeline, evaluates heuristics, persists state.

---

## 1  Scenario A — Mock (synthetic ground truth)

### 1.1  The scenario: "PTP Calibration World"

A pre-authored investigation with **3 versions**, **3 pipelines**, **15 failures**, **4 symptoms**, and **3 RCAs**. Designed to exercise every pipeline path.

#### RCAs (the criminals — known answers)

| ID | Title | Defect type | Category | Component | Affected versions | Jira |
|----|-------|-------------|----------|-----------|-------------------|------|
| R1 | Holdover timeout reduced from 300s to 60s in operator 4.21.0-202602070620 | `pb001` (product bug) | product | linuxptp-daemon | 4.20, 4.21, 4.22 | OCPBUGS-1001 |
| R2 | Test cleanup missing: PTP config CRD not deleted between test suites | `ab001` (automation bug) | automation | ptp-test-framework | 4.21 | OCPBUGS-1002 |
| R3 | NTP server unreachable from test cluster node (infra flap) | `si001` (system issue) | infra | cluster-infra | 4.21 | — |

#### Symptoms (the stories)

| ID | Name | Error pattern | Component | Maps to RCA |
|----|------|---------------|-----------|-------------|
| S1 | ptp4l sync timeout | `ptp4l.*FREERUN.*holdover exceeded` | linuxptp-daemon | R1 |
| S2 | Stale PTP config CRD assertion | `Expected PtpConfig.*not to exist.*but it does` | ptp-test-framework | R2 |
| S3 | NTP sync loss | `chronyd.*no selectable sources` | cluster-infra | R3 |
| S4 | ptp4l recovery failure | `ptp4l.*failed to recover lock within.*seconds` | linuxptp-daemon | R1 |

S1 and S4 are different stories pointing to the same criminal (R1). R1 is the "serial killer" across 3 versions.

#### Test failures (the witnesses)

| Case | Version | Job | Test name | Symptom | RCA | Pipeline path expected |
|------|---------|-----|-----------|---------|-----|----------------------|
| C1 | 4.20 | [T-TSC] | OCP-83297 PTP sync stability | S1 | R1 | F0 miss → F1 → F2 → F3 → F4 → F5 → F6 |
| C2 | 4.20 | [T-BC] | OCP-83297 PTP sync stability | S1 | R1 | F0 miss (first run) or hit (if C1 done) → F4 link → F5 |
| C3 | 4.21 | [T-TSC] | OCP-83297 PTP sync stability | S1 | R1 | F0 recall hit (serial killer) → F5 |
| C4 | 4.21 | [T-TSC] | OCP-83299 PTP config isolation | S2 | R2 | F0 miss → F1 → F2 → F3 → F5 → F6 |
| C5 | 4.21 | [T-TSC] | OCP-83300 PTP config cleanup | S2 | R2 | F0 hit (same symptom as C4) → F4 link → F5 |
| C6 | 4.21 | [T-BC] | OCP-83297 PTP sync stability | S1 | R1 | F0 recall hit → F5 |
| C7 | 4.21 | [T-BC] | OCP-83300 PTP config cleanup | S2 | R2 | F0 hit → F4 link → F5 |
| C8 | 4.21 | [BC-OC] | OCP-49734 NTP sync validation | S3 | R3 | F0 miss → F1 (infra) → skip → F5 |
| C9 | 4.22 | [T-TSC] | OCP-83297 PTP sync stability | S1 | R1 | F0 recall hit (serial killer, 3rd version) → F5 |
| C10 | 4.22 | [T-TSC] | OCP-83302 PTP recovery test | S4 | R1 | F0 miss (different symptom!) → F1 → F2 → F3 → F4 (links to R1) → F5 |
| C11 | 4.21 | [T-TSC] | OCP-83303 PTP flaky timing | — | — | F0 miss → F1 (flake) → skip → F5 (nd001) |
| C12 | 4.21 | [T-TSC] | OCP-83304 PTP ordered setup | S2 (cascade) | R2 | F0 miss → F1 (cascade from C4) → F4 link → F5 |

Design rationale:
- **C1–C2:** First-time investigation. Agent must discover R1 from scratch via F3.
- **C3, C6, C9:** Serial killer recall — F0 should recognize S1 from C1/C2.
- **C4–C5, C7:** Automation bug discovery + dedup.
- **C8:** Infrastructure skip path (F1 → skip).
- **C10:** Different symptom (S4) but same criminal (R1) — tests F4 cross-symptom correlation.
- **C11:** Flake skip path.
- **C12:** Cascade detection (BeforeSuite failure from C4's root cause).

### 1.2  Synthetic evidence (planted in fake repos)

For each RCA, plant discoverable evidence in synthetic git repos:

| RCA | Evidence | Repo | Planted at |
|-----|----------|------|-----------|
| R1 | Commit `abc1234`: changed `holdoverTimeout` from 300 to 60 in `pkg/daemon/config.go` | `linuxptp-daemon-operator` | branch `release-4.21`, date within test window |
| R2 | Missing `AfterSuite` cleanup in `test/e2e/ptp_config_test.go` — `Expect(cleanup).To(Succeed())` commented out | `ptp-test-framework` | branch `main`, visible in git blame |
| R3 | No code evidence — infra issue. Cluster NTP server logs show outage. | — | Planted in synthetic CI artifact/log file |

### 1.3  Synthetic context workspace

A context workspace JSON file defining the repos the agent can access. This is what F2 Resolve reads to select repos and what F3 Investigate uses to locate code. The workspace contains both relevant and irrelevant repos — the agent must pick the right ones.

```json
{
  "repos": [
    {
      "name": "linuxptp-daemon-operator",
      "path": "/tmp/calibrate/repos/linuxptp-daemon-operator",
      "purpose": "PTP operator: manages linuxptp-daemon DaemonSet, PtpConfig CRD, clock sync",
      "branch": "release-4.21"
    },
    {
      "name": "ptp-test-framework",
      "path": "/tmp/calibrate/repos/ptp-test-framework",
      "purpose": "E2E test suite for PTP operator: Ginkgo specs, test helpers, fixtures",
      "branch": "main"
    },
    {
      "name": "cluster-infra-config",
      "path": "/tmp/calibrate/repos/cluster-infra-config",
      "purpose": "CI cluster configuration: job profiles, NTP config, network templates",
      "branch": "main"
    },
    {
      "name": "sriov-network-operator",
      "path": "/tmp/calibrate/repos/sriov-network-operator",
      "purpose": "SR-IOV network operator: VF allocation, device plugin (NOT PTP-related)",
      "branch": "release-4.21"
    },
    {
      "name": "cnf-features-deploy",
      "path": "/tmp/calibrate/repos/cnf-features-deploy",
      "purpose": "CNF deployment manifests and CI profiles: contains job definitions for all telco operators",
      "branch": "master"
    }
  ]
}
```

#### Repo relevance ground truth

| Repo | Relevant to | Expected F2 selection |
|------|------------|----------------------|
| `linuxptp-daemon-operator` | R1 (S1, S4) — product code with the holdover change | Selected for C1, C2, C10 |
| `ptp-test-framework` | R2 (S2) — test code with missing cleanup | Selected for C4, C12 |
| `cluster-infra-config` | R3 (S3) — infra config (NTP). Also useful as secondary for R1 (CI profile) | Selected for C8; secondary for C1 |
| `sriov-network-operator` | **None** — red herring. Shares similar naming patterns but is unrelated to PTP failures. | Should NOT be selected |
| `cnf-features-deploy` | Tangential — CI job definitions. Useful as secondary context but not the source of any RCA. | May be selected as secondary; not a problem if included |

The workspace tests F2 Resolve's ability to:
- Use `purpose` metadata to rank repos (not just name matching)
- Reject irrelevant repos (sriov-network-operator is a trap)
- Select multiple repos for cross-reference when triage suggests it
- Use the correct branch from the workspace (not hardcode `main`)

#### Synthetic repo contents

Each repo is created programmatically (temp dirs + `git init`) with planted files and commits:

| Repo | Structure | Planted evidence |
|------|-----------|-----------------|
| `linuxptp-daemon-operator` | `pkg/daemon/config.go`, `pkg/daemon/sync.go`, `Makefile`, `go.mod` | Commit `abc1234` on `release-4.21`: `holdoverTimeout` 300→60. 3 other innocent commits as noise. |
| `ptp-test-framework` | `test/e2e/ptp_config_test.go`, `test/e2e/ptp_sync_test.go`, `test/helpers/` | `AfterSuite` cleanup commented out in `ptp_config_test.go`. 2 other innocent test commits as noise. |
| `cluster-infra-config` | `profiles/telco-ptp-4.21.yaml`, `ntp/chrony.conf`, `README.md` | NTP server address in `chrony.conf` points to unreachable host. CI artifact log `ntp-outage.log` with timestamps. |
| `sriov-network-operator` | `pkg/daemon/`, `api/`, `go.mod` | Normal code, no relevant changes. Trap: similar `pkg/daemon/` structure to `linuxptp-daemon-operator`. |
| `cnf-features-deploy` | `ci/jobs/telco-ptp-4.21.groovy`, `manifests/` | Job definition referencing the PTP operator version. No bugs planted. |

### 1.4  Synthetic RP API responses

A fake HTTP server (`httptest.Server`) that serves:

| Endpoint | Returns |
|----------|---------|
| `GET /api/v1/{project}/launch/{id}` | Synthetic launch JSON per version (3 launches: 33190, 33195, 33210) |
| `GET /api/v1/{project}/item?filter.eq.launchId={id}` | Synthetic test items with planted error messages and log snippets |
| `PUT /api/v1/{project}/item/{id}/update` | Records the defect type update (captured for metric comparison) |

Error messages in synthetic items match the symptom patterns exactly (with realistic noise — timestamps, item IDs, stack traces) so the fingerprint system and the model both have enough signal.

---

## 1B  Scenario B — Real (OCPBUGS-74895 / broken pipe + config hang)

Real calibration using actual Jira bugs, RP launches, error stacks, and local git repos. The agent investigates blindfolded; we compare against the known human-determined RCAs.

### 1B.1  Investigation context directory

For real calibration, the CLI needs git repos available locally — no network calls during investigation. The `asterisk` CLI maintains an **investigation context directory** where relevant repos are cloned/pulled at setup time:

```
.asterisk/context/
├── repos/
│   ├── cnf-gotests/               ← git clone/pull from ~/Workspace/cnf-gotests
│   ├── ptp-operator/              ← git clone/pull from ~/Workspace/ptp-operator
│   ├── linuxptp-daemon/           ← git clone/pull from ~/Workspace/linuxptp-daemon
│   ├── cloud-event-proxy/         ← git clone/pull from ~/Workspace/cloud-event-proxy
│   └── eco-gotests/               ← git clone/pull from ~/Workspace/eco-gotests
└── workspace.json                 ← context workspace pointing to repos/ paths
```

**Setup flow:**
1. User provides a context workspace (or calibration scenario references one).
2. `asterisk calibrate --setup` (or implicit on first run): for each repo in the workspace, `git clone` (if not present) or `git pull` (if present) from the source path/URL into `.asterisk/context/repos/`.
3. Rewrite workspace paths to point to `.asterisk/context/repos/*` so all investigation reads from the local copy.
4. Optionally checkout specific branches/commits per the envelope's git metadata or workspace overrides.

This means: (a) investigation is fully offline after setup, (b) the original repos in `~/Workspace/` are not modified, (c) the context is version-pinned to the checkout at setup time.

### 1B.2  Real context workspace

```json
{
  "repos": [
    {
      "name": "cnf-gotests",
      "path": ".asterisk/context/repos/cnf-gotests",
      "source": "~/Workspace/cnf-gotests",
      "purpose": "PTP test cases (Ginkgo); test code, assertions, test helpers. Error stacks point here.",
      "branch": "master"
    },
    {
      "name": "ptp-operator",
      "path": ".asterisk/context/repos/ptp-operator",
      "source": "~/Workspace/ptp-operator",
      "purpose": "SUT: PTP operator lifecycle, manages linuxptp-daemon DaemonSet, PtpConfig CRD, PTP profiles",
      "branch": "release-4.18"
    },
    {
      "name": "linuxptp-daemon",
      "path": ".asterisk/context/repos/linuxptp-daemon",
      "source": "~/Workspace/linuxptp-daemon",
      "purpose": "SUT: PTP daemon running on nodes — ptp4l, phc2sys, ts2phc processes; event socket communication with cloud-event-proxy",
      "branch": "release-4.18"
    },
    {
      "name": "cloud-event-proxy",
      "path": ".asterisk/context/repos/cloud-event-proxy",
      "source": "~/Workspace/cloud-event-proxy",
      "purpose": "Cloud Event Proxy: receives PTP events from daemon via Unix socket (/cloud-native/events.sock); publishes cloud events. Broken pipe issue is daemon→proxy communication.",
      "branch": "release-4.18"
    },
    {
      "name": "eco-gotests",
      "path": ".asterisk/context/repos/eco-gotests",
      "source": "~/Workspace/eco-gotests",
      "purpose": "Ecosystem QE test framework; may contain shared helpers used by cnf-gotests"
    }
  ]
}
```

### 1B.3  Known RCAs (ground truth — from Jira)

| ID | Jira | Title | Defect type | Component | Affects | Resolution |
|----|------|-------|-------------|-----------|---------|-----------|
| R1 | [OCPBUGS-74895](https://issues.redhat.com/browse/OCPBUGS-74895) | Broken pipe: daemon→proxy events.sock communication. Pipe buffer clog causes concatenated burst at receiver. | `pb001` (product bug) | linuxptp-daemon / cloud-event-proxy | 4.18.z, 4.19.z | Duplicate (→ R2) |
| R2 | [OCPBUGS-74904](https://issues.redhat.com/browse/OCPBUGS-74904) | Config change hang: daemon detects PTP config change, stops children services, doesn't bring them up again. | `pb001` (product bug) | linuxptp-daemon | 4.18.z, 4.19.z | Done |

**Relationship:** OCPBUGS-74895 (broken pipe) and OCPBUGS-74904 (config hang) are linked as duplicates — same underlying root cause. The broken pipe is a downstream effect of the daemon process management bug. Both assigned to `Networking / ptp`, severity Critical, marked as Regression.

### 1B.4  Known symptoms (ground truth)

| ID | Name | Error pattern (from stacks) | Component | Maps to RCA |
|----|------|----------------------------|-----------|-------------|
| S1 | Config change hang — process restart failure | phc2sys/ptp4l process kill → daemon doesn't restart; PANIC in recovery path | linuxptp-daemon | R2 (OCPBUGS-74904) |
| S2 | Broken pipe — event socket write failure | events.sock broken pipe → consumer events lost/corrupted after node reboot or interface down | linuxptp-daemon → cloud-event-proxy | R1 (OCPBUGS-74895) |
| S3 | BeforeEach cascade — 2-port OC setup fails | Shared BeforeEach at ptp_interfaces.go:498 fails → 3 child tests marked FAILED | cnf-gotests (cascade) | R1 (OCPBUGS-74895) — cascade |

### 1B.5  Known test failures (ground truth — 8 failures)

**Config change hang (R2 / OCPBUGS-74904) — 3 failures:**

| Case | Test name | Test ID | Failure type | File:line | Symptom | Expected path |
|------|-----------|---------|-------------|-----------|---------|--------------|
| C1 | PTP Recovery > ptp process restart > should recover the phc2sys process after killing it | 59862 | FAIL | ptp_recovery.go:121 | S1 | F0 miss → F1 → F2 → F3 → F5 |
| C2 | PTP Recovery > ptp process restart > should recover the ptp4l process after killing a ptp4l process related to phc2sys | 49737 | PANICKED | panic.go:115 | S1 | F0 miss/hit → F4 link to C1 → F5 |
| C3 | PTP Recovery > HTTP events using consumer validates system fully functional after removing consumer | 59996 | FAIL (AfterEach) | ptp_events_and_metrics.go:175 | S1 | F0 hit → F4 link → F5 |

**Broken pipe (R1 / OCPBUGS-74895) — 5 failures:**

| Case | Test name | Test ID | Failure type | File:line | Symptom | Expected path |
|------|-----------|---------|-------------|-----------|---------|--------------|
| C4 | PTP Recovery > ptp node reboot > validates PTP consumer events after ptp node reboot | 59995 | FAIL | ptp_events_and_metrics.go:221 | S2 | F0 miss → F1 → F2 → F3 → F5 |
| C5 | PTP Events and Metrics > interface down > should generate events when slave interface goes down and up | 49742 | FAIL | ptp_interfaces.go:753 | S2 | F0 hit → F4 link to C4 → F5 |
| C6 | PTP Events and Metrics > interface down OC 2 port > verifies 2-port oc ha failover when active port goes down | 80963 | FAIL (BeforeEach) | ptp_interfaces.go:498 | S3 (cascade) | F0 miss → F1 (cascade) → F4 link → F5 |
| C7 | PTP Events and Metrics > interface down OC 2 port > verifies 2-port oc ha holdover & freerun when both ports go down | 80964 | FAIL (BeforeEach) | ptp_interfaces.go:498 | S3 (cascade) | F0 hit (same BeforeEach) → F4 link → F5 |
| C8 | PTP Events and Metrics > interface down OC 2 port > verifies 2-port oc ha passive interface recovery | 82012 | FAIL (BeforeEach) | ptp_interfaces.go:498 | S3 (cascade) | F0 hit (same BeforeEach) → F4 link → F5 |

### 1B.6  Calibration design rationale

| Pattern | Cases | What it tests |
|---------|-------|---------------|
| **Two Jiras, one root cause** | R1 ↔ R2 (duplicate) | Serial killer / dedup: agent should discover both Jiras trace to the same daemon process management bug |
| **PANIC vs FAIL** | C2 (PANICKED) vs C1 (FAIL) | Different failure types, same symptom — agent must not be confused by PANICKED |
| **AfterEach failure** | C3 | Cleanup/teardown failure — agent must recognize this is AfterEach, not the test itself (guard G9) |
| **BeforeEach cascade** | C6, C7, C8 (all at ptp_interfaces.go:498) | Three tests fail because shared BeforeEach fails — one RCA, not three (guard G7/G8, cascade detection) |
| **Same file:line, different tests** | C6, C7, C8 | Agent must not just look at the test name but recognize the shared setup line |
| **Node reboot vs interface down** | C4 vs C5 | Different triggers (reboot vs interface down) but same underlying broken pipe symptom |

### 1B.7  RP launches

| Launch | URL | Version |
|--------|-----|---------|
| 32764 | `https://your-reportportal.example.com/ui/#ecosystem-qe/launches/all/32764` | 4.18 |
| 32719 | `https://your-reportportal.example.com/ui/#ecosystem-qe/launches/all/32719` | 4.18 |

For real calibration, RP data is fetched once at setup time and cached locally (same as mock's fake RP API, but populated from real RP responses instead of synthetic JSON). This way the calibration runs don't depend on RP being available.

---

## 2  Calibration mode: `--agent --dev-calibrate`

### 2.1  Flow

```
# Mock scenario (synthetic data, fake RP API, synthetic repos)
asterisk calibrate --scenario=ptp-mock [--runs=N] [--adapter=stub|cursor]

# Real scenario (real Jira ground truth, cached RP data, local repos)
asterisk calibrate --scenario=ptp-real [--runs=N] [--adapter=cursor] [--setup]
```

1. **Setup:**
   - **Mock:** Start fake RP API server. Create synthetic repos in temp dirs. Initialize clean v2 Store.
   - **Real:** `--setup` flag triggers: clone/pull repos from `~/Workspace/*` into `.asterisk/context/repos/`, fetch RP launch data and cache locally, rewrite workspace paths. Subsequent runs use cached data (no network).
2. **For each run (1..N):**
   a. Clear Store (fresh start per run, or cumulative — configurable).
   b. For each case in scenario order:
      - Run orchestrator pipeline (F0→...→F5) programmatically.
      - In `--dev-calibrate` mode, the orchestrator auto-fills prompts and collects model responses (instead of manual Cursor handoff). Uses a model adapter (Cursor via API, or a test stub that returns canned responses for deterministic runs).
      - After F5, capture the investigation result (structured artifact).
   c. After all cases: run F6 Report.
   d. **Score:** Compare all artifacts against ground truth. Compute metrics.
3. **Aggregate:** Across N runs, compute mean/stddev for each metric. Report.

### 2.2  Model adapter (pluggable)

| Mode | Behavior | Use case |
|------|----------|----------|
| **stub** | Returns pre-written "ideal" responses for each prompt. Deterministic. | Test the pipeline/heuristic/metric machinery without LLM variance. |
| **cursor** | Sends prompts to Cursor (via file handoff or future API). Non-deterministic. | Calibrate prompt quality against ground truth. |
| **llm-api** | Sends prompts to an LLM API (OpenAI, Anthropic, local). Non-deterministic. | Future: automated calibration without Cursor. |

For PoC, implement **stub** first (validates the harness), then **cursor** (validates prompts).

---

## 3  Instrumented metrics

Every metric is a number in [0, 1] (or a count). Each has a **threshold** — below threshold = fail.

### 3.1  Structured field metrics (exact match)

These compare structured JSON fields from artifacts against ground truth. No NLP needed.

| Metric | What it measures | Computation | Threshold |
|--------|-----------------|-------------|-----------|
| **M1: defect_type_accuracy** | Did the agent assign the correct defect type code? | `correct_defect_types / total_cases` | ≥ 0.80 |
| **M2: symptom_category_accuracy** | Did F1 Triage produce the correct symptom category? | `correct_categories / triaged_cases` | ≥ 0.75 |
| **M3: recall_hit_rate** | For cases where F0 should recall a prior RCA, did it? | `true_positive_recalls / expected_recalls` | ≥ 0.70 |
| **M4: recall_false_positive_rate** | For cases where F0 should NOT recall, did it incorrectly recall? | `false_positive_recalls / expected_misses` | ≤ 0.10 |
| **M5: serial_killer_detection** | Did F4 correctly link cases to the same RCA when they share the same ground-truth RCA? | `correctly_linked_cases / expected_links` | ≥ 0.70 |
| **M6: skip_accuracy** | For infra/flake cases, did the pipeline correctly skip deep investigation? | `correct_skips / expected_skips` | ≥ 0.80 |
| **M7: cascade_detection** | Did F1 detect cascade cases (C12)? | `detected_cascades / expected_cascades` | ≥ 0.50 |
| **M8: convergence_calibration** | Does the convergence score correlate with actual correctness? | Pearson correlation(convergence_score, is_correct) | ≥ 0.40 |

### 3.2  Workspace and repo selection metrics

F2 Resolve selects repos from the context workspace. The agent must use purpose metadata and triage output to pick the right repos and reject red herrings.

| Metric | What it measures | Computation | Threshold |
|--------|-----------------|-------------|-----------|
| **M9: repo_selection_precision** | Of the repos F2 selected, how many were actually relevant to the ground truth RCA? | `relevant_selected / total_selected` (per case, averaged) | ≥ 0.70 |
| **M10: repo_selection_recall** | Of the repos that should have been selected, how many were? | `relevant_selected / total_relevant` (per case, averaged) | ≥ 0.80 |
| **M11: red_herring_rejection** | Did the agent avoid selecting irrelevant repos (e.g. sriov-network-operator)? | `1 - (red_herring_selected / total_cases_with_f2)` | ≥ 0.80 |

### 3.3  Evidence metrics (set overlap)

Compare cited evidence refs against planted evidence.

| Metric | What it measures | Computation | Threshold |
|--------|-----------------|-------------|-----------|
| **M12: evidence_recall** | Did the agent find the planted evidence? | `planted_evidence_found / total_planted_evidence` | ≥ 0.60 |
| **M13: evidence_precision** | Of the evidence cited, how much is actually relevant? | `relevant_cited / total_cited` | ≥ 0.50 |

Evidence matching: normalize paths and commit SHAs; exact match on planted refs; fuzzy match on file paths (allow partial path match).

### 3.3  Semantic metrics (NLP comparison)

Compare natural language RCA messages against ground truth descriptions. These require a judge.

| Metric | What it measures | Computation | Threshold |
|--------|-----------------|-------------|-----------|
| **M14: rca_message_relevance** | Does the RCA message describe the actual root cause? | Judge prompt: "Given ground truth RCA X and agent RCA Y, score semantic overlap 0–1." Average across cases. | ≥ 0.60 |
| **M15: component_identification** | Did the agent correctly identify the affected component? | Exact match on component field OR keyword presence in RCA message. | ≥ 0.70 |

### 3.4  Pipeline behavior metrics (structural)

| Metric | What it measures | Computation | Threshold |
|--------|-----------------|-------------|-----------|
| **M16: pipeline_path_accuracy** | Did the case follow the expected pipeline path (F0→F1→...→FN)? | `correct_paths / total_cases` | ≥ 0.60 |
| **M17: loop_efficiency** | How many investigation loops (F3→F2→F3) were needed vs. expected? | `mean(actual_loops) / mean(expected_loops)` (closer to 1.0 = better) | 0.5–2.0 range |
| **M18: total_prompt_tokens** | Total tokens sent across all prompts for the scenario. | Sum of prompt lengths. | ≤ budget (configurable) |

### 3.5  Aggregate metrics

| Metric | Computation | Threshold |
|--------|-------------|-----------|
| **M19: overall_accuracy** | Weighted average of M1, M2, M5, M10, M12, M14. | ≥ 0.65 |
| **M20: run_variance** | Stddev of M19 across N runs. Lower = more deterministic. | ≤ 0.15 |

---

## 4  Metric extraction from natural language

The core challenge: prompt outputs are natural language JSON (model writes `artifact.json`). How do we extract metrics?

### 4.1  Structured fields (easy)

The artifact JSON schema (`contracts/artifact-schema.md`) defines typed fields: `defect_type`, `convergence_score`, `evidence_refs[]`, `case_ids[]`. These are directly comparable — parse JSON, compare values.

### 4.2  Semantic comparison (requires judge)

For `rca_message` (free text), use a **judge prompt**:

```
You are a calibration judge. Compare the agent's RCA against the known ground truth.

Ground truth RCA:
{{.GroundTruth.Title}}
{{.GroundTruth.Description}}

Agent's RCA:
{{.Agent.RCAMessage}}

Score the semantic overlap on a scale of 0 to 1:
- 1.0: Agent identified the exact same root cause with the same mechanism.
- 0.7: Agent identified the right component and general area but missed specifics.
- 0.4: Agent is in the right ballpark but wrong mechanism.
- 0.1: Agent identified a different root cause entirely.
- 0.0: Agent's RCA is unrelated or nonsensical.

Return JSON: {"score": <float>, "reasoning": "<one sentence>"}
```

The judge itself is an LLM call. For deterministic testing (stub mode), skip the judge and use keyword matching as a fallback.

### 4.3  Keyword extraction (fallback for stub mode)

For each ground truth RCA, define **required keywords** that must appear in the agent's output:

| RCA | Required keywords (any N of M) |
|-----|-------------------------------|
| R1 | `holdover`, `timeout`, `60`, `300`, `linuxptp`, `FREERUN` (≥ 3 of 6) |
| R2 | `cleanup`, `CRD`, `AfterSuite`, `isolation`, `ptp-config` (≥ 2 of 5) |
| R3 | `NTP`, `unreachable`, `chronyd`, `infra` (≥ 2 of 4) |

Keyword match score: `matched_keywords / required_threshold`.

---

## 5  Calibration report

After each run (or batch of N runs), produce a structured report:

```
=== Asterisk Calibration Report ===
Scenario: ptp-calibration
Runs: 5
Model: cursor

--- Structured Metrics ---
M1  defect_type_accuracy:      0.83 (10/12)  ✓ (≥0.80)
M2  symptom_category_accuracy: 0.78 (7/9)    ✓ (≥0.75)
M3  recall_hit_rate:           0.71 (5/7)     ✓ (≥0.70)
M4  recall_false_positive:     0.00 (0/5)     ✓ (≤0.10)
M5  serial_killer_detection:   0.80 (4/5)     ✓ (≥0.70)
M6  skip_accuracy:             1.00 (2/2)     ✓ (≥0.80)
M7  cascade_detection:         0.50 (1/2)     ✓ (≥0.50)
M8  convergence_calibration:   0.62           ✓ (≥0.40)

--- Workspace / Repo Selection ---
M9  repo_selection_precision:  0.86 (6/7)     ✓ (≥0.70)
M10 repo_selection_recall:     1.00 (5/5)     ✓ (≥0.80)
M11 red_herring_rejection:     0.83 (5/6)     ✓ (≥0.80)

--- Evidence Metrics ---
M12 evidence_recall:           0.67 (4/6)     ✓ (≥0.60)
M13 evidence_precision:        0.57 (4/7)     ✓ (≥0.50)

--- Semantic Metrics ---
M14 rca_message_relevance:     0.73           ✓ (≥0.60)
M15 component_identification:  0.83 (10/12)   ✓ (≥0.70)

--- Pipeline Metrics ---
M16 pipeline_path_accuracy:    0.67 (8/12)    ✓ (≥0.60)
M17 loop_efficiency:           1.2            ✓ (0.5–2.0)
M18 total_prompt_tokens:       45000          ✓ (≤60000)

--- Aggregate ---
M19 overall_accuracy:          0.76           ✓ (≥0.65)
M20 run_variance (5 runs):     0.08           ✓ (≤0.15)

RESULT: PASS (20/20 metrics within threshold)

--- Per-case breakdown ---
C1  OCP-83297 (4.20/T-TSC): defect=pb001 ✓  symptom=timeout ✓  rca=R1 ✓  path=F0→F1→F2→F3→F4→F5 ✓
C2  OCP-83297 (4.20/T-BC):  defect=pb001 ✓  symptom=timeout ✓  rca=R1 ✓  path=F0→F4→F5 ✓
C3  OCP-83297 (4.21/T-TSC): defect=pb001 ✓  recall=hit ✓      rca=R1 ✓  path=F0→F5 ✓
...
```

Report saved to `.asterisk/calibration/{scenario}/{timestamp}/report.txt` and `metrics.json`.

---

## Execution strategy

1. Author synthetic ground truth (scenario definition, planted evidence).
2. Build fake RP API server (serve synthetic launches, items, logs).
3. Build synthetic git repos (planted commits and files).
4. Build metric computation engine (structured + evidence + semantic + pipeline metrics).
5. Build calibration runner (setup → run pipeline → score → report).
6. Build model adapter (stub first, then cursor).
7. Build judge prompt for semantic comparison.
8. Run calibration with stub adapter (validates harness).
9. Run calibration with cursor/LLM (validates prompts).
10. Iterate: tune prompts based on metric gaps, re-calibrate.

## Tasks

### Phase 1A — Mock scenario ground truth

- [ ] **Mock scenario definition** — `internal/calibrate/scenarios/ptp-mock/scenario.json`: all RCAs, symptoms, cases, expected pipeline paths, required evidence, keyword sets, expected repo selections. Machine-readable so the scorer can load it.
- [ ] **Synthetic RP responses** — JSON files: 3 launches, test items per launch with realistic error messages, log snippets (matching symptom patterns + noise), environment attributes.
- [ ] **Synthetic context workspace** — `internal/calibrate/scenarios/ptp-mock/workspace.json`: 5 repos (3 relevant, 1 red herring, 1 tangential) with purpose metadata and branch overrides.
- [ ] **Synthetic git repos** — Programmatic setup (`git init` + planted commits in temp dirs). Each with noise commits alongside planted evidence.
- [ ] **Repo setup helper** — `internal/calibrate/repogen/setup.go`: CreateSyntheticRepos(scenarioDir, tempDir) → creates git repos, plants commits, returns workspace JSON with correct temp paths.

### Phase 1B — Real scenario ground truth

- [ ] **Real scenario definition** — `internal/calibrate/scenarios/ptp-real/scenario.json`: ground truth from Jira (OCPBUGS-74895, OCPBUGS-74904), 8 failures, 3 symptoms, 2 RCAs (linked as duplicates), expected pipeline paths, keyword sets for RCA matching.
- [ ] **Investigation context directory** — `internal/calibrate/context/setup.go`: SetupContext(workspace, targetDir) → for each repo in workspace, `git clone` (or `cp -r` from local path) into `targetDir/repos/`, rewrite workspace paths, optionally checkout specific branch/commit. Returns rewritten workspace.
- [ ] **Real context workspace** — `internal/calibrate/scenarios/ptp-real/workspace.json`: 5 repos (cnf-gotests, ptp-operator, linuxptp-daemon, cloud-event-proxy, eco-gotests) with source paths pointing to `~/Workspace/*` and purpose metadata.
- [ ] **RP data cache** — `internal/calibrate/rpcache/cache.go`: FetchAndCache(rpClient, launchIDs, cacheDir) → fetch launch + items from RP API once, save as JSON in cache dir. LoadFromCache(cacheDir, launchID) → serve cached data. Calibration runs use cache, not live RP.
- [ ] **Cached RP data** — Fetch and store: launch 32764 (envelope + items), launch 32719 (envelope + items). Save to `internal/calibrate/scenarios/ptp-real/rp-cache/`.

### Phase 2 — Test infrastructure

- [ ] **Fake RP API server** — `internal/calibrate/fakerpapi/server.go`: `httptest.Server` serving synthetic launches and items. Records PUT calls for defect type updates.
- [ ] **Model adapter interface** — `internal/calibrate/model.go`: `ModelAdapter` interface: `SendPrompt(prompt string) → response string`. Implementations: `StubAdapter` (canned responses), `CursorAdapter` (file handoff), future `LLMAdapter`.
- [ ] **Stub adapter** — Returns pre-written "ideal" responses per family per case. Loaded from scenario definition.

### Phase 3 — Metric engine

- [ ] **Metric types** — `internal/calibrate/metrics.go`: `Metric` struct (id, name, value, threshold, pass bool). `MetricSet` (collection of all metrics).
- [ ] **Structured scorers** — Compare defect_type, symptom_category, recall hit/miss, skip, cascade against ground truth. Pure functions: `ScoreDefectType(artifacts, groundTruth) → Metric`.
- [ ] **Workspace scorer** — Compare F2 Resolve's `selected_repos[]` against ground truth repo relevance. Compute precision, recall, red herring rejection rate.
- [ ] **Evidence scorer** — Compare `evidence_refs[]` against planted evidence. Normalize paths, exact + fuzzy match.
- [ ] **Semantic scorer** — Judge prompt for rca_message comparison. Keyword fallback for stub mode.
- [ ] **Pipeline scorer** — Compare actual pipeline path (from case state) against expected path.
- [ ] **Aggregate scorer** — Weighted average, variance across runs.

### Phase 4 — Calibration runner

- [ ] **Runner** — `internal/calibrate/runner.go`: `RunCalibration(scenario, adapter, runs) → CalibrationReport`. Orchestrates: setup → per-case pipeline → score → aggregate.
- [ ] **Per-case execution** — Drives the orchestrator programmatically: init state → for each step: fill template → adapter.SendPrompt → parse response → write artifact → evaluate heuristic → advance.
- [ ] **Report generator** — Produces human-readable report + machine-readable `metrics.json`.

### Phase 5 — CLI integration

- [ ] **Calibrate subcommand** — `asterisk calibrate --scenario=<name> [--runs=N] [--adapter=stub|cursor] [--threshold-file=<path>]`. Runs calibration and prints report.
- [ ] **Dev flag** — `--dev-calibrate` on existing `cursor` subcommand: runs single case through orchestrator with model adapter instead of manual handoff.

### Phase 6 — Validate

- [ ] **Stub run** — Full calibration with stub adapter. All metrics should be near-perfect (validates harness).
- [ ] **Cursor run** — Full calibration with cursor adapter. Baseline metrics established.
- [ ] **Threshold tuning** — Adjust thresholds based on initial cursor runs. Document baseline.
- [ ] **Validate (green)** — Stub run passes all metrics. Cursor run establishes baseline.
- [ ] **Tune (blue)** — Clean up, document.

## Acceptance criteria

- **Given** the PTP calibration scenario with 12 cases, 4 symptoms, 3 RCAs, and a context workspace with 5 repos (including 1 red herring),
- **When** `asterisk calibrate --scenario=ptp-calibration --adapter=stub --runs=1` is run,
- **Then** all 20 metrics are computed and reported, the stub run scores ≥ 0.95 on all metrics (near-perfect baseline), and the report is saved to disk.
- **And given** the same scenario with `--adapter=cursor --runs=3`,
- **When** calibration completes,
- **Then** aggregate metrics (M19) ≥ 0.65, run variance (M20) ≤ 0.15, repo selection recall (M10) ≥ 0.80, red herring rejection (M11) ≥ 0.80, and per-case breakdowns identify which cases/metrics need prompt tuning.
- **And** the fake RP API correctly records all defect type updates pushed during the run.
- **And** the calibration is repeatable: same scenario, same adapter, produces consistent results (within variance threshold).

## Notes

(Running log, newest first. YYYY-MM-DD HH:MM — decision or finding.)

- 2026-02-17 04:00 — Added real calibration scenario (Section 1B): OCPBUGS-74895 (broken pipe) + OCPBUGS-74904 (config change hang), 8 real failures, 3 symptoms, 2 RCAs (duplicate/same root cause). RP launches 32764, 32719. Real context workspace: 5 local repos (cnf-gotests, ptp-operator, linuxptp-daemon, cloud-event-proxy, eco-gotests) from ~/Workspace/. Added investigation context directory concept (.asterisk/context/repos/) for offline investigation — clone/pull at setup, pin versions, don't modify originals. Added RP data caching (fetch once, cache locally). Two calibration modes: `ptp-mock` (synthetic) and `ptp-real` (actual data).
- 2026-02-17 03:30 — Added synthetic context workspace: 5 repos (3 relevant, 1 red herring, 1 tangential) with purpose metadata. Added workspace/repo selection metrics (M9–M11): precision, recall, red herring rejection. Added synthetic repo setup helper (programmatic git init + planted commits). Total: 20 metrics across 6 categories.
- 2026-02-17 03:00 — Contract created. Closed-world E2E calibration: 12 synthetic cases across 3 versions, 4 symptoms, 3 RCAs. Fake RP API + synthetic git repos. Stub adapter for deterministic testing; cursor adapter for prompt calibration. Calibration report with per-case breakdown and pass/fail thresholds.
