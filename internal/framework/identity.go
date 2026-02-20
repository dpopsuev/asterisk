package framework

import "fmt"

// AgentIdentity identifies an agent traversing the graph.
// Placeholder; fully defined by contract III.1-personae which adds
// Color, Element, Position, and Alignment axes.
type AgentIdentity struct {
	Name string
}

// ModelIdentity records which LLM model ("ghost") is behind an adapter
// ("shell"). Populated at session start via the Identifiable interface.
type ModelIdentity struct {
	ModelName string `json:"model_name"`
	Provider  string `json:"provider"`
	Version   string `json:"version,omitempty"`
	Raw       string `json:"raw,omitempty"`
}

// String returns a deterministic, short identifier: "model@version/provider".
// Omits the @version segment when Version is empty.
func (m ModelIdentity) String() string {
	name := m.ModelName
	if name == "" {
		name = "unknown"
	}
	prov := m.Provider
	if prov == "" {
		prov = "unknown"
	}
	if m.Version != "" {
		return fmt.Sprintf("%s@%s/%s", name, m.Version, prov)
	}
	return fmt.Sprintf("%s/%s", name, prov)
}

// Tag returns a bracket-wrapped model name for log lines, e.g. "[claude-4-sonnet]".
// Truncated to keep total length under 24 chars.
func (m ModelIdentity) Tag() string {
	name := m.ModelName
	if name == "" {
		name = "unknown"
	}
	if len(name) > 20 {
		name = name[:20]
	}
	return fmt.Sprintf("[%s]", name)
}
