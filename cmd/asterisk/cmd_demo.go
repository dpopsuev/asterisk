package main

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"

	"github.com/dpopsuev/origami/kami"
	"github.com/dpopsuev/origami/logging"
	"github.com/spf13/cobra"
)

var (
	demoPort   int
	demoBind   string
	demoReplay string
	demoSpeed  float64
	demoLive   bool
)

var demoCmd = &cobra.Command{
	Use:   "demo",
	Short: "Launch the interactive RCA demo presentation",
	Long: `Starts a local web server presenting the Asterisk PoC demo as an
interactive, section-based SPA. Use --replay to play back a recorded
calibration session, or --live to connect to a running pipeline.`,
	RunE: runDemo,
}

func init() {
	demoCmd.Flags().IntVar(&demoPort, "port", 3000, "HTTP port for the presentation SPA")
	demoCmd.Flags().StringVar(&demoBind, "bind", "127.0.0.1", "bind address")
	demoCmd.Flags().StringVar(&demoReplay, "replay", "", "path to JSONL recording for replay mode")
	demoCmd.Flags().Float64Var(&demoSpeed, "speed", 1.0, "replay speed multiplier (e.g. 2.0 = 2x)")
	demoCmd.Flags().BoolVar(&demoLive, "live", false, "connect to a running pipeline via SSE")
}

func runDemo(cmd *cobra.Command, _ []string) error {
	log := logging.New("demo")

	bridge := kami.NewEventBridge(nil)
	defer bridge.Close()

	theme := PoliceStationTheme{}
	kabuki := PoliceStationKabuki{}
	srv := kami.NewServer(kami.Config{
		Port:   demoPort,
		Bind:   demoBind,
		Debug:  true,
		Logger: log,
		Bridge: bridge,
		Theme:  theme,
		Kabuki: kabuki,
		SPA:    kami.FrontendFS(),
	})

	ctx, cancel := signal.NotifyContext(cmd.Context(), os.Interrupt)
	defer cancel()

	if demoReplay != "" {
		replayer, err := kami.NewReplayer(bridge, demoReplay, demoSpeed)
		if err != nil {
			return fmt.Errorf("load recording: %w", err)
		}
		done := ctx.Done()
		go func() {
			if err := replayer.Play(done); err != nil {
				log.Error("replay error", slog.Any("err", err))
			}
		}()
	}

	addr := fmt.Sprintf("%s:%d", demoBind, demoPort)
	log.Info("starting demo presentation",
		slog.String("url", fmt.Sprintf("http://%s", addr)),
		slog.String("mode", demoMode(demoReplay, demoLive)))

	if err := srv.Start(ctx); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

func demoMode(replay string, live bool) string {
	if replay != "" {
		return "replay"
	}
	if live {
		return "live"
	}
	return "static"
}
