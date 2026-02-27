// PTPRealScenario returns the real calibration scenario from OCPBUGS-74895 (broken pipe)
// and OCPBUGS-74904 (config change hang).
//
// Ground truth:
//   - 2 RCAs (linked as duplicates — same underlying daemon process management bug)
//   - 3 symptoms
//   - 8 cases from real RP launches 32764 / 32719
//
// Real test names, real error messages, real file:line references.
// Uses StubAdapter for deterministic validation; real LLM/Cursor runs come later.
package scenarios

import (
	"asterisk/internal/calibrate"

	"github.com/dpopsuev/origami/knowledge"
)

// PTPRealScenario returns the real-world calibration scenario.
func PTPRealScenario() *calibrate.Scenario {
	return &calibrate.Scenario{
		Name:        "ptp-real",
		Description: "Real PTP Recovery: OCPBUGS-74895 (broken pipe) + OCPBUGS-74904 (config hang), 8 failures, 3 symptoms, 2 RCAs",
		RCAs:        ptpRealRCAs(),
		Symptoms:    ptpRealSymptoms(),
		Cases:       ptpRealCases(),
		Workspace:   ptpRealWorkspace(),
	}
}

func ptpRealRCAs() []calibrate.GroundTruthRCA {
	return []calibrate.GroundTruthRCA{
		{
			ID:               "R1",
			Title:            "Broken pipe: daemon→proxy events.sock communication — pipe buffer clog causes concatenated burst at receiver",
			Description:      "OCPBUGS-74895. The linuxptp-daemon writes PTP events to cloud-event-proxy via Unix socket /cloud-native/events.sock. Under burst conditions (node reboot, interface down) the pipe buffer clogs causing EPIPE. Concatenated burst at receiver corrupts the event stream. Duplicate of R2.",
			DefectType:       "pb001",
			Category:         "product",
			Component:        "linuxptp-daemon",
			AffectedVersions: []string{"4.18", "4.19"},
			JiraID:           "OCPBUGS-74895",
			RequiredKeywords: []string{"broken", "pipe", "events.sock", "socket", "buffer", "EPIPE", "cloud-event-proxy"},
			KeywordThreshold: 3,
			RelevantRepos:    []string{"linuxptp-daemon", "cloud-event-proxy", "cnf-gotests"},
		},
		{
			ID:               "R2",
			Title:            "Config change hang: daemon detects PTP config change, stops children services, doesn't bring them up again",
			Description:      "OCPBUGS-74904. When PTP configuration changes, the linuxptp-daemon kills child processes (ptp4l, phc2sys) but fails to restart them. The daemon enters a stuck state with no PTP synchronization. Same underlying root cause as OCPBUGS-74895 (broken pipe). Both are in Networking/PTP, severity Critical, marked Regression.",
			DefectType:       "pb001",
			Category:         "product",
			Component:        "linuxptp-daemon",
			AffectedVersions: []string{"4.18", "4.19"},
			JiraID:           "OCPBUGS-74904",
			RequiredKeywords: []string{"config", "change", "hang", "restart", "children", "ptp4l", "phc2sys", "kill", "stuck"},
			KeywordThreshold: 3,
			RelevantRepos:    []string{"linuxptp-daemon"},
		},
	}
}

func ptpRealSymptoms() []calibrate.GroundTruthSymptom {
	return []calibrate.GroundTruthSymptom{
		{
			ID:           "S1",
			Name:         "Config change hang — process restart failure",
			ErrorPattern: `phc2sys.*ptp4l.*process.*kill.*daemon.*restart|PANIC.*recovery.*path`,
			Component:    "linuxptp-daemon",
			MapsToRCA:    "R2",
		},
		{
			ID:           "S2",
			Name:         "Broken pipe — event socket write failure",
			ErrorPattern: `events\.sock.*broken pipe|consumer events lost.*reboot|EPIPE`,
			Component:    "linuxptp-daemon",
			MapsToRCA:    "R1",
		},
		{
			ID:           "S3",
			Name:         "BeforeEach cascade — 2-port OC setup fails",
			ErrorPattern: `BeforeEach.*ptp_interfaces\.go:498.*2-port.*OC`,
			Component:    "cnf-gotests",
			MapsToRCA:    "R1",
		},
	}
}

