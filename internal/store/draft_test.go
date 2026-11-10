package store

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestCheckPublishLength(t *testing.T) {
	n := func(v int) *int { return &v }
	live := "0123456789" // 10 bytes
	short := "abc"       // 3, under half
	if err := checkPublishLength(live, short, n(3), false); !errors.As(err, new(*PublishShrink)) {
		t.Fatalf("shrink: %v", err)
	}
	if err := checkPublishLength(live, short, n(3), true); err != nil {
		t.Fatalf("confirmed shrink: %v", err)
	}
	if err := checkPublishLength(live, short, n(2), true); !errors.As(err, new(*PublishMismatch)) {
		t.Fatalf("mismatch: %v", err)
	}
	if err := checkPublishLength(live, "0123456789", n(10), false); err != nil {
		t.Fatalf("same length: %v", err)
	}
	// Exactly half is not under half.
	if err := checkPublishLength(live, "01234", n(5), false); err != nil {
		t.Fatalf("half: %v", err)
	}
}

func TestDraftPublishLengths(t *testing.T) {
	ctx := context.Background()
	st := testStore(t)
	live := "0123456789"
	a, err := st.Create(ctx, "T", "html", live, false)
	if err != nil {
		t.Fatal(err)
	}
	before := a.UpdatedAt

	short := "abc"
	d, err := st.SaveDraft(ctx, a.WriteKey, nil, nil, &short, nil)
	if err != nil {
		t.Fatal(err)
	}
	if d.DraftBytes != len(short) || d.LiveBytes != len(live) {
		t.Fatalf("lengths: draft %d live %d", d.DraftBytes, d.LiveBytes)
	}
	got, err := st.Get(ctx, a.WriteKey)
	if err != nil {
		t.Fatal(err)
	}
	if got.Content != live || !got.UpdatedAt.Equal(before) {
		t.Fatalf("draft changed the live page: content %q updated %s want %s", got.Content, got.UpdatedAt, before)
	}

	// A second draft replaces the first.
	longer := short + "defghi" // 9 bytes, not under half of 10
	d2, err := st.SaveDraft(ctx, a.WriteKey, nil, nil, &longer, nil)
	if err != nil {
		t.Fatal(err)
	}
	if d2.ID == d.ID {
		t.Fatal("replacement draft kept the old id")
	}
	if _, err := st.PublishDraft(ctx, a.WriteKey, d.ID, len(longer), false); !errors.Is(err, ErrDraftNotFound) {
		t.Fatalf("old draft id: %v", err)
	}

	var mismatch *PublishMismatch
	if _, err := st.PublishDraft(ctx, a.WriteKey, d2.ID, len(longer)-1, false); !errors.As(err, &mismatch) {
		t.Fatalf("unequal publish: %v", err)
	} else if mismatch.DraftBytes != len(longer) || mismatch.ExpectedBytes != len(longer)-1 {
		t.Fatalf("mismatch numbers: %+v", mismatch)
	}
	got, _ = st.Get(ctx, a.WriteKey)
	if got.Content != live || !got.UpdatedAt.Equal(before) {
		t.Fatalf("unequal publish wrote the page: %q %s", got.Content, got.UpdatedAt)
	}

	if _, err := st.PublishDraft(ctx, a.WriteKey, d2.ID, len(longer), false); err != nil {
		t.Fatal(err)
	}
	got, _ = st.Get(ctx, a.WriteKey)
	if got.Content != longer || !got.UpdatedAt.After(before) {
		t.Fatalf("equal publish: %q %s", got.Content, got.UpdatedAt)
	}

	// Under half of the new live page.
	stub := "no"
	d3, err := st.SaveDraft(ctx, a.WriteKey, nil, nil, &stub, nil)
	if err != nil {
		t.Fatal(err)
	}
	var shrink *PublishShrink
	if _, err := st.PublishDraft(ctx, a.WriteKey, d3.ID, len(stub), false); !errors.As(err, &shrink) {
		t.Fatalf("shrink: %v", err)
	} else if shrink.DraftBytes != len(stub) || shrink.LiveBytes != len(longer) {
		t.Fatalf("shrink numbers: %+v", shrink)
	}
	got, _ = st.Get(ctx, a.WriteKey)
	if got.Content != longer {
		t.Fatalf("refused shrink wrote %q", got.Content)
	}
	published, err := st.PublishDraft(ctx, a.WriteKey, d3.ID, len(stub), true)
	if err != nil || published == nil || published.Content != stub {
		t.Fatalf("confirmed shrink: %v %v", published, err)
	}
}

func TestDraftDoesNotMoveUpdatedAt(t *testing.T) {
	ctx := context.Background()
	st := testStore(t)
	a, err := st.Create(ctx, "T", "html", "live page", false)
	if err != nil {
		t.Fatal(err)
	}
	// Give the clock a moment so a mistaken updated_at = now() cannot tie.
	time.Sleep(20 * time.Millisecond)
	body := "shorter"
	if _, err := st.SaveDraft(ctx, a.WriteKey, nil, nil, &body, nil); err != nil {
		t.Fatal(err)
	}
	got, err := st.Get(ctx, a.ReadKey)
	if err != nil {
		t.Fatal(err)
	}
	if !got.UpdatedAt.Equal(a.UpdatedAt) || got.Content != a.Content {
		t.Fatalf("live page moved: content %q updated %s", got.Content, got.UpdatedAt)
	}
}
