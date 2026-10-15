package store

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// User is an account row. PasswordHash is never serialized.
type User struct {
	ID                   int64      `json:"id"`
	Email                string     `json:"email"`
	PasswordHash         string     `json:"-"`
	DisplayName          string     `json:"display_name"`
	EmailVerifiedAt      *time.Time `json:"email_verified_at,omitempty"`
	DisabledAt           *time.Time `json:"disabled_at,omitempty"`
	StripeCustomerID     *string    `json:"-"`
	StripeSubscriptionID *string    `json:"-"`
	StripePriceID        *string    `json:"-"`
	SubscriptionStatus   string     `json:"subscription_status"`
	CurrentPeriodEnd     *time.Time `json:"current_period_end,omitempty"`
	CancelAtPeriodEnd    bool       `json:"cancel_at_period_end"`
	CreatedAt            time.Time  `json:"created_at"`
	UpdatedAt            time.Time  `json:"updated_at"`
}

func (u *User) Verified() bool { return u.EmailVerifiedAt != nil }
func (u *User) Disabled() bool { return u.DisabledAt != nil }

// AdminUser is a user plus usage, for the admin listing.
type AdminUser struct {
	User
	PageCount int `json:"page_count"`
}

// Subscription is the Stripe-derived state stored on a user.
type Subscription struct {
	CustomerID        string
	SubscriptionID    string
	PriceID           string
	Status            string
	CurrentPeriodEnd  *time.Time
	CancelAtPeriodEnd bool
}

const userColumns = `id, email, password_hash, display_name, email_verified_at, disabled_at,
	stripe_customer_id, stripe_subscription_id, stripe_price_id, subscription_status,
	current_period_end, cancel_at_period_end, created_at, updated_at`

