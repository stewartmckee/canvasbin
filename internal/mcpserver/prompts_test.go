package mcpserver

import (
	"context"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/stewartmckee/canvasbin/internal/config"
	"github.com/stewartmckee/canvasbin/internal/store"
)

func TestStarterPromptsRegistered(t *testing.T) {
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

	res, err := cs.ListPrompts(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Prompts) != len(mcpPrompts) {
		t.Fatalf("prompts: got %d want %d", len(res.Prompts), len(mcpPrompts))
	}
	got, err := cs.GetPrompt(ctx, &mcp.GetPromptParams{Name: "publish_tic_tac_toe"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Messages) != 1 || got.Messages[0].Content == nil {
		t.Fatalf("unexpected prompt result: %+v", got)
	}
}
