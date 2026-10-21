package httpapi

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/stewartmckee/canvasbin/internal/auth"
	"github.com/stewartmckee/canvasbin/internal/config"
	"github.com/stewartmckee/canvasbin/internal/links"
	"github.com/stewartmckee/canvasbin/internal/store"
)

func (s *Server) teamRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/teams", s.listTeams)
	mux.HandleFunc("POST /api/teams", s.createTeam)
	mux.HandleFunc("GET /api/teams/{id}", s.getTeam)
	mux.HandleFunc("PATCH /api/teams/{id}", s.renameTeam)
	mux.HandleFunc("DELETE /api/teams/{id}", s.deleteTeam)
	mux.HandleFunc("POST /api/teams/{id}/members", s.addTeamMember)
	mux.HandleFunc("PATCH /api/teams/{id}/members/{user}", s.setTeamMemberRole)
	mux.HandleFunc("DELETE /api/teams/{id}/members/{user}", s.removeTeamMember)
	mux.HandleFunc("DELETE /api/teams/{id}/invites/{invite}", s.cancelTeamInvite)
	mux.HandleFunc("GET /api/teams/{id}/pages", s.teamPages)
	mux.HandleFunc("GET /api/teams/{id}/activity", s.teamActivity)
	mux.HandleFunc("GET /api/teams/{id}/style-guide", s.getStyleGuide)
	mux.HandleFunc("PUT /api/teams/{id}/style-guide", s.setStyleGuide)
	mux.HandleFunc("POST /api/teams/{id}/billing/checkout", s.teamCheckout)
	mux.HandleFunc("POST /api/teams/{id}/billing/portal", s.teamPortal)

	mux.HandleFunc("GET /api/pages/{key}/comments", s.listComments)
	mux.HandleFunc("POST /api/pages/{key}/comments", s.addComment)
	mux.HandleFunc("DELETE /api/pages/{key}/comments/{id}", s.deleteComment)
	mux.HandleFunc("GET /api/pages/{key}/activity", s.pageActivity)
}

func (s *Server) logActivity(r *http.Request, e store.ActivityEntry) {
	if err := s.Store.LogActivity(r.Context(), e); err != nil {
		slog.Error("log activity", "err", err, "action", e.Action)
	}
}

// logPage records a page event on its team's timeline; personal pages have none.
func (s *Server) logPage(r *http.Request, u *store.User, a *store.Access, action string) {
	if a == nil || a.TeamID == nil || u == nil {
		return
	}
	var pageID *int64
	if action != store.ActivityPageDeleted {
		id := a.ID
		pageID = &id
	}
	s.logActivity(r, store.ActivityEntry{TeamID: *a.TeamID, UserID: u.ID, PageID: pageID, PageTitle: a.Title, Action: action})
}

func (s *Server) syncSeats(r *http.Request, teamID int64) {
	if s.Billing == nil {
		return
	}
	if err := s.Billing.SyncTeamSeats(r.Context(), teamID); err != nil {
		slog.Error("sync team seats", "err", err, "team", teamID)
	}
}

func pathID(w http.ResponseWriter, r *http.Request, name string) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue(name), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusNotFound, "not found")
		return 0, false
	}
	return id, true
}

// team loads {id} for a member. manage additionally requires owner or admin.
func (s *Server) team(w http.ResponseWriter, r *http.Request, manage bool) (*store.User, *store.Team, bool) {
	u, ok := s.requireUser(w, r)
	if !ok {
		return nil, nil, false
	}
	id, ok := pathID(w, r, "id")
	if !ok {
		return nil, nil, false
	}
	t, err := s.Store.GetTeam(r.Context(), id, u.ID)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "team not found")
		return nil, nil, false
	}
	if err != nil {
		writeStoreError(w, err)
		return nil, nil, false
	}
	if manage && !t.CanManage() {
		writeError(w, http.StatusForbidden, "only team owners and admins can do that")
		return nil, nil, false
	}
	return u, t, true
}

