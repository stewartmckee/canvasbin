// Package keys generates and parses the bearer keys that grant access to a page.
//
// A page has a random write key. Its read key is derived one-way from the write
// key, so a write-key holder can always produce the read-only link, while a read
// key cannot be turned back into a write key.
package keys

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"net/url"
	"strings"
)

// Length is the length of an encoded key (32 bytes, unpadded base64url).
const Length = 43

var enc = base64.RawURLEncoding

var ErrInvalid = errors.New("invalid key")

// NewWrite returns a fresh random write key.
func NewWrite() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return enc.EncodeToString(b), nil
}

// ReadFromWrite derives the read key for a write key.
func ReadFromWrite(writeKey string) string {
	sum := sha256.Sum256([]byte("canvasbin/read/v1:" + writeKey))
	return enc.EncodeToString(sum[:])
}

// Hash is the value stored in the database for a key.
func Hash(key string) []byte {
	sum := sha256.Sum256([]byte(key))
	return sum[:]
}

// Extensions are the view suffixes a page URL may carry.
var Extensions = []string{".html", ".md", ".pdf"}

// Split separates a path segment like "{key}.html" into key and extension.
func Split(seg string) (key, ext string) {
	for _, e := range Extensions {
		if strings.HasSuffix(seg, e) {
			return strings.TrimSuffix(seg, e), e
		}
	}
	return seg, ""
}

// Parse accepts a bare key or a Canvasbin URL (/{key}, /{key}.html, /{key}.md,
// /{key}.pdf, /raw/{key}, or the older /v/ and /e/ forms) and returns the key.
func Parse(s string) (string, error) {
	s = strings.TrimSpace(s)
	if i := strings.IndexAny(s, "?#"); i >= 0 {
		s = s[:i]
	}
	s, _ = Split(strings.TrimRight(s, "/"))
	if strings.Contains(s, "/") {
		if u, err := url.Parse(s); err == nil {
			s = u.Path
		}
		s = strings.TrimRight(s, "/")
		s = s[strings.LastIndex(s, "/")+1:]
	}
	if len(s) != Length {
		return "", ErrInvalid
	}
	if _, err := enc.DecodeString(s); err != nil {
		return "", ErrInvalid
	}
	return s, nil
}
