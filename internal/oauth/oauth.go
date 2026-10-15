// Package oauth implements the OAuth 2.1 authorization server (and the token
// check for the protected resource) that lets MCP clients act as a Canvasbin
// account. It is only mounted when ACCOUNTS is on.
//
// Supported: authorization code + PKCE (S256 only), refresh token rotation,
// RFC 7591 dynamic client registration, and Client ID Metadata Documents.
// All clients are public (no client secret). Tokens are stored as SHA-256
// hashes; the raw values are shown to the client once and never logged.
package oauth

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/stewartmckee/canvasbin/internal/config"
	"github.com/stewartmckee/canvasbin/internal/links"
	"github.com/stewartmckee/canvasbin/internal/ratelimit"
	"github.com/stewartmckee/canvasbin/internal/store"
)

const (
	ScopeRead    = "pages:read"
	ScopeWrite   = "pages:write"
	ScopeOffline = "offline_access"

	AccessTTL  = time.Hour
	RefreshTTL = 30 * 24 * time.Hour
	CodeTTL    = 10 * time.Minute

	// maxFormBytes caps every request body this package reads.
	maxFormBytes = 64 << 10
)

// SupportedScopes is the canonical (and advertised) scope order.
var SupportedScopes = []string{ScopeRead, ScopeWrite, ScopeOffline}

// Options configures a Provider.
type Options struct {
	Store  *store.Store
	Config config.Config
	// SessionUser resolves the browser session (httpapi.Server.SessionUser).
	SessionUser func(r *http.Request) *store.User
	// Limiter, when set, throttles registration and authorization per client IP.
	Limiter *ratelimit.Limiter
}

// Provider serves the OAuth endpoints and validates bearer tokens.
type Provider struct {
	Options
	cimd *cimdFetcher
}

func New(o Options) *Provider {
	return &Provider{Options: o, cimd: newCIMDFetcher()}
}

// Base is the issuer / public origin for r.
func (p *Provider) Base(r *http.Request) string { return links.Base(p.Config.PublicBaseURL, r) }

// ResourceURL is the canonical MCP resource identifier (the audience of every token).
func ResourceURL(base string) string { return base + "/mcp" }

// Routes mounts the metadata and OAuth endpoints on mux.
func (p *Provider) Routes(mux *http.ServeMux) {
	for _, path := range []string{
		"/.well-known/oauth-protected-resource",
		"/.well-known/oauth-protected-resource/mcp",
	} {
		mux.HandleFunc("GET "+path, p.protectedResource)
		mux.HandleFunc("OPTIONS "+path, preflight)
	}
	mux.HandleFunc("GET /.well-known/oauth-authorization-server", p.authorizationServer)
	mux.HandleFunc("OPTIONS /.well-known/oauth-authorization-server", preflight)

	mux.HandleFunc("POST /oauth/register", p.register)
	mux.HandleFunc("OPTIONS /oauth/register", preflight)
	mux.HandleFunc("GET /oauth/authorize", p.authorizeGet)
	mux.HandleFunc("POST /oauth/authorize", p.authorizePost)
	mux.HandleFunc("POST /oauth/token", p.token)
	mux.HandleFunc("OPTIONS /oauth/token", preflight)
	mux.HandleFunc("POST /oauth/revoke", p.revoke)
	mux.HandleFunc("OPTIONS /oauth/revoke", preflight)
}

func (p *Provider) protectedResource(w http.ResponseWriter, r *http.Request) {
	base := p.Base(r)
	writeJSON(w, http.StatusOK, map[string]any{
		"resource":                 ResourceURL(base),
		"authorization_servers":    []string{base},
		"scopes_supported":         SupportedScopes,
		"bearer_methods_supported": []string{"header"},
		"resource_name":            "Canvasbin",
	})
}

func (p *Provider) authorizationServer(w http.ResponseWriter, r *http.Request) {
	base := p.Base(r)
	writeJSON(w, http.StatusOK, map[string]any{
		"issuer":                                         base,
		"authorization_endpoint":                         base + "/oauth/authorize",
		"token_endpoint":                                 base + "/oauth/token",
		"registration_endpoint":                          base + "/oauth/register",
		"revocation_endpoint":                            base + "/oauth/revoke",
		"response_types_supported":                       []string{"code"},
		"grant_types_supported":                          []string{"authorization_code", "refresh_token"},
		"code_challenge_methods_supported":               []string{"S256"},
		"token_endpoint_auth_methods_supported":          []string{"none"},
		"scopes_supported":                               SupportedScopes,
		"client_id_metadata_document_supported":          true,
		"authorization_response_iss_parameter_supported": true,
	})
}

// ---- scopes ----

// NormalizeScope keeps only supported scopes, in canonical order. A request
// that names no page scope gets read and write.
func NormalizeScope(requested string) string {
	want := map[string]bool{}
	for _, s := range strings.Fields(requested) {
		want[s] = true
	}
	if !want[ScopeRead] && !want[ScopeWrite] {
		want[ScopeRead], want[ScopeWrite] = true, true
	}
	var out []string
	for _, s := range SupportedScopes {
		if want[s] {
			out = append(out, s)
		}
	}
	return strings.Join(out, " ")
}

// HasScope reports whether the space-separated scope grants need. Write implies read.
func HasScope(scope, need string) bool {
	for _, s := range strings.Fields(scope) {
		if s == need || (need == ScopeRead && s == ScopeWrite) {
			return true
		}
	}
	return false
}

// scopeSubset reports whether every scope in sub is in super.
func scopeSubset(sub, super string) bool {
	have := map[string]bool{}
	for _, s := range strings.Fields(super) {
		have[s] = true
	}
	for _, s := range strings.Fields(sub) {
		if !have[s] {
			return false
		}
	}
	return true
}

// ---- http helpers ----

func setCORS(w http.ResponseWriter) {
	h := w.Header()
	h.Set("Access-Control-Allow-Origin", "*")
	h.Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	h.Set("Access-Control-Allow-Headers", "Content-Type, Authorization, MCP-Protocol-Version")
}

func preflight(w http.ResponseWriter, _ *http.Request) {
	setCORS(w)
	w.WriteHeader(http.StatusNoContent)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	setCORS(w)
	h := w.Header()
	h.Set("Content-Type", "application/json")
	h.Set("Cache-Control", "no-store")
	h.Set("Pragma", "no-cache")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func oauthError(w http.ResponseWriter, status int, code, desc string) {
	writeJSON(w, status, map[string]string{"error": code, "error_description": desc})
}
