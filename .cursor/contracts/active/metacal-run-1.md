# Contract — metacal-run-1

**Status:** active  
**Goal:** Discover all models available in Cursor's auto-select via prompt negation, run one behavioral probe on each, and persist results as the first empirical meta-calibration dataset.  
**Serves:** Framework showcase (weekend side-quest — gate extension)

## Contract rules

- Discovery iterations are sequential (each depends on the previous exclusion list).
- The refactoring probe input is deterministic and version-controlled.
- Results are append-only JSON — never overwrite a prior run.
- Zero imports from Asterisk domain packages (`calibrate`, `orchestrate`, `origami`). This is framework-level.
- New code goes in `pkg/framework/metacal/`, consistent with the `pkg/framework/` location.

## Context

### The prompt negation technique

Cursor's auto-select picks a model for each agent task. By iteratively excluding previously discovered models in the system prompt, we force Cursor to select a different model each time. This is a zero-cost model enumeration technique — no API keys, no provider accounts, just prompt engineering against the IDE's own model router.

### Existing infrastructure

- **`KnownModels` registry** — `pkg/framework/known_models.go` tracks foundation models by name/provider/version.
- **`ModelIdentity`** — `pkg/framework/identity.go` is the struct for model fingerprints.
- **Identity wet test** — `pkg/framework/identity_wet_test.go` validates model self-identification against the registry.
- **MCP server** — `asterisk serve` supports Cursor subagent spawning via the Task tool.
- **Signal bus** — `emit_signal`/`get_signals` for subagent coordination.

### Relationship to meta-calibration vision contract

This contract is the **first empirical run** of the meta-calibration system described in `draft/meta-calibration.md`. It implements only the discovery loop and one probe (refactoring). The vision contract's full battery (5+ probes), normalization, and persona wiring remain future work. Results from this run feed into the vision contract's Phase 2-4 design.

### Current architecture

```mermaid
flowchart LR
    subgraph framework ["pkg/framework/"]
        KM["KnownModels\n(static registry)"]
        MI["ModelIdentity"]
        WET["identity_wet_test.go\n(single probe, manual)"]
    end
    WET -->|"validates against"| KM
    WET -->|"produces"| MI
```

### Desired architecture

```mermaid
flowchart LR
    subgraph framework ["pkg/framework/"]
        KM["KnownModels\n(static registry)"]
        MI["ModelIdentity"]
    end
    subgraph metacal ["pkg/framework/metacal/"]
        DR["DiscoveryRunner\n(negation loop)"]
        RP["RefactorProbe\n(behavioral scorer)"]
        ST["RunStore\n(append-only JSON)"]
        TY["DiscoveryResult\nProbeResult\nRunReport"]
    end
    DR -->|"spawns subagent with\nexclusion prompt"| CursorAuto["Cursor auto-select"]
    CursorAuto -->|"returns identity +\nprobe output"| DR
    DR -->|"scores"| RP
    DR -->|"persists"| ST
    DR -->|"updates"| KM
    ST -->|"stores"| TY
```

## FSC artifacts

| Artifact | Target | Compartment |
|----------|--------|-------------|
| First meta-calibration run results (JSON) | `pkg/framework/metacal/runs/` | domain |
| Model discovery protocol notes | `notes/` | domain |

## Execution strategy

Build types first (Phase 1), then the probe (Phase 2), then the discovery runner (Phase 3), then the store and CLI integration (Phase 4). Each phase is independently testable. The package has zero domain imports at all times.

The main agent orchestrates the discovery loop:
1. Call `get_next_step` equivalent (build exclusion prompt from prior results).
2. Spawn a Cursor subagent via the Task tool with the negation prompt.
3. Subagent identifies itself and runs the refactoring probe.
4. Main agent scores the probe output, persists the result, updates the exclusion list.
5. Repeat until termination condition.

## Tasks

### Phase 1 — Types

