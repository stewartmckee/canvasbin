// Package mcpserver exposes Canvasbin pages as MCP tools over Streamable HTTP.
package mcpserver

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/stewartmckee/canvasbin/internal/keys"
	"github.com/stewartmckee/canvasbin/internal/links"
	"github.com/stewartmckee/canvasbin/internal/ratelimit"
	"github.com/stewartmckee/canvasbin/internal/store"
)

const instructions = `Canvasbin hosts HTML or Markdown pages at unguessable URLs. There are no accounts.

A page URL's extension picks the view: /{key} opens the Canvasbin app, /{key}.html is the rendered page, /{key}.pdf downloads it as a PDF, and /{key}.md returns Markdown source for Markdown pages. create_page returns:
- html_url: the rendered page, view-only. Usually the best link to share.
- pdf_url: a PDF download of the rendered page.
- read_url: the app in read-only mode (preview and source).
- write_url: the browser editor. The key in it grants edit and delete, so share it only with collaborators.

You do not need the browser to make changes: call update_page or delete_page with the write key (or write_url) directly. Keep write_key if you will update the page later. Tools that take a "key" accept the bare key or any Canvasbin URL. HTML pages may be a fragment or a full document with <html>; scripts run in a sandbox.`

type Options struct {
	Store         *store.Store
	PublicBaseURL string
	CreateLimit   *ratelimit.Limiter
	Version       string
}

// Handler returns the Streamable HTTP handler mounted at /mcp.
func Handler(o Options) http.Handler {
	return mcp.NewStreamableHTTPHandler(func(r *http.Request) *mcp.Server {
		return newServer(o, links.Base(o.PublicBaseURL, r), links.ClientIP(r))
	}, &mcp.StreamableHTTPOptions{
		Stateless:           true,
		JSONResponse:        true,
		MaxRequestBodyBytes: 3 * store.MaxContentBytes,
	})
}

type tools struct {
	Options
	base     string
	clientIP string
}

func newServer(o Options, base, clientIP string) *mcp.Server {
	s := mcp.NewServer(&mcp.Implementation{Name: "canvasbin", Title: "Canvasbin", Version: o.Version},
		&mcp.ServerOptions{Instructions: instructions})
	t := &tools{Options: o, base: base, clientIP: clientIP}

	mcp.AddTool(s, &mcp.Tool{
		Name:        "create_page",
		Title:       "Create page",
		Description: "Publish a new HTML or Markdown page. Returns html_url (rendered, view-only), pdf_url, read_url (read-only app view) and write_url (browser editor), plus the keys.",
	}, t.create)
	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_page",
		Title:       "Get page",
		Description: "Fetch a page's title, format and source content using its read key, write key, or either URL.",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true},
	}, t.get)
	mcp.AddTool(s, &mcp.Tool{
		Name:        "update_page",
		Title:       "Update page",
		Description: "Edit a page directly: change its content, title or format. Requires the write key or write URL. Omitted fields are left unchanged; content replaces the whole page. Changes are live on the page URLs immediately.",
		Annotations: &mcp.ToolAnnotations{IdempotentHint: true},
	}, t.update)
	mcp.AddTool(s, &mcp.Tool{
		Name:        "delete_page",
		Title:       "Delete page",
		Description: "Permanently delete a page. Requires the write key or write URL. Both links stop working.",
		Annotations: &mcp.ToolAnnotations{DestructiveHint: ptr(true), IdempotentHint: true},
	}, t.remove)
	return s
}

type CreateInput struct {
	Content string `json:"content" jsonschema:"page source: HTML (fragment or full document) or Markdown"`
	Format  string `json:"format,omitempty" jsonschema:"html (default) or markdown"`
	Title   string `json:"title,omitempty" jsonschema:"title shown in the browser tab"`
}

type GetInput struct {
	Key string `json:"key" jsonschema:"read key, write key, or the full read/write URL"`
}

type UpdateInput struct {
	Key     string  `json:"key" jsonschema:"write key or write URL"`
	Content *string `json:"content,omitempty" jsonschema:"new full page source; replaces the existing content"`
	Format  *string `json:"format,omitempty" jsonschema:"html or markdown"`
	Title   *string `json:"title,omitempty" jsonschema:"new title"`
}

type DeleteInput struct {
	Key string `json:"key" jsonschema:"write key or write URL"`
}

type PageResult struct {
	Title     string    `json:"title"`
	Format    string    `json:"format"`
	Content   *string   `json:"content,omitempty"`
	CanWrite  bool      `json:"can_write"`
	ReadURL   string    `json:"read_url"`
	WriteURL  string    `json:"write_url,omitempty"`
	HTMLURL   string    `json:"html_url"`
	PDFURL    string    `json:"pdf_url"`
	RawURL    string    `json:"raw_url"`
	ReadKey   string    `json:"read_key"`
	WriteKey  string    `json:"write_key,omitempty"`
	UpdatedAt time.Time `json:"updated_at"`
}

type DeleteResult struct {
	Deleted bool `json:"deleted"`
}

func (t *tools) result(a *store.Access, withContent bool) PageResult {
	l := links.For(t.base, a.ReadKey, a.WriteKey)
	r := PageResult{
		Title: a.Title, Format: a.Format, CanWrite: a.CanWrite,
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
	a, err := t.Store.Create(ctx, in.Title, in.Format, in.Content)
	if err != nil {
		return nil, PageResult{}, err
	}
	return nil, t.result(a, false), nil
}

func (t *tools) get(ctx context.Context, _ *mcp.CallToolRequest, in GetInput) (*mcp.CallToolResult, PageResult, error) {
	key, err := keys.Parse(in.Key)
	if err != nil {
		return nil, PageResult{}, store.ErrNotFound
	}
	a, err := t.Store.Get(ctx, key)
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
	if in.Content == nil && in.Format == nil && in.Title == nil {
		return nil, PageResult{}, errors.New("nothing to update: pass content, title or format")
	}
	a, err := t.Store.Update(ctx, key, store.Changes{Title: in.Title, Format: in.Format, Content: in.Content})
	if err != nil {
		return nil, PageResult{}, err
	}
	return nil, t.result(a, false), nil
}

func (t *tools) remove(ctx context.Context, _ *mcp.CallToolRequest, in DeleteInput) (*mcp.CallToolResult, DeleteResult, error) {
	key, err := keys.Parse(in.Key)
	if err != nil {
		return nil, DeleteResult{}, store.ErrNotFound
	}
	if err := t.Store.Delete(ctx, key); err != nil {
		return nil, DeleteResult{}, err
	}
	return nil, DeleteResult{Deleted: true}, nil
}

func ptr[T any](v T) *T { return &v }
