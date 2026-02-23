# Contract — distillation-endgame

**Status:** active  
**Goal:** Migrate remaining framework-disguised packages from Asterisk to Origami and build three framework capabilities so domain tools register behavior, not execution machinery.  
**Serves:** Framework showcase (current goal)

## Contract rules

Global rules only, plus:

- **Cross-repo contract.** Tasks span `asterisk` and `origami` repositories. Origami version tag is bumped after each migration batch.
- **No pure-DSL goal.** Heuristic closures and store side effects stay as Go hooks. The target is "zero framework Go in the domain tool," not "zero Go."
- **Successor to `asterisk-origami-distillation`.** That contract proved the boundary with Achilles. This contract completes the migration and evolves the framework.

## Context

- `completed/distillation/asterisk-origami-distillation.md` — Predecessor contract. Proved boundary with Achilles. All 5 phases complete.
- `docs/distillation-manifest.md` — Package classification, dependency graph, boundary validation.
- Origami `contracts/active/origami-agentic-network-framework.md` — Origami identity and Extractor primitive.

### Current architecture

Framework-disguised packages still live in `asterisk/internal/`. The orchestrator uses passthrough bridge nodes and an imperative `RunStep` loop. The dispatch job queue is duplicated between calibration and metacal.

```mermaid
flowchart TD
    subgraph asteriskNow [Asterisk - current]
        metacalmcp["internal/metacalmcp"]
        curate["internal/curate"]
        logging["internal/logging"]
        fmt["internal/format"]
        ws["internal/workspace"]
        dispatch["internal/calibrate/dispatch"]
        bridge["internal/orchestrate/graph_bridge.go"]
        runner["internal/orchestrate/runner.go loop"]
        calibrate["internal/calibrate"]
        orchestrate["internal/orchestrate"]
        rp["internal/rp"]
        mcp["internal/mcp"]
        investigate["internal/investigate"]
        display["internal/display"]
        origamiBridge["internal/origami"]
    end
    subgraph origamiNow [Origami - current]
        dsl["Pipeline DSL"]
        graph["Graph Walk"]
        ext["Extractors"]
        mcpFw["mcp/ signal + server"]
        ouroboros["ouroboros/"]
    end
    metacalmcp --> origamiNow
    curate --> origamiNow
    bridge --> origamiNow
    calibrate --> dispatch
    calibrate --> logging
    orchestrate --> logging
    mcp --> calibrate
```

### Desired architecture

```mermaid
flowchart TD
    subgraph origamiTarget [Origami - target]
        dsl["Pipeline DSL"]
        graph["Graph Walk"]
        ext["Extractors"]
        mcpFw["mcp/"]
        ouroborosPkg["ouroboros/"]
        ouroborosmcpPkg["ouroborosmcp/"]
        curatePkg["curate/"]
        loggingPkg["logging/"]
        fmtPkg["format/"]
        wsPkg["workspace/"]
        dispatchPkg["dispatch/"]
        execEngine["Exec Engine"]
        schemas["Artifact Schemas"]
    end
    subgraph asteriskTarget [Asterisk - thin domain]
        hooks["Go hooks: heuristics + side effects"]
        yaml["3 YAML pipelines"]
        domain["calibrate + rp + investigate + display"]
        domainMcp["mcp/ domain server"]
        origamiBridge["internal/origami bridge"]
    end
    asteriskTarget --> origamiTarget
```

## FSC artifacts

| Artifact | Target | Compartment |
|----------|--------|-------------|
| Updated distillation manifest | `docs/` | domain |
| Architecture diagram (post-endgame) | `docs/` | domain |
| Dispatch pattern reference | Origami `docs/` | framework |

## Execution strategy

Three phases, each independently valuable. Phase 1 is pure migration (no API changes). Phase 2 designs new framework APIs. Phase 3 rewires Asterisk to use them.

## Coverage matrix

| Layer | Applies | Rationale |
|-------|---------|-----------|
| **Unit** | yes | Migrated packages retain their existing tests; new framework capabilities need unit tests |
| **Integration** | yes | `go build ./...` and `go test ./...` in all three repos after each migration batch |
| **Contract** | yes | Zero `asterisk/internal/` imports for migrated packages; `go list -deps` validation |
| **E2E** | yes | Asterisk calibration (stub) must pass after each phase; Achilles scan must still work |
| **Concurrency** | yes | MuxDispatcher, BatchFileDispatcher have race-sensitive code; `-race` required |
| **Security** | no | No new trust boundaries. Dispatch already assessed in predecessor contract. |

