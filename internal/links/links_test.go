package links

import (
	"net/http"
	"testing"
)

func TestClientIPIgnoresSpoofedForwardedFor(t *testing.T) {
	r := httptestRequest("203.0.113.8:443")
	r.Header.Set("X-Forwarded-For", "198.51.100.9")
	r.Header.Set("X-Real-IP", "198.51.100.9")
	if got := ClientIP(r); got != "203.0.113.8" {
		t.Fatalf("public peer trusted forwarded headers: %s", got)
	}
}

func TestClientIPUsesRightmostForwardedForBehindProxy(t *testing.T) {
	r := httptestRequest("172.18.0.2:41234")
	r.Header.Set("X-Forwarded-For", "198.51.100.9, 203.0.113.4")
	r.Header.Set("X-Real-IP", "198.51.100.9")
	if got := ClientIP(r); got != "203.0.113.4" {
		t.Fatalf("got %s", got)
	}
}

func TestBaseIgnoresForwardedHostFromPublicPeer(t *testing.T) {
	r := httptestRequest("203.0.113.8:443")
	r.Host = "canvasbin.dev"
	r.Header.Set("X-Forwarded-Host", "evil.example")
	r.Header.Set("X-Forwarded-Proto", "https")
	if got := Base("", r); got != "http://canvasbin.dev" {
		t.Fatalf("got %s", got)
	}
}

func TestBaseUsesForwardedHostFromPrivatePeer(t *testing.T) {
	r := httptestRequest("127.0.0.1:41234")
	r.Host = "app:8080"
	r.Header.Set("X-Forwarded-Host", "canvasbin.dev")
	r.Header.Set("X-Forwarded-Proto", "https")
	if got := Base("", r); got != "https://canvasbin.dev" {
		t.Fatalf("got %s", got)
	}
}

func TestBaseRejectsInjectedHost(t *testing.T) {
	r := httptestRequest("10.0.0.2:41234")
	r.Host = "app:8080"
	r.Header.Set("X-Forwarded-Host", "evil.example\r\nSet-Cookie: x")
	if got := Base("", r); got != "http://app:8080" {
		t.Fatalf("got %q", got)
	}
}

func httptestRequest(remote string) *http.Request {
	r, err := http.NewRequest(http.MethodGet, "/", nil)
	if err != nil {
		panic(err)
	}
	r.RemoteAddr = remote
	return r
}
