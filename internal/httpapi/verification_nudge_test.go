package httpapi

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stewartmckee/canvasbin/internal/config"
	"github.com/stewartmckee/canvasbin/internal/store"
)

func TestSendVerificationNudges(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	st, err := store.Open(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(st.Close)

	m := &fakeMailer{links: map[string]string{}}
	cfg := config.Config{
		Accounts:         true,
		PublicBaseURL:    "https://canvasbin.test",
		VerifyNudge:      true,
		VerifyNudgeAfter: time.Hour,
		MailgunKey:       "x",
	}
	s := &Server{Store: st, Config: cfg, Mailer: m, PublicBaseURL: cfg.PublicBaseURL}

	email := "nudge-worker-" + time.Now().Format("20060102150405") + "@example.com"
	u, err := st.CreateUser(ctx, email, "hash", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.TouchUserCreatedAt(ctx, u.ID, time.Now().Add(-48*time.Hour)); err != nil {
		t.Fatal(err)
	}

	s.sendVerificationNudges(ctx, time.Hour)
	if m.get("verify_nudge", email) == "" {
		t.Fatal("expected nudge email")
	}
}
