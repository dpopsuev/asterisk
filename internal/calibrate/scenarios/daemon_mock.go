// DaemonMockScenario returns the "Daemon Process Management" mock calibration scenario.
// Models the patterns from the real OCPBUGS-74895/74904 investigation with synthetic data:
// 2 RCAs (broken pipe + config hang, same underlying root cause), 3 symptoms, 8 cases.
//
// Patterns exercised:
//   - Two Jiras, one root cause (R1 broken pipe ↔ R2 config hang)
//   - PANIC vs FAIL (C2 PANICKED, C1 FAIL — same symptom)
//   - AfterEach failure (C3)
//   - BeforeEach cascade (C6, C7, C8 — same file:line)
//   - Node reboot vs interface down (C4 reboot, C5 interface — same symptom)
package scenarios

import "asterisk/internal/calibrate"

// DaemonMockScenario returns the second mock scenario for calibration.
func DaemonMockScenario() *calibrate.Scenario {
	return &calibrate.Scenario{
		Name:        "daemon-mock",
		Description: "Daemon Process Management: 2 RCAs (broken pipe + config hang), 3 symptoms, 8 cases",
		RCAs:        daemonMockRCAs(),
		Symptoms:    daemonMockSymptoms(),
		Cases:       daemonMockCases(),
		Workspace:   daemonMockWorkspace(),
	}
}

func daemonMockRCAs() []calibrate.GroundTruthRCA {
	return []calibrate.GroundTruthRCA{
		{
			ID:               "R1",
			Title:            "Broken pipe: daemon→proxy events.sock communication buffer clog",
			Description:      "The linuxptp-daemon writes PTP events to cloud-event-proxy via a Unix socket (/cloud-native/events.sock). Under burst conditions the pipe buffer clogs, causing EPIPE on write. Concatenated burst at receiver corrupts event stream.",
			DefectType:       "pb001",
			Category:         "product",
			Component:        "linuxptp-daemon",
			AffectedVersions: []string{"4.18", "4.19"},
			JiraID:           "OCPBUGS-74895",
			RequiredKeywords: []string{"broken", "pipe", "events.sock", "socket", "buffer", "EPIPE"},
			KeywordThreshold: 3,
			RelevantRepos:    []string{"linuxptp-daemon", "cloud-event-proxy", "cnf-gotests"},
		},
		{
			ID:               "R2",
			Title:            "Config change hang: daemon stops children services, doesn't restart them",
			Description:      "When a PTP config change is detected, the linuxptp-daemon kills child processes (ptp4l, phc2sys) but fails to restart them. The daemon enters a stuck state where no PTP synchronization occurs.",
			DefectType:       "pb001",
			Category:         "product",
			Component:        "linuxptp-daemon",
			AffectedVersions: []string{"4.18", "4.19"},
			JiraID:           "OCPBUGS-74904",
			RequiredKeywords: []string{"config", "hang", "restart", "children", "ptp4l", "phc2sys", "kill"},
			KeywordThreshold: 3,
			RelevantRepos:    []string{"linuxptp-daemon"},
		},
	}
}

func daemonMockSymptoms() []calibrate.GroundTruthSymptom {
	return []calibrate.GroundTruthSymptom{
		{
			ID:           "S1",
			Name:         "Config change hang — process restart failure",
			ErrorPattern: `ptp4l.*process.*kill.*daemon.*restart|phc2sys.*PANIC.*recovery`,
			Component:    "linuxptp-daemon",
			MapsToRCA:    "R2",
		},
		{
			ID:           "S2",
			Name:         "Broken pipe — event socket write failure",
			ErrorPattern: `events\.sock.*broken pipe|EPIPE.*cloud-event-proxy`,
			Component:    "linuxptp-daemon",
			MapsToRCA:    "R1",
		},
		{
			ID:           "S3",
			Name:         "BeforeEach cascade — 2-port OC setup fails",
			ErrorPattern: `BeforeEach.*ptp_interfaces\.go:498.*2-port.*setup`,
			Component:    "cnf-gotests",
			MapsToRCA:    "R1",
		},
	}
}

