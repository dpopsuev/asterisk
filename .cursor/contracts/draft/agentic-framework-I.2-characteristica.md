# Contract — Agentic Framework I.2: Characteristica

**Status:** draft
**Goal:** Define a declarative YAML/Go DSL for expressing pipelines as graphs, so that the F0-F6 pipeline, the Defect Court D0-D4 pipeline, or any future pipeline can be declared as a configuration rather than hardcoded in Go.
**Serves:** Architecture evolution (Framework foundation)

## Contract rules

- The DSL must be parseable from YAML files and constructible programmatically in Go.
- The DSL must map 1:1 to `framework.Graph` -- every DSL pipeline produces a valid `Graph` instance.
- The F0-F6 pipeline must be expressible in the DSL as the first validation case.
- The DSL is descriptive, not prescriptive: it declares structure, not implementation. Node `Process` functions are registered separately.
- Inspired by Leibniz's Characteristica Universalis -- a notation so clear that "controversies become calculations."

## Context

- `contracts/draft/agentic-framework-I.1-ontology.md` -- defines `Node`, `Edge`, `Walker`, `Graph`, `Zone` interfaces.
- `internal/orchestrate/heuristics.go` -- 17 heuristic rules that define the F0-F6 graph edges.
- `internal/orchestrate/types.go` -- `PipelineStep` enum that defines F0-F6 node identifiers.
- Plan reference: `agentic_framework_contracts_2daf3e14.plan.md` -- Tome I: Prima Materia.

## DSL specification

```yaml
pipeline: rca-investigation
description: "F0-F6 root cause analysis pipeline"

zones:
  backcourt:
    nodes: [recall, triage]
    element: fire
    stickiness: 0
  frontcourt:
    nodes: [resolve, investigate]
    element: water
    stickiness: 3
  paint:
    nodes: [correlate, review, report]
    element: air
    stickiness: 1

nodes:
  - name: recall
    element: fire
    family: recall
  - name: triage
    element: fire
    family: triage
  - name: resolve
    element: earth
    family: resolve
  - name: investigate
    element: water
    family: investigate
  - name: correlate
    element: air
    family: correlate
  - name: review
    element: diamond
    family: review
  - name: report
    element: air
    family: report

edges:
  - id: H1
    name: recall-hit
    from: recall
    to: review
    shortcut: true
    condition: "confidence >= recall_hit_threshold"

  - id: H3
    name: recall-uncertain
    from: recall
    to: triage
    condition: "recall_uncertain <= confidence < recall_hit"

  - id: H4
    name: recall-miss
    from: recall
    to: triage
    condition: "confidence < recall_uncertain OR no match"

  - id: H5
    name: triage-skip
    from: triage
    to: correlate
    shortcut: true
    condition: "skip_investigation == true"

  - id: H7
    name: triage-investigate
    from: triage
    to: resolve
    condition: "default (investigation needed)"

  - id: H9
    name: investigate-converged
    from: investigate
    to: correlate
    condition: "convergence_score >= convergence_sufficient"

  - id: H10
    name: investigate-loop
    from: investigate
    to: resolve
    loop: true
    condition: "convergence_score < convergence_sufficient AND loops < max_loops"

  - id: H10a
    name: investigate-exhausted
    from: investigate
    to: correlate
    condition: "loops >= max_loops"

  - id: H11
    name: correlate-dup
    from: correlate
    to: review
    shortcut: true
    condition: "is_duplicate AND confidence >= correlate_dup_threshold"

  - id: H12
    name: correlate-unique
    from: correlate
    to: review
    condition: "NOT is_duplicate OR confidence < correlate_dup_threshold"

  - id: H13
    name: review-approve
    from: review
    to: report
    condition: "decision == approve"

  - id: H14
    name: review-reassess
    from: review
    to: resolve
    loop: true
    condition: "decision == reassess"

  - id: H15
    name: review-overturn
    from: review
    to: report
    condition: "decision == overturn"

  - id: H17
    name: report-done
    from: report
    to: _done
    condition: "always (terminal)"

start: recall
done: _done
```

## Go types

