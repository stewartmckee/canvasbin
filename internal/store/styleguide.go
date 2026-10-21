package store

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
)

// MaxStyleGuideRunes is the longest style guide a team may store.
const MaxStyleGuideRunes = 8000

// ErrStyleGuideTooLong is returned when a style guide exceeds MaxStyleGuideRunes.
var ErrStyleGuideTooLong = errors.New("style guide is too long")

// cleanStyleGuide trims a style guide and rejects one that is too long.
// An empty result clears the guide. Internal line breaks are kept.
func cleanStyleGuide(s string) (string, error) {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	if utf8.RuneCountInString(s) > MaxStyleGuideRunes {
		return "", fmt.Errorf("%w (maximum %d characters)", ErrStyleGuideTooLong, MaxStyleGuideRunes)
	}
	return s, nil
}

// GetStyleGuide returns a team's style guide. An empty string means none is set.
func (s *Store) GetStyleGuide(ctx context.Context, teamID int64) (string, error) {
	var guide string
	err := s.db.QueryRow(ctx, `SELECT style_guide FROM teams WHERE id = $1`, teamID).Scan(&guide)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	return guide, err
}

// SetStyleGuide replaces a team's style guide and returns the stored text.
func (s *Store) SetStyleGuide(ctx context.Context, teamID int64, guide string) (string, error) {
	guide, err := cleanStyleGuide(guide)
	if err != nil {
		return "", err
	}
	tag, err := s.db.Exec(ctx,
		`UPDATE teams SET style_guide = $2, updated_at = now() WHERE id = $1`, teamID, guide)
	if err != nil {
		return "", err
	}
	if tag.RowsAffected() == 0 {
		return "", ErrNotFound
	}
	return guide, nil
}
