package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"asterisk/internal/display"
	"asterisk/internal/postinvest"
	"asterisk/internal/rp"
)

var pushFlags struct {
	artifactPath string
	rpBase       string
	rpKeyPath    string
}

var pushCmd = &cobra.Command{
	Use:   "push",
	Short: "Push an RCA artifact to ReportPortal as a defect update",
	RunE:  runPush,
}

func init() {
	f := pushCmd.Flags()
	f.StringVarP(&pushFlags.artifactPath, "file", "f", "", "Artifact file path (required)")
	f.StringVar(&pushFlags.rpBase, "rp-base-url", "", "RP base URL (optional)")
	f.StringVar(&pushFlags.rpKeyPath, "rp-api-key", ".rp-api-key", "Path to RP API key file")

	_ = pushCmd.MarkFlagRequired("file")
}

func runPush(cmd *cobra.Command, _ []string) error {
	pushStore := postinvest.NewMemPushStore()
	var pusher postinvest.Pusher = postinvest.DefaultPusher{}
	if pushFlags.rpBase != "" {
		key, err := rp.ReadAPIKey(pushFlags.rpKeyPath)
		if err != nil {
			return fmt.Errorf("read API key: %w", err)
		}
		client, err := rp.New(pushFlags.rpBase, key)
		if err != nil {
			return fmt.Errorf("create RP client: %w", err)
		}
		pusher = rp.NewPusher(client, "ecosystem-qe")
	}
	if err := pusher.Push(pushFlags.artifactPath, pushStore, "", ""); err != nil {
		return fmt.Errorf("push: %w", err)
	}
	rec := pushStore.LastPushed()
	if rec != nil {
		fmt.Fprintf(cmd.OutOrStdout(), "Pushed: launch=%s defect_type=%s\n", rec.LaunchID, display.DefectTypeWithCode(rec.DefectType))
	}
	return nil
}
