package mcpserver

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/stewartmckee/canvasbin/internal/auth"
	"github.com/stewartmckee/canvasbin/internal/config"
	"github.com/stewartmckee/canvasbin/internal/keys"
	"github.com/stewartmckee/canvasbin/internal/links"
	"github.com/stewartmckee/canvasbin/internal/store"
)

// accountInstructions replaces the anonymous-only text when accounts are on.
const accountInstructions = `Canvasbin hosts HTML or Markdown pages at unguessable URLs. You are signed in to the user's Canvasbin account (OAuth).

create_page publishes into the user's own library and counts toward their page quota; pass team (a team name or id from list_teams) to publish into a shared team space instead. get_page, update_page and delete_page work on pages the user owns, that were shared with them, or that belong to one of their teams (by read key or URL), and on any page you hold the write key for. Account tools:
- list_pages: list every page the user owns, has been shared, or can reach through a team (with role, team and links).
- list_teams: the user's teams, with role, plan status and page counts.
- claim_page: take ownership of an anonymous page you hold the write key for, so it appears in the user's library.
- share_page: give someone access to an owned page by email (viewer or editor); they are emailed an invitation. Team plan only.
- unshare_page: remove someone's access.
- list_comments / add_comment: read and post comments on a team page. Team members only.
- get_style_guide / set_style_guide: read, create, or update a team's style guide. Team plan only. Any member of that team can create or update it. set_style_guide creates the guide when the team has none and replaces it when one already exists. Pass an empty style_guide to clear it.

When the page quota is full, create_page and claim_page return an error that explains the user's options. Users delete pages they no longer need to stay within quota—when they ask to free space, stay under quota, remove old pages, or delete a specific page, call delete_page with that page's write key or write URL (list_pages helps find titles and keys). If they want several removed but do not name them, list their library, suggest which pages look disposable, and delete the ones they confirm.

Style guides are a Team plan feature. Before create_page into a team, call get_style_guide with that team's name or id. If it returns a style guide, follow it when you write the page. If the guide is empty, or the team's plan is not active, write the page as usual. Personal pages have no style guide. When the user wants to set or change how a team's pages are written, call set_style_guide: it creates the guide if there isn't one and updates it if there is.

` + pageInstructions

type ListPagesInput struct{}

