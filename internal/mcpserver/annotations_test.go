package mcpserver

import (
	"context"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/stewartmckee/canvasbin/internal/config"
	"github.com/stewartmckee/canvasbin/internal/store"
)

// The Claude and ChatGPT directories reject tools without a title and explicit
// readOnly, destructive and openWorld hints.
func TestToolAnnotations(t *testing.T) {
	ctx := context.Background()
	o := Options{Config: config.Config{Accounts: true}, Version: "test"}
	srv := newServer(o, "https://cb.example", "", &store.User{ID: 1})
	st, ct := mcp.NewInMemoryTransports()
	if _, err := srv.Connect(ctx, st, nil); err != nil {
		t.Fatal(err)
	}
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "1"}, nil).Connect(ctx, ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer cs.Close()

	res, err := cs.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	readOnly := map[string]bool{"get_page": true, "list_pages": true, "list_teams": true, "list_comments": true}
	destructive := map[string]bool{"update_page": true, "delete_page": true, "share_page": true, "unshare_page": true}
	if len(res.Tools) != 11 {
		t.Errorf("got %d tools, want 11", len(res.Tools))
	}
	for _, tool := range res.Tools {
		a := tool.Annotations
		if tool.Title == "" || a == nil || a.DestructiveHint == nil || a.OpenWorldHint == nil {
			t.Errorf("%s: missing title or hints: %+v", tool.Name, a)
			continue
		}
		if a.ReadOnlyHint != readOnly[tool.Name] {
			t.Errorf("%s: readOnlyHint = %v", tool.Name, a.ReadOnlyHint)
		}
		if *a.DestructiveHint != destructive[tool.Name] {
			t.Errorf("%s: destructiveHint = %v", tool.Name, *a.DestructiveHint)
		}
		if *a.OpenWorldHint != (tool.Name == "share_page") {
			t.Errorf("%s: openWorldHint = %v", tool.Name, *a.OpenWorldHint)
		}
	}
	init := cs.InitializeResult()
	if init.ServerInfo.WebsiteURL != "https://cb.example" || len(init.ServerInfo.Icons) == 0 {
		t.Errorf("server info: %+v", init.ServerInfo)
	}
}
