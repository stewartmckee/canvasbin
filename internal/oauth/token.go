package oauth

import (
	"context"
	"errors"
	"log/slog"
	"mime"
	"net/http"
	"strings"
	"time"

	"github.com/stewartmckee/canvasbin/internal/auth"
	"github.com/stewartmckee/canvasbin/internal/store"
)

const (
	accessPrefix  = "cba_"
	refreshPrefix = "cbr_"
)

// token is the token endpoint: authorization_code and refresh_token grants.
func (p *Provider) token(w http.ResponseWriter, r *http.Request) {
	mt, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if mt != "application/x-www-form-urlencoded" {
		oauthError(w, http.StatusBadRequest, "invalid_request", "Content-Type must be application/x-www-form-urlencoded")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxFormBytes)
	if err := r.ParseForm(); err != nil {
		oauthError(w, http.StatusBadRequest, "invalid_request", "invalid form body")
		return
	}
	// Parameters are only accepted in the body, never the query string.
	f := r.PostForm
	switch f.Get("grant_type") {
	case "authorization_code":
		p.codeGrant(w, r, f)
	case "refresh_token":
		p.refreshGrant(w, r, f)
	default:
		oauthError(w, http.StatusBadRequest, "unsupported_grant_type", "grant_type must be authorization_code or refresh_token")
	}
}

type form interface{ Get(string) string }

// activeUser loads the token owner, who must still be able to sign in.
func (p *Provider) activeUser(ctx context.Context, id int64) (*store.User, bool) {
	u, err := p.Store.GetUserByID(ctx, id)
	if err != nil {
		if !errors.Is(err, store.ErrNotFound) {
			slog.Error("oauth user lookup", "err", err)
		}
		return nil, false
	}
	return u, !u.Disabled()
}