```go
package framework

// PipelineDef is the top-level DSL structure for declaring a pipeline graph.
type PipelineDef struct {
    Pipeline    string              `yaml:"pipeline"`
    Description string              `yaml:"description,omitempty"`
    Zones       map[string]ZoneDef  `yaml:"zones"`
    Nodes       []NodeDef           `yaml:"nodes"`
    Edges       []EdgeDef           `yaml:"edges"`
    Start       string              `yaml:"start"`
    Done        string              `yaml:"done"`
}

// ZoneDef declares a meta-phase zone.
type ZoneDef struct {
    Nodes      []string `yaml:"nodes"`
    Element    string   `yaml:"element"`
    Stickiness int      `yaml:"stickiness"`
}

// NodeDef declares a node in the pipeline.
type NodeDef struct {
    Name    string `yaml:"name"`
    Element string `yaml:"element"`
    Family  string `yaml:"family"`
}

// EdgeDef declares a conditional edge between two nodes.
type EdgeDef struct {
    ID        string `yaml:"id"`
    Name      string `yaml:"name"`
    From      string `yaml:"from"`
    To        string `yaml:"to"`
    Shortcut  bool   `yaml:"shortcut,omitempty"`
    Loop      bool   `yaml:"loop,omitempty"`
    Condition string `yaml:"condition"`
}

// LoadPipeline parses a YAML pipeline definition and returns a PipelineDef.
func LoadPipeline(data []byte) (*PipelineDef, error)

// BuildGraph constructs a Graph from a PipelineDef and a NodeRegistry.
// The NodeRegistry maps node names to Node implementations.
func (def *PipelineDef) BuildGraph(registry NodeRegistry) (Graph, error)

// NodeRegistry maps node family names to Node factory functions.
type NodeRegistry map[string]func(def NodeDef) Node
```

## Execution strategy

1. Define `PipelineDef`, `ZoneDef`, `NodeDef`, `EdgeDef` structs with YAML tags.
2. Implement `LoadPipeline` -- parse YAML into `PipelineDef`.
3. Implement `PipelineDef.Validate()` -- check referential integrity (all edge endpoints exist, zones reference valid nodes, start node exists).
4. Implement `BuildGraph` -- construct a `Graph` from the definition + a `NodeRegistry`.
5. Express the F0-F6 pipeline as a YAML file and verify round-trip: YAML -> PipelineDef -> Graph -> walk.
6. Express the Defect Court D0-D4 pipeline as a second YAML file for validation.

## Tasks

- [ ] Create `internal/framework/dsl.go` -- `PipelineDef`, `ZoneDef`, `NodeDef`, `EdgeDef` structs
- [ ] Implement `LoadPipeline(data []byte) (*PipelineDef, error)` -- YAML parser
- [ ] Implement `PipelineDef.Validate() error` -- referential integrity checks
- [ ] Implement `BuildGraph(registry NodeRegistry) (Graph, error)` -- construct Graph from DSL
- [ ] Define `NodeRegistry` type and factory pattern
- [ ] Create `pipelines/rca-investigation.yaml` -- F0-F6 pipeline in DSL
- [ ] Create `pipelines/defect-court.yaml` -- D0-D4 pipeline in DSL (structure only, no Process implementations)
- [ ] Write `internal/framework/dsl_test.go` -- parse, validate, round-trip tests
- [ ] Write `internal/framework/build_test.go` -- build graph from DSL, walk with mock nodes
- [ ] Validate (green) -- `go build ./...`, all tests pass
- [ ] Tune (blue) -- review DSL ergonomics, ensure YAML is human-readable
- [ ] Validate (green) -- all tests still pass after tuning

## Acceptance criteria

- **Given** the F0-F6 pipeline YAML definition,
- **When** it is loaded and built into a Graph,
- **Then** the Graph contains 7 nodes (recall through report), 14+ edges (H1 through H17), and 3 zones (backcourt, frontcourt, paint).

- **Given** a YAML definition with a broken edge (references nonexistent node),
- **When** `Validate()` is called,
- **Then** it returns an error naming the invalid reference.

- **Given** a valid `PipelineDef` and a `NodeRegistry` with mock nodes,
- **When** `BuildGraph` is called and the graph is walked,
- **Then** edge evaluation determines the walk path through the graph.

## Notes

- 2026-02-20 -- Contract created. The DSL condition strings (e.g. "confidence >= recall_hit_threshold") are descriptive labels in Phase 1 -- actual evaluation logic remains in Go `Edge.Evaluate()` functions. A future contract could introduce a condition expression language.
- Depends on I.1-ontology for `Graph`, `Node`, `Edge`, `Zone` interfaces.
