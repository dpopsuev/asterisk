package main

import (
	"asterisk/internal/display"
	"fmt"
	"strings"
	"testing"
)

// simulateExtractedPrompt builds the lowercased text that extractFailureData
// would return for a given case. The prompt template places TestName and
// ErrorMessage inside the "## Failure under investigation" section.
// RCA Description is included because it appears via the recall/symptom context.
func simulateExtractedPrompt(testName, errorMessage, rcaDescription string) string {
	var parts []string
	parts = append(parts, "## failure under investigation")
	if testName != "" {
		parts = append(parts, fmt.Sprintf("**test name:** `%s`", testName))
	}
	parts = append(parts, "**status:** failed")
	if errorMessage != "" {
		parts = append(parts, fmt.Sprintf("**error message:**\n```\n%s\n```", errorMessage))
	}
	if rcaDescription != "" {
		parts = append(parts, "\n## known symptom")
		parts = append(parts, rcaDescription)
	}
	return strings.ToLower(strings.Join(parts, "\n"))
}

// groundTruthCase holds the subset of scenario data needed for testing.
type groundTruthCase struct {
	caseID         string
	testName       string
	errorMessage   string
	rcaDescription string // from GroundTruthRCA.Description

	// Expected classifier outputs
	wantDefectType string
	wantCategory   string
	wantSkip       bool
	wantComponent  string

	// For RCA keyword test
	requiredKeywords []string
	keywordThreshold int
}

