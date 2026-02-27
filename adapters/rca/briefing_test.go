package rca

import (
	"strings"
	"testing"

)

func TestGenerateBriefing_TriagePhase(t *testing.T) {
	md := GenerateBriefing(
		"ptp-mock", 1001, "triage", 1, 4, 10, 0,
		nil, nil, nil,
	)

	if !strings.Contains(md, "# Calibration Briefing — Batch 1") {
		t.Error("missing title")
	}
	if !strings.Contains(md, "Scenario: ptp-mock") {
		t.Error("missing scenario")
	}
	if !strings.Contains(md, "Suite ID: 1001") {
		t.Error("missing suite ID")
	}
	if !strings.Contains(md, "Phase: triage") {
		t.Error("missing phase")
	}
	if !strings.Contains(md, "Cases in this batch: 4") {
		t.Error("missing batch count")
	}
	// Should NOT have symptoms or clusters in triage phase with no prior data
	if strings.Contains(md, "Known symptoms") {
		t.Error("should not have symptoms section when no triage results")
	}
	if strings.Contains(md, "Cluster assignments") {
		t.Error("should not have clusters section when no clusters")
	}
}

func TestGenerateBriefing_InvestigationPhase(t *testing.T) {
	triageResults := []CalTriageResult{
		{
			Index: 0,
			CaseResult: &CaseResult{
				CaseID:          "C1",
				ActualComponent: "ptp4l",
			},
			TriageArtifact: &TriageResult{
				SymptomCategory:      "product",
				DefectTypeHypothesis: "pb001",
				Severity:             "high",
			},
		},
		{
			Index: 1,
			CaseResult: &CaseResult{
				CaseID:          "C2",
				ActualComponent: "phc2sys",
			},
			TriageArtifact: &TriageResult{
				SymptomCategory:      "infra",
				DefectTypeHypothesis: "ib003",
				Severity:             "medium",
			},
		},
	}

	clusters := []SymptomCluster{
		{
			Key:            "product|ptp4l|pb001",
			Representative: &triageResults[0],
			Members:        []*CalTriageResult{&triageResults[0]},
		},
		{
			Key:            "infra|phc2sys|ib003",
			Representative: &triageResults[1],
			Members:        []*CalTriageResult{&triageResults[1]},
		},
	}

	priorRCAs := []BriefingRCA{
		{ID: "R1", Component: "ptp4l", DefectType: "pb001", Summary: "Clock drift under load"},
	}

	md := GenerateBriefing(
		"ptp-real-ingest", 2001, "investigation", 3, 2, 10, 5,
		triageResults, clusters, priorRCAs,
	)

	if !strings.Contains(md, "Known symptoms") {
		t.Error("missing symptoms section")
	}
	if !strings.Contains(md, "C1") {
		t.Error("missing C1 in symptoms")
	}
	if !strings.Contains(md, "product") {
		t.Error("missing category in symptoms")
	}
	if !strings.Contains(md, "Cluster assignments") {
		t.Error("missing clusters section")
	}
	if !strings.Contains(md, "product / ptp4l / Product Bug") {
		t.Error("missing cluster key")
	}
	if !strings.Contains(md, "Prior RCAs") {
		t.Error("missing prior RCAs section")
	}
	if !strings.Contains(md, "Clock drift under load") {
		t.Error("missing RCA summary")
	}
	if !strings.Contains(md, "Completed so far: 5") {
		t.Error("missing completed count")
	}
}

func TestGenerateBriefing_EmptyCase(t *testing.T) {
	md := GenerateBriefing("empty", 0, "triage", 0, 0, 0, 0, nil, nil, nil)
	if !strings.Contains(md, "# Calibration Briefing") {
		t.Error("missing header")
	}
	if !strings.Contains(md, "Cases in this batch: 0") {
		t.Error("missing zero batch count")
	}
}
