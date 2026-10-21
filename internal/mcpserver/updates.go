package mcpserver

import (
	"context"
	"errors"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/stewartmckee/canvasbin/internal/keys"
	"github.com/stewartmckee/canvasbin/internal/store"
)

// updateInstructions is appended to the server instructions when page updates are on.
const updateInstructions = `

Page updates are emailed to every subscriber as soon as they are posted and can't be recalled. Before you call post_update (or update_page with notify: true), show the user the exact title and body you will post, and the page it will go on, and ask them to confirm. Only post once they approve that wording; if they ask for changes, show the revised draft again. Don't treat the original request to post an update as approval of your draft, because you may have added or changed details.`

func (t *tools) addUpdateTools(s *mcp.Server) {
	if !t.Config.Updates {
		return
	}
	mcp.AddTool(s, &mcp.Tool{
		Name:        "post_update",
		Title:       "Post page update",
		Description: "Publish an append-only update (title and Markdown body) on a page with updates enabled. Subscribers receive email when configured, and a posted update can't be recalled. Before calling, show the user the exact title and body and the page it will go on, and post only after they approve that wording. Updates need the page owner to be on the Signal plan.",
		Annotations: writeTool(false, false, false),
	}, t.postUpdate)
	mcp.AddTool(s, &mcp.Tool{
		Name:        "set_updates_enabled",
		Title:       "Enable page updates",
		Description: "Turn public updates and email subscriptions on or off for a page. Requires the write key, and turning updates on needs the page owner to be on the Signal plan.",
		Annotations: writeTool(true, true, false),
	}, t.setUpdatesEnabled)
}

type PostUpdateInput struct {
	Key   string `json:"key" jsonschema:"write key or write URL"`
	Title string `json:"title"`
	Body  string `json:"body" jsonschema:"Markdown body"`
}

type PostUpdateResult struct {
	ID      int64  `json:"id"`
	Title   string `json:"title"`
	HTMLURL string `json:"html_url"`
	Queued  int    `json:"notifications_queued"`
}

func (t *tools) postUpdate(ctx context.Context, _ *mcp.CallToolRequest, in PostUpdateInput) (*mcp.CallToolResult, PostUpdateResult, error) {
	if !t.Config.UpdatesFeed() {
		return nil, PostUpdateResult{}, errors.New("post_update is not available in notify_on_page_edit mode; use update_page with notify: true")
	}
	key, err := keys.Parse(in.Key)
	if err != nil {
		return nil, PostUpdateResult{}, store.ErrNotFound
	}
	a, err := t.pageWrite(ctx, key)
	if err != nil {
		return nil, PostUpdateResult{}, err
	}
	if !t.Config.PageHasUpdates(ctx, t.Store, a.Page) {
		return nil, PostUpdateResult{}, t.Config.UpdatesPlanError(t.base)
	}
	var uid *int64
	if t.user != nil {
		uid = &t.user.ID
	}
	u, err := t.Store.CreatePageUpdate(ctx, a.ID, uid, in.Title, in.Body)
	if err != nil {
		return nil, PostUpdateResult{}, err
	}
	queued := 0
	if t.Mailer != nil {
		queued, _ = t.Store.EnqueueUpdateNotifications(ctx, u.ID, a.ID, a.ReadKey)
	}
	l := t.result(a, false)
	return nil, PostUpdateResult{
		ID: u.ID, Title: u.Title,
		HTMLURL: l.HTMLURL + "#update-" + fmt.Sprint(u.ID),
		Queued:  queued,
	}, nil
}

type SetUpdatesEnabledInput struct {
	Key     string `json:"key" jsonschema:"write key or write URL"`
	Enabled bool   `json:"enabled"`
}

type SetUpdatesEnabledResult struct {
	UpdatesEnabled  bool `json:"updates_enabled"`
	SubscriberCount int  `json:"subscriber_count"`
}

func (t *tools) setUpdatesEnabled(ctx context.Context, _ *mcp.CallToolRequest, in SetUpdatesEnabledInput) (*mcp.CallToolResult, SetUpdatesEnabledResult, error) {
	key, err := keys.Parse(in.Key)
	if err != nil {
		return nil, SetUpdatesEnabledResult{}, store.ErrNotFound
	}
	a, err := t.pageWrite(ctx, key)
	if err != nil {
		return nil, SetUpdatesEnabledResult{}, err
	}
	if in.Enabled && !t.Config.PageHasUpdates(ctx, t.Store, a.Page) {
		return nil, SetUpdatesEnabledResult{}, t.Config.UpdatesPlanError(t.base)
	}
	if err := t.Store.SetUpdatesEnabled(ctx, a.ID, in.Enabled); err != nil {
		return nil, SetUpdatesEnabledResult{}, err
	}
	n, _ := t.Store.ActiveSubscriberCount(ctx, a.ID)
	return nil, SetUpdatesEnabledResult{UpdatesEnabled: in.Enabled, SubscriberCount: n}, nil
}

func (t *tools) pageWrite(ctx context.Context, key string) (*store.Access, error) {
	var a *store.Access
	var err error
	if t.user != nil {
		a, err = t.Store.GetForUser(ctx, key, t.user.ID)
	} else {
		a, err = t.Store.Get(ctx, key)
	}
	if err != nil {
		return nil, err
	}
	if !a.CanWrite {
		return nil, store.ErrReadOnly
	}
	return a, nil
}
