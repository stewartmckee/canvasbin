package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stewartmckee/canvasbin/internal/store"
)

func TestPasswordRoundTrip(t *testing.T) {
	h, err := HashPassword("correct horse battery")
	if err != nil {
		t.Fatal(err)
	}
	if h == "correct horse battery" || !strings.HasPrefix(h, "$2a$12$") {
		t.Fatalf("unexpected hash %q", h)
	}
	if !CheckPassword(h, "correct horse battery") {
		t.Fatal("correct password rejected")
	}
	if CheckPassword(h, "wrong password") {
		t.Fatal("wrong password accepted")
	}
}

func TestPasswordLimits(t *testing.T) {
	if _, err := HashPassword("short"); err != ErrWeakPassword {
		t.Fatalf("want ErrWeakPassword, got %v", err)
	}
	if _, err := HashPassword(strings.Repeat("a", 73)); err != ErrLongPassword {
		t.Fatalf("want ErrLongPassword, got %v", err)
	}
	if err := ValidatePassword(strings.Repeat("a", 72)); err != nil {
		t.Fatalf("72 bytes should be ok: %v", err)
	}
}

func TestTokens(t *testing.T) {
	a, err := NewToken()
	if err != nil {
		t.Fatal(err)
	}
	b, _ := NewToken()
	if a == b {
		t.Fatal("tokens must be unique")
	}
	if len(a) != 43 {
		t.Fatalf("want 43 chars (32 bytes base64url), got %d", len(a))
	}
	h1, h2 := HashToken(a), HashToken(a)
	if len(h1) != 32 || string(h1) != string(h2) {
		t.Fatal("hash must be deterministic 32-byte sha256")
	}
	if string(HashToken(a)) == string(HashToken(b)) {
		t.Fatal("different tokens must hash differently")
	}
}

func TestValidateEmail(t *testing.T) {
	got, err := ValidateEmail("  Alice@Example.COM ")
	if err != nil || got != "alice@example.com" {
		t.Fatalf("got %q, %v", got, err)
	}
	for _, bad := range []string{"", "nope", "a@b", "Bob <bob@example.com>", "a@@b.com"} {
		if _, err := ValidateEmail(bad); err == nil {
			t.Errorf("%q should be rejected", bad)
		}
	}
}

func TestSessionCookie(t *testing.T) {
	rec := httptest.NewRecorder()
	SetSessionCookie(rec, "tok", true)
	c := rec.Result().Cookies()[0]
	if c.Name != "canvasbin_session" || c.Value != "tok" || !c.HttpOnly || !c.Secure ||
		c.SameSite != http.SameSiteLaxMode || c.Path != "/" || c.MaxAge != 30*24*3600 {
		t.Fatalf("bad cookie: %+v", c)
	}
	rec = httptest.NewRecorder()
	ClearSessionCookie(rec, true)
	if c := rec.Result().Cookies()[0]; c.MaxAge >= 0 || c.Value != "" {
		t.Fatalf("cookie not cleared: %+v", c)
	}
}

type fakeUsers struct{ want []byte }

func (f fakeUsers) GetUserBySession(_ context.Context, h []byte) (*store.User, error) {
	if string(h) == string(f.want) {
		return &store.User{ID: 7, Email: "a@example.com"}, nil
	}
	return nil, store.ErrUnauthorized
}

func TestSessionFromRequest(t *testing.T) {
	users := fakeUsers{want: HashToken("good")}
	r := httptest.NewRequest("GET", "/", nil)
	if _, err := SessionFromRequest(r, users); err != store.ErrUnauthorized {
		t.Fatalf("no cookie: %v", err)
	}
	r.AddCookie(&http.Cookie{Name: CookieName, Value: "bad"})
	if _, err := SessionFromRequest(r, users); err != store.ErrUnauthorized {
		t.Fatalf("bad cookie: %v", err)
	}
	r = httptest.NewRequest("GET", "/", nil)
	r.AddCookie(&http.Cookie{Name: CookieName, Value: "good"})
	u, err := SessionFromRequest(r, users)
	if err != nil || u.ID != 7 {
		t.Fatalf("good cookie: %v %v", u, err)
	}
}
