# Asterisk — task runner
# Run `just` with no args to see available recipes.

set dotenv-load := false

bin_dir          := "bin"
asterisk         := bin_dir / "asterisk"
db_path      := ".asterisk/asterisk.db"
calib_dir    := ".asterisk/calibrate"

# ─── Default ──────────────────────────────────────────────

# List available recipes
default:
    @just --list

# ─── Build ────────────────────────────────────────────────

# Build via origami fold (YAML manifest → binary)
build:
    origami fold --output {{ asterisk }}

# ─── Test ─────────────────────────────────────────────────

# (Tests live in Origami — run `go test ./...` in Origami repo)

# ─── Lint ─────────────────────────────────────────────────

# Lint pipeline YAMLs
lint:
    origami lint --profile strict circuits/*.yaml

# ─── Calibration ─────────────────────────────────────────

# Run stub calibration (deterministic, no AI)
calibrate-stub scenario="ptp-mock":
    just build
    {{ asterisk }} calibrate --scenario={{ scenario }} --adapter=stub

# Run wet calibration with file dispatch
calibrate-wet scenario="ptp-real-ingest":
    just build
    {{ asterisk }} calibrate \
        --scenario={{ scenario }} \
        --adapter=llm \
        --dispatch=file \
        --clean

# Run wet calibration with debug logging
calibrate-debug scenario="ptp-real-ingest":
    just build
    {{ asterisk }} calibrate \
        --scenario={{ scenario }} \
        --adapter=llm \
        --dispatch=file \
        --clean \
        --agent-debug

# Run stub calibration with parallel workers
calibrate-parallel scenario="ptp-mock" workers="4":
    just build
    {{ asterisk }} calibrate --scenario={{ scenario }} --adapter=stub --parallel={{ workers }}

# Run wet calibration with token/cost report
calibrate-cost scenario="ptp-real-ingest":
    just build
    {{ asterisk }} calibrate \
        --scenario={{ scenario }} \
        --adapter=llm \
        --dispatch=file \
        --clean \
        --cost-report

# Run wet calibration with batch-file dispatch for multi-subagent mode
calibrate-batch scenario="ptp-real-ingest" batch="4":
    just build
    {{ asterisk }} calibrate \
        --scenario={{ scenario }} \
        --adapter=llm \
        --dispatch=batch-file \
        --batch-size={{ batch }} \
        --clean \
        --cost-report

# Run wet calibration and save results to .dev/calibration-runs/
calibrate-save scenario="ptp-real-ingest" round="":
    #!/usr/bin/env bash
    set -euo pipefail
    just build
    output=$({{ asterisk }} calibrate \
        --scenario={{ scenario }} \
        --adapter=llm \
        --dispatch=file \
        --clean 2>&1)
    echo "$output"
    if [ -n "{{ round }}" ]; then
        dest=".dev/calibration-runs/round-{{ round }}-results.txt"
        mkdir -p .dev/calibration-runs
        echo "$output" > "$dest"
        echo "--- Saved to $dest ---"
    fi

# Run E2E calibration with RP-sourced cases (4 live from RP, 26 embedded)
calibrate-e2e scenario="ptp-real-ingest":
    just build
    {{ asterisk }} calibrate \
        --scenario={{ scenario }} \
        --adapter=basic \
        --rp-base-url https://your-reportportal.example.com \
        --rp-api-key .rp-api-key

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
    {{ asterisk }} analyze \
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
