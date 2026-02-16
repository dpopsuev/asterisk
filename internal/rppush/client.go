package rppush

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
)

// Config holds RP API connection settings (same shape as rpfetch for consistency).
type Config struct {
	BaseURL string
	APIKey  string
	Project string
}

// Client is an RP API client for updating test item defect types.
type Client struct {
	HTTPClient *http.Client
	Config     Config
}

// NewClient returns a client with the given config.
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

// UpdateItemDefectType sends a request to RP to set the defect type for the given test item.
// Uses PUT /api/v1/{project}/item/{itemId}/update with an issues payload per RP 5.11.
func (c *Client) UpdateItemDefectType(itemID int, defectType string) error {
	u := fmt.Sprintf("%s/api/v1/%s/item/%d/update", c.Config.BaseURL, c.Config.Project, itemID)
	// RP 5.11 update test item: body may include issues; use a minimal payload for defect type.
	body := map[string]interface{}{
		"issues": []map[string]interface{}{
			{"issueType": defectType},
		},
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal body: %w", err)
	}
	req, err := http.NewRequest(http.MethodPut, u, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if c.Config.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.Config.APIKey)
	}
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("update item %s: %s", resp.Status, string(b))
	}
	return nil
}

// UpdateItemsDefectType updates defect type for each item ID (one API call per item).
func (c *Client) UpdateItemsDefectType(itemIDs []int, defectType string) error {
	for _, id := range itemIDs {
		if err := c.UpdateItemDefectType(id, defectType); err != nil {
			return fmt.Errorf("item %s: %w", strconv.Itoa(id), err)
		}
	}
	return nil
}
