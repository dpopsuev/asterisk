# Contract — Wet Calibration Victory

**Status:** closed (2026-02-17)  
**Goal:** Run all three calibration scenarios with the real cursor adapter (`--adapter=cursor`), iteratively tuning prompts, skill instructions, heuristics, or code until every scenario achieves 20/20 metrics — tracking progress via per-case Test Cards.

## Closure note

Closed without completing wet calibration via CursorAdapter. The mock-calibration-agent
(which stood in for the Cursor agent) scored 6/20 — an architectural dead end because
it lacked store access, relied on fragile text parsing, and had divergent heuristics.

**BasicAdapter baseline (zero-LLM):** M19 = 0.93 on Jira-verified ground truth (commit `aee60c1`).
This establishes the heuristic floor that any AI-driven adapter must exceed.

**Path forward:** Wet calibration with the real Cursor agent is deferred to the MCP path
(`mcp-server-foundation.md` -> `mcp-pipeline-tools.md` -> `mcp-calibration-mode.md`), which
gives Cursor direct tool access to the store and pipeline — solving the root causes that
made the mock-agent approach unviable. The mock-calibration-agent binary has been retired.

## Contract rules

- Global rules only.
- Every tuning change must be followed by a **dry check** (`--adapter=stub`) to confirm no regressions on the existing 20/20 stub results.
- Apply the **smallest possible fix** per iteration. One metric, one cause, one change.
- Document every iteration in the log: what failed, what changed, what improved.
- Do not modify ground truth (scenario definitions) unless the expectation itself is proven wrong. Prefer tuning prompts/skill/heuristics first.

## Calibration taxonomy

| Term | Adapter | Behavior | Purpose |
|------|---------|----------|---------|
| **Dry calibration** | `--adapter=stub` (StubAdapter) | Deterministic, pre-authored ideal responses | Validates pipeline machinery, heuristics, metric engine. **Already passing 20/20 on all 3 scenarios.** |
| **Wet calibration** | `--adapter=cursor` (CursorAdapter + Dispatcher) | Real AI reasoning via Cursor agent skill | Validates prompt quality, skill instructions, agent investigation ability. Non-deterministic. **This is what needs to pass.** |

**Dry** = pipeline proof. **Wet** = prompt proof.

CLI commands:

```bash
# Dry (regression check)
asterisk calibrate --scenario=ptp-mock --adapter=stub

# Wet (victory target)
asterisk calibrate --scenario=ptp-mock --adapter=cursor --dispatch=file
```

## Context

- **Dry status:** All three scenarios pass 20/20 with stub adapter. See `e2e-calibration.md`.
- **Wet status:** Untested. Requires `fs-dispatcher.md` (FileDispatcher) and `cursor-skill.md` (agent skill) to be implemented.
- **Metrics:** 20 metrics across 6 categories (Defect Type Accuracy, Overall Accuracy, etc.; M1–M20). See `e2e-calibration.md` §3.
- **Scenarios:** ptp-mock (12 cases), daemon-mock (8 cases), ptp-real (8 cases). 28 total cases.

## Test Cards

### Scenario: ptp-mock (12 cases, 3 RCAs, 4 symptoms)

