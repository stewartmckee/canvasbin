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
// (honouring X-Forwarded-* from the reverse proxy) when none is configured.
func Base(configured string, r *http.Request) string {
	if configured != "" {
		return strings.TrimRight(configured, "/")
	}
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	if p := firstValue(r.Header.Get("X-Forwarded-Proto")); p != "" {
		scheme = p
	}
	host := r.Host
	if h := firstValue(r.Header.Get("X-Forwarded-Host")); h != "" {
		host = h
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

// ClientIP returns the caller's address, preferring the proxy-supplied header.
func ClientIP(r *http.Request) string {
	if ip := firstValue(r.Header.Get("X-Forwarded-For")); ip != "" {
		return ip
	}
	if ip := r.Header.Get("X-Real-IP"); ip != "" {
		return ip
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func firstValue(h string) string {
	if i := strings.IndexByte(h, ','); i >= 0 {
		h = h[:i]
	}
	return strings.TrimSpace(h)
}
