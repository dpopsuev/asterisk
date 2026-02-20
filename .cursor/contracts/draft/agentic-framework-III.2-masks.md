# Contract -- Agentic Framework III.2: Masks

**Status:** draft
**Goal:** Define masks as detachable capability modifiers that agents can equip at specific pipeline nodes, inspired by Bionicle Kanohi masks and Jungian persona theory. Implement as middleware wrapping Node.Process.
**Serves:** Architecture evolution (Framework identity)

## Contract rules

- Masks are defined in `internal/framework/` alongside identity types.
- A mask modifies an agent's behavior WITHOUT changing their core identity (Color, Element, Position, Alignment remain unchanged).
- Masks are node-scoped: each mask declares which nodes it can be equipped at. Equipping a mask at an unauthorized node is a no-op.
- Multiple masks can be stacked (middleware chain). Order matters: first equipped = outermost wrapper.
- Masks do not import domain packages. They operate on the generic Node, Artifact, NodeContext interfaces.
- Inspired by: Bionicle Kanohi (specific powers granted by masks), Jungian Persona (the mask we present to the world), Warhammer wargear (equipment that modifies unit stats).

## Context

- `contracts/draft/agentic-framework-I.1-ontology.md` -- defines Node, Artifact, NodeContext interfaces.
- `contracts/draft/agentic-framework-III.1-personae.md` -- defines AgentIdentity with core identity axes.
- Plan reference: agentic_framework_contracts_2daf3e14.plan.md -- Tome III: Personae.

## Mask definitions

| Mask | Grants | Equipped at | Effect |
|------|--------|-------------|--------|
| Mask of Recall | Access to prior RCA database | F0 (Recall) | Injects prior RCA context into NodeContext. Enables recall-hit shortcutting. |
| Mask of the Forge | Access to workspace repos | F3 (Investigate) | Injects repo file trees, commit history, and code search into NodeContext. |
| Mask of Correlation | Cross-case pattern matching | F4 (Correlate) | Injects other cases' artifacts for duplicate detection and cross-version matching. |
| Mask of Judgment | Authority to approve/reject/reassess | F5 (Review) | Grants the agent authority to issue ReviewDecision. Without this mask, an agent can only recommend. |
| Mask of Indictment | Prosecution brief construction | D0 (Indict) | Formats investigation artifacts into formal charges with evidence weights. Shadow pipeline only. |
| Mask of Discovery | Raw data access (bypassing conclusions) | D1 (Discover) | Strips prior conclusions, provides only raw failure data. Shadow pipeline only. |

## Go types

```go
package framework

// Mask is a detachable capability modifier that wraps a Node's processing.
type Mask interface {
    Name() string
    Description() string
    ValidNodes() []string
    Wrap(next NodeProcessor) NodeProcessor
}

// NodeProcessor is the function signature for processing a node.
type NodeProcessor func(ctx context.Context, nc NodeContext) (Artifact, error)

// MaskRegistry holds available masks indexed by name.
type MaskRegistry map[string]Mask

// MaskedNode wraps a Node with one or more Masks applied.
type MaskedNode struct {
    Inner Node
    Masks []Mask
}

// Process executes the node with all masks applied as middleware.
func (mn *MaskedNode) Process(ctx context.Context, nc NodeContext) (Artifact, error)

// EquipMask adds a mask to a node. Returns error if the mask is
// not valid for this node (invalid node name).
func EquipMask(node Node, mask Mask) (*MaskedNode, error)

// EquipMasks adds multiple masks to a node (outermost first).
func EquipMasks(node Node, masks ...Mask) (*MaskedNode, error)
```

## Middleware pattern

Masks follow the standard middleware/decorator pattern:

```go
type recallMask struct{}

func (m *recallMask) Name() string            { return "mask-of-recall" }
func (m *recallMask) Description() string      { return "Injects prior RCA database context" }
func (m *recallMask) ValidNodes() []string     { return []string{"recall"} }

func (m *recallMask) Wrap(next NodeProcessor) NodeProcessor {
    return func(ctx context.Context, nc NodeContext) (Artifact, error) {
        nc.Meta["prior_rca_available"] = "true"
        artifact, err := next(ctx, nc)
        if err != nil {
            return nil, err
        }
        return artifact, nil
    }
}
```

The MaskedNode.Process builds the middleware chain:

```go
func (mn *MaskedNode) Process(ctx context.Context, nc NodeContext) (Artifact, error) {
    processor := mn.Inner.Process
    for i := len(mn.Masks) - 1; i >= 0; i-- {
        processor = mn.Masks[i].Wrap(processor)
    }
    return processor(ctx, nc)
}
```

## Execution strategy

1. Define the Mask interface and MaskedNode type.
2. Implement EquipMask and EquipMasks with node validation.
3. Implement the 4 Light pipeline masks (Recall, Forge, Correlation, Judgment) as skeleton implementations.
4. Write tests: mask wrapping, middleware chain ordering, invalid node rejection.
5. Defer Shadow pipeline masks (Indictment, Discovery) to III.3-shadow contract.

## Tasks

- [ ] Define Mask interface: Name(), Description(), ValidNodes(), Wrap()
- [ ] Define NodeProcessor function type
- [ ] Define MaskRegistry type
- [ ] Implement MaskedNode struct with Process method (middleware chain)
- [ ] Implement EquipMask(node, mask) with valid-node check
- [ ] Implement EquipMasks(node, masks...) for multi-mask stacking
- [ ] Implement skeleton RecallMask -- valid at "recall" node
- [ ] Implement skeleton ForgeMask -- valid at "investigate" node
- [ ] Implement skeleton CorrelationMask -- valid at "correlate" node
- [ ] Implement skeleton JudgmentMask -- valid at "review" node
- [ ] Write `internal/framework/mask_test.go` -- wrapping, chaining, ordering, invalid node rejection
- [ ] Validate (green) -- go build ./..., all tests pass
- [ ] Tune (blue) -- review middleware chain for correctness and performance
- [ ] Validate (green) -- all tests still pass after tuning

## Acceptance criteria

- **Given** a Node named "recall" and the Mask of Recall,
- **When** EquipMask(node, recallMask) is called,
- **Then** the returned MaskedNode processes through the mask's Wrap function.

- **Given** a Node named "investigate" and the Mask of Recall,
- **When** EquipMask(node, recallMask) is called,
- **Then** an error is returned (invalid node for this mask).

- **Given** a Node with two masks equipped (A outermost, B innermost),
- **When** Process is called,
- **Then** execution flows: A.pre -> B.pre -> Node.Process -> B.post -> A.post.

## Notes

- 2026-02-20 -- Contract created. Masks are the Kanohi of the Framework -- they grant specific powers at specific nodes without changing the agent's core identity. This maps to Jung's concept of persona: the mask we wear for specific social contexts.
- Shadow pipeline masks (Indictment, Discovery) are listed for completeness but implemented in III.3-shadow.
- Depends on I.1-ontology for Node, Artifact, NodeContext interfaces. Depends on III.1-personae for AgentIdentity (masks don't change identity, but identity determines which masks are available).
