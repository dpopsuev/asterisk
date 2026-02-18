package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"time"

	"asterisk/internal/preinvest"
	"asterisk/internal/rp"
	"asterisk/internal/store"
)

func checkTokenFile(path string) error {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return fmt.Errorf("RP API token file not found: %s\n\n"+
			"To get your RP API token:\n"+
			"  1. Log in to your ReportPortal instance\n"+
			"  2. Go to User Profile (top-right icon)\n"+
			"  3. Copy the API token (UUID format)\n"+
			"  4. Save it:  echo '<YOUR_TOKEN>' > .rp-api-key && chmod 600 .rp-api-key\n", path)
	}
	if err != nil {
		return fmt.Errorf("check token file: %w", err)
	}
	if perm := info.Mode().Perm(); perm&0044 != 0 {
		fmt.Fprintf(os.Stderr, "WARNING: %s is readable by group/others (mode %04o). Run: chmod 600 %s\n", path, perm, path)
	}
	return nil
}

func defaultWorkspaceRepos() []string {
	return []string{
		"ptp-operator",
		"linuxptp-daemon",
		"linuxptp-daemon-v2",
		"cloud-event-proxy",
		"ptp-operator-must-gather",
		"cluster-etcd-operator",
	}
}

// resolveRPProject returns the RP project name from the given flag value,
// falling back to $ASTERISK_RP_PROJECT. Returns "" if neither is set.
func resolveRPProject(flagValue string) string {
	if flagValue != "" {
		return flagValue
	}
	return os.Getenv("ASTERISK_RP_PROJECT")
}

// loadEnvelopeForAnalyze resolves the envelope from a file path or launch ID.
func loadEnvelopeForAnalyze(launch, dbPath, rpBase, rpKeyPath, rpProject string) *preinvest.Envelope {
	if _, err := os.Stat(launch); err == nil {
		data, err := os.ReadFile(launch)
		if err != nil {
			return nil
		}
		var env preinvest.Envelope
		if err := json.Unmarshal(data, &env); err != nil {
			return nil
		}
		return &env
	}

	launchID, err := strconv.Atoi(launch)
	if err != nil || launchID <= 0 {
		return nil
	}
	st, err := store.Open(dbPath)
	if err != nil {
		return nil
	}
	defer st.Close()

	env, _ := st.GetEnvelope(launchID)
	if env == nil && rpBase != "" {
		key, _ := rp.ReadAPIKey(rpKeyPath)
		client, err := rp.New(rpBase, key, rp.WithTimeout(30*time.Second))
		if err != nil {
			return nil
		}
		fetcher := rp.NewFetcher(client, rpProject)
		adapter := &store.PreinvestStoreAdapter{Store: st}
		if err := preinvest.FetchAndSave(fetcher, adapter, launchID); err != nil {
			return nil
		}
		env, _ = adapter.Get(launchID)
	}
	return env
}

// loadEnvelopeForCursor resolves the envelope from a file path or launch ID for cursor mode.
func loadEnvelopeForCursor(launch string, dbPath string) (*preinvest.Envelope, int) {
	if _, err := os.Stat(launch); err == nil {
		data, err := os.ReadFile(launch)
		if err != nil {
			return nil, 0
		}
		var env preinvest.Envelope
		if err := json.Unmarshal(data, &env); err != nil {
			return nil, 0
		}
		launchID, _ := strconv.Atoi(env.RunID)
		if launchID == 0 {
			launchID = 1
		}
		return &env, launchID
	}
	launchID, err := strconv.Atoi(launch)
	if err != nil || launchID <= 0 {
		return nil, 0
	}
	st, err := store.Open(dbPath)
	if err != nil {
		return nil, 0
	}
	defer st.Close()
	adapter := &store.PreinvestStoreAdapter{Store: st}
	env, _ := adapter.Get(launchID)
	if env == nil {
		return nil, 0
	}
	return env, launchID
}

