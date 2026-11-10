package store

import (
	"context"
	"time"
)

// ClaimUnverifiedForNudge marks verify_nudge_sent_at and returns users who signed
// up at least minAge ago, are still unverified, and have not been nudged before.
func (s *Store) ClaimUnverifiedForNudge(ctx context.Context, minAge time.Duration, limit int) ([]*User, error) {
	if limit <= 0 {
		limit = 20
	}
	cutoff := time.Now().Add(-minAge)
	rows, err := s.db.Query(ctx, `
		UPDATE users u SET verify_nudge_sent_at = now(), updated_at = now()
		WHERE u.id IN (
			SELECT id FROM users
			WHERE email_verified_at IS NULL
			  AND disabled_at IS NULL
			  AND verify_nudge_sent_at IS NULL
			  AND created_at <= $1
			ORDER BY created_at
			LIMIT $2
			FOR UPDATE SKIP LOCKED
		)
		RETURNING `+userColumns, cutoff, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*User
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}
