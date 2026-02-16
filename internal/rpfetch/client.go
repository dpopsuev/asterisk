package rpfetch

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"

	"asterisk/internal/preinvest"
)

// Config holds RP API connection settings.
type Config struct {
	BaseURL string // e.g. https://your-reportportal.example.com
	APIKey  string // Bearer token (e.g. from .rp-api-key)
	Project string // e.g. ecosystem-qe
}

// Client is an RP API client for fetching launch and test items.
type Client struct {
	HTTPClient *http.Client
	Config     Config
}

// NewClient returns a client with the given config. HTTPClient may be nil to use http.DefaultClient.
func NewClient(cfg Config) *Client {
	if cfg.BaseURL != "" {
		cfg.BaseURL = strings.TrimSuffix(cfg.BaseURL, "/")
	}
	c := &Client{Config: cfg, HTTPClient: http.DefaultClient}
	if c.HTTPClient == nil {
		c.HTTPClient = http.DefaultClient
	}
	return c
}

// Minimal RP API response shapes for unmarshalling.
type rpLaunch struct {
	ID        int    `json:"id"`
	UUID      string `json:"uuid"`
	Name      string `json:"name"`
	Number    int    `json:"number"`
	Status    string `json:"status"`
	StartTime int64  `json:"startTime"`
	EndTime   int64  `json:"endTime"`
}

type rpItem struct {
	ID        int    `json:"id"`
	UUID      string `json:"uuid"`
	Name      string `json:"name"`
	Type      string `json:"type"`
	Status    string `json:"status"`
	Path      string `json:"path"`
	LaunchID  int    `json:"launchId"`
	StartTime int64  `json:"startTime"`
	EndTime   int64  `json:"endTime"`
}

// FetchEnvelope fetches launch and failed test items from RP and returns a preinvest.Envelope.
func (c *Client) FetchEnvelope(launchID int) (*preinvest.Envelope, error) {
	launch, err := c.fetchLaunch(launchID)
	if err != nil {
		return nil, err
	}
	items, err := c.fetchItems(launchID, true) // failed only
	if err != nil {
		return nil, err
	}
	env := &preinvest.Envelope{
		RunID:       strconv.Itoa(launch.ID),
		LaunchUUID:  launch.UUID,
		Name:        launch.Name,
		FailureList: make([]preinvest.FailureItem, 0, len(items)),
	}
	for _, it := range items {
		path := it.Path
		if path == "" {
			path = strconv.Itoa(it.ID)
		}
		env.FailureList = append(env.FailureList, preinvest.FailureItem{
			ID:     it.ID,
			UUID:   it.UUID,
			Name:   it.Name,
			Type:   it.Type,
			Status: it.Status,
			Path:   path,
		})
	}
	return env, nil
}

func (c *Client) fetchLaunch(launchID int) (*rpLaunch, error) {
	u := fmt.Sprintf("%s/api/v1/%s/launch/%d", c.Config.BaseURL, c.Config.Project, launchID)
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("new request: %w", err)
	}
	if c.Config.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.Config.APIKey)
	}
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("launch %s: %s", resp.Status, string(body))
	}
	var launch rpLaunch
	if err := json.NewDecoder(resp.Body).Decode(&launch); err != nil {
		return nil, fmt.Errorf("decode launch: %w", err)
	}
	return &launch, nil
}

func (c *Client) fetchItems(launchID int, failedOnly bool) ([]rpItem, error) {
	var all []rpItem
	page := 1
	pageSize := 200
	for {
		u := fmt.Sprintf("%s/api/v1/%s/item?filter.eq.launchId=%d&page.size=%d&page.page=%d",
			c.Config.BaseURL, c.Config.Project, launchID, pageSize, page)
		if failedOnly {
			u += "&filter.eq.status=FAILED"
		}
		req, err := http.NewRequest(http.MethodGet, u, nil)
		if err != nil {
			return nil, fmt.Errorf("new request: %w", err)
		}
		if c.Config.APIKey != "" {
			req.Header.Set("Authorization", "Bearer "+c.Config.APIKey)
		}
		resp, err := c.HTTPClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("do request: %w", err)
		}
		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			return nil, fmt.Errorf("items %s: %s", resp.Status, string(body))
		}
		var pageData []rpItem
		if err := json.NewDecoder(resp.Body).Decode(&pageData); err != nil {
			resp.Body.Close()
			return nil, fmt.Errorf("decode items: %w", err)
		}
		resp.Body.Close()
		all = append(all, pageData...)
		if len(pageData) < pageSize {
			break
		}
		page++
	}
	return all, nil
}

// ReadAPIKey reads the first line of path (e.g. .rp-api-key) and returns it trimmed.
func ReadAPIKey(path string) (string, error) {
	// Prefer passing path; caller may use os.ReadFile. We keep a simple helper.
	return readFirstLine(path)
}

func readFirstLine(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	line := strings.TrimSpace(strings.Split(string(data), "\n")[0])
	return line, nil
}