type PageSummary struct {
	Title       string    `json:"title"`
	Format      string    `json:"format"`
	Role        string    `json:"role" jsonschema:"owner, editor or viewer"`
	CanWrite    bool      `json:"can_write"`
	AgenticOnly bool      `json:"agentic_only"`
	OwnerEmail  string    `json:"owner_email,omitempty"`
	Team        string    `json:"team,omitempty" jsonschema:"name of the team the page belongs to, if any"`
	ReadKey     string    `json:"read_key"`
	ReadURL     string    `json:"read_url"`
	HTMLURL     string    `json:"html_url"`
	PDFURL      string    `json:"pdf_url"`
	RawURL      string    `json:"raw_url"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type ListPagesResult struct {
	Pages []PageSummary `json:"pages"`
	Owned int           `json:"owned" jsonschema:"number of personal pages the user owns"`
	Quota int           `json:"quota" jsonschema:"number of personal pages the user may own"`
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
		Description: "List the pages the signed-in user owns, that were shared with them, or that belong to their teams, newest first, with each page's links. Requires sign-in.",
		Annotations: readTool(),
	}, t.list)
	mcp.AddTool(s, &mcp.Tool{
		Name:        "claim_page",
		Title:       "Claim page",
		Description: "Attach an anonymous page to the signed-in user's library using its write key. Counts toward their quota. Requires sign-in.",
		Annotations: writeTool(false, true, false),
	}, t.claim)
	mcp.AddTool(s, &mcp.Tool{
		Name:        "share_page",
		Title:       "Share page",
		Description: "Give someone access to a page you own, by email, as viewer or editor. People without an account get an invitation that applies when they sign up. Requires sign-in.",
		Annotations: writeTool(true, true, true),
	}, t.share)
	mcp.AddTool(s, &mcp.Tool{
		Name:        "unshare_page",
		Title:       "Unshare page",
		Description: "Remove someone's access (or pending invitation) from a page you own. Requires sign-in.",
		Annotations: writeTool(true, true, false),
	}, t.unshare)
	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_teams",
		Title:       "List my teams",
		Description: "List the teams the signed-in user belongs to, with their role, whether the team plan is active, and page counts.",
		Annotations: readTool(),
	}, t.listTeams)
	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_comments",
		Title:       "List comments",
		Description: "Read the comment thread on a team page, oldest first. Team members only.",
		Annotations: readTool(),
	}, t.listComments)
	mcp.AddTool(s, &mcp.Tool{
		Name:        "add_comment",
		Title:       "Add comment",
		Description: "Post a comment on a team page as the signed-in user. Team members only; comments are never shown to people viewing by link.",
		Annotations: writeTool(false, false, false),
	}, t.addComment)
	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_style_guide",
		Title:       "Get style guide",
		Description: "Read a team's style guide: the text that says how its pages should be written. Call this before create_page when publishing into a team, and follow the guide when one is set. Team plan only. An empty style_guide means the team has not set one.",
		Annotations: readTool(),
	}, t.getStyleGuide)
	mcp.AddTool(s, &mcp.Tool{
		Name:        "set_style_guide",
		Title:       "Create or update style guide",
		Description: "Create or update a team's style guide, the text assistants follow when they write that team's pages. Team plan only. Any member of the team can call this. If the team has no guide, this creates one. If it already has one, this replaces it. Pass an empty style_guide to clear it. created is true only when this call saved the team's first guide.",
		Annotations: writeTool(true, true, false),
	}, t.setStyleGuide)
}

type TeamSummary struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Role        string `json:"role" jsonschema:"owner, admin or member"`
	Active      bool   `json:"active" jsonschema:"whether the team plan is active; inactive teams can't create pages"`
	MemberCount int    `json:"member_count"`
	PageCount   int    `json:"page_count"`
	Quota       int    `json:"quota"`
}

type ListTeamsResult struct {
	Teams []TeamSummary `json:"teams"`
}

type CommentsInput struct {
	Key string `json:"key" jsonschema:"read key, write key or URL of a team page"`
}

type AddCommentInput struct {
	Key  string `json:"key" jsonschema:"read key, write key or URL of a team page"`
	Body string `json:"body" jsonschema:"comment text (plain text)"`
}

type CommentsResult struct {
	Comments []store.Comment `json:"comments"`
}

type StyleGuideInput struct {
	Team string `json:"team" jsonschema:"name or id of one of the user's teams"`
}

type SetStyleGuideInput struct {
	Team       string `json:"team" jsonschema:"name or id of one of the user's teams"`
	StyleGuide string `json:"style_guide" jsonschema:"the style guide text; empty clears it. Maximum 8000 characters"`
}

type StyleGuideResult struct {
	Team       string `json:"team"`
	TeamID     int64  `json:"team_id"`
	StyleGuide string `json:"style_guide"`
}

// StyleGuideWriteResult is what set_style_guide returns. Created distinguishes
// a first save from a later update.
type StyleGuideWriteResult struct {
	StyleGuideResult
	Created bool `json:"created" jsonschema:"true when this call created the team's first style guide; false when it updated or cleared an existing one"`
}

// teamForStyle resolves a team the user belongs to. Style guides exist only
// while that team's plan is active.
func (t *tools) teamForStyle(ctx context.Context, ref string) (*store.Team, error) {
	if err := t.requireUser(); err != nil {
		return nil, err
	}
	teams, err := t.Store.ListTeams(ctx, t.user.ID)
	if err != nil {
		return nil, err
	}
	ref = strings.TrimSpace(ref)
	var match *store.Team
	for i := range teams {
		if strconv.FormatInt(teams[i].ID, 10) == ref || strings.EqualFold(teams[i].Name, ref) {
			match = &teams[i]
			break
		}
	}
	if match == nil {
		return nil, fmt.Errorf("no team called %q; call list_teams to see the user's teams", ref)
	}
	if !match.Active() {
		return nil, config.StyleGuideInactiveError(t.base, match.ID)
	}
	return match, nil
}

func (t *tools) styleGuideResult(tm *store.Team, guide string) StyleGuideResult {
	return StyleGuideResult{Team: tm.Name, TeamID: tm.ID, StyleGuide: guide}
}

func (t *tools) getStyleGuide(ctx context.Context, _ *mcp.CallToolRequest, in StyleGuideInput) (*mcp.CallToolResult, StyleGuideResult, error) {
	tm, err := t.teamForStyle(ctx, in.Team)
	if err != nil {
		return nil, StyleGuideResult{}, err
	}
	guide, err := t.Store.GetStyleGuide(ctx, tm.ID)
	if err != nil {
		return nil, StyleGuideResult{}, err
	}
	return nil, t.styleGuideResult(tm, guide), nil
}

func (t *tools) setStyleGuide(ctx context.Context, _ *mcp.CallToolRequest, in SetStyleGuideInput) (*mcp.CallToolResult, StyleGuideWriteResult, error) {
	tm, err := t.teamForStyle(ctx, in.Team)
	if err != nil {
		return nil, StyleGuideWriteResult{}, err
	}
	prev, err := t.Store.GetStyleGuide(ctx, tm.ID)
	if err != nil {
		return nil, StyleGuideWriteResult{}, err
	}
	guide, err := t.Store.SetStyleGuide(ctx, tm.ID, in.StyleGuide)
	if err != nil {
		return nil, StyleGuideWriteResult{}, err
	}
	if guide != prev {
		detail := ""
		if guide == "" {
			detail = "cleared"
		}
		if err := t.Store.LogActivity(ctx, store.ActivityEntry{
			TeamID: tm.ID, UserID: t.user.ID, Action: store.ActivityStyleGuideUpdated, Detail: detail,
		}); err != nil {
			slog.Error("log activity", "err", err, "action", store.ActivityStyleGuideUpdated)
		}
	}
	return nil, StyleGuideWriteResult{
		StyleGuideResult: t.styleGuideResult(tm, guide),
		Created:          prev == "" && guide != "",
	}, nil
}

func (t *tools) listTeams(ctx context.Context, _ *mcp.CallToolRequest, _ ListPagesInput) (*mcp.CallToolResult, ListTeamsResult, error) {
	if err := t.requireUser(); err != nil {
		return nil, ListTeamsResult{}, err
	}
	teams, err := t.Store.ListTeams(ctx, t.user.ID)
	if err != nil {
		return nil, ListTeamsResult{}, err
	}
	out := ListTeamsResult{Teams: make([]TeamSummary, 0, len(teams))}
	for _, tm := range teams {
		out.Teams = append(out.Teams, TeamSummary{
			ID: tm.ID, Name: tm.Name, Role: tm.Role, Active: tm.Active(),
			MemberCount: tm.MemberCount, PageCount: tm.PageCount, Quota: t.Config.TeamPageQuota(),
		})
	}
	return nil, out, nil
}

// teamForCreate resolves a team by id or name among the user's teams and checks
// it can take another page.
func (t *tools) teamForCreate(ctx context.Context, ref string) (*store.Team, error) {
	if !t.user.Verified() {
		return nil, config.UnverifiedError(t.base)
	}
	teams, err := t.Store.ListTeams(ctx, t.user.ID)
	if err != nil {
		return nil, err
	}
	ref = strings.TrimSpace(ref)
	var match *store.Team
	for i := range teams {
		if strconv.FormatInt(teams[i].ID, 10) == ref || strings.EqualFold(teams[i].Name, ref) {
			match = &teams[i]
			break
		}
	}
	if match == nil {
		return nil, fmt.Errorf("no team called %q; call list_teams to see the user's teams", ref)
	}
	if !match.Active() {
		return nil, config.TeamInactiveError(t.base, match.ID)
	}
	if match.PageCount >= t.Config.TeamPageQuota() {
		return nil, t.Config.TeamQuotaError()
	}
	return match, nil
}

func (t *tools) logPage(ctx context.Context, a *store.Access, action string) {
	if a == nil || a.TeamID == nil || t.user == nil {
		return
	}
	var pageID *int64
	if action != store.ActivityPageDeleted {
		id := a.ID
		pageID = &id
	}
	if err := t.Store.LogActivity(ctx, store.ActivityEntry{
		TeamID: *a.TeamID, UserID: t.user.ID, PageID: pageID, PageTitle: a.Title, Action: action,
	}); err != nil {
		slog.Error("log activity", "err", err, "action", action)
	}
}

var errTeamPage = errors.New("comments are available on team pages, for members of the page's team")

func (t *tools) teamPage(ctx context.Context, rawKey string) (*store.Access, error) {
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
	if a.TeamID == nil || a.TeamRole == "" {
		return nil, errTeamPage
	}
	return a, nil
}

func (t *tools) listComments(ctx context.Context, _ *mcp.CallToolRequest, in CommentsInput) (*mcp.CallToolResult, CommentsResult, error) {
	a, err := t.teamPage(ctx, in.Key)
	if err != nil {
		return nil, CommentsResult{}, err
	}
	cs, err := t.Store.ListComments(ctx, a.ID)
	if err != nil {
		return nil, CommentsResult{}, err
	}
	return nil, CommentsResult{Comments: cs}, nil
}

func (t *tools) addComment(ctx context.Context, _ *mcp.CallToolRequest, in AddCommentInput) (*mcp.CallToolResult, store.Comment, error) {
	a, err := t.teamPage(ctx, in.Key)
	if err != nil {
		return nil, store.Comment{}, err
	}
	c, err := t.Store.AddComment(ctx, a.ID, t.user.ID, in.Body)
	if err != nil {
		return nil, store.Comment{}, err
	}
	t.logPage(ctx, a, store.ActivityCommentAdded)
	return nil, *c, nil
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
	pages, err := t.Store.ListOwnedAndShared(ctx, t.user.ID, true)
	if err != nil {
		return nil, ListPagesResult{}, err
	}
	out := ListPagesResult{Pages: make([]PageSummary, 0, len(pages)), Quota: t.Config.Quota(t.user)}
	for _, p := range pages {
		l := links.For(t.base, p.ReadKey, "")
		if p.Role == store.RoleOwner && p.TeamID == nil {
			out.Owned++
		}
		out.Pages = append(out.Pages, PageSummary{
			Title: p.Title, Format: p.Format, Role: p.Role, CanWrite: p.CanWrite, AgenticOnly: p.AgenticOnly,
			OwnerEmail: p.OwnerEmail, Team: p.TeamName,
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
		return config.UnverifiedError(t.base)
	}
	n, err := t.Store.CountOwned(ctx, t.user.ID)
	if err != nil {
		return err
	}
	if n >= t.Config.Quota(t.user) {
		return t.Config.QuotaError(t.user, t.base)
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
	if inTeam, err := t.Store.InAnyActiveTeam(ctx, t.user.ID); err != nil {
		return nil, SharesResult{}, err
	} else if !inTeam {
		return nil, SharesResult{}, errors.New("sharing with people by email is part of the Team plan; share the page's html_url or read_url instead")
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
