package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/stewartmckee/canvasbin/internal/keys"
)

const (
	SubscriberPending      = "pending"
	SubscriberActive       = "active"
	SubscriberUnsubscribed = "unsubscribed"

	MaxUpdateBodyBytes = 64 << 10
)

var (
	ErrUpdatesDisabled   = errors.New("updates are not enabled for this page")
	ErrUpdatesOff        = errors.New("updates feature is not available")
	ErrAlreadySubscribed = errors.New("already subscribed")
)

// PageUpdate is one append-only post on a page.
type PageUpdate struct {
	ID        int64     `json:"id"`
	PageID    int64     `json:"-"`
	Title     string    `json:"title"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"created_at"`
}

// SetUpdatesEnabled toggles public updates for a page.
func (s *Store) SetUpdatesEnabled(ctx context.Context, pageID int64, on bool) error {
	tag, err := s.db.Exec(ctx, `UPDATE pages SET updates_enabled = $2, updated_at = now() WHERE id = $1`, pageID, on)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// UpdatesEnabled reports whether a page accepts subscriptions and posts.
func (s *Store) UpdatesEnabled(ctx context.Context, pageID int64) (bool, error) {
	var on bool
	err := s.db.QueryRow(ctx, `SELECT updates_enabled FROM pages WHERE id = $1`, pageID).Scan(&on)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, ErrNotFound
	}
	return on, err
}

// ActiveSubscriberCount is the number of confirmed subscribers.
func (s *Store) ActiveSubscriberCount(ctx context.Context, pageID int64) (int, error) {
	var n int
	err := s.db.QueryRow(ctx,
		`SELECT count(*) FROM page_update_subscribers WHERE page_id = $1 AND status = $2`,
		pageID, SubscriberActive).Scan(&n)
	return n, err
}

func cleanUpdate(title, body string) (string, string, error) {
	title = cleanTitle(title)
	if title == "" {
		return "", "", errors.New("title is required")
	}
	if len(body) > MaxUpdateBodyBytes {
		return "", "", ErrTooLarge
	}
	return title, body, nil
}

// CreatePageUpdate inserts a post and bumps the page updated_at for live reload.
func (s *Store) CreatePageUpdate(ctx context.Context, pageID int64, userID *int64, title, body string) (*PageUpdate, error) {
	title, body, err := cleanUpdate(title, body)
	if err != nil {
		return nil, err
	}
	on, err := s.UpdatesEnabled(ctx, pageID)
	if err != nil {
		return nil, err
	}
	if !on {
		return nil, ErrUpdatesDisabled
	}
	var u PageUpdate
	err = s.db.QueryRow(ctx,
		`INSERT INTO page_updates (page_id, title, body, created_by_user_id)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, page_id, title, body, created_at`,
		pageID, title, body, userID,
	).Scan(&u.ID, &u.PageID, &u.Title, &u.Body, &u.CreatedAt)
	if err != nil {
		return nil, err
	}
	_, _ = s.db.Exec(ctx, `UPDATE pages SET updated_at = now() WHERE id = $1`, pageID)
	return &u, nil
}

