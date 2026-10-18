package mail

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSendVerify(t *testing.T) {
	var gotPath, gotUser, gotPass string
	var gotForm map[string]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotUser, gotPass, _ = r.BasicAuth()
		_ = r.ParseForm()
		gotForm = map[string]string{"from": r.PostForm.Get("from"), "to": r.PostForm.Get("to"), "text": r.PostForm.Get("text"), "subject": r.PostForm.Get("subject")}
		w.WriteHeader(200)
	}))
	defer srv.Close()

	c := New("test-key", "example.dev", "Canvasbin <noreply@example.dev>")
	c.BaseURL = srv.URL
	if err := c.SendVerify(context.Background(), "a@b.com", "https://x/verify?token=abc"); err != nil {
		t.Fatal(err)
	}
	if gotPath != "/v3/example.dev/messages" || gotUser != "api" || gotPass != "test-key" {
		t.Fatalf("bad request: %s %s %s", gotPath, gotUser, gotPass)
	}
	if gotForm["to"] != "a@b.com" || gotForm["from"] != "Canvasbin <noreply@example.dev>" || !strings.Contains(gotForm["text"], "token=abc") {
		t.Fatalf("bad form: %v", gotForm)
	}
}

func TestSendWelcome(t *testing.T) {
	var text, subject string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		text, subject = r.PostForm.Get("text"), r.PostForm.Get("subject")
	}))
	defer srv.Close()
	c := New("k", "example.dev", "x <x@example.dev>")
	c.BaseURL = srv.URL
	if err := c.SendWelcome(context.Background(), "a@b.com", "https://canvasbin.test"); err != nil {
		t.Fatal(err)
	}
	if subject == "" || !strings.Contains(text, "https://canvasbin.test/connect") || !strings.Contains(text, "https://canvasbin.test/new") {
		t.Fatalf("subject %q text %q", subject, text)
	}
}

func TestErrorsDoNotLeakKey(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Forbidden", 401)
	}))
	defer srv.Close()
	c := New("super-secret-key", "example.dev", "x <x@example.dev>")
	c.BaseURL = srv.URL
	err := c.SendReset(context.Background(), "a@b.com", "https://x/r")
	if err == nil || strings.Contains(err.Error(), "super-secret-key") {
		t.Fatalf("want error without key, got %v", err)
	}
	// Transport failure too.
	c.BaseURL = "http://127.0.0.1:1"
	if err := c.SendReset(context.Background(), "a@b.com", "x"); err == nil || strings.Contains(err.Error(), "super-secret-key") {
		t.Fatalf("transport error must not leak key: %v", err)
	}
}

func TestNoKeyAndHeaderInjection(t *testing.T) {
	if err := New("", "d", "f").SendVerify(context.Background(), "a@b.com", "x"); err == nil {
		t.Fatal("expected error without key")
	}
	if err := New("k", "d", "f").SendVerify(context.Background(), "a@b.com\nbcc: x@y.com", "x"); err == nil {
		t.Fatal("expected error for newline in recipient")
	}
}
