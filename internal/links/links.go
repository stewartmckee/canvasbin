// Package links builds the public URLs for a page.
package links

import (
	"net"
	"net/http"
	"strings"
)

// Links are the URLs for a page. /{key} opens the app (the editor for a write
// key, a read-only view for a read key); /{key}.html is the rendered page.
type Links struct {
	ReadURL  string `json:"read_url"`
	WriteURL string `json:"write_url,omitempty"`
	HTMLURL  string `json:"html_url"`
	PDFURL   string `json:"pdf_url"`
	RawURL   string `json:"raw_url"`
}

// Base returns the configured public base URL, or one derived from the request
// when none is configured. Forwarded host and scheme are used only for a
// request that arrived from a trusted proxy, and only if they are a plain
// http(s) origin. Account and update mail must not depend on this fallback:
// those modes require PUBLIC_BASE_URL.
func Base(configured string, r *http.Request) string {
	if configured != "" {
		return strings.TrimRight(configured, "/")
	}
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	host := r.Host
	if trustForwardedHeaders(remoteHost(r)) {
		if p := firstValue(r.Header.Get("X-Forwarded-Proto")); p == "http" || p == "https" {
			scheme = p
		}
		if h := firstValue(r.Header.Get("X-Forwarded-Host")); validHost(h) {
			host = h
		}
	}
	if !validHost(host) {
		return scheme + "://localhost"
	}
	return scheme + "://" + host
}

func For(base, readKey, writeKey string) Links {
	l := Links{
		ReadURL: base + "/" + readKey,
		HTMLURL: base + "/" + readKey + ".html",
		PDFURL:  base + "/" + readKey + ".pdf",
		RawURL:  base + "/raw/" + readKey,
	}
	if writeKey != "" {
		l.WriteURL = base + "/" + writeKey
	}
	return l
}

// ClientIP returns the caller's address for rate limits.
//
// Forwarding headers are ignored unless the TCP peer is a loopback or private
// address (the reverse proxy on this host or Docker network). A public peer
// cannot pick its own limit bucket. When the peer is trusted, the rightmost
// X-Forwarded-For entry is used: an appending proxy puts the address it saw
// there, and a client-supplied value stays to the left. X-Real-IP is not used;
// clients can set it unless every proxy overwrites it.
func ClientIP(r *http.Request) string {
	remote := remoteHost(r)
	if !trustForwardedHeaders(remote) {
		return remote
	}
	if ip := lastForwardedIP(r.Header.Get("X-Forwarded-For")); ip != "" {
		return ip
	}
	return remote
}

func remoteHost(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func trustForwardedHeaders(remote string) bool {
	ip := net.ParseIP(remote)
	if ip == nil {
		return false
	}
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast()
}

func lastForwardedIP(h string) string {
	var last string
	for _, part := range strings.Split(h, ",") {
		part = strings.TrimSpace(part)
		ip := net.ParseIP(part)
		if ip == nil {
			continue
		}
		last = ip.String()
	}
	return last
}

func validHost(h string) bool {
	if h == "" || strings.ContainsAny(h, " \t\r\n/\\") {
		return false
	}
	if host, _, err := net.SplitHostPort(h); err == nil {
		h = host
	}
	if ip := net.ParseIP(h); ip != nil {
		return true
	}
	return !strings.Contains(h, ":")
}

func firstValue(h string) string {
	if i := strings.IndexByte(h, ','); i >= 0 {
		h = h[:i]
	}
	return strings.TrimSpace(h)
}
