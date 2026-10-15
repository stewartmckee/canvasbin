package httpapi

import (
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/stewartmckee/canvasbin/internal/auth"
	"github.com/stewartmckee/canvasbin/internal/billing"
	"github.com/stewartmckee/canvasbin/internal/config"
	"github.com/stewartmckee/canvasbin/internal/keys"
	"github.com/stewartmckee/canvasbin/internal/links"
	"github.com/stewartmckee/canvasbin/internal/oauth"
	"github.com/stewartmckee/canvasbin/internal/store"
)

const (
	verifyTokenTTL = 24 * time.Hour
	resetTokenTTL  = time.Hour
)

// accounts reports whether the accounts feature is switched on.
func (s *Server) accounts() bool { return s.Config.Accounts }

// SessionUser returns the signed-in user for r, or nil for anonymous requests
// (including when accounts are off, or the session is expired or disabled).
func (s *Server) SessionUser(r *http.Request) *store.User {
	if !s.accounts() {
		return nil
	}
	u, err := auth.SessionFromRequest(r, s.Store)
	if err != nil {
		if !errors.Is(err, store.ErrUnauthorized) && !errors.Is(err, store.ErrDisabled) {
			slog.Error("session lookup", "err", err)
		}
		return nil
	}
	return u
}

// NewOAuth builds the MCP OAuth provider backed by this server's browser sessions.
func (s *Server) NewOAuth() *oauth.Provider {
	return oauth.New(oauth.Options{
		Store: s.Store, Config: s.Config, SessionUser: s.SessionUser, Limiter: s.AuthLimit,
	})
}

func (s *Server) requireUser(w http.ResponseWriter, r *http.Request) (*store.User, bool) {
	u := s.SessionUser(r)
	if u == nil {
		writeError(w, http.StatusUnauthorized, "sign in required")
		return nil, false
	}
	return u, true
}

func (s *Server) requireAdmin(w http.ResponseWriter, r *http.Request) (*store.User, bool) {
	u, ok := s.requireUser(w, r)
	if !ok {
		return nil, false
	}
	if !u.Verified() || !s.Config.IsAdmin(u.Email) {
		writeError(w, http.StatusForbidden, "admin only")
		return nil, false
	}
	return u, true
}

func (s *Server) secureCookies(r *http.Request) bool {
	return strings.HasPrefix(links.Base(s.Config.PublicBaseURL, r), "https://")
}

func (s *Server) baseURL(r *http.Request) string { return links.Base(s.Config.PublicBaseURL, r) }

func (s *Server) accountRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/register", s.register)
	mux.HandleFunc("POST /api/login", s.login)
	mux.HandleFunc("POST /api/logout", s.logout)
	mux.HandleFunc("POST /api/resend-verification", s.resendVerification)
	mux.HandleFunc("POST /api/forgot-password", s.forgotPassword)
	mux.HandleFunc("POST /api/reset-password", s.resetPassword)
	mux.HandleFunc("GET /api/me", s.me)
	mux.HandleFunc("GET /verify", s.verify)

	mux.HandleFunc("POST /api/pages/claim", s.claim)
	mux.HandleFunc("GET /api/library", s.library)
	mux.HandleFunc("GET /api/pages/{key}/shares", s.listShares)
	mux.HandleFunc("POST /api/pages/{key}/shares", s.addShare)
	mux.HandleFunc("DELETE /api/pages/{key}/shares/{email}", s.removeShare)

	mux.HandleFunc("GET /api/connections", s.listConnections)
	mux.HandleFunc("POST /api/connections/revoke", s.revokeConnection)

	mux.HandleFunc("POST /api/billing/checkout", s.checkout)
	mux.HandleFunc("POST /api/billing/portal", s.portal)
	mux.HandleFunc("POST /api/stripe/webhook", s.stripeWebhook)

	mux.HandleFunc("GET /api/admin/users", s.adminUsers)
	mux.HandleFunc("POST /api/admin/users/{id}/disable", s.adminDisable)
}

// ---- helpers ----

func (s *Server) allowAuth(w http.ResponseWriter, r *http.Request) bool {
	if s.AuthLimit != nil && !s.AuthLimit.Allow(links.ClientIP(r)) {
		writeError(w, http.StatusTooManyRequests, "too many attempts, try again shortly")
		return false
	}
	return true
}

