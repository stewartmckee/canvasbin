package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

// OAuthClient is a registered (dynamic or static) OAuth client for MCP.
type OAuthClient struct {
	ClientID                string
	ClientSecretHash        []byte
	RedirectURIs            []string
	ClientName              string
	TokenEndpointAuthMethod string
	CreatedAt               time.Time
}

// OAuthAuthCode is a pending authorization code (stored hashed).
type OAuthAuthCode struct {
	ClientID            string
	UserID              int64
	RedirectURI         string
	CodeChallenge       string
	CodeChallengeMethod string
	Scope               string
	Resource            string
	ExpiresAt           time.Time
}

// OAuthToken is an issued access/refresh token pair (stored hashed).
type OAuthToken struct {
	ID               int64
	AccessHash       []byte
	RefreshHash      []byte
	ClientID         string
	UserID           int64
	Scope            string
	Resource         string
	AccessExpiresAt  time.Time
	RefreshExpiresAt *time.Time
	RevokedAt        *time.Time
	CreatedAt        time.Time
}

func (s *Store) CreateOAuthClient(ctx context.Context, c OAuthClient) error {
	if c.TokenEndpointAuthMethod == "" {
		c.TokenEndpointAuthMethod = "none"
	}
	_, err := s.db.Exec(ctx,
		`INSERT INTO oauth_clients (client_id, client_secret_hash, redirect_uris, client_name, token_endpoint_auth_method)
		 VALUES ($1, $2, $3, $4, $5)`,
		c.ClientID, c.ClientSecretHash, c.RedirectURIs, c.ClientName, c.TokenEndpointAuthMethod)
	if isUniqueViolation(err) {
		return ErrConflict
	}
	return err
}

