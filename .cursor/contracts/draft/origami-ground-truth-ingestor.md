# Contract — origami-ground-truth-ingestor

**Status:** draft  
**Goal:** Agentic dataset curation: consume unstructured evidence (Jira, PRs, logs, files) into structured ground truth via AI subagents, stored as reviewable JSON files. Target: 18 verified → 50+ verified cases. Export as reviewable JSON and open GitHub PR for team review.  
**Serves:** Dataset growth (SHOULD for Phase 5a mitigation Item 4)

## Contract rules

Global rules only, plus:

- **Reviewability-first** (`rules/reviewability-first.mdc`): the canonical dataset is a git-tracked JSON file. Every mutation appears in PR diffs. No binary stores.
- **Adapter-driver pattern** (`rules/abstraction-boundaries.mdc`): `DatasetStore` interface with `FileStore` as the PoC driver. PTP is the first scenario driver.
- **CLI-first citizen**: all operations are CLI subcommands. MCP is a thin facade that delegates to the CLI layer.

## Context

- `notes/test-cards-assessment.md` — Data Card documenting the current ground truth dataset.
- `internal/calibrate/types.go` — `Scenario`, `GroundTruthRCA`, `GroundTruthCase` structs with full `json` tags.
- `internal/calibrate/scenarios/ptp_real_ingest.go` — 18 verified + 12 candidate cases (current Go structs).
- `internal/calibrate/dispatch/mux.go` — `MuxDispatcher` pattern for concurrent artifact routing.
- `rules/reviewability-first.mdc` — reviewability > performance; self-reinforcement loop.
- Previous plan discussion: storage format (JSON files, not SQLite), interface (CLI-first + thin MCP bridge), scope (generic from day one).

## Execution strategy

### Storage: JSON files, not SQLite

The canonical ground truth lives in `datasets/<scenario-name>.json` — a direct `json.Marshal` of `calibrate.Scenario`. The existing struct tags define the schema. No ORM, no DDL, no migration tooling.

```
datasets/
  ptp-real-ingest.json     # Full Scenario struct as JSON
```

The `DatasetStore` interface is minimal:

```go
type DatasetStore interface {
    List(ctx context.Context) ([]string, error)
    Load(ctx context.Context, name string) (*calibrate.Scenario, error)
    Save(ctx context.Context, s *calibrate.Scenario) error
}
```

`AttachEvidence`, `Promote`, and `ListCandidates` are operations on the in-memory `Scenario` struct followed by `Save()`. Every save produces a reviewable diff.

### Ingest Dispatcher

Each evidence ingestion is a `Dispatch` call that routes to a typed subagent, mirroring `dispatch.MuxDispatcher`:

```go
type IngestTask struct {
    TaskID   int64
    Type     string   // "jira", "pr", "log", "file"
    Source   string   // URL, file path, or inline content
    CaseID   string
    RCAID    string
    Content  []byte   // pre-fetched content
}

type Evidence struct {
    Type       string
    Source     string
    Fields     map[string]any
    Raw        []byte
    Confidence float64
}
```

### Evidence Source Adapters

```go
type EvidenceSource interface {
    Type() string
    Fetch(ctx context.Context, ref string) ([]byte, error)
    CanHandle(ref string) bool
}
```

PoC implementations: `JiraSource`, `GitHubPRSource`, `FileSource`, `URLSource`.

#### Robustness requirements

`EvidenceSource` adapters must handle real-world edge cases:

- **Rate limits**: Respect `Retry-After` headers from Jira and GitHub APIs. Exponential backoff with jitter.
- **Authentication**: Support both anonymous and authenticated access. Degrade gracefully when auth fails (mark evidence as `unavailable` with reason).
- **Large files**: Stream large artifacts (CI logs > 1 MB). Never load entire file into memory for extraction.
- **Partial data**: When a source returns incomplete data (e.g., Jira ticket with redacted fields, PR with no diff), extract what is available and record the gaps in `CompletenessResult.Missing`.
- **Timeouts**: Per-source configurable timeout (default 30s). Fail fast and record the timeout in evidence metadata.
- **Network errors**: Transient errors retry (max 3 attempts). Permanent errors (404, 403) fail immediately with descriptive error.

### Completeness Tracker

Scores each candidate's readiness for promotion:

```go
type CompletenessResult struct {
    CaseID     string
    RCAID      string
    Score      float64
    Present    []string
    Missing    []string
    Promotable bool
}
```

Required fields for verification: JiraID, FixPRs, SmokingGun, RequiredKeywords + KeywordThreshold, DefectType, Category, Component, ExpectedPath, ExpectedTriage.

### CLI (`asterisk origami`)

| Subcommand | Purpose |
|------------|---------|
| `status` | Dataset overview + per-candidate completeness |
| `inspect <id>` | Full case detail with evidence and gaps |
| `ingest` | Ingest evidence from source into case |
| `promote <id>` | Promote candidate to verified (with validation) |
| `import` | Seed store from existing Go scenario |
| `export` | Export dataset for calibration |

### MCP Bridge (thin facade)

One MCP tool per CLI command: `origami_status`, `origami_inspect`, `origami_ingest`, `origami_promote`. Same pattern as `internal/mcp/server.go`.

### Package layout

```
internal/origami/
    store.go          -- DatasetStore interface + FileStore implementation
    completeness.go   -- Completeness tracker (in-memory Scenario)
    ingest.go         -- IngestDispatcher + IngestTask + Evidence types
    source/
        jira.go       -- JiraSource adapter
        github.go     -- GitHubPRSource adapter
        file.go       -- FileSource adapter
    extract/
        extractor.go  -- AI extractor interface
        jira.go       -- Jira ticket -> structured fields
        pr.go         -- PR diff -> SmokingGun + keywords
        log.go        -- CI log -> error patterns
datasets/
    ptp-real-ingest.json  -- Canonical ground truth (git-tracked)
cmd/asterisk/
    cmd_origami.go    -- CLI subcommands
internal/mcp/
    server.go         -- Origami MCP tools (thin bridge)
```

