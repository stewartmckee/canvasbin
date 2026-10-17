package store

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

// ShowsBadge reports whether a page carries the "Made with Canvasbin" badge:
// true unless it belongs to an active team or its owner is on a paid plan.
func (s *Store) ShowsBadge(ctx context.Context, pageID int64) (bool, error) {
	var show bool
	err := s.db.QueryRow(ctx,
		`SELECT NOT (
		   COALESCE(t.subscription_status IN ('trialing', 'active', 'past_due'), false)
		   OR COALESCE(u.subscription_status IN ('trialing', 'active', 'past_due'), false))
		 FROM pages p
		 LEFT JOIN teams t ON t.id = p.team_id
		 LEFT JOIN users u ON u.id = p.owner_id
		 WHERE p.id = $1`, pageID).Scan(&show)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, ErrNotFound
	}
	return show, err
}
