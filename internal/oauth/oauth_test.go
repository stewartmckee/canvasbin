package oauth

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"
	"time"
)

func TestVerifyPKCE(t *testing.T) {
	// RFC 7636 appendix B.
	const verifier = "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"
	const challenge = "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM"
	if !VerifyPKCE(verifier, challenge) {
		t.Fatal("RFC 7636 example should verify")
	}
	for name, c := range map[string][2]string{
		"wrong verifier":              {verifier + "x", challenge},
		"wrong challenge":             {verifier, "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cN"},
		"plain (verifier==challenge)": {verifier, verifier},
		"short verifier":              {"short", challenge},
		"long verifier":               {strings.Repeat("a", 129), challenge},
		"bad verifier chars":          {strings.Repeat("a", 42) + "!", challenge},
		"empty":                       {"", ""},
		"padded challenge":            {verifier, challenge + "="},
	} {
		if VerifyPKCE(c[0], c[1]) {
			t.Errorf("%s: should not verify", name)
		}
	}
}

func TestValidateRedirectURI(t *testing.T) {
	good := []string{
		"https://example.com/cb",
		"https://example.com:8443/cb?x=1",
		"http://localhost/cb",
		"http://localhost:5173/cb",
		"http://127.0.0.1:33418/callback",
		"http://[::1]:9000/cb",
	}
	bad := []string{
		"", "http://example.com/cb", "http://localhost.evil.com/cb", "http://127.0.0.2/cb",
		"https://example.com/cb#frag", "https://user:pw@example.com/cb", "cursor://x/cb",
		"javascript:alert(1)", "/relative", "https:///nohost", "ftp://example.com/cb",
		"https://example.com/" + strings.Repeat("a", 2100),
	}
	for _, u := range good {
		if err := ValidateRedirectURI(u); err != nil {
			t.Errorf("%q should be valid: %v", u, err)
		}
	}
	for _, u := range bad {
		if err := ValidateRedirectURI(u); err == nil {
			t.Errorf("%q should be rejected", u)
		}
	}
}

func TestRedirectAllowed(t *testing.T) {
	reg := []string{"https://app.example/cb", "http://127.0.0.1:1111/cb"}
	for u, want := range map[string]bool{
		"https://app.example/cb":       true,
		"https://app.example/cb2":      false,
		"https://app.example:444/cb":   false,
		"http://127.0.0.1:55555/cb":    true, // loopback: any port
		"http://127.0.0.1:55555/other": false,
		"http://localhost:55555/cb":    false, // different loopback host
		"http://app.example/cb":        false,
		"https://evil.example/cb":      false,
		"https://app.example/cb#x":     false,
	} {
		if got := RedirectAllowed(reg, u); got != want {
			t.Errorf("RedirectAllowed(%q) = %v, want %v", u, got, want)
		}
	}
}

func TestBlockedIP(t *testing.T) {
	blocked := []string{
		"127.0.0.1", "::1", "10.0.0.1", "172.16.5.4", "192.168.1.1", "169.254.169.254", "fe80::1",
		"0.0.0.0", "::", "100.64.0.1", "224.0.0.1", "255.255.255.255", "198.18.0.1", "fc00::1", "fd12::1",
		"::ffff:127.0.0.1", "::ffff:10.1.2.3", "64:ff9b::7f00:1", "2002:7f00:1::1", "192.0.2.1",
	}
	for _, s := range blocked {
		if !blockedIP(netip.MustParseAddr(s)) {
			t.Errorf("%s should be blocked", s)
		}
	}
	for _, s := range []string{"8.8.8.8", "1.1.1.1", "93.184.216.34", "2606:4700:4700::1111"} {
		if blockedIP(netip.MustParseAddr(s)) {
			t.Errorf("%s should be allowed", s)
		}
	}
}

