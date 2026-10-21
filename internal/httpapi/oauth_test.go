package httpapi

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"regexp"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/stewartmckee/canvasbin/internal/config"
	"github.com/stewartmckee/canvasbin/internal/mcpserver"
	"github.com/stewartmckee/canvasbin/internal/store"
)

// newMCPEnv is newEnv with the MCP handler and OAuth wired the way main does.
func newMCPEnv(t *testing.T) *env {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	st, err := store.Open(context.Background(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(st.Close)
	m := &fakeMailer{links: map[string]string{}}
	cfg := config.Config{Accounts: true, FreeQuota: 20, ProQuota: 1000}
	s := &Server{Store: st, Config: cfg, Mailer: m, Web: fstest.MapFS{"index.html": {Data: []byte("<html></html>")}}}
	s.OAuth = s.NewOAuth()
	s.MCP = mcpserver.Handler(mcpserver.Options{
		Store: st, Config: cfg, Version: "test", Mailer: m,
		UserFromRequest: s.OAuth.UserFromRequest, ScopeFromRequest: s.OAuth.ScopeFromRequest,
	})
	ts := httptest.NewServer(s.Handler())
	t.Cleanup(ts.Close)
	return &env{srv: ts, mail: m, st: st}
}

func pkcePair() (verifier, challenge string) {
	verifier = strings.Repeat("v", 20) + base64.RawURLEncoding.EncodeToString([]byte("canvasbin-test-verifier-0123456789"))
	sum := sha256.Sum256([]byte(verifier))
	return verifier, base64.RawURLEncoding.EncodeToString(sum[:])
}

var csrfRe = regexp.MustCompile(`name="csrf" value="([^"]+)"`)

type oauthFlow struct {
	t        *testing.T
	e        *env
	user     *client
	clientID string
}

func (f *oauthFlow) authorizeQuery(challenge, extra string) string {
	q := url.Values{
		"response_type": {"code"}, "client_id": {f.clientID},
		"redirect_uri": {"http://127.0.0.1:34567/callback"}, "state": {"st4te"},
		"code_challenge": {challenge}, "code_challenge_method": {"S256"},
		"resource": {f.e.srv.URL + "/mcp"},
	}.Encode()
	return q + extra
}

// approve drives consent and returns the redirect Location.
func (f *oauthFlow) approve(query, action string) *url.URL {
	f.t.Helper()
	req, _ := http.NewRequest("GET", f.e.srv.URL+"/oauth/authorize?"+query, nil)
	req.AddCookie(&http.Cookie{Name: "canvasbin_session", Value: f.user.ck})
	resp, err := f.user.c.Do(req)
	if err != nil {
		f.t.Fatal(err)
	}
	page, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 200 {
		f.t.Fatalf("consent page: %d %s", resp.StatusCode, page)
	}
	if !strings.Contains(resp.Header.Get("Content-Security-Policy"), "frame-ancestors 'none'") {
		f.t.Fatal("consent page must forbid framing")
	}
	m := csrfRe.FindSubmatch(page)
	if m == nil {
		f.t.Fatalf("no csrf token in %s", page)
	}
	form, _ := url.ParseQuery(query)
	form.Set("csrf", string(m[1]))
	form.Set("action", action)
	return f.post(form, true)
}

// authorizeRedirect GETs /oauth/authorize as the signed-in user and expects an
// error redirect back to the client.
func (f *oauthFlow) authorizeRedirect(query string) *url.URL {
	f.t.Helper()
	req, _ := http.NewRequest("GET", f.e.srv.URL+"/oauth/authorize?"+query, nil)
	req.AddCookie(&http.Cookie{Name: "canvasbin_session", Value: f.user.ck})
	resp, err := f.user.c.Do(req)
	if err != nil {
		f.t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusSeeOther {
		f.t.Fatalf("expected redirect, got %d", resp.StatusCode)
	}
	loc, _ := url.Parse(resp.Header.Get("Location"))
	return loc
}

func (f *oauthFlow) post(form url.Values, withSession bool) *url.URL {
	f.t.Helper()
	req, _ := http.NewRequest("POST", f.e.srv.URL+"/oauth/authorize", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if withSession {
		req.AddCookie(&http.Cookie{Name: "canvasbin_session", Value: f.user.ck})
	}
	resp, err := f.user.c.Do(req)
	if err != nil {
		f.t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusSeeOther {
		f.t.Fatalf("authorize POST: %d", resp.StatusCode)
	}
	loc, err := url.Parse(resp.Header.Get("Location"))
	if err != nil {
		f.t.Fatal(err)
	}
	return loc
}

func (f *oauthFlow) token(form url.Values) (int, map[string]any) {
	f.t.Helper()
	resp, err := http.Post(f.e.srv.URL+"/oauth/token", "application/x-www-form-urlencoded", strings.NewReader(form.Encode()))
	if err != nil {
		f.t.Fatal(err)
	}
	defer resp.Body.Close()
	var out map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&out)
	return resp.StatusCode, out
}

// getCode runs authorize+approve and returns the code and the verifier used.
func (f *oauthFlow) getCode() (code, verifier string) {
	f.t.Helper()
	v, ch := pkcePair()
	loc := f.approve(f.authorizeQuery(ch, ""), "approve")
	q := loc.Query()
	if loc.Host != "127.0.0.1:34567" || q.Get("state") != "st4te" || q.Get("iss") != f.e.srv.URL || q.Get("code") == "" {
		f.t.Fatalf("unexpected redirect %s", loc)
	}
	return q.Get("code"), v
}

func (f *oauthFlow) tokenForm(code, verifier string) url.Values {
	return url.Values{
		"grant_type": {"authorization_code"}, "code": {code}, "code_verifier": {verifier},
		"client_id": {f.clientID}, "redirect_uri": {"http://127.0.0.1:34567/callback"},
		"resource": {f.e.srv.URL + "/mcp"},
	}
}

func (f *oauthFlow) mcp(token, body string) (int, http.Header, string) {
	f.t.Helper()
	req, _ := http.NewRequest("POST", f.e.srv.URL+"/mcp", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		f.t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, resp.Header, string(b)
}

func rpcCall(tool, args string) string {
	return `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"` + tool + `","arguments":` + args + `}}`
}

func TestOAuthFlowEndToEnd(t *testing.T) {
	e := newMCPEnv(t)
	user := e.newClient(t)
	email := fmt.Sprintf("oauth-e2e-%d@example.com", time.Now().UnixNano())
	user.signup(email)
	f := &oauthFlow{t: t, e: e, user: user}

	// Discovery.
	for _, p := range []string{"/.well-known/oauth-protected-resource", "/.well-known/oauth-protected-resource/mcp", "/.well-known/oauth-authorization-server"} {
		code, out := user.do("GET", p, nil)
		if code != 200 {
			t.Fatalf("%s: %d", p, code)
		}
		if p != "/.well-known/oauth-authorization-server" && out["resource"] != e.srv.URL+"/mcp" {
			t.Fatalf("resource = %v", out["resource"])
		}
	}

	// Dynamic registration: validation, then success.
	anon := e.newClient(t)
	anon.want(400, "POST", "/oauth/register", map[string]any{"redirect_uris": []string{"http://evil.example/cb"}})
	anon.want(400, "POST", "/oauth/register", map[string]any{"redirect_uris": []string{"https://app.example/cb"}, "token_endpoint_auth_method": "client_secret_basic"})
	reg := anon.want(201, "POST", "/oauth/register", map[string]any{
		"client_name": "Test Client", "redirect_uris": []string{"http://127.0.0.1:1234/callback"},
	})
	f.clientID, _ = reg["client_id"].(string)
	if f.clientID == "" || reg["token_endpoint_auth_method"] != "none" {
		t.Fatalf("registration: %v", reg)
	}

	// Not signed in: bounced to /login with next.
	_, ch := pkcePair()
	req, _ := http.NewRequest("GET", e.srv.URL+"/oauth/authorize?"+f.authorizeQuery(ch, ""), nil)
	resp, err := anon.c.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if loc := resp.Header.Get("Location"); resp.StatusCode != 302 || !strings.HasPrefix(loc, "/login?next=%2Foauth%2Fauthorize%3F") {
		t.Fatalf("expected login redirect, got %d %q", resp.StatusCode, loc)
	}

	// Untrusted redirect_uri is never redirected to.
	bad := strings.Replace(f.authorizeQuery(ch, ""), "127.0.0.1%3A34567", "evil.example", 1)
	bad = strings.Replace(bad, "http%3A%2F%2Fevil", "https%3A%2F%2Fevil", 1)
	req, _ = http.NewRequest("GET", e.srv.URL+"/oauth/authorize?"+bad, nil)
	resp, err = anon.c.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 400 || resp.Header.Get("Location") != "" {
		t.Fatalf("bad redirect_uri: %d", resp.StatusCode)
	}

	// Missing PKCE / wrong resource come back to the client as errors with iss.
	loc := f.authorizeRedirect(strings.Replace(f.authorizeQuery(ch, ""), "&code_challenge_method=S256", "", 1))
	if loc.Query().Get("error") != "invalid_request" || loc.Query().Get("iss") != e.srv.URL {
		t.Fatalf("missing method: %s", loc)
	}
	loc = f.authorizeRedirect(strings.Replace(f.authorizeQuery(ch, ""), url.QueryEscape(e.srv.URL+"/mcp"), "https%3A%2F%2Fother.example%2Fmcp", 1))
	if loc.Query().Get("error") != "invalid_target" {
		t.Fatalf("resource mismatch: %s", loc)
	}

	// Deny.
	_, ch2 := pkcePair()
	loc = f.approve(f.authorizeQuery(ch2, ""), "deny")
	if loc.Query().Get("error") != "access_denied" || loc.Query().Get("state") != "st4te" {
		t.Fatalf("deny: %s", loc)
	}

	// Consent POST without the CSRF token is refused.
	form, _ := url.ParseQuery(f.authorizeQuery(ch2, ""))
	form.Set("action", "approve")
	req, _ = http.NewRequest("POST", e.srv.URL+"/oauth/authorize", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: "canvasbin_session", Value: user.ck})
	resp, err = user.c.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 403 {
		t.Fatalf("missing csrf: %d", resp.StatusCode)
	}

	// Code exchange: wrong verifier burns the code; right verifier works once.
	code, verifier := f.getCode()
	if st, out := f.token(f.tokenForm(code, verifier+"x")); st != 400 || out["error"] != "invalid_grant" {
		t.Fatalf("wrong verifier: %d %v", st, out)
	}
	if st, out := f.token(f.tokenForm(code, verifier)); st != 400 || out["error"] != "invalid_grant" {
		t.Fatalf("burned code reused: %d %v", st, out)
	}
	code, verifier = f.getCode()
	other := f.tokenForm(code, verifier)
	other.Set("redirect_uri", "http://127.0.0.1:34567/other")
	if st, out := f.token(other); st != 400 || out["error"] != "invalid_grant" {
		t.Fatalf("redirect mismatch: %d %v", st, out)
	}
	code, verifier = f.getCode()
	st, tok := f.token(f.tokenForm(code, verifier))
	if st != 200 || tok["token_type"] != "Bearer" || tok["expires_in"] != float64(3600) {
		t.Fatalf("token: %d %v", st, tok)
	}
	access, refresh := tok["access_token"].(string), tok["refresh_token"].(string)
	if access == "" || refresh == "" {
		t.Fatal("missing tokens")
	}
	if st, _ := f.token(f.tokenForm(code, verifier)); st != 400 {
		t.Fatal("code must be single use")
	}
	// JSON bodies are refused.
	resp, _ = http.Post(e.srv.URL+"/oauth/token", "application/json", strings.NewReader(`{}`))
	resp.Body.Close()
	if resp.StatusCode != 400 {
		t.Fatalf("json token request: %d", resp.StatusCode)
	}

	// Tokens are stored hashed only.
	if _, err := e.st.GetOAuthTokenByAccess(context.Background(), []byte(access)); err == nil {
		t.Fatal("raw access token must not be a lookup key")
	}

	// MCP: every request needs a token, so clients sign in when they connect.
	initBody := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"t","version":"1"}}}`
	want := `Bearer resource_metadata="` + e.srv.URL + `/.well-known/oauth-protected-resource/mcp", scope="pages:write"`
	for _, body := range []string{initBody, `{"jsonrpc":"2.0","id":2,"method":"tools/list"}`, rpcCall("create_page", `{"content":"x"}`)} {
		if st, hdr, _ := f.mcp("", body); st != 401 || hdr.Get("WWW-Authenticate") != want {
			t.Fatalf("no token: %d %q", st, hdr.Get("WWW-Authenticate"))
		}
	}
	if st, hdr, _ := f.mcp("bogus", initBody); st != 401 || !strings.Contains(hdr.Get("WWW-Authenticate"), `error="invalid_token"`) {
		t.Fatalf("bad token: %d %q", st, hdr.Get("WWW-Authenticate"))
	}
	if st, _, body := f.mcp(access, initBody); st != 200 || !strings.Contains(body, "list_pages") || !strings.Contains(body, "get_style_guide") {
		t.Fatalf("initialize: %d %s", st, body)
	}
	if st, _, body := f.mcp(access, `{"jsonrpc":"2.0","id":2,"method":"tools/list"}`); st != 200 || !strings.Contains(body, "claim_page") || !strings.Contains(body, "call get_style_guide") {
		t.Fatalf("tools/list: %d %s", st, body)
	}
	// A pre-existing unowned page can still be edited by key once signed in.
	legacy, err := e.st.Create(context.Background(), "anon", "html", "<p>anon</p>", false)
	if err != nil {
		t.Fatal(err)
	}
	anonWriteKey := legacy.WriteKey
	if st, _, body := f.mcp(access, rpcCall("update_page", `{"key":"`+anonWriteKey+`","content":"<p>v2</p>"}`)); st != 200 || strings.Contains(body, `"isError":true`) {
		t.Fatalf("update of legacy page by key: %d %s", st, body)
	}

	// With the bearer token: owned create, list, claim, share.
	if st, _, body := f.mcp(access, rpcCall("create_page", `{"content":"# mine","format":"markdown","title":"mine"}`)); st != 200 || !strings.Contains(body, "read_key") {
		t.Fatalf("owned create: %d %s", st, body)
	}
	if st, _, body := f.mcp(access, rpcCall("claim_page", `{"key":"`+anonWriteKey+`"}`)); st != 200 || strings.Contains(body, `"isError":true`) {
		t.Fatalf("claim: %d %s", st, body)
	}
	st, _, body := f.mcp(access, rpcCall("list_pages", `{}`))
	if st != 200 || !strings.Contains(body, `\"owned\":2`) && !strings.Contains(body, `"owned":2`) {
		t.Fatalf("list_pages: %d %s", st, body)
	}
	st, _, body = f.mcp(access, rpcCall("share_page", `{"key":"`+anonWriteKey+`","email":"friend@example.com","role":"editor"}`))
	if st != 200 || !strings.Contains(body, "Team plan") {
		t.Fatalf("share without a team: %d %s", st, body)
	}
	e.activeTeam(t, email, "MCP team")
	st, _, body = f.mcp(access, rpcCall("share_page", `{"key":"`+anonWriteKey+`","email":"friend@example.com","role":"editor"}`))
	if st != 200 || strings.Contains(body, `"isError":true`) || !strings.Contains(body, "friend@example.com") {
		t.Fatalf("share: %d %s", st, body)
	}
	if e.mail.get("invite", "friend@example.com") == "" {
		t.Fatal("invite email not sent")
	}
	if st, _, body = f.mcp(access, rpcCall("unshare_page", `{"key":"`+anonWriteKey+`","email":"friend@example.com"}`)); st != 200 || strings.Contains(body, `"isError":true`) {
		t.Fatalf("unshare: %d %s", st, body)
	}

	// Team pages, comments and team listing over MCP.
	if st, _, body = f.mcp(access, rpcCall("list_teams", `{}`)); st != 200 || !strings.Contains(body, "MCP team") {
		t.Fatalf("list_teams: %d %s", st, body)
	}
	if st, _, body = f.mcp(access, rpcCall("get_style_guide", `{"team":"MCP team"}`)); st != 200 || !strings.Contains(body, "style_guide") {
		t.Fatalf("get_style_guide: %d %s", st, body)
	}
	if st, _, body = f.mcp(access, rpcCall("set_style_guide", `{"team":"mcp team","style_guide":"Use short sentences."}`)); st != 200 || !strings.Contains(body, "Use short sentences.") || (!strings.Contains(body, `"created":true`) && !strings.Contains(body, `\"created\":true`)) {
		t.Fatalf("create style guide: %d %s", st, body)
	}
	if st, _, body = f.mcp(access, rpcCall("set_style_guide", `{"team":"mcp team","style_guide":"Use short sentences. No emoji."}`)); st != 200 || !strings.Contains(body, "No emoji.") || (!strings.Contains(body, `"created":false`) && !strings.Contains(body, `\"created\":false`)) {
		t.Fatalf("update style guide: %d %s", st, body)
	}
	if st, _, body = f.mcp(access, rpcCall("get_style_guide", `{"team":"nope"}`)); !strings.Contains(body, "list_teams") {
		t.Fatalf("unknown team style guide: %d %s", st, body)
	}
	if st, _, body = f.mcp(access, rpcCall("create_page", `{"content":"t","title":"team doc","team":"mcp team"}`)); st != 200 || strings.Contains(body, `"isError":true`) {
		t.Fatalf("team create: %d %s", st, body)
	}
	m := regexp.MustCompile(`\\?"read_key\\?":\\?"([A-Za-z0-9_-]+)`).FindStringSubmatch(body)
	if m == nil {
		t.Fatalf("no read key in %s", body)
	}
	teamKey := m[1]
	if st, _, body = f.mcp(access, rpcCall("create_page", `{"content":"t","team":"nope"}`)); !strings.Contains(body, "list_teams") {
		t.Fatalf("unknown team: %d %s", st, body)
	}
	if st, _, body = f.mcp(access, rpcCall("add_comment", `{"key":"`+teamKey+`","body":"looks good"}`)); st != 200 || strings.Contains(body, `"isError":true`) {
		t.Fatalf("add_comment: %d %s", st, body)
	}
	if st, _, body = f.mcp(access, rpcCall("list_comments", `{"key":"`+teamKey+`"}`)); st != 200 || !strings.Contains(body, "looks good") {
		t.Fatalf("list_comments: %d %s", st, body)
	}
	if st, _, body = f.mcp(access, rpcCall("add_comment", `{"key":"`+anonWriteKey+`","body":"x"}`)); !strings.Contains(body, "team pages") {
		t.Fatalf("comment on personal page: %d %s", st, body)
	}
	if st, _, body = f.mcp(access, rpcCall("list_pages", `{}`)); !strings.Contains(body, "MCP team") {
		t.Fatalf("list_pages team: %d %s", st, body)
	}
	// A garbage bearer is refused even for public tools.
	if st, _, _ := f.mcp("cba_nope", rpcCall("get_page", `{"key":"`+anonWriteKey+`"}`)); st != 401 {
		t.Fatalf("bad bearer on public tool: %d", st)
	}

	// Refresh rotates; the old refresh token dies.
	rform := url.Values{"grant_type": {"refresh_token"}, "refresh_token": {refresh}, "client_id": {f.clientID}}
	st, tok2 := f.token(rform)
	if st != 200 || tok2["refresh_token"] == refresh || tok2["access_token"] == access {
		t.Fatalf("refresh: %d %v", st, tok2)
	}
	if st, out := f.token(rform); st != 400 || out["error"] != "invalid_grant" {
		t.Fatalf("old refresh reuse: %d %v", st, out)
	}
	if st, _, _ := f.mcp(access, rpcCall("list_pages", `{}`)); st != 401 {
		t.Fatalf("rotated-away access token should be revoked, got %d", st)
	}
	access2 := tok2["access_token"].(string)
	wrongClient := url.Values{"grant_type": {"refresh_token"}, "refresh_token": {tok2["refresh_token"].(string)}, "client_id": {"someone-else"}}
	if st, out := f.token(wrongClient); st != 400 || out["error"] != "invalid_grant" {
		t.Fatalf("refresh with other client: %d %v", st, out)
	}

	// Revocation.
	if st, _, _ := f.mcp(access2, rpcCall("list_pages", `{}`)); st != 200 {
		t.Fatalf("new access token: %d", st)
	}
	resp, _ = http.Post(e.srv.URL+"/oauth/revoke", "application/x-www-form-urlencoded", strings.NewReader("token="+access2))
	resp.Body.Close()
	if st, _, _ := f.mcp(access2, rpcCall("list_pages", `{}`)); st != 401 {
		t.Fatalf("revoked token: %d", st)
	}

	// Disconnecting from the account page signs the app out.
	code, verifier = f.getCode()
	st, tok3 := f.token(f.tokenForm(code, verifier))
	if st != 200 {
		t.Fatalf("token for disconnect: %d %v", st, tok3)
	}
	access3 := tok3["access_token"].(string)
	conns := user.want(200, "GET", "/api/connections", nil)
	apps, _ := conns["apps"].([]any)
	if len(apps) != 1 || apps[0].(map[string]any)["client_name"] != "Test Client" {
		t.Fatalf("connections: %v", conns)
	}
	anon.want(401, "GET", "/api/connections", nil)
	stranger := e.newClient(t)
	stranger.signup(uniq("oauth-stranger"))
	stranger.want(404, "POST", "/api/connections/revoke", map[string]string{"client_id": f.clientID})
	if st, _, _ := f.mcp(access3, rpcCall("list_pages", `{}`)); st != 200 {
		t.Fatalf("token before disconnect: %d", st)
	}
	user.want(204, "POST", "/api/connections/revoke", map[string]string{"client_id": f.clientID})
	if st, _, _ := f.mcp(access3, rpcCall("list_pages", `{}`)); st != 401 {
		t.Fatalf("token after disconnect: %d", st)
	}
	if st, out := f.token(url.Values{"grant_type": {"refresh_token"}, "refresh_token": {tok3["refresh_token"].(string)}, "client_id": {f.clientID}}); st != 400 || out["error"] != "invalid_grant" {
		t.Fatalf("refresh after disconnect: %d %v", st, out)
	}
	if out := user.want(200, "GET", "/api/connections", nil); len(out["apps"].([]any)) != 0 {
		t.Fatalf("connections after disconnect: %v", out)
	}

	// Metadata-document client ids that point inside the network are refused
	// without being fetched, and never redirect.
	for _, id := range []string{"https://127.0.0.1:9/client.json", "https://169.254.169.254/latest/meta-data"} {
		q := strings.Replace(f.authorizeQuery(ch, ""), url.QueryEscape(f.clientID), url.QueryEscape(id), 1)
		req, _ = http.NewRequest("GET", e.srv.URL+"/oauth/authorize?"+q, nil)
		resp, err = anon.c.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != 400 {
			t.Fatalf("cimd %s: %d", id, resp.StatusCode)
		}
	}
}
