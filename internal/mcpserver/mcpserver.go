// Package mcpserver exposes Canvasbin pages as MCP tools over Streamable HTTP.
package mcpserver

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/stewartmckee/canvasbin/internal/analytics"
	"github.com/stewartmckee/canvasbin/internal/config"
	"github.com/stewartmckee/canvasbin/internal/keys"
	"github.com/stewartmckee/canvasbin/internal/links"
	"github.com/stewartmckee/canvasbin/internal/mail"
	"github.com/stewartmckee/canvasbin/internal/ratelimit"
	"github.com/stewartmckee/canvasbin/internal/store"
)

const instructions = `Canvasbin hosts HTML or Markdown pages at unguessable URLs. There are no accounts.

` + pageInstructions

const pageInstructions = `When it helps: Canvasbin suits output the user wants to open outside the chat, keep, share with other people, download as a PDF, or have updated later at the same link, such as reports, dashboards, diagrams, documentation, checklists and meeting notes. Publish when the user asks for a page, link, PDF or something they can share. If a long or visual result looks like something they would want to share, you can ask whether they want it published; don't publish without the user asking or agreeing. Short answers that belong in the conversation don't need a page.

A page URL's extension picks the view: /{key} opens the Canvasbin app, /{key}.html is the rendered page, /{key}.pdf downloads it as a PDF, and /{key}.md returns Markdown source for Markdown pages. create_page returns:
- html_url: the rendered page, view-only. Usually the best link to share.
- pdf_url: a PDF download of the rendered page.
- read_url: the app in read-only mode (preview and source).
- write_url: the browser editor. The key in it grants edit and delete, so share it only with collaborators.

You do not need the browser to make changes: call update_page or delete_page with the write key (or write_url) directly. Keep write_key if you will update the page later. Tools that take a "key" accept the bare key or any Canvasbin URL. HTML pages may be a fragment or a full document with <html>; scripts run in a sandbox.

For data the user wants to store outside the chat (cross-platform context, chat backups, scratch notes) rather than share as a polished page, create or update with agentic_only true. Those pages are hidden from the human library by default but appear in list_pages with agentic_only set. Passing team with agentic_only true still stores a personal scratchpad.`

type Options struct {
	Store         *store.Store
	PublicBaseURL string
	CreateLimit   *ratelimit.Limiter
	Version       string

	// Config carries account settings (quota, ACCOUNTS flag). The zero value
	// leaves every tool anonymous, exactly as before accounts existed.
	Config config.Config
	// UserFromRequest, when set and Config.Accounts is on, resolves the caller
	// (e.g. from an OAuth bearer token) so tools act as that user. It returns
	// nil for anonymous callers. The OAuth layer plugs in here.
	UserFromRequest func(r *http.Request) *store.User
	// ScopeFromRequest, when set, returns the OAuth scope of the request's bearer
	// token so the gate can enforce pages:read / pages:write. Nil skips the check.
	ScopeFromRequest func(r *http.Request) string

	// Mailer and MailLimit send and throttle share invitations (accounts on).
	Mailer    mail.Sender
	MailLimit *ratelimit.Limiter

	// Analytics, when set, records page creation by AI assistants.
	Analytics *analytics.Client
}

// Handler returns the Streamable HTTP handler mounted at /mcp.
//
// With accounts on and UserFromRequest set, an auth gate runs first and every
// request needs a bearer token (see gate).
func Handler(o Options) http.Handler {
	h := http.Handler(mcp.NewStreamableHTTPHandler(func(r *http.Request) *mcp.Server {
		var u *store.User
		if o.Config.Accounts && o.UserFromRequest != nil {
			// The gate already resolved the token for authenticated calls.
			if cu, ok := r.Context().Value(userCtxKey{}).(*store.User); ok {
				u = cu
			} else {
				u = o.UserFromRequest(r)
			}
		}
		return newServer(o, links.Base(o.PublicBaseURL, r), links.ClientIP(r), u)
	}, &mcp.StreamableHTTPOptions{
		Stateless:           true,
		JSONResponse:        true,
		MaxRequestBodyBytes: 3 * store.MaxContentBytes,
	}))
	if o.Config.Accounts && o.UserFromRequest != nil {
		h = o.gate(h)
	}
	return h
}

type tools struct {
	Options
	base     string
	clientIP string
	user     *store.User // nil for anonymous callers
}

