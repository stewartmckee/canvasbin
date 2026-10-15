// Package billing wraps Stripe Checkout, the customer portal and webhooks.
// Keys come only from configuration (environment variables).
package billing

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/stripe/stripe-go/v82"
	"github.com/stripe/stripe-go/v82/client"
	"github.com/stripe/stripe-go/v82/webhook"

	"github.com/stewartmckee/canvasbin/internal/config"
	"github.com/stewartmckee/canvasbin/internal/store"
)

var (
	ErrNotConfigured = errors.New("billing is not configured")
	ErrNoCustomer    = errors.New("no billing account yet")
)

type Service struct {
	cfg   config.Config
	store *store.Store
	api   *client.API
}

func New(cfg config.Config, st *store.Store) *Service {
	s := &Service{cfg: cfg, store: st}
	if cfg.StripeSecretKey != "" {
		s.api = client.New(cfg.StripeSecretKey, nil)
	}
	return s
}

// Enabled reports whether Stripe is configured enough to start checkout.
func (s *Service) Enabled() bool { return s.api != nil && s.cfg.StripePricePro != "" }

// EnsureCustomer returns the user's Stripe customer id, creating one if needed.
func (s *Service) EnsureCustomer(ctx context.Context, u *store.User) (string, error) {
	if s.api == nil {
		return "", ErrNotConfigured
	}
	if u.StripeCustomerID != nil && *u.StripeCustomerID != "" {
		return *u.StripeCustomerID, nil
	}
	params := &stripe.CustomerParams{
		Email:    stripe.String(u.Email),
		Metadata: map[string]string{"canvasbin_user_id": strconv.FormatInt(u.ID, 10)},
	}
	if u.DisplayName != "" {
		params.Name = stripe.String(u.DisplayName)
	}
	params.Context = ctx
	// Idempotency stops concurrent clicks creating duplicate customers.
	params.SetIdempotencyKey(fmt.Sprintf("canvasbin-customer-%d", u.ID))
	c, err := s.api.Customers.New(params)
	if err != nil {
		return "", fmt.Errorf("create stripe customer: %w", err)
	}
	if err := s.store.SetStripeCustomer(ctx, u.ID, c.ID); err != nil {
		return "", err
	}
	u.StripeCustomerID = &c.ID
	return c.ID, nil
}

// CreateCheckoutSession starts a subscription checkout for the Pro price.
func (s *Service) CreateCheckoutSession(ctx context.Context, u *store.User, successURL, cancelURL string) (string, error) {
	if !s.Enabled() {
		return "", ErrNotConfigured
	}
	cust, err := s.EnsureCustomer(ctx, u)
	if err != nil {
		return "", err
	}
	uid := strconv.FormatInt(u.ID, 10)
	params := &stripe.CheckoutSessionParams{
		Mode:              stripe.String(string(stripe.CheckoutSessionModeSubscription)),
		Customer:          stripe.String(cust),
		ClientReferenceID: stripe.String(uid),
		SuccessURL:        stripe.String(successURL),
		CancelURL:         stripe.String(cancelURL),
		LineItems: []*stripe.CheckoutSessionLineItemParams{
			{Price: stripe.String(s.cfg.StripePricePro), Quantity: stripe.Int64(1)},
		},
		SubscriptionData: &stripe.CheckoutSessionSubscriptionDataParams{
			Metadata: map[string]string{"canvasbin_user_id": uid},
		},
		AllowPromotionCodes: stripe.Bool(true),
	}
	params.Context = ctx
	sess, err := s.api.CheckoutSessions.New(params)
	if err != nil {
		return "", fmt.Errorf("create checkout session: %w", err)
	}
	return sess.URL, nil
}

// CreatePortalSession opens the Stripe customer portal for managing a subscription.
func (s *Service) CreatePortalSession(ctx context.Context, u *store.User, returnURL string) (string, error) {
	if s.api == nil {
		return "", ErrNotConfigured
	}
	if u.StripeCustomerID == nil || *u.StripeCustomerID == "" {
		return "", ErrNoCustomer
	}
	params := &stripe.BillingPortalSessionParams{
		Customer:  stripe.String(*u.StripeCustomerID),
		ReturnURL: stripe.String(returnURL),
	}
	params.Context = ctx
	sess, err := s.api.BillingPortalSessions.New(params)
	if err != nil {
		return "", fmt.Errorf("create portal session: %w", err)
	}
	return sess.URL, nil
}

