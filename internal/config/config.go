// Package config loads account/billing settings from the environment. Secrets
// only ever come from env vars; nothing here has a default for a secret.
package config

import (
	"errors"
	"os"
	"strings"

	"github.com/stewartmckee/canvasbin/internal/store"
)

const (
	FreeQuota = 5
	ProQuota  = 100
	TeamQuota = 1000

	DefaultMailgunDomain = "canvasbin.dev"
	DefaultMailFrom      = "Canvasbin <noreply@canvasbin.dev>"
)

type Config struct {
	Accounts      bool
	PublicBaseURL string

	MailgunKey    string
	MailgunDomain string
	MailFrom      string

	StripeSecretKey      string
	StripeWebhookSecret  string
	StripePricePro       string
	StripePriceTeamBase  string // flat monthly team price
	StripePriceTeamSeat  string // per-member monthly price
	StripePublishableKey string

	AdminEmails []string

	// OpenAIAppsChallenge is the domain-verification token the OpenAI
	// platform asks for, served at /.well-known/openai-apps-challenge.
	OpenAIAppsChallenge string

	FreeQuota int
	ProQuota  int
	TeamQuota int
}

// Load reads configuration from the process environment.
func Load() Config {
	return fromGetenv(os.Getenv)
}

func fromGetenv(get func(string) string) Config {
	def := func(k, d string) string {
		if v := strings.TrimSpace(get(k)); v != "" {
			return v
		}
		return d
	}
	c := Config{
		Accounts:             truthy(get("ACCOUNTS")),
		PublicBaseURL:        strings.TrimRight(strings.TrimSpace(get("PUBLIC_BASE_URL")), "/"),
		MailgunKey:           strings.TrimSpace(get("MAILGUN_SENDING_KEY")),
		MailgunDomain:        def("MAILGUN_DOMAIN", DefaultMailgunDomain),
		MailFrom:             def("MAIL_FROM", DefaultMailFrom),
		StripeSecretKey:      strings.TrimSpace(get("STRIPE_SECRET_KEY")),
		StripeWebhookSecret:  strings.TrimSpace(get("STRIPE_WEBHOOK_SECRET")),
		StripePricePro:       strings.TrimSpace(get("STRIPE_PRICE_PRO")),
		StripePriceTeamBase:  strings.TrimSpace(get("STRIPE_PRICE_TEAM_BASE")),
		StripePriceTeamSeat:  strings.TrimSpace(get("STRIPE_PRICE_TEAM_SEAT")),
		StripePublishableKey: strings.TrimSpace(get("STRIPE_PUBLISHABLE_KEY")),
		OpenAIAppsChallenge:  strings.TrimSpace(get("OPENAI_APPS_CHALLENGE")),
		FreeQuota:            FreeQuota,
		ProQuota:             ProQuota,
		TeamQuota:            TeamQuota,
	}
	for _, e := range strings.Split(get("ADMIN_EMAILS"), ",") {
		if e = store.NormalizeEmail(e); e != "" {
			c.AdminEmails = append(c.AdminEmails, e)
		}
	}
	return c
}

func truthy(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "on", "true", "1", "yes":
		return true
	}
	return false
}

// Validate reports missing settings required for the enabled features.
func (c Config) Validate() error {
	if c.Accounts && c.MailgunKey == "" {
		return errors.New("ACCOUNTS is on but MAILGUN_SENDING_KEY is not set")
	}
	return nil
}

// BillingEnabled is true when Stripe checkout can be offered.
func (c Config) BillingEnabled() bool {
	return c.StripeSecretKey != "" && c.StripePricePro != ""
}

// TeamBillingEnabled is true when team subscriptions can be sold.
func (c Config) TeamBillingEnabled() bool {
	return c.StripeSecretKey != "" && c.StripePriceTeamBase != "" && c.StripePriceTeamSeat != ""
}

// TeamPageQuota is the number of pages an active team may hold.
func (c Config) TeamPageQuota() int {
	if c.TeamQuota == 0 {
		return TeamQuota
	}
	return c.TeamQuota
}

// IsAdmin reports whether email is listed in ADMIN_EMAILS.
func (c Config) IsAdmin(email string) bool {
	email = store.NormalizeEmail(email)
	for _, a := range c.AdminEmails {
		if a == email {
			return true
		}
	}
	return false
}

// IsPro is true while a subscription is trialing, active, or past_due (past_due
// keeps the paid quota while Stripe retries payment).
func IsPro(u *store.User) bool {
	if u == nil {
		return false
	}
	switch u.SubscriptionStatus {
	case "trialing", "active", "past_due":
		return true
	}
	return false
}

// Quota is the number of pages the user may own.
func (c Config) Quota(u *store.User) int {
	free, pro := c.PlanQuotas()
	if IsPro(u) {
		return pro
	}
	return free
}

// PlanQuotas returns the free and Pro page limits, applying defaults.
func (c Config) PlanQuotas() (free, pro int) {
	free, pro = c.FreeQuota, c.ProQuota
	if free == 0 {
		free = FreeQuota
	}
	if pro == 0 {
		pro = ProQuota
	}
	return free, pro
}