// ListPageUpdates returns posts newest first.
func (s *Store) ListPageUpdates(ctx context.Context, pageID int64, limit int) ([]PageUpdate, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	rows, err := s.db.Query(ctx,
		`SELECT id, page_id, title, body, created_at
		 FROM page_updates WHERE page_id = $1
		 ORDER BY created_at DESC, id DESC LIMIT $2`, pageID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []PageUpdate
	for rows.Next() {
		var u PageUpdate
		if err := rows.Scan(&u.ID, &u.PageID, &u.Title, &u.Body, &u.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

// UpdateSubscriber is an active subscriber row.
type UpdateSubscriber struct {
	ID    int64
	Email string
}

// SubscribePage starts or refreshes a pending subscription for email on pageID.
func (s *Store) SubscribePage(ctx context.Context, pageID int64, email string) (confirmToken string, err error) {
	email = NormalizeEmail(email)
	if email == "" {
		return "", errors.New("email is required")
	}
	on, err := s.UpdatesEnabled(ctx, pageID)
	if err != nil {
		return "", err
	}
	if !on {
		return "", ErrUpdatesDisabled
	}
	var status string
	err = s.db.QueryRow(ctx,
		`SELECT status FROM page_update_subscribers WHERE page_id = $1 AND email = $2`,
		pageID, email).Scan(&status)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return "", err
	}
	if err == nil && status == SubscriberActive {
		return "", ErrAlreadySubscribed
	}
	isNew := errors.Is(err, pgx.ErrNoRows)
	confirmToken, err = keys.NewWrite()
	if err != nil {
		return "", err
	}
	unsubToken, err := keys.NewWrite()
	if err != nil {
		return "", err
	}
	if isNew {
		_, err = s.db.Exec(ctx,
			`INSERT INTO page_update_subscribers (page_id, email, status, confirm_token, unsubscribe_token)
			 VALUES ($1, $2, $3, $4, $5)`,
			pageID, email, SubscriberPending, confirmToken, unsubToken)
		return confirmToken, err
	}
	_, err = s.db.Exec(ctx,
		`UPDATE page_update_subscribers SET status = $3, confirm_token = $4, unsubscribe_token = $5, confirmed_at = NULL
		 WHERE page_id = $1 AND email = $2`,
		pageID, email, SubscriberPending, confirmToken, unsubToken)
	return confirmToken, err
}

// SubscriptionPage describes the page a subscription link belongs to.
type SubscriptionPage struct {
	Title   string
	ReadKey string // empty for pages that don't keep their read key
	Email   string
}

// ConfirmSubscriber activates a pending subscription. Confirming again is
// harmless; an unsubscribed address isn't brought back by an old link.
func (s *Store) ConfirmSubscriber(ctx context.Context, token string) (SubscriptionPage, error) {
	var out SubscriptionPage
	err := s.db.QueryRow(ctx,
		`UPDATE page_update_subscribers s SET status = $2, confirmed_at = COALESCE(s.confirmed_at, now())
		 FROM pages p
		 WHERE s.confirm_token = $1 AND s.status IN ($2, $3) AND p.id = s.page_id
		 RETURNING p.title, COALESCE(p.read_key, ''), s.email`,
		token, SubscriberActive, SubscriberPending).Scan(&out.Title, &out.ReadKey, &out.Email)
	if errors.Is(err, pgx.ErrNoRows) {
		return out, ErrInvalidToken
	}
	return out, err
}

// UnsubscribeByToken marks a subscriber unsubscribed (one step).
func (s *Store) UnsubscribeByToken(ctx context.Context, token string) (SubscriptionPage, error) {
	var out SubscriptionPage
	err := s.db.QueryRow(ctx,
		`UPDATE page_update_subscribers s SET status = $2
		 FROM pages p
		 WHERE s.unsubscribe_token = $1 AND p.id = s.page_id
		 RETURNING p.title, COALESCE(p.read_key, ''), s.email`,
		token, SubscriberUnsubscribed).Scan(&out.Title, &out.ReadKey, &out.Email)
	if errors.Is(err, pgx.ErrNoRows) {
		return out, ErrInvalidToken
	}
	return out, err
}

// ListActiveSubscribers returns rows for notification fan-out.
func (s *Store) ListActiveSubscribers(ctx context.Context, pageID int64) ([]UpdateSubscriber, error) {
	rows, err := s.db.Query(ctx,
		`SELECT id, email FROM page_update_subscribers WHERE page_id = $1 AND status = $2`,
		pageID, SubscriberActive)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []UpdateSubscriber
	for rows.Next() {
		var sub UpdateSubscriber
		if err := rows.Scan(&sub.ID, &sub.Email); err != nil {
			return nil, err
		}
		out = append(out, sub)
	}
	return out, rows.Err()
}

// EnqueueUpdateNotifications queues one outbox row per active subscriber.
func (s *Store) EnqueueUpdateNotifications(ctx context.Context, updateID int64, pageID int64, readKey string) (int, error) {
	subs, err := s.ListActiveSubscribers(ctx, pageID)
	if err != nil {
		return 0, err
	}
	n := 0
	for _, sub := range subs {
		tag, err := s.db.Exec(ctx,
			`INSERT INTO notification_outbox (page_update_id, subscriber_id, read_key, send_after)
			 VALUES ($1, $2, $3, now()) ON CONFLICT DO NOTHING`,
			updateID, sub.ID, readKey)
		if err != nil {
			return n, err
		}
		n += int(tag.RowsAffected())
	}
	return n, nil
}

// OutboxItem is a pending notification email.
type OutboxItem struct {
	ID               int64
	UpdateID         int64
	Email            string
	PageTitle        string
	ReadKey          string
	UpdateTitle      string
	UpdateBody       string
	UnsubscribeToken string
}

// ClaimOutboxBatch fetches pending rows for sending.
func (s *Store) ClaimOutboxBatch(ctx context.Context, limit int) ([]OutboxItem, error) {
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	rows, err := tx.Query(ctx,
		`SELECT o.id, o.page_update_id, s.email, p.title, COALESCE(NULLIF(o.read_key, ''), p.read_key, ''),
		        u.title, u.body, s.unsubscribe_token
		 FROM notification_outbox o
		 JOIN page_update_subscribers s ON s.id = o.subscriber_id AND s.status = $1
		 JOIN page_updates u ON u.id = o.page_update_id
		 JOIN pages p ON p.id = u.page_id
		 WHERE o.sent_at IS NULL AND o.send_after <= now() AND o.attempts < 5
		 ORDER BY o.send_after, o.id
		 LIMIT $2
		 FOR UPDATE OF o SKIP LOCKED`, SubscriberActive, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []int64
	var items []OutboxItem
	for rows.Next() {
		var it OutboxItem
		if err := rows.Scan(&it.ID, &it.UpdateID, &it.Email, &it.PageTitle, &it.ReadKey,
			&it.UpdateTitle, &it.UpdateBody, &it.UnsubscribeToken); err != nil {
			return nil, err
		}
		ids = append(ids, it.ID)
		items = append(items, it)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(ids) > 0 {
		_, err = tx.Exec(ctx, `UPDATE notification_outbox SET attempts = attempts + 1 WHERE id = ANY($1)`, ids)
		if err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return items, nil
}

func (s *Store) MarkOutboxSent(ctx context.Context, id int64) error {
	_, err := s.db.Exec(ctx, `UPDATE notification_outbox SET sent_at = now(), last_error = '' WHERE id = $1`, id)
	return err
}

func (s *Store) MarkOutboxFailed(ctx context.Context, id int64, msg string) error {
	if len(msg) > 500 {
		msg = msg[:500]
	}
	_, err := s.db.Exec(ctx, `UPDATE notification_outbox SET last_error = $2 WHERE id = $1`, id, msg)
	return err
}
