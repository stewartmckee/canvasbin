// Package analytics sends server-side events to a self-hosted Umami instance,
// for activity the browser never sees (AI assistants, email links).
//
// Events carry a name and a few low-cardinality properties only: never page
// keys, titles, content, email addresses or user ids.
package analytics

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Umami silently drops events whose User-Agent looks like a bot; words such as
// "server" or "bot" trigger it.
const userAgent = "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Safari/537.36 Canvasbin/1.0"

// Client posts events to Umami's /api/send. A nil *Client is valid and does nothing.
type Client struct {
	endpoint string
	website  string
	hostname string
	http     *http.Client
}

// New returns a client, or nil when baseURL or websiteID is empty.
func New(baseURL, websiteID, publicBaseURL string) *Client {
	baseURL, websiteID = strings.TrimRight(strings.TrimSpace(baseURL), "/"), strings.TrimSpace(websiteID)
	if baseURL == "" || websiteID == "" {
		return nil
	}
	host := "canvasbin.dev"
	if u, err := url.Parse(publicBaseURL); err == nil && u.Host != "" {
		host = u.Host
	}
	return &Client{
		endpoint: baseURL + "/api/send",
		website:  websiteID,
		hostname: host,
		http:     &http.Client{Timeout: 5 * time.Second},
	}
}

// Enabled reports whether events are sent, so callers can skip work to build them.
func (c *Client) Enabled() bool { return c != nil }

// PageCreated records a page-create event, plus first-page when created (the
// number of pages the user has now created) is 1. source is "web" or "mcp".
func (c *Client) PageCreated(source, format string, team bool, created int) {
	path := "/new"
	if source == "mcp" {
		path = "/mcp"
	}
	c.Event(path, "page-create", map[string]any{"source": source, "format": format, "team": team})
	if created == 1 {
		c.Event(path, "first-page", map[string]any{"source": source})
	}
}

// Event records name against path in the background. data values should be
// short strings, numbers or booleans.
func (c *Client) Event(path, name string, data map[string]any) {
	if c == nil {
		return
	}
	body, err := json.Marshal(map[string]any{
		"type": "event",
		"payload": map[string]any{
			"website":  c.website,
			"hostname": c.hostname,
			"url":      path,
			"name":     name,
			"data":     data,
		},
	})
	if err != nil {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(body))
		if err != nil {
			return
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("User-Agent", userAgent)
		resp, err := c.http.Do(req)
		if err != nil {
			slog.Warn("analytics event", "name", name, "err", err)
			return
		}
		resp.Body.Close()
		if resp.StatusCode >= 300 {
			slog.Warn("analytics event", "name", name, "status", resp.StatusCode)
		}
	}()
}