type teamView struct {
	store.Team
	Active bool `json:"active"`
	Quota  int  `json:"quota"`
}

func (s *Server) teamView(t *store.Team) teamView {
	return teamView{Team: *t, Active: t.Active(), Quota: s.Config.TeamPageQuota()}
}

func (s *Server) teamViews(ts []store.Team) []teamView {
	out := make([]teamView, 0, len(ts))
	for i := range ts {
		out = append(out, s.teamView(&ts[i]))
	}
	return out
}

func (s *Server) listTeams(w http.ResponseWriter, r *http.Request) {
	u, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	teams, err := s.Store.ListTeams(r.Context(), u.ID)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"teams": s.teamViews(teams)})
}

func (s *Server) createTeam(w http.ResponseWriter, r *http.Request) {
	u, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	if !u.Verified() {
		writeStoreError(w, store.ErrUnverified)
		return
	}
	var in struct {
		Name string `json:"name"`
	}
	if !decode(w, r, &in) {
		return
	}
	t, err := s.Store.CreateTeam(r.Context(), u.ID, in.Name)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, s.teamView(t))
}

func (s *Server) getTeam(w http.ResponseWriter, r *http.Request) {
	_, t, ok := s.team(w, r, false)
	if !ok {
		return
	}
	members, invites, err := s.Store.ListTeamMembers(r.Context(), t.ID)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"team":            s.teamView(t),
		"members":         members,
		"invites":         invites,
		"billing_enabled": s.Billing != nil && s.Billing.TeamEnabled(),
	})
}

func styleGuideView(t *store.Team, guide string) map[string]any {
	return map[string]any{
		"team_id":     t.ID,
		"team":        t.Name,
		"style_guide": guide,
		"max_length":  store.MaxStyleGuideRunes,
		"can_edit":    true,
	}
}

// styleGuideTeam loads the team for a member and refuses the call unless the
// Team plan is active. Any member may then read, create, or update the guide.
func (s *Server) styleGuideTeam(w http.ResponseWriter, r *http.Request) (*store.User, *store.Team, bool) {
	u, t, ok := s.team(w, r, false)
	if !ok {
		return nil, nil, false
	}
	if !t.Active() {
		writeStoreError(w, config.StyleGuideInactiveError(s.baseURL(r), t.ID))
		return nil, nil, false
	}
	return u, t, true
}

func (s *Server) getStyleGuide(w http.ResponseWriter, r *http.Request) {
	_, t, ok := s.styleGuideTeam(w, r)
	if !ok {
		return
	}
	guide, err := s.Store.GetStyleGuide(r.Context(), t.ID)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, styleGuideView(t, guide))
}

func (s *Server) setStyleGuide(w http.ResponseWriter, r *http.Request) {
	u, t, ok := s.styleGuideTeam(w, r)
	if !ok {
		return
	}
	var in struct {
		StyleGuide string `json:"style_guide"`
	}
	if !decode(w, r, &in) {
		return
	}
	prev, err := s.Store.GetStyleGuide(r.Context(), t.ID)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	guide, err := s.Store.SetStyleGuide(r.Context(), t.ID, in.StyleGuide)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	if guide != prev {
		detail := ""
		if guide == "" {
			detail = "cleared"
		}
		s.logActivity(r, store.ActivityEntry{
			TeamID: t.ID, UserID: u.ID, Action: store.ActivityStyleGuideUpdated, Detail: detail,
		})
	}
	out := styleGuideView(t, guide)
	out["created"] = prev == "" && guide != ""
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) renameTeam(w http.ResponseWriter, r *http.Request) {
	u, t, ok := s.team(w, r, true)
	if !ok {
		return
	}
	var in struct {
		Name string `json:"name"`
	}
	if !decode(w, r, &in) {
		return
	}
	if err := s.Store.RenameTeam(r.Context(), t.ID, in.Name); err != nil {
		writeStoreError(w, err)
		return
	}
	t, err := s.Store.GetTeam(r.Context(), t.ID, u.ID)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	s.logActivity(r, store.ActivityEntry{TeamID: t.ID, UserID: u.ID, Action: store.ActivityTeamRenamed, Detail: t.Name})
	writeJSON(w, http.StatusOK, s.teamView(t))
}

