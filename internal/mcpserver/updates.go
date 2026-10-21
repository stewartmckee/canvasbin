package mcpserver

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/stewartmckee/canvasbin/internal/keys"
	"github.com/stewartmckee/canvasbin/internal/store"
)

// updateInstructions is appended to the server instructions when page updates are on.
const updateInstructions = `

Page updates are emailed to every subscriber when they are published, and a published update can't be recalled. Before you call post_update (or update_page with notify: true), review it with the user:
1. Show the exact title and body you will post, and the page it will go on.
2. Ask whether to publish it now or schedule it for later. For later, agree a date and time and confirm the timezone, then pass publish_at as an ISO 8601 time with that offset (for example 2026-10-22T18:00:00+01:00).
3. Post only once they approve the wording and the timing. If they ask for changes, show the revised draft again.
Don't treat the original request to post an update as approval of your draft, because you may have added or changed details. A scheduled update stays hidden and unsent until its time; list_scheduled_updates shows what's waiting and cancel_scheduled_update removes one before it goes out.`

func (t *tools) addUpdateTools(s *mcp.Server) {
	if !t.Config.Updates {
		return
	}
	mcp.AddTool(s, &mcp.Tool{
		Name:        "post_update",
		Title:       "Post page update",
		Description: "Publish an append-only update (title and Markdown body) on a page with updates enabled, now or at a scheduled publish_at. Subscribers are emailed when it is published, and a published update can't be recalled. Before calling, show the user the exact title, body and page, ask whether to publish now or later, and call only after they approve the wording and timing. Updates need the page owner to be on the Signal plan.",
		Annotations: writeTool(false, false, false),
	}, t.postUpdate)
	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_scheduled_updates",
		Title:       "List scheduled updates",
		Description: "List a page's updates that are scheduled but not yet published, soonest first. Requires the write key.",
		Annotations: readTool(),
	}, t.listScheduledUpdates)
	mcp.AddTool(s, &mcp.Tool{
		Name:        "cancel_scheduled_update",
		Title:       "Cancel scheduled update",
		Description: "Delete a scheduled update before it is published, so it never appears or gets emailed. Published updates can't be cancelled. Requires the write key.",
		Annotations: writeTool(true, true, false),
	}, t.cancelScheduledUpdate)
	mcp.AddTool(s, &mcp.Tool{
		Name:        "set_updates_enabled",
		Title:       "Enable page updates",
		Description: "Turn public updates and email subscriptions on or off for a page. Requires the write key, and turning updates on needs the page owner to be on the Signal plan.",
		Annotations: writeTool(true, true, false),
	}, t.setUpdatesEnabled)
}

type PostUpdateInput struct {
	Key       string `json:"key" jsonschema:"write key or write URL"`
	Title     string `json:"title"`
	Body      string `json:"body" jsonschema:"Markdown body"`
	PublishAt string `json:"publish_at,omitempty" jsonschema:"when to publish, as ISO 8601 with a timezone offset (e.g. 2026-10-22T18:00:00+01:00); omit to publish now. Up to a year ahead."`
}

type PostUpdateResult struct {
	ID          int64     `json:"id"`
	Title       string    `json:"title"`
	HTMLURL     string    `json:"html_url"`
	Scheduled   bool      `json:"scheduled"`
	PublishedAt time.Time `json:"publish_at"`
	Queued      int       `json:"notifications_queued" jsonschema:"emails queued now; 0 for a scheduled update, whose subscribers are emailed when it publishes"`
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
	var at time.Time
	if s := strings.TrimSpace(in.PublishAt); s != "" {
		at, err = time.Parse(time.RFC3339, s)
		if err != nil {
			return nil, PostUpdateResult{}, errors.New("publish_at must be ISO 8601 with a timezone offset, e.g. 2026-10-22T18:00:00+01:00")
		}
	}
	u, err := t.Store.CreatePageUpdate(ctx, a.ID, uid, in.Title, in.Body, a.ReadKey, at)
	if err != nil {
		return nil, PostUpdateResult{}, err
	}
	queued := 0
	if t.Mailer != nil && !u.Scheduled {
		queued, _ = t.Store.EnqueueUpdateNotifications(ctx, u.ID, a.ID, a.ReadKey)
	}
	l := t.result(a, false)
	return nil, PostUpdateResult{
		ID: u.ID, Title: u.Title,
		HTMLURL:     l.HTMLURL + "#update-" + fmt.Sprint(u.ID),
		Scheduled:   u.Scheduled,
		PublishedAt: u.PublishedAt,
		Queued:      queued,
	}, nil
}

type ScheduledUpdatesInput struct {
	Key string `json:"key" jsonschema:"write key or write URL"`
}

type ScheduledUpdate struct {
	ID          int64     `json:"id"`
	Title       string    `json:"title"`
	Body        string    `json:"body"`
	PublishedAt time.Time `json:"publish_at"`
}

type ScheduledUpdatesResult struct {
	Updates []ScheduledUpdate `json:"updates"`
}

func (t *tools) listScheduledUpdates(ctx context.Context, _ *mcp.CallToolRequest, in ScheduledUpdatesInput) (*mcp.CallToolResult, ScheduledUpdatesResult, error) {
	a, err := t.updatesPage(ctx, in.Key)
	if err != nil {
		return nil, ScheduledUpdatesResult{}, err
	}
	list, err := t.Store.ListScheduledUpdates(ctx, a.ID)
	if err != nil {
		return nil, ScheduledUpdatesResult{}, err
	}
	out := ScheduledUpdatesResult{Updates: []ScheduledUpdate{}}
	for _, u := range list {
		out.Updates = append(out.Updates, ScheduledUpdate{ID: u.ID, Title: u.Title, Body: u.Body, PublishedAt: u.PublishedAt})
	}
	return nil, out, nil
}

type CancelScheduledUpdateInput struct {
	Key string `json:"key" jsonschema:"write key or write URL"`
	ID  int64  `json:"id" jsonschema:"id of the scheduled update, from post_update or list_scheduled_updates"`
}

type CancelScheduledUpdateResult struct {
	Cancelled bool `json:"cancelled"`
}

func (t *tools) cancelScheduledUpdate(ctx context.Context, _ *mcp.CallToolRequest, in CancelScheduledUpdateInput) (*mcp.CallToolResult, CancelScheduledUpdateResult, error) {
	a, err := t.updatesPage(ctx, in.Key)
	if err != nil {
		return nil, CancelScheduledUpdateResult{}, err
	}
	if err := t.Store.CancelScheduledUpdate(ctx, a.ID, in.ID); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, CancelScheduledUpdateResult{}, errors.New("no scheduled update with that id on this page; it may already be published")
		}
		return nil, CancelScheduledUpdateResult{}, err
	}
	return nil, CancelScheduledUpdateResult{Cancelled: true}, nil
}

func (t *tools) updatesPage(ctx context.Context, key string) (*store.Access, error) {
	k, err := keys.Parse(key)
	if err != nil {
		return nil, store.ErrNotFound
	}
	return t.pageWrite(ctx, k)
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