- [ ] Create `pkg/framework/metacal/types.go` with `DiscoveryResult`, `ProbeResult`, `ProbeScore`, `RunReport`, `DiscoveryConfig`
- [ ] `DiscoveryResult`: `Iteration int`, `ModelIdentity`, `ExclusionPrompt string`, `ProbeResult`, `Timestamp`
- [ ] `ProbeResult`: `ProbeID string`, `RawOutput string`, `Score ProbeScore`, `Elapsed time.Duration`
- [ ] `ProbeScore`: `Renames int`, `FunctionSplits int`, `CommentsAdded int`, `StructuralChanges int`, `TotalScore float64`
- [ ] `RunReport`: `RunID string`, `StartTime`, `EndTime`, `Config DiscoveryConfig`, `Results []DiscoveryResult`, `UniqueModels []ModelIdentity`
- [ ] `DiscoveryConfig`: `MaxIterations int`, `ProbeID string`, `TerminateOnRepeat bool`
- [ ] Unit tests for type construction and JSON round-trip

### Phase 2 — Refactoring probe

- [ ] Create `pkg/framework/metacal/probe.go` with `RefactorProbe` struct
- [ ] Create deterministic messy Go function input (hardcoded in `probe.go` or `testdata/`)
- [ ] Implement `ScoreRefactorOutput(original, refactored string) ProbeScore` — counts renames, splits, comments, structural changes
- [ ] Implement `BuildProbePrompt(input string) string` — the prompt given to the subagent alongside the messy code
- [ ] Unit tests for scorer with known inputs/outputs

### Phase 3 — Discovery runner

- [ ] Create `pkg/framework/metacal/discovery.go` with `DiscoveryRunner`
- [ ] Implement `BuildExclusionPrompt(seen []ModelIdentity) string` — constructs the negation prompt
- [ ] Implement `ParseIdentityResponse(raw string) (ModelIdentity, error)` — extracts model identity from subagent response
- [ ] Implement `ParseProbeResponse(raw string) (string, error)` — extracts refactored code from subagent response
- [ ] Implement `Run(ctx context.Context) (*RunReport, error)` — the main discovery loop
- [ ] Termination conditions: repeat model, max iterations, subagent error, 2 consecutive same model
- [ ] Unit tests with stub responses

### Phase 4 — Store and integration

- [ ] Create `pkg/framework/metacal/store.go` with `RunStore` interface and `FileRunStore` (append-only JSON)
- [ ] `SaveRun(report RunReport) error`, `LoadRun(runID string) (RunReport, error)`, `ListRuns() ([]string, error)`
- [ ] Auto-register newly discovered models into `KnownModels` (or print the registration line)
- [ ] Unit tests for store round-trip

### Phase 5 — Validate and tune

- [ ] Validate (green) — all tests pass, zero domain imports, store round-trip works
- [ ] Tune (blue) — refactor for quality, review naming, ensure scorer is deterministic
- [ ] Validate (green) — all tests still pass after tuning

## Acceptance criteria

**Given** a Cursor IDE session with auto-select enabled,  
**When** the discovery runner executes with `MaxIterations=15`,  
**Then**:
- At least 2 distinct foundation models are discovered (based on current Cursor offering)
- Each discovered model has a `DiscoveryResult` with a scored `ProbeResult`
- Results are persisted to JSON in `pkg/framework/metacal/runs/`
- Previously unknown models are flagged with the exact `KnownModels` registration line
- The run terminates cleanly (no infinite loops, no orphaned subagents)

**Given** a `RunReport` with N discovered models,  
**When** `ProbeScore` values are compared,  
**Then** scores are deterministic for identical probe outputs (same input → same score).

**Given** the discovery runner is executed twice,  
**When** results are loaded from the store,  
**Then** both runs are independently loadable and neither overwrites the other.

## Security assessment

No trust boundaries affected. The discovery runner operates entirely within the Cursor IDE session. Probe inputs are synthetic (no user data, no secrets). Model identity data is non-sensitive (publicly known model names and providers).

## Notes

2026-02-21 15:00 — Contract created. Extends the weekend side-quest with a first empirical meta-calibration run. Concept: prompt negation to enumerate Cursor's model pool. Discovery + one refactoring probe per model. Automated via Task tool subagents.