func (s *Server) deleteTeam(w http.ResponseWriter, r *http.Request) {
	_, t, ok := s.team(w, r, true)
	if !ok {
		return
	}
	if t.Role != store.TeamRoleOwner {
		writeError(w, http.StatusForbidden, "only the team owner can delete the team")
		return
	}
	if t.Active() {
		writeError(w, http.StatusConflict, "cancel the team subscription before deleting the team")
		return
	}
	if err := s.Store.DeleteTeam(r.Context(), t.ID); err != nil {
		writeStoreError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) writeTeamMembers(w http.ResponseWriter, r *http.Request, teamID int64) {
	members, invites, err := s.Store.ListTeamMembers(r.Context(), teamID)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"members": members, "invites": invites})
}

func (s *Server) addTeamMember(w http.ResponseWriter, r *http.Request) {
	u, t, ok := s.team(w, r, true)
	if !ok {
		return
	}
	var in struct {
		Email string `json:"email"`
		Role  string `json:"role"`
	}
	if !decode(w, r, &in) {
		return
	}
	email, err := auth.ValidateEmail(in.Email)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if in.Role == "" {
		in.Role = store.TeamRoleMember
	}
	if in.Role == store.TeamRoleAdmin && t.Role != store.TeamRoleOwner {
		writeError(w, http.StatusForbidden, "only the team owner can add admins")
		return
	}
	if !s.allowMail(w, "team:"+u.Email) {
		return
	}
	added, _, err := s.Store.AddTeamMember(r.Context(), t.ID, u.ID, email, in.Role)
	if errors.Is(err, store.ErrConflict) {
		writeError(w, http.StatusConflict, "already a member of this team")
		return
	}
	if err != nil {
		writeStoreError(w, err)
		return
	}
	inviter := u.DisplayName
	if inviter == "" {
		inviter = u.Email
	}
	link := s.baseURL(r) + "/teams/" + strconv.FormatInt(t.ID, 10)
	if added == nil {
		link = s.baseURL(r) + "/register"
	}
	if err := s.Mailer.SendTeamInvite(r.Context(), email, link, inviter, t.Name); err != nil {
		slog.Error("send team invite email", "err", err)
	}
	if added != nil {
		s.logActivity(r, store.ActivityEntry{TeamID: t.ID, UserID: added.ID, Action: store.ActivityMemberJoined, Detail: "added by " + u.Email})
		s.syncSeats(r, t.ID)
	} else {
		s.logActivity(r, store.ActivityEntry{TeamID: t.ID, UserID: u.ID, Action: store.ActivityMemberInvited, Detail: email})
	}
	s.writeTeamMembers(w, r, t.ID)
}

func (s *Server) setTeamMemberRole(w http.ResponseWriter, r *http.Request) {
	u, t, ok := s.team(w, r, true)
	if !ok {
		return
	}
	if t.Role != store.TeamRoleOwner {
		writeError(w, http.StatusForbidden, "only the team owner can change roles")
		return
	}
	uid, ok := pathID(w, r, "user")
	if !ok {
		return
	}
	var in struct {
		Role string `json:"role"`
	}
	if !decode(w, r, &in) {
		return
	}
	if err := s.Store.SetTeamMemberRole(r.Context(), t.ID, uid, in.Role); err != nil {
		writeStoreError(w, err)
		return
	}
	s.logActivity(r, store.ActivityEntry{TeamID: t.ID, UserID: u.ID, Action: store.ActivityRoleChanged, Detail: memberEmail(r, s, uid) + " is now " + in.Role})
	s.writeTeamMembers(w, r, t.ID)
}