func scanUser(row pgx.Row) (*User, error) {
	var u User
	err := row.Scan(&u.ID, &u.Email, &u.PasswordHash, &u.DisplayName, &u.EmailVerifiedAt, &u.DisabledAt,
		&u.StripeCustomerID, &u.StripeSubscriptionID, &u.StripePriceID, &u.SubscriptionStatus,
		&u.CurrentPeriodEnd, &u.CancelAtPeriodEnd, &u.CreatedAt, &u.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// NormalizeEmail lowercases and trims an address for storage and lookup.
func NormalizeEmail(e string) string { return strings.ToLower(strings.TrimSpace(e)) }

func isUniqueViolation(err error) bool {
	var pe *pgconn.PgError
	return errors.As(err, &pe) && pe.Code == "23505"
}

func (s *Store) CreateUser(ctx context.Context, email, passwordHash, displayName string) (*User, error) {
	u, err := scanUser(s.db.QueryRow(ctx,
		`INSERT INTO users (email, password_hash, display_name) VALUES ($1, $2, $3)
		 RETURNING `+userColumns, NormalizeEmail(email), passwordHash, strings.TrimSpace(displayName)))
	if isUniqueViolation(err) {
		return nil, ErrEmailTaken
	}
	return u, err
}

func (s *Store) GetUserByEmail(ctx context.Context, email string) (*User, error) {
	return scanUser(s.db.QueryRow(ctx, `SELECT `+userColumns+` FROM users WHERE email = $1`, NormalizeEmail(email)))
}

func (s *Store) GetUserByID(ctx context.Context, id int64) (*User, error) {
	return scanUser(s.db.QueryRow(ctx, `SELECT `+userColumns+` FROM users WHERE id = $1`, id))
}

func (s *Store) GetUserByStripeCustomer(ctx context.Context, customerID string) (*User, error) {
	return scanUser(s.db.QueryRow(ctx, `SELECT `+userColumns+` FROM users WHERE stripe_customer_id = $1`, customerID))
}

// SetVerified marks the email verified (first verification wins).
func (s *Store) SetVerified(ctx context.Context, userID int64) error {
	_, err := s.db.Exec(ctx,
		`UPDATE users SET email_verified_at = COALESCE(email_verified_at, now()), updated_at = now() WHERE id = $1`, userID)
	return err
}

// SetPassword replaces the password hash and signs the user out everywhere.
func (s *Store) SetPassword(ctx context.Context, userID int64, passwordHash string) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck
	tag, err := tx.Exec(ctx, `UPDATE users SET password_hash = $2, updated_at = now() WHERE id = $1`, userID, passwordHash)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	if _, err := tx.Exec(ctx, `DELETE FROM sessions WHERE user_id = $1`, userID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// SetStripeCustomer records the Stripe customer id for a user.
func (s *Store) SetStripeCustomer(ctx context.Context, userID int64, customerID string) error {
	_, err := s.db.Exec(ctx,
		`UPDATE users SET stripe_customer_id = $2, updated_at = now() WHERE id = $1`, userID, customerID)
	if isUniqueViolation(err) {
		return ErrConflict
	}
	return err
}

// UpdateSubscription overwrites the subscription fields for a user.
func (s *Store) UpdateSubscription(ctx context.Context, userID int64, sub Subscription) error {
	_, err := s.db.Exec(ctx,
		`UPDATE users SET
		   stripe_customer_id = COALESCE(NULLIF($2, ''), stripe_customer_id),
		   stripe_subscription_id = NULLIF($3, ''),
		   stripe_price_id = NULLIF($4, ''),
		   subscription_status = $5,
		   current_period_end = $6,
		   cancel_at_period_end = $7,
		   updated_at = now()
		 WHERE id = $1`,
		userID, sub.CustomerID, sub.SubscriptionID, sub.PriceID, sub.Status, sub.CurrentPeriodEnd, sub.CancelAtPeriodEnd)
	if isUniqueViolation(err) {
		return ErrConflict
	}
	return err
}

// SetDisabled disables or re-enables an account. Disabling also ends its sessions
// and OAuth tokens.
func (s *Store) SetDisabled(ctx context.Context, userID int64, disabled bool) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck
	var q string
	if disabled {
		q = `UPDATE users SET disabled_at = COALESCE(disabled_at, now()), updated_at = now() WHERE id = $1`
	} else {
		q = `UPDATE users SET disabled_at = NULL, updated_at = now() WHERE id = $1`
	}
	tag, err := tx.Exec(ctx, q, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	if disabled {
		if _, err := tx.Exec(ctx, `DELETE FROM sessions WHERE user_id = $1`, userID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx,
			`UPDATE oauth_tokens SET revoked_at = now() WHERE user_id = $1 AND revoked_at IS NULL`, userID); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

// ListUsers returns users newest first with their owned page counts.
func (s *Store) ListUsers(ctx context.Context, limit, offset int) ([]AdminUser, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}
	rows, err := s.db.Query(ctx,
		`SELECT `+userColumns+`, (SELECT count(*) FROM pages p WHERE p.owner_id = users.id)
		 FROM users ORDER BY created_at DESC, id DESC LIMIT $1 OFFSET $2`, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []AdminUser{}
	for rows.Next() {
		var u AdminUser
		if err := rows.Scan(&u.ID, &u.Email, &u.PasswordHash, &u.DisplayName, &u.EmailVerifiedAt, &u.DisabledAt,
			&u.StripeCustomerID, &u.StripeSubscriptionID, &u.StripePriceID, &u.SubscriptionStatus,
			&u.CurrentPeriodEnd, &u.CancelAtPeriodEnd, &u.CreatedAt, &u.UpdatedAt, &u.PageCount); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

// ---- sessions ----

type Session struct {
	ID        int64
	UserID    int64
	ExpiresAt time.Time
}

func (s *Store) CreateSession(ctx context.Context, userID int64, tokenHash []byte, expiresAt time.Time) error {
	_, err := s.db.Exec(ctx,
		`INSERT INTO sessions (user_id, token_hash, expires_at) VALUES ($1, $2, $3)`, userID, tokenHash, expiresAt)
	return err
}

// GetSession returns an unexpired session.
func (s *Store) GetSession(ctx context.Context, tokenHash []byte) (*Session, error) {
	var sess Session
	err := s.db.QueryRow(ctx,
		`SELECT id, user_id, expires_at FROM sessions WHERE token_hash = $1 AND expires_at > now()`, tokenHash,
	).Scan(&sess.ID, &sess.UserID, &sess.ExpiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrUnauthorized
	}
	if err != nil {
		return nil, err
	}
	return &sess, nil
}

// GetUserBySession resolves a live session to its user. Disabled users are rejected.
func (s *Store) GetUserBySession(ctx context.Context, tokenHash []byte) (*User, error) {
	sess, err := s.GetSession(ctx, tokenHash)
	if err != nil {
		return nil, err
	}
	u, err := s.GetUserByID(ctx, sess.UserID)
	if errors.Is(err, ErrNotFound) {
		return nil, ErrUnauthorized
	}
	if err != nil {
		return nil, err
	}
	if u.Disabled() {
		return nil, ErrDisabled
	}
	return u, nil
}

func (s *Store) DeleteSession(ctx context.Context, tokenHash []byte) error {
	_, err := s.db.Exec(ctx, `DELETE FROM sessions WHERE token_hash = $1`, tokenHash)
	return err
}

// DeleteExpiredSessions removes sessions and email tokens past their expiry.
func (s *Store) DeleteExpiredSessions(ctx context.Context) error {
	if _, err := s.db.Exec(ctx, `DELETE FROM sessions WHERE expires_at <= now()`); err != nil {
		return err
	}
	_, err := s.db.Exec(ctx, `DELETE FROM email_tokens WHERE expires_at <= now() - interval '7 days'`)
	return err
}

// ---- email tokens ----

const (
	PurposeVerify = "verify"
	PurposeReset  = "reset"
)

func (s *Store) CreateEmailToken(ctx context.Context, userID int64, purpose string, tokenHash []byte, expiresAt time.Time) error {
	_, err := s.db.Exec(ctx,
		`INSERT INTO email_tokens (user_id, purpose, token_hash, expires_at) VALUES ($1, $2, $3, $4)`,
		userID, purpose, tokenHash, expiresAt)
	return err
}

// ConsumeEmailToken marks a token used and returns its user. Unknown, used,
// expired or wrong-purpose tokens yield ErrInvalidToken.
func (s *Store) ConsumeEmailToken(ctx context.Context, purpose string, tokenHash []byte) (int64, error) {
	var userID int64
	err := s.db.QueryRow(ctx,
		`UPDATE email_tokens SET used_at = now()
		 WHERE token_hash = $1 AND purpose = $2 AND used_at IS NULL AND expires_at > now()
		 RETURNING user_id`, tokenHash, purpose).Scan(&userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, ErrInvalidToken
	}
	return userID, err
}
