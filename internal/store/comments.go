package store

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
)

const MaxCommentRunes = 4000

var ErrBadComment = errors.New("comment is empty")

// Comment is a note left on a team page by a member.
type Comment struct {
	ID          int64     `json:"id"`
	UserID      *int64    `json:"-"`
	AuthorEmail string    `json:"author_email"`
	AuthorName  string    `json:"author_name"`
	Body        string    `json:"body"`
	CreatedAt   time.Time `json:"created_at"`
	// CanDelete is set per viewer: authors and team admins may delete.
	CanDelete bool `json:"can_delete"`
}

func cleanComment(b string) (string, error) {
	b = strings.TrimSpace(b)
	if b == "" {
		return "", ErrBadComment
	}
	if utf8.RuneCountInString(b) > MaxCommentRunes {
		b = string([]rune(b)[:MaxCommentRunes])
	}
	return b, nil
}

func (s *Store) AddComment(ctx context.Context, pageID, userID int64, body string) (*Comment, error) {
	body, err := cleanComment(body)
	if err != nil {
		return nil, err
	}
	var c Comment
	err = s.db.QueryRow(ctx,
		`WITH ins AS (
		   INSERT INTO page_comments (page_id, user_id, body) VALUES ($1, $2, $3)
		   RETURNING id, user_id, body, created_at)
		 SELECT ins.id, ins.user_id, u.email, u.display_name, ins.body, ins.created_at
		 FROM ins JOIN users u ON u.id = ins.user_id`, pageID, userID, body,
	).Scan(&c.ID, &c.UserID, &c.AuthorEmail, &c.AuthorName, &c.Body, &c.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// ListComments returns a page's comments, oldest first.
func (s *Store) ListComments(ctx context.Context, pageID int64) ([]Comment, error) {
	rows, err := s.db.Query(ctx,
		`SELECT c.id, c.user_id, COALESCE(u.email, ''), COALESCE(u.display_name, ''), c.body, c.created_at
		 FROM page_comments c LEFT JOIN users u ON u.id = c.user_id
		 WHERE c.page_id = $1 ORDER BY c.created_at, c.id`, pageID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Comment{}
	for rows.Next() {
		var c Comment
		if err := rows.Scan(&c.ID, &c.UserID, &c.AuthorEmail, &c.AuthorName, &c.Body, &c.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// GetComment returns one comment on a page.
func (s *Store) GetComment(ctx context.Context, pageID, commentID int64) (*Comment, error) {
	var c Comment
	err := s.db.QueryRow(ctx,
		`SELECT c.id, c.user_id, COALESCE(u.email, ''), COALESCE(u.display_name, ''), c.body, c.created_at
		 FROM page_comments c LEFT JOIN users u ON u.id = c.user_id
		 WHERE c.page_id = $1 AND c.id = $2`, pageID, commentID,
	).Scan(&c.ID, &c.UserID, &c.AuthorEmail, &c.AuthorName, &c.Body, &c.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (s *Store) DeleteComment(ctx context.Context, pageID, commentID int64) error {
	tag, err := s.db.Exec(ctx, `DELETE FROM page_comments WHERE page_id = $1 AND id = $2`, pageID, commentID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
