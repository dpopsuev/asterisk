package rca

import (
	"os"

	"github.com/dpopsuev/origami/adapters/rp"
	"asterisk/adapters/store"
	"github.com/dpopsuev/origami/knowledge"
)

// BuildParams constructs the full TemplateParams from available data.
func BuildParams(
	st store.Store,
	caseData *store.Case,
	env *rp.Envelope,
	catalog *knowledge.KnowledgeSourceCatalog,
	step CircuitStep,
	caseDir string,
) *TemplateParams {
	params := &TemplateParams{
		CaseID:   caseData.ID,
		StepName: string(step),
		Taxonomy: DefaultTaxonomy(),
		Timestamps: &TimestampParams{
			ClockPlaneNote: "Note: Timestamps may originate from different clock planes (executor, test node, SUT). Cross-plane time comparisons may be unreliable.",
		},
	}

	if env != nil {
		params.LaunchID = env.RunID
		params.Envelope = &EnvelopeParams{
			Name:  env.Name,
			RunID: env.RunID,
		}
		for _, f := range env.FailureList {
			params.Siblings = append(params.Siblings, SiblingParams{
				ID: f.ID, Name: f.Name, Status: f.Status,
			})
		}
	}

	params.Failure = &FailureParams{
		TestName:     caseData.Name,
		ErrorMessage: caseData.ErrorMessage,
		LogSnippet:   caseData.LogSnippet,
		LogTruncated: caseData.LogTruncated,
		Status:       caseData.Status,
	}

	wsp := buildWorkspaceParams(env, catalog)
	params.Workspace = wsp

	if catalog != nil {
		params.AlwaysReadSources = loadAlwaysReadSources(catalog)
	}

	params.Prior = loadPriorArtifacts(caseDir)

	if st != nil && caseData.SymptomID != 0 {
		params.History = loadHistory(st, caseData.SymptomID)
	}

	if st != nil && step == StepF0Recall && caseData.SymptomID == 0 && params.History == nil {
		params.History = findRecallCandidates(st, caseData.Name)
	}

	if st != nil && step == StepF0Recall {
		params.RecallDigest = buildRecallDigest(st)
	}

	return params
}

func buildWorkspaceParams(env *rp.Envelope, catalog *knowledge.KnowledgeSourceCatalog) *WorkspaceParams {
	wsp := &WorkspaceParams{}

	if catalog != nil && len(catalog.Sources) > 0 {
		wsp.ReposStatus = Resolved
		for _, s := range catalog.Sources {
			wsp.Repos = append(wsp.Repos, RepoParams{
				Name:    s.Name,
				Path:    s.URI,
				Purpose: s.Purpose,
				Branch:  s.Branch,
			})
		}
	} else {
		wsp.ReposStatus = Unavailable
	}

	if env != nil && len(env.LaunchAttributes) > 0 {
		wsp.AttrsStatus = Resolved
		for _, a := range env.LaunchAttributes {
			wsp.LaunchAttributes = append(wsp.LaunchAttributes, AttributeParams{
				Key:    a.Key,
				Value:  a.Value,
				System: a.System,
			})
		}
	} else {
		wsp.AttrsStatus = Unavailable
	}

	if env != nil {
		seen := map[string]bool{}
		for _, f := range env.FailureList {
			for _, ext := range f.ExternalIssues {
				if ext.TicketID != "" && !seen[ext.TicketID] {
					seen[ext.TicketID] = true
					wsp.JiraLinks = append(wsp.JiraLinks, JiraLinkParams{
						TicketID: ext.TicketID,
						URL:      ext.URL,
					})
				}
			}
		}
	}
	if len(wsp.JiraLinks) > 0 {
		wsp.JiraStatus = Resolved
	} else {
		wsp.JiraStatus = Unavailable
	}

	return wsp
}

func loadPriorArtifacts(caseDir string) *PriorParams {
	if caseDir == "" {
		return nil
	}
	prior := &PriorParams{}
	prior.RecallResult, _ = ReadArtifact[RecallResult](caseDir, ArtifactFilename(StepF0Recall))
	prior.TriageResult, _ = ReadArtifact[TriageResult](caseDir, ArtifactFilename(StepF1Triage))
	prior.ResolveResult, _ = ReadArtifact[ResolveResult](caseDir, ArtifactFilename(StepF2Resolve))
	prior.InvestigateResult, _ = ReadArtifact[InvestigateArtifact](caseDir, ArtifactFilename(StepF3Invest))
	prior.CorrelateResult, _ = ReadArtifact[CorrelateResult](caseDir, ArtifactFilename(StepF4Correlate))
	return prior
}

