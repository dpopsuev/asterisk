// asterisk is the main CLI: analyze, push -f, cursor (next prompt), save -f (ingest artifact).
// Usage: asterisk analyze --launch=<path|id> [--workspace=<path>] -o <artifact>
//        asterisk push -f <artifact-path>
//        asterisk cursor --launch=<path|id> [--workspace=<path>] [--case-id=<id>] [-o <prompt-file>]
//        asterisk save -f <artifact-path>
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strconv"
	"text/template"

	"asterisk/internal/investigate"
	"asterisk/internal/postinvest"
	"asterisk/internal/preinvest"
	"asterisk/internal/rpfetch"
	"asterisk/internal/rppush"
	"asterisk/internal/store"
	"asterisk/internal/workspace"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}
	sub := os.Args[1]
	args := os.Args[2:]
	switch sub {
	case "analyze":
		runAnalyze(args)
	case "push":
		runPush(args)
	case "cursor":
		runCursor(args)
	case "save":
		runSave(args)
	default:
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Fprintf(os.Stderr, "usage: asterisk <analyze|push|cursor|save> [options]\n")
	fmt.Fprintf(os.Stderr, "  asterisk analyze --launch=<path|id> [--workspace=<path>] -o <artifact>\n")
	fmt.Fprintf(os.Stderr, "  asterisk push -f <artifact-path>\n")
	fmt.Fprintf(os.Stderr, "  asterisk cursor --launch=<path|id> [--workspace=<path>] [--case-id=<id>] [-o <prompt-file>]\n")
	fmt.Fprintf(os.Stderr, "  asterisk save -f <artifact-path>\n")
}

func runAnalyze(args []string) {
	fs := flag.NewFlagSet("analyze", flag.ExitOnError)
	launch := fs.String("launch", "", "Path to envelope JSON or launch ID (required)")
	workspacePath := fs.String("workspace", "", "Path to context workspace file (YAML/JSON)")
	artifactPath := fs.String("o", "", "Output artifact path (required)")
	dbPath := fs.String("db", store.DefaultDBPath, "Store DB path")
	rpBase := fs.String("rp-base-url", "", "RP base URL (optional; for fetch by launch ID)")
	rpKeyPath := fs.String("rp-api-key", ".rp-api-key", "Path to RP API key file")
	_ = fs.Parse(args)

	if *launch == "" || *artifactPath == "" {
		fs.PrintDefaults()
		os.Exit(1)
	}

	// Resolve envelope source and launch ID
	var envSrc investigate.EnvelopeSource
	var launchID int
	if _, err := os.Stat(*launch); err == nil {
		// Treat as file path
		data, err := os.ReadFile(*launch)
		if err != nil {
			fmt.Fprintf(os.Stderr, "read envelope: %v\n", err)
			os.Exit(1)
		}
		var env preinvest.Envelope
		if err := json.Unmarshal(data, &env); err != nil {
			fmt.Fprintf(os.Stderr, "parse envelope: %v\n", err)
			os.Exit(1)
		}
		launchID, _ = strconv.Atoi(env.RunID)
		if launchID == 0 {
			launchID = 1
		}
		envSrc = &singleEnvelopeSource{env: &env, launchID: launchID}
	} else {
		// Treat as launch ID; need store and optionally fetch from RP
		var err error
		launchID, err = strconv.Atoi(*launch)
		if err != nil || launchID <= 0 {
			fmt.Fprintf(os.Stderr, "launch must be path to envelope JSON or positive launch ID\n")
			os.Exit(1)
		}
		st, err := store.Open(*dbPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "open store: %v\n", err)
			os.Exit(1)
		}
		defer st.Close()
		adapter := &store.PreinvestStoreAdapter{Store: st}
		env, _ := adapter.Get(launchID)
		if env == nil && *rpBase != "" {
			key, _ := rpfetch.ReadAPIKey(*rpKeyPath)
			client := rpfetch.NewClient(rpfetch.Config{BaseURL: *rpBase, APIKey: key, Project: "ecosystem-qe"})
			fetcher := rpfetch.NewFetcher(client)
			if err := preinvest.FetchAndSave(fetcher, adapter, launchID); err != nil {
				fmt.Fprintf(os.Stderr, "fetch: %v\n", err)
				os.Exit(1)
			}
			env, _ = adapter.Get(launchID)
		}
		if env == nil {
			fmt.Fprintf(os.Stderr, "no envelope for launch %d (fetch from RP with --rp-base-url or provide envelope file)\n", launchID)
			os.Exit(1)
		}
		envSrc = adapter
	}

	// Load workspace
	var ws *workspace.Workspace
	if *workspacePath != "" {
		w, err := workspace.LoadFromPath(*workspacePath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "load workspace: %v\n", err)
			os.Exit(1)
		}
		ws = w
	}

	if err := investigate.AnalyzeWithWorkspace(envSrc, launchID, *artifactPath, ws); err != nil {
		fmt.Fprintf(os.Stderr, "analyze: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Artifact: %s\n", *artifactPath)
}

