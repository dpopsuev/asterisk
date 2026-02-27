package demo_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/dpopsuev/origami/kami"

	"asterisk/internal/demo"
)

func TestDemoServer_KabukiAPI(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	bridge := kami.NewEventBridge(nil)
	defer bridge.Close()

	srv := kami.NewServer(kami.Config{
		Bridge: bridge,
		Theme:  demo.PoliceStationTheme{},
		Kabuki: demo.PoliceStationKabuki{},
	})

	httpAddr, _, err := srv.StartOnAvailablePort(ctx)
	if err != nil {
		t.Fatalf("start server: %v", err)
	}
	base := fmt.Sprintf("http://%s", httpAddr)

	t.Run("kabuki", func(t *testing.T) {
		resp, err := http.Get(base + "/api/kabuki")
		if err != nil {
			t.Fatalf("GET /api/kabuki: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Fatalf("status = %d, want 200", resp.StatusCode)
		}

		var payload map[string]any
		if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
			t.Fatalf("decode: %v", err)
		}
		for _, key := range []string{"hero", "problem", "results", "competitive", "architecture", "roadmap", "closing", "transition_line"} {
			if _, ok := payload[key]; !ok {
				t.Errorf("missing key %q in /api/kabuki response", key)
			}
		}

		order, ok := payload["section_order"].([]any)
		if !ok || len(order) == 0 {
			t.Fatal("section_order missing or empty")
		}
		if len(order) != 27 {
			t.Errorf("section_order has %d entries, want 27", len(order))
		}

		showcases, ok := payload["code_showcases"].([]any)
		if !ok || len(showcases) == 0 {
			t.Fatal("code_showcases missing or empty")
		}
		if len(showcases) != 4 {
			t.Errorf("code_showcases has %d entries, want 4", len(showcases))
		}

		concepts, ok := payload["concepts"].([]any)
		if !ok || len(concepts) == 0 {
			t.Fatal("concepts missing or empty")
		}
		if len(concepts) != 11 {
			t.Errorf("concepts has %d entries, want 11", len(concepts))
		}
	})

	t.Run("theme", func(t *testing.T) {
		resp, err := http.Get(base + "/api/theme")
		if err != nil {
			t.Fatalf("GET /api/theme: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Fatalf("status = %d, want 200", resp.StatusCode)
		}

		var payload map[string]any
		if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if payload["name"] != "Asterisk Police Station" {
			t.Errorf("theme name = %v, want 'Asterisk Police Station'", payload["name"])
		}
	})

	t.Run("pipeline", func(t *testing.T) {
		resp, err := http.Get(base + "/api/pipeline")
		if err != nil {
			t.Fatalf("GET /api/pipeline: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Fatalf("status = %d, want 200", resp.StatusCode)
		}

		var payload map[string]any
		if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
			t.Fatalf("decode: %v", err)
		}
		nodes, ok := payload["nodes"].(map[string]any)
		if !ok || len(nodes) == 0 {
			t.Error("pipeline nodes is empty or not a map")
		}
	})

	t.Run("health", func(t *testing.T) {
		resp, err := http.Get(base + "/api/health")
		if err != nil {
			t.Fatalf("GET /api/health: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Fatalf("status = %d, want 200", resp.StatusCode)
		}
	})
}

func TestDemoServer_ReplayMode(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	bridge := kami.NewEventBridge(nil)
	defer bridge.Close()

	replayer, err := kami.NewReplayer(bridge, "../../testdata/demo/sample.jsonl", 100.0)
	if err != nil {
		t.Fatalf("load recording: %v", err)
	}

	srv := kami.NewServer(kami.Config{
		Bridge: bridge,
		Theme:  demo.PoliceStationTheme{},
		Kabuki: demo.PoliceStationKabuki{},
	})

	httpAddr, _, err := srv.StartOnAvailablePort(ctx)
	if err != nil {
		t.Fatalf("start server: %v", err)
	}

	done := ctx.Done()
	replayDone := make(chan error, 1)
	go func() {
		replayDone <- replayer.Play(done)
	}()

	resp, err := http.Get(fmt.Sprintf("http://%s/api/kabuki", httpAddr))
	if err != nil {
		t.Fatalf("GET /api/kabuki: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	select {
	case err := <-replayDone:
		if err != nil {
			t.Fatalf("replay error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("replay did not complete within 5s at 100x speed")
	}
}
