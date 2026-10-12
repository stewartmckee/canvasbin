package store

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/stewartmckee/canvasbin/internal/keys"
)

//go:embed schema.sql
var schema string

const (
	FormatHTML     = "html"
	FormatMarkdown = "markdown"

	MaxContentBytes = 1 << 20
	MaxTitleRunes   = 200
)

var (
	ErrNotFound  = errors.New("page not found")
	ErrReadOnly  = errors.New("this key is read-only")
	ErrTooLarge  = fmt.Errorf("content exceeds %d bytes", MaxContentBytes)
	ErrBadFormat = errors.New(`format must be "html" or "markdown"`)
)

type Page struct {
	Title     string    `json:"title"`
	Format    string    `json:"format"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Access is a page as seen through a particular key.
type Access struct {
	Page
	CanWrite bool
	ReadKey  string
	WriteKey string // empty unless the key used was the write key
}

// Changes holds optional fields for an update.
type Changes struct {
	Title   *string
	Format  *string
	Content *string
}

type Store struct {
	db *pgxpool.Pool
}

func Open(ctx context.Context, databaseURL string) (*Store, error) {
	db, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, err
	}
	var lastErr error
	for i := 0; i < 30; i++ {
		if lastErr = db.Ping(ctx); lastErr == nil {
			break
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(time.Second):
		}
	}
	if lastErr != nil {
		db.Close()
		return nil, fmt.Errorf("connect to postgres: %w", lastErr)
	}
	if _, err := db.Exec(ctx, schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("apply schema: %w", err)
	}
	return &Store{db: db}, nil
}

func (s *Store) Close() { s.db.Close() }

func (s *Store) Ping(ctx context.Context) error { return s.db.Ping(ctx) }

// NormalizeFormat maps an empty format to html and rejects anything else unknown.
func NormalizeFormat(f string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(f)) {
	case "", FormatHTML:
		return FormatHTML, nil
	case FormatMarkdown, "md":
		return FormatMarkdown, nil
	}
	return "", ErrBadFormat
}

func cleanTitle(t string) string {
	t = strings.TrimSpace(t)
	if utf8.RuneCountInString(t) > MaxTitleRunes {
		t = string([]rune(t)[:MaxTitleRunes])
	}
	return t
}

func (s *Store) Create(ctx context.Context, title, format, content string) (*Access, error) {
	format, err := NormalizeFormat(format)
	if err != nil {
		return nil, err
	}
	if len(content) > MaxContentBytes {
		return nil, ErrTooLarge
	}
	writeKey, err := keys.NewWrite()
	if err != nil {
		return nil, err
	}
	readKey := keys.ReadFromWrite(writeKey)
	a := &Access{CanWrite: true, ReadKey: readKey, WriteKey: writeKey}
	a.Title, a.Format, a.Content = cleanTitle(title), format, content
	err = s.db.QueryRow(ctx,
		`INSERT INTO pages (title, format, content, read_key_hash, write_key_hash)
		 VALUES ($1, $2, $3, $4, $5) RETURNING created_at, updated_at`,
		a.Title, a.Format, a.Content, keys.Hash(readKey), keys.Hash(writeKey),
	).Scan(&a.CreatedAt, &a.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return a, nil
}

// Get looks a page up by either its read key or its write key.
func (s *Store) Get(ctx context.Context, key string) (*Access, error) {
	h := keys.Hash(key)
	var a Access
	err := s.db.QueryRow(ctx,
		`SELECT title, format, content, created_at, updated_at, write_key_hash = $1
		 FROM pages WHERE read_key_hash = $1 OR write_key_hash = $1`, h,
	).Scan(&a.Title, &a.Format, &a.Content, &a.CreatedAt, &a.UpdatedAt, &a.CanWrite)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if a.CanWrite {
		a.WriteKey = key
		a.ReadKey = keys.ReadFromWrite(key)
	} else {
		a.ReadKey = key
	}
	return &a, nil
}

func (s *Store) Update(ctx context.Context, writeKey string, c Changes) (*Access, error) {
	if c.Format != nil {
		f, err := NormalizeFormat(*c.Format)
		if err != nil {
			return nil, err
		}
		c.Format = &f
	}
	if c.Content != nil && len(*c.Content) > MaxContentBytes {
		return nil, ErrTooLarge
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
		 WHERE write_key_hash = $1`,
		keys.Hash(writeKey), c.Title, c.Format, c.Content)
	if err != nil {
		return nil, err
	}
	if tag.RowsAffected() == 0 {
		return nil, s.missingOrReadOnly(ctx, writeKey)
	}
	return s.Get(ctx, writeKey)
}

func (s *Store) Delete(ctx context.Context, writeKey string) error {
	tag, err := s.db.Exec(ctx, `DELETE FROM pages WHERE write_key_hash = $1`, keys.Hash(writeKey))
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return s.missingOrReadOnly(ctx, writeKey)
	}
	return nil
}

func (s *Store) missingOrReadOnly(ctx context.Context, key string) error {
	var exists bool
	if err := s.db.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM pages WHERE read_key_hash = $1)`, keys.Hash(key),
	).Scan(&exists); err != nil {
		return err
	}
	if exists {
		return ErrReadOnly
	}
	return ErrNotFound
}