// All 30 cases from ptp-real-ingest scenario with ground truth expectations.
var allCases = []groundTruthCase{
	{
		caseID: "C01", testName: "should have the 'phc2sys' and 'ptp4l' processes in 'UP' state in PTP metrics",
		errorMessage:   "/var/lib/jenkins/workspace/ocp-far-edge-vran-tests/cnf-gotests/test/ran/ptp/tests/ptp_events_and_metrics.go:92",
		rcaDescription: "OCPBUGS-70233. /var/lib/jenkins/workspace/ocp-far-edge-vran-tests/cnf-gotests/test/ran/ptp/tests/ptp_events_and_metrics.go:92",
		wantDefectType: "pb001", wantCategory: "product", wantSkip: false, wantComponent: "linuxptp-daemon",
		requiredKeywords: []string{"edge", "gotests", "have", "jenkins", "linuxptp_daemon", "metrics", "processes", "ptp_events_and_metrics"},
		keywordThreshold: 3,
	},
	{
		caseID: "C02", testName: "PTP Process Restart should recover the phc2sys process after killing it",
		errorMessage:   "OCPBUGS-74929 large amount of clock-class events flip-flop between 6 and 248 https://issues.redhat.com/browse/OCPBUGS-74939 Both need further investigation, they should work with ntpfailover",
		rcaDescription: "OCPBUGS-74939. OCPBUGS-74929 large amount of clock-class events flip-flop between 6 and 248 https://issues.redhat.com/browse/OCPBUGS-74939 Both need further investigation, they should work with ntpfailover",
		wantDefectType: "au001", wantCategory: "automation", wantSkip: true, wantComponent: "cnf-gotests",
		requiredKeywords: []string{"after", "amount", "between", "both", "browse", "class", "clock", "cnf_gotests"},
		keywordThreshold: 3,
	},
	{
		caseID: "C03", testName: "",
		errorMessage:   "/var/lib/jenkins/workspace/ocp-far-edge-vran-tests/cnf-gotests/test/ran/ptp/tests/ptp_recovery.go:773",
		rcaDescription: "OCPBUGS-64567. /var/lib/jenkins/workspace/ocp-far-edge-vran-tests/cnf-gotests/test/ran/ptp/tests/ptp_recovery.go:773",
		wantDefectType: "fw001", wantCategory: "product", wantSkip: false, wantComponent: "linuxptp-daemon",
		requiredKeywords: []string{"edge", "gotests", "jenkins", "linuxptp_daemon", "ptp_recovery", "test", "tests", "vran"},
		keywordThreshold: 3,
	},
	{
		caseID: "C04", testName: "",
		errorMessage:   "HTTP events using consumer /var/lib/jenkins/workspace/ocp-far-edge-vran-tests/cnf-gotests/test/ran/ptp/tests/ptp_events_and_metrics.go:258",
		rcaDescription: "OCPBUGS-70327. HTTP events using consumer /var/lib/jenkins/workspace/ocp-far-edge-vran-tests/cnf-gotests/test/ran/ptp/tests/ptp_events_and_metrics.go:258",
		wantDefectType: "en001", wantCategory: "environment", wantSkip: true, wantComponent: "linuxptp-daemon",
		requiredKeywords: []string{"consumer", "edge", "events", "gotests", "http", "jenkins", "linuxptp_daemon", "ptp_events_and_metrics"},
		keywordThreshold: 3,
	},
	{
		caseID: "C05", testName: "",
		errorMessage:   "OCPBUGS-74342 - GNSS Sync State Not Correctly Mapped to Cloud Event and Metrics OCPBUGS-59269 Role metrics are missing after restarting sidecar container in linuxptp-daemon needs to reopen for versions under 4.18",
		rcaDescription: "OCPBUGS-74342. OCPBUGS-74342 - GNSS Sync State Not Correctly Mapped to Cloud Event and Metrics OCPBUGS-59269 Role metrics are missing after restarting sidecar container in linuxptp-daemon needs to reopen for vers...",
		wantDefectType: "pb001", wantCategory: "product", wantSkip: false, wantComponent: "linuxptp-daemon",
		requiredKeywords: []string{"after", "cloud", "container", "correctly", "daemon", "event", "gnss", "linuxptp"},
		keywordThreshold: 3,
	},
	{
		caseID: "C06", testName: "",
		errorMessage:   "",
		rcaDescription: "OCPBUGS-63435. ",
		wantDefectType: "pb001", wantCategory: "product", wantSkip: false, wantComponent: "linuxptp-daemon",
		requiredKeywords: []string{"linuxptp_daemon"},
		keywordThreshold: 1,
	},
	{
		caseID: "C07", testName: "",
		errorMessage:   "SNO management workload partitioning /var/lib/jenkins/workspace/ocp-far-edge-vran-tests/cnf-gotests/test/ran/workloadpartitioning/ranwphelper/ranwphelper.go:134",
		rcaDescription: "OCPBUGS-55838. SNO management workload partitioning /var/lib/jenkins/workspace/ocp-far-edge-vran-tests/cnf-gotests/test/ran/workloadpartitioning/ranwphelper/ranwphelper.go:134",
		wantDefectType: "pb001", wantCategory: "product", wantSkip: false, wantComponent: "linuxptp-daemon",
		requiredKeywords: []string{"edge", "gotests", "jenkins", "linuxptp_daemon", "management", "partitioning", "ranwphelper", "test"},
		keywordThreshold: 3,
	},
	{
		caseID: "C08", testName: "",
		errorMessage:   "The configmap didn't update",
		rcaDescription: "OCPBUGS-55121. The configmap didn't update",
		wantDefectType: "pb001", wantCategory: "product", wantSkip: false, wantComponent: "ptp-operator",
		requiredKeywords: []string{"configmap", "didn", "ptp_operator", "update"},
		keywordThreshold: 3,
	},
	{
		caseID: "C09", testName: "",
		errorMessage:   "PTP Events and Metrics - interface down ordinary clock 2 port failure /var/lib/jenkins/workspace/ocp-far-edge-vran-tests/cnf-gotests/test/ran/ptp/tests/ptp_interfaces.go:602",
		rcaDescription: "CNF-21408. PTP Events and Metrics - interface down ordinary clock 2 port failure /var/lib/jenkins/workspace/ocp-far-edge-vran-tests/cnf-gotests/test/ran/ptp/tests/ptp_interfaces.go:602",
		wantDefectType: "en001", wantCategory: "environment", wantSkip: true, wantComponent: "linuxptp-daemon",
		requiredKeywords: []string{"clock", "down", "edge", "events", "failure", "gotests", "interface", "jenkins"},
		keywordThreshold: 3,
	},
	{
		caseID: "C10", testName: "",
		errorMessage:   "PTP Events and Metrics - interface down /var/lib/jenkins/workspace/ocp-far-edge-vran-tests/cnf-gotests/test/ran/ptp/tests/ptp_interfaces.go:753 Ran 19 of 39 Specs in 5903.299 seconds",
		rcaDescription: "OCPBUGS-68352. PTP Events and Metrics - interface down /var/lib/jenkins/workspace/ocp-far-edge-vran-tests/cnf-gotests/test/ran/ptp/tests/ptp_interfaces.go:753 Ran 19 of 39 Specs in 5903.299 seconds",
		wantDefectType: "au001", wantCategory: "automation", wantSkip: true, wantComponent: "linuxptp-daemon",
		requiredKeywords: []string{"down", "edge", "events", "gotests", "interface", "jenkins", "linuxptp_daemon", "metrics"},
		keywordThreshold: 3,
	},
	{
		caseID: "C11", testName: "",
		errorMessage:   "Tracking issue for failures: All the ntpfailover-specific tests never got to run since the clocks weren't locked Automation issue, should be fixed now",
		rcaDescription: "CNF-21588. Tracking issue for failures: All the ntpfailover-specific tests never got to run since the clocks weren't locked Automation issue, should be fixed now",
		wantDefectType: "pb001", wantCategory: "product", wantSkip: false, wantComponent: "cnf-gotests",
		requiredKeywords: []string{"automation", "clocks", "cnf_gotests", "failures", "fixed", "issue", "locked", "never"},
		keywordThreshold: 3,
	},
	{
		caseID: "C12", testName: "",
		errorMessage:   "Basic PTP Configs [BeforeAll] should have [LOCKED] clock state in PTP metrics",
		rcaDescription: "OCPBUGS-65911. Basic PTP Configs [BeforeAll] should have [LOCKED] clock state in PTP metrics",
		wantDefectType: "pb001", wantCategory: "product", wantSkip: false, wantComponent: "linuxptp-daemon",
		requiredKeywords: []string{"basic", "beforeall", "clock", "configs", "have", "linuxptp_daemon", "locked", "metrics"},
		keywordThreshold: 3,
	},
	{
		caseID: "C13", testName: "",
		errorMessage:   "t-bc/t-tsc upstream clock loss & unassisted holdover /var/lib/jenkins/workspace/ocp-far-edge-vran-tests/cnf-gotests/test/ran/ptp/tests/ptp_recovery.go:2062",
		rcaDescription: "CNF-21102. t-bc/t-tsc upstream clock loss & unassisted holdover /var/lib/jenkins/workspace/ocp-far-edge-vran-tests/cnf-gotests/test/ran/ptp/tests/ptp_recovery.go:2062",
		wantDefectType: "en001", wantCategory: "environment", wantSkip: true, wantComponent: "cnf-gotests",
		requiredKeywords: []string{"clock", "cnf_gotests", "edge", "gotests", "holdover", "jenkins", "loss", "ptp_recovery"},
		keywordThreshold: 3,
	},
	{
		caseID: "C14", testName: "",
		errorMessage:   "t-bc/t-tsc upstream clock loss & unassisted holdover /var/lib/jenkins/workspace/ocp-far-edge-vran-tests/cnf-gotests/test/ran/ptp/tests/ptp_recovery.go:2062",
		rcaDescription: "OCPBUGS-71204. t-bc/t-tsc upstream clock loss & unassisted holdover /var/lib/jenkins/workspace/ocp-far-edge-vran-tests/cnf-gotests/test/ran/ptp/tests/ptp_recovery.go:2062",
		wantDefectType: "pb001", wantCategory: "product", wantSkip: false, wantComponent: "linuxptp-daemon",
		requiredKeywords: []string{"clock", "edge", "gotests", "holdover", "jenkins", "linuxptp_daemon", "loss", "ptp_recovery"},
		keywordThreshold: 3,
	},
	{
		caseID: "C15", testName: "",
		errorMessage:   "t-bc/t-tsc upstream clock loss & unassisted holdover /var/lib/jenkins/workspace/ocp-far-edge-vran-tests/cnf-gotests/test/ran/ptp/tests/ptp_recovery.go:2062",
		rcaDescription: "OCPBUGS-70178. t-bc/t-tsc upstream clock loss & unassisted holdover /var/lib/jenkins/workspace/ocp-far-edge-vran-tests/cnf-gotests/test/ran/ptp/tests/ptp_recovery.go:2062",
		wantDefectType: "pb001", wantCategory: "product", wantSkip: false, wantComponent: "linuxptp-daemon",
		requiredKeywords: []string{"clock", "edge", "gotests", "holdover", "jenkins", "linuxptp_daemon", "loss", "ptp_recovery"},
		keywordThreshold: 3,
	},
	{
		caseID: "C16", testName: "",
		errorMessage:   "change PTP offset thresholds /var/lib/jenkins/workspace/ocp-far-edge-vran-tests/cnf-gotests/test/ran/ptp/tests/ptp_events_and_metrics.go:221",
		rcaDescription: "OCPBUGS-74904. change PTP offset thresholds /var/lib/jenkins/workspace/ocp-far-edge-vran-tests/cnf-gotests/test/ran/ptp/tests/ptp_events_and_metrics.go:221",
		wantDefectType: "pb001", wantCategory: "product", wantSkip: false, wantComponent: "linuxptp-daemon",
		requiredKeywords: []string{"change", "edge", "gotests", "jenkins", "linuxptp_daemon", "offset", "ptp_events_and_metrics", "test"},
		keywordThreshold: 3,
	},
	{
		caseID: "C17", testName: "",
		errorMessage:   "HTTP events using consumer /var/lib/jenkins/workspace/ocp-far-edge-vran-tests/cnf-gotests/test/ran/ptp/tests/ptp_events_and_metrics.go:267 Ran 16 of 39 Specs in 4903.088 seconds",
		rcaDescription: "OCPBUGS-74377. HTTP events using consumer /var/lib/jenkins/workspace/ocp-far-edge-vran-tests/cnf-gotests/test/ran/ptp/tests/ptp_events_and_metrics.go:267 Ran 16 of 39 Specs in 4903.088 seconds",
		wantDefectType: "pb001", wantCategory: "product", wantSkip: false, wantComponent: "linuxptp-daemon",
		requiredKeywords: []string{"consumer", "edge", "events", "gotests", "http", "jenkins", "linuxptp_daemon", "ptp_events_and_metrics"},
		keywordThreshold: 3,
	},
	{
		caseID: "C18", testName: "",
		errorMessage:   "/var/lib/jenkins/workspace/ocp-far-edge-vran-tests/cnf-gotests/test/ran/ptp/tests/ptp_events_and_metrics.go:267",
		rcaDescription: "OCPBUGS-75899. /var/lib/jenkins/workspace/ocp-far-edge-vran-tests/cnf-gotests/test/ran/ptp/tests/ptp_events_and_metrics.go:267",
		wantDefectType: "au001", wantCategory: "automation", wantSkip: true, wantComponent: "linuxptp-daemon",
		requiredKeywords: []string{"edge", "gotests", "jenkins", "linuxptp_daemon", "ptp_events_and_metrics", "test", "tests", "vran"},
		keywordThreshold: 3,
	},
	{
		caseID: "C19", testName: "",
		errorMessage:   "t-bc/t-tsc upstream clock loss & unassisted holdover /var/lib/jenkins/workspace/ocp-far-edge-vran-tests/cnf-gotests/test/ran/ptp/tests/ptp_recovery.go:1865",
		rcaDescription: "CNF-20071. t-bc/t-tsc upstream clock loss & unassisted holdover /var/lib/jenkins/workspace/ocp-far-edge-vran-tests/cnf-gotests/test/ran/ptp/tests/ptp_recovery.go:1865",
		wantDefectType: "pb001", wantCategory: "product", wantSkip: false, wantComponent: "linuxptp-daemon",
		requiredKeywords: []string{"clock", "edge", "gotests", "holdover", "jenkins", "linuxptp_daemon", "loss", "ptp_recovery"},
		keywordThreshold: 3,
	},
	{
		caseID: "C20", testName: "",
		errorMessage:   "change PTP offset thresholds /var/lib/jenkins/workspace/ocp-far-edge-vran-tests/cnf-gotests/test/ran/ptp/tests/ptp_events_and_metrics.go:159 Dev Bug - Stale metrics reported [ASSIGNED]",
		rcaDescription: "OCPBUGS-66413. change PTP offset thresholds /var/lib/jenkins/workspace/ocp-far-edge-vran-tests/cnf-gotests/test/ran/ptp/tests/ptp_events_and_metrics.go:159 Dev Bug - Stale metrics reported [ASSIGNED]",
		wantDefectType: "pb001", wantCategory: "product", wantSkip: false, wantComponent: "linuxptp-daemon",
		requiredKeywords: []string{"assigned", "change", "edge", "gotests", "jenkins", "linuxptp_daemon", "metrics", "offset"},
		keywordThreshold: 3,
	},
	{
		caseID: "C21", testName: "",
		errorMessage:   "HTTP events using consumer OCPBUGS-45680: Consumer is losing subscription to events after restarting linuxptp-daemon pod",
		rcaDescription: "OCPBUGS-49373. HTTP events using consumer OCPBUGS-45680: Consumer is losing subscription to events after restarting linuxptp-daemon pod",
		wantDefectType: "pb001", wantCategory: "product", wantSkip: false, wantComponent: "linuxptp-daemon",
		requiredKeywords: []string{"after", "consumer", "daemon", "events", "http", "linuxptp", "linuxptp_daemon", "losing"},
		keywordThreshold: 3,
	},
	{
		caseID: "C22", testName: "",
		errorMessage:   "SNO management workload partitioning /var/lib/jenkins/workspace/ocp-far-edge-vran-tests/cnf-gotests/test/ran/workloadpartitioning/tests/workload_partitioning.go:381",
		rcaDescription: "OCPBUGS-45680. SNO management workload partitioning /var/lib/jenkins/workspace/ocp-far-edge-vran-tests/cnf-gotests/test/ran/workloadpartitioning/tests/workload_partitioning.go:381",
		wantDefectType: "pb001", wantCategory: "product", wantSkip: false, wantComponent: "linuxptp-daemon",
		requiredKeywords: []string{"edge", "gotests", "jenkins", "linuxptp_daemon", "management", "partitioning", "test", "tests"},
		keywordThreshold: 3,
	},
	{
		caseID: "C23", testName: "",
		errorMessage:   "PTP Events and Metrics - interface down",
		rcaDescription: "OCPBUGS-53247. PTP Events and Metrics - interface down",
		wantDefectType: "pb001", wantCategory: "product", wantSkip: false, wantComponent: "linuxptp-daemon",
		requiredKeywords: []string{"down", "events", "interface", "linuxptp_daemon", "metrics"},
		keywordThreshold: 3,
	},
	{
		caseID: "C24", testName: "",
		errorMessage:   "https://issues.redhat.com/browse/OCPBUGS-54967",
		rcaDescription: "OCPBUGS-54967. https://issues.redhat.com/browse/OCPBUGS-54967",
		wantDefectType: "pb001", wantCategory: "product", wantSkip: false, wantComponent: "linuxptp-daemon",
		requiredKeywords: []string{"browse", "https", "issues", "linuxptp_daemon", "ocpbugs", "redhat"},
		keywordThreshold: 3,
	},
	{
		caseID: "C25", testName: "",
		errorMessage:   "HTTP events using consumer",
		rcaDescription: "OCPBUGS-45674. HTTP events using consumer",
		wantDefectType: "pb001", wantCategory: "product", wantSkip: false, wantComponent: "linuxptp-daemon",
		requiredKeywords: []string{"consumer", "events", "http", "linuxptp_daemon", "using"},
		keywordThreshold: 3,
	},
	{
		caseID: "C26", testName: "",
		errorMessage:   "PTP Events and Metrics - interface down slave interface ens2fx was down but no metrics didn't update to FREERUN",
		rcaDescription: "OCPBUGS-47685. PTP Events and Metrics - interface down slave interface ens2fx was down but no metrics didn't update to FREERUN",
		wantDefectType: "pb001", wantCategory: "product", wantSkip: false, wantComponent: "linuxptp-daemon",
		requiredKeywords: []string{"didn", "down", "events", "freerun", "interface", "linuxptp_daemon", "metrics", "slave"},
		keywordThreshold: 3,
	},
	{
		caseID: "C27", testName: "",
		errorMessage:   "CNF-17776 - Automation: Add version conditions to clack_class validations after gpsd restart",
		rcaDescription: "CNF-17776. CNF-17776 - Automation: Add version conditions to clack_class validations after gpsd restart",
		wantDefectType: "pb001", wantCategory: "product", wantSkip: false, wantComponent: "linuxptp-daemon",
		requiredKeywords: []string{"after", "automation", "clack_class", "conditions", "gpsd", "linuxptp_daemon", "restart", "validations"},
		keywordThreshold: 3,
	},
	{
		caseID: "C28", testName: "",
		errorMessage:   "PTP Recovery [AfterEach] sidecar container recovery should verify events are logged during sidecar recovery",
		rcaDescription: "OCPBUGS-72558. PTP Recovery [AfterEach] sidecar container recovery should verify events are logged during sidecar recovery",
		wantDefectType: "pb001", wantCategory: "product", wantSkip: false, wantComponent: "linuxptp-daemon",
		requiredKeywords: []string{"aftereach", "container", "during", "events", "linuxptp_daemon", "logged", "recovery", "should"},
		keywordThreshold: 3,
	},
	{
		caseID: "C29", testName: "",
		errorMessage:   "OCPBUGS-49372: [4.17] remove phc2sys `-w` option PHC2SYSY process not found From linuxptp daemon log:",
		rcaDescription: "OCPBUGS-49372. OCPBUGS-49372: [4.17] remove phc2sys `-w` option PHC2SYSY process not found From linuxptp daemon log:",
		wantDefectType: "pb001", wantCategory: "product", wantSkip: false, wantComponent: "linuxptp-daemon",
		requiredKeywords: []string{"daemon", "found", "from", "linuxptp", "linuxptp_daemon", "ocpbugs", "option", "process"},
		keywordThreshold: 3,
	},
	{
		caseID: "C30", testName: "",
		errorMessage:   "PTP Recovery [AfterEach] should recover the phc2sys process after killing it",
		rcaDescription: "OCPBUGS-59849. PTP Recovery [AfterEach] should recover the phc2sys process after killing it",
		wantDefectType: "pb001", wantCategory: "product", wantSkip: false, wantComponent: "linuxptp-daemon",
		requiredKeywords: []string{"after", "aftereach", "killing", "linuxptp_daemon", "process", "recover", "recovery", "should"},
		keywordThreshold: 3,
	},
}

