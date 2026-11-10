package httpapi

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/stewartmckee/canvasbin/internal/auth"
	"github.com/stewartmckee/canvasbin/internal/store"
)

// RunVerificationNudgeWorker emails stale unverified accounts once, with starter
// prompts and use-case links, until ctx is cancelled.
//
// Schedule: runs once at process start, then every hour. Each pass claims up to
// 20 users at a time where email is still unverified, the account is at least
// Config.VerifyNudgeAfter old (default 24h), and verify_nudge_sent_at is unset.
// Disable with VERIFY_NUDGE=off; tune delay with VERIFY_NUDGE_AFTER (e.g. 48h).
func (s *Server) RunVerificationNudgeWorker(ctx context.Context) {
	if !s.accounts() || !s.Config.VerifyNudge || s.Mailer == nil {
		return
	}
	minAge := s.Config.VerifyNudgeAfter
	if minAge <= 0 {
		minAge = 24 * time.Hour
	}
	t := time.NewTicker(time.Hour)
	defer t.Stop()
	s.sendVerificationNudges(ctx, minAge)
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			s.sendVerificationNudges(ctx, minAge)
		}
	}
}

func (s *Server) sendVerificationNudges(ctx context.Context, minAge time.Duration) {
	for {
		users, err := s.Store.ClaimUnverifiedForNudge(ctx, minAge, 20)
		if err != nil {
			slog.Error("claim verification nudges", "err", err)
			return
		}
		if len(users) == 0 {
			return
		}
		base := s.PublicBaseURL
		if base == "" {
			base = "https://canvasbin.dev"
		}
		for _, u := range users {
			if s.MailLimit != nil && !s.MailLimit.Allow(u.Email) {
				slog.Info("verification nudge rate limited", "user_id", u.ID)
				continue
			}
			link, err := s.issueVerifyLink(ctx, u.ID, base)
			if err != nil {
				slog.Error("verification nudge token", "err", err, "user_id", u.ID)
				continue
			}
			if err := s.Mailer.SendVerifyNudge(ctx, u.Email, link, base); err != nil {
				slog.Error("send verification nudge", "err", err, "user_id", u.ID)
			}
		}
	}
}

func (s *Server) issueVerifyLink(ctx context.Context, userID int64, base string) (string, error) {
	token, err := auth.NewToken()
	if err != nil {
		return "", err
	}
	if err := s.Store.CreateEmailToken(ctx, userID, store.PurposeVerify, auth.HashToken(token), time.Now().Add(verifyTokenTTL)); err != nil {
		return "", err
	}
	base = strings.TrimRight(strings.TrimSpace(base), "/")
	if base == "" {
		base = "https://canvasbin.dev"
	}
	return base + "/verify?token=" + token, nil
}
