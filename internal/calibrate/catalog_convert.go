package calibrate

import "github.com/dpopsuev/origami/knowledge"

// ScenarioToCatalog converts a calibrate.WorkspaceConfig to a knowledge.KnowledgeSourceCatalog
// so store-aware adapters can use it for BuildParams.
func ScenarioToCatalog(wc WorkspaceConfig) *knowledge.KnowledgeSourceCatalog {
	cat := &knowledge.KnowledgeSourceCatalog{}
	for _, r := range wc.Repos {
		cat.Sources = append(cat.Sources, knowledge.Source{
			Name:    r.Name,
			Kind:    knowledge.SourceKindRepo,
			URI:     r.Path,
			Purpose: r.Purpose,
			Branch:  r.Branch,
		})
	}
	return cat
}
