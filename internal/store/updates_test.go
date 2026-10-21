package store

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestPageUpdatesFlow(t *testing.T) {
	ctx := context.Background()
	st := testStore(t)
	a, err := st.Create(ctx, "News", "markdown", "# hi", false)
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
	if _, err := st.ConfirmSubscriber(ctx, tok); err != nil {
		t.Fatal(err)
	}
	u, err := st.CreatePageUpdate(ctx, a.ID, nil, "Hello", "Body text", a.ReadKey, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	if u.Scheduled {
		t.Fatal("an update without publish_at should publish now")
	}
	n, err := st.EnqueueUpdateNotifications(ctx, u.ID, a.ID, a.ReadKey)
	if err != nil || n != 1 {
		t.Fatalf("enqueue: n=%d err=%v", n, err)
	}
	list, err := st.ListPageUpdates(ctx, a.ID, 10)
	if err != nil || len(list) != 1 || list[0].Title != "Hello" {
		t.Fatalf("list: %v err=%v", list, err)
	}
	if due, _ := st.ClaimDueUpdates(ctx, 10); len(due) != 0 {
		t.Fatalf("an immediate update must not be handed over again: %v", due)
	}
}

func TestScheduledPageUpdates(t *testing.T) {
	ctx := context.Background()
	st := testStore(t)
	a, err := st.Create(ctx, "News", "markdown", "# hi", false)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.SetUpdatesEnabled(ctx, a.ID, true); err != nil {
		t.Fatal(err)
	}

	for _, at := range []time.Time{time.Now().Add(-time.Hour), time.Now().Add(2 * MaxUpdateSchedule)} {
		if _, err := st.CreatePageUpdate(ctx, a.ID, nil, "Bad", "", a.ReadKey, at); !errors.Is(err, ErrBadSchedule) {
			t.Errorf("publish at %v: err = %v, want ErrBadSchedule", at, err)
		}
	}

	later, err := st.CreatePageUpdate(ctx, a.ID, nil, "Later", "b", a.ReadKey, time.Now().Add(time.Hour))
	if err != nil || !later.Scheduled {
		t.Fatalf("schedule: %+v err=%v", later, err)
	}
	cancel, err := st.CreatePageUpdate(ctx, a.ID, nil, "Cancel me", "b", a.ReadKey, time.Now().Add(2*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if list, _ := st.ListPageUpdates(ctx, a.ID, 10); len(list) != 0 {
		t.Fatalf("scheduled updates must stay hidden: %v", list)
	}
	if sched, _ := st.ListScheduledUpdates(ctx, a.ID); len(sched) != 2 || sched[0].ID != later.ID {
		t.Fatalf("scheduled list: %v", sched)
	}
	if due, _ := st.ClaimDueUpdates(ctx, 10); len(due) != 0 {
		t.Fatalf("nothing is due yet: %v", due)
	}

	if err := st.CancelScheduledUpdate(ctx, a.ID, cancel.ID); err != nil {
		t.Fatal(err)
	}
	if err := st.CancelScheduledUpdate(ctx, a.ID, cancel.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("second cancel: %v", err)
	}

	if _, err := st.db.Exec(ctx, `UPDATE page_updates SET published_at = now() - interval '1 second' WHERE id = $1`, later.ID); err != nil {
		t.Fatal(err)
	}
	due, err := st.ClaimDueUpdates(ctx, 10)
	if err != nil || len(due) != 1 || due[0].ID != later.ID || due[0].ReadKey != a.ReadKey {
		t.Fatalf("claim: %+v err=%v", due, err)
	}
	if due, _ := st.ClaimDueUpdates(ctx, 10); len(due) != 0 {
		t.Fatalf("claimed twice: %v", due)
	}
	if err := st.CancelScheduledUpdate(ctx, a.ID, later.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("published updates can't be cancelled: %v", err)
	}
	if list, _ := st.ListPageUpdates(ctx, a.ID, 10); len(list) != 1 || list[0].Scheduled {
		t.Fatalf("published list: %v", list)
	}
}
