# Contract — Agentic Framework I.1: Ontology

**Status:** complete  
**Goal:** Define `Node`, `Edge`, `Walker`, `Graph` as generic Go interfaces in `internal/framework/`, making the implicit relationships between existing `orchestrate` types (`HeuristicRule`, `PipelineStep`, `CaseState`) and graph concepts explicit. Absorb scope of `agentmux-decoupling.md`.  
**Serves:** Architecture evolution (Framework foundation)

## Contract rules

- The `internal/framework/` package tree must have **zero imports** from Asterisk domain packages (`internal/orchestrate`, `internal/store`, `internal/workspace`, `internal/rp`, `internal/calibrate`).
- Existing `orchestrate` types remain untouched. This contract defines the generic layer; adaptation is a separate migration step.
- All Framework types must be interface-first: concrete implementations come in domain adapter packages, not in `internal/framework/`.
- Backward compatibility: the existing F0-F6 pipeline must work identically. The Framework layer is additive.
- This contract absorbs the scope of `agentmux-decoupling.md`. The generic `Agent` interface from that contract becomes the `Walker` interface here. The `Task` and `Result` types map to `NodeContext` and `Artifact`.

## Context

- `internal/orchestrate/types.go` — `PipelineStep` (IS a Node identifier), `CaseState` (IS walker state), `HeuristicRule` (IS an Edge), `HeuristicAction` (IS a Transition).
- `internal/orchestrate/heuristics.go` — 17 heuristic rules that ARE edges in the F0-F6 graph.
- `internal/orchestrate/runner.go` — the pipeline engine that IS a graph walker.
- `contracts/draft/agentmux-decoupling.md` — defines generic `Agent`, `Task`, `Result`, `AgentPool`, `Scheduler`, `ResultCollector`. These concepts fold into the Framework ontology.
- `internal/framework/identity.go` — `ModelIdentity` (already implemented). Records which foundation LLM ("ghost") is behind an adapter ("shell"). Includes `Wrapper` field for hosting environments (Cursor, Azure). `AgentIdentity` (placeholder) and `ModelIdentity` are siblings: identity = who the persona is + which model powers it.
- `internal/framework/known_models.go` — `KnownModels` registry, `KnownWrappers` set, `IsWrapperName()` validation. Foundation models are registered here; wrappers are rejected.
- `internal/calibrate/adapter.go` — `Identifiable` interface: `Identify() (ModelIdentity, error)`. Adapters implement this to self-report their foundation model at session start.
- Plan reference: `agentic_framework_contracts_2daf3e14.plan.md` — Tome I: Prima Materia.
- Inspiration: Neural networks (nodes/edges/weights), Kabbalah Tree of Life (hierarchical emanation), Hermeticism ("as above, so below" — graph topology mirrors problem structure at every scale).

## Type mapping

| Framework Type | Existing Orchestrate Type | Relationship |
|----------------|--------------------------|--------------|
| `Node` | `PipelineStep` | A `PipelineStep` is the identifier of a Node. The Node wraps the step with processing logic and elemental affinity. |
| `Edge` | `HeuristicRule` | A `HeuristicRule` IS an Edge: it connects two Nodes with a conditional transition. `Evaluate()` IS the edge weight function. |
| `Transition` | `HeuristicAction` | A `HeuristicAction` IS a Transition: the result of evaluating an Edge, containing the next Node and context additions. |
| `Walker` | `CaseState` + `Agent` | A Walker combines walker state (`CaseState`) with agent identity and processing capability. |
| `WalkerState` | `CaseState` | `CaseState` IS walker state: position (current step), history, loop counts, status. |
| `Graph` | `Runner` + `[]HeuristicRule` | The Runner's heuristic evaluation loop IS a graph walk. The `[]HeuristicRule` defines the graph structure. |
| `Zone` | (new concept) | Meta-phase grouping of Nodes: Backcourt, Frontcourt, Paint. |
| `Artifact` | `RecallResult`, `TriageResult`, etc. | Typed outputs from Node processing. |
| `NodeContext` | `Task` (from agentmux) | Input to a Node's `Process` method: the accumulated context for this walker at this node. |

## Key types

```go
package framework

import "context"

// Node is a processing stage in a pipeline graph.
type Node interface {
    Name() string
    ElementAffinity() Element
    Process(ctx context.Context, nc NodeContext) (Artifact, error)
}

// Edge is a conditional connection between two Nodes.
type Edge interface {
    ID() string
    From() string    // source node name
    To() string      // target node name
    IsShortcut() bool
    IsLoop() bool
    Evaluate(artifact Artifact, state *WalkerState) *Transition
}

// Walker is an agent traversing a graph.
type Walker interface {
    Identity() AgentIdentity
    State() *WalkerState
    Handle(ctx context.Context, node Node, nc NodeContext) (Artifact, error)
}

// Graph is a directed graph of Nodes connected by Edges, partitioned into Zones.
type Graph interface {
    Name() string
    Nodes() []Node
    Edges() []Edge
    Zones() []Zone
    NodeByName(name string) (Node, bool)
    EdgesFrom(nodeName string) []Edge
    Walk(ctx context.Context, walker Walker, startNode string) error
}

// Zone is a meta-phase grouping of Nodes with shared characteristics.
type Zone struct {
    Name            string
    NodeNames       []string
    ElementAffinity Element
    Stickiness      int // 0-3 stickiness value for agents in this zone
}

// Transition is the result of evaluating an Edge.
type Transition struct {
    NextNode         string
    ContextAdditions map[string]any
    Explanation      string
}

// WalkerState tracks a walker's progress through a graph.
type WalkerState struct {
    ID          string            // walker/case identifier
    CurrentNode string            // current position in the graph
    LoopCounts  map[string]int    // per-edge loop counters
    Status      string            // running, paused, done, error
    History     []StepRecord      // log of completed nodes
    Context     map[string]any    // accumulated context
}

// StepRecord logs a completed node visit.
type StepRecord struct {
    Node        string // node name
    Outcome     string // e.g. "recall-hit", "triage-investigate"
    EdgeID      string // which edge rule matched
    Timestamp   string // ISO 8601
}

// Artifact is the output of a Node's processing.
// The framework treats it as opaque; typed artifacts are domain-specific.
type Artifact interface {
    Type() string                // e.g. "recall", "triage", "investigate"
    Confidence() float64         // quality signal (0.0-1.0)
    Raw() any                    // underlying typed artifact
}

// NodeContext is the input to a Node's Process method.
type NodeContext struct {
    WalkerState  *WalkerState
    PriorArtifact Artifact        // output from the previous node (nil at start)
    Meta         map[string]string // extensible metadata
}

// AgentIdentity is a placeholder here; fully defined in III.1-personae.
// See also ModelIdentity (already implemented in identity.go) which
// records the foundation LLM behind the agent.
type AgentIdentity struct {
    Name string
}

// Element is a placeholder here; fully defined in II.1-elements.
type Element string
```

