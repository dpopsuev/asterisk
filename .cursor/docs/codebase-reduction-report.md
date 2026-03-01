# Codebase Reduction Report

**Baseline date:** 2026-02-17  
**Status:** Pre-implementation (contracts defined, not yet executed)  
**Purpose:** Track LOC before/after as the 4 architecture contracts are implemented.

## Current Baseline

### Total Codebase

| Category | LOC |
|----------|-----|
| Source (non-test `.go`) | 15,208 |
| Tests (`*_test.go`) | 5,769 |
| **Total Go** | **20,977** |
| External dependencies (`go.mod` lines) | 36 |

### Per-Package Source LOC (non-test)

| Package | LOC | % of Total |
|---------|-----|------------|
| `calibrate/` (root, 18 files) | 4,603 | 30.3% |
| `calibrate/scenarios/` | 2,704 | 17.8% |
| `store/` | 2,992 | 19.7% |
| `orchestrate/` | 1,585 | 10.4% |
| `cmd/mock-calibration-agent/` | 1,284 | 8.4% |
| `cmd/asterisk/main.go` | 880 | 5.8% |
| `rp/` | 762 | 5.0% |
| `display/` | 206 | 1.4% |
| `format/` | 165 | 1.1% |
| `preinvest/` | 92 | 0.6% |
| `wiring/` | 96 | 0.6% |
| `investigate/` | 94 | 0.6% |
| `postinvest/` | 82 | 0.5% |
| `workspace/` | 73 | 0.5% |

### `calibrate/` Breakdown (the God package)

| Concern | Files | LOC |
|---------|-------|-----|
| Data types | `types.go` | 230 |
| Circuit runner | `runner.go`, `analysis.go` | 819 |
| Parallel execution | `parallel.go` | 531 |
| Metrics scoring | `metrics.go` | 707 |
| Formatting | `report.go`, `briefing.go`, `tokimeter.go` | 447 |
| Dispatchers | `dispatcher.go`, `batch_dispatcher.go`, `token_dispatcher.go`, `batch_manifest.go` | 757 |
| Adapters | `adapter.go`, `basic_adapter.go`, `cursor_adapter.go` | 669 |
| Lifecycle | `lifecycle.go`, `cluster.go`, `tokens.go` | 443 |
| **Total** | **18 files** | **4,603** |

---

## Projected Impact (per contract)

### Contract 1: `calibrate-decomposition.md`

| Area | Before | After (est.) | Change |
|------|--------|-------------|--------|
| `calibrate/` root | 4,603 | ~2,200 | -2,403 (moved to sub-packages) |
| `calibrate/dispatch/` | — | ~757 | +757 (moved from root) |
| `calibrate/metrics/` | — | ~707 | +707 (moved from root) |
| `calibrate/adapt/` | — | ~669 | +669 (moved from root) |
| **Net LOC change** | — | — | **0** (restructure, no reduction) |
| **Architectural improvement** | 1 flat package | 4 focused packages | Separation of concerns |

### Contract 2: `cobra-cli.md`

| Area | Before | After (est.) | Reduction |
|------|--------|-------------|-----------|
| `cmd/asterisk/main.go` | 880 | ~50 | -830 |
| `cmd/asterisk/` total | 880 | ~280 | -600 (~68%) |
| Business logic extracted | ~100 LOC embedded | 0 in CLI | Moved to packages |
| **New dependency** | — | `spf13/cobra` | +1 direct dep |

### Contract 3: `critical-test-coverage.md`

| Area | Before | After (est.) | Change |
|------|--------|-------------|--------|
| Test files | 26 | ~34 | +8 new test files |
| Test LOC | 5,769 | ~7,500 | +1,731 (more tests) |
| **Net source change** | — | — | **0** (tests only) |

### Contract 4: `concurrency-modernization.md`

| Area | Before | After (est.) | Reduction |
|------|--------|-------------|-----------|
| `parallel.go` | 531 | ~330 | -201 (~38%) |
| `dispatcher.go` (FileDispatcher) | 337 | ~190 | -147 (~44%) |
| Mutable globals | 1 (`orchestrate.BasePath`) | 0 | Eliminated |
| **New dependencies** | — | `fsnotify/fsnotify` (optional) | +0–1 direct dep |
| **Net LOC reduction** | — | — | **~348** |

---

## Cumulative Projection

| Metric | Before | After All 4 Contracts | Delta |
|--------|--------|----------------------|-------|
| Source LOC | 15,208 | ~14,260 | -948 (~6.2%) |
| Test LOC | 5,769 | ~7,500 | +1,731 (+30%) |
| Packages (calibrate sub) | 1 flat (18 files) | 4 focused packages | Better SoC |
| CLI files | 1 (880 LOC) | 8 (~280 LOC) | Better DX |
| Mutable globals | 1 | 0 | Eliminated |
| Direct Go deps | ~12 | ~14 | +2 (cobra, fsnotify) |

---

## What Gets Offloaded to Libraries

| Library | Replaces | LOC Saved |
|---------|----------|-----------|
| `spf13/cobra` | Hand-rolled `flag` + `switch` + `printUsage()` | ~600 |
| `golang.org/x/sync/errgroup` | Manual goroutine pool + semaphore + channel | ~200 |
| `fsnotify/fsnotify` (optional) | Timer-based polling loop | ~150 |
| `jedib0t/go-pretty/v6` | `strings.Builder` table construction | ~300 (already done) |

**Total library offload: ~1,250 LOC** (of which 300 already done via unified-formatter).

---

## What Remains Bespoke (and why)

| Component | LOC | Justification |
|-----------|-----|---------------|
| Circuit heuristics (H1–H18) | ~400 | Deeply domain-specific; no generic rules engine fits |
| Metrics scoring (M1–M20) | ~707 | Entirely domain-specific; no library exists |
| RP client | ~762 | Proprietary API; no community client |
| Store (SQLite) | ~2,992 | Right-sized for raw SQL; ORM is overkill |
| Display labels | ~206 | Trivial domain-specific lookup table |
| Math helpers | ~50 | `mean`, `stddev`, `pearsonCorrelation` — gonum is 400K+ LOC |
| Clustering (Jaccard) | ~30 | 30 LOC; go-edlib is not worth the dependency |

---

## Bug Classes Eliminated (projected)

| Bug Class | Contract | How |
|-----------|----------|-----|
| Manual flag parsing errors | `cobra-cli.md` | Cobra handles validation, help, completion |
| Goroutine leak / race conditions | `concurrency-modernization.md` | errgroup structured concurrency |
| Polling CPU waste | `concurrency-modernization.md` | fsnotify event-driven |
| Import cycle risk | `calibrate-decomposition.md` | Clear sub-package boundaries |
| Global state corruption | `cobra-cli.md` + `concurrency-modernization.md` | Config struct replaces `orchestrate.BasePath` |

---

## Measurement Protocol

After each contract is implemented:

1. Run `find internal/ cmd/ -name '*.go' ! -name '*_test.go' | xargs wc -l | tail -1` for source LOC
2. Run `find internal/ cmd/ -name '*_test.go' | xargs wc -l | tail -1` for test LOC
3. Run `go test -race ./...` to confirm no regressions
4. Update this report with actual numbers

---

## Cross-Reference

- Architecture: `.cursor/docs/architecture.md`
- Test gap register: `.cursor/docs/test-gap-register.md`
- Off-the-shelf rule: `.cursor/rules/off-the-shelf.mdc`
- Contracts: `calibrate-decomposition.md`, `cobra-cli.md`, `critical-test-coverage.md`, `concurrency-modernization.md`