func loadAlwaysReadSources(catalog *knowledge.KnowledgeSourceCatalog) []AlwaysReadSource {
	alwaysSources := catalog.AlwaysReadSources()
	if len(alwaysSources) == 0 {
		return nil
	}
	var result []AlwaysReadSource
	for _, s := range alwaysSources {
		if s.LocalPath == "" {
			continue
		}
		content, err := os.ReadFile(s.LocalPath)
		if err != nil || len(content) == 0 {
			continue
		}
		result = append(result, AlwaysReadSource{
			Name:    s.Name,
			Purpose: s.Purpose,
			Content: string(content),
		})
	}
	return result
}

// findRecallCandidates searches the store for symptoms matching the test name.
// At F0_RECALL the case hasn't been triaged yet (SymptomID == 0), so we match
// on test name — the most reliable attribute available before triage.
func findRecallCandidates(st store.Store, testName string) *HistoryParams {
	if testName == "" {
		return nil
	}
	candidates, err := st.FindSymptomCandidates(testName)
	if err != nil || len(candidates) == 0 {
		return nil
	}

	best := candidates[0]
	for _, c := range candidates[1:] {
		if c.OccurrenceCount > best.OccurrenceCount {
			best = c
		} else if c.OccurrenceCount == best.OccurrenceCount && c.LastSeenAt > best.LastSeenAt {
			best = c
		}
	}

	history := loadHistory(st, best.ID)
	if history == nil {
		return nil
	}

	if best.Status == "dormant" && history.SymptomInfo != nil {
		history.SymptomInfo.IsDormantReactivation = true
	}

	return history
}

func buildRecallDigest(st store.Store) []RecallDigestEntry {
	rcas, err := st.ListRCAs()
	if err != nil || len(rcas) == 0 {
		return nil
	}
	digest := make([]RecallDigestEntry, 0, len(rcas))
	for _, rca := range rcas {
		summary := rca.Description
		if len(summary) > 200 {
			summary = summary[:200] + "..."
		}
		digest = append(digest, RecallDigestEntry{
			ID:         rca.ID,
			Component:  rca.Component,
			DefectType: rca.DefectType,
			Summary:    summary,
		})
	}
	return digest
}

func loadHistory(st store.Store, symptomID int64) *HistoryParams {
	history := &HistoryParams{}

	sym, err := st.GetSymptom(symptomID)
	if err == nil && sym != nil {
		history.SymptomInfo = &SymptomInfoParams{
			Name:            sym.Name,
			OccurrenceCount: sym.OccurrenceCount,
			FirstSeen:       sym.FirstSeenAt,
			LastSeen:         sym.LastSeenAt,
			Status:          sym.Status,
		}
	}

	links, err := st.GetRCAsForSymptom(symptomID)
	if err == nil {
		for _, link := range links {
			rca, err := st.GetRCAV2(link.RCAID)
			if err != nil || rca == nil {
				continue
			}
			history.PriorRCAs = append(history.PriorRCAs, PriorRCAParams{
				ID:               rca.ID,
				Title:            rca.Title,
				DefectType:       rca.DefectType,
				Status:           rca.Status,
				AffectedVersions: rca.AffectedVersions,
				JiraLink:         rca.JiraLink,
				ResolvedAt:       rca.ResolvedAt,
			})
		}
	}

	return history
}

// LayeredCatalog filters a catalog using layered routing: base → version →
// investigation. Returns a new catalog containing only the matched sources
// in dependency-resolved order.
func LayeredCatalog(
	catalog *knowledge.KnowledgeSourceCatalog,
	version string,
	component string,
	deps *knowledge.DepGraph,
) *knowledge.KnowledgeSourceCatalog {
	if catalog == nil {
		return nil
	}

	baseTags := map[string]string{knowledge.LayerTagKey: knowledge.LayerBase}
	versionTags := map[string]string{knowledge.LayerTagKey: knowledge.LayerVersion}
	investigationTags := map[string]string{knowledge.LayerTagKey: knowledge.LayerInvestigation}

	if component != "" {
		baseTags["role"] = "sut"
	}
	if version != "" {
		versionTags["version"] = version
	}

	router := knowledge.NewRouter(catalog, knowledge.RequestTagMatchRule{})
	sources := router.LayeredRoute(baseTags, versionTags, investigationTags)

	if deps != nil {
		ordered, err := deps.OrderSources(sources)
		if err == nil {
			sources = ordered
		}
	}

	return &knowledge.KnowledgeSourceCatalog{Sources: sources}
}