func ptpRealCases() []calibrate.GroundTruthCase {
	return []calibrate.GroundTruthCase{
		// ===== Config change hang (R2 / OCPBUGS-74904) — 3 failures =====

		// C1: phc2sys restart (FAIL). First investigation — full pipeline.
		// Triage: product, 2 candidates → F2 → F3 → F4 (not dup) → F5 → F6.
		{
			ID: "C1", Version: "4.18", Job: "[recovery]",
			TestName:     "PTP Recovery > ptp process restart > should recover the phc2sys process after killing it",
			TestID:       "59862",
			ErrorMessage: "ptp_recovery.go:121: Expected phc2sys process to restart within 30s after config change, but daemon is stuck — no child process restart observed",
			LogSnippet:   "2026-01-10T08:15:00Z [STEP] Kill phc2sys and wait for restart\n2026-01-10T08:15:01Z daemon: PTP config change detected, stopping children\n2026-01-10T08:15:02Z daemon: killed phc2sys (pid 12345)\n2026-01-10T08:15:32Z FAIL: ptp_recovery.go:121\nExpected phc2sys process to restart within 30s\nbut daemon is stuck — no child process restart observed",
			SymptomID: "S1", RCAID: "R2",
			ExpectedPath:    []string{"F0", "F1", "F2", "F3", "F4", "F5", "F6"},
			ExpectRecallHit: false,
			ExpectedLoops:   0,
			ExpectedRecall:  &calibrate.ExpectedRecall{Match: false, Confidence: 0.0},
			ExpectedTriage: &calibrate.ExpectedTriage{
				SymptomCategory:      "product",
				Severity:             "critical",
				DefectTypeHypothesis: "pb001",
				CandidateRepos:       []string{"linuxptp-daemon", "ptp-operator"},
			},
			ExpectedResolve: &calibrate.ExpectedResolve{
				SelectedRepos: []calibrate.ExpectedResolveRepo{
					{Name: "linuxptp-daemon", Reason: "SUT daemon with config change and process management code"},
				},
			},
			ExpectedInvest: &calibrate.ExpectedInvest{
				RCAMessage:       "Config change handler in linuxptp-daemon kills child processes (phc2sys, ptp4l) but fails to restart them. The daemon detects a PTP config change, stops children services, but never brings them back up. OCPBUGS-74904.",
				DefectType:       "pb001",
				Component:        "linuxptp-daemon",
				ConvergenceScore: 0.85,
				EvidenceRefs:     []string{"linuxptp-daemon:pkg/daemon/daemon.go:config_change_handler"},
			},
			ExpectedCorrelate: &calibrate.ExpectedCorrelate{
				IsDuplicate: false, Confidence: 0.3,
			},
			ExpectedReview: &calibrate.ExpectedReview{Decision: "approve"},
		},

		// C2: ptp4l restart (PANICKED). Same symptom (S1) as C1, different failure type.
		// Recall hit (H1) → F5 → F6.
		{
			ID: "C2", Version: "4.18", Job: "[recovery]",
			TestName:     "PTP Recovery > ptp process restart > should recover the ptp4l process after killing a ptp4l process related to phc2sys",
			TestID:       "49737",
			ErrorMessage: "panic.go:115: PANICKED — ptp4l process recovery failed, daemon stuck in kill loop [recovered]",
			LogSnippet:   "2026-01-10T08:45:00Z [STEP] Kill ptp4l related to phc2sys\n2026-01-10T08:45:01Z daemon: config change — killing ptp4l\n2026-01-10T08:45:02Z PANIC: panic.go:115\nptp4l process recovery failed — daemon stuck in kill loop\n[recovered]",
			SymptomID: "S1", RCAID: "R2",
			ExpectedPath:    []string{"F0", "F5", "F6"},
			ExpectRecallHit: true,
			ExpectedLoops:   0,
			ExpectedRecall: &calibrate.ExpectedRecall{
				Match: true, Confidence: 0.95,
			},
			ExpectedReview: &calibrate.ExpectedReview{Decision: "approve"},
		},

		// C3: consumer events (FAIL AfterEach). Same symptom (S1).
		// Recall hit → F5 → F6.
		{
			ID: "C3", Version: "4.18", Job: "[events]",
			TestName:     "PTP Recovery > HTTP events using consumer validates system fully functional after removing consumer",
			TestID:       "59996",
			ErrorMessage: "[AfterEach] ptp_events_and_metrics.go:175: daemon failed to restart ptp4l after consumer removal — config change hang",
			LogSnippet:   "2026-01-10T09:00:00Z [AfterEach] ptp_events_and_metrics.go:175\n2026-01-10T09:00:01Z cleanup: removing event consumer\n2026-01-10T09:00:02Z daemon: config change triggered by consumer removal\n2026-01-10T09:00:32Z FAIL [AfterEach]: daemon failed to restart ptp4l\nconfig change hang after consumer removal",
			SymptomID: "S1", RCAID: "R2",
			ExpectedPath:    []string{"F0", "F5", "F6"},
			ExpectRecallHit: true,
			ExpectedLoops:   0,
			ExpectedRecall: &calibrate.ExpectedRecall{
				Match: true, Confidence: 0.92,
			},
			ExpectedReview: &calibrate.ExpectedReview{Decision: "approve"},
		},

		// ===== Broken pipe (R1 / OCPBUGS-74895) — 5 failures =====

		// C4: node reboot (FAIL). New symptom (S2), full investigation.
		// Triage: product, 2 candidates → F2 → F3 → F4 (not dup) → F5 → F6.
		{
			ID: "C4", Version: "4.18", Job: "[recovery]",
			TestName:     "PTP Recovery > ptp node reboot > validates PTP consumer events after ptp node reboot",
			TestID:       "59995",
			ErrorMessage: "ptp_events_and_metrics.go:221: events.sock broken pipe after node reboot — PTP consumer events lost, event stream corrupted",
			LogSnippet:   "2026-01-10T10:00:00Z [STEP] Reboot PTP node and validate events\n2026-01-10T10:00:30Z daemon: node reboot complete, reinitializing PTP\n2026-01-10T10:00:35Z write /cloud-native/events.sock: broken pipe (EPIPE)\n2026-01-10T10:00:36Z cloud-event-proxy: consumer events lost after pipe break\nFAIL: ptp_events_and_metrics.go:221\nExpected PTP consumer events to resume after reboot\nbut events.sock broken pipe — stream corrupted",
			SymptomID: "S2", RCAID: "R1",
			ExpectedPath:    []string{"F0", "F1", "F2", "F3", "F4", "F5", "F6"},
			ExpectRecallHit: false,
			ExpectedLoops:   0,
			ExpectedRecall:  &calibrate.ExpectedRecall{Match: false, Confidence: 0.0},
			ExpectedTriage: &calibrate.ExpectedTriage{
				SymptomCategory:      "product",
				Severity:             "critical",
				DefectTypeHypothesis: "pb001",
				CandidateRepos:       []string{"linuxptp-daemon", "cloud-event-proxy"},
			},
			ExpectedResolve: &calibrate.ExpectedResolve{
				SelectedRepos: []calibrate.ExpectedResolveRepo{
					{Name: "linuxptp-daemon", Reason: "Daemon writes PTP events to events.sock"},
					{Name: "cloud-event-proxy", Reason: "Proxy reads events from events.sock"},
				},
			},
			ExpectedInvest: &calibrate.ExpectedInvest{
				RCAMessage:       "Broken pipe on Unix socket /cloud-native/events.sock (OCPBUGS-74895). The linuxptp-daemon writes PTP events to cloud-event-proxy. After node reboot, burst of events clogs the pipe buffer causing EPIPE. Events are lost and the stream is corrupted at the receiver. Same underlying daemon process management bug as OCPBUGS-74904.",
				DefectType:       "pb001",
				Component:        "linuxptp-daemon",
				ConvergenceScore: 0.82,
				EvidenceRefs:     []string{"linuxptp-daemon:pkg/event/socket.go:write_event", "cloud-event-proxy:pkg/receiver/handler.go:read_events"},
			},
			ExpectedCorrelate: &calibrate.ExpectedCorrelate{
				IsDuplicate: false, Confidence: 0.40,
			},
			ExpectedReview: &calibrate.ExpectedReview{Decision: "approve"},
		},

		// C5: interface down (FAIL). Same symptom (S2) as C4, different trigger.
		// Recall hit → F5 → F6.
		{
			ID: "C5", Version: "4.18", Job: "[events]",
			TestName:     "PTP Events and Metrics > interface down > should generate events when slave interface goes down and up",
			TestID:       "49742",
			ErrorMessage: "ptp_interfaces.go:753: events.sock broken pipe when slave interface ens3f0 went down — consumer events not received",
			LogSnippet:   "2026-01-10T11:00:00Z PTP: slave interface ens3f0 went down\n2026-01-10T11:00:01Z write /cloud-native/events.sock: broken pipe (EPIPE)\n2026-01-10T11:00:02Z consumer: no events received after interface down\nFAIL: ptp_interfaces.go:753\nExpected PTP events after interface down/up",
			SymptomID: "S2", RCAID: "R1",
			ExpectedPath:    []string{"F0", "F5", "F6"},
			ExpectRecallHit: true,
			ExpectedLoops:   0,
			ExpectedRecall: &calibrate.ExpectedRecall{
				Match: true, Confidence: 0.93,
			},
			ExpectedReview: &calibrate.ExpectedReview{Decision: "approve"},
		},

		// C6: 2-port OC failover (FAIL BeforeEach cascade). New symptom (S3).
		// Triage: cascade + single candidate (H7) → F3 → F4 (dup to R1, H15) → Done.
		{
			ID: "C6", Version: "4.18", Job: "[events]",
			TestName:     "PTP Events and Metrics > interface down OC 2 port > verifies 2-port oc ha failover when active port goes down",
			TestID:       "80963",
			ErrorMessage: "[BeforeEach] ptp_interfaces.go:498: failed to setup 2-port OC HA profile: events.sock broken pipe during initialization",
			LogSnippet:   "2026-01-10T12:00:00Z [BeforeEach] ptp_interfaces.go:498\nsetup 2-port OC HA profile\n2026-01-10T12:00:01Z events.sock: broken pipe during OC profile initialization\nFAIL [BeforeEach]: failed to setup 2-port OC HA profile",
			SymptomID: "S3", RCAID: "R1",
			ExpectedPath:    []string{"F0", "F1", "F3", "F4"},
			ExpectRecallHit: false,
			ExpectCascade:   true,
			ExpectedLoops:   0,
			ExpectedRecall:  &calibrate.ExpectedRecall{Match: false, Confidence: 0.15},
			ExpectedTriage: &calibrate.ExpectedTriage{
				SymptomCategory:      "product",
				Severity:             "high",
				DefectTypeHypothesis: "pb001",
				CandidateRepos:       []string{"cnf-gotests"},
				CascadeSuspected:     true,
			},
			ExpectedInvest: &calibrate.ExpectedInvest{
				RCAMessage:       "Cascade from events.sock broken pipe (OCPBUGS-74895). BeforeEach at ptp_interfaces.go:498 fails because the pipe is broken during 2-port OC HA profile initialization. Same root cause as the node reboot broken pipe failure. Three child tests (C6, C7, C8) all fail at this shared setup line.",
				DefectType:       "pb001",
				Component:        "linuxptp-daemon",
				ConvergenceScore: 0.82,
				EvidenceRefs:     []string{"cnf-gotests:test/ptp_interfaces.go:498"},
			},
			ExpectedCorrelate: &calibrate.ExpectedCorrelate{
				IsDuplicate: true, Confidence: 0.88,
			},
			ExpectedReview: &calibrate.ExpectedReview{Decision: "approve"},
		},

		// C7: 2-port OC holdover (FAIL BeforeEach). Same BeforeEach as C6.
		// Recall hit → F5 → F6.
		{
			ID: "C7", Version: "4.18", Job: "[events]",
			TestName:     "PTP Events and Metrics > interface down OC 2 port > verifies 2-port oc ha holdover & freerun when both ports go down",
			TestID:       "80964",
			ErrorMessage: "[BeforeEach] ptp_interfaces.go:498: failed to setup 2-port OC HA profile: events.sock broken pipe during initialization",
			LogSnippet:   "2026-01-10T12:00:02Z [BeforeEach] ptp_interfaces.go:498\nFAIL [BeforeEach]: same cascade — events.sock broken pipe",
			SymptomID: "S3", RCAID: "R1",
			ExpectedPath:    []string{"F0", "F5", "F6"},
			ExpectRecallHit: true,
			ExpectedLoops:   0,
			ExpectedRecall: &calibrate.ExpectedRecall{
				Match: true, Confidence: 0.92,
			},
			ExpectedReview: &calibrate.ExpectedReview{Decision: "approve"},
		},

		// C8: 2-port OC passive recovery (FAIL BeforeEach). Same BeforeEach as C6.
		// Recall hit → F5 → F6.
		{
			ID: "C8", Version: "4.18", Job: "[events]",
			TestName:     "PTP Events and Metrics > interface down OC 2 port > verifies 2-port oc ha passive interface recovery",
			TestID:       "82012",
			ErrorMessage: "[BeforeEach] ptp_interfaces.go:498: failed to setup 2-port OC HA profile: events.sock broken pipe during initialization",
			LogSnippet:   "2026-01-10T12:00:03Z [BeforeEach] ptp_interfaces.go:498\nFAIL [BeforeEach]: same cascade — events.sock broken pipe",
			SymptomID: "S3", RCAID: "R1",
			ExpectedPath:    []string{"F0", "F5", "F6"},
			ExpectRecallHit: true,
			ExpectedLoops:   0,
			ExpectedRecall: &calibrate.ExpectedRecall{
				Match: true, Confidence: 0.92,
			},
			ExpectedReview: &calibrate.ExpectedReview{Decision: "approve"},
		},
	}
}