## Package layout

```
internal/
  framework/                     # Generic agent pipeline framework
    node.go                      # Node, Artifact, NodeContext interfaces
    edge.go                      # Edge, Transition interfaces
    walker.go                    # Walker, WalkerState, StepRecord
    graph.go                     # Graph, Zone interfaces + default impl
    identity.go                  # AgentIdentity placeholder (III.1) + ModelIdentity (IMPLEMENTED)
    known_models.go              # KnownModels registry, KnownWrappers, IsWrapperName (IMPLEMENTED)
    element.go                   # Element placeholder (extended by II.1)
    errors.go                    # Framework-specific error types
```

## Execution strategy

1. Define all interfaces and types in `internal/framework/`.
2. Write comprehensive tests for the default `Graph` implementation (building a simple 3-node graph, walking it, verifying transitions).
3. Do NOT migrate the existing `orchestrate` package yet — that's the final migration step after all three Tomes are complete.
4. The placeholder types (`AgentIdentity`, `Element`) will be replaced with full definitions by contracts II.1 and III.1.

## Tasks

- [x] Create `internal/framework/node.go` — `Node`, `Artifact`, `NodeContext` interfaces
- [x] Create `internal/framework/edge.go` — `Edge`, `Transition` types
- [x] Create `internal/framework/walker.go` — `Walker`, `WalkerState`, `StepRecord`
- [x] Create `internal/framework/graph.go` — `Graph`, `Zone` interfaces + `DefaultGraph` implementation
- [x] Create `internal/framework/identity.go` — `AgentIdentity` placeholder
- [x] Create `internal/framework/element.go` — `Element` placeholder type
- [x] Create `internal/framework/errors.go` — `ErrNoEdge`, `ErrNodeNotFound`, `ErrMaxLoops`
- [x] Write `internal/framework/graph_test.go` — build 3-node graph, walk it, verify edge evaluation and transitions
- [x] Write `internal/framework/walker_test.go` — walker state transitions, history accumulation, loop counting
- [x] Validate (green) — `go build ./...`, all tests pass, existing pipeline unchanged
- [x] Tune (blue) — review interfaces for minimality, ensure zero domain imports
- [x] Validate (green) — all tests still pass after tuning

## Acceptance criteria

- **Given** `go list -deps ./internal/framework/...` is run,
- **When** the output is inspected,
- **Then** no Asterisk domain packages appear (`internal/orchestrate`, `internal/store`, `internal/workspace`, `internal/rp`, `internal/calibrate`).

- **Given** the F0-F6 pipeline is run after this contract is complete,
- **When** the full test suite runs,
- **Then** behavior is identical to pre-contract (the Framework layer is additive).

- **Given** the type mapping table above,
- **When** a developer reads `internal/framework/`,
- **Then** the mapping from Framework types to `orchestrate` types is self-evident from naming and documentation.

## Security assessment

| OWASP | Finding | Mitigation |
|-------|---------|------------|
| A04 | `NodeContext.Meta` and `WalkerState.Context` accept arbitrary key-value pairs. | Document as opaque transport; consumers validate before acting. Same mitigation as `agentmux-decoupling.md`. |

## Notes

- 2026-02-21 15:00 — Contract complete. All interfaces and types implemented in `internal/framework/`. 10 graph tests, 4 walker tests passing. Zero domain imports confirmed via `go list -deps`. Moved to `completed/framework/`.
- 2026-02-20 21:30 — FSC diffusion: `ModelIdentity`, `KnownModels`, `KnownWrappers`, and `Identifiable` are already implemented in `internal/framework/` and `internal/calibrate/adapter.go`. This contract's `AgentIdentity` placeholder will be expanded by III.1-personae to include a `ModelIdentity` field so every persona knows which foundation model powers it.
- 2026-02-20 — Contract created. Absorbs `agentmux-decoupling.md` scope. The generic `Agent` interface becomes `Walker`; `Task` becomes `NodeContext`; `Result` becomes `Artifact`. Package target changed from `internal/agentmux/` to `internal/framework/` to reflect broader scope.
- The existing 17 heuristic rules in `orchestrate/heuristics.go` are a concrete implementation of the `Edge` interface. Migration will wrap each `HeuristicRule` as an `Edge` adapter.
