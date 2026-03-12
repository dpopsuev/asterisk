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

The framework playground is in the Origami repo: `go run ./examples/playground/` (in `github.com/dpopsuev/origami`).

## Calibration

Offline calibration uses pre-captured RP + harvester bundles (defaults to stub backend):

```bash
just calibrate-offline scenario=ptp              # stub backend (deterministic)
just calibrate-offline scenario=ptp backend=llm   # LLM backend (wet run)
```

Online calibration fetches live RP data and harvester sources:

```bash
just calibrate-online scenario=ptp backend=llm
```

## Harvester Bundles

Capture an offline harvester bundle for a domain:

```bash
just capture-harvester scenario=ptp
```

Validate a captured bundle:

```bash
just validate-bundle domain=ocp/ptp
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
docs(guide): add Adversarial Dialectic section to framework guide
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
origami.yaml                    Manifest — single entrypoint for origami fold
internal/circuits/              Circuit YAML definitions (RCA, calibration, ingest)
internal/prompts/               LLM prompt templates per circuit step
internal/scorecards/            Calibration metric scorecards
internal/datasets/              Calibration datasets (scenarios, ground truth)
internal/schema.yaml            SQLite schema definition
examples/                       Example launch data for quickstart
```

## License

Apache License 2.0. See [LICENSE](LICENSE).
