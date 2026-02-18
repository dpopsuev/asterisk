# Human-Readable Output

> **Status:** complete  
> **Rule:** `.cursor/rules/human-readable-output.mdc`  
> **Principle:** Code is for machines, words are for humans.

## Problem

Opaque machine codes (`pb001`, `F0_RECALL`, `R21`, `M9`, `H3`) appear in every human-facing surface: CLI output, markdown reports, logs, and documentation. A human reading `defect=pb001 path=F0→F1→F2` gets zero information. The same data as `defect=Product Bug path=Recall→Triage→Resolve` is immediately useful.

## Scope

### Code families with violations

| Family | Codes | Human names | Violations |
|--------|-------|-------------|------------|
| Defect types | `pb001`, `au001`, `en001`, `fw001`, `ti001`, `ib003` | Product Bug, Automation Bug, Environment Issue, Firmware Issue, To Investigate, Infrastructure Bug | ~9 |
| Pipeline stages | `F0_RECALL` ... `F6_REPORT` | Recall, Triage, Resolve, Investigate, Correlate, Review, Report | ~12 |
| Case/RCA/Symptom IDs | `C01`-`C30`, `R1`-`R30`, `S1`-`S30` | Jira ID + test name | ~8 |
| Metric IDs | `M1`-`M20` | `m.Name` field exists (e.g. `defect_type_accuracy`) | ~3 |
| Heuristic IDs | `H1`-`H18` | `.Name` field exists (e.g. `recall-hit`) | ~4 |
| Cluster keys | `product\|ptp4l\|pb001` | Split and humanize segments | ~2 |

### Files with violations

**Go code (human-facing output):**
- `internal/calibrate/report.go` — calibration report
- `internal/calibrate/briefing.go` — batch briefing
- `internal/calibrate/tokimeter.go` — TokiMeter bill
- `internal/calibrate/analysis.go` — analysis report
- `internal/calibrate/runner.go` — `stepName()` helper
- `internal/calibrate/metrics.go` — metric detail strings
- `internal/calibrate/parallel.go` — log messages
- `cmd/asterisk/main.go` — CLI status output
- `cmd/mock-calibration-agent/main.go` — responder log
- `internal/orchestrate/heuristics.go` — explanation strings

**Documentation:**
- `.cursor/notes/jira-audit-report.md`
- `README.md`
- `.cursor/docs/subagent-cost-model.mdc`
- `.cursor/contracts/index.mdc`
- Various other contracts and notes

## Tasks

- [x] **T1** Create `.cursor/rules/human-readable-output.mdc`
- [x] **T2** Create this contract
- [x] **T3** Create `internal/display/display.go` with lookup functions + tests. *(Done — 6 families: DefectType, Stage, Metric, Heuristic, ClusterKey. 100% test coverage.)*
- [x] **T4** Apply `display.X()` to calibration formatters (report, briefing, tokimeter, analysis, runner, metrics). *(Done — 6 files updated.)*
- [x] **T5** Apply `display.X()` to CLI and agent output (main.go, mock-agent, parallel, heuristics). *(Done — 4 files updated.)*
- [x] **T6** Humanize documentation (jira-audit-report, README, contracts, notes). *(Done — 5+ docs updated.)*
- [x] **T7** Update test assertions (briefing_test, classify_test failure messages, tokimeter_test). *(Done — 3 test files updated.)*
- [x] **T8** Run `go test ./...`, fix failures, commit. *(Done — 12/12 packages pass.)*

## Completion criteria

- [ ] No raw machine code appears in any `fmt.Printf`, `fmt.Sprintf`, `log.Printf`, or markdown output that a human reads.
- [ ] `internal/display/display.go` exists with 100% coverage of all code families.
- [ ] All tests pass.
- [ ] Rule `.cursor/rules/human-readable-output.mdc` is active and indexed.
