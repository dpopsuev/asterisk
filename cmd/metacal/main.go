package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"asterisk/pkg/framework"
	"asterisk/pkg/framework/metacal"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	var err error
	switch os.Args[1] {
	case "prompt":
		err = cmdPrompt(os.Args[2:])
	case "analyze":
		err = cmdAnalyze(os.Args[2:])
	case "save":
		err = cmdSave(os.Args[2:])
	default:
		printUsage()
		os.Exit(1)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Fprintf(os.Stderr, `metacal — meta-calibration discovery CLI

Subcommands:
  prompt   Build the full discovery prompt (identity + exclusions + probe)
  analyze  Parse a subagent response: extract identity, code, and score
  save     Persist a RunReport JSON to the append-only store

Usage:
  metacal prompt [--exclude-file FILE]
  metacal analyze --response-file FILE   (use - for stdin)
  metacal save --report-file FILE [--runs-dir DIR]   (use - for stdin)
`)
}

func cmdPrompt(args []string) error {
	fs := flag.NewFlagSet("prompt", flag.ContinueOnError)
	excludeFile := fs.String("exclude-file", "", "JSON file with array of ModelIdentity to exclude")
	if err := fs.Parse(args); err != nil {
		return err
	}

	var exclude []framework.ModelIdentity
	if *excludeFile != "" {
		data, err := os.ReadFile(*excludeFile)
		if err != nil {
			return fmt.Errorf("read exclude file: %w", err)
		}
		if err := json.Unmarshal(data, &exclude); err != nil {
			return fmt.Errorf("parse exclude file: %w", err)
		}
	}

	fmt.Print(metacal.BuildFullPrompt(exclude))
	return nil
}

// AnalyzeResult is the structured output of the analyze subcommand.
type AnalyzeResult struct {
	Identity framework.ModelIdentity `json:"identity"`
	Key      string                  `json:"key"`
	Code     string                  `json:"code"`
	Score    metacal.ProbeScore      `json:"score"`
	Known    bool                    `json:"known"`
	Wrapper  bool                    `json:"wrapper"`
}

func cmdAnalyze(args []string) error {
	fs := flag.NewFlagSet("analyze", flag.ContinueOnError)
	responseFile := fs.String("response-file", "", "text file containing the raw subagent response")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if *responseFile == "" {
		return fmt.Errorf("--response-file is required")
	}

	var data []byte
	var err error
	if *responseFile == "-" {
		data, err = io.ReadAll(os.Stdin)
	} else {
		data, err = os.ReadFile(*responseFile)
	}
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}
	raw := string(data)

	mi, err := metacal.ParseIdentityResponse(raw)
	if err != nil {
		return fmt.Errorf("parse identity: %w", err)
	}

	code, err := metacal.ParseProbeResponse(raw)
	if err != nil {
		return fmt.Errorf("parse code: %w", err)
	}

	score := metacal.ScoreRefactorOutput(code)

	result := AnalyzeResult{
		Identity: mi,
		Key:      metacal.ModelKey(mi),
		Code:     code,
		Score:    score,
		Known:    framework.IsKnownModel(mi),
		Wrapper:  framework.IsWrapperName(mi.ModelName),
	}

	out, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal result: %w", err)
	}
	fmt.Println(string(out))
	return nil
}

func cmdSave(args []string) error {
	fs := flag.NewFlagSet("save", flag.ContinueOnError)
	reportFile := fs.String("report-file", "", "JSON file containing the RunReport")
	runsDir := fs.String("runs-dir", filepath.Join("pkg", "framework", "metacal", "runs"), "directory to save run reports")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if *reportFile == "" {
		return fmt.Errorf("--report-file is required")
	}

	var data []byte
	var err error
	if *reportFile == "-" {
		data, err = io.ReadAll(os.Stdin)
	} else {
		data, err = os.ReadFile(*reportFile)
	}
	if err != nil {
		return fmt.Errorf("read report: %w", err)
	}

	var report metacal.RunReport
	if err := json.Unmarshal(data, &report); err != nil {
		return fmt.Errorf("parse report: %w", err)
	}

	store, err := metacal.NewFileRunStore(*runsDir)
	if err != nil {
		return fmt.Errorf("create store: %w", err)
	}

	if err := store.SaveRun(report); err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "saved run %q to %s\n", report.RunID, *runsDir)
	return nil
}
