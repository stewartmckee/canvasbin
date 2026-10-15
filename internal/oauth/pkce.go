package oauth

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"net/url"
	"strings"
)

// VerifyPKCE checks a code_verifier against its S256 code_challenge
// (BASE64URL(SHA256(verifier)), RFC 7636). The verifier must be 43-128
// unreserved characters.
func VerifyPKCE(verifier, challenge string) bool {
	if !validVerifier(verifier) || !ValidChallenge(challenge) {
		return false
	}
	sum := sha256.Sum256([]byte(verifier))
	got := base64.RawURLEncoding.EncodeToString(sum[:])
	return subtle.ConstantTimeCompare([]byte(got), []byte(challenge)) == 1
}

// ValidChallenge reports whether c looks like an S256 challenge: the unpadded
// base64url of a 32-byte digest (43 characters).
func ValidChallenge(c string) bool {
	if len(c) != 43 {
		return false
	}
	for i := 0; i < len(c); i++ {
		if !isBase64URL(c[i]) {
			return false
		}
	}
	return true
}

func validVerifier(v string) bool {
	if len(v) < 43 || len(v) > 128 {
		return false
	}
	for i := 0; i < len(v); i++ {
		c := v[i]
		if !(isBase64URL(c) || c == '.' || c == '~') {
			return false
		}
	}
	return true
}

func isBase64URL(c byte) bool {
	return c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-' || c == '_'
}

// ---- redirect URIs ----

const maxRedirectURILen = 2048

func isLoopbackHost(h string) bool {
	return h == "localhost" || h == "127.0.0.1" || h == "::1"
}

// ValidateRedirectURI accepts https URLs and loopback http URLs
// (http://localhost, http://127.0.0.1, http://[::1], any port). Fragments and
// userinfo are never allowed.
func ValidateRedirectURI(raw string) error {
	if raw == "" || len(raw) > maxRedirectURILen {
		return errBadRedirect
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || u.Fragment != "" || u.User != nil || u.Opaque != "" || strings.Contains(raw, "#") {
		return errBadRedirect
	}
	switch u.Scheme {
	case "https":
		return nil
	case "http":
		if isLoopbackHost(u.Hostname()) {
			return nil
		}
	}
	return errBadRedirect
}

// RedirectAllowed reports whether got may be used by a client that registered
// the given URIs. Matching is exact, except that a registered loopback http URI
// matches any port (RFC 8252 section 7.3).
func RedirectAllowed(registered []string, got string) bool {
	if ValidateRedirectURI(got) != nil {
		return false
	}
	gu, err := url.Parse(got)
	if err != nil {
		return false
	}
	for _, reg := range registered {
		if reg == got {
			return true
		}
		ru, err := url.Parse(reg)
		if err != nil || ru.Scheme != "http" || gu.Scheme != "http" || !isLoopbackHost(ru.Hostname()) {
			continue
		}
		if ru.Hostname() == gu.Hostname() && ru.Path == gu.Path && ru.RawQuery == gu.RawQuery {
			return true
		}
	}
	return false
}
