package mcpserver

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Starter prompts surfaced in Claude's prompt menu (and on /connect).
var mcpPrompts = []struct {
	name, title, desc, text string
}{
	{
		name:  "publish_tic_tac_toe",
		title: "Publish a tic-tac-toe game",
		desc:  "Create a playable tic-tac-toe page on Canvasbin",
		text:  "Create a new canvasbin and put a game of tic tac toe on it.",
	},
	{
		name:  "publish_agentic_newsroom",
		title: "Agentic newsroom SPA",
		desc:  "Newsroom-style single-page app with navigation and articles",
		text:  "Create a canvasbin with the latest agentic news, create it in a newsroom style and make it a single page application with navigation and in depth articles.",
	},
	{
		name:  "publish_this_conversation",
		title: "Publish this conversation",
		desc:  "Turn the current chat into a shareable Canvasbin page",
		text:  "Publish the main content of this conversation as a polished Canvasbin HTML page. Give me the html_url and pdf_url when done.",
	},
}

func (t *tools) addPrompts(s *mcp.Server) {
	if !t.Config.Accounts || t.Config.SignalProduct() {
		return
	}
	for _, p := range mcpPrompts {
		text := p.text
		s.AddPrompt(&mcp.Prompt{
			Name:        p.name,
			Title:       p.title,
			Description: p.desc,
		}, func(_ context.Context, _ *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
			return &mcp.GetPromptResult{
				Description: p.desc,
				Messages: []*mcp.PromptMessage{{
					Role:    "user",
					Content: &mcp.TextContent{Text: text},
				}},
			}, nil
		})
	}
}
