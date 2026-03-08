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

# Build the OCI image (rebuilds binary first)
container-build:
    just build
    docker build -t asterisk .

# Start container (MCP :9100, Kami :3001)
container-run:
    docker run -d --name asterisk-server \
        -p 9100:9100 -p 3001:3001 \
        {{ binary }} --transport http --kami-port 3001

# Stop and remove container
container-stop:
    docker rm -f asterisk-server

# Hot-swap: rebuild binary + image, restart container
container-restart:
    -just container-stop
    just container-build
    just container-run

# Tail container logs
container-logs:
    docker logs -f asterisk-server

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