func ptpRealWorkspace() calibrate.WorkspaceConfig {
	return calibrate.WorkspaceConfig{
		Sources: ptpDocSources(),
		Repos: []calibrate.RepoConfig{
			{
				Name:           "cnf-gotests",
				Purpose:        "PTP test cases (Ginkgo): test code, assertions, test helpers. Error stacks point here for cascade failures at ptp_interfaces.go:498.",
				Branch:         "master",
				RelevantToRCAs: []string{"R1"},
			},
			{
				Name:           "ptp-operator",
				Purpose:        "SUT: PTP operator lifecycle, manages linuxptp-daemon DaemonSet, PtpConfig CRD, PTP profiles",
				Branch:         "release-4.18",
				RelevantToRCAs: []string{},
			},
			{
				Name:           "linuxptp-daemon",
				Purpose:        "SUT: PTP daemon running on nodes — ptp4l, phc2sys, ts2phc processes; event socket communication with cloud-event-proxy; config change handling",
				Branch:         "release-4.18",
				RelevantToRCAs: []string{"R1", "R2"},
			},
			{
				Name:           "cloud-event-proxy",
				Purpose:        "Cloud Event Proxy: receives PTP events from daemon via Unix socket (/cloud-native/events.sock); publishes cloud events. Broken pipe issue is daemon→proxy communication.",
				Branch:         "release-4.18",
				RelevantToRCAs: []string{"R1"},
			},
			{
				Name:         "eco-gotests",
				Purpose:      "Ecosystem QE test framework; may contain shared helpers used by cnf-gotests (NOT PTP-specific)",
				Branch:       "master",
			IsRedHerring: true,
		},
	},
	}
}

func ptpDocSources() []knowledge.Source {
	return []knowledge.Source{
		{
			Name:       "ptp-operator-architecture",
			Kind:       knowledge.SourceKindDoc,
			Purpose:    "PTP architecture disambiguation: linuxptp-daemon (pod) vs linuxptp-daemon (repo), component relationships, event flow",
			ReadPolicy: knowledge.ReadAlways,
			LocalPath:  "datasets/docs/ptp/architecture.md",
		},
	}
}
