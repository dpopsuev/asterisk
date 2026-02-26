// Package scenarios provides embedded calibration scenario definitions.
package scenarios

import "asterisk/internal/calibrate"

// PTPMockScenario returns the "PTP Calibration World" mock scenario.
// 3 versions, 3 pipelines, 12 cases, 4 symptoms, 3 RCAs.
func PTPMockScenario() *calibrate.Scenario {
	return &calibrate.Scenario{
		Name:             "ptp-mock",
		Description:      "PTP Calibration World: synthetic ground truth with 12 cases, 4 symptoms, 3 RCAs across 3 versions",
		DryCappedMetrics: []string{"M12", "M13"},
		RCAs:             ptpMockRCAs(),
		Symptoms:         ptpMockSymptoms(),
		Cases:            ptpMockCases(),
		Workspace:        ptpMockWorkspace(),
	}
}

func ptpMockRCAs() []calibrate.GroundTruthRCA {
	return []calibrate.GroundTruthRCA{
		{
			ID:               "R1",
			Title:            "Holdover timeout reduced from 300s to 60s in operator 4.21.0-202602070620",
			Description:      "The holdover timeout was changed from 300 seconds to 60 seconds in linuxptp-daemon config, causing PTP sync to fail prematurely under normal clock recovery scenarios.",
			DefectType:       "pb001",
			Category:         "product",
			Component:        "linuxptp-daemon",
			AffectedVersions: []string{"4.20", "4.21", "4.22"},
			JiraID:           "OCPBUGS-1001",
			RequiredKeywords: []string{"holdover", "timeout", "60", "300", "linuxptp", "FREERUN"},
			KeywordThreshold: 3,
			RelevantRepos:    []string{"linuxptp-daemon-operator"},
		},
		{
			ID:               "R2",
			Title:            "Test cleanup missing: PTP config CRD not deleted between test suites",
			Description:      "The AfterSuite cleanup in ptp_config_test.go was commented out, leaving stale PtpConfig CRDs that cause assertion failures in subsequent tests.",
			DefectType:       "au001",
			Category:         "automation",
			Component:        "ptp-test-framework",
			AffectedVersions: []string{"4.21"},
			JiraID:           "OCPBUGS-1002",
			RequiredKeywords: []string{"cleanup", "CRD", "AfterSuite", "isolation", "ptp-config"},
			KeywordThreshold: 2,
			RelevantRepos:    []string{"ptp-test-framework"},
		},
		{
			ID:               "R3",
			Title:            "NTP server unreachable from test cluster node (infra flap)",
			Description:      "The NTP server configured in chrony.conf became unreachable from the test cluster, causing chronyd to lose all selectable sources. Infrastructure issue, not a code bug.",
			DefectType:       "en001",
			Category:         "environment",
			Component:        "cluster-infra",
			AffectedVersions: []string{"4.21"},
			RequiredKeywords: []string{"NTP", "unreachable", "chronyd", "infra"},
			KeywordThreshold: 2,
			RelevantRepos:    []string{"cluster-infra-config"},
		},
	}
}

func ptpMockSymptoms() []calibrate.GroundTruthSymptom {
	return []calibrate.GroundTruthSymptom{
		{
			ID:           "S1",
			Name:         "ptp4l sync timeout",
			ErrorPattern: `ptp4l.*FREERUN.*holdover exceeded`,
			Component:    "linuxptp-daemon",
			MapsToRCA:    "R1",
		},
		{
			ID:           "S2",
			Name:         "Stale PTP config CRD assertion",
			ErrorPattern: `Expected PtpConfig.*not to exist.*but it does`,
			Component:    "ptp-test-framework",
			MapsToRCA:    "R2",
		},
		{
			ID:           "S3",
			Name:         "NTP sync loss",
			ErrorPattern: `chronyd.*no selectable sources`,
			Component:    "cluster-infra",
			MapsToRCA:    "R3",
		},
		{
			ID:           "S4",
			Name:         "ptp4l recovery failure",
			ErrorPattern: `ptp4l.*failed to recover lock within.*seconds`,
			Component:    "linuxptp-daemon",
			MapsToRCA:    "R1",
		},
	}
}