| Card | Test name | Pattern | Expected path | Critical metrics | Dry | Wet | Iter | Tuning notes |
|------|-----------|---------|---------------|-----------------|-----|-----|------|-------------|
| ptp-mock/C1 | OCP-83297 PTP sync stability (4.20/T-TSC) | first-discovery | F0→F1→F2→F3→F4→F5→F6 | M1, M2, M9, M10, M12, M14, M15, M16 | pass | untested | — | — |
| ptp-mock/C2 | OCP-83297 PTP sync stability (4.20/T-BC) | recall-hit | F0→F5→F6 | M3, M5, M16 | pass | untested | — | — |
| ptp-mock/C3 | OCP-83297 PTP sync stability (4.21/T-TSC) | recall-hit, serial-killer | F0→F5→F6 | M3, M5, M16 | pass | untested | — | — |
| ptp-mock/C4 | OCP-83299 PTP config isolation (4.21/T-TSC) | first-discovery | F0→F1→F3→F4→F5→F6 | M1, M2, M9, M12, M14, M15, M16 | pass | untested | — | — |
| ptp-mock/C5 | OCP-83300 PTP config cleanup (4.21/T-TSC) | recall-hit | F0→F5→F6 | M3, M5, M16 | pass | untested | — | — |
| ptp-mock/C6 | OCP-83297 PTP sync stability (4.21/T-BC) | recall-hit, serial-killer | F0→F5→F6 | M3, M5, M16 | pass | untested | — | — |
| ptp-mock/C7 | OCP-83300 PTP config cleanup (4.21/T-BC) | recall-hit | F0→F5→F6 | M3, M16 | pass | untested | — | — |
| ptp-mock/C8 | OCP-49734 NTP sync validation (4.21/BC-OC) | triage-skip (infra) | F0→F1→F5→F6 | M2, M6, M16 | pass | untested | — | — |
| ptp-mock/C9 | OCP-83297 PTP sync stability (4.22/T-TSC) | recall-hit, serial-killer | F0→F5→F6 | M3, M5, M16 | pass | untested | — | — |
| ptp-mock/C10 | OCP-83302 PTP recovery test (4.22/T-TSC) | first-discovery, cross-symptom-dedup | F0→F1→F3→F4 | M1, M2, M5, M14, M15, M16 | pass | untested | — | — |
| ptp-mock/C11 | OCP-83303 PTP flaky timing (4.21/T-TSC) | triage-skip (flake) | F0→F1→F5→F6 | M2, M6, M16 | pass | untested | — | — |
| ptp-mock/C12 | OCP-83304 PTP ordered setup (4.21/T-TSC) | cascade, dedup | F0→F1→F3→F4 | M2, M5, M7, M14, M16 | pass | untested | — | — |

**Summary: 0/12 wet passing.**

### Scenario: daemon-mock (8 cases, 2 RCAs, 3 symptoms)

| Card | Test name | Pattern | Expected path | Critical metrics | Dry | Wet | Iter | Tuning notes |
|------|-----------|---------|---------------|-----------------|-----|-----|------|-------------|
| daemon-mock/C1 | PTP Recovery > phc2sys restart (4.18/recovery) | first-discovery | F0→F1→F2→F3→F4→F5→F6 | M1, M2, M9, M10, M12, M14, M15, M16 | pass | untested | — | — |
| daemon-mock/C2 | PTP Recovery > ptp4l restart PANIC (4.18/recovery) | recall-hit, PANIC-vs-FAIL | F0→F5→F6 | M3, M5, M16 | pass | untested | — | — |
| daemon-mock/C3 | PTP Recovery > HTTP events AfterEach (4.18/events) | recall-hit, AfterEach | F0→F5→F6 | M3, M16 | pass | untested | — | — |
| daemon-mock/C4 | PTP Recovery > node reboot broken pipe (4.18/recovery) | first-discovery | F0→F1→F2→F3→F4→F5→F6 | M1, M2, M9, M10, M12, M14, M15, M16 | pass | untested | — | — |
| daemon-mock/C5 | PTP Events > interface down broken pipe (4.18/events) | recall-hit | F0→F5→F6 | M3, M5, M16 | pass | untested | — | — |
| daemon-mock/C6 | PTP Events > 2-port OC failover BeforeEach (4.18/events) | cascade, dedup | F0→F1→F3→F4 | M2, M5, M7, M14, M16 | pass | untested | — | — |
| daemon-mock/C7 | PTP Events > 2-port OC holdover BeforeEach (4.18/events) | recall-hit, cascade | F0→F5→F6 | M3, M16 | pass | untested | — | — |
| daemon-mock/C8 | PTP Events > 2-port OC passive BeforeEach (4.18/events) | recall-hit, cascade | F0→F5→F6 | M3, M16 | pass | untested | — | — |

**Summary: 0/8 wet passing.**

### Scenario: ptp-real (8 cases, 2 RCAs, 3 symptoms)