func memberEmail(r *http.Request, s *Server, uid int64) string {
	if m, err := s.Store.GetUserByID(r.Context(), uid); err == nil {
		return m.Email
	}
	return "a member"
}

// removeTeamMember removes someone (owners and admins), or lets a member leave.
func (s *Server) removeTeamMember(w http.ResponseWriter, r *http.Request) {
	u, t, ok := s.team(w, r, false)
	if !ok {
		return
	}
	uid, ok := pathID(w, r, "user")
	if !ok {
		return
	}
	self := uid == u.ID
	if !self && !t.CanManage() {
		writeError(w, http.StatusForbidden, "only team owners and admins can remove members")
		return
	}
	if !self && t.Role == store.TeamRoleAdmin {
		if target, err := s.Store.GetTeam(r.Context(), t.ID, uid); err == nil && target.Role == store.TeamRoleAdmin {
			writeError(w, http.StatusForbidden, "only the team owner can remove admins")
			return
		}
	}
	email := memberEmail(r, s, uid)
	if err := s.Store.RemoveTeamMember(r.Context(), t.ID, uid); err != nil {
		writeStoreError(w, err)
		return
	}
	if self {
		s.logActivity(r, store.ActivityEntry{TeamID: t.ID, UserID: u.ID, Action: store.ActivityMemberLeft})
	} else {
		s.logActivity(r, store.ActivityEntry{TeamID: t.ID, UserID: u.ID, Action: store.ActivityMemberRemoved, Detail: email})
	}
	s.syncSeats(r, t.ID)
	if self {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	s.writeTeamMembers(w, r, t.ID)
}

func (s *Server) cancelTeamInvite(w http.ResponseWriter, r *http.Request) {
	_, t, ok := s.team(w, r, true)
	if !ok {
		return
	}
	id, ok := pathID(w, r, "invite")
	if !ok {
		return
	}
	if err := s.Store.CancelTeamInvite(r.Context(), t.ID, id); err != nil {
		writeStoreError(w, err)
		return
	}
	s.writeTeamMembers(w, r, t.ID)
}

func (s *Server) teamPages(w http.ResponseWriter, r *http.Request) {
	u, t, ok := s.team(w, r, false)
	if !ok {
		return
	}
	pages, err := s.Store.ListTeamPages(r.Context(), t.ID, u.ID)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	base := s.baseURL(r)
	items := make([]libraryItem, 0, len(pages))
	for _, p := range pages {
		var l links.Links
		if p.ReadKey != "" {
			l = links.For(base, p.ReadKey, "")
		}
		items = append(items, libraryItem{PageSummary: p, Links: l})
	}
	writeJSON(w, http.StatusOK, map[string]any{"pages": items})
}

func (s *Server) teamActivity(w http.ResponseWriter, r *http.Request) {
	_, t, ok := s.team(w, r, false)
	if !ok {
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	items, err := s.Store.ListActivity(r.Context(), t.ID, nil, limit)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"activity": items})
}

func (s *Server) teamCheckout(w http.ResponseWriter, r *http.Request) {
	u, t, ok := s.team(w, r, true)
	if !ok {
		return
	}
	if s.Billing == nil || !s.Billing.TeamEnabled() {
		writeError(w, http.StatusServiceUnavailable, "billing is not available")
		return
	}
	if t.Active() {
		writeError(w, http.StatusConflict, "already subscribed; use the billing portal to manage the plan")
		return
	}
	back := s.baseURL(r) + "/teams/" + strconv.FormatInt(t.ID, 10)
	url, err := s.Billing.CreateTeamCheckoutSession(r.Context(), t, u, t.MemberCount, back+"?billing=success", back+"?billing=cancelled")
	if err != nil {
		writeBillingError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"url": url})
}

