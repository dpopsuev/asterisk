//go:build e2e

package main

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/dpopsuev/origami/kami"
)

func TestDemoBrowser_RendersPresentation(t *testing.T) {
	if kami.FrontendFS() == nil {
		t.Skip("frontend not built")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	bridge := kami.NewEventBridge(nil)
	defer bridge.Close()

	srv := kami.NewServer(kami.Config{
		Bridge: bridge,
		Debug:  true,
		Theme:  PoliceStationTheme{},
		Kabuki: PoliceStationKabuki{},
		SPA:    kami.FrontendFS(),
	})
	httpAddr, _, err := srv.StartOnAvailablePort(ctx)
	if err != nil {
		t.Fatalf("start server: %v", err)
	}
	base := fmt.Sprintf("http://%s", httpAddr)

	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("no-sandbox", true),
	)
	allocCtx, allocCancel := chromedp.NewExecAllocator(ctx, opts...)
	defer allocCancel()

	browserCtx, browserCancel := chromedp.NewContext(allocCtx)
	defer browserCancel()

	t.Run("page loads and React mounts", func(t *testing.T) {
		var title string
		var rootHTML string
		err := chromedp.Run(browserCtx,
			chromedp.Navigate(base),
			chromedp.WaitReady("#root", chromedp.ByID),
			chromedp.Title(&title),
			chromedp.InnerHTML("#root", &rootHTML, chromedp.ByID),
		)
		if err != nil {
			t.Fatalf("chromedp: %v", err)
		}

		if rootHTML == "" {
			t.Error("React did not mount — #root is empty")
		}
	})

	t.Run("Kabuki data fetched and rendered", func(t *testing.T) {
		var heroText string
		err := chromedp.Run(browserCtx,
			chromedp.Navigate(base),
			chromedp.WaitReady("#root", chromedp.ByID),
			chromedp.Sleep(2*time.Second),
			chromedp.InnerHTML("#root", &heroText, chromedp.ByID),
		)
		if err != nil {
			t.Fatalf("chromedp: %v", err)
		}

		for _, want := range []string{"Asterisk", "Root-Cause"} {
			if !containsCI(heroText, want) {
				t.Errorf("rendered HTML missing %q", want)
			}
		}
	})

	t.Run("API endpoints reachable from browser context", func(t *testing.T) {
		js := fmt.Sprintf(`
			var xhr = new XMLHttpRequest();
			xhr.open('GET', '%s/api/kabuki', false);
			xhr.send();
			JSON.stringify(Object.keys(JSON.parse(xhr.responseText)));
		`, base)
		var result string
		err := chromedp.Run(browserCtx,
			chromedp.Navigate(base),
			chromedp.WaitReady("#root", chromedp.ByID),
			chromedp.Evaluate(js, &result),
		)
		if err != nil {
			t.Fatalf("chromedp: %v", err)
		}

		for _, key := range []string{"hero", "problem", "results", "closing"} {
			if !containsCI(result, key) {
				t.Errorf("kabuki API response missing key %q, got keys: %s", key, result)
			}
		}
	})
}

func containsCI(s, substr string) bool {
	return len(s) >= len(substr) &&
		len(substr) > 0 &&
		(s == substr || len(s) > 0 && findCI(s, substr))
}

func findCI(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		match := true
		for j := range substr {
			sc, tc := s[i+j], substr[j]
			if sc >= 'A' && sc <= 'Z' {
				sc += 32
			}
			if tc >= 'A' && tc <= 'Z' {
				tc += 32
			}
			if sc != tc {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}