| Card | Test name | Pattern | Expected path | Critical metrics | Dry | Wet | Iter | Tuning notes |
|------|-----------|---------|---------------|-----------------|-----|-----|------|-------------|
| ptp-real/C1 | PTP Recovery > phc2sys restart FAIL (4.18/recovery) | first-discovery | F0→F1→F2→F3→F4→F5→F6 | M1, M2, M9, M10, M12, M14, M15, M16 | pass | untested | — | — |
| ptp-real/C2 | PTP Recovery > ptp4l restart PANICKED (4.18/recovery) | recall-hit, PANIC-vs-FAIL | F0→F5→F6 | M3, M5, M16 | pass | untested | — | — |
| ptp-real/C3 | PTP Recovery > HTTP events AfterEach (4.18/events) | recall-hit, AfterEach | F0→F5→F6 | M3, M16 | pass | untested | — | — |
| ptp-real/C4 | PTP Recovery > node reboot broken pipe (4.18/recovery) | first-discovery | F0→F1→F2→F3→F4→F5→F6 | M1, M2, M9, M10, M12, M14, M15, M16 | pass | untested | — | — |
| ptp-real/C5 | PTP Events > interface down broken pipe (4.18/events) | recall-hit | F0→F5→F6 | M3, M5, M16 | pass | untested | — | — |
| ptp-real/C6 | PTP Events > 2-port OC failover BeforeEach (4.18/events) | cascade, dedup | F0→F1→F3→F4 | M2, M5, M7, M14, M16 | pass | untested | — | — |
| ptp-real/C7 | PTP Events > 2-port OC holdover BeforeEach (4.18/events) | recall-hit, cascade | F0→F5→F6 | M3, M16 | pass | untested | — | — |
| ptp-real/C8 | PTP Events > 2-port OC passive BeforeEach (4.18/events) | recall-hit, cascade | F0→F5→F6 | M3, M16 | pass | untested | — | — |

**Summary: 0/8 wet passing.**

### Pattern index (cross-scenario)

Use this to diagnose systemic failures — if all cards with a pattern fail, the issue is in the prompt/skill for that pattern, not in a single case.

| Pattern | Cards | Key prompts/heuristics involved |
|---------|-------|--------------------------------|
| first-discovery | ptp-mock/C1, C4; daemon-mock/C1, C4; ptp-real/C1, C4 | triage, resolve, investigate, correlate templates |
| recall-hit | ptp-mock/C2,C3,C5,C6,C7,C9; daemon-mock/C2,C3,C5,C7,C8; ptp-real/C2,C3,C5,C7,C8 | recall template, Recall Hit (H1) heuristic |
| serial-killer | ptp-mock/C3, C6, C9, C10 | recall + correlate templates, Correlate Duplicate (H15) heuristic |
| triage-skip | ptp-mock/C8, C11 | triage template, Triage Skip Infra/Flake (H4/H5) heuristics |
| cascade | ptp-mock/C12; daemon-mock/C6,C7,C8; ptp-real/C6,C7,C8 | triage template (cascade detection), Triage Single Repo / Resolve Multi (H7/H8) guards |
| PANIC-vs-FAIL | daemon-mock/C2; ptp-real/C2 | recall template (must not be confused by failure type) |
| AfterEach | daemon-mock/C3; ptp-real/C3 | recall template (must recognize AfterEach), G9 guard |
| cross-symptom-dedup | ptp-mock/C10 | correlate template, Correlate Duplicate (H15) cross-version match |
| red-herring-rejection | all cases with F2 | resolve template (must reject sriov-network-operator / eco-gotests) |

## Victory loop

### Iteration protocol

1. **Run wet calibration** on one scenario: `asterisk calibrate --scenario=<name> --adapter=cursor --dispatch=file`
2. **Read the report.** For each failing metric, identify which cases caused the failure.
3. **Update Test Cards** with wet status (`pass`/`fail`) and notes.
4. **Diagnose the root cause** using the decision tree:
   - **Prompt issue** → tune the relevant template in `.cursor/prompts/`
   - **Skill issue** → tune instructions in `.cursor/skills/asterisk-investigate/`
   - **Heuristic issue** → tune threshold in `internal/orchestrate/heuristics.go`
   - **Code bug** → fix in `internal/orchestrate/` or `internal/calibrate/`
   - **Ground truth wrong** → fix scenario definition (last resort)
5. **Apply the smallest possible fix.**
6. **Dry check** → `asterisk calibrate --scenario=<name> --adapter=stub` must still be 20/20.
7. **Re-run wet.** Repeat from step 1 until passing.
8. **Move to next scenario.** Repeat.
9. **Victory** when all 3 scenarios pass.

### Recommended scenario order

