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

	// MaxUpdateSchedule is how far ahead an update can be scheduled.
	MaxUpdateSchedule = 366 * 24 * time.Hour
	// A publish time this close to now (or slightly past it) means "now".
	updateScheduleSlack = time.Minute
)

var (
	ErrUpdatesDisabled   = errors.New("updates are not enabled for this page")
	ErrUpdatesOff        = errors.New("updates feature is not available")
	ErrAlreadySubscribed = errors.New("already subscribed")
	ErrBadSchedule       = errors.New("publish time must be in the future and within a year")
)

// PageUpdate is one append-only post on a page.
type PageUpdate struct {
	ID          int64     `json:"id"`
	PageID      int64     `json:"-"`
	Title       string    `json:"title"`
	Body        string    `json:"body"`
	CreatedAt   time.Time `json:"created_at"`
	PublishedAt time.Time `json:"published_at"`
	Scheduled   bool      `json:"scheduled"`
}

const updateCols = `id, page_id, title, body, created_at, published_at, published_at > now()`

func scanUpdate(row pgx.Row, u *PageUpdate) error {
	return row.Scan(&u.ID, &u.PageID, &u.Title, &u.Body, &u.CreatedAt, &u.PublishedAt, &u.Scheduled)
}

// SetUpdatesEnabled toggles public updates for a page.
func (s *Store) SetUpdatesEnabled(ctx context.Context, pageID int64, on bool) error {
	tag, err := s.db.Exec(ctx,
		`WITH p AS (UPDATE pages SET updates_enabled = $2, updated_at = now() WHERE id = $1 RETURNING id, updated_at)
		 SELECT `+notifyPageSQL+` FROM p`, pageID, on)
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

// CreatePageUpdate inserts a post. A zero publishAt (or one within a minute of
// now) publishes immediately: the caller queues the emails and the page's
// updated_at is bumped for live reload. A later publishAt keeps the update
// hidden until then, and ClaimDueUpdates hands it over for emailing.
// readKey is kept so scheduled emails can link to pages that don't store it.
func (s *Store) CreatePageUpdate(ctx context.Context, pageID int64, userID *int64, title, body, readKey string, publishAt time.Time) (*PageUpdate, error) {
	title, body, err := cleanUpdate(title, body)
	if err != nil {
		return nil, err
	}
	scheduled := false
	if !publishAt.IsZero() {
		d := time.Until(publishAt)
		if d < -updateScheduleSlack || d > MaxUpdateSchedule {
			return nil, ErrBadSchedule
		}
		scheduled = d > updateScheduleSlack
	}
	on, err := s.UpdatesEnabled(ctx, pageID)
	if err != nil {
		return nil, err
	}
	if !on {
		return nil, ErrUpdatesDisabled
	}
	var u PageUpdate
	err = scanUpdate(s.db.QueryRow(ctx,
		`INSERT INTO page_updates (page_id, title, body, created_by_user_id, read_key, published_at, notified_at)
		 SELECT $1, $2, $3, $4, $5, t, CASE WHEN $7 THEN NULL ELSE now() END
		 FROM (SELECT CASE WHEN $7 THEN $6::timestamptz ELSE now() END AS t) x
		 RETURNING `+updateCols,
		pageID, title, body, userID, readKey, publishAt, scheduled,
	), &u)
	if err != nil {
		return nil, err
	}
	if !scheduled {
		_, _ = s.db.Exec(ctx,
			`WITH p AS (UPDATE pages SET updated_at = now() WHERE id = $1 RETURNING id, updated_at)
			 SELECT `+notifyPageSQL+` FROM p`, pageID)
	}
	return &u, nil
}

func (s *Store) listUpdates(ctx context.Context, q string, args ...any) ([]PageUpdate, error) {
	rows, err := s.db.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []PageUpdate
	for rows.Next() {
		var u PageUpdate
		if err := scanUpdate(rows, &u); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

// ListPageUpdates returns published posts newest first.
func (s *Store) ListPageUpdates(ctx context.Context, pageID int64, limit int) ([]PageUpdate, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	return s.listUpdates(ctx,
		`SELECT `+updateCols+` FROM page_updates WHERE page_id = $1 AND published_at <= now()
		 ORDER BY published_at DESC, id DESC LIMIT $2`, pageID, limit)
}

// ListScheduledUpdates returns posts waiting to be published, soonest first.
func (s *Store) ListScheduledUpdates(ctx context.Context, pageID int64) ([]PageUpdate, error) {
	return s.listUpdates(ctx,
		`SELECT `+updateCols+` FROM page_updates WHERE page_id = $1 AND published_at > now()
		 ORDER BY published_at, id LIMIT 100`, pageID)
}

// CancelScheduledUpdate deletes an update that hasn't been published yet.
func (s *Store) CancelScheduledUpdate(ctx context.Context, pageID, updateID int64) error {
	tag, err := s.db.Exec(ctx,
		`DELETE FROM page_updates WHERE id = $1 AND page_id = $2 AND published_at > now() AND notified_at IS NULL`,
		updateID, pageID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// DueUpdate is a scheduled update whose publish time has passed.
type DueUpdate struct {
	ID      int64
	PageID  int64
	ReadKey string
}

// ClaimDueUpdates marks published scheduled updates as notified and returns
// them so the caller can queue their emails. It also bumps each page's
// updated_at so open pages reload and show the update.
func (s *Store) ClaimDueUpdates(ctx context.Context, limit int) ([]DueUpdate, error) {
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	rows, err := s.db.Query(ctx,
		`WITH due AS (
		   SELECT id FROM page_updates
		   WHERE notified_at IS NULL AND published_at <= now()
		   ORDER BY published_at, id LIMIT $1
		   FOR UPDATE SKIP LOCKED
		 ), claimed AS (
		   UPDATE page_updates u SET notified_at = now()
		   FROM due WHERE u.id = due.id
		   RETURNING u.id, u.page_id, u.read_key
		 ), bumped AS (
		   UPDATE pages p SET updated_at = now()
		   WHERE p.id IN (SELECT page_id FROM claimed)
		   RETURNING p.id, COALESCE(p.read_key, '') AS read_key, `+notifyPageSQL+` AS notified
		 )
		 SELECT c.id, c.page_id, COALESCE(NULLIF(c.read_key, ''), b.read_key, '')
		 FROM claimed c LEFT JOIN bumped b ON b.id = c.page_id`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []DueUpdate
	for rows.Next() {
		var d DueUpdate
		if err := rows.Scan(&d.ID, &d.PageID, &d.ReadKey); err != nil {
			return nil, err
		}
		out = append(out, d)
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