func TestValidateClientIDURL(t *testing.T) {
	good := []string{"https://example.com/oauth/client.json", "https://app.example.org/.well-known/x"}
	bad := []string{
		"http://example.com/c.json",       // not https
		"https://example.com",             // no path
		"https://example.com/",            // root path
		"https://example.com/c.json#frag", // fragment
		"https://u:p@example.com/c.json",  // userinfo
		"https://example.com/a/../c.json", // dot segments
		"https://localhost/c.json",
		"https://foo.localhost/c.json",
		"https://127.0.0.1/c.json",
		"https://[::1]/c.json",
		"https://10.0.0.5/c.json",
		"https://169.254.169.254/latest/meta-data",
		"https://[::ffff:192.168.0.1]/c.json",
		"file:///etc/passwd",
	}
	for _, id := range good {
		if _, err := ValidateClientIDURL(id); err != nil {
			t.Errorf("%q should be valid: %v", id, err)
		}
	}
	for _, id := range bad {
		if _, err := ValidateClientIDURL(id); err == nil {
			t.Errorf("%q should be rejected", id)
		}
	}
}

// The dialer is the real SSRF defence: it checks the address actually being
// connected to, so DNS names pointing at internal hosts are refused too.
func TestSafeDialerBlocksInternalAddresses(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	_, port, _ := net.SplitHostPort(ln.Addr().String())

	d := safeDialer()
	for _, addr := range []string{"127.0.0.1:" + port, "localhost:" + port} {
		conn, err := d.DialContext(context.Background(), "tcp", addr)
		if err == nil {
			conn.Close()
			t.Fatalf("dial %s should be blocked", addr)
		}
		if !errors.Is(err, errBlockedAddr) {
			t.Errorf("dial %s: want errBlockedAddr, got %v", addr, err)
		}
	}
}

func TestFetcherRefusesInternalClientIDs(t *testing.T) {
	var hit bool
	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hit = true
		_, _ = w.Write([]byte(`{}`))
	}))
	defer ts.Close()

	f := newCIMDFetcher()
	for _, id := range []string{ts.URL + "/client.json", "https://169.254.169.254/latest/meta-data", "https://localhost/c.json"} {
		if _, err := f.Get(context.Background(), id); err == nil {
			t.Errorf("%s should be refused", id)
		}
	}
	if hit {
		t.Fatal("internal server was contacted")
	}
}

func TestFetcherRedirectToInternalBlocked(t *testing.T) {
	// A fetch that redirects to http:// (e.g. an internal service) is refused by
	// CheckRedirect before any connection.
	c := newSafeHTTPClient()
	req, _ := http.NewRequest("GET", "http://example.com/", nil)
	if err := c.CheckRedirect(req, nil); err == nil {
		t.Fatal("non-https redirect should be refused")
	}
	req, _ = http.NewRequest("GET", "https://example.com/", nil)
	if err := c.CheckRedirect(req, make([]*http.Request, cimdMaxRedirect+1)); err == nil {
		t.Fatal("redirect chain should be capped")
	}
}

func TestParseClientDocument(t *testing.T) {
	const id = "https://app.example/client.json"
	ok := `{"client_id":"` + id + `","client_name":"My\u0007 App","redirect_uris":["https://app.example/cb","http://127.0.0.1/cb"],"token_endpoint_auth_method":"none"}`
	c, err := ParseClientDocument(id, []byte(ok))
	if err != nil {
		t.Fatal(err)
	}
	if c.Name != "My App" || len(c.RedirectURIs) != 2 {
		t.Fatalf("unexpected client %+v", c)
	}
	for name, doc := range map[string]string{
		"id mismatch":   `{"client_id":"https://other.example/c","redirect_uris":["https://app.example/cb"]}`,
		"no redirects":  `{"client_id":"` + id + `"}`,
		"bad redirect":  `{"client_id":"` + id + `","redirect_uris":["http://evil.example/cb"]}`,
		"secret client": `{"client_id":"` + id + `","redirect_uris":["https://app.example/cb"],"token_endpoint_auth_method":"client_secret_basic"}`,
		"has secret":    `{"client_id":"` + id + `","redirect_uris":["https://app.example/cb"],"client_secret":"x"}`,
		"not json":      `<html>`,
	} {
		if _, err := ParseClientDocument(id, []byte(doc)); err == nil {
			t.Errorf("%s: should be rejected", name)
		}
	}
}

