# Boundary Map — Asterisk internal/ packages

Achilles heuristic: "Would Achilles (AI vulnerability discovery tool, second Origami consumer) need this?" If yes, the pattern is generic and belongs in Origami. If no, it stays in Asterisk.

## Classification

### Stays in Asterisk (domain-specific)

| Package | Purpose | Achilles needs? | Rationale |
|---------|---------|-----------------|-----------|
| `investigate` | Mock investigation: reads envelope, creates cases, writes RCA placeholder artifact | No | Artifact shape is RP/RCA-specific; Achilles has scan→classify→assess→report |
| `preinvest` | Pre-investigation fetch: obtains execution envelope from stub or RP API | No | Envelope shape (launch, failure list, RP attributes) is RP-specific |
| `postinvest` | Post-investigation push: records defect type to mock store, optional Jira fields | No | RP/Jira push is RP-specific; Achilles outputs SARIF/reports |
| `rp` | ReportPortal 5.11 API client (launches, items, envelope, project) | No | RP API is RP-only |
| `store` | Persistence facade: suite, pipeline, launch, job, case, triage, symptom, RCA | No | **Investigated (2026-02-25):** 30+ method interface, v1/v2 coexist, all domain-specific. v1 is alive (5 callers). No generic patterns extractable. v1 disappears when interactive runner is removed. |

### Generic pattern, domain-specific schema (migration candidates)

| Package | Purpose | Achilles needs? | Rationale |
|---------|---------|-----------------|-----------|
| `calibrate` | E2E calibration: runs F0-F6 pipeline against ground truth, measures M1-M20 metrics | Maybe | Pattern (scenario → adapter → runner → metrics) is generic; schema (defect types, RCA fields, M1-M20) is domain-specific |
| `display` | Maps machine codes to human-readable names for CLI/reports | Maybe | Pattern (code→display-name registry) is generic; codes (pb001, F0-F6) are RP-specific |
| `orchestrate` | Pipeline definition (YAML DSL), heuristics, templates, params, bridge adapters | Maybe | Pipeline orchestration is generic; F0-F6 steps and RCA heuristics are domain-specific |
| `origami` | DatasetStore, mapper, completeness (curate bridge) | Maybe | DatasetStore and completeness checks are generic; field schema is domain-specific |
| `mcpconfig` | Wraps Origami's PipelineServer with Asterisk hooks (scenarios, adapters, step schemas) | Maybe | The pattern (config + CreateSession + FormatReport + StepSchemas) is generic; the wiring is domain-specific |

## Deduplication catalog

Each candidate below has a generic pattern that Achilles would reuse. Migration means extracting the pattern into Origami and leaving the domain-specific schema in Asterisk.

### P1 — High priority (shared by any Origami consumer running calibration)

| Candidate | Current location | Generic pattern | Domain-specific part | Follow-up |
|-----------|-----------------|-----------------|---------------------|-----------|
| Calibration runner | `calibrate/runner.go`, `calibrate/parallel.go` | Scenario loading, adapter dispatch loop, parallel worker pool, per-case pipeline walk, metric scoring, report formatting | M1-M20 metric definitions, defect type enums, RP field extraction | Separate contract: extract `calibrate.Runner` interface + parallel worker to Origami |
| Metric scoring framework | `calibrate/metrics.go`, `calibrate/score.go` | Score(expected, actual) → float64 with configurable thresholds, metric registry | Metric definitions (M1-M20), field-specific comparators | Separate contract: extract `MetricScorer` interface to Origami |
| MCP config pattern | `mcpconfig/server.go` | CreateSession hook, FormatReport hook, StepSchema registration, adapter factory | Asterisk scenarios, RP adapters, F0-F6 step schemas | No migration needed — each consumer writes its own config; pattern is documented |

### P2 — Medium priority (shared by consumers with curation/dataset needs)

| Candidate | Current location | Generic pattern | Domain-specific part | Follow-up |
|-----------|-----------------|-----------------|---------------------|-----------|
| DatasetStore / FileStore | `origami/dataset.go` | CRUD for curated datasets, file-backed persistence, schema validation | Field names (defect_type, component, etc.) | Separate contract: extract to Origami's `curate` package |
| Ground truth schema + completeness | `origami/completeness.go` | Schema completeness scoring (required vs optional fields, coverage %) | Which fields are required for RCA | Bundle with DatasetStore migration |
| Display formatting | `display/display.go` | Code→human-name registry, table formatting | pb001→"Product Bug", F0→"Recall" | Separate contract: extract `DisplayRegistry` interface to Origami |

### P3 — Low priority (niche patterns, low reuse probability)

| Candidate | Current location | Generic pattern | Domain-specific part | Follow-up |
|-----------|-----------------|-----------------|---------------------|-----------|
| Investigation envelope pattern | `investigate/investigate.go` | "Fetch context → create cases → walk pipeline → write artifact" | Envelope shape, RP attributes | No separate contract — too tightly coupled to RP; Achilles builds its own |
| Heuristic edge evaluation | `orchestrate/heuristics.go` | Rule-based edge selection with threshold config | RCA-specific rules (H1-H18) | No separate contract — expression edges in YAML DSL already generalize this |
| Interactive runner | `orchestrate/runner.go` | File-based step execution (`cursor` + `save` commands) | Reimplements `framework.Runner.Walk()` | Candidate for removal, not migration |
