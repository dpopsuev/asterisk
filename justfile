# Asterisk — task runner
# Run `just` with no args to see available recipes.

set dotenv-load := false

bin_dir     := "bin"
cmd_asterisk := "./cmd/asterisk"
cmd_mock     := "./cmd/run-mock-flow"
db_path      := ".asterisk/asterisk.db"
calib_dir    := ".asterisk/calibrate"

# ─── Default ──────────────────────────────────────────────

# List available recipes
default:
    @just --list

# ─── Build ────────────────────────────────────────────────

# Build the asterisk CLI
build:
    @mkdir -p {{ bin_dir }}
    go build -o {{ bin_dir }}/asterisk {{ cmd_asterisk }}

# Build all binaries (asterisk + mock-flow)
build-all: build
    go build -o {{ bin_dir }}/run-mock-flow {{ cmd_mock }}

# Install asterisk to GOPATH/bin
install:
    go install {{ cmd_asterisk }}

# ─── Test ─────────────────────────────────────────────────

# Run all Go tests
test:
    go test ./...

# Run all Go tests with verbose output
test-v:
    go test -v ./...

# Run all Ginkgo BDD suites
test-ginkgo:
    go run github.com/onsi/ginkgo/v2/ginkgo -r

# Run only the wiring Ginkgo suite
test-ginkgo-wiring:
    go run github.com/onsi/ginkgo/v2/ginkgo ./internal/wiring/...

# Run tests for a specific package (e.g. just test-pkg orchestrate)
test-pkg pkg:
    go test -v ./internal/{{ pkg }}/...

# ─── Lint ─────────────────────────────────────────────────

# Run go vet
vet:
    go vet ./...

# Run golangci-lint
lint:
    golangci-lint run ./...

# Run staticcheck
staticcheck:
    staticcheck ./...

# All checks: vet + lint + staticcheck
check: vet lint staticcheck

# ─── Calibration ─────────────────────────────────────────

# Run stub calibration (deterministic, no AI)
calibrate-stub scenario="ptp-mock":
    go run {{ cmd_asterisk }} calibrate --scenario={{ scenario }} --adapter=stub

# Run wet calibration with file dispatch
calibrate-wet scenario="ptp-real-ingest":
    go run {{ cmd_asterisk }} calibrate \
        --scenario={{ scenario }} \
        --adapter=cursor \
        --dispatch=file \
        --clean

# Run wet calibration with debug logging
calibrate-debug scenario="ptp-real-ingest":
    go run {{ cmd_asterisk }} calibrate \
        --scenario={{ scenario }} \
        --adapter=cursor \
        --dispatch=file \
        --clean \
        --agent-debug

# Run stub calibration with parallel workers
calibrate-parallel scenario="ptp-mock" workers="4":
    go run {{ cmd_asterisk }} calibrate --scenario={{ scenario }} --adapter=stub --parallel={{ workers }}

# Run wet calibration with token/cost report
calibrate-cost scenario="ptp-real-ingest":
    go run {{ cmd_asterisk }} calibrate \
        --scenario={{ scenario }} \
        --adapter=cursor \
        --dispatch=file \
        --clean \
        --cost-report

# Run wet calibration with batch-file dispatch for multi-subagent mode
calibrate-batch scenario="ptp-real-ingest" batch="4":
    go run {{ cmd_asterisk }} calibrate \
        --scenario={{ scenario }} \
        --adapter=cursor \
        --dispatch=batch-file \
        --batch-size={{ batch }} \
        --clean \
        --cost-report

# Run wet calibration and save results to .dev/calibration-runs/
calibrate-save scenario="ptp-real-ingest" round="":
    #!/usr/bin/env bash
    set -euo pipefail
    output=$(go run {{ cmd_asterisk }} calibrate \
        --scenario={{ scenario }} \
        --adapter=cursor \
        --dispatch=file \
        --clean 2>&1)
    echo "$output"
    if [ -n "{{ round }}" ]; then
        dest=".dev/calibration-runs/round-{{ round }}-results.txt"
        mkdir -p .dev/calibration-runs
        echo "$output" > "$dest"
        echo "--- Saved to $dest ---"
    fi

# ─── Clean ────────────────────────────────────────────────

# Remove build artifacts
clean-bin:
    rm -rf {{ bin_dir }}

# Remove runtime artifacts (DB + calibration dir)
clean-runtime:
    rm -f {{ db_path }} {{ db_path }}-journal
    rm -rf {{ calib_dir }}

# Remove stray root-level binaries (safety net)
clean-stray:
    rm -f asterisk signal-responder

# Full cleanup: binaries + runtime + stray
clean: clean-bin clean-runtime clean-stray

# ─── Run ──────────────────────────────────────────────────

# Run an analysis on the example launch
run-example:
    just build
    ./{{ bin_dir }}/asterisk analyze \
        --launch=examples/pre-investigation-33195-4.21/envelope_33195_4.21.json \
        -o /tmp/asterisk-artifact.json

# ─── Data Ingestion (Python scripts in .dev/) ────────────

# Ingest CI results HTML into ptp-cases.json
ingest:
    cd .dev && python3 scripts/ingest_ci_results.py

# Select diverse calibration cases from ingested data
select-cases:
    cd .dev && python3 scripts/select_cases.py

# Generate Go scenario file from selected cases
generate-scenario:
    cd .dev && python3 scripts/generate_scenario.py

# Full ingestion pipeline: ingest → select → generate
ingest-all: ingest select-cases generate-scenario