func TestCIMDCache(t *testing.T) {
	f := newCIMDFetcher()
	now := time.Now()
	f.now = func() time.Time { return now }
	f.put(&Client{ID: "https://a.example/c"})
	if _, ok := f.cached("https://a.example/c"); !ok {
		t.Fatal("expected cache hit")
	}
	now = now.Add(cimdCacheTTL + time.Second)
	if _, ok := f.cached("https://a.example/c"); ok {
		t.Fatal("expected expiry")
	}
}

func TestNormalizeScopeAndHasScope(t *testing.T) {
	for in, want := range map[string]string{
		"":                             "pages:read pages:write",
		"pages:read":                   "pages:read",
		"offline_access pages:write":   "pages:write offline_access",
		"offline_access":               "pages:read pages:write offline_access",
		"bogus pages:read pages:read":  "pages:read",
		"pages:write pages:read admin": "pages:read pages:write",
	} {
		if got := NormalizeScope(in); got != want {
			t.Errorf("NormalizeScope(%q) = %q, want %q", in, got, want)
		}
	}
	if !HasScope("pages:write", ScopeRead) || HasScope("pages:read", ScopeWrite) {
		t.Fatal("write should imply read, read should not imply write")
	}
	if !scopeSubset("pages:read", "pages:read pages:write") || scopeSubset("pages:write", "pages:read") {
		t.Fatal("scopeSubset wrong")
	}
}

func TestBearerToken(t *testing.T) {
	for h, want := range map[string]string{"Bearer abc": "abc", "bearer abc": "abc", "BEARER  abc ": "abc", "Basic abc": "", "Bearer": "", "": ""} {
		r := httptest.NewRequest("POST", "/mcp", nil)
		if h != "" {
			r.Header.Set("Authorization", h)
		}
		got, ok := BearerToken(r)
		if got != want || ok != (want != "") {
			t.Errorf("BearerToken(%q) = %q,%v", h, got, ok)
		}
	}
}

func TestMetadata(t *testing.T) {
	p := New(Options{})
	mux := http.NewServeMux()
	p.Routes(mux)
	get := func(path string) string {
		rec := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "https://canvasbin.example"+path, nil)
		mux.ServeHTTP(rec, r)
		if rec.Code != 200 {
			t.Fatalf("%s: %d", path, rec.Code)
		}
		return rec.Body.String()
	}
	for _, path := range []string{"/.well-known/oauth-protected-resource", "/.well-known/oauth-protected-resource/mcp"} {
		b := get(path)
		for _, want := range []string{`"resource":"https://canvasbin.example/mcp"`, `"authorization_servers":["https://canvasbin.example"]`, `"scopes_supported":["pages:read","pages:write","offline_access"]`} {
			if !strings.Contains(b, want) {
				t.Errorf("%s missing %s in %s", path, want, b)
			}
		}
	}
	b := get("/.well-known/oauth-authorization-server")
	for _, want := range []string{
		`"issuer":"https://canvasbin.example"`, `"authorization_endpoint":"https://canvasbin.example/oauth/authorize"`,
		`"token_endpoint":"https://canvasbin.example/oauth/token"`, `"registration_endpoint":"https://canvasbin.example/oauth/register"`,
		`"code_challenge_methods_supported":["S256"]`, `"client_id_metadata_document_supported":true`,
		`"token_endpoint_auth_methods_supported":["none"]`, `"authorization_response_iss_parameter_supported":true`,
		`"grant_types_supported":["authorization_code","refresh_token"]`,
	} {
		if !strings.Contains(b, want) {
			t.Errorf("authorization-server metadata missing %s in %s", want, b)
		}
	}
}

func TestTokenEndpointRequiresForm(t *testing.T) {
	p := New(Options{})
	rec := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/oauth/token", strings.NewReader(`{"grant_type":"authorization_code"}`))
	r.Header.Set("Content-Type", "application/json")
	p.token(rec, r)
	if rec.Code != 400 || !strings.Contains(rec.Body.String(), "invalid_request") {
		t.Fatalf("got %d %s", rec.Code, rec.Body.String())
	}
	rec = httptest.NewRecorder()
	r = httptest.NewRequest("POST", "/oauth/token", strings.NewReader("grant_type=password"))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	p.token(rec, r)
	if rec.Code != 400 || !strings.Contains(rec.Body.String(), "unsupported_grant_type") {
		t.Fatalf("got %d %s", rec.Code, rec.Body.String())
	}
}
