package store

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
)

// PageChange is sent by the pages_notify_change trigger when a page's title,
// format or content changes, or the page is deleted.
type PageChange struct {
	ID        int64     `json:"id"`
	Deleted   bool      `json:"deleted"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ListenPageChanges calls fn for every page change until ctx is done,
// reconnecting if the database connection drops. onReconnect runs after a
// reconnect so callers can recover changes missed while disconnected.
func (s *Store) ListenPageChanges(ctx context.Context, fn func(PageChange), onReconnect func()) {
	backoff := time.Second
	for first := true; ctx.Err() == nil; first = false {
		if !first && onReconnect != nil {
			onReconnect()
		}
		err := s.listen(ctx, fn, func() { backoff = time.Second })
		if ctx.Err() != nil {
			return
		}
		slog.Error("page change listener", "err", err)
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
		backoff = min(backoff*2, 30*time.Second)
	}
}

func (s *Store) listen(ctx context.Context, fn func(PageChange), connected func()) error {
	pc, err := s.db.Acquire(ctx)
	if err != nil {
		return err
	}
	conn := pc.Hijack()
	defer conn.Close(context.Background())
	if _, err := conn.Exec(ctx, "LISTEN page_changed"); err != nil {
		return err
	}
	connected()
	for {
		n, err := conn.WaitForNotification(ctx)
		if err != nil {
			return err
		}
		var c PageChange
		if err := json.Unmarshal([]byte(n.Payload), &c); err != nil {
			slog.Error("page change payload", "err", err)
			continue
		}
		fn(c)
	}
}

// PageUpdatedAt returns when a page last changed, or ErrNotFound.
func (s *Store) PageUpdatedAt(ctx context.Context, id int64) (time.Time, error) {
	var t time.Time
	err := s.db.QueryRow(ctx, `SELECT updated_at FROM pages WHERE id = $1`, id).Scan(&t)
	if errors.Is(err, pgx.ErrNoRows) {
		return t, ErrNotFound
	}
	return t, err
}
