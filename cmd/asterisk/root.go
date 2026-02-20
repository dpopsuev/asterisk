package main

import (
	"fmt"
	"log/slog"
	"os"
	"strings"

	"asterisk/internal/logging"

	"github.com/spf13/cobra"
)

// version is set at build time via -ldflags.
var version = "dev"

var (
	logLevel  string
	logFormat string
)

var rootCmd = &cobra.Command{
	Use:   "asterisk",
	Short: "Evidence-based RCA for ReportPortal test failures",
	Long:  "Asterisk performs root-cause analysis on ReportPortal CI failures\nby correlating with external repos and CI/infrastructure data.",
	CompletionOptions: cobra.CompletionOptions{
		HiddenDefaultCmd: true,
	},
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		level := parseLogLevel(logLevel)
		logging.Init(level, logFormat)
	},
}

func init() {
	rootCmd.PersistentFlags().StringVar(&logLevel, "log-level", "info", "log level: debug, info, warn, error")
	rootCmd.PersistentFlags().StringVar(&logFormat, "log-format", "text", "log format: text, json")

	rootCmd.AddCommand(analyzeCmd)
	rootCmd.AddCommand(pushCmd)
	rootCmd.AddCommand(cursorCmd)
	rootCmd.AddCommand(saveCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(calibrateCmd)
	rootCmd.AddCommand(serveCmd)
	rootCmd.Version = version
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

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
