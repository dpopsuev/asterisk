package framework

import (
	"fmt"
	"strings"
)

// Color represents an agent's personality on the warm-cool spectrum.
type Color struct {
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
	Hex         string `json:"hex"`
	Family      string `json:"family"`
}

// Alignment represents an agent's motivational orientation.
type Alignment string

const (
	AlignmentLight  Alignment = "light"
	AlignmentShadow Alignment = "shadow"
)

// Position represents an agent's court position (structural role).
type Position string

const (
	PositionPG Position = "PG"
	PositionSG Position = "SG"
	PositionPF Position = "PF"
	PositionC  Position = "C"
)

// MetaPhase represents a zone in the pipeline graph.
type MetaPhase string

const (
	MetaPhaseBk MetaPhase = "Backcourt"
	MetaPhaseFc MetaPhase = "Frontcourt"
	MetaPhasePt MetaPhase = "Paint"
)

// CostProfile describes the resource cost of using an agent.
type CostProfile struct {
	TokensPerStep int     `json:"tokens_per_step"`
	LatencyMs     int     `json:"latency_ms"`
	CostPerToken  float64 `json:"cost_per_token"`
}

// AgentIdentity is the complete identity of an agent in the Framework.
// Axes 1-4 (persona) are set at configuration time.
// Axis 5 (Model) is discovered at runtime via the Identifiable interface.
type AgentIdentity struct {
	PersonaName     string    `json:"persona_name"`
	Color           Color     `json:"color"`
	Element         Element   `json:"element"`
	Position        Position  `json:"position"`
	Alignment       Alignment `json:"alignment"`
	HomeZone        MetaPhase `json:"home_zone"`
	StickinessLevel int       `json:"stickiness_level"`

	Model ModelIdentity `json:"model"`

	StepAffinity    map[string]float64 `json:"step_affinity,omitempty"`
	PersonalityTags []string           `json:"personality_tags,omitempty"`
	PromptPreamble  string             `json:"prompt_preamble,omitempty"`
	CostProfile     CostProfile        `json:"cost_profile,omitempty"`
}

// HomeZoneFor returns the MetaPhase for a given Position.
func HomeZoneFor(p Position) MetaPhase {
	switch p {
	case PositionPG:
		return MetaPhaseBk
	case PositionSG:
		return MetaPhasePt
	case PositionPF:
		return MetaPhaseFc
	case PositionC:
		return MetaPhaseFc
	default:
		return ""
	}
}

// Tag returns a log-friendly tag like "[crimson/herald]".
func (id AgentIdentity) Tag() string {
	color := strings.ToLower(id.Color.Name)
	if color == "" {
		color = "none"
	}
	name := strings.ToLower(id.PersonaName)
	if name == "" {
		name = "anon"
	}
	return fmt.Sprintf("[%s/%s]", color, name)
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
