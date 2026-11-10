package store

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestClaimUnverifiedForNudge(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	st, err := Open(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(st.Close)

	email := "nudge-" + time.Now().Format("20060102150405") + "@example.com"
	u, err := st.CreateUser(ctx, email, "hash", "")
	if err != nil {
		t.Fatal(err)
	}
	claimed, err := st.ClaimUnverifiedForNudge(ctx, time.Minute, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(claimed) != 0 {
		t.Fatalf("expected no claims before min age, got %d", len(claimed))
	}
	if err := st.TouchUserCreatedAt(ctx, u.ID, time.Now().Add(-48*time.Hour)); err != nil {
		t.Fatal(err)
	}
	claimed, err = st.ClaimUnverifiedForNudge(ctx, 24*time.Hour, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(claimed) != 1 || claimed[0].ID != u.ID {
		t.Fatalf("claim: %+v", claimed)
	}
	again, err := st.ClaimUnverifiedForNudge(ctx, 24*time.Hour, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(again) != 0 {
		t.Fatalf("expected single nudge only, got %d", len(again))
	}
}
