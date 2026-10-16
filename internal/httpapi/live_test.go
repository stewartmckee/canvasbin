package httpapi

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
)

func TestLiveUpdates(t *testing.T) {
	e := newEnv(t, false, 20)
	c := e.newClient(t)
	p := c.want(201, "POST", "/api/pages", map[string]any{"title": "Live", "content": "<p>one</p>"})
	readKey, writeKey := p["read_key"].(string), p["write_key"].(string)

	resp, err := http.Get(e.srv.URL + "/" + readKey + ".html")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if !strings.Contains(string(body), "data-canvasbin-live") || !strings.Contains(string(body), readKey) {
		t.Fatal("rendered page should include the live reload script")
	}
	resp, err = http.Get(e.srv.URL + "/" + readKey + ".html?live=0")
	if err != nil {
		t.Fatal(err)
	}
	body, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	if strings.Contains(string(body), "data-canvasbin-live") {
		t.Fatal("live=0 should leave the page untouched")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	wsURL := "ws" + strings.TrimPrefix(e.srv.URL, "http") + "/api/pages/" + readKey + "/live"
	conn, _, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{HTTPHeader: http.Header{"Origin": {"null"}}})
	if err != nil {
		t.Fatal(err)
	}
	defer conn.CloseNow()

	var m liveMessage
	if err := wsjson.Read(ctx, conn, &m); err != nil || m.Type != "hello" || m.UpdatedAt == "" {
		t.Fatalf("hello: %v %+v", err, m)
	}
	hello := m.UpdatedAt

	// The database listener may still be starting, so retry the edit until an
	// event arrives.
	msgs := make(chan liveMessage, 8)
	go func() {
		for {
			var m liveMessage
			if wsjson.Read(ctx, conn, &m) != nil {
				close(msgs)
				return
			}
			msgs <- m
		}
	}()
	var got liveMessage
	for i := 0; got.Type == "" && i < 10; i++ {
		c.want(200, "PUT", "/api/pages/"+writeKey, map[string]any{"content": "<p>v" + string(rune('a'+i)) + "</p>"})
		select {
		case got = <-msgs:
		case <-time.After(time.Second):
		}
	}
	if got.Type != "updated" || got.UpdatedAt == hello {
		t.Fatalf("expected an update event, got %+v", got)
	}

	c.want(204, "DELETE", "/api/pages/"+writeKey, nil)
	deadline := time.After(5 * time.Second)
	for got.Type != "deleted" {
		select {
		case got = <-msgs:
			if got.Type != "deleted" && got.Type != "updated" {
				t.Fatalf("expected deleted, got %+v", got)
			}
		case <-deadline:
			t.Fatal("no deleted event")
		}
	}

	if _, _, err := websocket.Dial(ctx, wsURL, nil); err == nil {
		t.Fatal("subscribing to a deleted page should fail")
	}
}