## Tasks

### Phase 1 — Storage foundation
- [x] Define `DatasetStore` interface + `FileStore` implementation (`internal/origami/store.go`)
- [ ] Write `asterisk gt import --from-go ptp-real-ingest` to serialize Go structs → `datasets/ptp-real-ingest.json`
- [ ] Write `asterisk gt export --scenario ptp-real-ingest` to load JSON → `calibrate.Scenario`
- [x] Unit tests: round-trip (import → load → save → load → compare)

### Phase 2 — Completeness + CLI status
- [x] Implement completeness tracker (`internal/origami/completeness.go`)
- [x] Write `asterisk gt status` CLI command (`cmd/asterisk/cmd_gt.go`)
- [ ] Unit tests: completeness scoring, promotion readiness

### Phase 3 — Evidence sources + ingest
- [ ] Implement `EvidenceSource` interface + PoC adapters (Jira, GitHub PR, File)
- [ ] Write `asterisk origami ingest --case <id> --source <ref>` CLI command
- [ ] Unit tests: source detection, fetch, structured extraction

### Phase 4 — AI subagents
- [ ] Implement `IngestDispatcher` (MuxDispatcher-influenced) for routing to extractors
- [ ] AI extractor subagent prompts (Jira ticket, PR diff, CI log)
- [ ] Integration tests: end-to-end ingest with stub AI responses

### Phase 5 — Promote + calibration integration
- [ ] Write `asterisk origami promote <id>` with completeness validation
- [ ] Wire `DatasetStore.Load()` as an alternative scenario source in calibration runner
- [ ] Integration tests: promote candidate → re-run calibration → case appears in scoring

### Phase 6 — MCP bridge
- [ ] Add Origami MCP tools to `internal/mcp/server.go` (thin facade to CLI layer)
- [ ] Integration tests: agent conversation flow via MCP

### Phase 7 — Dataset expansion + GitHub PR
- [ ] Target dataset size: 50+ verified cases (current: 18 verified + 12 candidates)
- [ ] Identify additional RP launches for ingestion (new PTP CI runs, other operator CIs)
- [ ] Run `asterisk origami import` to produce initial `datasets/ptp-real-ingest.json` from Go structs
- [ ] Open GitHub PR with `datasets/ptp-real-ingest.json` for team review
- [ ] Iterate on PR: ingest evidence for candidates, promote ready candidates, update JSON, push
- [ ] Acceptance: PR merged with 30+ verified cases in JSON dataset

### Validation
- [ ] Validate (green) — all tests pass, acceptance criteria met.
- [ ] Tune (blue) — refactor for quality. No behavior changes.
- [ ] Validate (green) — all tests still pass after tuning.

## Acceptance criteria

**Given** the existing `ptp-real-ingest` Go scenario with 18 verified + 12 candidates,  
**When** `asterisk origami import --from-go ptp-real-ingest` is run,  
**Then** `datasets/ptp-real-ingest.json` contains the full Scenario as reviewable JSON.

**Given** a candidate case C01 with missing FixPR and SmokingGun,  
**When** `asterisk origami ingest --case C01 --source "redhat-cne/cloud-event-proxy#633"` is run,  
**Then** the JSON file is updated with extracted evidence and the diff is git-visible.

**Given** a candidate with all required fields present,  
**When** `asterisk origami promote C01` is run,  
**Then** the case moves from `candidates` to `cases` with `verified: true` in the JSON file.

**Given** the updated `datasets/ptp-real-ingest.json`,  
**When** calibration runs with `--scenario ptp-real-ingest`,  
**Then** the runner loads from JSON and scores only verified cases (same behavior as Go structs).

**Given** the PR with `datasets/ptp-real-ingest.json` is merged,  
**When** calibration runs with `--scenario ptp-real-ingest`,  
**Then** it uses the JSON dataset with 30+ verified cases and scores on a statistically meaningful sample.

**Given** the `JiraSource` adapter encounters a rate-limited Jira API response,  
**When** `asterisk origami ingest` is run,  
**Then** the adapter retries with exponential backoff and does not crash or lose partial progress.

**Structural invariant:** The canonical dataset is always a git-tracked JSON file. Every Origami mutation produces a reviewable diff.

## Notes

2026-02-18 — Revised storage from SQLite to JSON files based on reviewability-first principle. "The real benefit is reviewability — you can see the full dataset in a PR." This enables the self-reinforcement loop: agent mutates dataset → PR diff visible → human/agent reviews → feedback improves process. New rule `reviewability-first.mdc` codifies this.

2026-02-18 — User confirmed: CLI-first citizen (any agent can use it), thin MCP layer (facade/bridge pattern), generic from day one with PTP as first driver, adapter-driver pattern for storage.

2026-02-19 04:00 — Phase 5a mitigation Item 4: dataset size target raised to 50+ verified (from 18), robustness requirements added to EvidenceSource adapters (rate limits, auth, large files, partial data, timeouts), Phase 7 added for dataset expansion and GitHub PR workflow. PR is the reviewable delivery mechanism for dataset updates.

2026-02-19 06:00 — **PoC scope trimming**: For SHOULD-tier execution, only Phases 1-2 (storage foundation + completeness tracker + CLI status) and Phase 7 (dataset expansion + GitHub PR) are in scope. Phases 3-6 (evidence sources, AI extractors, promote + calibration integration, MCP bridge) are deferred to post-PoC. This cuts the contract from a multi-week effort to 2-3 days. Execution order: 4th of 6 SHOULD contracts.