func (p *Provider) codeGrant(w http.ResponseWriter, r *http.Request, f form) {
	code, verifier, clientID := f.Get("code"), f.Get("code_verifier"), f.Get("client_id")
	if code == "" || verifier == "" || clientID == "" {
		oauthError(w, http.StatusBadRequest, "invalid_request", "code, code_verifier and client_id are required")
		return
	}
	// The code is consumed before any other check, so a failed attempt can't be retried.
	rec, err := p.Store.ConsumeAuthCode(r.Context(), auth.HashToken(code))
	if errors.Is(err, store.ErrInvalidToken) {
		oauthError(w, http.StatusBadRequest, "invalid_grant", "authorization code is invalid, expired or already used")
		return
	}
	if err != nil {
		slog.Error("oauth consume code", "err", err)
		oauthError(w, http.StatusInternalServerError, "server_error", "internal error")
		return
	}
	if rec.ClientID != clientID || rec.RedirectURI != f.Get("redirect_uri") {
		oauthError(w, http.StatusBadRequest, "invalid_grant", "client_id or redirect_uri does not match the authorization request")
		return
	}
	if rec.CodeChallengeMethod != "S256" || !VerifyPKCE(verifier, rec.CodeChallenge) {
		oauthError(w, http.StatusBadRequest, "invalid_grant", "PKCE verification failed")
		return
	}
	if res := f.Get("resource"); res != "" && res != rec.Resource {
		oauthError(w, http.StatusBadRequest, "invalid_target", "resource does not match the authorization request")
		return
	}
	u, ok := p.activeUser(r.Context(), rec.UserID)
	if !ok {
		oauthError(w, http.StatusBadRequest, "invalid_grant", "account is not available")
		return
	}
	if !u.Verified() {
		oauthError(w, http.StatusBadRequest, "invalid_grant", "email address is not verified")
		return
	}
	next, resp, err := newTokenPair(u.ID, clientID, rec.Scope, rec.Resource)
	if err == nil {
		err = p.Store.CreateOAuthToken(r.Context(), next)
	}
	if err != nil {
		slog.Error("oauth issue token", "err", err)
		oauthError(w, http.StatusInternalServerError, "server_error", "internal error")
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (p *Provider) refreshGrant(w http.ResponseWriter, r *http.Request, f form) {
	refresh, clientID := f.Get("refresh_token"), f.Get("client_id")
	if refresh == "" || clientID == "" {
		oauthError(w, http.StatusBadRequest, "invalid_request", "refresh_token and client_id are required")
		return
	}
	old, err := p.Store.GetOAuthTokenByRefresh(r.Context(), auth.HashToken(refresh))
	if errors.Is(err, store.ErrInvalidToken) {
		oauthError(w, http.StatusBadRequest, "invalid_grant", "refresh token is invalid, expired or revoked")
		return
	}
	if err != nil {
		slog.Error("oauth refresh lookup", "err", err)
		oauthError(w, http.StatusInternalServerError, "server_error", "internal error")
		return
	}
	if old.ClientID != clientID {
		oauthError(w, http.StatusBadRequest, "invalid_grant", "refresh token was issued to a different client")
		return
	}
	if res := f.Get("resource"); res != "" && res != old.Resource {
		oauthError(w, http.StatusBadRequest, "invalid_target", "resource does not match the original grant")
		return
	}
	scope := old.Scope
	if req := strings.TrimSpace(f.Get("scope")); req != "" {
		if !scopeSubset(req, old.Scope) {
			oauthError(w, http.StatusBadRequest, "invalid_scope", "scope exceeds the original grant")
			return
		}
		scope = strings.Join(strings.Fields(req), " ")
	}
	u, ok := p.activeUser(r.Context(), old.UserID)
	if !ok || !u.Verified() {
		oauthError(w, http.StatusBadRequest, "invalid_grant", "account is not available")
		return
	}
	next, resp, err := newTokenPair(u.ID, clientID, scope, old.Resource)
	if err == nil {
		err = p.Store.RotateOAuthToken(r.Context(), old.ID, next)
	}
	if errors.Is(err, store.ErrInvalidToken) { // lost a race: already rotated
		oauthError(w, http.StatusBadRequest, "invalid_grant", "refresh token is invalid, expired or revoked")
		return
	}
	if err != nil {
		slog.Error("oauth rotate token", "err", err)
		oauthError(w, http.StatusInternalServerError, "server_error", "internal error")
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

// newTokenPair mints an access/refresh pair. Only the hashes go in the store
// record; the raw tokens appear solely in the response.
func newTokenPair(userID int64, clientID, scope, resource string) (store.OAuthToken, map[string]any, error) {
	at, err := auth.NewToken()
	if err != nil {
		return store.OAuthToken{}, nil, err
	}
	rt, err := auth.NewToken()
	if err != nil {
		return store.OAuthToken{}, nil, err
	}
	access, refresh := accessPrefix+at, refreshPrefix+rt
	now := time.Now()
	refreshExp := now.Add(RefreshTTL)
	rec := store.OAuthToken{
		AccessHash: auth.HashToken(access), RefreshHash: auth.HashToken(refresh),
		ClientID: clientID, UserID: userID, Scope: scope, Resource: resource,
		AccessExpiresAt: now.Add(AccessTTL), RefreshExpiresAt: &refreshExp,
	}
	return rec, map[string]any{
		"access_token":  access,
		"token_type":    "Bearer",
		"expires_in":    int(AccessTTL / time.Second),
		"refresh_token": refresh,
		"scope":         scope,
	}, nil
}

// revoke is RFC 7009 token revocation. It answers 200 whether or not the token existed.
func (p *Provider) revoke(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxFormBytes)
	if err := r.ParseForm(); err != nil {
		oauthError(w, http.StatusBadRequest, "invalid_request", "invalid form body")
		return
	}
	if t := r.PostForm.Get("token"); t != "" {
		if err := p.Store.RevokeOAuthTokenByHash(r.Context(), auth.HashToken(t)); err != nil {
			slog.Error("oauth revoke", "err", err)
		}
	}
	setCORS(w)
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
}

// ---- bearer validation (protected resource side) ----

// Identity is a validated bearer token.
type Identity struct {
	User     *store.User
	Scope    string
	ClientID string
}

// BearerToken extracts the token from an "Authorization: Bearer" header.
func BearerToken(r *http.Request) (string, bool) {
	h := r.Header.Get("Authorization")
	if len(h) < 8 || !strings.EqualFold(h[:7], "bearer ") {
		return "", false
	}
	t := strings.TrimSpace(h[7:])
	return t, t != ""
}

// Authenticate validates the request's bearer token: hash lookup, not revoked,
// not expired, issued for this server's MCP resource, owner still enabled. It
// returns nil for anything else.
func (p *Provider) Authenticate(r *http.Request) *Identity {
	tok, ok := BearerToken(r)
	if !ok {
		return nil
	}
	rec, err := p.Store.GetOAuthTokenByAccess(r.Context(), auth.HashToken(tok))
	if err != nil {
		if !errors.Is(err, store.ErrInvalidToken) {
			slog.Error("oauth token lookup", "err", err)
		}
		return nil
	}
	if rec.Resource != ResourceURL(p.Base(r)) {
		return nil
	}
	u, ok := p.activeUser(r.Context(), rec.UserID)
	if !ok {
		return nil
	}
	return &Identity{User: u, Scope: rec.Scope, ClientID: rec.ClientID}
}

// UserFromRequest resolves a bearer access token to its user (nil if none or invalid).
func (p *Provider) UserFromRequest(r *http.Request) *store.User {
	if id := p.Authenticate(r); id != nil {
		return id.User
	}
	return nil
}

// ScopeFromRequest returns the scope granted to the request's bearer token.
func (p *Provider) ScopeFromRequest(r *http.Request) string {
	if id := p.Authenticate(r); id != nil {
		return id.Scope
	}
	return ""
}
