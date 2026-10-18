package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"testing/fstest"
	"time"

	"github.com/stewartmckee/canvasbin/internal/auth"
	"github.com/stewartmckee/canvasbin/internal/config"
	"github.com/stewartmckee/canvasbin/internal/live"
	"github.com/stewartmckee/canvasbin/internal/store"
)

type fakeMailer struct {
	mu    sync.Mutex
	links map[string]string // "kind:email" -> link
}

func (f *fakeMailer) put(kind, email, link string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.links[kind+":"+email] = link
	return nil
}
func (f *fakeMailer) get(kind, email string) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.links[kind+":"+email]
}
func (f *fakeMailer) SendVerify(_ context.Context, e, l string) error { return f.put("verify", e, l) }
func (f *fakeMailer) SendReset(_ context.Context, e, l string) error  { return f.put("reset", e, l) }
func (f *fakeMailer) SendInvite(_ context.Context, e, l, _ string) error {
	return f.put("invite", e, l)
}

func (f *fakeMailer) SendTeamInvite(_ context.Context, e, l, _, _ string) error {
	return f.put("team", e, l)
}

func (f *fakeMailer) SendWelcome(_ context.Context, e, base string) error {
	return f.put("welcome", e, base)
}

type env struct {
	srv  *httptest.Server
	mail *fakeMailer
	st   *store.Store
}

func newEnv(t *testing.T, accounts bool, quota int) *env {
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
	cfg := config.Config{Accounts: accounts, FreeQuota: quota, ProQuota: 1000}
	s := &Server{
		Store: st, Config: cfg, Mailer: m,
		Web: fstest.MapFS{
			"index.html":             {Data: []byte("<html></html>")},
			"prerender/pricing.html": {Data: []byte("<html>prerendered pricing</html>")},
		},
		Live: live.New(100),
	}
	ctx, stop := context.WithCancel(context.Background())
	t.Cleanup(stop)
	go s.RunLive(ctx)
	ts := httptest.NewServer(s.Handler())
	t.Cleanup(ts.Close)
	return &env{srv: ts, mail: m, st: st}
}

type client struct {
	t  *testing.T
	e  *env
	c  *http.Client
	ck string
}

func (e *env) newClient(t *testing.T) *client {
	return &client{t: t, e: e, c: &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
}

func (c *client) do(method, path string, body any) (int, map[string]any) {
	c.t.Helper()
	var rd *bytes.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	} else {
		rd = bytes.NewReader(nil)
	}
	req, _ := http.NewRequest(method, c.e.srv.URL+path, rd)
	if c.ck != "" {
		req.AddCookie(&http.Cookie{Name: "canvasbin_session", Value: c.ck})
	}
	resp, err := c.c.Do(req)
	if err != nil {
		c.t.Fatal(err)
	}
	defer resp.Body.Close()
	for _, ck := range resp.Cookies() {
		if ck.Name == "canvasbin_session" {
			c.ck = ck.Value
			if ck.MaxAge < 0 {
				c.ck = ""
			}
		}
	}
	var out map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&out)
	return resp.StatusCode, out
}

func (c *client) want(code int, method, path string, body any) map[string]any {
	c.t.Helper()
	got, out := c.do(method, path, body)
	if got != code {
		c.t.Fatalf("%s %s: status %d, want %d (%v)", method, path, got, code, out)
	}
	return out
}

