package analytics

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestEvent(t *testing.T) {
	got := make(chan map[string]any, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/send" || r.Method != http.MethodPost {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		got <- body
	}))
	defer srv.Close()

	New(srv.URL+"/", "site-1", "https://example.com").Event("/mcp", "page-create", map[string]any{"source": "mcp"})
	select {
	case body := <-got:
		p, _ := body["payload"].(map[string]any)
		if body["type"] != "event" || p["website"] != "site-1" || p["hostname"] != "example.com" ||
			p["url"] != "/mcp" || p["name"] != "page-create" {
			t.Fatalf("payload = %v", body)
		}
		if d, _ := p["data"].(map[string]any); d["source"] != "mcp" {
			t.Fatalf("data = %v", p["data"])
		}
	case <-time.After(3 * time.Second):
		t.Fatal("no event sent")
	}
}

func TestDisabled(t *testing.T) {
	if New("", "site", "") != nil || New("https://x", "", "") != nil {
		t.Fatal("expected nil client without URL and website")
	}
	var c *Client
	c.Event("/", "x", nil)
}
