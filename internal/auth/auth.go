// Package auth holds password, token and session-cookie helpers.
package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"net/http"
	"net/mail"
	"strings"
	"time"
	"unicode/utf8"

	"golang.org/x/crypto/bcrypt"

	"github.com/stewartmckee/canvasbin/internal/store"
)

const (
	CookieName      = "canvasbin_session"
	SessionLifetime = 30 * 24 * time.Hour
	BcryptCost      = 12

	MinPasswordLen = 8
	// bcrypt ignores (or rejects) input beyond 72 bytes.
	MaxPasswordBytes = 72
)

var (
	ErrWeakPassword = errors.New("password must be at least 8 characters")
	ErrLongPassword = errors.New("password must be at most 72 bytes")
	ErrBadEmail     = errors.New("enter a valid email address")
)

// NewToken returns 32 random bytes as unpadded base64url.
func NewToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// HashToken is the SHA-256 stored in place of a token.
func HashToken(token string) []byte {
	sum := sha256.Sum256([]byte(token))
	return sum[:]
}

func HashPassword(password string) (string, error) {
	if err := ValidatePassword(password); err != nil {
		return "", err
	}
	h, err := bcrypt.GenerateFromPassword([]byte(password), BcryptCost)
	return string(h), err
}

func CheckPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

func ValidatePassword(p string) error {
	if utf8.RuneCountInString(p) < MinPasswordLen {
		return ErrWeakPassword
	}
	if len(p) > MaxPasswordBytes {
		return ErrLongPassword
	}
	return nil
}

// ValidateEmail accepts a bare address (no display name) and returns it normalised.
func ValidateEmail(e string) (string, error) {
	e = store.NormalizeEmail(e)
	a, err := mail.ParseAddress(e)
	if err != nil || a.Address != e || len(e) > 254 || !strings.Contains(e[strings.LastIndex(e, "@"):], ".") {
		return "", ErrBadEmail
	}
	return e, nil
}

// SetSessionCookie sets the session cookie. secure should be true except when
// serving plain http for local development.
func SetSessionCookie(w http.ResponseWriter, token string, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    token,
		Path:     "/",
		MaxAge:   int(SessionLifetime / time.Second),
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})
}

func ClearSessionCookie(w http.ResponseWriter, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})
}

// TokenFromRequest returns the raw session token cookie, if present.
func TokenFromRequest(r *http.Request) (string, bool) {
	c, err := r.Cookie(CookieName)
	if err != nil || c.Value == "" {
		return "", false
	}
	return c.Value, true
}

// SessionUsers resolves a hashed session token to a user.
type SessionUsers interface {
	GetUserBySession(ctx context.Context, tokenHash []byte) (*store.User, error)
}

// SessionFromRequest returns the signed-in user for the request's session
// cookie, or store.ErrUnauthorized when there is none or it has expired.
func SessionFromRequest(r *http.Request, users SessionUsers) (*store.User, error) {
	token, ok := TokenFromRequest(r)
	if !ok {
		return nil, store.ErrUnauthorized
	}
	return users.GetUserBySession(r.Context(), HashToken(token))
}
