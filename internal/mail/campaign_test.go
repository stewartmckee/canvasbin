package mail

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWithEmailUTM(t *testing.T) {
	got := withEmailUTM("https://canvasbin.dev/connect", campaignVerifyNudge, "connect")
	for _, want := range []string{"utm_source=email", "utm_medium=transactional", "utm_campaign=verify_nudge", "utm_content=connect"} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in %q", want, got)
		}
	}
	got2 := withEmailUTM("https://canvasbin.dev/verify?token=abc", campaignVerifySignup, "confirm")
	for _, want := range []string{"token=abc", "utm_campaign=verify_signup", "utm_content=confirm"} {
		if !strings.Contains(got2, want) {
			t.Fatalf("missing %q in %q", want, got2)
		}
	}
}

func TestSendResetUTM(t *testing.T) {
	var body string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		body = r.PostForm.Get("text")
		w.WriteHeader(200)
	}))
	defer srv.Close()
	c := New("k", "d", "f <f@d>")
	c.BaseURL = srv.URL
	if err := c.SendReset(context.Background(), "a@b.com", "https://canvasbin.test/reset?token=x"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(body, "utm_campaign=password_reset") {
		t.Fatalf("body %q", body)
	}
}
