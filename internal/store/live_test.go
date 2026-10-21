package store

import (
	"context"
	"testing"
	"time"
)

// Open pages reload on page_changed, so update changes must send it even
// though they don't touch the title, format or content the trigger watches.
func TestUpdateChangesNotifyLivePages(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	st := testStore(t)
	a, err := st.Create(ctx, "News", "markdown", "# hi", false)
	if err != nil {
		t.Fatal(err)
	}
	changes := make(chan PageChange, 16)
	go st.ListenPageChanges(ctx, func(c PageChange) {
		if c.ID == a.ID {
			changes <- c
		}
	}, nil)
	// Give the listener time to connect before making changes.
	time.Sleep(300 * time.Millisecond)

	expect := func(what string) {
		t.Helper()
		select {
		case c := <-changes:
			if c.Deleted || c.UpdatedAt.IsZero() {
				t.Errorf("%s: bad change %+v", what, c)
			}
		case <-time.After(3 * time.Second):
			t.Fatalf("%s: no page_changed notification", what)
		}
	}

	if err := st.SetUpdatesEnabled(ctx, a.ID, true); err != nil {
		t.Fatal(err)
	}
	expect("enable updates")

	if _, err := st.CreatePageUpdate(ctx, a.ID, nil, "Now", "b", a.ReadKey, time.Time{}); err != nil {
		t.Fatal(err)
	}
	expect("post update")

	later, err := st.CreatePageUpdate(ctx, a.ID, nil, "Later", "b", a.ReadKey, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	select {
	case c := <-changes:
		t.Fatalf("a scheduled update must not reload the page before it publishes: %+v", c)
	case <-time.After(300 * time.Millisecond):
	}
	if _, err := st.db.Exec(ctx, `UPDATE page_updates SET published_at = now() WHERE id = $1`, later.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := st.ClaimDueUpdates(ctx, 10); err != nil {
		t.Fatal(err)
	}
	expect("scheduled update published")
}
