package mcpserver

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/stewartmckee/canvasbin/internal/store"
)

func gzipB64(s string) string {
	var buf bytes.Buffer
	zw, err := gzip.NewWriterLevel(&buf, gzip.BestCompression)
	if err != nil {
		panic(err)
	}
	if _, err := zw.Write([]byte(s)); err != nil {
		panic(err)
	}
	if err := zw.Close(); err != nil {
		panic(err)
	}
	return base64.StdEncoding.EncodeToString(buf.Bytes())
}

func TestPageContent(t *testing.T) {
	plain := "<p>Hello</p>"
	on := true
	off := false
	encoded := gzipB64(plain)
	wrapped := encoded[:8] + "\n" + encoded[8:]

	cases := []struct {
		name    string
		content string
		gzipped *bool
		want    string
		wantErr string
	}{
		{"omitted", plain, nil, plain, ""},
		{"false", plain, &off, plain, ""},
		{"gzipped", encoded, &on, plain, ""},
		{"wrapped base64", wrapped, &on, plain, ""},
		{"bad base64", "not base64!!!", &on, "", "standard base64"},
		{"not gzip", base64.StdEncoding.EncodeToString([]byte("hello")), &on, "", "not valid gzip"},
	}
	for _, tc := range cases {
		got, err := pageContent(tc.content, tc.gzipped)
		if tc.wantErr != "" {
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("%s: err = %v", tc.name, err)
			}
			continue
		}
		if err != nil || got != tc.want {
			t.Errorf("%s: got %q, %v", tc.name, got, err)
		}
	}

	huge := strings.Repeat("a", store.MaxContentBytes+1)
	if _, err := pageContent(gzipB64(huge), &on); !errors.Is(err, store.ErrTooLarge) {
		t.Errorf("oversized gzip: %v", err)
	}
}

func TestGzippedAdvertisedOnWriteTools(t *testing.T) {
	ctx := context.Background()
	srv := newServer(Options{Version: "test"}, "https://cb.example", "", nil)
	st, ct := mcp.NewInMemoryTransports()
	if _, err := srv.Connect(ctx, st, nil); err != nil {
		t.Fatal(err)
	}
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "1"}, nil).Connect(ctx, ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer cs.Close()
	if !strings.Contains(cs.InitializeResult().Instructions, "gzipped to true") {
		t.Fatal("instructions should tell the agent to gzip page source")
	}
	res, err := cs.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"create_page", "create_draft"} {
		var tool *mcp.Tool
		for _, candidate := range res.Tools {
			if candidate.Name == name {
				tool = candidate
				break
			}
		}
		if tool == nil {
			t.Fatalf("missing %s", name)
		}
		if !strings.Contains(tool.Description, "gzipped to true") || !strings.Contains(tool.Description, "plain HTML or Markdown") {
			t.Errorf("%s description: %s", name, tool.Description)
		}
		raw, err := json.Marshal(tool.InputSchema)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(raw), `"gzipped"`) {
			t.Errorf("%s schema missing gzipped: %s", name, raw)
		}
	}
	for _, tool := range res.Tools {
		if tool.Name == "publish_draft" && (!strings.Contains(tool.Description, "expected_bytes") || !strings.Contains(tool.Description, "Do not copy draft_bytes")) {
			t.Errorf("publish_draft description: %s", tool.Description)
		}
	}
}
