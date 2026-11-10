package geo

import (
	"net/http"
	"testing"
)

func TestDetectFromCloudflareHeader(t *testing.T) {
	r := &http.Request{Header: http.Header{}}
	r.Header.Set("CF-IPCountry", "GB")
	if Detect(r) != RegionUK {
		t.Fatal("expected UK")
	}
	r.Header.Set("CF-IPCountry", "US")
	if Detect(r) != RegionIntl {
		t.Fatal("expected intl")
	}
}

func TestFromRequestCookie(t *testing.T) {
	r := &http.Request{Header: http.Header{}}
	r.Header.Set("CF-IPCountry", "GB")
	r.AddCookie(&http.Cookie{Name: CookieName(), Value: "intl"})
	if FromRequest(r) != RegionIntl {
		t.Fatal("cookie should override header")
	}
}
