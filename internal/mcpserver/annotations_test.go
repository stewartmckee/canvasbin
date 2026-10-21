package mcpserver

import (
	"context"
	"strings"
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
	readOnly := map[string]bool{
		"get_page": true, "list_pages": true, "list_teams": true, "list_comments": true, "get_style_guide": true,
	}
	destructive := map[string]bool{
		"update_page": true, "delete_page": true, "share_page": true, "unshare_page": true, "set_style_guide": true,
	}
	if len(res.Tools) != 13 {
		t.Errorf("got %d tools, want 13", len(res.Tools))
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
		if tool.Name == "create_page" && !strings.Contains(tool.Description, "get_style_guide") {
			t.Errorf("create_page should tell the agent to read the style guide: %s", tool.Description)
		}
		if tool.Name == "get_style_guide" && !strings.Contains(tool.Description, "Team plan") {
			t.Errorf("get_style_guide should name the Team plan: %s", tool.Description)
		}
	}
	init := cs.InitializeResult()
	if init.ServerInfo.WebsiteURL != "https://cb.example" || len(init.ServerInfo.Icons) == 0 {
		t.Errorf("server info: %+v", init.ServerInfo)
	}
	if !strings.Contains(init.Instructions, "get_style_guide") || !strings.Contains(init.Instructions, "Team plan") {
		t.Errorf("instructions should tell the agent to read a team style guide: %s", init.Instructions)
	}
}

// Update emails can't be recalled, so the agent must get the wording approved
// first. Only accounts whose plan includes updates see the tools at all.
func TestUpdateInstructionsRequireReview(t *testing.T) {
	ctx := context.Background()
	price := func(s string) *string { return &s }
	signal := &store.User{ID: 1, SubscriptionStatus: "active", StripePriceID: price("price_signal")}
	pro := &store.User{ID: 2, SubscriptionStatus: "active", StripePriceID: price("price_pro")}
	cases := []struct {
		name    string
		updates bool
		user    *store.User
		want    bool
	}{
		{"signal", true, signal, true},
		{"updates off", false, signal, false},
		{"individual plan", true, pro, false},
		{"anonymous", true, nil, false},
	}
	for _, tc := range cases {
		on := tc.want
		o := Options{Config: config.Config{Accounts: true, Updates: tc.updates, StripePriceSignal: "price_signal"}, Version: "test"}
		srv := newServer(o, "https://cb.example", "", tc.user)
		st, ct := mcp.NewInMemoryTransports()
		if _, err := srv.Connect(ctx, st, nil); err != nil {
			t.Fatal(err)
		}
		cs, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "1"}, nil).Connect(ctx, ct, nil)
		if err != nil {
			t.Fatal(err)
		}
		if got := strings.Contains(cs.InitializeResult().Instructions, "publish it now or schedule it"); got != on {
			t.Errorf("%s: review instruction present = %v", tc.name, got)
		}
		res, err := cs.ListTools(ctx, nil)
		if err != nil {
			t.Fatal(err)
		}
		listed := false
		for _, tool := range res.Tools {
			if tool.Name == "post_update" {
				listed = true
				if !strings.Contains(tool.Description, "approve") {
					t.Errorf("post_update should require approval: %s", tool.Description)
				}
			}
			if strings.Contains(tool.Description, "Signal") {
				t.Errorf("%s: %s names the unlisted Signal plan", tc.name, tool.Name)
			}
		}
		if listed != on {
			t.Errorf("%s: post_update listed = %v, want %v", tc.name, listed, on)
		}
		cs.Close()
	}
}

func TestAnonymousToolsOmitStyleGuide(t *testing.T) {
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
	res, err := cs.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(cs.InitializeResult().Instructions, "style guide") {
		t.Fatal("anonymous server should not mention style guides")
	}
	for _, tool := range res.Tools {
		if tool.Name == "get_style_guide" || tool.Name == "set_style_guide" {
			t.Fatalf("anonymous server listed %s", tool.Name)
		}
		if tool.Name == "create_page" && strings.Contains(tool.Description, "style guide") {
			t.Fatalf("anonymous create_page mentions a style guide: %s", tool.Description)
		}
	}
}
