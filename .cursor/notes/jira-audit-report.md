# Jira Ground Truth Audit Report

**Date:** 2026-02-17  
**Scope:** 30 PTP calibration cases in `internal/calibrate/scenarios/ptp_real_ingest.go`  
**Sources:** Red Hat Jira (public WebFetch), user-exported HTML (private CNF project), GitHub PRs  
**Coverage:** 28/30 Jira tickets fetched; 2 inaccessible (OCPBUGS-55838, OCPBUGS-65911)

---

## Accessibility Summary

| Access Method | Count | Jira IDs |
|---------------|-------|----------|
| Public (WebFetch) | 23 | OCPBUGS-70233, -74939, -64567, -70327, -74342, -63435, -55121, -68352, -71204, -70178, -74904, -74377, -75899, -66413, -49373, -45680, -53247, -54967, -45674, -47685, -72558, -49372, -59849 |
| User export (HTML) | 5 | CNF-21408, CNF-21588, CNF-21102, CNF-20071, CNF-17776 |
| Inaccessible | 2 | OCPBUGS-55838 (RCA-7), OCPBUGS-65911 (RCA-12) |

---

## Per-Case Audit

### OCPBUGS-70233 (RCA-1 / Case-01)

- **Resolution:** Duplicate
- **Jira Component:** Networking / ptp
- **Severity:** Critical
- **Affects:** 4.20
- **GitHub PRs:** None (duplicate)
- **Ground Truth:** defect=Product Bug (pb001), component=linuxptp-daemon
- **Verdict:** MATCH. Duplicate of another product bug; defect type and component correct.

### OCPBUGS-74939 (RCA-2 / Case-02)

- **Resolution:** Unresolved
- **Jira Component:** Networking / ptp
- **Severity:** Important
- **Affects:** 4.20, 4.21
- **GitHub PRs:** None
- **Issue Links:** relates to OCPBUGS-76372, OCPBUGS-76412
- **Ground Truth:** defect=Automation Bug (au001), component=cnf-gotests
- **Verdict:** REVIEW NEEDED. Jira component is "Networking/ptp" (a product component), but ground truth says au001/cnf-gotests. The description indicates a cloud-event-proxy issue (FREERUN event not generated). Keeping as-is pending further classification -- the human annotation said "automation issue" in the CI sheet.

### OCPBUGS-64567 (RCA-3 / Case-03)

- **Resolution:** Done
- **Jira Component:** Networking / ptp
- **Severity:** Important
- **Affects:** 4.18.z, 4.19.z
- **GitHub PRs:** None
- **Ground Truth:** defect=Firmware Issue (fw001), component=linuxptp-daemon
- **Verdict:** MATCH. Clock unable to lock after soft reboot -- firmware-level issue. Component correct.

### OCPBUGS-70327 (RCA-4 / Case-04)

