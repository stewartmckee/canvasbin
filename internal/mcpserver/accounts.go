package mcpserver

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/stewartmckee/canvasbin/internal/auth"
	"github.com/stewartmckee/canvasbin/internal/keys"
	"github.com/stewartmckee/canvasbin/internal/links"
	"github.com/stewartmckee/canvasbin/internal/store"
)

// accountInstructions replaces the anonymous-only text when accounts are on.
const accountInstructions = `Canvasbin hosts HTML or Markdown pages at unguessable URLs. You are signed in to the user's Canvasbin account (OAuth).

create_page publishes into the user's own library and counts toward their page quota. get_page, update_page and delete_page work on pages the user owns or that were shared with them (by read key or URL), and on any page you hold the write key for. Account tools:
- list_pages: list every page the user owns or has been shared (with role and links).
- claim_page: take ownership of an anonymous page you hold the write key for, so it appears in the user's library.
- share_page: give someone access to an owned page by email (viewer or editor); they are emailed an invitation.
- unshare_page: remove someone's access.

` + pageInstructions

type ListPagesInput struct{}

type PageSummary struct {
	Title      string    `json:"title"`
	Format     string    `json:"format"`
	Role       string    `json:"role" jsonschema:"owner, editor or viewer"`
	CanWrite   bool      `json:"can_write"`
	OwnerEmail string    `json:"owner_email,omitempty"`
	ReadKey    string    `json:"read_key"`
	ReadURL    string    `json:"read_url"`
	HTMLURL    string    `json:"html_url"`
	PDFURL     string    `json:"pdf_url"`
	RawURL     string    `json:"raw_url"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type ListPagesResult struct {
	Pages []PageSummary `json:"pages"`
	Owned int           `json:"owned" jsonschema:"number of pages the user owns"`
	Quota int           `json:"quota" jsonschema:"number of pages the user may own"`
}

type ClaimInput struct {
	Key string `json:"key" jsonschema:"write key or write URL of an unowned page"`
}

type ShareInput struct {
	Key   string `json:"key" jsonschema:"read key, write key or URL of a page the user owns"`
	Email string `json:"email" jsonschema:"email address to share with"`
	Role  string `json:"role,omitempty" jsonschema:"viewer (default) or editor"`
}

type UnshareInput struct {
	Key   string `json:"key" jsonschema:"read key, write key or URL of a page the user owns"`
	Email string `json:"email" jsonschema:"email address to remove"`
}

type SharesResult struct {
	Shares  []store.Share  `json:"shares"`
	Invites []store.Invite `json:"invites" jsonschema:"pending invitations for people without an account yet"`
}

func (t *tools) addAccountTools(s *mcp.Server) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_pages",
		Title:       "List my pages",
		Description: "List the pages the signed-in user owns or that were shared with them, newest first. Requires sign-in.",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true},
	}, t.list)
	mcp.AddTool(s, &mcp.Tool{
		Name:        "claim_page",
		Title:       "Claim page",
		Description: "Attach an anonymous page to the signed-in user's library using its write key. Counts toward their quota. Requires sign-in.",
		Annotations: &mcp.ToolAnnotations{IdempotentHint: true},
	}, t.claim)
	mcp.AddTool(s, &mcp.Tool{
		Name:        "share_page",
		Title:       "Share page",
		Description: "Give someone access to a page you own, by email, as viewer or editor. People without an account get an invitation that applies when they sign up. Requires sign-in.",
		Annotations: &mcp.ToolAnnotations{IdempotentHint: true},
	}, t.share)
	mcp.AddTool(s, &mcp.Tool{
		Name:        "unshare_page",
		Title:       "Unshare page",
		Description: "Remove someone's access (or pending invitation) from a page you own. Requires sign-in.",
		Annotations: &mcp.ToolAnnotations{DestructiveHint: ptr(true), IdempotentHint: true},
	}, t.unshare)
}

var errSignIn = errors.New("sign in required")

// requireUser is defence in depth: the gate already refuses unauthenticated
// calls to these tools.
func (t *tools) requireUser() error {
	if t.user == nil {
		return errSignIn
	}
	return nil
}

func (t *tools) list(ctx context.Context, _ *mcp.CallToolRequest, _ ListPagesInput) (*mcp.CallToolResult, ListPagesResult, error) {
	if err := t.requireUser(); err != nil {
		return nil, ListPagesResult{}, err
	}
	pages, err := t.Store.ListOwnedAndShared(ctx, t.user.ID)
	if err != nil {
		return nil, ListPagesResult{}, err
	}
	out := ListPagesResult{Pages: make([]PageSummary, 0, len(pages)), Quota: t.Config.Quota(t.user)}
	for _, p := range pages {
		l := links.For(t.base, p.ReadKey, "")
		if p.Role == store.RoleOwner {
			out.Owned++
		}
		out.Pages = append(out.Pages, PageSummary{
			Title: p.Title, Format: p.Format, Role: p.Role, CanWrite: p.CanWrite, OwnerEmail: p.OwnerEmail,
			ReadKey: p.ReadKey, ReadURL: l.ReadURL, HTMLURL: l.HTMLURL, PDFURL: l.PDFURL, RawURL: l.RawURL,
			CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt,
		})
	}
	return nil, out, nil
}

func (t *tools) claim(ctx context.Context, _ *mcp.CallToolRequest, in ClaimInput) (*mcp.CallToolResult, PageResult, error) {
	if err := t.requireUser(); err != nil {
		return nil, PageResult{}, err
	}
	key, err := keys.Parse(in.Key)
	if err != nil {
		return nil, PageResult{}, store.ErrNotFound
	}
	// Claiming a page you already own doesn't use quota.
	if existing, gerr := t.Store.GetForUser(ctx, key, t.user.ID); gerr == nil && existing.Role == store.RoleOwner {
		return nil, t.result(existing, false), nil
	}
	if err := t.checkQuota(ctx); err != nil {
		return nil, PageResult{}, err
	}
	a, err := t.Store.Claim(ctx, key, t.user.ID)
	if err != nil {
		return nil, PageResult{}, err
	}
	return nil, t.result(a, false), nil
}

// checkQuota mirrors the HTTP API: verified email and room under the quota.
func (t *tools) checkQuota(ctx context.Context) error {
	if !t.user.Verified() {
		return store.ErrUnverified
	}
	n, err := t.Store.CountOwned(ctx, t.user.ID)
	if err != nil {
		return err
	}
	if n >= t.Config.Quota(t.user) {
		return store.ErrQuotaExceeded
	}
	return nil
}

// ownedPage loads the page behind key and requires the caller to own it.
func (t *tools) ownedPage(ctx context.Context, rawKey string) (*store.Access, error) {
	if err := t.requireUser(); err != nil {
		return nil, err
	}
	key, err := keys.Parse(rawKey)
	if err != nil {
		return nil, store.ErrNotFound
	}
	a, err := t.Store.GetForUser(ctx, key, t.user.ID)
	if err != nil {
		return nil, err
	}
	if a.Role != store.RoleOwner {
		return nil, errors.New("only the owner can manage sharing")
	}
	return a, nil
}

func (t *tools) shares(ctx context.Context, pageID int64) (SharesResult, error) {
	shares, invites, err := t.Store.ListShares(ctx, pageID)
	return SharesResult{Shares: shares, Invites: invites}, err
}

func (t *tools) share(ctx context.Context, _ *mcp.CallToolRequest, in ShareInput) (*mcp.CallToolResult, SharesResult, error) {
	a, err := t.ownedPage(ctx, in.Key)
	if err != nil {
		return nil, SharesResult{}, err
	}
	email, err := auth.ValidateEmail(in.Email)
	if err != nil {
		return nil, SharesResult{}, err
	}
	role := in.Role
	if role == "" {
		role = store.RoleViewer
	}
	if role != store.RoleViewer && role != store.RoleEditor {
		return nil, SharesResult{}, errors.New(`role must be "viewer" or "editor"`)
	}
	if t.MailLimit != nil && !t.MailLimit.Allow("share:"+t.user.Email) {
		return nil, SharesResult{}, errors.New("too many emails requested, try again later")
	}
	if _, err := t.Store.ShareWith(ctx, a.ID, t.user.ID, email, role); err != nil {
		return nil, SharesResult{}, err
	}
	if t.Mailer != nil {
		inviter := t.user.DisplayName
		if inviter == "" {
			inviter = t.user.Email
		}
		if err := t.Mailer.SendInvite(ctx, email, t.base+"/"+a.ReadKey, inviter); err != nil {
			slog.Error("send invite email", "err", err)
		}
	}
	out, err := t.shares(ctx, a.ID)
	return nil, out, err
}

func (t *tools) unshare(ctx context.Context, _ *mcp.CallToolRequest, in UnshareInput) (*mcp.CallToolResult, SharesResult, error) {
	a, err := t.ownedPage(ctx, in.Key)
	if err != nil {
		return nil, SharesResult{}, err
	}
	removed, err := t.Store.RemoveShare(ctx, a.ID, in.Email)
	if err != nil {
		return nil, SharesResult{}, err
	}
	if !removed {
		return nil, SharesResult{}, errors.New("no such share")
	}
	out, err := t.shares(ctx, a.ID)
	return nil, out, err
}
