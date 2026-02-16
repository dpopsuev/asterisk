// run-mock-flow runs the full mock flow: fetch → analyze → push.
// Usage: go run ./cmd/run-mock-flow -launch 33195 -artifact /tmp/artifact.json
// Contract: .cursor/contracts/mock-wiring.md (entry point).
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"asterisk/internal/postinvest"
	"asterisk/internal/preinvest"
	"asterisk/internal/wiring"
)

func main() {
	launchID := flag.Int("launch", 33195, "Launch ID")
	artifactPath := flag.String("artifact", "", "Path to write/read artifact (required)")
	workspace := flag.String("workspace", ".", "Workspace root (for fixture path)")
	flag.Parse()

	if *artifactPath == "" {
		fmt.Fprintln(os.Stderr, "usage: run-mock-flow -launch <id> -artifact <path> [-workspace <dir>]")
		flag.PrintDefaults()
		os.Exit(1)
	}

	env := loadEnvelope(*workspace)
	if env == nil {
		fmt.Fprintln(os.Stderr, "could not load fixture envelope (run from repo root or set -workspace)")
		os.Exit(1)
	}

	fetcher := preinvest.NewStubFetcher(env)
	envelopeStore := preinvest.NewMemStore()
	pushStore := postinvest.NewMemPushStore()

	if err := wiring.Run(fetcher, envelopeStore, *launchID, *artifactPath, pushStore, "", ""); err != nil {
		fmt.Fprintf(os.Stderr, "Run: %v\n", err)
		os.Exit(1)
	}

	rec := pushStore.LastPushed()
	if rec != nil {
		fmt.Printf("Pushed: launch=%s defect_type=%s\n", rec.LaunchID, rec.DefectType)
	}
	fmt.Printf("Artifact: %s\n", *artifactPath)
}

func loadEnvelope(workspace string) *preinvest.Envelope {
	path := filepath.Join(workspace, "examples", "pre-investigation-33195-4.21", "envelope_33195_4.21.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var env preinvest.Envelope
	if err := json.Unmarshal(data, &env); err != nil {
		return nil
	}
	return &env
}