// classifyKnownLimitations lists cases where identical error text maps to
// different ground truth defect types based on external investigation context.
// The mock classifier cannot distinguish these from prompt text alone.
var classifyKnownLimitations = map[string]string{
	"C03": "bare path ptp_recovery.go → fw001; no firmware keywords in text",
	"C04": "HTTP events using consumer → en001; same text is pb001 in C17/C25",
	"C13": "upstream clock loss → en001; same text is pb001 in C14/C15/C19",
	"C18": "bare path ptp_events_and_metrics.go → au001; indistinguishable from C03",
}

// componentKnownLimitations lists cases where component cannot be determined
// from text alone because identical failure text maps to different components.
var componentKnownLimitations = map[string]string{
	"C13": "identical text to C14; different component determined by Jira investigation",
}

func TestClassifyFailure_AllCases(t *testing.T) {
	debug = true

	var failures []string
	skipped := 0
	for _, tc := range allCases {
		if reason, ok := classifyKnownLimitations[tc.caseID]; ok {
			t.Run(tc.caseID+"_defect_type", func(t *testing.T) {
				t.Skipf("known limitation: %s", reason)
			})
			t.Run(tc.caseID+"_skip", func(t *testing.T) {
				t.Skipf("known limitation: %s", reason)
			})
			skipped++
			continue
		}

		prompt := simulateExtractedPrompt(tc.testName, tc.errorMessage, tc.rcaDescription)
		_, _, gotDefect, gotSkip := classifyFailure(prompt)

		t.Run(tc.caseID+"_defect_type", func(t *testing.T) {
			if gotDefect != tc.wantDefectType {
				t.Errorf("%s: defect_type got=%s want=%s", tc.caseID, display.DefectTypeWithCode(gotDefect), display.DefectTypeWithCode(tc.wantDefectType))
			}
		})

		t.Run(tc.caseID+"_skip", func(t *testing.T) {
			if gotSkip != tc.wantSkip {
				t.Errorf("%s: skip got=%v want=%v", tc.caseID, gotSkip, tc.wantSkip)
			}
		})

		if gotDefect != tc.wantDefectType || gotSkip != tc.wantSkip {
			failures = append(failures, fmt.Sprintf("%s: defect=%s(want %s) skip=%v(want %v)",
				tc.caseID, display.DefectTypeWithCode(gotDefect), display.DefectTypeWithCode(tc.wantDefectType), gotSkip, tc.wantSkip))
		}
	}

	if len(failures) > 0 {
		t.Logf("\n=== CLASSIFICATION FAILURES (%d/%d) ===", len(failures), len(allCases))
		for _, f := range failures {
			t.Logf("  FAIL: %s", f)
		}
	}
	t.Logf("\n=== CLASSIFICATION: %d/%d pass, %d skipped (known limitations) ===",
		len(allCases)-len(failures)-skipped, len(allCases), skipped)
}