func newServer(o Options, base, clientIP string, user *store.User) *mcp.Server {
	text := instructions
	if o.Config.Accounts {
		text = accountInstructions
	}
	s := mcp.NewServer(&mcp.Implementation{
		Name:        "canvasbin",
		Title:       "Canvasbin",
		Description: "Publish HTML and Markdown pages at shareable links, and keep them up to date.",
		Version:     o.Version,
		WebsiteURL:  base,
		Icons: []mcp.Icon{
			{Source: base + "/favicon.svg", MIMEType: "image/svg+xml", Sizes: []string{"any"}},
			{Source: base + "/icon-512.png", MIMEType: "image/png", Sizes: []string{"512x512"}},
		},
	}, &mcp.ServerOptions{Instructions: text})
	t := &tools{Options: o, base: base, clientIP: clientIP, user: user}

	createDesc := "Publish a new HTML or Markdown page. Returns html_url (rendered, view-only), pdf_url, read_url (read-only app view) and write_url (browser editor), plus the keys."
	updateDesc := "Edit a page directly: change its content, title or format. Requires the write key or write URL. Omitted fields are left unchanged; content replaces the whole page. Changes are live on the page URLs immediately."
	if o.Config.Accounts {
		createDesc += " When the page is for a team, call get_style_guide for that team before you write it, and follow the style guide when one is set. Style guides are a Team plan feature; if the guide is empty or the plan is not active, write the page as usual."
		updateDesc += " When the page belongs to a team, call get_style_guide for that team if you have not already, and keep following the guide when one is set."
	}
	mcp.AddTool(s, &mcp.Tool{
		Name:        "create_page",
		Title:       "Create page",
		Description: createDesc,
		Annotations: writeTool(false, false, false),
	}, t.create)
	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_page",
		Title:       "Get page",
		Description: "Fetch a page's title, format and source content using its read key, write key, or either URL.",
		Annotations: readTool(),
	}, t.get)
	mcp.AddTool(s, &mcp.Tool{
		Name:        "update_page",
		Title:       "Update page",
		Description: updateDesc,
		Annotations: writeTool(true, true, false),
	}, t.update)
	mcp.AddTool(s, &mcp.Tool{
		Name:        "delete_page",
		Title:       "Delete page",
		Description: "Permanently delete a page. Requires the write key or write URL. Both links stop working.",
		Annotations: writeTool(true, true, false),
	}, t.remove)
	if o.Config.Accounts {
		t.addAccountTools(s)
	}
	t.addUpdateTools(s)
	return s
}

type CreateInput struct {
	Content     string `json:"content" jsonschema:"page source: HTML (fragment or full document) or Markdown"`
	Format      string `json:"format,omitempty" jsonschema:"html (default) or markdown"`
	Title       string `json:"title,omitempty" jsonschema:"title shown in the browser tab"`
	Team        string `json:"team,omitempty" jsonschema:"name or id of one of the user's teams to create the page in; omit for a personal page"`
	AgenticOnly *bool  `json:"agentic_only,omitempty" jsonschema:"when true, creates an agent scratchpad (hidden from the web library by default); team is ignored and the page is stored as personal"`
}

type GetInput struct {
	Key string `json:"key" jsonschema:"read key, write key, or the full read/write URL"`
}

type UpdateInput struct {
	Key         string  `json:"key" jsonschema:"write key or write URL"`
	Content     *string `json:"content,omitempty" jsonschema:"new full page source; replaces the existing content"`
	Format      *string `json:"format,omitempty" jsonschema:"html or markdown"`
	Title       *string `json:"title,omitempty" jsonschema:"new title"`
	AgenticOnly *bool   `json:"agentic_only,omitempty" jsonschema:"mark or unmark the page as an agent scratchpad"`
	Notify      *bool   `json:"notify,omitempty" jsonschema:"when updates use notify_on_page_edit mode, email subscribers about this edit"`
}

type DeleteInput struct {
	Key string `json:"key" jsonschema:"write key or write URL"`
}

