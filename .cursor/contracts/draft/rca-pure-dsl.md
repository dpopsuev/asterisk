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

The pipeline structure (`pipeline_rca.yaml`) and routing edges are already DSL. The remaining Go code falls into categories that need decomposition — and the right decomposition target is **marbles**, not just "YAML configs."

### Marble discovery

The 6,000 lines of Go in `rca/` are not random domain code. They encode reusable analysis patterns that repeat across any RCA-style tool. The decomposition should identify **candidate marbles** — composable subgraph building blocks that both Asterisk and Achilles import:

| Pattern | Current Go | Marble candidate | Reuse signal |
|---------|-----------|-----------------|-------------|
| LLM extraction | Extractors: prompt template → structured output | `llm-extract` marble (prompt + schema → parsed result) | Both tools send prompts and parse structured responses |
| Context assembly | Transformers: walker state → prompt variables | `context-builder` marble (state + template → filled prompt) | Both tools build prompts from accumulated evidence |
| Store persistence | Hooks: step completion → save to DB | `persist` marble (artifact → sqlite.exec chain) | Both tools persist results per step |
| Scoring | Metrics: results → quality scores | `score` marble (results + scorecard → metric values) | Both tools evaluate output quality |
| Report generation | Report formatting: analysis → structured output | `report` marble (scored results → formatted report) | Both tools produce human-readable reports |
| Dispatch routing | Framework adapters: model selection + fallback | `dispatch` marble (intent → provider → fallback chain) | Both tools route to LLM providers |

These are Origami's first user-discovered marbles. The decomposition is not "delete Go" — it is "extract marbles from lived experience." The Go code is the prototype; the marbles are the product.

### Sibling validation: Achilles

Achilles (`github.com/dpopsuev/achilles`) is a proactive security vulnerability discovery tool. Its pipeline is structurally identical: scan → classify → assess → report. Both tools:

- Walk a graph of analysis nodes
- Use LLM extractors for reasoning
- Score results against ground truth
- Produce structured reports

Achilles is the **litmus test** for every marble extracted from Asterisk. If a marble works for CI post-mortem RCA but not for security probing RCA, the abstraction is wrong — go back and generalize. The goal is not to serve Asterisk; the goal is to serve any analysis tool on Origami.

This makes `rca-pure-dsl` the most strategically important contract in the project: it produces the first marble catalog, validated by two real consumers.

## FSC artifacts

| Artifact | Target | Compartment |
|----------|--------|-------------|
| Marble catalog (first user-discovered marbles) | Origami `docs/` | domain |
| Origami framework gaps inventory | `docs/` | domain |
| RCA decomposition analysis | `docs/` | domain |

## Execution strategy

**Phase 1: Marble discovery** (design, no code changes)

Catalog every Go file in `adapters/rca/`. For each file/function, classify into a marble candidate or framework gap:

| Category | Marble candidate | Origami primitive needed | Achilles reuse? |
|----------|-----------------|------------------------|-----------------|
| Already DSL | — (pipeline_rca.yaml) | None | Yes (achilles.yaml) |
| LLM extraction | `llm-extract` | Declarative extractor DSL | Yes (scan, classify, assess) |
| Context assembly | `context-builder` | State-to-prompt transformer | Yes (evidence assembly) |
| Store persistence | `persist` | SQLite adapter (from `adapter-migration`) | Yes (finding storage) |
| Scoring | `score` | `calibrate/` metric definition DSL | Yes (vulnerability scoring) |
| Report generation | `report` | Template/format marble | Yes (security report) |
| Dispatch routing | `dispatch` | Adapter-level YAML config | Yes (model routing) |
| Pure glue | Delete | None | — |

Cross-validate every candidate marble against Achilles's pipeline. If a marble is Asterisk-only, the abstraction is wrong.

**Phase 2: Marble implementation** — build the marbles in Origami, validated by both Asterisk and Achilles.

**Phase 3: Migration** — replace `rca/` Go files with marble imports + YAML configuration.

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

### Phase 1: Marble discovery

- [ ] Catalog every file in `adapters/rca/` with LOC, category, marble candidate, and Origami gap
- [ ] Catalog every file in `adapters/rca/adapt/` (framework adapter layer)
- [ ] Map Achilles's pipeline (scan → classify → assess → report) to the same marble candidates
- [ ] Produce the marble catalog: name, interface, inputs/outputs, Asterisk usage, Achilles usage
- [ ] Produce the Origami gaps inventory: what primitives are missing to support the marbles
- [ ] Design review: validate the marble catalog with a Plan Mode session

### Phase 2-4: Implementation (tasks TBD after Phase 1)

Tasks will be defined after the marble discovery. Each marble becomes its own implementation task in Origami, validated by both consumers.

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

2026-02-27 22:30 — Reframed as marble discovery. The 6K lines of Go in rca/ are prototypes for Origami's first user-discovered marbles: llm-extract, context-builder, persist, score, report, dispatch. Phase 1 produces the marble catalog, cross-validated against Achilles. This is the most strategically important contract — it defines the reusable building blocks for any analysis tool on Origami.
2026-02-27 22:15 — Contract drafted. Gate-tier: this is the final step to zero-Go Asterisk. Phase 1 is design-only — no code changes. Achilles validates the abstraction.
