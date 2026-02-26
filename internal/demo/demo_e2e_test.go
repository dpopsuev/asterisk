package demo_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/dpopsuev/origami/kami"

	"asterisk/internal/demo"
)

func startDemoServer(t *testing.T, withSPA, withReplay bool) string {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	t.Cleanup(cancel)

	bridge := kami.NewEventBridge(nil)
	t.Cleanup(func() { bridge.Close() })

	cfg := kami.Config{
		Bridge: bridge,
		Debug:  true,
		Theme:  demo.PoliceStationTheme{},
		Kabuki: demo.PoliceStationKabuki{},
	}
	if withSPA {
		cfg.SPA = kami.FrontendFS()
	}

	srv := kami.NewServer(cfg)
	httpAddr, _, err := srv.StartOnAvailablePort(ctx)
	if err != nil {
		t.Fatalf("start server: %v", err)
	}

	if withReplay {
		replayer, err := kami.NewReplayer(bridge, "../../testdata/demo/sample.jsonl", 100.0)
		if err != nil {
			t.Fatalf("load recording: %v", err)
		}
		done := ctx.Done()
		go func() { _ = replayer.Play(done) }()
	}

	return httpAddr
}

func TestDemoE2E_SPAServed(t *testing.T) {
	if kami.FrontendFS() == nil {
		t.Skip("frontend not built — run: cd origami/kami/frontend && npm run build")
	}

	addr := startDemoServer(t, true, false)
	base := fmt.Sprintf("http://%s", addr)

	t.Run("root returns HTML with SPA shell", func(t *testing.T) {
		resp, err := http.Get(base + "/")
		if err != nil {
			t.Fatalf("GET /: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			t.Fatalf("status = %d, want 200", resp.StatusCode)
		}

		ct := resp.Header.Get("Content-Type")
		if !strings.Contains(ct, "text/html") {
			t.Fatalf("Content-Type = %q, want text/html", ct)
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		html := string(body)

		for _, needle := range []string{
			"<!doctype html>",
			`<div id="root">`,
			`<script type="module"`,
			"/assets/index-",
		} {
			if !strings.Contains(strings.ToLower(html), strings.ToLower(needle)) {
				t.Errorf("index.html missing %q", needle)
			}
		}
	})

	t.Run("JS bundle served", func(t *testing.T) {
		resp, err := http.Get(base + "/assets/index-Du9fjL1r.js")
		if err != nil {
			t.Fatalf("GET JS bundle: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			t.Errorf("JS bundle status = %d, want 200", resp.StatusCode)
		}
		ct := resp.Header.Get("Content-Type")
		if !strings.Contains(ct, "javascript") {
			t.Errorf("JS Content-Type = %q, want javascript", ct)
		}
	})

	t.Run("CSS bundle served", func(t *testing.T) {
		resp, err := http.Get(base + "/assets/index-L7tMqy6u.css")
		if err != nil {
			t.Fatalf("GET CSS bundle: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			t.Errorf("CSS bundle status = %d, want 200", resp.StatusCode)
		}
	})

	t.Run("without SPA returns fallback text", func(t *testing.T) {
		noSPA := startDemoServer(t, false, false)
		resp, err := http.Get(fmt.Sprintf("http://%s/", noSPA))
		if err != nil {
			t.Fatalf("GET /: %v", err)
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		if !strings.Contains(string(body), "Kami debugger running") {
			t.Errorf("expected fallback message, got: %s", string(body))
		}
	})
}

func TestDemoE2E_FullDemoFlow(t *testing.T) {
	if kami.FrontendFS() == nil {
		t.Skip("frontend not built — run: cd origami/kami/frontend && npm run build")
	}

	addr := startDemoServer(t, true, true)
	base := fmt.Sprintf("http://%s", addr)

	t.Run("SPA serves index.html", func(t *testing.T) {
		resp, err := http.Get(base + "/")
		if err != nil {
			t.Fatalf("GET /: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Fatalf("status = %d", resp.StatusCode)
		}
		body, _ := io.ReadAll(resp.Body)
		if !strings.Contains(string(body), `<div id="root">`) {
			t.Error("SPA shell not found")
		}
	})

	t.Run("all API endpoints respond", func(t *testing.T) {
		endpoints := map[string]func(map[string]any){
			"/api/health": func(body map[string]any) {
				if body["status"] != "ok" {
					t.Errorf("health status = %v", body["status"])
				}
			},
			"/api/theme": func(body map[string]any) {
				if body["name"] != "Asterisk Police Station" {
					t.Errorf("theme name = %v", body["name"])
				}
			},
			"/api/kabuki": func(body map[string]any) {
				for _, key := range []string{"hero", "problem", "results", "competitive", "architecture", "roadmap", "closing", "transition_line"} {
					if _, ok := body[key]; !ok {
						t.Errorf("kabuki missing %q", key)
					}
				}
			},
			"/api/pipeline": func(body map[string]any) {
				nodes, ok := body["nodes"].(map[string]any)
				if !ok || len(nodes) == 0 {
					t.Error("pipeline nodes empty")
				}
			},
		}

		for path, validate := range endpoints {
			t.Run(path, func(t *testing.T) {
				resp, err := http.Get(base + path)
				if err != nil {
					t.Fatalf("GET %s: %v", path, err)
				}
				defer resp.Body.Close()
				if resp.StatusCode != 200 {
					t.Fatalf("status = %d", resp.StatusCode)
				}
				var body map[string]any
				if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
					t.Fatalf("decode: %v", err)
				}
				validate(body)
			})
		}
	})

	t.Run("SSE stream available", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		req, _ := http.NewRequestWithContext(ctx, "GET", base+"/events/stream", nil)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("GET /events/stream: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			t.Fatalf("status = %d", resp.StatusCode)
		}
		ct := resp.Header.Get("Content-Type")
		if !strings.Contains(ct, "text/event-stream") {
			t.Errorf("Content-Type = %q, want text/event-stream", ct)
		}
	})
}
