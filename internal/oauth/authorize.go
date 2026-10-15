package oauth

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"html/template"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/stewartmckee/canvasbin/internal/auth"
	"github.com/stewartmckee/canvasbin/internal/store"
)

// authRequest is a validated authorization request.
type authRequest struct {
	Client      *Client
	RedirectURI string
	State       string
	Scope       string // normalized
	Resource    string
	Challenge   string
}

// authFailure is a request we could not honour. When Page is set the error is
// shown to the user (the client or redirect URI can't be trusted); otherwise it
// is returned to the client via redirect.
type authFailure struct {
	Page        string
	Code        string
	Description string
	RedirectURI string
	State       string
}

// parseAuthRequest validates the OAuth parameters in v.
func (p *Provider) parseAuthRequest(r *http.Request, v url.Values, base string) (*authRequest, *authFailure) {
	client, err := p.resolveClient(r, v.Get("client_id"))
	if err != nil {
		return nil, &authFailure{Page: err.Error()}
	}
	redirect := v.Get("redirect_uri")
	if redirect == "" {
		if len(client.RedirectURIs) != 1 {
			return nil, &authFailure{Page: "redirect_uri is required"}
		}
		redirect = client.RedirectURIs[0]
	}
	if !RedirectAllowed(client.RedirectURIs, redirect) {
		return nil, &authFailure{Page: "redirect_uri does not match the client's registered redirect URIs"}
	}
	state := v.Get("state")
	fail := func(code, desc string) (*authRequest, *authFailure) {
		return nil, &authFailure{Code: code, Description: desc, RedirectURI: redirect, State: state}
	}
	if v.Get("response_type") != "code" {
		return fail("unsupported_response_type", "response_type must be code")
	}
	challenge := v.Get("code_challenge")
	if !ValidChallenge(challenge) {
		return fail("invalid_request", "code_challenge is required (S256)")
	}
	if m := v.Get("code_challenge_method"); m != "S256" {
		return fail("invalid_request", "code_challenge_method must be S256")
	}
	resource := ResourceURL(base)
	if got := v.Get("resource"); got != "" && got != resource {
		return fail("invalid_target", "resource must be "+resource)
	}
	return &authRequest{
		Client: client, RedirectURI: redirect, State: state,
		Scope: NormalizeScope(v.Get("scope")), Resource: resource, Challenge: challenge,
	}, nil
}

func (p *Provider) authorizeGet(w http.ResponseWriter, r *http.Request) {
	if !p.allow(r) {
		errorPage(w, http.StatusTooManyRequests, "Too many attempts, try again shortly.")
		return
	}
	base := p.Base(r)
	req, fail := p.parseAuthRequest(r, r.URL.Query(), base)
	if fail != nil {
		p.respondFailure(w, r, base, fail)
		return
	}
	u := p.SessionUser(r)
	if u == nil {
		http.Redirect(w, r, "/login?next="+url.QueryEscape(r.URL.RequestURI()), http.StatusFound)
		return
	}
	if !u.Verified() {
		errorPage(w, http.StatusForbidden, "Verify your email address before connecting an app.")
		return
	}
	p.renderConsent(w, r, req, u)
}

func (p *Provider) authorizePost(w http.ResponseWriter, r *http.Request) {
	if !p.allow(r) {
		errorPage(w, http.StatusTooManyRequests, "Too many attempts, try again shortly.")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxFormBytes)
	if err := r.ParseForm(); err != nil {
		errorPage(w, http.StatusBadRequest, "Invalid form submission.")
		return
	}
	base := p.Base(r)
	req, fail := p.parseAuthRequest(r, r.PostForm, base)
	if fail != nil {
		p.respondFailure(w, r, base, fail)
		return
	}
	u := p.SessionUser(r)
	if u == nil || !u.Verified() {
		errorPage(w, http.StatusUnauthorized, "Your session has expired. Sign in and start again from the app.")
		return
	}
	token, _ := auth.TokenFromRequest(r)
	if subtle.ConstantTimeCompare([]byte(csrfToken(token)), []byte(r.PostForm.Get("csrf"))) != 1 {
		errorPage(w, http.StatusForbidden, "This form has expired. Start again from the app.")
		return
	}
	if r.PostForm.Get("action") != "approve" {
		redirectClient(w, r, req.RedirectURI, base, req.State, map[string]string{
			"error": "access_denied", "error_description": "The user denied the request",
		})
		return
	}

	code, err := auth.NewToken()
	if err != nil {
		slog.Error("oauth code", "err", err)
		errorPage(w, http.StatusInternalServerError, "Something went wrong. Try again.")
		return
	}
	err = p.Store.CreateAuthCode(r.Context(), auth.HashToken(code), store.OAuthAuthCode{
		ClientID: req.Client.ID, UserID: u.ID, RedirectURI: req.RedirectURI,
		CodeChallenge: req.Challenge, CodeChallengeMethod: "S256",
		Scope: req.Scope, Resource: req.Resource, ExpiresAt: time.Now().Add(CodeTTL),
	})
	if err != nil {
		slog.Error("oauth code", "err", err)
		errorPage(w, http.StatusInternalServerError, "Something went wrong. Try again.")
		return
	}
	redirectClient(w, r, req.RedirectURI, base, req.State, map[string]string{"code": code})
}