func runPush(args []string) {
	fs := flag.NewFlagSet("push", flag.ExitOnError)
	artifactPath := fs.String("f", "", "Artifact file path (required)")
	rpBase := fs.String("rp-base-url", "", "RP base URL (optional)")
	rpKeyPath := fs.String("rp-api-key", ".rp-api-key", "Path to RP API key file")
	_ = fs.Parse(args)

	if *artifactPath == "" {
		fs.PrintDefaults()
		os.Exit(1)
	}

	pushStore := postinvest.NewMemPushStore()
	var pusher postinvest.Pusher = postinvest.DefaultPusher{}
	if *rpBase != "" {
		key, err := rpfetch.ReadAPIKey(*rpKeyPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "read API key: %v\n", err)
			os.Exit(1)
		}
		client := rppush.NewClient(rppush.Config{BaseURL: *rpBase, APIKey: key, Project: "ecosystem-qe"})
		pusher = rppush.NewRPPusher(client)
	}
	if err := pusher.Push(*artifactPath, pushStore, "", ""); err != nil {
		fmt.Fprintf(os.Stderr, "push: %v\n", err)
		os.Exit(1)
	}
	rec := pushStore.LastPushed()
	if rec != nil {
		fmt.Printf("Pushed: launch=%s defect_type=%s\n", rec.LaunchID, rec.DefectType)
	}
}

func runCursor(args []string) {
	fs := flag.NewFlagSet("cursor", flag.ExitOnError)
	launch := fs.String("launch", "", "Path to envelope JSON or launch ID (required)")
	workspacePath := fs.String("workspace", "", "Path to context workspace file (YAML/JSON)")
	caseID := fs.Int("case-id", 0, "Failure (test item) ID; default first from envelope")
	artifactPath := fs.String("o", "", "Output prompt path (default stdout)")
	templatePath := fs.String("template", ".cursor/prompts/rca.md", "Prompt template file")
	dbPath := fs.String("db", store.DefaultDBPath, "Store DB path")
	_ = fs.Parse(args)

	if *launch == "" {
		fs.PrintDefaults()
		os.Exit(1)
	}

	env, _ := loadEnvelopeForCursor(*launch, *dbPath)
	if env == nil {
		fmt.Fprintf(os.Stderr, "could not load envelope for launch %q\n", *launch)
		os.Exit(1)
	}
	if len(env.FailureList) == 0 {
		fmt.Fprintf(os.Stderr, "envelope has no failures\n")
		os.Exit(1)
	}
	item := env.FailureList[0]
	for _, f := range env.FailureList {
		if *caseID == 0 || f.ID == *caseID {
			item = f
			break
		}
	}
	if *caseID != 0 && item.ID != *caseID {
		fmt.Fprintf(os.Stderr, "case-id %d not in envelope\n", *caseID)
		os.Exit(1)
	}

	tmplData, err := os.ReadFile(*templatePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read template: %v\n", err)
		os.Exit(1)
	}
	tmpl, err := template.New("prompt").Parse(string(tmplData))
	if err != nil {
		fmt.Fprintf(os.Stderr, "parse template: %v\n", err)
		os.Exit(1)
	}
	params := map[string]string{
		"LaunchID":     env.RunID,
		"CaseID":       strconv.Itoa(item.ID),
		"WorkspacePath": *workspacePath,
		"FailedTestName": item.Name,
		"ArtifactPath": ".asterisk/artifact.json",
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, params); err != nil {
		fmt.Fprintf(os.Stderr, "execute template: %v\n", err)
		os.Exit(1)
	}
	prompt := buf.String()
	if *artifactPath != "" {
		if err := os.WriteFile(*artifactPath, []byte(prompt), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "write prompt: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Prompt: %s\n", *artifactPath)
	} else {
		fmt.Print(prompt)
	}
}

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

func runSave(args []string) {
	fs := flag.NewFlagSet("save", flag.ExitOnError)
	artifactPath := fs.String("f", "", "Artifact file path (required)")
	dbPath := fs.String("db", store.DefaultDBPath, "Store DB path")
	_ = fs.Parse(args)

	if *artifactPath == "" {
		fs.PrintDefaults()
		os.Exit(1)
	}
	data, err := os.ReadFile(*artifactPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read artifact: %v\n", err)
		os.Exit(1)
	}
	var a investigate.Artifact
	if err := json.Unmarshal(data, &a); err != nil {
		fmt.Fprintf(os.Stderr, "parse artifact: %v\n", err)
		os.Exit(1)
	}
	launchID, _ := strconv.Atoi(a.LaunchID)
	if launchID == 0 {
		fmt.Fprintf(os.Stderr, "invalid launch_id in artifact\n")
		os.Exit(1)
	}

	st, err := store.Open(*dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "open store: %v\n", err)
		os.Exit(1)
	}
	defer st.Close()

	title := a.RCAMessage
	if len(title) > 80 {
		title = title[:80] + "..."
	}
	if title == "" {
		title = "RCA"
	}
	rcaID, err := st.SaveRCA(&store.RCA{
		Title:       title,
		Description: a.RCAMessage,
		DefectType:  a.DefectType,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "save RCA: %v\n", err)
		os.Exit(1)
	}
	cases, _ := st.ListCasesByLaunch(launchID)
	for _, c := range cases {
		for _, itemID := range a.CaseIDs {
			if c.ItemID == itemID {
				_ = st.LinkCaseToRCA(c.ID, rcaID)
				break
			}
		}
	}
	// If no cases in store yet, create them from artifact and link
	if len(cases) == 0 {
		for _, itemID := range a.CaseIDs {
			caseID, err := st.CreateCase(launchID, itemID)
			if err != nil {
				continue
			}
			_ = st.LinkCaseToRCA(caseID, rcaID)
		}
	}
	fmt.Printf("Saved RCA (id=%d); linked to %d case(s)\n", rcaID, len(a.CaseIDs))
}

// singleEnvelopeSource returns a fixed envelope for one launch ID (for file-based envelope).
// Implements investigate.EnvelopeSource.
type singleEnvelopeSource struct {
	env      *preinvest.Envelope
	launchID int
}

func (s *singleEnvelopeSource) Get(launchID int) (*preinvest.Envelope, error) {
	if launchID == s.launchID {
		return s.env, nil
	}
	return nil, nil
}
