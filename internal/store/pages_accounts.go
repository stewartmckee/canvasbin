package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/stewartmckee/canvasbin/internal/keys"
)

// PageSummary is one row of a user's library.
type PageSummary struct {
	ID         int64     `json:"-"`
	Title      string    `json:"title"`
	Format     string    `json:"format"`
	ReadKey    string    `json:"read_key"`
	Role       string    `json:"role"` // owner, editor or viewer
	CanWrite   bool      `json:"can_write"`
	OwnerEmail string    `json:"owner_email,omitempty"`
	TeamID     *int64    `json:"team_id,omitempty"`
	TeamName   string    `json:"team_name,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// GetForUser looks a page up by key as a signed-in user. Write access is granted
// by the write key, by owning the page, by an editor share, or by membership of
// the page's team.
func (s *Store) GetForUser(ctx context.Context, key string, userID int64) (*Access, error) {
	h := keys.Hash(key)
	var (
		a          Access
		isWriteKey bool
		role       *string
		teamRole   *string
		noReadKey  bool
	)
	err := s.db.QueryRow(ctx,
		`SELECT p.id, p.owner_id, p.team_id, p.title, p.format, p.content, p.created_at, p.updated_at,
		        p.write_key_hash = $1, s.role, tm.role, COALESCE(t.name, ''), p.read_key IS NULL
		 FROM pages p
		 LEFT JOIN page_shares s ON s.page_id = p.id AND s.user_id = $2
		 LEFT JOIN team_members tm ON tm.team_id = p.team_id AND tm.user_id = $2
		 LEFT JOIN teams t ON t.id = p.team_id
		 WHERE p.read_key_hash = $1 OR p.write_key_hash = $1`, h, userID,
	).Scan(&a.ID, &a.OwnerID, &a.TeamID, &a.Title, &a.Format, &a.Content, &a.CreatedAt, &a.UpdatedAt,
		&isWriteKey, &role, &teamRole, &a.TeamName, &noReadKey)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if teamRole != nil {
		a.TeamRole = *teamRole
	}
	a.Role = pageRole(userID, a.OwnerID, a.TeamID, a.TeamRole, role)
	a.CanWrite = isWriteKey || a.Role == RoleOwner || a.Role == RoleEditor
	if isWriteKey {
		a.WriteKey = key
		a.ReadKey = keys.ReadFromWrite(key)
	} else {
		a.ReadKey = key
	}
	if noReadKey {
		s.backfillReadKey(ctx, &a)
	}
	return &a, nil
}

// pageRole works out a user's role on a page. On a team page the creator, team
// owners and admins act as owner and other members as editors; someone who has
// left the team keeps only what a direct share gives them.
func pageRole(userID int64, ownerID, teamID *int64, teamRole string, shareRole *string) string {
	switch {
	case teamID != nil && (teamRole == TeamRoleOwner || teamRole == TeamRoleAdmin):
		return RoleOwner
	case teamID != nil && teamRole != "" && ownerID != nil && *ownerID == userID:
		return RoleOwner
	case teamID != nil && teamRole != "":
		return RoleEditor
	case teamID == nil && ownerID != nil && *ownerID == userID:
		return RoleOwner
	case shareRole != nil:
		return *shareRole
	}
	return ""
}

// backfillReadKey stores the read key of an owned page that was assigned an
// owner without one (pages created before accounts, or moved between users by
// hand), so it shows up with links in the owner's library. Opening the page by
// either key proves the key, so this is safe; failures are ignored.
func (s *Store) backfillReadKey(ctx context.Context, a *Access) {
	if a.OwnerID == nil || a.ReadKey == "" {
		return
	}
	_, _ = s.db.Exec(ctx, `UPDATE pages SET read_key = $2 WHERE id = $1 AND read_key IS NULL`, a.ID, a.ReadKey)
}

// GetByID returns a page without any key-based access: CanWrite is false and
// WriteKey is empty. ReadKey is only populated for owned pages.
func (s *Store) GetByID(ctx context.Context, id int64) (*Access, error) {
	var a Access
	err := s.db.QueryRow(ctx,
		`SELECT id, owner_id, team_id, title, format, content, created_at, updated_at, COALESCE(read_key, '')
		 FROM pages WHERE id = $1`, id,
	).Scan(&a.ID, &a.OwnerID, &a.TeamID, &a.Title, &a.Format, &a.Content, &a.CreatedAt, &a.UpdatedAt, &a.ReadKey)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &a, nil
}

// PageIDByKey resolves a read or write key to a page id.
func (s *Store) PageIDByKey(ctx context.Context, key string) (int64, error) {
	h := keys.Hash(key)
	var id int64
	err := s.db.QueryRow(ctx,
		`SELECT id FROM pages WHERE read_key_hash = $1 OR write_key_hash = $1`, h).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, ErrNotFound
	}
	return id, err
}

// UpdateForUser applies changes to the page behind key if the user may write it.
func (s *Store) UpdateForUser(ctx context.Context, key string, userID int64, c Changes) (*Access, error) {
	a, err := s.GetForUser(ctx, key, userID)
	if err != nil {
		return nil, err
	}
	if !a.CanWrite {
		return nil, ErrReadOnly
	}
	if err := s.UpdateByID(ctx, a.ID, c); err != nil {
		return nil, err
	}
	out, err := s.GetForUser(ctx, key, userID)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// UpdateByID applies changes to a page by id, with the same validation as Update.
func (s *Store) UpdateByID(ctx context.Context, id int64, c Changes) error {
	if c.Format != nil {
		f, err := NormalizeFormat(*c.Format)
		if err != nil {
			return err
		}
		c.Format = &f
	}
	if c.Content != nil && len(*c.Content) > MaxContentBytes {
		return ErrTooLarge
	}
	if c.Title != nil {
		t := cleanTitle(*c.Title)
		c.Title = &t
	}
	tag, err := s.db.Exec(ctx,
		`UPDATE pages SET
		   title = COALESCE($2, title),
		   format = COALESCE($3, format),
		   content = COALESCE($4, content),
		   updated_at = now()
		 WHERE id = $1`, id, c.Title, c.Format, c.Content)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteForUser deletes a page and returns what it was. Only the owner or a
// write-key holder may delete; editors may change content but not remove it.
func (s *Store) DeleteForUser(ctx context.Context, key string, userID int64) (*Access, error) {
	a, err := s.GetForUser(ctx, key, userID)
	if err != nil {
		return nil, err
	}
	if a.Role != RoleOwner && a.WriteKey == "" {
		return nil, ErrReadOnly
	}
	if _, err = s.db.Exec(ctx, `DELETE FROM pages WHERE id = $1`, a.ID); err != nil {
		return nil, err
	}
	return a, nil
}

// Claim attaches an unowned page to userID, given its write key. Claiming a page
// you already own is a no-op; a page owned by someone else is ErrConflict.
func (s *Store) Claim(ctx context.Context, writeKey string, userID int64) (*Access, error) {
	readKey := keys.ReadFromWrite(writeKey)
	tag, err := s.db.Exec(ctx,
		`UPDATE pages SET owner_id = $2, read_key = $3
		 WHERE write_key_hash = $1 AND owner_id IS NULL`,
		keys.Hash(writeKey), userID, readKey)
	if err != nil {
		return nil, err
	}
	if tag.RowsAffected() == 0 {
		var owner *int64
		err := s.db.QueryRow(ctx,
			`SELECT owner_id FROM pages WHERE write_key_hash = $1`, keys.Hash(writeKey)).Scan(&owner)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		if err != nil {
			return nil, err
		}
		if owner == nil || *owner != userID {
			return nil, ErrConflict
		}
	}
	return s.GetForUser(ctx, writeKey, userID)
}

// CountOwned is the number of personal (non-team) pages userID owns, which is
// what the personal quota limits.
func (s *Store) CountOwned(ctx context.Context, userID int64) (int, error) {
	var n int
	err := s.db.QueryRow(ctx, `SELECT count(*) FROM pages WHERE owner_id = $1 AND team_id IS NULL`, userID).Scan(&n)
	return n, err
}

// ListOwnedAndShared returns every page the user owns, has been shared, or can
// reach through a team, newest first.
func (s *Store) ListOwnedAndShared(ctx context.Context, userID int64) ([]PageSummary, error) {
	rows, err := s.db.Query(ctx,
		`SELECT p.id, p.owner_id, p.team_id, COALESCE(t.name, ''), p.title, p.format, COALESCE(p.read_key, ''),
		        s.role, COALESCE(tm.role, ''), COALESCE(o.email, ''), p.created_at, p.updated_at
		 FROM pages p
		 LEFT JOIN page_shares s ON s.page_id = p.id AND s.user_id = $1
		 LEFT JOIN team_members tm ON tm.team_id = p.team_id AND tm.user_id = $1
		 LEFT JOIN teams t ON t.id = p.team_id
		 LEFT JOIN users o ON o.id = p.owner_id
		 WHERE (p.owner_id = $1 AND p.team_id IS NULL) OR s.user_id = $1 OR tm.user_id = $1
		 ORDER BY p.updated_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []PageSummary{}
	for rows.Next() {
		var (
			p         PageSummary
			ownerID   *int64
			shareRole *string
			teamRole  string
		)
		if err := rows.Scan(&p.ID, &ownerID, &p.TeamID, &p.TeamName, &p.Title, &p.Format, &p.ReadKey,
			&shareRole, &teamRole, &p.OwnerEmail, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		p.Role = pageRole(userID, ownerID, p.TeamID, teamRole, shareRole)
		if p.Role == "" {
			continue
		}
		p.CanWrite = p.Role == RoleOwner || p.Role == RoleEditor
		if ownerID != nil && *ownerID == userID {
			p.OwnerEmail = ""
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// ListTeamPages returns a team's pages, newest first, for a member.
func (s *Store) ListTeamPages(ctx context.Context, teamID, userID int64) ([]PageSummary, error) {
	all, err := s.ListOwnedAndShared(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := []PageSummary{}
	for _, p := range all {
		if p.TeamID != nil && *p.TeamID == teamID {
			out = append(out, p)
		}
	}
	return out, nil
}