func (p *Provider) respondFailure(w http.ResponseWriter, r *http.Request, base string, f *authFailure) {
	if f.Page != "" {
		errorPage(w, http.StatusBadRequest, f.Page)
		return
	}
	redirectClient(w, r, f.RedirectURI, base, f.State, map[string]string{
		"error": f.Code, "error_description": f.Description,
	})
}

// redirectClient sends the browser back to the client with params, the state
// and the issuer (RFC 9207 mix-up defence).
func redirectClient(w http.ResponseWriter, r *http.Request, redirectURI, issuer, state string, params map[string]string) {
	u, err := url.Parse(redirectURI)
	if err != nil {
		errorPage(w, http.StatusBadRequest, "Invalid redirect_uri.")
		return
	}
	q := u.Query()
	for k, v := range params {
		q.Set(k, v)
	}
	if state != "" {
		q.Set("state", state)
	}
	q.Set("iss", issuer)
	u.RawQuery = q.Encode()
	w.Header().Set("Cache-Control", "no-store")
	http.Redirect(w, r, u.String(), http.StatusSeeOther)
}

// csrfToken binds the consent form to the browser session. The session token is
// a secret only the signed-in browser holds, so a cross-site page can't forge it.
func csrfToken(sessionToken string) string {
	sum := sha256.Sum256([]byte("canvasbin-oauth-consent:" + sessionToken))
	return hex.EncodeToString(sum[:])
}

// ---- pages ----

const pageCSS = `body{font:16px/1.5 system-ui,sans-serif;margin:0;background:#f6f6f7;color:#1a1a1a}
main{max-width:30rem;margin:10vh auto;background:#fff;padding:2rem;border-radius:12px;box-shadow:0 1px 8px rgba(0,0,0,.1)}
h1{font-size:1.25rem;margin:0 0 1rem}ul{padding-left:1.25rem}.muted{color:#666;font-size:.875rem;word-break:break-all}
.row{display:flex;gap:.75rem;margin-top:1.5rem}button{font:inherit;padding:.6rem 1.2rem;border-radius:8px;border:1px solid #ccc;background:#fff;cursor:pointer}
button.primary{background:#1a1a1a;color:#fff;border-color:#1a1a1a}`

var consentTmpl = template.Must(template.New("consent").Parse(`<!doctype html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<meta name="robots" content="noindex"><title>Connect to Canvasbin</title><style>` + pageCSS + `</style></head>
<body><main>
<h1>Allow {{if .Name}}{{.Name}}{{else}}this app{{end}} to use your Canvasbin account?</h1>
<p class="muted">Signed in as {{.Email}}</p>
<p>This app will be able to:</p>
<ul>
{{range .Scopes}}<li>{{.}}</li>{{end}}
</ul>
<p class="muted">Client: {{.ClientID}}<br>Returns to: {{.RedirectHost}}</p>
<form method="post" action="/oauth/authorize">
<input type="hidden" name="response_type" value="code">
<input type="hidden" name="client_id" value="{{.ClientID}}">
<input type="hidden" name="redirect_uri" value="{{.RedirectURI}}">
<input type="hidden" name="state" value="{{.State}}">
<input type="hidden" name="scope" value="{{.Scope}}">
<input type="hidden" name="resource" value="{{.Resource}}">
<input type="hidden" name="code_challenge" value="{{.Challenge}}">
<input type="hidden" name="code_challenge_method" value="S256">
<input type="hidden" name="csrf" value="{{.CSRF}}">
<div class="row">
<button class="primary" type="submit" name="action" value="approve">Allow</button>
<button type="submit" name="action" value="deny">Deny</button>
</div>
</form>
</main></body></html>`))

var errorTmpl = template.Must(template.New("error").Parse(`<!doctype html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<meta name="robots" content="noindex"><title>Canvasbin</title><style>` + pageCSS + `</style></head>
<body><main><h1>Can't connect this app</h1><p>{{.}}</p></main></body></html>`))

func setPageHeaders(w http.ResponseWriter) {
	h := w.Header()
	h.Set("Content-Type", "text/html; charset=utf-8")
	h.Set("Cache-Control", "no-store")
	h.Set("X-Frame-Options", "DENY")
	// No form-action: browsers apply it to the redirect back to the client too.
	h.Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; frame-ancestors 'none'; base-uri 'none'")
}

func errorPage(w http.ResponseWriter, status int, msg string) {
	setPageHeaders(w)
	w.WriteHeader(status)
	_ = errorTmpl.Execute(w, msg)
}

var scopeLabels = map[string]string{
	ScopeRead:    "Read your pages",
	ScopeWrite:   "Create, edit, share and delete your pages",
	ScopeOffline: "Stay connected until you revoke access",
}

func (p *Provider) renderConsent(w http.ResponseWriter, r *http.Request, req *authRequest, u *store.User) {
	token, _ := auth.TokenFromRequest(r)
	var scopes []string
	for _, s := range strings.Fields(req.Scope) {
		scopes = append(scopes, scopeLabels[s])
	}
	host := ""
	if ru, err := url.Parse(req.RedirectURI); err == nil {
		host = ru.Host
	}
	setPageHeaders(w)
	_ = consentTmpl.Execute(w, map[string]any{
		"Name": req.Client.Name, "Email": u.Email, "Scopes": scopes, "Scope": req.Scope,
		"ClientID": req.Client.ID, "RedirectURI": req.RedirectURI, "RedirectHost": host,
		"State": req.State, "Resource": req.Resource, "Challenge": req.Challenge,
		"CSRF": csrfToken(token),
	})
}