func (c *client) signup(email string) {
	c.t.Helper()
	c.want(201, "POST", "/api/register", map[string]string{"email": email, "password": "password123", "display_name": "T"})
	link := c.e.mail.get("verify", email)
	if link == "" {
		c.t.Fatal("no verification email")
	}
	u, _ := url.Parse(link)
	req, _ := http.NewRequest("GET", c.e.srv.URL+"/verify?"+u.RawQuery, nil)
	resp, err := c.c.Do(req)
	if err != nil {
		c.t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusSeeOther || resp.Header.Get("Location") != "/" {
		c.t.Fatalf("verify: %d %s", resp.StatusCode, resp.Header.Get("Location"))
	}
	for _, ck := range resp.Cookies() {
		if ck.Name == "canvasbin_session" {
			if !ck.HttpOnly || ck.SameSite != http.SameSiteLaxMode {
				c.t.Fatalf("weak cookie %+v", ck)
			}
			c.ck = ck.Value
		}
	}
	if c.ck == "" {
		c.t.Fatal("verify did not sign in")
	}
	if c.e.mail.get("welcome", email) != c.e.srv.URL {
		c.t.Fatal("no welcome email")
	}
}

func TestWelcomeSentOnce(t *testing.T) {
	e := newEnv(t, true, 5)
	c := e.newClient(t)
	email := uniq("welcome")
	c.signup(email)
	_ = e.mail.put("welcome", email, "")

	u, err := e.st.GetUserByEmail(context.Background(), email)
	if err != nil {
		t.Fatal(err)
	}
	if err := e.st.CreateEmailToken(context.Background(), u.ID, store.PurposeVerify, auth.HashToken(email), time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	resp, err := c.c.Get(e.srv.URL + "/verify?token=" + url.QueryEscape(email))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if got := e.mail.get("welcome", email); got != "" {
		t.Fatalf("welcome sent again: %q", got)
	}
}

func uniq(p string) string { return fmt.Sprintf("%s-%d@example.com", p, time.Now().UnixNano()) }

// activeTeam creates a team owned by email with a paid-up subscription, as the
// Stripe webhook would leave it.
func (e *env) activeTeam(t *testing.T, email, name string) int64 {
	t.Helper()
	ctx := context.Background()
	u, err := e.st.GetUserByEmail(ctx, email)
	if err != nil {
		t.Fatal(err)
	}
	tm, err := e.st.CreateTeam(ctx, u.ID, name)
	if err != nil {
		t.Fatal(err)
	}
	if err := e.st.UpdateTeamSubscription(ctx, tm.ID, store.Subscription{
		SubscriptionID: fmt.Sprintf("sub_test_%d", time.Now().UnixNano()), Status: "active",
	}); err != nil {
		t.Fatal(err)
	}
	return tm.ID
}

func TestAccountsLifecycle(t *testing.T) {
	e := newEnv(t, true, 2)
	c := e.newClient(t)
	email := uniq("life")

	// Registering signs in unverified; creating pages waits for verification.
	reg := c.want(201, "POST", "/api/register", map[string]string{"email": email, "password": "password123"})
	if u, _ := reg["user"].(map[string]any); u == nil || u["email_verified_at"] != nil {
		t.Fatalf("register should sign in unverified: %v", reg)
	}
	c.want(409, "POST", "/api/register", map[string]string{"email": email, "password": "password123"})
	c.want(400, "POST", "/api/register", map[string]string{"email": uniq("weak"), "password": "short"})
	c.want(200, "GET", "/api/me", nil)
	if out := c.want(403, "POST", "/api/pages", map[string]string{"content": "x"}); out["code"] != "unverified" {
		t.Fatalf("want unverified code: %v", out)
	}
	c.want(204, "POST", "/api/logout", nil)
	c.want(200, "POST", "/api/login", map[string]string{"email": email, "password": "password123"})
	c.want(204, "POST", "/api/logout", nil)
	c.want(401, "GET", "/api/me", nil)
	c2 := e.newClient(t)
	c2.signup(uniq("other")) // exercises /verify
	me := c2.want(200, "GET", "/api/me", nil)
	if me["plan"] != "free" || me["quota"].(float64) != 2 {
		t.Fatalf("me: %v", me)
	}

	// Login / logout.
	c.signup(uniq("login"))
	c.want(204, "POST", "/api/logout", nil)
	c.want(401, "GET", "/api/me", nil)
	c.want(401, "POST", "/api/login", map[string]string{"email": email, "password": "wrong-password"})

	// Password reset (also proves the mailbox, so it verifies the account).
	c.want(200, "POST", "/api/forgot-password", map[string]string{"email": email})
	c.want(200, "POST", "/api/forgot-password", map[string]string{"email": "ghost@example.com"})
	ru, _ := url.Parse(e.mail.get("reset", email))
	token := ru.Query().Get("token")
	if token == "" || ru.Path != "/reset-password" {
		t.Fatalf("bad reset link %v", ru)
	}
	c.want(400, "POST", "/api/reset-password", map[string]string{"token": "nope", "password": "newpassword1"})
	c.want(200, "POST", "/api/reset-password", map[string]string{"token": token, "password": "newpassword1"})
	c.want(400, "POST", "/api/reset-password", map[string]string{"token": token, "password": "newpassword2"})
	c.want(401, "POST", "/api/login", map[string]string{"email": email, "password": "password123"})
	c.want(200, "POST", "/api/login", map[string]string{"email": email, "password": "newpassword1"})
}

func TestOwnedPagesQuotaAndSharing(t *testing.T) {
	e := newEnv(t, true, 2)
	owner, friend, anon := e.newClient(t), e.newClient(t), e.newClient(t)
	ownerEmail, friendEmail := uniq("own"), uniq("friend")
	owner.signup(ownerEmail)
	friend.signup(friendEmail)

	p := owner.want(201, "POST", "/api/pages", map[string]string{"title": "A", "content": "<p>a</p>"})
	if p["owned"] != true || p["write_key"] == "" {
		t.Fatalf("created page: %v", p)
	}
	readKey, writeKey := p["read_key"].(string), p["write_key"].(string)
	owner.want(201, "POST", "/api/pages", map[string]string{"content": "b"})
	out := owner.want(403, "POST", "/api/pages", map[string]string{"content": "c"})
	if out["code"] != "quota_exceeded" || !strings.Contains(fmt.Sprint(out["error"]), "pages on the Free plan are in use") {
		t.Fatalf("want quota_exceeded: %v", out)
	}

	// Owner writes via read key + session; anonymous read key stays read-only.
	got := owner.want(200, "GET", "/api/pages/"+readKey, nil)
	if got["can_write"] != true || got["role"] != "owner" {
		t.Fatalf("owner get: %v", got)
	}
	if got := anon.want(200, "GET", "/api/pages/"+readKey, nil); got["can_write"] != false {
		t.Fatalf("anon get: %v", got)
	}
	anon.want(403, "PUT", "/api/pages/"+readKey, map[string]string{"content": "x"})
	owner.want(200, "PUT", "/api/pages/"+readKey, map[string]string{"content": "<p>v2</p>"})
	// Anonymous write-key access is unchanged.
	anon.want(200, "PUT", "/api/pages/"+writeKey, map[string]string{"title": "A2"})

	// Sharing is owner-only; invites to unknown emails become shares on verification.
	friend.want(403, "GET", "/api/pages/"+readKey+"/shares", nil)
	anon.want(401, "GET", "/api/pages/"+readKey+"/shares", nil)
	owner.want(400, "POST", "/api/pages/"+readKey+"/shares", map[string]string{"email": friendEmail, "role": "admin"})
	// Email sharing is a Team plan feature.
	if out := owner.want(402, "POST", "/api/pages/"+readKey+"/shares", map[string]string{"email": friendEmail, "role": "viewer"}); out["code"] != "team_required" {
		t.Fatalf("want team_required: %v", out)
	}
	e.activeTeam(t, ownerEmail, "Sharers")
	owner.want(200, "POST", "/api/pages/"+readKey+"/shares", map[string]string{"email": friendEmail, "role": "viewer"})
	friend.want(403, "PUT", "/api/pages/"+readKey, map[string]string{"content": "nope"})
	lst := owner.want(200, "POST", "/api/pages/"+readKey+"/shares", map[string]string{"email": friendEmail, "role": "editor"})
	if len(lst["shares"].([]any)) != 1 {
		t.Fatalf("shares: %v", lst)
	}
	friend.want(200, "PUT", "/api/pages/"+readKey, map[string]string{"content": "<p>friend</p>"})
	friend.want(403, "DELETE", "/api/pages/"+readKey, nil)
	lib := friend.want(200, "GET", "/api/library", nil)
	pages := lib["pages"].([]any)
	if len(pages) != 1 || pages[0].(map[string]any)["role"] != "editor" {
		t.Fatalf("friend library: %v", lib)
	}
	if !strings.HasSuffix(pages[0].(map[string]any)["html_url"].(string), readKey+".html") {
		t.Fatalf("library links: %v", pages[0])
	}
	owner.want(204, "DELETE", "/api/pages/"+readKey+"/shares/"+url.PathEscape(friendEmail), nil)
	owner.want(404, "DELETE", "/api/pages/"+readKey+"/shares/"+url.PathEscape(friendEmail), nil)
	friend.want(403, "PUT", "/api/pages/"+readKey, map[string]string{"content": "nope"})

	// Invite to a not-yet-registered email.
	late := uniq("late")
	owner.want(200, "POST", "/api/pages/"+readKey+"/shares", map[string]string{"email": late, "role": "viewer"})
	if e.mail.get("invite", late) == "" {
		t.Fatal("no invite email")
	}
	lateC := e.newClient(t)
	lateC.signup(late)
	if pages := lateC.want(200, "GET", "/api/library", nil)["pages"].([]any); len(pages) != 1 {
		t.Fatalf("invited user library: %v", pages)
	}

	// Signed-out visitors can't create pages once accounts are on.
	anon.want(401, "POST", "/api/pages", map[string]string{"content": "anon"})

	// Claim a pre-accounts anonymous page with its write key.
	legacy, err := e.st.Create(context.Background(), "", "html", "anon")
	if err != nil {
		t.Fatal(err)
	}
	pg := map[string]any{"read_key": legacy.ReadKey, "write_key": legacy.WriteKey}
	friend.want(404, "POST", "/api/pages/claim", map[string]string{"key": pg["read_key"].(string)})
	friend.want(200, "POST", "/api/pages/claim", map[string]string{"key": pg["write_key"].(string)})
	friend.want(200, "POST", "/api/pages/claim", map[string]string{"key": pg["write_key"].(string)}) // idempotent
	lateC.want(409, "POST", "/api/pages/claim", map[string]string{"key": pg["write_key"].(string)})
	anon.want(401, "POST", "/api/pages/claim", map[string]string{"key": pg["write_key"].(string)})

	// Owner can delete.
	owner.want(204, "DELETE", "/api/pages/"+readKey, nil)
	anon.want(404, "GET", "/api/pages/"+readKey, nil)
}

func TestAdminDisable(t *testing.T) {
	e := newEnv(t, true, 20)
	adminEmail := uniq("admin")
	e.srvConfig(func(c *config.Config) { c.AdminEmails = []string{adminEmail} })
	admin, victim := e.newClient(t), e.newClient(t)
	victimEmail := uniq("victim")
	admin.signup(adminEmail)
	victim.signup(victimEmail)

	victim.want(403, "GET", "/api/admin/users", nil)
	users := admin.want(200, "GET", "/api/admin/users", nil)["users"].([]any)
	var vid, aid float64
	for _, u := range users {
		if m := u.(map[string]any); m["email"] == victimEmail {
			vid = m["id"].(float64)
		} else if m["email"] == adminEmail {
			aid = m["id"].(float64)
		}
		if _, leaked := u.(map[string]any)["password_hash"]; leaked {
			t.Fatal("password hash leaked")
		}
	}
	if vid == 0 {
		t.Fatal("victim not listed")
	}
	admin.want(409, "POST", "/api/admin/users/"+fmt.Sprint(int64(aid))+"/disable", nil)
	admin.want(200, "POST", fmt.Sprintf("/api/admin/users/%d/disable", int64(vid)), nil)
	victim.want(401, "GET", "/api/me", nil)
	victim.want(403, "POST", "/api/login", map[string]string{"email": victimEmail, "password": "password123"})
	admin.want(200, "POST", fmt.Sprintf("/api/admin/users/%d/disable", int64(vid)), map[string]bool{"disabled": false})
	victim.want(200, "POST", "/api/login", map[string]string{"email": victimEmail, "password": "password123"})
}

// srvConfig rebuilds the test server with an adjusted Config.
func (e *env) srvConfig(f func(*config.Config)) {
	e.srv.Close()
	cfg := config.Config{Accounts: true, FreeQuota: 20, ProQuota: 1000}
	f(&cfg)
	s := &Server{Store: e.st, Config: cfg, Mailer: e.mail, Web: fstest.MapFS{"index.html": {Data: []byte("<html></html>")}}}
	e.srv = httptest.NewServer(s.Handler())
}

// TestLegacyLinksWithAccountsOn checks that capability links handed out before
// accounts existed (anonymous pages, old /v/ and /e/ URLs) keep working when
// ACCOUNTS is on, whether or not the visitor is signed in.
func TestLegacyLinksWithAccountsOn(t *testing.T) {
	// Create the page with accounts off, as the old deployment did.
	off := newEnv(t, false, 20)
	p := off.newClient(t).want(201, "POST", "/api/pages",
		map[string]string{"title": "legacy", "format": "markdown", "content": "# old"})
	rk, wk := p["read_key"].(string), p["write_key"].(string)

	e := newEnv(t, true, 20)
	anon, stranger, holder := e.newClient(t), e.newClient(t), e.newClient(t)
	stranger.signup(uniq("stranger"))
	holder.signup(uniq("holder"))

	status := func(c *client, path string) (int, string) {
		t.Helper()
		req, _ := http.NewRequest("GET", e.srv.URL+path, nil)
		if c.ck != "" {
			req.AddCookie(&http.Cookie{Name: "canvasbin_session", Value: c.ck})
		}
		resp, err := c.c.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		return resp.StatusCode, resp.Header.Get("Location")
	}

	for _, c := range []*client{anon, stranger} {
		for _, path := range []string{"/" + rk, "/" + rk + ".html", "/" + rk + ".md", "/raw/" + rk, "/" + wk, "/" + wk + ".html"} {
			if code, _ := status(c, path); code != 200 {
				t.Fatalf("GET %s: %d", path, code)
			}
		}
		if code, loc := status(c, "/v/"+rk); code != 301 || loc != "/"+rk+".html" {
			t.Fatalf("/v/ redirect: %d %s", code, loc)
		}
		if code, loc := status(c, "/e/"+wk); code != 301 || loc != "/"+wk {
			t.Fatalf("/e/ redirect: %d %s", code, loc)
		}

		// Read key stays read-only; write key still grants edit to anyone holding it.
		if g := c.want(200, "GET", "/api/pages/"+rk, nil); g["can_write"] != false || g["write_key"] != nil {
			t.Fatalf("read key leaked write access: %v", g)
		}
		c.want(403, "PUT", "/api/pages/"+rk, map[string]string{"content": "nope"})
		if g := c.want(200, "GET", "/api/pages/"+wk, nil); g["can_write"] != true || g["write_key"] != wk {
			t.Fatalf("write key lost access: %v", g)
		}
		c.want(200, "PUT", "/api/pages/"+wk, map[string]string{"content": "# edited"})
	}

	// A signed-in write-key holder can still delete an unowned legacy page.
	holder.want(204, "DELETE", "/api/pages/"+wk, nil)
	anon.want(404, "GET", "/api/pages/"+rk, nil)
}

func TestMarketingPagesIndexable(t *testing.T) {
	get := func(srv *httptest.Server, path string) *http.Response {
		t.Helper()
		resp, err := http.Get(srv.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		return resp
	}
	on := newEnv(t, true, 20)
	for _, p := range []string{"/", "/features", "/pricing", "/privacy", "/terms", "/contact", "/use-cases", "/use-cases/family", "/use-cases/news-apps-games"} {
		if r := get(on.srv, p); r.StatusCode != 200 || r.Header.Get("X-Robots-Tag") != "" {
			t.Errorf("%s: status %d robots %q", p, r.StatusCode, r.Header.Get("X-Robots-Tag"))
		}
	}
	for _, p := range []string{"/login", "/account", "/" + strings.Repeat("a", 43), "/api/config"} {
		if r := get(on.srv, p); r.Header.Get("X-Robots-Tag") == "" {
			t.Errorf("%s should be noindex", p)
		}
	}
	if r := get(on.srv, "/robots.txt"); r.StatusCode != 200 {
		t.Errorf("robots.txt: %d", r.StatusCode)
	}
	if cfg := on.newClient(t).want(200, "GET", "/api/config", nil); cfg["free_quota"] != float64(20) || cfg["pro_quota"] != float64(1000) {
		t.Errorf("config quotas: %v", cfg)
	}

	off := newEnv(t, false, 20)
	if r := get(off.srv, "/"); r.Header.Get("X-Robots-Tag") == "" {
		t.Error("accounts off: home should stay noindex")
	}
}

func TestPrerenderedPagesAndNotFound(t *testing.T) {
	e := newEnv(t, true, 20)
	body := func(c *client, path string) (int, string) {
		t.Helper()
		req, _ := http.NewRequest("GET", e.srv.URL+path, nil)
		if c.ck != "" {
			req.AddCookie(&http.Cookie{Name: "canvasbin_session", Value: c.ck})
		}
		resp, err := c.c.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, string(b)
	}
	anon, user := e.newClient(t), e.newClient(t)
	user.signup(uniq("prerender"))

	if code, b := body(anon, "/pricing"); code != 200 || b != "<html>prerendered pricing</html>" {
		t.Errorf("signed out /pricing: %d %q", code, b)
	}
	if code, b := body(user, "/pricing"); code != 200 || strings.Contains(b, "prerendered") {
		t.Errorf("signed in /pricing should get the app shell: %d %q", code, b)
	}
	if code, b := body(anon, "/features"); code != 200 || b != "<html></html>" {
		t.Errorf("no prerender falls back to the shell: %d %q", code, b)
	}
	for _, p := range []string{"/login", "/teams/12", "/teams/new", "/use-cases/developers", "/connect/claude"} {
		if code, _ := body(anon, p); code != 200 {
			t.Errorf("%s: %d", p, code)
		}
	}
	for from, to := range map[string]string{
		"/features/": "/features", "/use-cases/family/": "/use-cases/family", "/index.html": "/",
		"/pricing/?utm_source=x": "/pricing?utm_source=x",
	} {
		resp, err := anon.c.Get(e.srv.URL + from)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusMovedPermanently || resp.Header.Get("Location") != to {
			t.Errorf("%s: %d -> %q, want 301 -> %q", from, resp.StatusCode, resp.Header.Get("Location"), to)
		}
	}
	for _, p := range []string{"/nope", "/teams/abc", "/use-cases/cats", "/prerender/pricing.html", "/features/extra", "/evil.example/"} {
		if code, b := body(anon, p); code != 404 || b != "<html></html>" {
			t.Errorf("%s: %d %q", p, code, b)
		}
	}
}

func TestBadgeOnFreePages(t *testing.T) {
	e := newEnv(t, true, 20)
	email := uniq("badge")
	c := e.newClient(t)
	c.signup(email)
	p := c.want(201, "POST", "/api/pages", map[string]string{"title": "b", "format": "html", "content": "<p>hi</p>"})
	html := func() string {
		t.Helper()
		resp, err := http.Get(e.srv.URL + "/" + p["read_key"].(string) + ".html")
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		return string(b)
	}
	if !strings.Contains(html(), "Made with Canvasbin") {
		t.Fatal("free page should carry the badge")
	}
	u, err := e.st.GetUserByEmail(context.Background(), email)
	if err != nil {
		t.Fatal(err)
	}
	if err := e.st.UpdateSubscription(context.Background(), u.ID, store.Subscription{
		SubscriptionID: fmt.Sprintf("sub_test_%d", time.Now().UnixNano()), Status: "active",
	}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(html(), "Made with Canvasbin") {
		t.Fatal("paid page should not carry the badge")
	}
}

func TestAccountsOffKeepsAnonymousAPI(t *testing.T) {
	e := newEnv(t, false, 20)
	c := e.newClient(t)
	// Account routes don't exist.
	for _, p := range []string{"/api/me", "/api/library", "/api/admin/users"} {
		c.want(404, "GET", p, nil)
	}
	c.want(404, "POST", "/api/register", map[string]string{"email": "a@b.com", "password": "password123"})
	if cfg := c.want(200, "GET", "/api/config", nil); cfg["accounts"] != false {
		t.Fatalf("config: %v", cfg)
	}

	p := c.want(201, "POST", "/api/pages", map[string]string{"title": "t", "content": "hi"})
	if p["owned"] != false {
		t.Fatalf("anon page owned: %v", p)
	}
	rk, wk := p["read_key"].(string), p["write_key"].(string)
	c.want(403, "PUT", "/api/pages/"+rk, map[string]string{"content": "x"})
	c.want(200, "PUT", "/api/pages/"+wk, map[string]string{"content": "v2"})
	if g := c.want(200, "GET", "/api/pages/"+rk, nil); g["content"] != "v2" {
		t.Fatalf("get: %v", g)
	}
	// A stray session cookie is ignored when accounts are off.
	c.ck = "whatever"
	c.want(204, "DELETE", "/api/pages/"+wk, nil)
}
