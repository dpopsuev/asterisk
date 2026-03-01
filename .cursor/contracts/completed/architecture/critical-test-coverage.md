# Contract: Critical Test Coverage

**Priority:** HIGH — quality  
**Status:** Complete  
**Depends on:** Ideally after `calibrate-decomposition.md` (metrics in separate package), but can start in parallel

## Goal

Close the 3 critical and 5 high/medium severity test coverage gaps identified in the architecture analysis. Target: 80%+ unit coverage for critical paths, 70%+ for high, 60%+ for medium.

## Gap Summary

| ID | Severity | File | Functions | Current | Target |
|----|----------|------|-----------|---------|--------|
| GAP-C1 | Critical | `calibrate/metrics.go` | 20 metric scorers (M1–M20) | 0% unit | 80%+ |
| GAP-C2 | Critical | `calibrate/runner.go` | `RunCalibration`, `runSingleCalibration` | 0% unit | 80%+ |
| GAP-C3 | Critical | `calibrate/analysis.go` | `RunAnalysis` | 0% unit | 80%+ |
| GAP-H1 | High | `rp/pusher.go` | `Pusher.Push` | 0% isolated | 70%+ |
| GAP-H2 | High | `rp/fetcher.go` | `Fetcher.Fetch` | 0% isolated | 70%+ |
| GAP-H3 | High | `calibrate/basic_adapter.go` | `BasicAdapter.Classify` | 0% unit | 70%+ |
| GAP-M1 | Medium | `calibrate/report.go` | `FormatReport`, `FormatAnalysisReport` | 0% direct | 60%+ |
| GAP-M2 | Medium | `orchestrate/runner.go` | `ApplyStoreEffects`, `SaveArtifactAndAdvance` | 0% direct | 60%+ |

## Tasks

### Phase 1: Critical gaps (Red → Green → Blue)

#### GAP-C1: Metrics scorers
1. **Red:** Create `internal/calibrate/metrics_test.go` (or `metrics/metrics_test.go` if decomposed)
2. Table-driven tests for each M1–M20 scorer
3. Use fixture data extracted from existing `ptp_mock` and `daemon_mock` scenarios
4. Test edge cases: empty results, all-pass, all-fail, partial
5. **Green:** All 20 scorers return expected values for fixtures
6. **Blue:** Refactor any scorer that's hard to test in isolation

#### GAP-C2: Runner
1. **Red:** Create `internal/calibrate/runner_test.go`
2. Test `runSingleCalibration` with:
   - Minimal in-memory store
   - Stub adapter (returns fixed classification)
   - Single-case scenario
3. Verify: correct circuit step sequence, state transitions, artifact generation
4. **Green:** Runner produces expected `CaseResult` for stub scenario
5. **Blue:** Extract any untestable logic into helper functions

#### GAP-C3: Analysis
1. **Red:** Create `internal/calibrate/analysis_test.go`
2. Test `RunAnalysis` with stub adapter + fixture envelope
3. Verify: analysis report generated, per-case breakdown present
4. **Green:** Analysis produces expected output structure
5. **Blue:** Simplify analysis flow if test reveals coupling

### Phase 2: High gaps

#### GAP-H1: RP Pusher
1. Create `internal/rp/pusher_test.go`
2. Use `httptest.NewServer` to simulate RP defect update endpoint
3. Test: successful push, 4xx error, 5xx retry, timeout, malformed response
4. Verify request body serialization

#### GAP-H2: RP Fetcher
1. Create `internal/rp/fetcher_test.go`
2. Use `httptest.NewServer` with fixture JSON responses
3. Test: successful fetch, pagination, 404, timeout, malformed JSON
4. Verify envelope construction from API response

#### GAP-H3: BasicAdapter
1. Create `internal/calibrate/basic_adapter_test.go`
2. Table-driven tests: keyword match scenarios, no-match, edge cases
3. Test confidence scoring logic
4. Verify defect type classification for known patterns

### Phase 3: Medium gaps

#### GAP-M1: Report formatting
1. Create output tests for `FormatReport` and `FormatAnalysisReport`
2. Verify: table structure, metric display, per-case breakdown
3. Snapshot-based or substring assertions

#### GAP-M2: Orchestrate runner effects
1. Create unit tests for `ApplyStoreEffects` and `SaveArtifactAndAdvance`
2. Use mock store to verify state mutations
3. Verify: correct entity creation, state advancement

## Completion Criteria

- All 8 gaps have dedicated test files
- Critical gaps: >= 80% branch coverage
- High gaps: >= 70% branch coverage
- Medium gaps: >= 60% branch coverage
- Full test suite passes: `go test ./...`
- No regressions in existing tests

## Cross-Reference
- Test gap register: `.cursor/docs/test-gap-register.md`
- Testing methodology: `.cursor/rules/testing-methodology.mdc`
- Architecture: `.cursor/docs/architecture.md`
- Calibrate decomposition: `.cursor/contracts/calibrate-decomposition.md`