func ptpMockCases() []calibrate.GroundTruthCase {
	return []calibrate.GroundTruthCase{
		// C1: First-time investigation — full path, discovers R1 from scratch.
		// Triage sees 2 candidate repos → goes to F2 Resolve → F3 Investigate → F4 → F5 → F6.
		{
			ID: "C1", Version: "4.20", Job: "[T-TSC]",
			TestName:     "OCP-83297 PTP sync stability",
			ErrorMessage: "ptp4l[12345.678]: port 1: FREERUN state, holdover exceeded after 60s (expected 300s)",
			LogSnippet:   "2026-02-01T10:00:00Z ptp4l[12345.678]: port 1: FREERUN state, holdover exceeded after 60s\n2026-02-01T10:00:01Z ptp4l[12345.678]: timed out waiting for clock recovery\nFAIL: Expected clock state to be LOCKED within 300s timeout",
			SymptomID:    "S1", RCAID: "R1",
			ExpectedPath:    []string{"F0", "F1", "F2", "F3", "F4", "F5", "F6"},
			ExpectRecallHit: false,
			ExpectedLoops:   0,
			ExpectedRecall:  &calibrate.ExpectedRecall{Match: false, Confidence: 0.0},
			ExpectedTriage: &calibrate.ExpectedTriage{
				SymptomCategory: "product", Severity: "critical",
				DefectTypeHypothesis: "pb001",
				CandidateRepos:       []string{"linuxptp-daemon-operator", "cluster-infra-config"},
			},
			ExpectedResolve: &calibrate.ExpectedResolve{
				SelectedRepos: ptpResolveRepos("linuxptp-daemon-operator"),
			},
			ExpectedInvest: &calibrate.ExpectedInvest{
				RCAMessage:       "Holdover timeout reduced from 300s to 60s in linuxptp-daemon config (commit abc1234 on release-4.21). The ptp4l process enters FREERUN state and cannot recover because the holdover period is too short.",
				DefectType:       "pb001",
				Component:        "linuxptp-daemon",
				ConvergenceScore: 0.85,
				EvidenceRefs:     []string{"linuxptp-daemon-operator:pkg/daemon/config.go:abc1234"},
			},
			ExpectedCorrelate: &calibrate.ExpectedCorrelate{
				IsDuplicate: false, Confidence: 0.3,
			},
			ExpectedReview: &calibrate.ExpectedReview{Decision: "approve"},
		},
		// C2: Same symptom as C1 but different job.
		// Recall hit (H1, confidence 0.95) → skip to F5 Review → F6 Report.
		{
			ID: "C2", Version: "4.20", Job: "[T-BC]",
			TestName:     "OCP-83297 PTP sync stability",
			ErrorMessage: "ptp4l[23456.789]: port 1: FREERUN state, holdover exceeded after 60s (expected 300s)",
			LogSnippet:   "2026-02-01T11:30:00Z ptp4l[23456.789]: port 1: FREERUN state, holdover exceeded after 60s\nFAIL: Expected clock state to be LOCKED within 300s timeout",
			SymptomID:    "S1", RCAID: "R1",
			ExpectedPath:    []string{"F0", "F5", "F6"},
			ExpectRecallHit: true,
			ExpectedLoops:   0,
			ExpectedRecall: &calibrate.ExpectedRecall{
				Match: true, Confidence: 0.95,
			},
			ExpectedReview: &calibrate.ExpectedReview{Decision: "approve"},
		},
		// C3: Serial killer recall — same symptom (S1), version 4.21.
		// Recall hit → F5 → F6.
		{
			ID: "C3", Version: "4.21", Job: "[T-TSC]",
			TestName:     "OCP-83297 PTP sync stability",
			ErrorMessage: "ptp4l[34567.890]: port 1: FREERUN state, holdover exceeded after 60s (expected 300s)",
			LogSnippet:   "2026-02-05T08:15:00Z ptp4l[34567.890]: port 1: FREERUN state, holdover exceeded after 60s\nFAIL: Expected clock state to be LOCKED within 300s timeout",
			SymptomID:    "S1", RCAID: "R1",
			ExpectedPath:    []string{"F0", "F5", "F6"},
			ExpectRecallHit: true,
			ExpectedLoops:   0,
			ExpectedRecall: &calibrate.ExpectedRecall{
				Match: true, Confidence: 0.95,
			},
			ExpectedReview: &calibrate.ExpectedReview{Decision: "approve"},
		},
		// C4: Automation bug — missing cleanup.
		// Triage sees single candidate repo (H7) → skip F2, go to F3 → F4 → F5 → F6.
		{
			ID: "C4", Version: "4.21", Job: "[T-TSC]",
			TestName:     "OCP-83299 PTP config isolation",
			ErrorMessage: "Expected PtpConfig 'test-ptp-config' not to exist in namespace 'openshift-ptp' but it does",
			LogSnippet:   "2026-02-05T09:00:00Z STEP: Verifying PTP config isolation\n2026-02-05T09:00:01Z Expected PtpConfig 'test-ptp-config' not to exist in namespace 'openshift-ptp' but it does\nFAIL: Stale PTP config CRD found from previous test suite",
			SymptomID:    "S2", RCAID: "R2",
			ExpectedPath:    []string{"F0", "F1", "F3", "F4", "F5", "F6"},
			ExpectRecallHit: false,
			ExpectedLoops:   0,
			ExpectedRecall:  &calibrate.ExpectedRecall{Match: false, Confidence: 0.0},
			ExpectedTriage: &calibrate.ExpectedTriage{
				SymptomCategory: "automation", Severity: "high",
				DefectTypeHypothesis: "au001",
				CandidateRepos:       []string{"ptp-test-framework"},
			},
			ExpectedResolve: &calibrate.ExpectedResolve{
				SelectedRepos: ptpResolveRepos("ptp-test-framework"),
			},
			ExpectedInvest: &calibrate.ExpectedInvest{
				RCAMessage:       "AfterSuite cleanup in ptp_config_test.go is commented out. Stale PtpConfig CRDs from previous test suites are not deleted, causing assertion failures when the next suite checks for config isolation.",
				DefectType:       "au001",
				Component:        "ptp-test-framework",
				ConvergenceScore: 0.90,
				EvidenceRefs:     []string{"ptp-test-framework:test/e2e/ptp_config_test.go:AfterSuite"},
			},
			ExpectedCorrelate: &calibrate.ExpectedCorrelate{
				IsDuplicate: false, Confidence: 0.3,
			},
			ExpectedReview: &calibrate.ExpectedReview{Decision: "approve"},
		},
		// C5: Same symptom as C4 — recall hit → F5 → F6.
		{
			ID: "C5", Version: "4.21", Job: "[T-TSC]",
			TestName:     "OCP-83300 PTP config cleanup",
			ErrorMessage: "Expected PtpConfig 'cleanup-config' not to exist in namespace 'openshift-ptp' but it does",
			LogSnippet:   "2026-02-05T09:30:00Z Expected PtpConfig 'cleanup-config' not to exist\nFAIL: CRD cleanup failure",
			SymptomID:    "S2", RCAID: "R2",
			ExpectedPath:    []string{"F0", "F5", "F6"},
			ExpectRecallHit: true,
			ExpectedLoops:   0,
			ExpectedRecall: &calibrate.ExpectedRecall{
				Match: true, Confidence: 0.92,
			},
			ExpectedReview: &calibrate.ExpectedReview{Decision: "approve"},
		},
		// C6: Serial killer — S1 in 4.21/T-BC. Recall hit → F5 → F6.
		{
			ID: "C6", Version: "4.21", Job: "[T-BC]",
			TestName:     "OCP-83297 PTP sync stability",
			ErrorMessage: "ptp4l[45678.901]: port 1: FREERUN state, holdover exceeded after 60s (expected 300s)",
			LogSnippet:   "2026-02-05T11:00:00Z ptp4l[45678.901]: port 1: FREERUN state, holdover exceeded after 60s\nFAIL: Expected clock state to be LOCKED",
			SymptomID:    "S1", RCAID: "R1",
			ExpectedPath:    []string{"F0", "F5", "F6"},
			ExpectRecallHit: true,
			ExpectedLoops:   0,
			ExpectedRecall: &calibrate.ExpectedRecall{
				Match: true, Confidence: 0.95,
			},
			ExpectedReview: &calibrate.ExpectedReview{Decision: "approve"},
		},
		// C7: S2 recall hit in T-BC → F5 → F6.
		{
			ID: "C7", Version: "4.21", Job: "[T-BC]",
			TestName:     "OCP-83300 PTP config cleanup",
			ErrorMessage: "Expected PtpConfig 'bc-test-config' not to exist in namespace 'openshift-ptp' but it does",
			LogSnippet:   "2026-02-05T11:30:00Z Expected PtpConfig 'bc-test-config' not to exist\nFAIL: Stale CRD",
			SymptomID:    "S2", RCAID: "R2",
			ExpectedPath:    []string{"F0", "F5", "F6"},
			ExpectRecallHit: true,
			ExpectedLoops:   0,
			ExpectedRecall: &calibrate.ExpectedRecall{
				Match: true, Confidence: 0.92,
			},
			ExpectedReview: &calibrate.ExpectedReview{Decision: "approve"},
		},
		// C8: Infrastructure skip — NTP failure. Triage infra (H4) → F5 → F6.
		{
			ID: "C8", Version: "4.21", Job: "[BC-OC]",
			TestName:     "OCP-49734 NTP sync validation",
			ErrorMessage: "chronyd[5678]: no selectable sources — all NTP servers unreachable",
			LogSnippet:   "2026-02-05T12:00:00Z chronyd[5678]: no selectable sources\n2026-02-05T12:00:01Z chronyd[5678]: can't synchronise: no NTP server reachable\nFAIL: Expected NTP sync within 60s",
			SymptomID:    "S3", RCAID: "R3",
			ExpectedPath:    []string{"F0", "F1", "F5", "F6"},
			ExpectRecallHit: false,
			ExpectSkip:      true,
			ExpectedLoops:   0,
			ExpectedRecall:  &calibrate.ExpectedRecall{Match: false, Confidence: 0.0},
			ExpectedTriage: &calibrate.ExpectedTriage{
				SymptomCategory:      "infra",
				Severity:             "medium",
				DefectTypeHypothesis: "en001",
				SkipInvestigation:    true,
			},
			ExpectedReview: &calibrate.ExpectedReview{Decision: "approve"},
		},
		// C9: Serial killer 3rd version — S1 in 4.22. Recall hit → F5 → F6.
		{
			ID: "C9", Version: "4.22", Job: "[T-TSC]",
			TestName:     "OCP-83297 PTP sync stability",
			ErrorMessage: "ptp4l[56789.012]: port 1: FREERUN state, holdover exceeded after 60s (expected 300s)",
			LogSnippet:   "2026-02-10T08:00:00Z ptp4l[56789.012]: port 1: FREERUN state, holdover exceeded after 60s\nFAIL: Expected clock state to be LOCKED",
			SymptomID:    "S1", RCAID: "R1",
			ExpectedPath:    []string{"F0", "F5", "F6"},
			ExpectRecallHit: true,
			ExpectedLoops:   0,
			ExpectedRecall: &calibrate.ExpectedRecall{
				Match: true, Confidence: 0.95,
			},
			ExpectedReview: &calibrate.ExpectedReview{Decision: "approve"},
		},
		// C10: Different symptom (S4) but same criminal (R1).
		// Triage single candidate (H7) → F3 → F4 correlate (H15 dup >= 0.80) → DONE.
		{
			ID: "C10", Version: "4.22", Job: "[T-TSC]",
			TestName:     "OCP-83302 PTP recovery test",
			ErrorMessage: "ptp4l[67890.123]: failed to recover lock within 120 seconds after holdover",
			LogSnippet:   "2026-02-10T09:00:00Z ptp4l[67890.123]: failed to recover lock within 120 seconds\n2026-02-10T09:02:00Z ptp4l[67890.123]: recovery timeout, staying in FREERUN\nFAIL: PTP lock recovery failed",
			SymptomID:    "S4", RCAID: "R1",
			ExpectedPath:    []string{"F0", "F1", "F3", "F4"},
			ExpectRecallHit: false,
			ExpectedLoops:   0,
			ExpectedRecall:  &calibrate.ExpectedRecall{Match: false, Confidence: 0.1},
			ExpectedTriage: &calibrate.ExpectedTriage{
				SymptomCategory: "product", Severity: "critical",
				DefectTypeHypothesis: "pb001",
				CandidateRepos:       []string{"linuxptp-daemon-operator"},
			},
			ExpectedResolve: &calibrate.ExpectedResolve{
				SelectedRepos: ptpResolveRepos("linuxptp-daemon-operator"),
			},
			ExpectedInvest: &calibrate.ExpectedInvest{
				RCAMessage:       "PTP lock recovery fails because the holdover timeout was reduced from 300s to 60s. The ptp4l process enters FREERUN after 60s and never recovers. Same root cause as the sync stability failures — commit abc1234 on release-4.21.",
				DefectType:       "pb001",
				Component:        "linuxptp-daemon",
				ConvergenceScore: 0.80,
				EvidenceRefs:     []string{"linuxptp-daemon-operator:pkg/daemon/config.go:abc1234"},
			},
			ExpectedCorrelate: &calibrate.ExpectedCorrelate{
				IsDuplicate: true, Confidence: 0.88, CrossVersionMatch: true,
			},
			ExpectedReview: &calibrate.ExpectedReview{Decision: "approve"},
		},
		// C11: Flake skip — no real RCA. Triage flake (H5) → F5 → F6.
		{
			ID: "C11", Version: "4.21", Job: "[T-TSC]",
			TestName:     "OCP-83303 PTP flaky timing",
			ErrorMessage: "Timed out after 10.000s: Expected PTP offset to stabilize within 10s (intermittent)",
			LogSnippet:   "2026-02-05T14:00:00Z ptp4l: offset +1234567ns, s2 LOCKED\n2026-02-05T14:00:05Z ptp4l: offset -987654ns, s2 LOCKED\nFAIL: Timed out after 10.000s",
			SymptomID:    "", RCAID: "",
			ExpectedPath:    []string{"F0", "F1", "F5", "F6"},
			ExpectRecallHit: false,
			ExpectSkip:      true,
			ExpectedLoops:   0,
			ExpectedRecall:  &calibrate.ExpectedRecall{Match: false, Confidence: 0.0},
			ExpectedTriage: &calibrate.ExpectedTriage{
				SymptomCategory:      "flake",
				Severity:             "low",
				DefectTypeHypothesis: "nd001",
				SkipInvestigation:    true,
			},
			ExpectedReview: &calibrate.ExpectedReview{Decision: "approve"},
		},
		// C12: Cascade detection — BeforeSuite failure from C4's root cause.
		// Triage detects cascade, recommends investigation with single candidate.
		// H7 single repo → F3 → F4 (correlate dup → DONE).
		{
			ID: "C12", Version: "4.21", Job: "[T-TSC]",
			TestName:     "OCP-83304 PTP ordered setup",
			ErrorMessage: "Expected PtpConfig 'ordered-setup' not to exist in namespace 'openshift-ptp' but it does [cascade from OCP-83299]",
			LogSnippet:   "2026-02-05T15:00:00Z BeforeSuite: checking PTP config state\n2026-02-05T15:00:01Z Expected PtpConfig 'ordered-setup' not to exist\nFAIL: Cascade failure from stale CRD (related to OCP-83299)",
			SymptomID:    "S2", RCAID: "R2",
			ExpectedPath:    []string{"F0", "F1", "F3", "F4"},
			ExpectRecallHit: false,
			ExpectCascade:   true,
			ExpectedLoops:   0,
			ExpectedRecall:  &calibrate.ExpectedRecall{Match: false, Confidence: 0.15},
			ExpectedTriage: &calibrate.ExpectedTriage{
				SymptomCategory:      "automation",
				Severity:             "high",
				DefectTypeHypothesis: "au001",
				CandidateRepos:       []string{"ptp-test-framework"},
				CascadeSuspected:     true,
			},
			ExpectedResolve: &calibrate.ExpectedResolve{
				SelectedRepos: ptpResolveRepos("ptp-test-framework"),
			},
			ExpectedInvest: &calibrate.ExpectedInvest{
				RCAMessage:       "Cascade from stale PtpConfig CRD. Same root cause as OCP-83299 — AfterSuite cleanup missing.",
				DefectType:       "au001",
				Component:        "ptp-test-framework",
				ConvergenceScore: 0.85,
				EvidenceRefs:     []string{"ptp-test-framework:test/e2e/ptp_config_test.go:AfterSuite"},
			},
			ExpectedCorrelate: &calibrate.ExpectedCorrelate{
				IsDuplicate: true, Confidence: 0.90,
			},
			ExpectedReview: &calibrate.ExpectedReview{Decision: "approve"},
		},
	}
}

