# Asterisk — task runner
# Run `just` with no args to see available recipes.

set dotenv-load := false

bin_dir          := "bin"
binary           := bin_dir / "asterisk"
state_dir    := env("XDG_STATE_HOME", env("HOME", "") / ".local/state") / "asterisk"
db_path      := state_dir / "asterisk.db"
calib_dir    := state_dir / "calibrate"
origami_dir  := "../origami"
rca_test_pkg := origami_dir / "schematics/rca/mcpconfig"

# ─── Default ──────────────────────────────────────────────

# List available recipes
default:
    @just --list

# ─── Build ────────────────────────────────────────────────

# Build unified binary via origami fold
build:
    origami fold

# ─── Test ─────────────────────────────────────────────────

# (Tests live in Origami — run `go test ./...` in Origami repo)

# ─── Lint ─────────────────────────────────────────────────

# Lint pipeline YAMLs
lint:
    origami lint --profile strict circuits/*.yaml

# ─── Calibration ─────────────────────────────────────────
# Calibration and analysis now run as go test wrappers in Origami.
# Wet calibration uses the MCP server via Papercup skill.

# Run stub calibration (deterministic, no AI)
calibrate-stub scenario="ptp-mock":
    CALIBRATE_SCENARIO={{ scenario }} CALIBRATE_BACKEND=stub \
        go test -run TestCalibrate_PTPMock -v ./{{ rca_test_pkg }}/ -timeout 2m

# Run analysis on a local envelope file (heuristic backend)
analyze envelope:
    ANALYZE_ENVELOPE={{ envelope }} \
        go test -run TestAnalyze_Heuristic -v ./{{ rca_test_pkg }}/ -timeout 2m

# ─── Container ───────────────────────────────────────────

# Build domain-serve image via origami fold --domain-only --container
container-build:
    origami fold --domain-only --container

# Full deploy: build domain image, build Origami images, start compose stack
deploy:
    just container-build
    cd {{ origami_dir }} && just build-images
    docker compose -f {{ origami_dir }}/deploy/docker-compose.yaml up -d

# Stop the compose stack
deploy-stop:
    docker compose -f {{ origami_dir }}/deploy/docker-compose.yaml down

# Rebuild domain + restart the full stack
deploy-restart:
    just deploy-stop
    just deploy

# Tail all compose service logs
deploy-logs:
    docker compose -f {{ origami_dir }}/deploy/docker-compose.yaml logs -f

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

# Run an analysis on an envelope (heuristic backend)
run-example envelope="examples/envelope.json":
    just analyze {{ envelope }}

# ─── Data Ingestion (Python scripts in .dev/) ────────────

# Ingest CI results HTML into ptp-cases.json
ingest:
    cd .dev && python3 scripts/ingest_ci_results.py

# Select diverse calibration cases from ingested data
select-cases:
    cd .dev && python3 scripts/select_cases.py

# Full ingestion pipeline: ingest → select
ingest-all: ingest select-cases
