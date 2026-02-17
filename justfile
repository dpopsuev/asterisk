# Asterisk — task runner
# Run `just` with no args to see available recipes.

set dotenv-load := false

bin_dir     := "bin"
cmd_asterisk := "./cmd/asterisk"
cmd_responder := "./cmd/mock-calibration-agent"
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

# Build the mock calibration agent
build-responder:
    @mkdir -p {{ bin_dir }}
    go build -o {{ bin_dir }}/mock-calibration-agent {{ cmd_responder }}

# Build all binaries (asterisk + mock-calibration-agent + mock-flow)
build-all: build build-responder
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

# Run wet calibration with file dispatch + auto-responder
calibrate-wet scenario="ptp-real-ingest":
    go run {{ cmd_asterisk }} calibrate \
        --scenario={{ scenario }} \
        --adapter=cursor \
        --dispatch=file \
        --responder=auto \
        --clean

# Run wet calibration with debug logging
calibrate-debug scenario="ptp-real-ingest":
    go run {{ cmd_asterisk }} calibrate \
        --scenario={{ scenario }} \
        --adapter=cursor \
        --dispatch=file \
        --responder=auto \
        --clean \
        --agent-debug

# Run wet calibration and save results to .dev/calibration-runs/
calibrate-save scenario="ptp-real-ingest" round="":
    #!/usr/bin/env bash
    set -euo pipefail
    output=$(go run {{ cmd_asterisk }} calibrate \
        --scenario={{ scenario }} \
        --adapter=cursor \
        --dispatch=file \
        --responder=auto \
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
    rm -f asterisk mock-calibration-agent signal-responder

# Kill orphaned mock-calibration-agent processes
kill-responders:
    -pkill -f "mock-calibration-agent" 2>/dev/null || true
    -pkill -f "go run.*mock-calibration-agent" 2>/dev/null || true

# Full cleanup: binaries + runtime + stray + orphan processes
clean: clean-bin clean-runtime clean-stray kill-responders

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