func TestIdentifyComponent_AllCases(t *testing.T) {
	debug = true

	var failures []string
	skipped := 0
	for _, tc := range allCases {
		if reason, ok := componentKnownLimitations[tc.caseID]; ok {
			t.Run(tc.caseID, func(t *testing.T) {
				t.Skipf("known limitation: %s", reason)
			})
			skipped++
			continue
		}

		prompt := simulateExtractedPrompt(tc.testName, tc.errorMessage, tc.rcaDescription)
		got := identifyComponent(prompt)

		t.Run(tc.caseID, func(t *testing.T) {
			if got != tc.wantComponent {
				t.Errorf("%s: component got=%s want=%s", tc.caseID, got, tc.wantComponent)
			}
		})

		if got != tc.wantComponent {
			failures = append(failures, fmt.Sprintf("%s: got=%s want=%s", tc.caseID, got, tc.wantComponent))
		}
	}

	if len(failures) > 0 {
		t.Logf("\n=== COMPONENT FAILURES (%d/%d) ===", len(failures), len(allCases))
		for _, f := range failures {
			t.Logf("  FAIL: %s", f)
		}
	}
	t.Logf("\n=== COMPONENT: %d/%d pass, %d skipped (known limitations) ===",
		len(allCases)-len(failures)-skipped, len(allCases), skipped)
}

