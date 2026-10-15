package oauth

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/stewartmckee/canvasbin/internal/auth"
	"github.com/stewartmckee/canvasbin/internal/links"
	"github.com/stewartmckee/canvasbin/internal/store"
)

type registrationRequest struct {
	RedirectURIs            []string `json:"redirect_uris"`
	ClientName              string   `json:"client_name"`
	TokenEndpointAuthMethod string   `json:"token_endpoint_auth_method"`
	GrantTypes              []string `json:"grant_types"`
	ResponseTypes           []string `json:"response_types"`
}

func (p *Provider) allow(r *http.Request) bool {
	return p.Limiter == nil || p.Limiter.Allow(links.ClientIP(r))
}

// register is RFC 7591 dynamic client registration for public clients.
func (p *Provider) register(w http.ResponseWriter, r *http.Request) {
	if !p.allow(r) {
		oauthError(w, http.StatusTooManyRequests, "invalid_request", "too many attempts, try again shortly")
		return
	}
	var in registrationRequest
	r.Body = http.MaxBytesReader(w, r.Body, maxFormBytes)
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		oauthError(w, http.StatusBadRequest, "invalid_client_metadata", "request body must be a JSON object")
		return
	}
	if len(in.RedirectURIs) == 0 || len(in.RedirectURIs) > maxRedirectURIs {
		oauthError(w, http.StatusBadRequest, "invalid_redirect_uri", "provide between 1 and 10 redirect_uris")
		return
	}
	for _, ru := range in.RedirectURIs {
		if err := ValidateRedirectURI(ru); err != nil {
			oauthError(w, http.StatusBadRequest, "invalid_redirect_uri", err.Error())
			return
		}
	}
	if m := in.TokenEndpointAuthMethod; m != "" && m != "none" {
		oauthError(w, http.StatusBadRequest, "invalid_client_metadata", `only token_endpoint_auth_method "none" is supported`)
		return
	}
	for _, g := range in.GrantTypes {
		if g != "authorization_code" && g != "refresh_token" {
			oauthError(w, http.StatusBadRequest, "invalid_client_metadata", "unsupported grant_type "+quote(g))
			return
		}
	}
	for _, rt := range in.ResponseTypes {
		if rt != "code" {
			oauthError(w, http.StatusBadRequest, "invalid_client_metadata", `response_types must be ["code"]`)
			return
		}
	}

	tok, err := auth.NewToken()
	if err != nil {
		slog.Error("oauth register", "err", err)
		oauthError(w, http.StatusInternalServerError, "server_error", "internal error")
		return
	}
	name := cleanName(in.ClientName)
	c := store.OAuthClient{
		ClientID:                "cbc_" + tok[:22],
		RedirectURIs:            in.RedirectURIs,
		ClientName:              name,
		TokenEndpointAuthMethod: "none",
	}
	if err := p.Store.CreateOAuthClient(r.Context(), c); err != nil {
		slog.Error("oauth register", "err", err)
		oauthError(w, http.StatusInternalServerError, "server_error", "internal error")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"client_id":                  c.ClientID,
		"client_id_issued_at":        time.Now().Unix(),
		"client_name":                name,
		"redirect_uris":              c.RedirectURIs,
		"token_endpoint_auth_method": "none",
		"grant_types":                []string{"authorization_code", "refresh_token"},
		"response_types":             []string{"code"},
	})
}

func quote(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

// resolveClient finds the client for a client_id: a metadata document for https
// URLs, otherwise a dynamically registered client.
func (p *Provider) resolveClient(r *http.Request, clientID string) (*Client, error) {
	if clientID == "" {
		return nil, errors.New("client_id is required")
	}
	if IsCIMDClientID(clientID) {
		return p.cimd.Get(r.Context(), clientID)
	}
	c, err := p.Store.GetOAuthClient(r.Context(), clientID)
	if errors.Is(err, store.ErrNotFound) {
		return nil, errors.New("unknown client_id")
	}
	if err != nil {
		slog.Error("oauth client lookup", "err", err)
		return nil, errors.New("could not look up client")
	}
	return &Client{ID: c.ClientID, Name: c.ClientName, RedirectURIs: c.RedirectURIs}, nil
}
