package httpapi

import (
	"encoding/json"
	"io/fs"
	"log"

	"github.com/stewartmckee/canvasbin/web"
)

// marketingPaths lists public URLs for sitemap.xml and indexability. Keep in
// sync with web/marketing-paths.json (copied into dist at build time).
var marketingPaths = mustMarketingPaths()

func mustMarketingPaths() []string {
	raw, err := fs.ReadFile(web.Dist(), "marketing-paths.json")
	if err != nil {
		log.Printf("httpapi: marketing-paths.json missing from web embed, using defaults: %v", err)
		return defaultMarketingPaths()
	}
	var paths []string
	if err := json.Unmarshal(raw, &paths); err != nil {
		log.Printf("httpapi: invalid marketing-paths.json: %v", err)
		return defaultMarketingPaths()
	}
	return paths
}

func defaultMarketingPaths() []string {
	return []string{
		"/", "/features", "/pricing", "/privacy", "/terms", "/contact",
		"/use-cases", "/use-cases/developers", "/use-cases/business", "/use-cases/family",
		"/use-cases/news-apps-games",
		"/connect", "/connect/claude", "/connect/chatgpt", "/connect/cursor",
		"/agent-scratchpad", "/agent-scratchpad/share-context-across-platforms", "/agent-scratchpad/backup-ai-chats",
	}
}
