package store

import (
	"context"
	"os"
	"testing"
)

func testStore(t *testing.T) *Store {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	st, err := Open(context.Background(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(st.Close)
	return st
}

func TestPageUpdatesFlow(t *testing.T) {
	ctx := context.Background()
	st := testStore(t)
	a, err := st.Create(ctx, "News", "markdown", "# hi")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.SetUpdatesEnabled(ctx, a.ID, true); err != nil {
		t.Fatal(err)
	}
	tok, err := st.SubscribePage(ctx, a.ID, "sub@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.ConfirmSubscriber(ctx, tok); err != nil {
		t.Fatal(err)
	}
	u, err := st.CreatePageUpdate(ctx, a.ID, nil, "Hello", "Body text")
	if err != nil {
		t.Fatal(err)
	}
	n, err := st.EnqueueUpdateNotifications(ctx, u.ID, a.ID, a.ReadKey)
	if err != nil || n != 1 {
		t.Fatalf("enqueue: n=%d err=%v", n, err)
	}
	list, err := st.ListPageUpdates(ctx, a.ID, 10)
	if err != nil || len(list) != 1 || list[0].Title != "Hello" {
		t.Fatalf("list: %v err=%v", list, err)
	}
}