// ErrBadSignature is returned when a webhook fails signature verification.
var ErrBadSignature = errors.New("invalid webhook signature")

// HandleWebhook verifies and applies a Stripe event. Unknown events are ignored.
func (s *Service) HandleWebhook(ctx context.Context, payload []byte, signature string) error {
	if s.cfg.StripeWebhookSecret == "" {
		return ErrNotConfigured
	}
	ev, err := webhook.ConstructEventWithOptions(payload, signature, s.cfg.StripeWebhookSecret,
		webhook.ConstructEventOptions{IgnoreAPIVersionMismatch: true})
	if err != nil {
		return ErrBadSignature
	}
	return s.apply(ctx, ev)
}

func (s *Service) apply(ctx context.Context, ev stripe.Event) error {
	switch ev.Type {
	case "checkout.session.completed":
		var cs stripe.CheckoutSession
		if err := json.Unmarshal(ev.Data.Raw, &cs); err != nil {
			return err
		}
		if cs.Mode != stripe.CheckoutSessionModeSubscription || cs.Subscription == nil || cs.Subscription.ID == "" {
			return nil
		}
		// The subscription.* events carry full state; checkout completion only
		// needs to link the customer when a subscription event raced ahead.
		custID := ""
		if cs.Customer != nil {
			custID = cs.Customer.ID
		}
		uid, _ := strconv.ParseInt(cs.ClientReferenceID, 10, 64)
		if uid == 0 || custID == "" || s.api == nil {
			return nil
		}
		sub, err := s.api.Subscriptions.Get(cs.Subscription.ID, &stripe.SubscriptionParams{Params: stripe.Params{Context: ctx}})
		if err != nil {
			return fmt.Errorf("fetch subscription: %w", err)
		}
		return s.applySubscription(ctx, sub, uid)
	case "customer.subscription.created", "customer.subscription.updated", "customer.subscription.deleted":
		var sub stripe.Subscription
		if err := json.Unmarshal(ev.Data.Raw, &sub); err != nil {
			return err
		}
		return s.applySubscription(ctx, &sub, 0)
	}
	return nil
}

// applySubscription writes sub onto the matching user. hintUserID is used when
// the customer isn't linked to a user yet.
func (s *Service) applySubscription(ctx context.Context, sub *stripe.Subscription, hintUserID int64) error {
	if sub.Customer == nil {
		return nil
	}
	u, err := s.store.GetUserByStripeCustomer(ctx, sub.Customer.ID)
	if errors.Is(err, store.ErrNotFound) {
		uid := hintUserID
		if uid == 0 {
			uid, _ = strconv.ParseInt(sub.Metadata["canvasbin_user_id"], 10, 64)
		}
		if uid == 0 {
			slog.Warn("stripe subscription for unknown customer", "subscription", sub.ID)
			return nil
		}
		u, err = s.store.GetUserByID(ctx, uid)
	}
	if errors.Is(err, store.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	return s.store.UpdateSubscription(ctx, u.ID, SubscriptionFrom(sub))
}

// SubscriptionFrom maps a Stripe subscription to the stored fields. A deleted
// subscription keeps its history but its status ("canceled") removes Pro.
func SubscriptionFrom(sub *stripe.Subscription) store.Subscription {
	out := store.Subscription{
		SubscriptionID:    sub.ID,
		Status:            string(sub.Status),
		CancelAtPeriodEnd: sub.CancelAtPeriodEnd,
	}
	if sub.Customer != nil {
		out.CustomerID = sub.Customer.ID
	}
	if sub.Items != nil && len(sub.Items.Data) > 0 {
		item := sub.Items.Data[0]
		if item.Price != nil {
			out.PriceID = item.Price.ID
		}
		// With recent API versions the billing period lives on the item.
		if item.CurrentPeriodEnd > 0 {
			t := time.Unix(item.CurrentPeriodEnd, 0).UTC()
			out.CurrentPeriodEnd = &t
		}
	}
	return out
}
