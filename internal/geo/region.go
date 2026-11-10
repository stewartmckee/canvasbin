package geo

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/stewartmckee/canvasbin/internal/links"
)

// Region selects which Stripe prices and displayed currency to use.
type Region string

const (
	RegionUK   Region = "uk"
	RegionIntl Region = "intl"
)

const billingRegionCookie = "cb_billing_region"

// Detect chooses UK (GBP) vs international (USD) from proxy geo headers, a
// short-lived IP lookup, or defaults to international for unknown addresses.
func Detect(r *http.Request) Region {
	if c := strings.TrimSpace(r.Header.Get("X-Billing-Region")); c == "uk" || c == "intl" {
		return Region(c)
	}
	for _, h := range []string{"CF-IPCountry", "X-Country-Code", "X-Appengine-Country", "CloudFront-Viewer-Country"} {
		if cc := countryFromHeader(r.Header.Get(h)); cc != "" {
			if cc == "GB" {
				return RegionUK
			}
			return RegionIntl
		}
	}
	ip := links.ClientIP(r)
	if cc := lookupCountry(ip); cc == "GB" {
		return RegionUK
	}
	return RegionIntl
}

// FromRequest returns the billing region for this request, preferring the
// sticky cookie set by /api/config when present.
func FromRequest(r *http.Request) Region {
	if c, err := r.Cookie(billingRegionCookie); err == nil {
		switch strings.TrimSpace(c.Value) {
		case "uk":
			return RegionUK
		case "intl":
			return RegionIntl
		}
	}
	return Detect(r)
}

// CookieName is the sticky billing region cookie.
func CookieName() string { return billingRegionCookie }

func countryFromHeader(v string) string {
	v = strings.TrimSpace(strings.ToUpper(v))
	if v == "UK" {
		return "GB"
	}
	if len(v) == 2 {
		return v
	}
	return ""
}

var (
	cache   = map[string]cacheEntry{}
	cacheMu sync.Mutex
)

type cacheEntry struct {
	country string
	expires time.Time
}

func lookupCountry(ip string) string {
	if ip == "" {
		return ""
	}
	parsed := net.ParseIP(ip)
	if parsed == nil || parsed.IsLoopback() || parsed.IsPrivate() || parsed.IsLinkLocalUnicast() {
		return ""
	}
	cacheMu.Lock()
	if e, ok := cache[ip]; ok && time.Now().Before(e.expires) {
		cacheMu.Unlock()
		return e.country
	}
	cacheMu.Unlock()

	cc := fetchCountryCode(ip)
	cacheMu.Lock()
	cache[ip] = cacheEntry{country: cc, expires: time.Now().Add(24 * time.Hour)}
	cacheMu.Unlock()
	return cc
}
