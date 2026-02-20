package framework

import (
	"context"
	"fmt"
)

// NodeProcessor is the function signature for processing a node.
type NodeProcessor func(ctx context.Context, nc NodeContext) (Artifact, error)

// Mask is a detachable capability modifier that wraps a Node's processing.
// Masks grant powers at specific nodes without changing the agent's core identity.
type Mask interface {
	Name() string
	Description() string
	ValidNodes() []string
	Wrap(next NodeProcessor) NodeProcessor
}

// MaskRegistry holds available masks indexed by name.
type MaskRegistry map[string]Mask

// MaskedNode wraps a Node with one or more Masks applied as middleware.
type MaskedNode struct {
	Inner Node
	Masks []Mask
}

func (mn *MaskedNode) Name() string              { return mn.Inner.Name() }
func (mn *MaskedNode) ElementAffinity() Element   { return mn.Inner.ElementAffinity() }

// Process executes the node with all masks applied as a middleware chain.
// First equipped = outermost wrapper: A.pre -> B.pre -> Node -> B.post -> A.post.
func (mn *MaskedNode) Process(ctx context.Context, nc NodeContext) (Artifact, error) {
	var processor NodeProcessor = mn.Inner.Process
	for i := len(mn.Masks) - 1; i >= 0; i-- {
		processor = mn.Masks[i].Wrap(processor)
	}
	return processor(ctx, nc)
}

// EquipMask adds a mask to a node. Returns error if the mask is not valid
// for this node's name. If the node is already a MaskedNode, the mask is
// appended to the existing chain.
func EquipMask(node Node, mask Mask) (*MaskedNode, error) {
	if !isValidNode(node.Name(), mask.ValidNodes()) {
		return nil, fmt.Errorf("mask %q cannot be equipped at node %q (valid: %v)",
			mask.Name(), node.Name(), mask.ValidNodes())
	}

	if mn, ok := node.(*MaskedNode); ok {
		mn.Masks = append(mn.Masks, mask)
		return mn, nil
	}

	return &MaskedNode{Inner: node, Masks: []Mask{mask}}, nil
}

// EquipMasks adds multiple masks to a node. First mask = outermost wrapper.
func EquipMasks(node Node, masks ...Mask) (*MaskedNode, error) {
	var result *MaskedNode
	for _, mask := range masks {
		var err error
		if result == nil {
			result, err = EquipMask(node, mask)
		} else {
			result, err = EquipMask(result, mask)
		}
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}

func isValidNode(nodeName string, validNodes []string) bool {
	for _, vn := range validNodes {
		if vn == nodeName {
			return true
		}
	}
	return false
}

// --- Skeleton mask implementations for the Light pipeline ---

type recallMask struct{}

func (m *recallMask) Name() string        { return "mask-of-recall" }
func (m *recallMask) Description() string  { return "Injects prior RCA database context" }
func (m *recallMask) ValidNodes() []string { return []string{"recall"} }
func (m *recallMask) Wrap(next NodeProcessor) NodeProcessor {
	return func(ctx context.Context, nc NodeContext) (Artifact, error) {
		if nc.Meta == nil {
			nc.Meta = make(map[string]any)
		}
		nc.Meta["prior_rca_available"] = true
		return next(ctx, nc)
	}
}

// NewRecallMask returns the Mask of Recall (valid at "recall" node).
func NewRecallMask() Mask { return &recallMask{} }

type forgeMask struct{}

func (m *forgeMask) Name() string        { return "mask-of-the-forge" }
func (m *forgeMask) Description() string  { return "Injects workspace repo context" }
func (m *forgeMask) ValidNodes() []string { return []string{"investigate"} }
func (m *forgeMask) Wrap(next NodeProcessor) NodeProcessor {
	return func(ctx context.Context, nc NodeContext) (Artifact, error) {
		if nc.Meta == nil {
			nc.Meta = make(map[string]any)
		}
		nc.Meta["workspace_repos_available"] = true
		return next(ctx, nc)
	}
}

// NewForgeMask returns the Mask of the Forge (valid at "investigate" node).
func NewForgeMask() Mask { return &forgeMask{} }

type correlationMask struct{}

func (m *correlationMask) Name() string        { return "mask-of-correlation" }
func (m *correlationMask) Description() string  { return "Enables cross-case pattern matching" }
func (m *correlationMask) ValidNodes() []string { return []string{"correlate"} }
func (m *correlationMask) Wrap(next NodeProcessor) NodeProcessor {
	return func(ctx context.Context, nc NodeContext) (Artifact, error) {
		if nc.Meta == nil {
			nc.Meta = make(map[string]any)
		}
		nc.Meta["cross_case_matching"] = true
		return next(ctx, nc)
	}
}

// NewCorrelationMask returns the Mask of Correlation (valid at "correlate" node).
func NewCorrelationMask() Mask { return &correlationMask{} }

type judgmentMask struct{}

func (m *judgmentMask) Name() string        { return "mask-of-judgment" }
func (m *judgmentMask) Description() string  { return "Grants authority to approve/reject/reassess" }
func (m *judgmentMask) ValidNodes() []string { return []string{"review"} }
func (m *judgmentMask) Wrap(next NodeProcessor) NodeProcessor {
	return func(ctx context.Context, nc NodeContext) (Artifact, error) {
		if nc.Meta == nil {
			nc.Meta = make(map[string]any)
		}
		nc.Meta["review_authority"] = true
		return next(ctx, nc)
	}
}

// NewJudgmentMask returns the Mask of Judgment (valid at "review" node).
func NewJudgmentMask() Mask { return &judgmentMask{} }

// DefaultLightMasks returns the 4 Light pipeline masks in a registry.
func DefaultLightMasks() MaskRegistry {
	masks := []Mask{NewRecallMask(), NewForgeMask(), NewCorrelationMask(), NewJudgmentMask()}
	reg := make(MaskRegistry, len(masks))
	for _, m := range masks {
		reg[m.Name()] = m
	}
	return reg
}