func (s *Server) allowMail(w http.ResponseWriter, email string) bool {
	if s.MailLimit != nil && !s.MailLimit.Allow(email) {
		writeError(w, http.StatusTooManyRequests, "too many emails requested, try again later")
		return false
	}
	return true
}

// sendVerification issues a fresh verification token and emails it. Failures are
// logged (never including secrets) and returned.
func (s *Server) sendVerification(r *http.Request, u *store.User) error {
	token, err := auth.NewToken()
	if err != nil {
		return err
	}
	if err := s.Store.CreateEmailToken(r.Context(), u.ID, store.PurposeVerify, auth.HashToken(token), time.Now().Add(verifyTokenTTL)); err != nil {
		return err
	}
	link := s.baseURL(r) + "/verify?token=" + token
	return s.Mailer.SendVerify(r.Context(), u.Email, link)
}

func (s *Server) startSession(w http.ResponseWriter, r *http.Request, u *store.User) error {
	token, err := auth.NewToken()
	if err != nil {
		return err
	}
	if err := s.Store.CreateSession(r.Context(), u.ID, auth.HashToken(token), time.Now().Add(auth.SessionLifetime)); err != nil {
		return err
	}
	auth.SetSessionCookie(w, token, s.secureCookies(r))
	return nil
}

var (
	dummyOnce sync.Once
	dummyHash string
)

// burnPasswordCheck spends roughly the same time as a real bcrypt comparison so
// login timing doesn't reveal whether an email is registered.
func burnPasswordCheck(password string) {
	dummyOnce.Do(func() { dummyHash, _ = auth.HashPassword("canvasbin-dummy-password") })
	auth.CheckPassword(dummyHash, password)
}

// ---- auth endpoints ----

type credentials struct {
	Email       string `json:"email"`
	Password    string `json:"password"`
	DisplayName string `json:"display_name"`
}

