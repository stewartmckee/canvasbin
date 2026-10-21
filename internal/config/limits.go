package config

import (
	"fmt"

	"github.com/stewartmckee/canvasbin/internal/store"
)

// Prices as shown on the pricing page; keep in step with web/src/site.js.
const (
	ProPrice  = "£2.99/month"
	TeamPrice = "£15/month plus £4 per member"
)

// The errors below wrap the store sentinels with what the user can do next.
// Agents relay them verbatim, so they name the plan, the limit and a link.

// QuotaError explains that u's personal pages are all in use.
func (c Config) QuotaError(u *store.User, base string) error {
	free, pro := c.PlanQuotas()
	if IsPro(u) {
		if c.TeamBillingEnabled() {
			return fmt.Errorf("%w: all %d pages on the Individual plan are in use. Delete a page that is no longer needed, or start a Team (%s, %d shared pages) at %s/pricing",
				store.ErrQuotaExceeded, pro, TeamPrice, c.TeamPageQuota(), base)
		}
		return fmt.Errorf("%w: all %d pages on the Individual plan are in use. Delete a page that is no longer needed",
			store.ErrQuotaExceeded, pro)
	}
	if c.BillingEnabled() {
		return fmt.Errorf("%w: all %d pages on the Free plan are in use. Upgrade to Individual (%s for %d pages, no Canvasbin badge) at %s/pricing, or delete a page that is no longer needed",
			store.ErrQuotaExceeded, free, ProPrice, pro, base)
	}
	return fmt.Errorf("%w: all %d pages on the Free plan are in use. Delete a page that is no longer needed",
		store.ErrQuotaExceeded, free)
}

// TeamQuotaError explains that a team's pages are all in use.
func (c Config) TeamQuotaError() error {
	return fmt.Errorf("%w: all %d of this team's pages are in use. Delete a team page that is no longer needed, or email support@canvasbin.dev for more room",
		store.ErrQuotaExceeded, c.TeamPageQuota())
}

// TeamInactiveError explains that a team can't take new pages until it pays.
func TeamInactiveError(base string, teamID int64) error {
	return fmt.Errorf("%w. The team owner can restart it at %s/teams/%d", store.ErrTeamInactive, base, teamID)
}

// StyleGuideInactiveError explains that style guides are a Team plan feature.
func StyleGuideInactiveError(base string, teamID int64) error {
	return fmt.Errorf("%w: a style guide is part of the Team plan. The team owner can subscribe at %s/teams/%d",
		store.ErrTeamInactive, base, teamID)
}

// UnverifiedError explains how to unlock page creation for a new account.
func UnverifiedError(base string) error {
	return fmt.Errorf("%w: click the link in the verification email from Canvasbin, or resend it from %s/account, then try again",
		store.ErrUnverified, base)
}
