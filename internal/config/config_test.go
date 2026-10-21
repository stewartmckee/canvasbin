package config

import (
	"testing"

	"github.com/stewartmckee/canvasbin/internal/store"
)

func env(m map[string]string) func(string) string { return func(k string) string { return m[k] } }

func TestLoadDefaults(t *testing.T) {
	c := fromGetenv(env(nil))
	if c.Accounts || c.MailgunDomain != "canvasbin.dev" || c.MailFrom != "Canvasbin <noreply@canvasbin.dev>" {
		t.Fatalf("bad defaults: %+v", c)
	}
	if c.FreeQuota != 5 || c.ProQuota != 100 {
		t.Fatalf("bad quotas: %+v", c)
	}
	if err := c.Validate(); err != nil {
		t.Fatalf("accounts off needs no mailgun key: %v", err)
	}
}

func TestLoadAccounts(t *testing.T) {
	for _, v := range []string{"on", "ON", "true", "1"} {
		if !fromGetenv(env(map[string]string{"ACCOUNTS": v})).Accounts {
			t.Errorf("ACCOUNTS=%q should enable", v)
		}
	}
	if fromGetenv(env(map[string]string{"ACCOUNTS": "off"})).Accounts {
		t.Error("ACCOUNTS=off should disable")
	}
	c := fromGetenv(env(map[string]string{"ACCOUNTS": "on", "ADMIN_EMAILS": " A@x.com, ,b@x.com "}))
	if err := c.Validate(); err == nil {
		t.Error("accounts on without a mailgun key should fail validation")
	}
	if !c.IsAdmin("a@X.com") || !c.IsAdmin("b@x.com") || c.IsAdmin("c@x.com") || len(c.AdminEmails) != 2 {
		t.Errorf("admin parsing wrong: %v", c.AdminEmails)
	}
}

func TestValidateUpdatesNeedsMail(t *testing.T) {
	c := fromGetenv(env(map[string]string{"UPDATES": "on"}))
	if err := c.Validate(); err == nil {
		t.Fatal("updates on without mailgun key should fail validation")
	}
}

func TestQuota(t *testing.T) {
	c := fromGetenv(env(nil))
	cases := map[string]int{"": 5, "canceled": 5, "incomplete": 5, "unpaid": 5, "trialing": 100, "active": 100, "past_due": 100}
	for status, want := range cases {
		u := &store.User{SubscriptionStatus: status}
		if got := c.Quota(u); got != want {
			t.Errorf("status %q: quota %d, want %d", status, got, want)
		}
		if IsPro(u) != (want == 100) {
			t.Errorf("status %q: IsPro mismatch", status)
		}
	}
}

func TestSignalPlan(t *testing.T) {
	c := fromGetenv(env(map[string]string{"UPDATES": "on", "STRIPE_PRICE_PRO": "price_pro", "STRIPE_PRICE_SIGNAL": "price_signal"}))
	str := func(s string) *string { return &s }
	cases := []struct {
		status  string
		price   *string
		plan    string
		updates bool
	}{
		{"", nil, PlanFree, false},
		{"canceled", str("price_signal"), PlanFree, false},
		{"active", str("price_pro"), PlanPro, false},
		{"active", nil, PlanPro, false},
		{"active", str("price_signal"), PlanSignal, true},
		{"past_due", str("price_signal"), PlanSignal, true},
	}
	for _, tc := range cases {
		u := &store.User{SubscriptionStatus: tc.status, StripePriceID: tc.price}
		if got := c.Plan(u); got != tc.plan {
			t.Errorf("%s/%v: plan %q, want %q", tc.status, tc.price, got, tc.plan)
		}
		if got := c.HasUpdates(u); got != tc.updates {
			t.Errorf("%s/%v: updates %v, want %v", tc.status, tc.price, got, tc.updates)
		}
	}
	signal := &store.User{SubscriptionStatus: "active", StripePriceID: str("price_signal")}
	if got := c.Quota(signal); got != SignalQuota {
		t.Errorf("signal quota %d, want %d", got, SignalQuota)
	}
	c.Updates = false
	if c.HasUpdates(signal) {
		t.Error("UPDATES=off should disable updates even on Signal")
	}
}