// createAnalysisScaffolding creates v2 store entities for all failures in the envelope.
func createAnalysisScaffolding(st store.Store, env *preinvest.Envelope) (int64, []*store.Case) {
	rpLaunchID, _ := strconv.Atoi(env.RunID)

	suiteID, _ := st.CreateSuite(&store.InvestigationSuite{
		Name:        fmt.Sprintf("Analysis %s", env.Name),
		Description: fmt.Sprintf("Automated analysis for launch %s", env.RunID),
		Status:      "active",
	})

	vID, _ := st.CreateVersion(&store.Version{Label: "unknown"})
	if vID == 0 {
		v, _ := st.GetVersionByLabel("unknown")
		if v != nil {
			vID = v.ID
		}
	}

	pID, _ := st.CreatePipeline(&store.Pipeline{
		SuiteID:    suiteID,
		VersionID:  vID,
		Name:       env.Name,
		RPLaunchID: rpLaunchID,
		Status:     "complete",
	})

	lID, _ := st.CreateLaunch(&store.Launch{
		PipelineID: pID,
		RPLaunchID: rpLaunchID,
		Name:       env.Name,
		Status:     "complete",
	})

	jID, _ := st.CreateJob(&store.Job{
		LaunchID: lID,
		Name:     env.Name,
		Status:   "complete",
	})

	var cases []*store.Case
	for _, f := range env.FailureList {
		caseID, _ := st.CreateCaseV2(&store.Case{
			JobID:    jID,
			LaunchID: lID,
			RPItemID: f.ID,
			Name:     f.Name,
			Status:   "open",
		})
		c, _ := st.GetCaseV2(caseID)
		if c != nil {
			cases = append(cases, c)
		}
	}

	return suiteID, cases
}

// ensureCaseInStore finds or creates the full v2 scaffolding for a failure item.
func ensureCaseInStore(st store.Store, env *preinvest.Envelope, rpLaunchID int, item preinvest.FailureItem) *store.Case {
	suites, _ := st.ListSuites()
	for _, suite := range suites {
		if suite.Status != "open" {
			continue
		}
		pipelines, _ := st.ListPipelinesBySuite(suite.ID)
		for _, p := range pipelines {
			launches, _ := st.ListLaunchesByPipeline(p.ID)
			for _, l := range launches {
				jobs, _ := st.ListJobsByLaunch(l.ID)
				for _, j := range jobs {
					cases, _ := st.ListCasesByJob(j.ID)
					for _, c := range cases {
						if c.RPItemID == item.ID {
							return c
						}
					}
				}
			}
		}
	}

	suiteID, _ := st.CreateSuite(&store.InvestigationSuite{
		Name:        fmt.Sprintf("Investigation %s", env.Name),
		Description: fmt.Sprintf("Auto-created for launch %s", env.RunID),
	})

	vID, _ := st.CreateVersion(&store.Version{Label: "unknown"})
	if vID == 0 {
		v, _ := st.GetVersionByLabel("unknown")
		if v != nil {
			vID = v.ID
		}
	}

	pID, _ := st.CreatePipeline(&store.Pipeline{
		SuiteID:    suiteID,
		VersionID:  vID,
		Name:       env.Name,
		RPLaunchID: rpLaunchID,
	})

	lID, _ := st.CreateLaunch(&store.Launch{
		PipelineID: pID,
		RPLaunchID: rpLaunchID,
		Name:       env.Name,
	})

	jID, _ := st.CreateJob(&store.Job{
		LaunchID: lID,
		RPItemID: item.ID,
		Name:     item.Name,
	})

	caseID, _ := st.CreateCaseV2(&store.Case{
		JobID:    jID,
		LaunchID: lID,
		RPItemID: item.ID,
		Name:     item.Name,
		Status:   "open",
	})

	caseData, _ := st.GetCaseV2(caseID)
	if caseData == nil {
		fmt.Fprintf(os.Stderr, "failed to create case in store\n")
		os.Exit(1)
	}
	return caseData
}
