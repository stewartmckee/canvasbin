package billing

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stripe/stripe-go/v82"
	"github.com/stripe/stripe-go/v82/webhook"

	"github.com/stewartmckee/canvasbin/internal/config"
	"github.com/stewartmckee/canvasbin/internal/store"
)

const testSecret = "test-webhook-secret-placeholder"

func TestSubscriptionFrom(t *testing.T) {
	end := time.Date(2030, 1, 2, 3, 4, 5, 0, time.UTC)
	sub := &stripe.Subscription{
		ID:                "sub_1",
		Status:            stripe.SubscriptionStatusPastDue,
		CancelAtPeriodEnd: true,
		Customer:          &stripe.Customer{ID: "cus_1"},
		Items: &stripe.SubscriptionItemList{Data: []*stripe.SubscriptionItem{
			{Price: &stripe.Price{ID: "price_1"}, CurrentPeriodEnd: end.Unix()},
		}},
	}
	got := SubscriptionFrom(sub)
	if got.CustomerID != "cus_1" || got.SubscriptionID != "sub_1" || got.PriceID != "price_1" ||
		got.Status != "past_due" || !got.CancelAtPeriodEnd || got.CurrentPeriodEnd == nil || !got.CurrentPeriodEnd.Equal(end) {
		t.Fatalf("bad mapping: %+v", got)
	}
}

func TestWebhookRejectsBadSignature(t *testing.T) {
	s := New(config.Config{StripeWebhookSecret: testSecret}, nil)
	if err := s.HandleWebhook(context.Background(), []byte(`{}`), "t=1,v1=bad"); !errors.Is(err, ErrBadSignature) {
		t.Fatalf("want ErrBadSignature, got %v", err)
	}
	s = New(config.Config{}, nil)
	if err := s.HandleWebhook(context.Background(), []byte(`{}`), ""); !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("want ErrNotConfigured, got %v", err)
	}
}

func TestWebhookUpdatesSubscription(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	st, err := store.Open(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	u, err := st.CreateUser(ctx, fmt.Sprintf("bill-%d@example.com", time.Now().UnixNano()), "h", "")
	if err != nil {
		t.Fatal(err)
	}
	cust := fmt.Sprintf("cus_test_%d", u.ID)
	if err := st.SetStripeCustomer(ctx, u.ID, cust); err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{StripeWebhookSecret: testSecret}
	s := New(cfg, st)

	send := func(typ, status string) {
		t.Helper()
		payload := fmt.Sprintf(`{"id":"evt_1","object":"event","type":%q,"data":{"object":{"id":"sub_%d","object":"subscription","status":%q,"customer":%q,"cancel_at_period_end":false,"items":{"object":"list","data":[{"id":"si_1","price":{"id":"price_pro"},"current_period_end":1893456000}]}}}}`,
			typ, u.ID, status, cust)
		signed := webhook.GenerateTestSignedPayload(&webhook.UnsignedPayload{Payload: []byte(payload), Secret: testSecret})
		if err := s.HandleWebhook(ctx, signed.Payload, signed.Header); err != nil {
			t.Fatal(err)
		}
	}
	send("customer.subscription.updated", "active")
	got, _ := st.GetUserByID(ctx, u.ID)
	if !config.IsPro(got) || got.StripePriceID == nil || *got.StripePriceID != "price_pro" || got.CurrentPeriodEnd == nil {
		t.Fatalf("not pro after active: %+v", got)
	}
	send("customer.subscription.deleted", "canceled")
	got, _ = st.GetUserByID(ctx, u.ID)
	if config.IsPro(got) || got.SubscriptionStatus != "canceled" {
		t.Fatalf("still pro after cancel: %+v", got)
	}
}

func TestTeamRef(t *testing.T) {
	if id, ok := teamRef("team_42"); !ok || id != 42 {
		t.Fatalf("team_42: %d %v", id, ok)
	}
	for _, ref := range []string{"42", "", "team_", "team_x", "team_-1"} {
		if _, ok := teamRef(ref); ok {
			t.Fatalf("%q should not be a team ref", ref)
		}
	}
}
