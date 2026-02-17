package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// version is set at build time via -ldflags.
var version = "dev"

var rootCmd = &cobra.Command{
	Use:   "asterisk",
	Short: "Evidence-based RCA for ReportPortal test failures",
	Long:  "Asterisk performs root-cause analysis on ReportPortal CI failures\nby correlating with external repos and CI/infrastructure data.",
	CompletionOptions: cobra.CompletionOptions{
		HiddenDefaultCmd: true,
	},
}

func init() {
	rootCmd.AddCommand(analyzeCmd)
	rootCmd.AddCommand(pushCmd)
	rootCmd.AddCommand(cursorCmd)
	rootCmd.AddCommand(saveCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(calibrateCmd)
	rootCmd.Version = version
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