type PageResult struct {
	Title       string    `json:"title"`
	Format      string    `json:"format"`
	Content     *string   `json:"content,omitempty"`
	AgenticOnly bool      `json:"agentic_only"`
	CanWrite    bool      `json:"can_write"`
	ReadURL     string    `json:"read_url"`
	WriteURL    string    `json:"write_url,omitempty"`
	HTMLURL     string    `json:"html_url"`
	PDFURL      string    `json:"pdf_url"`
	RawURL      string    `json:"raw_url"`
	ReadKey     string    `json:"read_key"`
	WriteKey    string    `json:"write_key,omitempty"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type DeleteResult struct {
	Deleted bool `json:"deleted"`
}

func (t *tools) result(a *store.Access, withContent bool) PageResult {
	l := links.For(t.base, a.ReadKey, a.WriteKey)
	r := PageResult{
		Title: a.Title, Format: a.Format, AgenticOnly: a.AgenticOnly, CanWrite: a.CanWrite,
		ReadURL: l.ReadURL, WriteURL: l.WriteURL, HTMLURL: l.HTMLURL, PDFURL: l.PDFURL, RawURL: l.RawURL,
		ReadKey: a.ReadKey, WriteKey: a.WriteKey, UpdatedAt: a.UpdatedAt,
	}
	if withContent {
		c := a.Content
		r.Content = &c
	}
	return r
}

func (t *tools) create(ctx context.Context, _ *mcp.CallToolRequest, in CreateInput) (*mcp.CallToolResult, PageResult, error) {
	if t.CreateLimit != nil && !t.CreateLimit.Allow(t.clientIP) {
		return nil, PageResult{}, errors.New("too many pages created, try again shortly")
	}
	var (
		a   *store.Access
		err error
	)
	if t.Config.Accounts && t.user == nil {
		return nil, PageResult{}, errors.New("sign in to create pages")
	}
	agentic := in.AgenticOnly != nil && *in.AgenticOnly
	if t.user != nil && in.Team != "" && !agentic {
		team, terr := t.teamForCreate(ctx, in.Team)
		if terr != nil {
			return nil, PageResult{}, terr
		}
		a, err = t.Store.CreateInTeam(ctx, t.user.ID, team.ID, in.Title, in.Format, in.Content, false)
		if err == nil {
			t.logPage(ctx, a, store.ActivityPageCreated)
		}
	} else if t.user != nil {
		if err := t.checkQuota(ctx); err != nil {
			return nil, PageResult{}, err
		}
		a, err = t.Store.CreateOwned(ctx, t.user.ID, in.Title, in.Format, in.Content, agentic)
	} else {
		a, err = t.Store.Create(ctx, in.Title, in.Format, in.Content, agentic)
	}
	if err != nil {
		return nil, PageResult{}, err
	}
	if t.user != nil && t.Analytics.Enabled() {
		n, _ := t.Store.CountCreated(ctx, t.user.ID)
		t.Analytics.PageCreated("mcp", a.Format, a.TeamID != nil, n)
	}
	return nil, t.result(a, false), nil
}

func (t *tools) get(ctx context.Context, _ *mcp.CallToolRequest, in GetInput) (*mcp.CallToolResult, PageResult, error) {
	key, err := keys.Parse(in.Key)
	if err != nil {
		return nil, PageResult{}, store.ErrNotFound
	}
	var a *store.Access
	if t.user != nil {
		a, err = t.Store.GetForUser(ctx, key, t.user.ID)
	} else {
		a, err = t.Store.Get(ctx, key)
	}
	if err != nil {
		return nil, PageResult{}, err
	}
	return nil, t.result(a, true), nil
}

func (t *tools) update(ctx context.Context, _ *mcp.CallToolRequest, in UpdateInput) (*mcp.CallToolResult, PageResult, error) {
	key, err := keys.Parse(in.Key)
	if err != nil {
		return nil, PageResult{}, store.ErrNotFound
	}
	if in.Content == nil && in.Format == nil && in.Title == nil && in.AgenticOnly == nil {
		return nil, PageResult{}, errors.New("nothing to update: pass content, title, format or agentic_only")
	}
	changes := store.Changes{Title: in.Title, Format: in.Format, Content: in.Content, AgenticOnly: in.AgenticOnly}
	var a *store.Access
	if t.user != nil {
		a, err = t.Store.UpdateForUser(ctx, key, t.user.ID, changes)
		if err == nil {
			t.logPage(ctx, a, store.ActivityPageEdited)
		}
	} else {
		a, err = t.Store.Update(ctx, key, changes)
	}
	if err != nil {
		return nil, PageResult{}, err
	}
	t.maybeNotifyPageEdit(ctx, a, in.Notify != nil && *in.Notify)
	return nil, t.result(a, false), nil
}

func (t *tools) maybeNotifyPageEdit(ctx context.Context, a *store.Access, notify bool) {
	if !t.Config.Updates || t.Config.UpdatesFeed() || !notify || t.Mailer == nil {
		return
	}
	if !t.Config.PageHasUpdates(ctx, t.Store, a.Page) {
		return
	}
	on, err := t.Store.UpdatesEnabled(ctx, a.ID)
	if err != nil || !on {
		return
	}
	title := a.Title
	if title == "" {
		title = "Page updated"
	}
	u, err := t.Store.CreatePageUpdate(ctx, a.ID, nil, title, "The page has been updated.")
	if err != nil {
		return
	}
	_, _ = t.Store.EnqueueUpdateNotifications(ctx, u.ID, a.ID, a.ReadKey)
}

func (t *tools) remove(ctx context.Context, _ *mcp.CallToolRequest, in DeleteInput) (*mcp.CallToolResult, DeleteResult, error) {
	key, err := keys.Parse(in.Key)
	if err != nil {
		return nil, DeleteResult{}, store.ErrNotFound
	}
	if t.user != nil {
		var a *store.Access
		a, err = t.Store.DeleteForUser(ctx, key, t.user.ID)
		if err == nil {
			t.logPage(ctx, a, store.ActivityPageDeleted)
		}
	} else {
		err = t.Store.Delete(ctx, key)
	}
	if err != nil {
		return nil, DeleteResult{}, err
	}
	return nil, DeleteResult{Deleted: true}, nil
}

func ptr[T any](v T) *T { return &v }

// Tool annotations. Directories require every hint to be explicit. Tools stay
// within Canvasbin (openWorld false) except sharing, which emails anyone.
func readTool() *mcp.ToolAnnotations {
	return &mcp.ToolAnnotations{ReadOnlyHint: true, DestructiveHint: ptr(false), IdempotentHint: true, OpenWorldHint: ptr(false)}
}

func writeTool(destructive, idempotent, openWorld bool) *mcp.ToolAnnotations {
	return &mcp.ToolAnnotations{DestructiveHint: ptr(destructive), IdempotentHint: idempotent, OpenWorldHint: ptr(openWorld)}
}
