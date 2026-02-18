// asterisk-analyze-rp-cursor is a purpose-built binary for AI-driven RCA
// via the Cursor agent. It wraps `asterisk analyze` with baked-in defaults:
//
//   - --adapter=cursor --dispatch=file (Cursor agent as the reasoning engine)
//   - RP URL and project from environment ($ASTERISK_RP_URL, $ASTERISK_RP_PROJECT)
//   - --report enabled (human-readable Markdown alongside JSON artifact)
//   - Output to .asterisk/output/rca-<launch>.json
//
// Usage:
//
//	asterisk-analyze-rp-cursor 33195
//	asterisk-analyze-rp-cursor 33195 --prompt-dir .cursor/prompts
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func main() {
	if len(os.Args) < 2 || os.Args[1] == "-h" || os.Args[1] == "--help" {
		printUsage()
		os.Exit(0)
	}

	launchID := os.Args[1]

	rpURL := os.Getenv("ASTERISK_RP_URL")
	if rpURL == "" {
		fmt.Fprintln(os.Stderr, "error: ASTERISK_RP_URL is not set")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Set it to your ReportPortal instance:")
		fmt.Fprintln(os.Stderr, "  export ASTERISK_RP_URL=https://your-rp-instance.example.com")
		os.Exit(1)
	}

	rpProject := os.Getenv("ASTERISK_RP_PROJECT")
	if rpProject == "" {
		fmt.Fprintln(os.Stderr, "error: ASTERISK_RP_PROJECT is not set")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Set it to your RP project name:")
		fmt.Fprintln(os.Stderr, "  export ASTERISK_RP_PROJECT=your-project-name")
		os.Exit(1)
	}

	asteriskBin := findAsteriskBinary()
	if asteriskBin == "" {
		fmt.Fprintln(os.Stderr, "error: asterisk binary not found")
		fmt.Fprintln(os.Stderr, "Build it first: go build -o bin/asterisk ./cmd/asterisk/")
		os.Exit(1)
	}

	args := []string{
		"analyze", launchID,
		"--adapter=cursor",
		"--dispatch=file",
		"--report",
		"--rp-base-url", rpURL,
		"--rp-project", rpProject,
	}
	args = append(args, os.Args[2:]...)

	cmd := exec.Command(asteriskBin, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()

	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			os.Exit(exitErr.ExitCode())
		}
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func findAsteriskBinary() string {
	candidates := []string{
		filepath.Join("bin", "asterisk"),
		"asterisk",
	}
	for _, c := range candidates {
		if p, err := exec.LookPath(c); err == nil {
			return p
		}
	}
	return ""
}

func printUsage() {
	fmt.Println(`asterisk-analyze-rp-cursor — AI-driven RCA via Cursor agent

Usage:
  asterisk-analyze-rp-cursor <LAUNCH_ID> [extra flags...]

Baked-in defaults:
  --adapter=cursor     Cursor agent as the reasoning engine (F0-F6 pipeline)
  --dispatch=file      Signal-based communication via signal.json
  --report             Human-readable Markdown report alongside JSON artifact
  --rp-base-url        From $ASTERISK_RP_URL
  --rp-project         From $ASTERISK_RP_PROJECT

Prerequisites:
  1. ASTERISK_RP_URL     — export ASTERISK_RP_URL=https://your-rp-instance.example.com
  2. ASTERISK_RP_PROJECT — export ASTERISK_RP_PROJECT=your-project-name
  3. .rp-api-key         — RP API token file (chmod 600)
  4. bin/asterisk         — build with: go build -o bin/asterisk ./cmd/asterisk/

Extra flags are forwarded to 'asterisk analyze', e.g.:
  asterisk-analyze-rp-cursor 33195 --prompt-dir .cursor/prompts
  asterisk-analyze-rp-cursor 33195 --db /tmp/my.db`)
}
