package curate

import (
	"asterisk/internal/framework"
	"context"
	"fmt"
	"os"
)

// LoadCurationPipeline reads and parses the curation pipeline YAML from a file path.
func LoadCurationPipeline(yamlPath string) (*framework.PipelineDef, error) {
	data, err := os.ReadFile(yamlPath)
	if err != nil {
		return nil, fmt.Errorf("curate: read pipeline %q: %w", yamlPath, err)
	}
	return ParseCurationPipeline(data)
}

// ParseCurationPipeline parses curation pipeline YAML bytes.
func ParseCurationPipeline(data []byte) (*framework.PipelineDef, error) {
	return framework.LoadPipeline(data)
}

// curationNode implements framework.Node for curation pipeline stages.
type curationNode struct {
	name    string
	element framework.Element
	family  string
}

func (n *curationNode) Name() string                        { return n.name }
func (n *curationNode) ElementAffinity() framework.Element  { return n.element }
func (n *curationNode) Process(_ context.Context, _ framework.NodeContext) (framework.Artifact, error) {
	return nil, nil
}

func newCurationNode(def framework.NodeDef) framework.Node {
	return &curationNode{
		name:    def.Name,
		element: framework.Element(def.Element),
		family:  def.Family,
	}
}

// DefaultNodeRegistry returns a NodeRegistry with all curation node families registered.
func DefaultNodeRegistry() framework.NodeRegistry {
	return framework.NodeRegistry{
		"fetch":    newCurationNode,
		"extract":  newCurationNode,
		"validate": newCurationNode,
		"enrich":   newCurationNode,
		"promote":  newCurationNode,
	}
}

// curationEdge wraps an EdgeDef with custom evaluation logic.
type curationEdge struct {
	def      framework.EdgeDef
	evalFunc func(framework.Artifact, *framework.WalkerState) *framework.Transition
}

func (e *curationEdge) ID() string       { return e.def.ID }
func (e *curationEdge) From() string     { return e.def.From }
func (e *curationEdge) To() string       { return e.def.To }
func (e *curationEdge) IsShortcut() bool { return e.def.Shortcut }
func (e *curationEdge) IsLoop() bool     { return e.def.Loop }
func (e *curationEdge) Evaluate(a framework.Artifact, s *framework.WalkerState) *framework.Transition {
	if e.evalFunc != nil {
		return e.evalFunc(a, s)
	}
	return &framework.Transition{NextNode: e.def.To, Explanation: e.def.Condition}
}

// CurationArtifact is a generic artifact carrying a Record and evaluation metadata.
type CurationArtifact struct {
	ArtifactType string       `json:"type"`
	Rec          *Record      `json:"record,omitempty"`
	RawEvid      *RawEvidence `json:"raw_evidence,omitempty"`
	Conf         float64      `json:"confidence"`
	Complete     bool         `json:"complete"`
	MoreSources  bool         `json:"more_sources"`
}

func (a *CurationArtifact) Type() string       { return a.ArtifactType }
func (a *CurationArtifact) Confidence() float64 { return a.Conf }
func (a *CurationArtifact) Raw() any            { return a }

// MaxFetchLoops controls how many times CE3 will loop back to fetch
// before giving up and promoting incomplete records.
const MaxFetchLoops = 3

// DefaultEdgeFactory returns an EdgeFactory with evaluation logic for the
// curation pipeline edges CE1-CE6.
func DefaultEdgeFactory() framework.EdgeFactory {
	return framework.EdgeFactory{
		"CE1": func(def framework.EdgeDef) framework.Edge {
			return &curationEdge{
				def: def,
				evalFunc: func(_ framework.Artifact, _ *framework.WalkerState) *framework.Transition {
					return &framework.Transition{NextNode: def.To, Explanation: "proceed to extraction"}
				},
			}
		},
		"CE2": func(def framework.EdgeDef) framework.Edge {
			return &curationEdge{
				def: def,
				evalFunc: func(_ framework.Artifact, _ *framework.WalkerState) *framework.Transition {
					return &framework.Transition{NextNode: def.To, Explanation: "proceed to validation"}
				},
			}
		},
		"CE3": func(def framework.EdgeDef) framework.Edge {
			return &curationEdge{
				def: def,
				evalFunc: func(a framework.Artifact, s *framework.WalkerState) *framework.Transition {
					ca, ok := a.(*CurationArtifact)
					if !ok {
						return nil
					}
					if !ca.Complete && ca.MoreSources {
						loopCount := s.IncrementLoop("CE3")
						if loopCount > MaxFetchLoops {
							return nil
						}
						return &framework.Transition{
							NextNode:    def.To,
							Explanation: "missing required fields, more sources available",
						}
					}
					return nil
				},
			}
		},
		"CE4": func(def framework.EdgeDef) framework.Edge {
			return &curationEdge{
				def: def,
				evalFunc: func(a framework.Artifact, _ *framework.WalkerState) *framework.Transition {
					ca, ok := a.(*CurationArtifact)
					if !ok {
						return nil
					}
					if ca.Complete || (!ca.MoreSources && ca.Rec != nil) {
						return &framework.Transition{NextNode: def.To, Explanation: "completeness above threshold"}
					}
					return nil
				},
			}
		},
		"CE5": func(def framework.EdgeDef) framework.Edge {
			return &curationEdge{
				def: def,
				evalFunc: func(_ framework.Artifact, _ *framework.WalkerState) *framework.Transition {
					return &framework.Transition{NextNode: def.To, Explanation: "proceed to promotion"}
				},
			}
		},
		"CE6": func(def framework.EdgeDef) framework.Edge {
			return &curationEdge{
				def: def,
				evalFunc: func(_ framework.Artifact, _ *framework.WalkerState) *framework.Transition {
					return &framework.Transition{NextNode: def.To, Explanation: "always (terminal)"}
				},
			}
		},
	}
}

// BuildCurationGraph parses pipeline YAML bytes and builds a framework.Graph
// with the default curation registries.
func BuildCurationGraph(yamlData []byte) (framework.Graph, error) {
	def, err := ParseCurationPipeline(yamlData)
	if err != nil {
		return nil, err
	}
	return def.BuildGraph(DefaultNodeRegistry(), DefaultEdgeFactory())
}
