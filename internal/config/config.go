// Package config loads account/billing settings from the environment. Secrets
// only ever come from env vars; nothing here has a default for a secret.
package config

import (
	"context"
	"errors"
	"os"
	"strings"

	"github.com/stewartmckee/canvasbin/internal/store"
)

const (
	FreeQuota   = 5
	ProQuota    = 100
	SignalQuota = 100
	TeamQuota   = 1000

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
	StripePriceSignal    string // Signal plan: Individual-style page limit plus page updates
	StripePriceTeamBase  string // flat monthly team price
	StripePriceTeamSeat  string // per-member monthly price
	StripePublishableKey string

	AdminEmails []string

	// OpenAIAppsChallenge is the domain-verification token the OpenAI
	// platform asks for, served at /.well-known/openai-apps-challenge.
	OpenAIAppsChallenge string

	// UmamiURL and UmamiWebsiteID enable server-side analytics events.
	UmamiURL       string
	UmamiWebsiteID string

	Updates     bool
	UpdatesMode string // feed (default) or notify_on_page_edit
	ProductName string

	FreeQuota   int
	ProQuota    int
	SignalQuota int
	TeamQuota   int
}

// Personal plans. Pro is shown to people as "Individual".
const (
	PlanFree   = "free"
	PlanPro    = "pro"
	PlanSignal = "signal"
)

const (
	UpdatesModeFeed         = "feed"
	UpdatesModeNotifyOnEdit = "notify_on_page_edit"
)

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
		StripePriceSignal:    strings.TrimSpace(get("STRIPE_PRICE_SIGNAL")),
		StripePriceTeamBase:  strings.TrimSpace(get("STRIPE_PRICE_TEAM_BASE")),
		StripePriceTeamSeat:  strings.TrimSpace(get("STRIPE_PRICE_TEAM_SEAT")),
		StripePublishableKey: strings.TrimSpace(get("STRIPE_PUBLISHABLE_KEY")),
		OpenAIAppsChallenge:  strings.TrimSpace(get("OPENAI_APPS_CHALLENGE")),
		UmamiURL:             strings.TrimSpace(get("UMAMI_URL")),
		UmamiWebsiteID:       strings.TrimSpace(get("UMAMI_WEBSITE_ID")),
		Updates:              truthy(get("UPDATES")),
		UpdatesMode:          def("UPDATES_MODE", UpdatesModeFeed),
		ProductName:          strings.TrimSpace(get("PRODUCT_NAME")),
		FreeQuota:            FreeQuota,
		ProQuota:             ProQuota,
		SignalQuota:          SignalQuota,
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
	if c.Updates && c.MailgunKey == "" {
		return errors.New("UPDATES is on but MAILGUN_SENDING_KEY is not set")
	}
	if c.UpdatesMode != "" && c.UpdatesMode != UpdatesModeFeed && c.UpdatesMode != UpdatesModeNotifyOnEdit {
		return errors.New("UPDATES_MODE must be feed or notify_on_page_edit")
	}
	return nil
}

// ProductNameOrDefault is the branding name for email and UI copy.
func (c Config) ProductNameOrDefault() string {
	if c.ProductName != "" {
		return c.ProductName
	}
	return "Canvasbin"
}

// UpdatesFeed is true when page posts use the append-only feed (not notify-on-edit only).
func (c Config) UpdatesFeed() bool {
	return c.UpdatesMode == "" || c.UpdatesMode == UpdatesModeFeed
}

// BillingEnabled is true when Stripe checkout can be offered.
func (c Config) BillingEnabled() bool {
	return c.StripeSecretKey != "" && c.StripePricePro != ""
}

// SignalBillingEnabled is true when the Signal plan can be sold.
func (c Config) SignalBillingEnabled() bool {
	return c.StripeSecretKey != "" && c.StripePriceSignal != ""
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

// Plan is the user's personal plan: free, pro, or signal. Any paid
// subscription that isn't on the Signal price counts as pro.
func (c Config) Plan(u *store.User) string {
	if !IsPro(u) {
		return PlanFree
	}
	if c.StripePriceSignal != "" && u.StripePriceID != nil && *u.StripePriceID == c.StripePriceSignal {
		return PlanSignal
	}
	return PlanPro
}

// HasUpdates reports whether u's pages may use public updates and email
// subscribers, which come with the Signal plan.
func (c Config) HasUpdates(u *store.User) bool {
	return c.Updates && c.Plan(u) == PlanSignal
}

// PageHasUpdates reports whether p may use updates: it must be a personal page
// whose owner is on the Signal plan. Team and anonymous pages never qualify.
func (c Config) PageHasUpdates(ctx context.Context, st *store.Store, p store.Page) bool {
	if !c.Updates || p.TeamID != nil || p.OwnerID == nil {
		return false
	}
	u, err := st.GetUserByID(ctx, *p.OwnerID)
	if err != nil {
		return false
	}
	return c.HasUpdates(u)
}

// Quota is the number of pages the user may own.
func (c Config) Quota(u *store.User) int {
	free, pro := c.PlanQuotas()
	switch c.Plan(u) {
	case PlanSignal:
		return c.SignalPageQuota()
	case PlanPro:
		return pro
	}
	return free
}

// SignalPageQuota is the number of pages a Signal subscriber may own.
func (c Config) SignalPageQuota() int {
	if c.SignalQuota == 0 {
		return SignalQuota
	}
	return c.SignalQuota
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
