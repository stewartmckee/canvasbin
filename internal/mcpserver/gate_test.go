package mcpserver

import (
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/stewartmckee/canvasbin/internal/config"
	"github.com/stewartmckee/canvasbin/internal/store"
)

func call(tool string) string {
	return `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"` + tool + `","arguments":{}}}`
}

func TestToolCalls(t *testing.T) {
	cases := []struct {
		name string
		body string
		want []string
	}{
		{"initialize", `{"jsonrpc":"2.0","id":1,"method":"initialize"}`, nil},
		{"notification", `{"jsonrpc":"2.0","method":"notifications/initialized"}`, nil},
		{"single", call("create_page"), []string{"create_page"}},
		{"batch", `[` + call("get_page") + `,` + call("list_pages") + `]`, []string{"get_page", "list_pages"}},
		{"leading whitespace", "\n  " + call("list_pages"), []string{"list_pages"}},
		{"not a call", `{"method":"prompts/get","params":{"name":"list_pages"}}`, nil},
		{"garbage", `not json`, nil},
		{"empty", ``, nil},
	}
	for _, c := range cases {
		if got := ToolCalls([]byte(c.body)); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: ToolCalls = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestGate(t *testing.T) {
	var reached int
	var gotBody string
	var ctxUser *store.User
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reached++
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		ctxUser, _ = r.Context().Value(userCtxKey{}).(*store.User)
		w.WriteHeader(http.StatusOK)
	})
	alice := &store.User{ID: 1, Email: "alice@example.com"}
	o := Options{
		PublicBaseURL: "https://cb.example",
		Config:        config.Config{Accounts: true},
		UserFromRequest: func(r *http.Request) *store.User {
			if r.Header.Get("Authorization") == "Bearer good" {
				return alice
			}
			return nil
		},
		ScopeFromRequest: func(r *http.Request) string {
			if r.Header.Get("Authorization") == "Bearer good" {
				return "pages:read"
			}
			return ""
		},
	}
	h := o.gate(next)
	const meta = `resource_metadata="https://cb.example/.well-known/oauth-protected-resource/mcp", scope="pages:write"`

	do := func(method, body, auth string) *httptest.ResponseRecorder {
		reached, gotBody, ctxUser = 0, "", nil
		r := httptest.NewRequest(method, "/mcp", strings.NewReader(body))
		if auth != "" {
			r.Header.Set("Authorization", auth)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, r)
		return rec
	}

	// Every request needs a valid token, initialize included.
	for name, body := range map[string]string{
		"initialize": `{"jsonrpc":"2.0","id":1,"method":"initialize"}`,
		"tools/list": `{"jsonrpc":"2.0","id":2,"method":"tools/list"}`,
		"get":        call("get_page"),
		"create":     call("create_page"),
	} {
		for auth, want := range map[string]string{
			"":           "Bearer " + meta,
			"Basic abc":  "Bearer " + meta,
			"Bearer bad": `Bearer error="invalid_token", ` + meta,
		} {
			rec := do("POST", body, auth)
			if rec.Code != 401 || reached != 0 || rec.Header().Get("WWW-Authenticate") != want {
				t.Errorf("%s/%q: code=%d reached=%d hdr=%q", name, auth, rec.Code, reached, rec.Header().Get("WWW-Authenticate"))
			}
		}
	}
	if rec := do("GET", "", ""); rec.Code != 401 || reached != 0 {
		t.Errorf("GET without token: code=%d", rec.Code)
	}

	// Valid token: passes with the user in context and the body preserved.
	init := `{"jsonrpc":"2.0","id":1,"method":"initialize"}`
	if rec := do("POST", init, "Bearer good"); rec.Code != 200 || ctxUser != alice || gotBody != init {
		t.Errorf("initialize with token: code=%d user=%v", rec.Code, ctxUser)
	}
	if rec := do("POST", call("list_pages"), "Bearer good"); rec.Code != 200 || ctxUser != alice {
		t.Errorf("list_pages with token: code=%d user=%v", rec.Code, ctxUser)
	}
	// Read-only scope can't write, even inside a batch.
	for _, body := range []string{call("create_page"), `[` + call("get_page") + `,` + call("update_page") + `]`} {
		rec := do("POST", body, "Bearer good")
		if rec.Code != 403 || reached != 0 || !strings.Contains(rec.Header().Get("WWW-Authenticate"), `error="insufficient_scope"`) {
			t.Errorf("write with read scope: code=%d hdr=%q", rec.Code, rec.Header().Get("WWW-Authenticate"))
		}
	}
	if rec := do("GET", "", "Bearer good"); rec.Code != 200 || reached != 1 {
		t.Errorf("GET with token: code=%d reached=%d", rec.Code, reached)
	}
}

func TestGateBodyCap(t *testing.T) {
	o := Options{
		Config:           config.Config{Accounts: true},
		UserFromRequest:  func(*http.Request) *store.User { return &store.User{ID: 1} },
		ScopeFromRequest: func(*http.Request) string { return "pages:write" },
	}
	h := o.gate(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { t.Fatal("should not reach next") }))
	r := httptest.NewRequest("POST", "/mcp", strings.NewReader(strings.Repeat("x", maxGateBody+1)))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("code=%d", rec.Code)
	}
}
