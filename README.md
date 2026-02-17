<p align="center">
  <img src="asterisk-rh.png" alt="Asterisk" width="200" />
</p>

<h1 align="center">Asterisk</h1>

<p align="center">
  Evidence-based root cause analysis for ReportPortal test failures.
</p>

---

## Overview

**Asterisk** is a standalone CLI written in Go that performs automated Root Cause Analysis (RCA) on test failures reported in [ReportPortal](https://reportportal.io/). It correlates failures with external repositories, CI pipeline context, and historical investigation data to produce explainable, evidence-first RCA reports with confidence scores.

Asterisk ingests a ReportPortal launch, runs each failure through a six-stage investigation pipeline (Recall–Report, F0–F6), and outputs structured RCA artifacts that can be pushed back to ReportPortal or exported to issue trackers.

### Who is it for?

- **QA engineers** who need fast, structured failure analysis across CI runs.
- **Triagers** who want probable root cause categories with evidence links.
- **Developers** who need failures mapped to suspect commits, repos, and components.
- **Release managers** tracking regressions across versions.

---

## Architecture

### Investigation Pipeline (Recall–Report, F0–F6)

Every failed test case flows through a heuristic-routed pipeline. At each stage, the engine evaluates rules (Recall Hit, Triage Skip Infra, Correlate Duplicate, etc.; H1–H18) to decide the next step — short-circuiting when possible, looping when evidence is insufficient.

```mermaid
flowchart LR
    F0[Recall F0] --> F1[Triage F1]
    F0 -->|"Recall Hit (H1): prior RCA match"| F5
    F1 -->|"Triage Skip Infra/Flake (H4/H5)"| F5[Review F5]
    F1 -->|"Triage Single Repo (H7)"| F3[Investigate F3]
    F1 --> F2[Resolve F2]
    F2 --> F3
    F3 -->|"Investigate Converged (H9)"| F4[Correlate F4]
    F3 -->|"Investigate Low (H10): low confidence"| F2
    F4 -->|"Correlate Duplicate (H15)"| Done((Done))
    F4 --> F5
    F5 -->|"Review Approve (H12)"| F6[Report F6]
    F5 -->|"Review Reassess (H13)"| F3
    F6 --> Done
```

| Stage | Purpose | Output |
|-------|---------|--------|
| **Recall (F0)** | Check if this failure matches a known symptom/RCA | `recall-result.json` |
| **Triage (F1)** | Classify symptom category, severity, defect type | `triage-result.json` |
| **Resolve (F2)** | Select candidate repositories for investigation | `resolve-result.json` |
| **Investigate (F3)** | Deep RCA — analyze logs, commits, code paths | `artifact.json` |
| **Correlate (F4)** | Match against other cases; detect duplicates | `correlate-result.json` |
| **Review (F5)** | Present findings; approve, reassess, or overturn | `review-decision.json` |
| **Report (F6)** | Generate Jira draft / regression table | `jira-draft.json` |

### Batch Dispatch (Multi-Subagent Mode)

When using `--dispatch=batch-file`, the CLI writes a `batch-manifest.json` that coordinates multiple parallel signals:

```
Go CLI (orchestrator)          Cursor Skill (parent agent)
       |                              |
       |-- write N signals  --------->|
       |-- write manifest   --------->|
       |-- write briefing   --------->|
       |                              |-- spawn Task(C1)
       |                              |-- spawn Task(C2)
       |                              |-- spawn Task(C3)
       |                              |-- spawn Task(C4)
       |                              |-- wait all
       |<-- read N artifacts ---------|
       |-- next batch... ------------>|
```

The parent agent reads the manifest, spawns up to 4 Task subagents, each processing one case independently. Adaptive scheduling adjusts batch size based on quality, wall-clock time, and token budget.

### Data Model

Asterisk uses a two-tier persistence model inspired by a forensic metaphor:

```mermaid
flowchart TB
    subgraph tier1 [Investigation Tree]
        Suite[Investigation Suite]
        Suite --> Launch
        Launch --> Job
        Job --> Case[Case -- the Witness]
        Case --> Triage
    end

    subgraph tier2 [Global Knowledge]
        Symptom[Symptom -- the Story]
        RCA[RCA -- the Criminal]
        Symptom <-->|"linked"| RCA
    end

    Case -.->|"identifies"| Symptom
    Case -.->|"resolves to"| RCA
```

- **Case** (witness): a single failed test -- the unit of investigation.
- **Symptom** (story): a recurring failure pattern with fingerprint and error signature.
- **RCA** (criminal): the root cause -- a specific bug, config issue, or infra fault.

---

## Features

- **ReportPortal integration** -- fetch launches, test items, and logs via the RP 5.11 API; push RCA defect updates back.
- **Prompt-driven investigation** -- each pipeline stage uses a structured prompt template (`.cursor/prompts/`), enabling AI-assisted analysis via Cursor or any LLM adapter.
- **Heuristic routing** — 18 configurable rules (Recall Hit, Triage Skip Infra, Correlate Duplicate, etc.; H1–H18) with tunable thresholds for recall confidence, convergence, loop limits, and duplicate detection.
- **Calibration framework** — run the full pipeline against ground-truth scenarios with 20 metrics (Defect Type Accuracy, Overall Accuracy, etc.; M1–M20) covering defect classification, recall accuracy, repo selection, semantic quality, and pipeline path correctness.
- **Storage layer** -- SQLite-backed persistence (via pure-Go `modernc.org/sqlite`) with an in-memory alternative for testing.
- **Context workspace** -- YAML/JSON configuration mapping failures to relevant repositories with purpose metadata.
- **File-based dispatch** -- `signal.json` polling protocol enabling automated agent communication without stdin.
- **Multi-subagent mode** -- batch dispatch with `--dispatch=batch-file` enables up to 4 parallel Cursor subagents per batch, with shared briefing, adaptive scheduling, and token budget enforcement.
- **Evidence-first outputs** -- every conclusion cites evidence (logs, commits, pipeline data) with confidence scores.

---

## Quick Start

### Prerequisites

- **Go** 1.24+
- **just** (optional, for task runner -- `cargo install just`)

### Build

```bash
# Build all binaries (using just)
just build-all

# Or with Make
make build-all

# Or directly
go build -o bin/asterisk ./cmd/asterisk/
go build -o bin/mock-calibration-agent ./cmd/mock-calibration-agent/
```

### Run an analysis

```bash
# Analyze a ReportPortal launch from a local envelope
asterisk analyze --launch=examples/pre-investigation-33195-4.21/envelope_33195_4.21.json \
                 --workspace=workspace.yaml \
                 -o /tmp/rca-artifact.json

# Analyze by launch ID (requires RP credentials)
asterisk analyze --launch=33195 \
                 --rp-base-url=https://your-rp-instance.com \
                 --rp-api-key=.rp-api-key \
                 -o /tmp/rca-artifact.json
```

### Run calibration

```bash
# Stub calibration (deterministic, no AI)
asterisk calibrate --scenario=ptp-mock --adapter=stub

# Wet calibration with file dispatcher and auto-responder
asterisk calibrate --scenario=ptp-real-ingest --adapter=cursor \
                   --dispatch=file --responder=auto --clean

# Batch calibration with multi-subagent mode (up to 4 parallel)
asterisk calibrate --scenario=ptp-real-ingest --adapter=cursor \
                   --dispatch=batch-file --batch-size=4 \
                   --responder=auto --clean --cost-report
```

---

## CLI Reference

```
asterisk <analyze|push|cursor|save|status|calibrate> [options]
```

### `analyze` — Run Recall–Report (F0–F6) pipeline

| Flag | Default | Description |
|------|---------|-------------|
| `--launch` | *(required)* | Path to envelope JSON or RP launch ID |
| `--workspace` | | Path to context workspace file (YAML/JSON) |
| `-o` | *(required)* | Output artifact path |
| `--db` | `.asterisk/asterisk.db` | Store DB path |
| `--adapter` | `basic` | Adapter: `basic` (heuristic) |
| `--rp-base-url` | | ReportPortal base URL |
| `--rp-api-key` | `.rp-api-key` | Path to RP API key file |

### `push` -- Push artifact to ReportPortal

| Flag | Default | Description |
|------|---------|-------------|
| `-f` | *(required)* | Path to the artifact file |
| `--rp-base-url` | | ReportPortal base URL |
| `--rp-api-key` | `.rp-api-key` | Path to RP API key file |

### `cursor` -- Orchestrate interactive investigation

| Flag | Default | Description |
|------|---------|-------------|
| `--launch` | *(required)* | Path to envelope JSON or launch ID |
| `--workspace` | | Path to context workspace file |
| `--case-id` | | Investigate a specific case only |
| `--prompt-dir` | `.cursor/prompts` | Prompt template directory |
| `--db` | `.asterisk/asterisk.db` | Store DB path |

### `save` -- Save artifact and advance pipeline

| Flag | Default | Description |
|------|---------|-------------|
| `-f` | *(required)* | Path to artifact file |
| `--case-id` | *(required)* | Case ID |
| `--suite-id` | *(required)* | Suite ID |
| `--db` | `.asterisk/asterisk.db` | Store DB path |

### `status` -- Show investigation state

| Flag | Default | Description |
|------|---------|-------------|
| `--case-id` | *(required)* | Case ID |
| `--suite-id` | *(required)* | Suite ID |

### `calibrate` -- Run calibration against ground truth

| Flag | Default | Description |
|------|---------|-------------|
| `--scenario` | `ptp-mock` | Scenario: `ptp-mock`, `daemon-mock`, `ptp-real`, `ptp-real-ingest` |
| `--adapter` | `stub` | Model adapter: `stub` (deterministic), `cursor` (AI) |
| `--dispatch` | `stdin` | Dispatch mode: `stdin`, `file`, `batch-file` |
| `--runs` | `1` | Number of calibration runs |
| `--prompt-dir` | `.cursor/prompts` | Prompt template directory |
| `--clean` | `true` | Remove artifacts and DB before starting |
| `--responder` | `auto` | Responder lifecycle: `auto`, `external`, `none` |
| `--agent-debug` | `false` | Verbose debug logging for dispatcher |
| `--batch-size` | `4` | Max signals per batch (batch-file mode) |
| `--parallel` | `1` | Number of parallel workers (1 = serial) |
| `--token-budget` | `0` | Max concurrent dispatches (0 = same as parallel) |
| `--cost-report` | `false` | Write token-report.json with per-case cost breakdown |

---

## Calibration

The calibration framework validates the investigation pipeline end-to-end against known ground truth.

### Scenarios

| Scenario | Cases | Description |
|----------|-------|-------------|
| `ptp-mock` | 12 | Synthetic PTP failures: holdover, cleanup, NTP across 3 OCP versions |
| `daemon-mock` | 8 | Synthetic daemon process failures: broken pipe, config hang |
| `ptp-real` | 8 | Real PTP bugs: OCPBUGS-74895 (broken pipe), OCPBUGS-74904 (config hang) |
| `ptp-real-ingest` | 30 | Ingested from CI data: 30 real PTP failures (Jan 2025 -- Feb 2026) |

### Adapters

- **Stub** -- returns ground-truth answers deterministically. Validates pipeline logic and heuristic routing. Always scores 20/20.
- **Basic** -- zero-LLM heuristic adapter. Uses keyword analysis and store lookups to produce answers in-process, without any AI. This is the primary quality baseline (M19 = 0.93 on Jira-verified ground truth).
- **Cursor** -- fills prompt templates, dispatches to an external AI agent via the configured dispatcher, and evaluates real AI outputs against ground truth.

### Metrics (M1–M20)

| Group | Metrics | What they measure |
|-------|---------|-------------------|
| Classification | Defect Type Accuracy (M1), Symptom Category Accuracy (M2) | Triage accuracy |
| Recall | Recall Hit Rate (M3), Recall False Positive Rate (M4) | Known-symptom detection |
| Knowledge | Serial Killer Detection (M5), Skip Accuracy (M6), Cascade Detection (M7) | Pattern recognition |
| Convergence | Convergence Calibration (M8) | Investigation depth quality |
| Repo Selection | Repo Selection Precision (M9), Repo Selection Recall (M10) | Repository targeting |
| Evidence | Red Herring Rejection (M11), Evidence Recall (M12), Evidence Precision (M13) | Supporting data quality |
| Semantic | RCA Message Relevance (M14), Component Identification (M15) | Output usefulness |
| Pipeline | Pipeline Path Accuracy (M16), Loop Efficiency (M17), Total Prompt Tokens (M18) | Routing correctness |
| Aggregate | Overall Accuracy (M19), Run Variance (M20) | System-level performance |

Target: **Overall Accuracy (M19) >= 0.95**.

### How calibration drives improvement

Calibration is a **deterministic Go test harness**, not a self-learning AI loop.
The pipeline does not update itself at runtime; a developer (with AI assistance)
updates the code based on calibration results.

The improvement cycle works like this:

1. **Run calibration** — `asterisk calibrate --scenario=ptp-mock --adapter=basic`
   produces a metrics report (M1–M20) comparing pipeline output to ground truth.
2. **Read the metrics** — identify which metrics dropped and on which cases.
3. **Diagnose** — trace the failing cases through the pipeline to find the root
   cause (wrong heuristic threshold, missing keyword, bad prompt template, etc.).
4. **Fix the code** — update heuristics, prompt templates, adapter logic, or
   scenario data. This is a normal code change committed to git.
5. **Re-run calibration** — confirm the fix improved the target metric without
   regressing others. Repeat until the target is met.

The AI assists in steps 2–4 (analyzing results, proposing fixes, writing code),
but the calibration harness itself is plain Go — reproducible, deterministic,
and version-controlled. Every improvement is a git commit, not a hidden model
update.

**Current baselines:**

| Adapter | M19 (Overall Accuracy) | Notes |
|---------|------------------------|-------|
| Stub    | 1.00 (20/20)           | Perfect by construction |
| Basic   | 0.93                   | Zero-LLM heuristic baseline |
| Cursor  | TBD                    | Pending MCP integration |

---

## Project Structure

```
asterisk/
├── cmd/
│   ├── asterisk/              # Main CLI (analyze, push, cursor, save, status, calibrate)
│   ├── mock-calibration-agent/ # Mock agent for automated calibration (testing only)
│   └── run-mock-flow/         # Dev tool: run mock fetch→analyze→push flow
├── internal/
│   ├── calibrate/             # Calibration runner, adapters, dispatchers, metrics
│   │   └── scenarios/         # Ground-truth scenarios (ptp-mock, daemon-mock, ptp-real, ...)
│   ├── orchestrate/           # Recall–Report (F0–F6) pipeline engine, heuristics (H1–H18), state, templates
│   ├── store/                 # Persistence: Store interface, SqlStore (SQLite), MemStore
│   ├── rp/                    # ReportPortal API client (fetch, push, project/launch scopes)
│   ├── preinvest/             # Pre-investigation: envelope fetch and storage
│   ├── investigate/           # Investigation: envelope → artifact analysis
│   ├── postinvest/            # Post-investigation: push results to RP
│   ├── workspace/             # Context workspace: repo mappings (YAML/JSON)
│   └── wiring/                # End-to-end mock flow (Ginkgo integration tests)
├── examples/                  # Fixture data (launch 33195 envelope + items)
├── .cursor/
│   ├── prompts/               # Recall–Report (F0–F6) prompt templates (Markdown)
│   ├── contracts/             # Work contracts (execution plans)
│   ├── docs/                  # Deep references (data model, envelope, artifacts)
│   ├── notes/                 # Short summaries (PoC constraints, workspace structure)
│   ├── guide/                 # Dev workflows (test matrix, BDD templates)
│   ├── strategy/              # Invariants (evidence-first RCA)
│   ├── glossary/              # Domain terminology
│   ├── goals/                 # PoC and MVP goal checklists
│   ├── skills/                # Cursor agent skills (investigate, bootstrap, ...)
│   ├── rules/                 # Development rules (Go testing, security, scenarios)
│   └── security-cases/        # OWASP security findings (SEC-001 through SEC-005)
├── .dev/                      # Private dev data (git-ignored): scripts, calibration runs
├── justfile                   # Task runner (just) — primary
├── Makefile                   # Build and test targets (legacy)
├── roadmap.md                 # Phased roadmap with user stories
└── go.mod                     # Go 1.24, SQLite, Ginkgo, Gomega
```

---

## Development

### Methodology

Asterisk follows **BDD + TDD + AI**:

1. **Gherkin first** -- write acceptance criteria (Given/When/Then) for every story.
2. **Test matrix** -- unit, integration, contract, E2E, and security tests per feature.
3. **Red-Green-Blue** -- fail (red), pass (green), tune prompts/refactor (blue).
4. **Calibration loop** -- run scenarios, diagnose weakest metrics, apply fixes, re-run. The AI assists analysis and code changes; the harness itself is deterministic Go. See [How calibration drives improvement](#how-calibration-drives-improvement).

### Testing

```bash
# All Go tests
just test

# Ginkgo BDD suites (all)
just test-ginkgo

# Ginkgo wiring suite only
just test-ginkgo-wiring

# All checks: vet + lint + staticcheck
just check

# Full cleanup: binaries + runtime + orphan processes
just clean
```

### Prompt Templates

Investigation prompts live in `.cursor/prompts/` organized by pipeline stage:

| Directory | Stage | Template |
|-----------|-------|----------|
| `recall/` | Recall (F0) | `judge-similarity.md` |
| `triage/` | Triage (F1) | `classify-symptoms.md` |
| `resolve/` | Resolve (F2) | `select-repo.md` |
| `investigate/` | Investigate (F3) | `deep-rca.md` |
| `correlate/` | Correlate (F4) | `match-cases.md` |
| `review/` | Review (F5) | `present-findings.md` |
| `report/` | Report (F6) | `regression-table.md` |

---

## Roadmap

| Phase | Focus | Status |
|-------|-------|--------|
| **0 -- Foundations** | Ingest RP failures, map to repositories | Done |
| **1 -- Evidence Gathering** | Commit history, CI pipeline context, calibration framework | Done |
| **2 -- Parallel & Multi-Agent** | Parallel pipeline, batch dispatch, multi-subagent skill | Done |
| **3 — Calibration Victory** | Reach Overall Accuracy (M19) >= 0.95 across all scenarios | In progress |
| **4 -- Reporting** | Export to RP / GitHub, learn from outcomes | Planned |
| **5 -- Advanced** | Flaky test detection, cluster metrics, NL explanations | Future |

---

## Quality Gates

- No PII or secrets in outputs (redaction required).
- Deterministic outputs for the same inputs (seeded ordering).
- All external API calls have timeouts and retries with exponential backoff.
- Partial results with clear errors when data sources are unavailable.
- Every AI-generated conclusion must cite evidence (logs, commits, pipeline data).
- Security evaluated against OWASP Top 10 before shipping.

---

## License

*License information to be added.*
