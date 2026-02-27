# Contract — rca-pure-dsl

**Status:** draft  
**Goal:** The entire `adapters/rca/` directory (40+ files, 6,000+ LOC) is expressed as DSL — zero Go domain logic remains in Asterisk.  
**Serves:** Polishing & Presentation (100% DSL north star)

## Contract rules

- There is no Python in an Ansible repository. Asterisk follows the same model: **zero Go code.** Any Go that cannot become YAML is a missing Origami primitive — file it, implement it, then delete the Go.
- Asterisk and Achilles are sibling playbook repositories on the same framework. Achilles does proactive security RCA; Asterisk does passive CI post-mortem RCA. Patterns extracted here must work for both.
- Go code remaining in Asterisk after this contract: **none.** The CLI entry point moves to Origami (like `ansible` is the CLI, not the playbook repo). Tests become pipeline validation YAML.

## Context

Asterisk's `adapters/rca/` is the largest non-DSL surface in the project. It contains:

- Pipeline orchestration (runner, walker, state machine)
- Extractors (LLM prompt → structured output for each pipeline step)
- Transformers (context building, prompt filling)
- Hooks (store persistence on step completion)
- Heuristic routing (now DSL via `pipeline_rca.yaml` edges — already done)
- Calibration runner (drives multi-case scoring)
- Metrics and scoring (domain-specific metric calculators)
- Report formatting
- Framework adapters (basic, stub, LLM)

The pipeline structure (`pipeline_rca.yaml`) and routing edges are already DSL. The remaining Go code falls into categories that need decomposition before DSL-ification:

1. **Extractors** — LLM prompt templates + response parsing. Could become YAML-defined extractors with prompt templates as embedded files.
2. **Transformers** — context assembly from walker state. Could become `core.jq`-style transformers or a new context-builder DSL.
3. **Hooks** — store persistence calls. Could become `sqlite.exec` transformer chains if the SQLite adapter (from `adapter-migration`) lands first.
4. **Metrics** — domain-specific scoring functions. Could become a metrics DSL in Origami's `calibrate/` package.
5. **Framework adapters** (basic, stub, LLM) — dispatch routing. Could become adapter-level configuration in YAML.
6. **Report formatting** — output generation. Could become a template/format DSL.

### Sibling pattern: Achilles

Achilles (`github.com/dpopsuev/achilles`) is a proactive security vulnerability discovery tool. Its pipeline is structurally identical: scan → classify → assess → report. Both tools:

- Walk a graph of analysis nodes
- Use LLM extractors for reasoning
- Score results against ground truth
- Produce structured reports

If `rca/` domain logic becomes DSL primitives in Origami, Achilles benefits directly. Any pattern that works only for CI RCA but not for security RCA is too domain-specific — push it to a higher abstraction.

## FSC artifacts

| Artifact | Target | Compartment |
|----------|--------|-------------|
| Origami framework gaps inventory | `docs/` | domain |
| RCA decomposition analysis | `docs/` | domain |

## Execution strategy

**Phase 1: Decomposition analysis** (design, no code changes)

Catalog every Go file in `adapters/rca/`. For each file/function, classify:

| Category | DSL target | Origami primitive needed |
|----------|-----------|------------------------|
| Already DSL | `pipeline_rca.yaml` | None |
| Extractor | YAML extractor def + prompt template | Declarative extractor DSL |
| Transformer | YAML transformer chain | Existing `core.*` or new primitive |
| Hook | `sqlite.exec` chain | SQLite adapter (from `adapter-migration`) |
| Metric | Metrics DSL | `calibrate/` metric definition DSL |
| Adapter config | adapter.yaml | Existing adapter manifest |
| Report | Template DSL | New or existing `core.template` |
| Pure glue | Delete after DSL-ification | None |

**Phase 2: Origami gaps** — implement missing primitives identified in Phase 1.

**Phase 3: Migration** — convert `rca/` files to DSL, one category at a time.

**Phase 4: Validate** — `just calibrate-stub` and `just calibrate-wet` produce identical results.

## Coverage matrix

| Layer | Applies | Rationale |
|-------|---------|-----------|
| **Unit** | yes | Each new Origami primitive gets unit tests |
| **Integration** | yes | `just calibrate-stub` must produce identical results at every step |
| **Contract** | yes | New DSL primitives need schema validation in `origami lint` |
| **E2E** | yes | `just calibrate-stub` + `just calibrate-wet` are the final gates |
| **Concurrency** | yes | Walker parallelism must work with new DSL primitives |
| **Security** | yes | LLM prompt templates are trust boundaries (injection risk) |

## Tasks

### Phase 1: Decomposition

- [ ] Catalog every file in `adapters/rca/` with LOC, category, DSL target, and Origami gap
- [ ] Catalog every file in `adapters/rca/adapt/` (framework adapter layer)
- [ ] Cross-reference with Achilles pipeline — identify shared patterns
- [ ] Produce the Origami gaps inventory (what primitives are missing)
- [ ] Design review: validate the decomposition with a Plan Mode session

### Phase 2-4: Implementation (tasks TBD after Phase 1)

Tasks will be defined after the decomposition analysis. The scope and ordering depends on what Origami primitives are missing.

### Tail

- [ ] Validate (green) — `go build`, `go test`, `just calibrate-stub`, `just test-race`
- [ ] Tune (blue) — review DSL definitions for consistency
- [ ] Validate (green) — all gates still pass

## Acceptance criteria

- **Given** the Asterisk repository, **when** listing all files, **then** zero `.go` files exist. Only YAML pipelines, scenarios, schemas, prompt templates, and configuration.
- **Given** `origami run asterisk-rca`, **when** executed against test data, **then** the output is identical to today's `just calibrate-stub`.
- **Given** Achilles's 4-node pipeline, **when** using the same Origami DSL primitives, **then** it can express its pipeline as pure YAML too.
- **Given** a new RCA-style analysis tool, **when** defining its pipeline, **then** it needs only YAML + prompt templates — zero Go code.

## Security assessment

| OWASP | Finding | Mitigation |
|-------|---------|------------|
| A03: Injection | LLM prompt templates accept pipeline variables | Sanitize variable interpolation; validate template inputs against schema |

## Notes

2026-02-27 22:15 — Contract drafted. Gate-tier: this is the final step to 100% DSL Asterisk. Phase 1 (decomposition) is design-only — no code changes. Implementation phases depend on Origami gaps identified during decomposition. Achilles is the validation sibling: if the DSL primitives work for both CI RCA and security RCA, the abstraction is correct.
