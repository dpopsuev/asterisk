# Contributing to Asterisk

## Prerequisites

- **Go 1.24+**
- **make** (GNU Make)

## Build

```bash
make build        # builds bin/asterisk
make build-all    # builds all binaries
```

## Test

```bash
make test                # go test ./...
make test-ginkgo         # Ginkgo specs (BDD)
make test-ginkgo-wiring  # wiring suite only
```

## Run the Framework Playground

No ReportPortal needed:

```bash
make playground
```

Or directly:

```bash
go run ./examples/framework/
```

## Calibration

Stub calibration runs entirely offline (no RP, no AI):

```bash
bin/asterisk calibrate --scenario=ptp-mock --adapter=stub
```

## Commit Conventions

We use [Conventional Commits](https://www.conventionalcommits.org/):

```
<type>(<scope>): <imperative summary>
```

**Types:** `feat`, `fix`, `refactor`, `docs`, `test`, `chore`, `perf`, `ci`  
**Scope:** the most specific package or area (e.g. `framework`, `calibrate`, `cli`)

Examples:

```
feat(framework): add Mask of Correlation for cross-case matching
fix(calibrate): correct M19 computation for edge cases
docs(guide): add Shadow Court section to framework guide
```

## Development Cycle

We follow **Red-Orange-Green-Yellow-Blue**:

1. **Red** — write a failing test
2. **Orange** — add error/anomaly logging
3. **Green** — implement until the test passes
4. **Yellow** — add success/decision logging
5. **Blue** — refactor

See `.cursor/rules/testing-methodology.mdc` for details.

## Project Structure

```
cmd/asterisk/          CLI entrypoint
internal/framework/    Agentic pipeline framework (zero domain imports)
internal/orchestrate/  Step runner and heuristics
internal/calibrate/    Calibration (M1-M20 metrics, scenarios)
internal/curate/       Curation pipeline
pipelines/             YAML pipeline definitions
examples/framework/    Framework playground (start here)
docs/                  Developer guides
```

## License

Apache License 2.0. See [LICENSE](LICENSE).