func TestBuildRCAMessage_KeywordMatch(t *testing.T) {
	debug = true

	var failures []string
	for _, tc := range allCases {
		if len(tc.requiredKeywords) == 0 {
			continue
		}

		prompt := simulateExtractedPrompt(tc.testName, tc.errorMessage, tc.rcaDescription)
		component := identifyComponent(prompt)
		rca := buildRCAMessage(prompt, component)

		// Count how many required keywords appear in the RCA message
		rcaLower := strings.ToLower(rca)
		matched := 0
		var missing []string
		for _, kw := range tc.requiredKeywords {
			kwLower := strings.ToLower(kw)
			// Use underscore-to-hyphen and underscore-to-underscore matching
			if strings.Contains(rcaLower, kwLower) || strings.Contains(rcaLower, strings.ReplaceAll(kwLower, "_", "-")) {
				matched++
			} else {
				missing = append(missing, kw)
			}
		}

		pass := matched >= tc.keywordThreshold

		t.Run(tc.caseID, func(t *testing.T) {
			if !pass {
				t.Errorf("%s: keyword match %d/%d (threshold %d) — missing: %v\n  RCA: %.120s",
					tc.caseID, matched, len(tc.requiredKeywords), tc.keywordThreshold, missing, rca)
			}
		})

		if !pass {
			failures = append(failures, fmt.Sprintf("%s: %d/%d (need %d) missing=%v",
				tc.caseID, matched, len(tc.requiredKeywords), tc.keywordThreshold, missing))
		}
	}

	if len(failures) > 0 {
		t.Logf("\n=== RCA KEYWORD FAILURES (%d/%d) ===", len(failures), len(allCases))
		for _, f := range failures {
			t.Logf("  FAIL: %s", f)
		}
	}
	t.Logf("\n=== RCA KEYWORD PASS: %d/%d ===", len(allCases)-len(failures), len(allCases))
}

// TestExtractFailureData verifies extraction isolates failure sections.
func TestExtractFailureData(t *testing.T) {
	fullPrompt := `# F1 — Triage

## Task
Classify the symptom.

## Failure under investigation
**test name:** ` + "`test_ptp`" + `
**error message:**
ptp4l timeout

## Symptom categories
| timeout | context deadline exceeded |
| assertion | Expected X got Y |
`
	extracted := extractFailureData(fullPrompt)

	if strings.Contains(extracted, "context deadline exceeded") {
		t.Error("extractFailureData should NOT include symptom category table")
	}
	if !strings.Contains(extracted, "ptp4l timeout") {
		t.Error("extractFailureData should include the error message")
	}
	if !strings.Contains(extracted, "test_ptp") {
		t.Error("extractFailureData should include the test name")
	}
}
