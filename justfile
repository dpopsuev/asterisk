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

# Run offline calibration (pre-captured RP + knowledge bundles)
calibrate-offline scenario="ptp" backend="stub":
    cd {{ origami_dir }} && \
    CALIBRATE_SCENARIO={{ scenario }} CALIBRATE_BACKEND={{ backend }} CALIBRATE_MODE=offline \
        go test -run TestCalibrate -v ./schematics/rca/mcpconfig/ -timeout 5m

# Run online calibration (live RP + knowledge fetch)
calibrate-online scenario="ptp" backend="stub":
    cd {{ origami_dir }} && \
    CALIBRATE_SCENARIO={{ scenario }} CALIBRATE_BACKEND={{ backend }} CALIBRATE_MODE=online \
        go test -run TestCalibrate -v ./schematics/rca/mcpconfig/ -timeout 10m

# Run analysis on a local envelope file (heuristic backend)
analyze envelope:
    cd {{ origami_dir }} && \
    ANALYZE_ENVELOPE={{ envelope }} \
        go test -run TestAnalyze_Heuristic -v ./schematics/rca/mcpconfig/ -timeout 2m

# ─── Harvester Bundles ────────────────────────────────────

# Capture a harvester bundle for a domain scenario
capture-harvester scenario="ptp" domain="ocp/ptp":
    origami capture --schematic=harvester \
        --source-pack=domains/{{ domain }}/sources/{{ scenario }}.yaml \
        --output=domains/{{ domain }}/offline/harvester/ \
        --overwrite -v

# Validate an offline bundle (RP + harvester)
validate-bundle domain="ocp/ptp":
    origami validate-bundle --path=domains/{{ domain }}/offline/harvester/

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