## Tasks

### Phase 1 — Code Migration (6 packages)

- [x] Move `internal/metacalmcp/` to `origami/ouroborosmcp/`; update Asterisk imports
- [x] Move `cmd/metacal/` to `origami/cmd/origami` (ouroboros subcommand); update module paths
- [ ] Move `internal/curate/` to `origami/curate/`; update Asterisk and `internal/origami/` imports
- [ ] Move `internal/logging/` to `origami/logging/`; update all 14 Asterisk importers
- [ ] Move `internal/format/` to `origami/format/`; update all 9 Asterisk importers
- [ ] Move `internal/workspace/` to `origami/workspace/`; update all 11 Asterisk importers
- [ ] `go build ./...` and `go test ./...` green in both repos
- [ ] Tag Origami `v0.2.0`; update Asterisk `go.mod`

### Phase 2 — Framework Evolution (3 capabilities)

- [ ] **Capability A — Artifact Schemas:** Design schema declaration in YAML nodes; implement validation in Origami
- [ ] **Capability B — Node Execution Engine:** Implement `Runner.Walk()` that drives `Process()` on real nodes; domain registers factories
- [ ] **Capability C — Generic Dispatch:** Promote entire `dispatch/` package to `origami/dispatch/` — all four dispatchers (`Stdin`, `File`, `Mux`, `Batch`). Refactor `StdinDispatcher` to accept a `StdinTemplate` (banner, instructions, done prompt) so domains configure text, not code.
- [ ] Unit tests for all three capabilities
- [ ] Tag Origami `v0.3.0`

### Phase 3 — Asterisk Thin Domain

- [ ] Replace `graph_bridge.go` passthrough nodes with real node factories registered into Origami's exec engine
- [ ] Replace imperative `RunStep` loop with framework `Runner.Walk()` + registered heuristic hooks
- [ ] Replace `loadCurrentArtifact` switch with schema-driven deserialization
- [ ] Migrate calibration dispatch to `origami/dispatch/`; pass Asterisk-specific `StdinTemplate` at construction
- [ ] `go build ./...` and `go test ./...` green; stub calibration passes
- [ ] Validate (green) — all acceptance criteria met
- [ ] Tune (blue) — remove dead code (`graph_bridge.go`, bridge types, duplicate dispatch)
- [ ] Validate (green) — all tests still pass after tuning

## Acceptance criteria

**Given** the Origami framework at `github.com/dpopsuev/origami` and Asterisk at `asterisk`,  
**When** this contract is complete,  
**Then**:
- `asterisk/internal/` contains only domain packages: `calibrate/`, `orchestrate/` (heuristics + side effects only), `mcp/`, `rp/`, `investigate/`, `display/`, `origami/`
- `asterisk/internal/metacalmcp/`, `internal/curate/`, `internal/logging/`, `internal/format/`, `internal/workspace/` no longer exist
- `asterisk/cmd/metacal/` no longer exists
- `origami/ouroborosmcp/`, `origami/curate/`, `origami/logging/`, `origami/format/`, `origami/workspace/`, `origami/dispatch/` exist with tests
- Origami provides `Runner.Walk()` that drives node `Process()` and evaluates edges
- Origami provides artifact schema validation from YAML node declarations
- `graph_bridge.go` and `bridgeNode` are deleted from Asterisk
- The imperative `RunStep` for-loop in `runner.go` is replaced by framework execution
- `go build ./...` passes in Asterisk, Origami, and Achilles
- `go test ./...` passes in all three repos
- Stub calibration (`just calibrate-stub`) produces the same metrics as before

## Security assessment

No new trust boundaries affected. Dispatch interfaces move repos but retain the same security properties assessed in `asterisk-origami-distillation`.

| OWASP | Finding | Mitigation |
|-------|---------|------------|
| A03 Injection | Dispatch payloads are JSON — no shell expansion | Same as predecessor |
| A08 Data Integrity | Artifact schema validation adds a new integrity check | Positive: validates shapes before processing |

## Notes

2026-02-23 01:00 — Updated Capability C: `StdinDispatcher` moves to Origami too. The mechanism (print banner, show case/step, print instructions, block on Enter, read artifact) is generic. The domain-specific part (instruction text) becomes a `StdinTemplate` config struct. All four dispatchers now move to `origami/dispatch/`.

2026-02-23 00:15 — Contract created. Successor to `asterisk-origami-distillation` (now complete). Analysis from conversation: 6 packages identified for migration, 3 framework capabilities designed, anti-goal (pure DSL) documented.
