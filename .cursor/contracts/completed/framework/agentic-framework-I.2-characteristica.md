# Contract — Agentic Framework I.2: Characteristica

**Status:** complete
**Goal:** Define a declarative YAML/Go DSL for expressing pipelines as graphs, so that the F0-F6 pipeline, the Defect Court D0-D4 pipeline, or any future pipeline can be declared as a configuration rather than hardcoded in Go.
**Serves:** Architecture evolution (Framework foundation)

## Contract rules

- The DSL must be parseable from YAML files and constructible programmatically in Go.
- The DSL must map 1:1 to `framework.Graph` -- every DSL pipeline produces a valid `Graph` instance.
- The F0-F6 pipeline must be expressible in the DSL as the first validation case.
- The DSL is descriptive, not prescriptive: it declares structure, not implementation. Node `Process` functions are registered separately.
- All 8 DSL design principles in `rules/domain/dsl-design-principles.mdc` apply. In particular: YAML as canonical format (P1), declarative intent (P2), reading-first layout (P3), progressive disclosure (P7), and round-trip fidelity (P8).
- Inspired by Leibniz's Characteristica Universalis -- a notation so clear that "controversies become calculations."

## Design principles

This contract implements the 8 principles codified in `rules/domain/dsl-design-principles.mdc`:

| Principle | How this contract applies it |
|-----------|------------------------------|
| P1: YAML as canonical | One `.yaml` file per pipeline in `pipelines/`. All other formats derived. |
| P2: Declarative intent | `condition` fields are human-readable labels; Go `NodeRegistry` holds executable logic. |
| P3: Reading-first layout | YAML sections ordered: pipeline > zones > nodes > edges > start/done. Edges sorted by source node. |
| P4: One concept per block | Each YAML list item is exactly one `NodeDef` or `EdgeDef`. Zones are one map entry each. |
| P5: Human labels, machine IDs | Every edge has `id` (machine, e.g. "H1") and `name` (human, e.g. "recall-hit"). |
| P6: Derived visualization | `Render(*PipelineDef) string` generates Mermaid from the parsed definition. |
| P7: Progressive disclosure | `zones`, `element`, `stickiness` are optional with sensible defaults. Minimal pipeline needs only `nodes`, `edges`, `start`, `done`. |
| P8: Round-trip fidelity | `LoadPipeline -> PipelineDef -> MarshalYAML` produces semantically equivalent YAML. |

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

// Render generates a Mermaid flowchart string from a pipeline definition (P6).
// The output is a valid Mermaid graph that can be embedded in Markdown.
func Render(def *PipelineDef) string
```

## Execution strategy

1. Define `PipelineDef`, `ZoneDef`, `NodeDef`, `EdgeDef` structs with YAML tags. Apply progressive disclosure (P7): optional fields use `omitempty`, defaults are zero-values.
2. Implement `LoadPipeline` -- parse YAML into `PipelineDef`.
3. Implement `PipelineDef.Validate()` -- check referential integrity (all edge endpoints exist, zones reference valid nodes, start node exists).
4. Implement `BuildGraph` -- construct a `Graph` from the definition + a `NodeRegistry`.
5. Implement `Render(*PipelineDef) string` -- generate Mermaid flowchart from a parsed pipeline definition (P6: derived visualization).
6. Express the F0-F6 pipeline as a YAML file and verify round-trip (P8): YAML -> PipelineDef -> Graph -> walk.
7. Express the Defect Court D0-D4 pipeline as a second YAML file for validation.
8. Verify round-trip fidelity (P8): `LoadPipeline -> Marshal -> LoadPipeline` produces equivalent results.

## Tasks

- [x] Create `internal/framework/dsl.go` -- `PipelineDef`, `ZoneDef`, `NodeDef`, `EdgeDef` structs (P7: optional fields with `omitempty`)
- [x] Implement `LoadPipeline(data []byte) (*PipelineDef, error)` -- YAML parser
- [x] Implement `PipelineDef.Validate() error` -- referential integrity checks
- [x] Implement `BuildGraph(registry NodeRegistry) (Graph, error)` -- construct Graph from DSL
- [x] Define `NodeRegistry` type and factory pattern
- [x] Implement `Render(*PipelineDef) string` -- generate Mermaid flowchart from parsed definition (P6)
- [x] Create `pipelines/rca-investigation.yaml` -- F0-F6 pipeline in DSL (P3: reading-first layout)
- [x] Create `pipelines/defect-court.yaml` -- D0-D4 pipeline in DSL (structure only, no Process implementations)
- [x] Write `internal/framework/dsl_test.go` -- parse, validate, round-trip fidelity (P8) tests
- [x] Write `internal/framework/build_test.go` -- build graph from DSL, walk with mock nodes
- [x] Write `internal/framework/render_test.go` -- Mermaid output correctness (P6)
- [x] Validate (green) -- `go build ./...`, all tests pass
- [x] Tune (blue) -- review DSL ergonomics, ensure YAML is human-readable, verify progressive disclosure (P7)
- [x] Validate (green) -- all tests still pass after tuning

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

- 2026-02-21 18:30 -- Contract complete. DSL layer (dsl.go, render.go), 2 pipeline YAMLs (rca-investigation, defect-court), orchestrate graph bridge (graph_bridge.go), runner refactored to use graph evaluation. 15 DSL tests, 5 build tests, 4 render tests passing. `DefaultHeuristics`/`EvaluateHeuristics` kept exported with deprecation notes for calibrate package compatibility. Moved to `completed/framework/`.
- 2026-02-21 14:30 -- DSL design principles research completed. 8 principles codified in `rules/domain/dsl-design-principles.mdc` and diffused into this contract. Key additions: `Render()` for derived Mermaid visualization (P6), progressive disclosure in struct design (P7), round-trip fidelity test (P8). YAML confirmed as primary format after assessing YAML, CUE, Mermaid, HCL, and custom DSL options against the dual-audience constraint (human + AI readers). Dagster's YAML DSL + code registry pattern validates our `PipelineDef` + `NodeRegistry` approach.
- 2026-02-20 -- Contract created. The DSL condition strings (e.g. "confidence >= recall_hit_threshold") are descriptive labels in Phase 1 -- actual evaluation logic remains in Go `Edge.Evaluate()` functions. A future contract could introduce a condition expression language.
- Depends on I.1-ontology for `Graph`, `Node`, `Edge`, `Zone` interfaces.