- **Resolution:** Duplicate
- **Jira Component:** Networking / ptp
- **Severity:** Important
- **Affects:** 4.18.z
- **Duplicates:** OCPBUGS-74377 (fix PR: cloud-event-proxy#633)
- **Ground Truth:** defect=Environment Issue (en001), component=linuxptp-daemon, repo=linuxptp-daemon
- **Verdict:** MISMATCH. Fix is in `cloud-event-proxy` (PR #633 via parent OCPBUGS-74377). Component should be `cloud-event-proxy`. RelevantRepo should be `cloud-event-proxy`.

### OCPBUGS-74342 (RCA-5 / Case-05)

- **Resolution:** Unresolved
- **Jira Component:** Cloud Native Events / Cloud Event Proxy
- **Severity:** Important
- **Affects:** 4.20.z
- **Fix Version:** 4.20.z
- **GitHub PRs:** cloud-event-proxy#632
- **Clone chain:** clones OCPBUGS-73856 -> OCPBUGS-72558 -> OCPBUGS-63435
- **Ground Truth:** defect=Product Bug (pb001), component=linuxptp-daemon, repo=linuxptp-daemon
- **Verdict:** MISMATCH. Jira explicitly says "Cloud Native Events / Cloud Event Proxy". Component should be `cloud-event-proxy`. RelevantRepo should be `cloud-event-proxy`.

### OCPBUGS-63435 (RCA-6 / Case-06)

- **Resolution:** Done
- **Jira Component:** Cloud Native Events / Cloud Event Proxy
- **Severity:** Important
- **Affects:** 4.20.z
- **Fix Version:** 4.20.z
- **GitHub PRs:** cloud-event-proxy#613
- **Ground Truth:** defect=Product Bug (pb001), component=linuxptp-daemon, repo=linuxptp-daemon
- **Verdict:** MISMATCH. Same as RCA-5. Component should be `cloud-event-proxy`. RelevantRepo should be `cloud-event-proxy`. Fix: SyncState shouldn't assume ANTENNA-DISCONNECTED.

### OCPBUGS-55838 (RCA-7 / Case-07)

- **Status:** INACCESSIBLE (3 fetch attempts failed)
- **Ground Truth:** defect=Product Bug (pb001), component=linuxptp-daemon
- **Description from scenario:** "SNO management workload partitioning"
- **Verdict:** NO DATA. User export needed to verify.

### OCPBUGS-55121 (RCA-8 / Case-08)

- **Resolution:** Done-Errata
- **Jira Component:** Networking / ptp
- **Severity:** Important
- **Affects:** 4.14-4.18
- **GitHub PRs:** cloud-event-proxy#485
- **Clone of:** OCPBUGS-45680
- **Ground Truth:** defect=Product Bug (pb001), component=ptp-operator, repo=ptp-operator
- **Verdict:** MISMATCH. Fix PR is in `cloud-event-proxy` (subscription error handling). Component should be `cloud-event-proxy`. RelevantRepo should be `cloud-event-proxy`.

### CNF-21408 (RCA-9 / Case-09, private, from export)

- **Resolution:** Unresolved
- **Component:** None
- **GitHub PRs:** eco-gotests#1135
- **Ground Truth:** defect=Environment Issue (en001), component=linuxptp-daemon, repo=linuxptp-daemon
- **Verdict:** PARTIAL MATCH. DefectType en001 (environment) is correct -- NetworkManager managing PTP interfaces is an environment/config issue. Component is arguable: the fix is in eco-gotests (test framework), but the root cause is NM config. Keeping as-is.

### OCPBUGS-68352 (RCA-10 / Case-10)

- **Resolution:** Unresolved
- **Jira Component:** Networking / ptp
- **Severity:** Critical
- **Affects:** 4.16-4.21
- **Fix Version:** 4.22
- **GitHub PRs:** openshift/linuxptp-daemon#520, k8snetworkplumbingwg/linuxptp-daemon#135
- **Ground Truth:** defect=Automation Bug (au001), component=linuxptp-daemon, repo=linuxptp-daemon
- **Verdict:** MISMATCH. This is a real product bug (phc2sys command concatenation is malformed). DefectType should be `pb001`, not `au001`. Component and repo are correct (linuxptp-daemon).

### CNF-21588 (RCA-11 / Case-11, private, from export)

- **Resolution:** Unresolved
- **Component:** CNF vRAN / Far Edge
- **GitHub PRs:** None
- **Ground Truth:** defect=Product Bug (pb001), component=cnf-gotests, repo=cnf-gotests
- **Verdict:** REVIEW NEEDED. This is a tracking story, not a specific bug. Multiple ntpfailover failures tracked under one umbrella. Component cnf-gotests seems appropriate.

### OCPBUGS-65911 (RCA-12 / Case-12)

- **Status:** INACCESSIBLE (2 fetch attempts failed)
- **Ground Truth:** defect=Product Bug (pb001), component=linuxptp-daemon
- **Description from scenario:** "Basic PTP Configs should have LOCKED clock state in PTP metrics"
- **Verdict:** NO DATA. User export needed to verify.

### CNF-21102 (RCA-13 / Case-13, private, from export)

- **Resolution:** Done
- **Component:** CNF vRAN / Far Edge
- **GitHub PRs:** cloud-event-proxy#604 (fix t-bc ptp4l metric when unlocked)
- **Ground Truth:** defect=Environment Issue (en001), component=cnf-gotests, repo=cnf-gotests
- **Verdict:** MISMATCH. Fix PR is in `cloud-event-proxy` (ptp4l metric fix). This is a product bug in metric reporting, not an environment issue. DefectType should be `pb001`. RelevantRepo should include `cloud-event-proxy`.

### OCPBUGS-71204 (RCA-14 / Case-14)

- **Resolution:** Done
- **Jira Component:** Networking / ptp
- **Severity:** Important
- **Affects:** 4.20, 4.21
- **Fix Version:** 4.21
- **GitHub PRs:** openshift/linuxptp-daemon#524
- **Clones:** OCPBUGS-70227
- **Ground Truth:** defect=Product Bug (pb001), component=linuxptp-daemon, repo=linuxptp-daemon
- **Verdict:** MATCH. Fix is in linuxptp-daemon (NOTIFY_CMLDS optional in pmc regex). All correct.

### OCPBUGS-70178 (RCA-15 / Case-15)

- **Resolution:** Duplicate
- **Jira Component:** Networking / ptp
- **Severity:** Critical
- **Affects:** 4.20.z
- **Relates to:** OCPBUGS-68352
- **GitHub PRs:** None (duplicate)
- **Ground Truth:** defect=Product Bug (pb001), component=linuxptp-daemon, repo=linuxptp-daemon
- **Verdict:** MATCH. Duplicate of phc2sys concatenation bug (OCPBUGS-68352). All correct.

### OCPBUGS-74904 (RCA-16 / Case-16)

- **Resolution:** Done
- **Jira Component:** Networking / ptp
- **Severity:** Critical
- **Affects:** 4.18.z, 4.19.z
- **Clones/Duplicates:** OCPBUGS-74895
- **GitHub PRs:** None listed
- **Ground Truth:** defect=Product Bug (pb001), component=linuxptp-daemon, repo=linuxptp-daemon
- **Verdict:** MATCH. PTP Daemon stops bringing back child services after config change. Product bug. Component plausible.

### OCPBUGS-74377 (RCA-17 / Case-17)

- **Resolution:** Done
- **Jira Component:** Networking / ptp
- **Severity:** Important
- **Affects:** 4.20
- **GitHub PRs:** cloud-event-proxy#633
- **Clones:** OCPBUGS-74296
- **Release Note:** Process_status collection decoupled from profile-loading state.
- **Ground Truth:** defect=Product Bug (pb001), component=linuxptp-daemon, repo=linuxptp-daemon
- **Verdict:** MISMATCH. Fix PR is in `cloud-event-proxy` (process status before profile validation). Component should be `cloud-event-proxy`. RelevantRepo should be `cloud-event-proxy`.

### OCPBUGS-75899 (RCA-18 / Case-18)

- **Resolution:** Unresolved
- **Jira Component:** Networking / ptp
- **Severity:** Important
- **Affects:** 4.14, 4.21
- **GitHub PRs:** None
- **Ground Truth:** defect=Automation Bug (au001), component=linuxptp-daemon, repo=linuxptp-daemon
- **Verdict:** REVIEW NEEDED. HOLDOVER event generated when BC master interface goes down. Classified as automation issue in CI sheet. Keeping as-is pending resolution.

### CNF-20071 (RCA-19 / Case-19, private, from export)

- **Resolution:** Done
- **Component:** ptp
- **Severity:** Moderate
- **Related:** OCPBUGS-62719, OCPBUGS-63158
- **GitHub PRs:** None
- **Ground Truth:** defect=Product Bug (pb001), component=linuxptp-daemon, repo=linuxptp-daemon
- **Verdict:** REVIEW NEEDED. Test fails because ts2phc metrics report LOCKED while interface is down. May be "As Designed" per dev comments. DefectType could be en001 or au001 instead of pb001.

### OCPBUGS-66413 (RCA-20 / Case-20)

- **Resolution:** Unresolved
- **Jira Component:** Networking / ptp
- **Severity:** Critical
- **Affects:** 4.18-4.21
- **Fix Version:** 4.21.z
- **Duplicated by:** OCPBUGS-74884
- **Release Note:** Stale metrics from previously applied profile not cleaned up.
- **Ground Truth:** defect=Product Bug (pb001), component=linuxptp-daemon, repo=linuxptp-daemon
- **Verdict:** MATCH. Stale metrics bug, component is in linuxptp-daemon area. All correct.

### OCPBUGS-49373 (RCA-21 / Case-21)

- **Resolution:** Done-Errata
- **Jira Component:** Networking / ptp
- **Severity:** Moderate
- **Affects:** 4.14.z
- **GitHub PRs:** cnf-features-deploy#2178
- **Clone of:** OCPBUGS-44603
- **Depends on:** OCPBUGS-49372
- **Ground Truth:** defect=Product Bug (pb001), component=linuxptp-daemon, repo=linuxptp-daemon
- **Verdict:** MISMATCH. Fix is in `cnf-features-deploy` (ZTP config: remove phc2sys `-w` option). RelevantRepo should be `cnf-features-deploy`.

### OCPBUGS-45680 (RCA-22 / Case-22)

- **Resolution:** Done-Errata
- **Jira Component:** Networking / ptp
- **Severity:** Important
- **Affects:** 4.14-4.18
- **GitHub PRs:** cloud-event-proxy#422, #469, #472
- **Ground Truth:** defect=Product Bug (pb001), component=linuxptp-daemon, repo=linuxptp-daemon
- **Verdict:** MISMATCH. All fix PRs are in `cloud-event-proxy` (consumer subscription error handling). Component should be `cloud-event-proxy`. RelevantRepo should be `cloud-event-proxy`.

### OCPBUGS-53247 (RCA-23 / Case-23)

- **Resolution:** Done-Errata
- **Jira Component:** Networking / ptp
- **Severity:** Important
- **Affects:** 4.19
- **GitHub PRs:** cloud-event-proxy#458
- **Ground Truth:** defect=Product Bug (pb001), component=linuxptp-daemon, repo=linuxptp-daemon
- **Verdict:** MISMATCH. Fix is in `cloud-event-proxy` (CLOCK_REALTIME state evaluation for HA profile). Component should be `cloud-event-proxy`. RelevantRepo should be `cloud-event-proxy`.

### OCPBUGS-54967 (RCA-24 / Case-24)

- **Resolution:** Done-Errata
- **Jira Component:** Networking / ptp
- **Severity:** Moderate
- **Affects:** 4.19.0
- **GitHub PRs:** k8snetworkplumbingwg/linuxptp-daemon#29
- **Release Note:** Delay applying ptpconfig to capture early logs from ptp4l.
- **Ground Truth:** defect=Product Bug (pb001), component=linuxptp-daemon, repo=linuxptp-daemon
- **Verdict:** MATCH. Fix is in linuxptp-daemon (upstream). All correct.

### OCPBUGS-45674 (RCA-25 / Case-25)

- **Resolution:** Duplicate
- **Jira Component:** Networking / ptp
- **Severity:** Moderate
- **Affects:** 4.15-4.18
- **Duplicates:** OCPBUGS-43847
- **GitHub PRs:** None
- **Ground Truth:** defect=Product Bug (pb001), component=linuxptp-daemon, repo=linuxptp-daemon
- **Verdict:** MATCH. Duplicate of holdover/freerun event issue. Component plausible.

### OCPBUGS-47685 (RCA-26 / Case-26)

- **Resolution:** Done-Errata
- **Jira Component:** Networking / ptp
- **Severity:** Important
- **Affects:** 4.16-4.18
- **GitHub PRs:** cloud-event-proxy#422
- **Ground Truth:** defect=Product Bug (pb001), component=linuxptp-daemon, repo=linuxptp-daemon
- **Verdict:** MISMATCH. Fix is in `cloud-event-proxy` (consumer subscription loss after cold reboot). Component should be `cloud-event-proxy`. RelevantRepo should be `cloud-event-proxy`.

### CNF-17776 (RCA-27 / Case-27, private, from export)

- **Resolution:** Done
- **Component:** None
- **Severity:** Moderate
- **Related:** OCPBUGS-55687
- **GitHub PRs:** None (GitLab MR: cnf-gotests !818)
- **Ground Truth:** defect=Product Bug (pb001), component=linuxptp-daemon, repo=linuxptp-daemon
- **Verdict:** MISMATCH. This is an automation fix (add version conditions to clock_class validations). DefectType should be `au001`. Component should be `cnf-gotests`. RelevantRepo should be `cnf-gotests`.

### OCPBUGS-72558 (RCA-28 / Case-28)

- **Resolution:** Done
- **Jira Component:** Cloud Native Events / Cloud Event Proxy
- **Severity:** Important
- **Affects:** 4.20.z
- **Fix Version:** 4.20.z
- **GitHub PRs:** cloud-event-proxy#624
- **Clone of:** OCPBUGS-63435
- **Release Note:** GNSS sync state events: change from ANTENNA-DISCONNECTED to FAILURE-NOFIX.
- **Ground Truth:** defect=Product Bug (pb001), component=linuxptp-daemon, repo=linuxptp-daemon
- **Verdict:** MISMATCH. Jira explicitly says "Cloud Native Events / Cloud Event Proxy". Component should be `cloud-event-proxy`. RelevantRepo should be `cloud-event-proxy`.

### OCPBUGS-49372 (RCA-29 / Case-29)

- **Resolution:** Done
- **Jira Component:** Networking / ptp
- **Severity:** Moderate
- **Affects:** 4.14.z
- **GitHub PRs:** cnf-features-deploy#2177
- **Blocks:** OCPBUGS-49373 (RCA-21)
- **Ground Truth:** defect=Product Bug (pb001), component=linuxptp-daemon, repo=linuxptp-daemon
- **Verdict:** MISMATCH. Fix is in `cnf-features-deploy` (remove phc2sys `-w` option from ZTP config). RelevantRepo should be `cnf-features-deploy`.

### OCPBUGS-59849 (RCA-30 / Case-30, from export)

- **Resolution:** Cannot Reproduce
- **Jira Component:** Networking / ptp
- **Severity:** Important
- **Affects:** 4.17.z
- **GitHub PRs:** None
- **Ground Truth:** defect=Product Bug (pb001), component=linuxptp-daemon, repo=linuxptp-daemon
- **Verdict:** REVIEW NEEDED. Resolution is "Cannot Reproduce". The issue (LOCKED state sent for DOWN interface) was not reproducible with newer ptp operator. DefectType pb001 is reasonable but may warrant reclassification.

---

## Mismatch Summary

### Component / RelevantRepo corrections needed (12 cases)

| Case | Current Component | Correct Component | Current Repo | Correct Repo | Evidence |
|------|-------------------|-------------------|--------------|--------------|----------|
| OCPBUGS-70327 (RCA-4) | linuxptp-daemon | cloud-event-proxy | linuxptp-daemon | cloud-event-proxy | Dup of OCPBUGS-74377, fix PR cloud-event-proxy#633 |
| OCPBUGS-74342 (RCA-5) | linuxptp-daemon | cloud-event-proxy | linuxptp-daemon | cloud-event-proxy | Jira says "Cloud Native Events / Cloud Event Proxy", PR #632 |
| OCPBUGS-63435 (RCA-6) | linuxptp-daemon | cloud-event-proxy | linuxptp-daemon | cloud-event-proxy | Jira says "Cloud Native Events / Cloud Event Proxy", PR #613 |
| OCPBUGS-55121 (RCA-8) | ptp-operator | cloud-event-proxy | ptp-operator | cloud-event-proxy | Fix PR cloud-event-proxy#485 |
| CNF-21102 (RCA-13) | cnf-gotests | cloud-event-proxy | cnf-gotests | cloud-event-proxy | Fix PR cloud-event-proxy#604 |
| OCPBUGS-74377 (RCA-17) | linuxptp-daemon | cloud-event-proxy | linuxptp-daemon | cloud-event-proxy | Fix PR cloud-event-proxy#633 |
| OCPBUGS-49373 (RCA-21) | linuxptp-daemon | cnf-features-deploy | linuxptp-daemon | cnf-features-deploy | Fix PR cnf-features-deploy#2178 |
| OCPBUGS-45680 (RCA-22) | linuxptp-daemon | cloud-event-proxy | linuxptp-daemon | cloud-event-proxy | Fix PRs cloud-event-proxy#422,#469,#472 |
| OCPBUGS-53247 (RCA-23) | linuxptp-daemon | cloud-event-proxy | linuxptp-daemon | cloud-event-proxy | Fix PR cloud-event-proxy#458 |
| OCPBUGS-47685 (RCA-26) | linuxptp-daemon | cloud-event-proxy | linuxptp-daemon | cloud-event-proxy | Fix PR cloud-event-proxy#422 |
| CNF-17776 (RCA-27) | linuxptp-daemon | cnf-gotests | linuxptp-daemon | cnf-gotests | Automation fix, MR cnf-gotests !818 |
| OCPBUGS-72558 (RCA-28) | linuxptp-daemon | cloud-event-proxy | linuxptp-daemon | cloud-event-proxy | Jira says "Cloud Native Events / Cloud Event Proxy", PR #624 |
| OCPBUGS-49372 (RCA-29) | linuxptp-daemon | cnf-features-deploy | linuxptp-daemon | cnf-features-deploy | Fix PR cnf-features-deploy#2177 |

### DefectType corrections needed (2 cases)

| Case | Current DefectType | Correct DefectType | Evidence |
|------|--------------------|--------------------|----------|
| OCPBUGS-68352 (RCA-10) | Automation Bug (au001) | Product Bug (pb001) | Real phc2sys concatenation bug, fix PRs linuxptp-daemon#520/#135 |
| CNF-17776 (RCA-27) | Product Bug (pb001) | Automation Bug (au001) | Automation fix: version conditions for clock_class validations |

### Cases needing review (4 cases)

| Case | Issue | Notes |
|------|-------|-------|
| OCPBUGS-74939 (RCA-2) | DefectType au001 vs Jira showing product area | CI sheet says automation; Jira component is product |
| CNF-21588 (RCA-11) | Tracking story, not single bug | Umbrella story for ntpfailover failures |
| CNF-20071 (RCA-19) | Possibly "As Designed" | Dev says ts2phc behavior is expected; may be au001 or en001 |
| OCPBUGS-59849 (RCA-30) | Cannot Reproduce | Issue not seen in newer versions |

### Inaccessible (2 cases)

| Case | Jira ID | Notes |
|------|---------|-------|
| OCPBUGS-55838 (RCA-7) | OCPBUGS-55838 | 3 fetch attempts failed. Description: "SNO management workload partitioning" |
| OCPBUGS-65911 (RCA-12) | OCPBUGS-65911 | 2 fetch attempts failed. Description: "Basic PTP Configs LOCKED clock state" |

---

## GitHub PR Summary

| Jira | Repo | PR | Fix Description | Merged |
|------|------|----|-----------------|--------|
| OCPBUGS-45680 | redhat-cne/cloud-event-proxy | #469, #472 | Error handling in configmap and consumer subscription | Yes |
| OCPBUGS-47685 | redhat-cne/cloud-event-proxy | #422 | Consumer subscription loss after cold reboot | Yes |
| OCPBUGS-49372 | openshift-kni/cnf-features-deploy | #2177 | [4.17] Remove phc2sys `-w` option | Yes |
| OCPBUGS-49373 | openshift-kni/cnf-features-deploy | #2178 | [4.16] Remove phc2sys `-w` option | Yes |
| OCPBUGS-53247 | redhat-cne/cloud-event-proxy | #458 | CLOCK_REALTIME HA profile state evaluation | Yes |
| OCPBUGS-54967 | k8snetworkplumbingwg/linuxptp-daemon | #29 | Delay ptpconfig load to capture early logs | Yes |
| OCPBUGS-55121 | redhat-cne/cloud-event-proxy | #485 | Backport consumer subscription error handling to 4.18 | Yes |
| OCPBUGS-63435 | redhat-cne/cloud-event-proxy | #613 | SyncState: FAILURE-NOFIX not ANTENNA-DISCONNECTED | Yes |
| OCPBUGS-68352 | openshift/linuxptp-daemon | #520 | Upstream sync (phc2sys concatenation fix) | Yes |
| OCPBUGS-68352 | k8snetworkplumbingwg/linuxptp-daemon | #135 | Ensure phc2sys commandline properly concatenated | Yes |
| OCPBUGS-71204 | openshift/linuxptp-daemon | #524 | Make NOTIFY_CMLDS optional in pmc regex | Yes |
| OCPBUGS-72558 | redhat-cne/cloud-event-proxy | #624 | Cherry-pick GNSS fix to release-4.20 | Yes |
| OCPBUGS-74342 | redhat-cne/cloud-event-proxy | #632 | Cherry-pick GNSS fix to release-4.18 | Yes |
| OCPBUGS-74377 | redhat-cne/cloud-event-proxy | #633 | Process status before profile validation (4.18) | Yes |
| CNF-21102 | redhat-cne/cloud-event-proxy | #604 | Fix t-bc ptp4l metric when unlocked | Yes |
| CNF-21408 | rh-ecosystem-edge/eco-gotests | #1135 | Toggle NM-managed state for PTP interfaces | Yes |

---

## Clone Chain Map

Three distinct bug families span multiple cases:

**Family 1: GNSS Sync State mapping** (RCA-5, RCA-6, RCA-28)
- Root: OCPBUGS-63435 (RCA-6) -> cloud-event-proxy#613
- Clone: OCPBUGS-72558 (RCA-28) -> cloud-event-proxy#624 (4.20 backport)
- Clone: OCPBUGS-73856 -> OCPBUGS-74342 (RCA-5) -> cloud-event-proxy#632 (4.18 backport)

**Family 2: Consumer subscription loss** (RCA-8, RCA-22, RCA-26)
- Root: OCPBUGS-45680 (RCA-22) -> cloud-event-proxy#422,#469,#472
- Clone: OCPBUGS-55121 (RCA-8) -> cloud-event-proxy#485 (4.18 backport)
- Related: OCPBUGS-47685 (RCA-26) -> cloud-event-proxy#422

**Family 3: phc2sys `-w` option removal** (RCA-21, RCA-29)
- Root: OCPBUGS-44603 -> OCPBUGS-49372 (RCA-29) -> cnf-features-deploy#2177 (4.17)
- Clone: OCPBUGS-49373 (RCA-21) -> cnf-features-deploy#2178 (4.16)
