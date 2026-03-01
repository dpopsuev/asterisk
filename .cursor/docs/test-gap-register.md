# Test Gap Register

Severity-rated test coverage gaps with function-level detail. Updated as part of the architecture analysis.

## Severity Definitions

| Severity | Meaning |
|----------|---------|
| **Critical** | Core domain logic with zero unit tests; only exercised via integration. Failure risk: regressions go undetected. |
| **High** | External boundary (HTTP, file I/O) or complex classification logic with no isolated tests. Failure risk: integration breakage. |
| **Medium** | Formatting/output functions or internal state mutations with no direct tests. Failure risk: cosmetic regressions, silent state corruption. |
| **Low** | Helper functions or trivial adapters with adequate implicit coverage. |

## Gap Register

### Critical Gaps

#### GAP-C1: `calibrate/metrics.go` — Zero unit tests for M1–M20 metric scorers

- **File:** `internal/calibrate/metrics.go` (~710 LOC)
- **Functions:** All 20 metric scorer functions (e.g., `scoreDefectTypeAccuracy`, `scoreRCARelevance`, `scoreEvidenceQuality`, etc.)
- **Current coverage:** Zero unit tests. Only exercised transitively via `parallel_test.go` integration.
- **Risk:** Any regression in a single metric scorer goes undetected until a full calibration run breaks.
- **Remediation:** Create `metrics_test.go` with table-driven tests for each M1–M20 scorer using fixture data from existing scenarios.
- **Contract:** `critical-test-coverage.md`

#### GAP-C2: `calibrate/runner.go` — No unit tests for RunCalibration / runSingleCalibration

- **File:** `internal/calibrate/runner.go` (~450 LOC)
- **Functions:** `RunCalibration`, `runSingleCalibration`
- **Current coverage:** Only via `parallel_test.go` which tests the parallel wrapper, not the core runner logic.
- **Risk:** Circuit sequencing bugs, state transition errors.
- **Remediation:** Create `runner_test.go` with minimal store + stub adapter, testing circuit step sequencing and state transitions.
- **Contract:** `critical-test-coverage.md`

#### GAP-C3: `calibrate/analysis.go` — No test for RunAnalysis

- **File:** `internal/calibrate/analysis.go` (~370 LOC)
- **Functions:** `RunAnalysis`, `FormatAnalysisReport`
- **Current coverage:** Zero tests. `RunAnalysis` drives the full F0–F6 analysis flow.
- **Risk:** Analysis circuit breakage goes undetected.
- **Remediation:** Unit test with stub adapter + fixture envelope.
- **Contract:** `critical-test-coverage.md`

### High Gaps

#### GAP-H1: `rp/pusher.go` — No httptest coverage for Pusher.Push

- **File:** `internal/rp/pusher.go`
- **Functions:** `Pusher.Push` (or equivalent defect update method)
- **Current coverage:** No `httptest` coverage. RP push logic is only tested against real RP.
- **Risk:** HTTP error handling, retry logic, request serialization bugs.
- **Remediation:** Create `pusher_test.go` with `httptest.NewServer` stubs.
- **Contract:** `critical-test-coverage.md`

#### GAP-H2: `rp/fetcher.go` — No httptest coverage for Fetcher.Fetch

- **File:** `internal/rp/fetcher.go`
- **Functions:** `Fetcher.Fetch` (envelope fetch)
- **Current coverage:** No `httptest` coverage.
- **Risk:** Response parsing, error handling, pagination bugs.
- **Remediation:** Create `fetcher_test.go` with `httptest.NewServer` stubs and fixture JSON responses.
- **Contract:** `critical-test-coverage.md`

#### GAP-H3: `calibrate/basic_adapter.go` — No tests for BasicAdapter keyword classifier

- **File:** `internal/calibrate/basic_adapter.go` (~250 LOC)
- **Functions:** `BasicAdapter.Classify`, keyword matching, confidence scoring
- **Current coverage:** Zero tests. Critical classification logic untested.
- **Risk:** Classification regressions directly impact M19 (Overall Accuracy).
- **Remediation:** Create `basic_adapter_test.go` with table-driven tests covering keyword match, no-match, edge cases.
- **Contract:** `critical-test-coverage.md`

### Medium Gaps

#### GAP-M1: `calibrate/report.go` — No output tests for FormatReport / FormatAnalysisReport

- **File:** `internal/calibrate/report.go` (~200 LOC)
- **Functions:** `FormatReport`, `FormatAnalysisReport`
- **Current coverage:** No direct output tests.
- **Risk:** Cosmetic regressions in calibration reports.
- **Remediation:** Snapshot or substring-based output tests.
- **Contract:** `critical-test-coverage.md`

#### GAP-M2: `orchestrate/runner.go` — No direct tests for ApplyStoreEffects / SaveArtifactAndAdvance

- **File:** `internal/orchestrate/runner.go`
- **Functions:** `ApplyStoreEffects`, `SaveArtifactAndAdvance`
- **Current coverage:** Only tested transitively via orchestrate integration tests.
- **Risk:** Silent state corruption in store operations.
- **Remediation:** Direct unit tests with mock store.
- **Contract:** `critical-test-coverage.md`

## Coverage Targets

| Severity | Current | Target | Timeline |
|----------|---------|--------|----------|
| Critical (3 gaps) | 0% unit | 80%+ unit | Contract 3 (`critical-test-coverage.md`) |
| High (3 gaps) | 0% isolated | 70%+ isolated | Contract 3 |
| Medium (2 gaps) | 0% direct | 60%+ direct | Contract 3 |

## Cross-Reference

- Architecture: `.cursor/docs/architecture.md`
- Test methodology: `.cursor/rules/testing-methodology.mdc`
- Security coverage: `.cursor/rules/security-analysis.mdc`
- Implementation contract: `.cursor/contracts/critical-test-coverage.md`
