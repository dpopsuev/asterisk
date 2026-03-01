package main

import (
	"fmt"
	"log/slog"
	"os"
	"strings"

	origamicli "github.com/dpopsuev/origami/cli"
	"github.com/dpopsuev/origami/logging"
	"github.com/spf13/cobra"
)

var version = "dev"

func main() {
	c, err := origamicli.NewCLI("asterisk", "Evidence-based RCA for ReportPortal test failures").
		WithVersion(version).
		WithCircuit("circuits/asterisk-ingest.yaml").
		WithExtraCommand(analyzeCmd).
		WithExtraCommand(calibrateCmd).
		WithExtraCommand(consumeCmd).
		WithExtraCommand(datasetCmd).
		WithExtraCommand(demoCmd).
		WithExtraCommand(serveCmd).
		WithExtraCommand(pushCmd).
		WithExtraCommand(cursorCmd).
		WithExtraCommand(saveCmd).
		WithExtraCommand(statusCmd).
		WithExtraCommand(gtCmd).
		Build()
	if err != nil {
		fmt.Fprintf(os.Stderr, "build CLI: %v\n", err)
		os.Exit(1)
	}

	root := c.Root()
	root.Long = "Asterisk performs root-cause analysis on ReportPortal CI failures\nby correlating with external repos and CI/infrastructure data."
	root.CompletionOptions = cobra.CompletionOptions{HiddenDefaultCmd: true}

	var logLevel, logFormat string
	root.PersistentFlags().StringVar(&logLevel, "log-level", "info", "log level: debug, info, warn, error")
	root.PersistentFlags().StringVar(&logFormat, "log-format", "text", "log format: text, json")
	root.PersistentPreRun = func(_ *cobra.Command, _ []string) {
		logging.Init(parseLogLevel(logLevel), logFormat)
	}

	if err := c.Execute(); err != nil {
		os.Exit(1)
	}
}

func parseLogLevel(s string) slog.Level {
	switch strings.ToLower(s) {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
