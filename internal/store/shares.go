package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

// Share is a user who has been granted access to a page.
type Share struct {
	UserID      int64     `json:"user_id"`
	Email       string    `json:"email"`
	DisplayName string    `json:"display_name"`
	Role        string    `json:"role"`
	CreatedAt   time.Time `json:"created_at"`
}

// Invite is a pending share for an email that has no account yet.
type Invite struct {
	ID        int64     `json:"id"`
	Email     string    `json:"email"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"created_at"`
}

// ShareResult says what ShareWith did.
type ShareResult struct {
	// Shared is true when the user already had an account and was shared directly;
	// otherwise an Invite was recorded and an email should go out.
	Shared bool
	User   *User
	Invite *Invite
}

func validRole(r string) bool { return r == RoleViewer || r == RoleEditor }

// ShareWith grants email access to a page, or records an invite if no such user
// exists. Re-sharing updates the role. Sharing with the owner is ErrConflict.
func (s *Store) ShareWith(ctx context.Context, pageID, inviterID int64, email, role string) (*ShareResult, error) {
	if !validRole(role) {
		return nil, errors.New(`role must be "viewer" or "editor"`)
	}
	email = NormalizeEmail(email)
	u, err := s.GetUserByEmail(ctx, email)
	switch {
	case err == nil:
		var owner *int64
		if err := s.db.QueryRow(ctx, `SELECT owner_id FROM pages WHERE id = $1`, pageID).Scan(&owner); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil, ErrNotFound
			}
			return nil, err
		}
		if owner != nil && *owner == u.ID {
			return nil, ErrConflict
		}
		if _, err := s.db.Exec(ctx,
			`INSERT INTO page_shares (page_id, user_id, role) VALUES ($1, $2, $3)
			 ON CONFLICT (page_id, user_id) DO UPDATE SET role = EXCLUDED.role`, pageID, u.ID, role); err != nil {
			return nil, err
		}
		return &ShareResult{Shared: true, User: u}, nil
	case errors.Is(err, ErrNotFound):
		inv := &Invite{}
		err := s.db.QueryRow(ctx,
			`INSERT INTO page_invites (page_id, email, role, invited_by) VALUES ($1, $2, $3, $4)
			 ON CONFLICT (page_id, email) DO UPDATE SET role = EXCLUDED.role
			 RETURNING id, email, role, created_at`, pageID, email, role, inviterID,
		).Scan(&inv.ID, &inv.Email, &inv.Role, &inv.CreatedAt)
		if err != nil {
			return nil, err
		}
		return &ShareResult{Invite: inv}, nil
	default:
		return nil, err
	}
}

// ListShares returns the shares and pending invites for a page.
func (s *Store) ListShares(ctx context.Context, pageID int64) ([]Share, []Invite, error) {
	rows, err := s.db.Query(ctx,
		`SELECT u.id, u.email, u.display_name, ps.role, ps.created_at
		 FROM page_shares ps JOIN users u ON u.id = ps.user_id
		 WHERE ps.page_id = $1 ORDER BY ps.created_at`, pageID)
	if err != nil {
		return nil, nil, err
	}
	shares := []Share{}
	for rows.Next() {
		var sh Share
		if err := rows.Scan(&sh.UserID, &sh.Email, &sh.DisplayName, &sh.Role, &sh.CreatedAt); err != nil {
			rows.Close()
			return nil, nil, err
		}
		shares = append(shares, sh)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}
	irows, err := s.db.Query(ctx,
		`SELECT id, email, role, created_at FROM page_invites WHERE page_id = $1 ORDER BY created_at`, pageID)
	if err != nil {
		return nil, nil, err
	}
	defer irows.Close()
	invites := []Invite{}
	for irows.Next() {
		var inv Invite
		if err := irows.Scan(&inv.ID, &inv.Email, &inv.Role, &inv.CreatedAt); err != nil {
			return nil, nil, err
		}
		invites = append(invites, inv)
	}
	return shares, invites, irows.Err()
}

// RemoveShare revokes a share or pending invite for email. It reports whether
// anything was removed.
func (s *Store) RemoveShare(ctx context.Context, pageID int64, email string) (bool, error) {
	email = NormalizeEmail(email)
	tag, err := s.db.Exec(ctx,
		`DELETE FROM page_shares WHERE page_id = $1 AND user_id = (SELECT id FROM users WHERE email = $2)`,
		pageID, email)
	if err != nil {
		return false, err
	}
	removed := tag.RowsAffected() > 0
	tag, err = s.db.Exec(ctx, `DELETE FROM page_invites WHERE page_id = $1 AND email = $2`, pageID, email)
	if err != nil {
		return false, err
	}
	return removed || tag.RowsAffected() > 0, nil
}

// AcceptInvites converts any pending invites for the user's email into shares.
// Call it once the email is verified so that invites can't be claimed by
// someone who merely typed the address at signup.
func (s *Store) AcceptInvites(ctx context.Context, userID int64, email string) (int, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck
	tag, err := tx.Exec(ctx,
		`INSERT INTO page_shares (page_id, user_id, role)
		 SELECT i.page_id, $1, i.role FROM page_invites i
		 JOIN pages p ON p.id = i.page_id
		 WHERE i.email = $2 AND (p.owner_id IS NULL OR p.owner_id <> $1)
		 ON CONFLICT (page_id, user_id) DO NOTHING`, userID, NormalizeEmail(email))
	if err != nil {
		return 0, err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM page_invites WHERE email = $1`, NormalizeEmail(email)); err != nil {
		return 0, err
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}
	return int(tag.RowsAffected()), nil
}
