package config

import (
	"errors"
	"strings"
	"testing"

	"github.com/stewartmckee/canvasbin/internal/store"
)

func TestLimitErrors(t *testing.T) {
	const base = "https://canvasbin.dev"
	billing := Config{FreeQuota: 5, ProQuota: 100, StripeSecretKey: "x", StripePricePro: "p",
		StripePriceTeamBase: "b", StripePriceTeamSeat: "s"}
	free, pro := &store.User{}, &store.User{SubscriptionStatus: "active"}

	cases := []struct {
		name string
		err  error
		is   error
		want []string
		not  []string
	}{
		{"free with billing", billing.QuotaError(free, base), store.ErrQuotaExceeded,
			[]string{"5 pages on the Free plan", "Upgrade to Individual", ProPrice, "100 pages", base + "/pricing", "delete a page"}, nil},
		{"free without billing", Config{}.QuotaError(free, base), store.ErrQuotaExceeded,
			[]string{"5 pages on the Free plan", "Delete a page"}, []string{"Upgrade", "/pricing"}},
		{"individual", billing.QuotaError(pro, base), store.ErrQuotaExceeded,
			[]string{"100 pages on the Individual plan", "start a Team", TeamPrice, base + "/pricing"}, nil},
		{"team full", billing.TeamQuotaError(), store.ErrQuotaExceeded,
			[]string{"this team's pages", "support@canvasbin.dev"}, nil},
		{"team inactive", TeamInactiveError(base, 7), store.ErrTeamInactive,
			[]string{base + "/teams/7"}, nil},
		{"unverified", UnverifiedError(base), store.ErrUnverified,
			[]string{"verification email", base + "/account"}, nil},
	}
	for _, c := range cases {
		if !errors.Is(c.err, c.is) {
			t.Errorf("%s: %v does not wrap %v", c.name, c.err, c.is)
		}
		for _, w := range c.want {
			if !strings.Contains(c.err.Error(), w) {
				t.Errorf("%s: %q missing %q", c.name, c.err, w)
			}
		}
		for _, w := range c.not {
			if strings.Contains(c.err.Error(), w) {
				t.Errorf("%s: %q should not mention %q", c.name, c.err, w)
			}
		}
	}
}
