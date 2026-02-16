package workspace

// Repo is one context source (local path or remote URL) with optional meta.
// See docs/context-workspace.mdc.
type Repo struct {
	Path    string `json:"path" yaml:"path"`       // local path (absolute or relative to workspace file)
	URL     string `json:"url" yaml:"url"`        // remote git URL (alternative to path)
	Name    string `json:"name,omitempty" yaml:"name,omitempty"`
	Purpose string `json:"purpose,omitempty" yaml:"purpose,omitempty"` // agentic meta: role in RCA
	Branch  string `json:"branch,omitempty" yaml:"branch,omitempty"`   // override envelope ref
}

// Workspace is the context workspace: list of repos for analysis.
type Workspace struct {
	Repos []Repo `json:"repos" yaml:"repos"`
}