func (s *Server) register(w http.ResponseWriter, r *http.Request) {
	if !s.allowAuth(w, r) {
		return
	}
	var in credentials
	if !decode(w, r, &in) {
		return
	}
	email, err := auth.ValidateEmail(in.Email)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	hash, err := auth.HashPassword(in.Password)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if !s.allowMail(w, email) {
		return
	}
	u, err := s.Store.CreateUser(r.Context(), email, hash, in.DisplayName)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	if err := s.sendVerification(r, u); err != nil {
		slog.Error("send verification email", "err", err)
	}
	// Signed in straight away; creating pages waits for verification.
	if err := s.startSession(w, r, u); err != nil {
		writeStoreError(w, err)
		return
	}
	s.writeMe(w, r, http.StatusCreated, u)
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	if !s.allowAuth(w, r) {
		return
	}
	var in credentials
	if !decode(w, r, &in) {
		return
	}
	u, err := s.Store.GetUserByEmail(r.Context(), in.Email)
	if errors.Is(err, store.ErrNotFound) {
		burnPasswordCheck(in.Password)
		writeError(w, http.StatusUnauthorized, "invalid email or password")
		return
	}
	if err != nil {
		writeStoreError(w, err)
		return
	}
	if !auth.CheckPassword(u.PasswordHash, in.Password) {
		writeError(w, http.StatusUnauthorized, "invalid email or password")
		return
	}
	if u.Disabled() {
		writeStoreError(w, store.ErrDisabled)
		return
	}
	if err := s.startSession(w, r, u); err != nil {
		writeStoreError(w, err)
		return
	}
	s.writeMe(w, r, http.StatusOK, u)
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	if token, ok := auth.TokenFromRequest(r); ok {
		if err := s.Store.DeleteSession(r.Context(), auth.HashToken(token)); err != nil {
			slog.Error("delete session", "err", err)
		}
	}
	auth.ClearSessionCookie(w, s.secureCookies(r))
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) resendVerification(w http.ResponseWriter, r *http.Request) {
	if !s.allowAuth(w, r) {
		return
	}
	var in credentials
	if !decode(w, r, &in) {
		return
	}
	email := store.NormalizeEmail(in.Email)
	if !s.allowMail(w, email) {
		return
	}
	// Same response whether or not the account exists.
	if u, err := s.Store.GetUserByEmail(r.Context(), email); err == nil && !u.Verified() && !u.Disabled() {
		if err := s.sendVerification(r, u); err != nil {
			slog.Error("send verification email", "err", err)
		}
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) forgotPassword(w http.ResponseWriter, r *http.Request) {
	if !s.allowAuth(w, r) {
		return
	}
	var in credentials
	if !decode(w, r, &in) {
		return
	}
	email := store.NormalizeEmail(in.Email)
	if !s.allowMail(w, email) {
		return
	}
	if u, err := s.Store.GetUserByEmail(r.Context(), email); err == nil && !u.Disabled() {
		if err := s.sendReset(r, u); err != nil {
			slog.Error("send reset email", "err", err)
		}
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) sendReset(r *http.Request, u *store.User) error {
	token, err := auth.NewToken()
	if err != nil {
		return err
	}
	if err := s.Store.CreateEmailToken(r.Context(), u.ID, store.PurposeReset, auth.HashToken(token), time.Now().Add(resetTokenTTL)); err != nil {
		return err
	}
	return s.Mailer.SendReset(r.Context(), u.Email, s.baseURL(r)+"/reset-password?token="+token)
}

func (s *Server) resetPassword(w http.ResponseWriter, r *http.Request) {
	if !s.allowAuth(w, r) {
		return
	}
	var in struct {
		Token    string `json:"token"`
		Password string `json:"password"`
	}
	if !decode(w, r, &in) {
		return
	}
	hash, err := auth.HashPassword(in.Password)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	uid, err := s.Store.ConsumeEmailToken(r.Context(), store.PurposeReset, auth.HashToken(in.Token))
	if err != nil {
		writeStoreError(w, err)
		return
	}
	if err := s.Store.SetPassword(r.Context(), uid, hash); err != nil {
		writeStoreError(w, err)
		return
	}
	// Following a reset link proves control of the mailbox.
	if err := s.Store.SetVerified(r.Context(), uid); err != nil {
		slog.Error("set verified", "err", err)
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// verify consumes an email-verification link, signs the user in and goes home.
func (s *Server) verify(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	uid, err := s.Store.ConsumeEmailToken(r.Context(), store.PurposeVerify, auth.HashToken(token))
	if err != nil {
		if !errors.Is(err, store.ErrInvalidToken) {
			slog.Error("verify token", "err", err)
		}
		http.Redirect(w, r, "/?verify=invalid", http.StatusSeeOther)
		return
	}
	ctx := r.Context()
	u, err := s.Store.GetUserByID(ctx, uid)
	if err != nil {
		slog.Error("verify user", "err", err)
		http.Redirect(w, r, "/?verify=invalid", http.StatusSeeOther)
		return
	}
	if err := s.Store.SetVerified(ctx, uid); err != nil {
		slog.Error("set verified", "err", err)
		http.Redirect(w, r, "/?verify=invalid", http.StatusSeeOther)
		return
	}
	if _, err := s.Store.AcceptInvites(ctx, uid, u.Email); err != nil {
		slog.Error("accept invites", "err", err)
	}
	if u.Disabled() {
		http.Redirect(w, r, "/?verify=disabled", http.StatusSeeOther)
		return
	}
	if err := s.startSession(w, r, u); err != nil {
		slog.Error("start session", "err", err)
		http.Redirect(w, r, "/?verify=ok", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

type meResponse struct {
	User                 *store.User `json:"user"`
	IsAdmin              bool        `json:"is_admin"`
	Plan                 string      `json:"plan"`
	Quota                int         `json:"quota"`
	PageCount            int         `json:"page_count"`
	BillingEnabled       bool        `json:"billing_enabled"`
	StripePublishableKey string      `json:"stripe_publishable_key,omitempty"`
}

func (s *Server) writeMe(w http.ResponseWriter, r *http.Request, status int, u *store.User) {
	n, err := s.Store.CountOwned(r.Context(), u.ID)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	plan := "free"
	if config.IsPro(u) {
		plan = "pro"
	}
	writeJSON(w, status, meResponse{
		User:                 u,
		IsAdmin:              s.Config.IsAdmin(u.Email) && u.Verified(),
		Plan:                 plan,
		Quota:                s.Config.Quota(u),
		PageCount:            n,
		BillingEnabled:       s.Billing != nil && s.Billing.Enabled(),
		StripePublishableKey: s.Config.StripePublishableKey,
	})
}

func (s *Server) me(w http.ResponseWriter, r *http.Request) {
	u, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	s.writeMe(w, r, http.StatusOK, u)
}

// ---- library / claim ----

// checkQuota returns ErrQuotaExceeded if the user can't own another page.
func (s *Server) checkQuota(r *http.Request, u *store.User) error {
	if !u.Verified() {
		return store.ErrUnverified
	}
	n, err := s.Store.CountOwned(r.Context(), u.ID)
	if err != nil {
		return err
	}
	if n >= s.Config.Quota(u) {
		return store.ErrQuotaExceeded
	}
	return nil
}

func (s *Server) claim(w http.ResponseWriter, r *http.Request) {
	u, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	var in struct {
		Key string `json:"key"`
	}
	if !decode(w, r, &in) {
		return
	}
	key, err := keys.Parse(in.Key)
	if err != nil {
		writeError(w, http.StatusNotFound, store.ErrNotFound.Error())
		return
	}
	// Claiming a page you already own doesn't use quota.
	if existing, err := s.Store.GetForUser(r.Context(), key, u.ID); err == nil && existing.Role == store.RoleOwner {
		writeJSON(w, http.StatusOK, s.response(r, existing))
		return
	}
	if err := s.checkQuota(r, u); err != nil {
		writeStoreError(w, err)
		return
	}
	a, err := s.Store.Claim(r.Context(), key, u.ID)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, s.response(r, a))
}

type libraryItem struct {
	store.PageSummary
	links.Links
}

func (s *Server) library(w http.ResponseWriter, r *http.Request) {
	u, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	pages, err := s.Store.ListOwnedAndShared(r.Context(), u.ID)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	base := s.baseURL(r)
	items := make([]libraryItem, 0, len(pages))
	for _, p := range pages {
		// Pages assigned an owner before their read key was stored have no
		// links until someone opens them by key (see Store.backfillReadKey).
		var l links.Links
		if p.ReadKey != "" {
			l = links.For(base, p.ReadKey, "")
		}
		items = append(items, libraryItem{PageSummary: p, Links: l})
	}
	writeJSON(w, http.StatusOK, map[string]any{"pages": items})
}

// ---- shares ----

// ownedPage loads the page behind {key} and requires the caller to own it.
func (s *Server) ownedPage(w http.ResponseWriter, r *http.Request) (*store.User, *store.Access, bool) {
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
	if a.Role != store.RoleOwner {
		writeError(w, http.StatusForbidden, "only the owner can manage sharing")
		return nil, nil, false
	}
	return u, a, true
}

func (s *Server) writeShares(w http.ResponseWriter, r *http.Request, status int, pageID int64) {
	shares, invites, err := s.Store.ListShares(r.Context(), pageID)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, status, map[string]any{"shares": shares, "invites": invites})
}

func (s *Server) listShares(w http.ResponseWriter, r *http.Request) {
	_, a, ok := s.ownedPage(w, r)
	if !ok {
		return
	}
	s.writeShares(w, r, http.StatusOK, a.ID)
}

func (s *Server) addShare(w http.ResponseWriter, r *http.Request) {
	u, a, ok := s.ownedPage(w, r)
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
		in.Role = store.RoleViewer
	}
	if in.Role != store.RoleViewer && in.Role != store.RoleEditor {
		writeError(w, http.StatusBadRequest, `role must be "viewer" or "editor"`)
		return
	}
	if !s.allowMail(w, "share:"+u.Email) {
		return
	}
	_, err = s.Store.ShareWith(r.Context(), a.ID, u.ID, email, in.Role)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	inviter := u.DisplayName
	if inviter == "" {
		inviter = u.Email
	}
	if err := s.Mailer.SendInvite(r.Context(), email, s.baseURL(r)+"/"+a.ReadKey, inviter); err != nil {
		slog.Error("send invite email", "err", err)
	}
	s.writeShares(w, r, http.StatusOK, a.ID)
}

func (s *Server) removeShare(w http.ResponseWriter, r *http.Request) {
	_, a, ok := s.ownedPage(w, r)
	if !ok {
		return
	}
	removed, err := s.Store.RemoveShare(r.Context(), a.ID, r.PathValue("email"))
	if err != nil {
		writeStoreError(w, err)
		return
	}
	if !removed {
		writeError(w, http.StatusNotFound, "no such share")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---- connected apps ----

func (s *Server) listConnections(w http.ResponseWriter, r *http.Request) {
	u, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	apps, err := s.Store.ListConnectedApps(r.Context(), u.ID)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	for i := range apps {
		if apps[i].ClientName == "" {
			apps[i].ClientName = appLabel(apps[i].ClientID)
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"apps": apps})
}

// appLabel names a client without a registered name: the host of a URL
// client ID, or a generic label.
func appLabel(clientID string) string {
	if oauth.IsCIMDClientID(clientID) {
		if u, err := url.Parse(clientID); err == nil && u.Hostname() != "" {
			return u.Hostname()
		}
	}
	return "Unnamed app"
}

// revokeConnection signs an app out by revoking every token it holds for the
// caller. The client ID is in the body because URL client IDs contain slashes.
func (s *Server) revokeConnection(w http.ResponseWriter, r *http.Request) {
	u, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	var in struct {
		ClientID string `json:"client_id"`
	}
	if !decode(w, r, &in) {
		return
	}
	n, err := s.Store.RevokeClientTokens(r.Context(), u.ID, in.ClientID)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	if n == 0 {
		writeError(w, http.StatusNotFound, "no such connection")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---- billing ----

func (s *Server) checkout(w http.ResponseWriter, r *http.Request) {
	u, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	if s.Billing == nil || !s.Billing.Enabled() {
		writeError(w, http.StatusServiceUnavailable, "billing is not available")
		return
	}
	if config.IsPro(u) {
		writeError(w, http.StatusConflict, "already subscribed; use the billing portal to manage your plan")
		return
	}
	base := s.baseURL(r)
	url, err := s.Billing.CreateCheckoutSession(r.Context(), u, base+"/?billing=success", base+"/?billing=cancelled")
	if err != nil {
		writeBillingError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"url": url})
}

func (s *Server) portal(w http.ResponseWriter, r *http.Request) {
	u, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	if s.Billing == nil {
		writeError(w, http.StatusServiceUnavailable, "billing is not available")
		return
	}
	url, err := s.Billing.CreatePortalSession(r.Context(), u, s.baseURL(r)+"/")
	if err != nil {
		writeBillingError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"url": url})
}

func writeBillingError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, billing.ErrNotConfigured):
		writeError(w, http.StatusServiceUnavailable, "billing is not available")
	case errors.Is(err, billing.ErrNoCustomer):
		writeError(w, http.StatusConflict, err.Error())
	default:
		slog.Error("billing", "err", err)
		writeError(w, http.StatusBadGateway, "billing provider error")
	}
}

func (s *Server) stripeWebhook(w http.ResponseWriter, r *http.Request) {
	if s.Billing == nil {
		writeError(w, http.StatusServiceUnavailable, "billing is not available")
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
	if err != nil {
		writeError(w, http.StatusRequestEntityTooLarge, "payload too large")
		return
	}
	err = s.Billing.HandleWebhook(r.Context(), body, r.Header.Get("Stripe-Signature"))
	switch {
	case err == nil:
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	case errors.Is(err, billing.ErrBadSignature):
		writeError(w, http.StatusBadRequest, "invalid signature")
	case errors.Is(err, billing.ErrNotConfigured):
		writeError(w, http.StatusServiceUnavailable, "billing is not available")
	default:
		// 5xx makes Stripe retry delivery.
		slog.Error("stripe webhook", "err", err)
		writeError(w, http.StatusInternalServerError, "webhook processing failed")
	}
}

// ---- admin ----

func (s *Server) adminUsers(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	users, err := s.Store.ListUsers(r.Context(), limit, offset)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"users": users})
}

func (s *Server) adminDisable(w http.ResponseWriter, r *http.Request) {
	admin, ok := s.requireAdmin(w, r)
	if !ok {
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}
	disabled := true
	if r.ContentLength != 0 {
		var in struct {
			Disabled *bool `json:"disabled"`
		}
		if !decode(w, r, &in) {
			return
		}
		if in.Disabled != nil {
			disabled = *in.Disabled
		}
	}
	if disabled && id == admin.ID {
		writeError(w, http.StatusConflict, "you can't disable your own account")
		return
	}
	if err := s.Store.SetDisabled(r.Context(), id, disabled); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "user not found")
			return
		}
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": id, "disabled": disabled})
}