1. **ptp-mock** first — 12 cases, broadest pattern coverage, best for initial prompt tuning.
2. **daemon-mock** second — 8 cases, exercises PANIC-vs-FAIL, AfterEach, and cascade specifically.
3. **ptp-real** last — 8 cases, real Jira bugs, most realistic, hardest.

### Victory criteria

All three commands must exit 0 (PASS, 20/20 metrics):

```bash
asterisk calibrate --scenario=ptp-mock    --adapter=cursor --dispatch=file  # PASS
asterisk calibrate --scenario=daemon-mock --adapter=cursor --dispatch=file  # PASS
asterisk calibrate --scenario=ptp-real    --adapter=cursor --dispatch=file  # PASS
```

## Execution strategy

1. Ensure prerequisites are complete (fs-dispatcher + cursor-skill).
2. Run ptp-mock wet. Iterate until 20/20.
3. Run daemon-mock wet. Iterate until 20/20.
4. Run ptp-real wet. Iterate until 20/20.
5. Final dry check on all three to confirm no regressions.
6. Document victory baseline in the iteration log.

## Tasks

- [ ] **Prerequisites check** — Confirm `fs-dispatcher.md` and `cursor-skill.md` are implemented. `--dispatch=file` works. Cursor agent skill is installed and responds to signals.
- [ ] **Wet ptp-mock iteration 1** — Run wet calibration on ptp-mock. Record result. Update Test Cards. Diagnose failures.
- [ ] **Wet ptp-mock tuning** — Iterate: tune prompts/skill/heuristics, dry check, re-run wet, until 20/20. May take multiple iterations.
- [ ] **Wet daemon-mock iteration 1** — Run wet calibration on daemon-mock. Record result. Update Test Cards. Diagnose failures.
- [ ] **Wet daemon-mock tuning** — Iterate until 20/20.
- [ ] **Wet ptp-real iteration 1** — Run wet calibration on ptp-real. Record result. Update Test Cards. Diagnose failures.
- [ ] **Wet ptp-real tuning** — Iterate until 20/20.
- [ ] **Final dry regression check** — Run all three scenarios with `--adapter=stub`, confirm all still 20/20.
- [ ] **Victory** — All three scenarios wet-passing 20/20. Document baseline metrics and iteration count in log.

## Acceptance criteria

- **Given** the FileDispatcher and Cursor skill are implemented,
- **When** `asterisk calibrate --scenario=ptp-mock --adapter=cursor --dispatch=file` is run,
- **Then** the Cursor agent completes all 12 cases and the calibration report shows 20/20 metrics PASS.

- **Given** ptp-mock is wet-passing,
- **When** `asterisk calibrate --scenario=daemon-mock --adapter=cursor --dispatch=file` is run,
- **Then** the Cursor agent completes all 8 cases and the calibration report shows 20/20 metrics PASS.

- **Given** daemon-mock is wet-passing,
- **When** `asterisk calibrate --scenario=ptp-real --adapter=cursor --dispatch=file` is run,
- **Then** the Cursor agent completes all 8 cases and the calibration report shows 20/20 metrics PASS.

- **Given** all three scenarios are wet-passing,
- **When** dry calibration is re-run on all three (`--adapter=stub`),
- **Then** all three still show 20/20 PASS (no regressions from tuning).

- **Given** the Test Cards table in this contract,
- **When** calibration completes,
- **Then** all 28 cards show `wet: pass` and the `Tuning notes` column documents what was changed for any card that initially failed.

## Dependencies

| Contract | Required for | Status |
|----------|-------------|--------|
| `fs-dispatcher.md` | `--dispatch=file` mode for automated signal exchange | active |
| `cursor-skill.md` | Agent skill that responds to FileDispatcher signals | active |
| `e2e-calibration.md` | Calibration framework, metrics, scenarios | complete (dry) |

## Iteration log

(Running log, newest first. One entry per iteration.)

| Iter | Date | Scenario | Result | Metrics | Cards changed | What changed |
|------|------|----------|--------|---------|--------------|-------------|
| — | — | — | — | — | — | (no iterations yet) |

## Notes

(Running log, newest first. YYYY-MM-DD HH:MM — decision or finding.)

- 2026-02-16 23:45 — Contract created. 28 Test Cards across 3 scenarios. Dry: all 20/20. Wet: all untested. Dependencies: fs-dispatcher + cursor-skill must be implemented before first wet run.