func (s *Server) teamPortal(w http.ResponseWriter, r *http.Request) {
	_, t, ok := s.team(w, r, true)
	if !ok {
		return
	}
	if s.Billing == nil {
		writeError(w, http.StatusServiceUnavailable, "billing is not available")
		return
	}
	url, err := s.Billing.CreateTeamPortalSession(r.Context(), t, s.baseURL(r)+"/teams/"+strconv.FormatInt(t.ID, 10))
	if err != nil {
		writeBillingError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"url": url})
}

// checkTeamCreate reports why u can't add a page to teamID, if they can't.
func (s *Server) checkTeamCreate(r *http.Request, u *store.User, teamID int64) error {
	base := links.Base(s.PublicBaseURL, r)
	if !u.Verified() {
		return config.UnverifiedError(base)
	}
	t, err := s.Store.GetTeam(r.Context(), teamID, u.ID)
	if err != nil {
		return err
	}
	if !t.Active() {
		return config.TeamInactiveError(base, t.ID)
	}
	if t.PageCount >= s.Config.TeamPageQuota() {
		return s.Config.TeamQuotaError()
	}
	return nil
}

// ---- comments and page activity (team pages only) ----

// teamPage loads {key} for a member of the page's team.
func (s *Server) teamPage(w http.ResponseWriter, r *http.Request) (*store.User, *store.Access, bool) {
	u, ok := s.requireUser(w, r)
	if !ok {
		return nil, nil, false
	}
	key, ok := pathKey(w, r)
	if !ok {
		return nil, nil, false
	}
	a, err := s.Store.GetForUser(r.Context(), key, u.ID)
	if err != nil {
		writeStoreError(w, err)
		return nil, nil, false
	}
	if a.TeamID == nil || a.TeamRole == "" {
		writeJSON(w, http.StatusForbidden, map[string]string{
			"error": "comments and activity are available on team pages", "code": "team_required"})
		return nil, nil, false
	}
	return u, a, true
}

func canDeleteComment(u *store.User, a *store.Access, c *store.Comment) bool {
	return (c.UserID != nil && *c.UserID == u.ID) ||
		a.TeamRole == store.TeamRoleOwner || a.TeamRole == store.TeamRoleAdmin
}

func (s *Server) listComments(w http.ResponseWriter, r *http.Request) {
	u, a, ok := s.teamPage(w, r)
	if !ok {
		return
	}
	cs, err := s.Store.ListComments(r.Context(), a.ID)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	for i := range cs {
		cs[i].CanDelete = canDeleteComment(u, a, &cs[i])
	}
	writeJSON(w, http.StatusOK, map[string]any{"comments": cs})
}

func (s *Server) addComment(w http.ResponseWriter, r *http.Request) {
	u, a, ok := s.teamPage(w, r)
	if !ok {
		return
	}
	var in struct {
		Body string `json:"body"`
	}
	if !decode(w, r, &in) {
		return
	}
	c, err := s.Store.AddComment(r.Context(), a.ID, u.ID, in.Body)
	if errors.Is(err, store.ErrBadComment) {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err != nil {
		writeStoreError(w, err)
		return
	}
	c.CanDelete = true
	s.logPage(r, u, a, store.ActivityCommentAdded)
	writeJSON(w, http.StatusCreated, c)
}

func (s *Server) deleteComment(w http.ResponseWriter, r *http.Request) {
	u, a, ok := s.teamPage(w, r)
	if !ok {
		return
	}
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	c, err := s.Store.GetComment(r.Context(), a.ID, id)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	if !canDeleteComment(u, a, c) {
		writeError(w, http.StatusForbidden, "you can only delete your own comments")
		return
	}
	if err := s.Store.DeleteComment(r.Context(), a.ID, id); err != nil {
		writeStoreError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) pageActivity(w http.ResponseWriter, r *http.Request) {
	_, a, ok := s.teamPage(w, r)
	if !ok {
		return
	}
	id := a.ID
	items, err := s.Store.ListActivity(r.Context(), *a.TeamID, &id, 50)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"activity": items})
}
