package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
)

// PublishMismatch means expected_bytes is not the stored draft's byte length.
// The live page is left unchanged.
type PublishMismatch struct {
	DraftBytes    int
	ExpectedBytes int
	LiveBytes     int
}

func (e *PublishMismatch) Error() string {
	return fmt.Sprintf("expected_bytes %d does not match the stored draft (%d bytes); nothing was published", e.ExpectedBytes, e.DraftBytes)
}

// PublishShrink means the draft is under half the live page and confirm_shrink
// was not set. The live page is left unchanged.
type PublishShrink struct {
	DraftBytes int
	LiveBytes  int
}

func (e *PublishShrink) Error() string {
	return fmt.Sprintf("draft is %d bytes, under half of the live page (%d bytes); pass confirm_shrink to publish a shorter page", e.DraftBytes, e.LiveBytes)
}

// Draft is one unpublished replacement for a page. A later save replaces it.
type Draft struct {
	ID          int64
	Content     string
	Title       *string
	Format      *string
	AgenticOnly *bool
	DraftBytes  int
	LiveBytes   int
	ReadKey     string
}

func checkPublishLength(live, next string, expected *int, confirm bool) error {
	nextN, liveN := len(next), len(live)
	got := 0
	if expected != nil {
		got = *expected
	}
	if expected == nil || got != nextN {
		return &PublishMismatch{DraftBytes: nextN, ExpectedBytes: got, LiveBytes: liveN}
	}
	if liveN > 0 && nextN*2 < liveN && !confirm {
		return &PublishShrink{DraftBytes: nextN, LiveBytes: liveN}
	}
	return nil
}

// SaveDraft stores one draft for a page the write key can edit. The live page,
// its URL and its updated_at are not changed.
func (s *Store) SaveDraft(ctx context.Context, writeKey string, title, format, content *string, agentic *bool) (*Draft, error) {
	a, err := s.Get(ctx, writeKey)
	if err != nil {
		return nil, err
	}
	if !a.CanWrite {
		return nil, ErrReadOnly
	}
	return s.saveDraft(ctx, a, title, format, content, agentic)
}

// SaveDraftForUser stores one draft when the user may write the page.
func (s *Store) SaveDraftForUser(ctx context.Context, key string, userID int64, title, format, content *string, agentic *bool) (*Draft, error) {
	a, err := s.GetForUser(ctx, key, userID)
	if err != nil {
		return nil, err
	}
	if !a.CanWrite {
		return nil, ErrReadOnly
	}
	return s.saveDraft(ctx, a, title, format, content, agentic)
}

func (s *Store) saveDraft(ctx context.Context, a *Access, title, format, content *string, agentic *bool) (*Draft, error) {
	if format != nil {
		f, err := NormalizeFormat(*format)
		if err != nil {
			return nil, err
		}
		format = &f
	}
	if title != nil {
		t := cleanTitle(*title)
		title = &t
	}
	body := a.Content
	if content != nil {
		body = *content
	}
	if len(body) > MaxContentBytes {
		return nil, ErrTooLarge
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `DELETE FROM page_drafts WHERE page_id = $1`, a.ID); err != nil {
		return nil, err
	}
	var id int64
	err = tx.QueryRow(ctx,
		`INSERT INTO page_drafts (page_id, title, format, content, agentic_only)
		 VALUES ($1, $2, $3, $4, $5) RETURNING id`,
		a.ID, title, format, body, agentic,
	).Scan(&id)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &Draft{
		ID: id, Content: body, Title: title, Format: format, AgenticOnly: agentic,
		DraftBytes: len(body), LiveBytes: len(a.Content), ReadKey: a.ReadKey,
	}, nil
}

// PublishDraft copies the draft onto the live page when expected matches the
// stored draft's byte length. A mismatch or an unconfirmed shrink publishes
// nothing.
func (s *Store) PublishDraft(ctx context.Context, writeKey string, draftID int64, expected int, confirm bool) (*Access, error) {
	a, err := s.Get(ctx, writeKey)
	if err != nil {
		return nil, err
	}
	if !a.CanWrite {
		return nil, ErrReadOnly
	}
	if err := s.applyDraft(ctx, a.ID, draftID, expected, confirm); err != nil {
		return nil, err
	}
	return s.Get(ctx, writeKey)
}

// PublishDraftForUser is PublishDraft for a signed-in user who may write the page.
func (s *Store) PublishDraftForUser(ctx context.Context, key string, userID, draftID int64, expected int, confirm bool) (*Access, error) {
	a, err := s.GetForUser(ctx, key, userID)
	if err != nil {
		return nil, err
	}
	if !a.CanWrite {
		return nil, ErrReadOnly
	}
	if err := s.applyDraft(ctx, a.ID, draftID, expected, confirm); err != nil {
		return nil, err
	}
	return s.GetForUser(ctx, key, userID)
}

func (s *Store) applyDraft(ctx context.Context, pageID, draftID int64, expected int, confirm bool) error {
	var d Draft
	var title, format sql.NullString
	var agentic sql.NullBool
	err := s.db.QueryRow(ctx,
		`SELECT id, title, format, content, agentic_only FROM page_drafts WHERE page_id = $1 AND id = $2`,
		pageID, draftID,
	).Scan(&d.ID, &title, &format, &d.Content, &agentic)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrDraftNotFound
	}
	if err != nil {
		return err
	}
	if title.Valid {
		d.Title = &title.String
	}
	if format.Valid {
		d.Format = &format.String
	}
	if agentic.Valid {
		d.AgenticOnly = &agentic.Bool
	}
	expectedBytes := expected
	if err := s.UpdateByID(ctx, pageID, Changes{
		Title: d.Title, Format: d.Format, Content: &d.Content, AgenticOnly: d.AgenticOnly,
		ExpectedBytes: &expectedBytes, ConfirmShrink: confirm,
	}); err != nil {
		return err
	}
	_, err = s.db.Exec(ctx, `DELETE FROM page_drafts WHERE id = $1 AND page_id = $2`, draftID, pageID)
	return err
}

// LastNonEmptyLine is the last line of source that is not only whitespace.
func LastNonEmptyLine(s string) string {
	lines := strings.Split(s, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line != "" {
			return line
		}
	}
	return ""
}
