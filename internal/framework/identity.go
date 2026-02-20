package framework

import "fmt"

// AgentIdentity identifies an agent traversing the graph.
// Placeholder; fully defined by contract III.1-personae which adds
// Color, Element, Position, and Alignment axes.
type AgentIdentity struct {
	Name string
}

// ModelIdentity records which foundation LLM model ("ghost") is behind
// an adapter ("shell"). The Wrapper field records the hosting environment
// (e.g. Cursor, Azure) that may sit between the caller and the foundation model.
type ModelIdentity struct {
	ModelName string `json:"model_name"`
	Provider  string `json:"provider"`
	Version   string `json:"version,omitempty"`
	Wrapper   string `json:"wrapper,omitempty"`
	Raw       string `json:"raw,omitempty"`
}

// String returns "model@version/provider (via wrapper)".
// Omits @version when empty. Omits "(via wrapper)" when empty.
func (m ModelIdentity) String() string {
	name := m.ModelName
	if name == "" {
		name = "unknown"
	}
	prov := m.Provider
	if prov == "" {
		prov = "unknown"
	}

	var s string
	if m.Version != "" {
		s = fmt.Sprintf("%s@%s/%s", name, m.Version, prov)
	} else {
		s = fmt.Sprintf("%s/%s", name, prov)
	}

	if m.Wrapper != "" {
		s += fmt.Sprintf(" (via %s)", m.Wrapper)
	}
	return s
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