func (s *Store) GetOAuthClient(ctx context.Context, clientID string) (*OAuthClient, error) {
	var c OAuthClient
	err := s.db.QueryRow(ctx,
		`SELECT client_id, client_secret_hash, redirect_uris, client_name, token_endpoint_auth_method, created_at
		 FROM oauth_clients WHERE client_id = $1`, clientID,
	).Scan(&c.ClientID, &c.ClientSecretHash, &c.RedirectURIs, &c.ClientName, &c.TokenEndpointAuthMethod, &c.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (s *Store) CreateAuthCode(ctx context.Context, codeHash []byte, c OAuthAuthCode) error {
	if c.CodeChallengeMethod == "" {
		c.CodeChallengeMethod = "S256"
	}
	_, err := s.db.Exec(ctx,
		`INSERT INTO oauth_auth_codes
		   (code_hash, client_id, user_id, redirect_uri, code_challenge, code_challenge_method, scope, resource, expires_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		codeHash, c.ClientID, c.UserID, c.RedirectURI, c.CodeChallenge, c.CodeChallengeMethod, c.Scope, c.Resource, c.ExpiresAt)
	return err
}

// ConsumeAuthCode atomically marks an unused, unexpired code as used and returns
// it. Anything else yields ErrInvalidToken.
func (s *Store) ConsumeAuthCode(ctx context.Context, codeHash []byte) (*OAuthAuthCode, error) {
	var c OAuthAuthCode
	err := s.db.QueryRow(ctx,
		`UPDATE oauth_auth_codes SET used_at = now()
		 WHERE code_hash = $1 AND used_at IS NULL AND expires_at > now()
		 RETURNING client_id, user_id, redirect_uri, code_challenge, code_challenge_method, scope, resource, expires_at`,
		codeHash,
	).Scan(&c.ClientID, &c.UserID, &c.RedirectURI, &c.CodeChallenge, &c.CodeChallengeMethod, &c.Scope, &c.Resource, &c.ExpiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrInvalidToken
	}
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (s *Store) CreateOAuthToken(ctx context.Context, t OAuthToken) error {
	_, err := s.db.Exec(ctx,
		`INSERT INTO oauth_tokens
		   (access_hash, refresh_hash, client_id, user_id, scope, resource, access_expires_at, refresh_expires_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		t.AccessHash, t.RefreshHash, t.ClientID, t.UserID, t.Scope, t.Resource, t.AccessExpiresAt, t.RefreshExpiresAt)
	return err
}

const oauthTokenColumns = `id, access_hash, refresh_hash, client_id, user_id, scope, resource,
	access_expires_at, refresh_expires_at, revoked_at, created_at`

func scanOAuthToken(row pgx.Row) (*OAuthToken, error) {
	var t OAuthToken
	err := row.Scan(&t.ID, &t.AccessHash, &t.RefreshHash, &t.ClientID, &t.UserID, &t.Scope, &t.Resource,
		&t.AccessExpiresAt, &t.RefreshExpiresAt, &t.RevokedAt, &t.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrInvalidToken
	}
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// GetOAuthTokenByAccess returns a live (unrevoked, unexpired) access token.
func (s *Store) GetOAuthTokenByAccess(ctx context.Context, accessHash []byte) (*OAuthToken, error) {
	return scanOAuthToken(s.db.QueryRow(ctx,
		`SELECT `+oauthTokenColumns+` FROM oauth_tokens
		 WHERE access_hash = $1 AND revoked_at IS NULL AND access_expires_at > now()`, accessHash))
}

// GetOAuthTokenByRefresh returns a live refresh token.
func (s *Store) GetOAuthTokenByRefresh(ctx context.Context, refreshHash []byte) (*OAuthToken, error) {
	return scanOAuthToken(s.db.QueryRow(ctx,
		`SELECT `+oauthTokenColumns+` FROM oauth_tokens
		 WHERE refresh_hash = $1 AND revoked_at IS NULL
		   AND (refresh_expires_at IS NULL OR refresh_expires_at > now())`, refreshHash))
}

// RevokeOAuthToken revokes one token pair by id. Revoking twice is harmless.
func (s *Store) RevokeOAuthToken(ctx context.Context, id int64) error {
	_, err := s.db.Exec(ctx, `UPDATE oauth_tokens SET revoked_at = COALESCE(revoked_at, now()) WHERE id = $1`, id)
	return err
}

// RevokeOAuthTokenByHash revokes the pair that a presented access or refresh
// token belongs to (RFC 7009 style).
func (s *Store) RevokeOAuthTokenByHash(ctx context.Context, hash []byte) error {
	_, err := s.db.Exec(ctx,
		`UPDATE oauth_tokens SET revoked_at = COALESCE(revoked_at, now())
		 WHERE access_hash = $1 OR refresh_hash = $1`, hash)
	return err
}

// ConnectedApp is an OAuth client holding at least one live token for a user.
type ConnectedApp struct {
	ClientID    string    `json:"client_id"`
	ClientName  string    `json:"client_name"`
	Scope       string    `json:"scope"`
	ConnectedAt time.Time `json:"connected_at"`
	LastUsedAt  time.Time `json:"last_used_at"`
}

// ListConnectedApps returns each client with a live token for userID, newest
// activity first. ClientName is empty for clients that aren't registered rows
// (URL client IDs).
func (s *Store) ListConnectedApps(ctx context.Context, userID int64) ([]ConnectedApp, error) {
	rows, err := s.db.Query(ctx,
		`SELECT t.client_id, COALESCE(MAX(c.client_name), ''), MAX(t.scope), MIN(t.created_at), MAX(t.created_at)
		 FROM oauth_tokens t
		 LEFT JOIN oauth_clients c ON c.client_id = t.client_id
		 WHERE t.user_id = $1 AND t.revoked_at IS NULL
		   AND (t.access_expires_at > now() OR t.refresh_expires_at IS NULL OR t.refresh_expires_at > now())
		 GROUP BY t.client_id
		 ORDER BY MAX(t.created_at) DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	apps := []ConnectedApp{}
	for rows.Next() {
		var a ConnectedApp
		if err := rows.Scan(&a.ClientID, &a.ClientName, &a.Scope, &a.ConnectedAt, &a.LastUsedAt); err != nil {
			return nil, err
		}
		apps = append(apps, a)
	}
	return apps, rows.Err()
}

// RevokeClientTokens revokes every live token clientID holds for userID and
// reports how many were revoked.
func (s *Store) RevokeClientTokens(ctx context.Context, userID int64, clientID string) (int64, error) {
	tag, err := s.db.Exec(ctx,
		`UPDATE oauth_tokens SET revoked_at = now()
		 WHERE user_id = $1 AND client_id = $2 AND revoked_at IS NULL`, userID, clientID)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

// RotateOAuthToken revokes old and stores next in one transaction (refresh grant).
func (s *Store) RotateOAuthToken(ctx context.Context, oldID int64, next OAuthToken) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck
	tag, err := tx.Exec(ctx, `UPDATE oauth_tokens SET revoked_at = now() WHERE id = $1 AND revoked_at IS NULL`, oldID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrInvalidToken
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO oauth_tokens
		   (access_hash, refresh_hash, client_id, user_id, scope, resource, access_expires_at, refresh_expires_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		next.AccessHash, next.RefreshHash, next.ClientID, next.UserID, next.Scope, next.Resource,
		next.AccessExpiresAt, next.RefreshExpiresAt); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