func daemonMockCases() []calibrate.GroundTruthCase {
	return []calibrate.GroundTruthCase{
		// C1: Config hang — first investigation, full pipeline.
		// Triage sees 2 candidate repos → F2 → F3 → F4 (not dup) → F5 → F6.
		{
			ID: "C1", Version: "4.18", Job: "[recovery]",
			TestName:     "PTP Recovery > ptp process restart > should recover phc2sys after killing it",
			ErrorMessage: "ptp4l[8901]: process killed by daemon config change; daemon failed to restart phc2sys within 30s",
			LogSnippet:   "2026-01-15T10:00:00Z daemon: PTP config change detected, stopping children\n2026-01-15T10:00:01Z daemon: killed phc2sys (pid 4321)\n2026-01-15T10:00:31Z FAIL: ptp4l process killed by daemon config change; daemon failed to restart phc2sys within 30s\nFAIL: Expected phc2sys to be running after config change",
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
					{Name: "linuxptp-daemon", Reason: "SUT daemon with config change handling"},
				},
			},
			ExpectedInvest: &calibrate.ExpectedInvest{
				RCAMessage:       "Config change handler in linuxptp-daemon kills child processes (ptp4l, phc2sys) but fails to restart them. The daemon enters a stuck state after config change detection.",
				DefectType:       "pb001",
				Component:        "linuxptp-daemon",
				ConvergenceScore: 0.85,
				EvidenceRefs:     []string{"linuxptp-daemon:pkg/daemon/process.go:config_change"},
			},
			ExpectedCorrelate: &calibrate.ExpectedCorrelate{
				IsDuplicate: false, Confidence: 0.3,
			},
			ExpectedReview: &calibrate.ExpectedReview{Decision: "approve"},
		},

		// C2: Config hang PANICKED — same symptom (S1), different failure type.
		// Recall hit (H1, confidence 0.95) → F5 → F6.
		{
			ID: "C2", Version: "4.18", Job: "[recovery]",
			TestName:     "PTP Recovery > ptp process restart > should recover ptp4l after killing it",
			ErrorMessage: "PANIC: ptp4l process recovery failed — daemon stuck in kill loop [recovered]\npanic.go:115",
			LogSnippet:   "2026-01-15T10:30:00Z daemon: PTP config change detected\n2026-01-15T10:30:01Z PANIC: ptp4l process recovery failed — daemon stuck in kill loop\npanic.go:115\n[recovered]",
			SymptomID: "S1", RCAID: "R2",
			ExpectedPath:    []string{"F0", "F5", "F6"},
			ExpectRecallHit: true,
			ExpectedLoops:   0,
			ExpectedRecall: &calibrate.ExpectedRecall{
				Match: true, Confidence: 0.95,
			},
			ExpectedReview: &calibrate.ExpectedReview{Decision: "approve"},
		},

		// C3: Config hang AfterEach — cleanup failure after test.
		// Recall hit (same symptom S1) → F5 → F6.
		{
			ID: "C3", Version: "4.18", Job: "[events]",
			TestName:     "PTP Recovery > HTTP events > validates system after removing consumer",
			ErrorMessage: "[AfterEach] ptp_events_and_metrics.go:175: daemon failed to restart ptp4l after consumer removal",
			LogSnippet:   "2026-01-15T11:00:00Z [AfterEach] cleanup: removing event consumer\n2026-01-15T11:00:01Z daemon: config change triggered by consumer removal\n2026-01-15T11:00:31Z FAIL [AfterEach]: daemon failed to restart ptp4l after consumer removal",
			SymptomID: "S1", RCAID: "R2",
			ExpectedPath:    []string{"F0", "F5", "F6"},
			ExpectRecallHit: true,
			ExpectedLoops:   0,
			ExpectedRecall: &calibrate.ExpectedRecall{
				Match: true, Confidence: 0.92,
			},
			ExpectedReview: &calibrate.ExpectedReview{Decision: "approve"},
		},

		// C4: Broken pipe — new symptom (S2), full investigation.
		// Triage sees 2 candidate repos → F2 → F3 → F4 (not dup) → F5 → F6.
		{
			ID: "C4", Version: "4.18", Job: "[recovery]",
			TestName:     "PTP Recovery > ptp node reboot > validates PTP consumer events after reboot",
			ErrorMessage: "events.sock: broken pipe writing PTP event after node reboot; consumer events lost",
			LogSnippet:   "2026-01-15T12:00:00Z daemon: node reboot detected, reinitializing PTP\n2026-01-15T12:00:05Z write /cloud-native/events.sock: broken pipe (EPIPE)\n2026-01-15T12:00:06Z cloud-event-proxy: consumer events lost after pipe break\nFAIL: Expected PTP consumer events to resume after reboot",
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
					{Name: "linuxptp-daemon", Reason: "Daemon writes to events.sock"},
					{Name: "cloud-event-proxy", Reason: "Proxy reads from events.sock"},
				},
			},
			ExpectedInvest: &calibrate.ExpectedInvest{
				RCAMessage:       "Broken pipe on Unix socket /cloud-native/events.sock. The linuxptp-daemon writes PTP events to cloud-event-proxy via this socket. Under burst (e.g. after node reboot), the pipe buffer clogs causing EPIPE. Events are lost or corrupted at the receiver.",
				DefectType:       "pb001",
				Component:        "linuxptp-daemon",
				ConvergenceScore: 0.80,
				EvidenceRefs:     []string{"linuxptp-daemon:pkg/event/socket.go:write_event", "cloud-event-proxy:pkg/receiver/handler.go:pipe_read"},
			},
			ExpectedCorrelate: &calibrate.ExpectedCorrelate{
				IsDuplicate: false, Confidence: 0.35,
			},
			ExpectedReview: &calibrate.ExpectedReview{Decision: "approve"},
		},

		// C5: Broken pipe — same symptom (S2), interface down trigger.
		// Recall hit → F5 → F6.
		{
			ID: "C5", Version: "4.18", Job: "[events]",
			TestName:     "PTP Events > interface down > should generate events when slave interface goes down",
			ErrorMessage: "events.sock: broken pipe writing PTP event after interface down; consumer events lost",
			LogSnippet:   "2026-01-15T13:00:00Z PTP: slave interface ens3f0 went down\n2026-01-15T13:00:01Z write /cloud-native/events.sock: broken pipe (EPIPE)\nFAIL: Expected PTP events after interface down/up cycle",
			SymptomID: "S2", RCAID: "R1",
			ExpectedPath:    []string{"F0", "F5", "F6"},
			ExpectRecallHit: true,
			ExpectedLoops:   0,
			ExpectedRecall: &calibrate.ExpectedRecall{
				Match: true, Confidence: 0.93,
			},
			ExpectedReview: &calibrate.ExpectedReview{Decision: "approve"},
		},

		// C6: Cascade — BeforeEach fails for 2-port OC test setup.
		// New symptom (S3), cascade from broken pipe (R1).
		// Triage single candidate (H7) → F3 → F4 (dup links to R1, H15) → Done.
		{
			ID: "C6", Version: "4.18", Job: "[events]",
			TestName:     "PTP Events > interface down OC 2 port > verifies 2-port oc ha failover",
			ErrorMessage: "[BeforeEach] ptp_interfaces.go:498: failed to setup 2-port OC profile: events.sock: broken pipe during initialization",
			LogSnippet:   "2026-01-15T14:00:00Z [BeforeEach] ptp_interfaces.go:498: setting up 2-port OC profile\n2026-01-15T14:00:01Z events.sock: broken pipe during OC initialization\nFAIL [BeforeEach]: failed to setup 2-port OC profile",
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
				RCAMessage:       "Cascade from broken pipe. BeforeEach at ptp_interfaces.go:498 fails because the events.sock pipe is broken during OC profile initialization. Same root cause as the node reboot pipe failure.",
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

		// C7: Same BeforeEach cascade as C6 — recall hit → F5 → F6.
		{
			ID: "C7", Version: "4.18", Job: "[events]",
			TestName:     "PTP Events > interface down OC 2 port > verifies 2-port oc ha holdover and freerun",
			ErrorMessage: "[BeforeEach] ptp_interfaces.go:498: failed to setup 2-port OC profile: events.sock: broken pipe during initialization",
			LogSnippet:   "2026-01-15T14:00:02Z [BeforeEach] ptp_interfaces.go:498: failed to setup 2-port OC profile\nFAIL [BeforeEach]: same BeforeEach cascade",
			SymptomID: "S3", RCAID: "R1",
			ExpectedPath:    []string{"F0", "F5", "F6"},
			ExpectRecallHit: true,
			ExpectedLoops:   0,
			ExpectedRecall: &calibrate.ExpectedRecall{
				Match: true, Confidence: 0.92,
			},
			ExpectedReview: &calibrate.ExpectedReview{Decision: "approve"},
		},

		// C8: Same BeforeEach cascade as C6 — recall hit → F5 → F6.
		{
			ID: "C8", Version: "4.18", Job: "[events]",
			TestName:     "PTP Events > interface down OC 2 port > verifies 2-port oc ha passive interface recovery",
			ErrorMessage: "[BeforeEach] ptp_interfaces.go:498: failed to setup 2-port OC profile: events.sock: broken pipe during initialization",
			LogSnippet:   "2026-01-15T14:00:03Z [BeforeEach] ptp_interfaces.go:498: failed to setup 2-port OC profile\nFAIL [BeforeEach]: same BeforeEach cascade",
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

func daemonMockWorkspace() calibrate.WorkspaceConfig {
	return calibrate.WorkspaceConfig{
		Repos: []calibrate.RepoConfig{
			{
				Name:           "linuxptp-daemon",
				Purpose:        "SUT: PTP daemon running on nodes — ptp4l, phc2sys, ts2phc processes; event socket communication with cloud-event-proxy",
				Branch:         "release-4.18",
				RelevantToRCAs: []string{"R1", "R2"},
			},
			{
				Name:           "cloud-event-proxy",
				Purpose:        "Cloud Event Proxy: receives PTP events from daemon via Unix socket (/cloud-native/events.sock); publishes cloud events",
				Branch:         "release-4.18",
				RelevantToRCAs: []string{"R1"},
			},
			{
				Name:           "cnf-gotests",
				Purpose:        "PTP test cases (Ginkgo): test code, assertions, test helpers. Error stacks point here for cascade failures.",
				Branch:         "master",
				RelevantToRCAs: []string{"R1"},
			},
			{
				Name:           "ptp-operator",
				Purpose:        "PTP operator lifecycle: manages linuxptp-daemon DaemonSet, PtpConfig CRD, PTP profiles",
				Branch:         "release-4.18",
				RelevantToRCAs: []string{},
			},
			{
				Name:         "eco-gotests",
				Purpose:      "Ecosystem QE test framework; shared helpers used by other test suites (NOT PTP-specific)",
				Branch:       "master",
				IsRedHerring: true,
			},
		},
	}
}