func ptpMockWorkspace() calibrate.WorkspaceConfig {
	return calibrate.WorkspaceConfig{
		Repos: []calibrate.RepoConfig{
			{
				Name:           "linuxptp-daemon-operator",
				Purpose:        "PTP operator: manages linuxptp-daemon DaemonSet, PtpConfig CRD, clock sync",
				Branch:         "release-4.21",
				RelevantToRCAs: []string{"R1"},
			},
			{
				Name:           "ptp-test-framework",
				Purpose:        "E2E test suite for PTP operator: Ginkgo specs, test helpers, fixtures",
				Branch:         "main",
				RelevantToRCAs: []string{"R2"},
			},
			{
				Name:           "cluster-infra-config",
				Purpose:        "CI cluster configuration: job profiles, NTP config, network templates",
				Branch:         "main",
				RelevantToRCAs: []string{"R3"},
			},
			{
				Name:         "sriov-network-operator",
				Purpose:      "SR-IOV network operator: VF allocation, device plugin (NOT PTP-related)",
				Branch:       "release-4.21",
				IsRedHerring: true,
			},
			{
				Name:    "cnf-features-deploy",
				Purpose: "CNF deployment manifests and CI profiles: contains job definitions for all telco operators",
				Branch:  "master",
			},
		},
	}
}

func ptpResolveRepos(name string) []calibrate.ExpectedResolveRepo {
	purposeMap := map[string]string{
		"linuxptp-daemon-operator": "Product code with holdover config",
		"ptp-test-framework":       "Test code with missing cleanup",
		"cluster-infra-config":     "Infrastructure configuration",
	}
	return []calibrate.ExpectedResolveRepo{{
		Name:   name,
		Reason: purposeMap[name],
	}}
}
