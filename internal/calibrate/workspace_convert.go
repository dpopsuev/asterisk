package calibrate

import "asterisk/internal/workspace"

// ScenarioToWorkspace converts a calibrate.WorkspaceConfig to a workspace.Workspace
// so store-aware adapters can use it for BuildParams.
func ScenarioToWorkspace(wc WorkspaceConfig) *workspace.Workspace {
	ws := &workspace.Workspace{}
	for _, r := range wc.Repos {
		ws.Repos = append(ws.Repos, workspace.Repo{
			Name:    r.Name,
			Path:    r.Path,
			Purpose: r.Purpose,
			Branch:  r.Branch,
		})
	}
	return ws
}
