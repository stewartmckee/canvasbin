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
	"strings"
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

// TeamEnabled reports whether team subscriptions can be sold.
func (s *Service) TeamEnabled() bool {
	return s.api != nil && s.cfg.StripePriceTeamBase != "" && s.cfg.StripePriceTeamSeat != ""
}

// ensureTeamCustomer returns the team's Stripe customer, creating one billed to
// the given user's email if needed.
func (s *Service) ensureTeamCustomer(ctx context.Context, t *store.Team, billTo *store.User) (string, error) {
	if t.StripeCustomerID != nil && *t.StripeCustomerID != "" {
		return *t.StripeCustomerID, nil
	}
	params := &stripe.CustomerParams{
		Name:     stripe.String(t.Name),
		Email:    stripe.String(billTo.Email),
		Metadata: map[string]string{"canvasbin_team_id": strconv.FormatInt(t.ID, 10)},
	}
	params.Context = ctx
	params.SetIdempotencyKey(fmt.Sprintf("canvasbin-team-customer-%d", t.ID))
	c, err := s.api.Customers.New(params)
	if err != nil {
		return "", fmt.Errorf("create stripe customer: %w", err)
	}
	if err := s.store.SetTeamStripeCustomer(ctx, t.ID, c.ID); err != nil {
		return "", err
	}
	t.StripeCustomerID = &c.ID
	return c.ID, nil
}

// CreateTeamCheckoutSession starts the team subscription: the flat base price
// plus one seat per current member.
func (s *Service) CreateTeamCheckoutSession(ctx context.Context, t *store.Team, billTo *store.User, seats int, successURL, cancelURL string) (string, error) {
	if !s.TeamEnabled() {
		return "", ErrNotConfigured
	}
	if seats < 1 {
		seats = 1
	}
	cust, err := s.ensureTeamCustomer(ctx, t, billTo)
	if err != nil {
		return "", err
	}
	tid := strconv.FormatInt(t.ID, 10)
	params := &stripe.CheckoutSessionParams{
		Mode:              stripe.String(string(stripe.CheckoutSessionModeSubscription)),
		Customer:          stripe.String(cust),
		ClientReferenceID: stripe.String(teamRefPrefix + tid),
		SuccessURL:        stripe.String(successURL),
		CancelURL:         stripe.String(cancelURL),
		LineItems: []*stripe.CheckoutSessionLineItemParams{
			{Price: stripe.String(s.cfg.StripePriceTeamBase), Quantity: stripe.Int64(1)},
			{Price: stripe.String(s.cfg.StripePriceTeamSeat), Quantity: stripe.Int64(int64(seats))},
		},
		SubscriptionData: &stripe.CheckoutSessionSubscriptionDataParams{
			Metadata: map[string]string{"canvasbin_team_id": tid},
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

// CreateTeamPortalSession opens the customer portal for a team's subscription.
func (s *Service) CreateTeamPortalSession(ctx context.Context, t *store.Team, returnURL string) (string, error) {
	if s.api == nil {
		return "", ErrNotConfigured
	}
	if t.StripeCustomerID == nil || *t.StripeCustomerID == "" {
		return "", ErrNoCustomer
	}
	params := &stripe.BillingPortalSessionParams{
		Customer:  stripe.String(*t.StripeCustomerID),
		ReturnURL: stripe.String(returnURL),
	}
	params.Context = ctx
	sess, err := s.api.BillingPortalSessions.New(params)
	if err != nil {
		return "", fmt.Errorf("create portal session: %w", err)
	}
	return sess.URL, nil
}

// SyncTeamSeats sets the seat quantity on a team's subscription to its member
// count, prorating the change. Teams without a subscription are left alone.
func (s *Service) SyncTeamSeats(ctx context.Context, teamID int64) error {
	if !s.TeamEnabled() {
		return nil
	}
	t, err := s.store.GetTeamByID(ctx, teamID)
	if err != nil {
		return err
	}
	if t.StripeSubscriptionID == nil || *t.StripeSubscriptionID == "" || !t.Active() {
		return nil
	}
	sub, err := s.api.Subscriptions.Get(*t.StripeSubscriptionID, &stripe.SubscriptionParams{Params: stripe.Params{Context: ctx}})
	if err != nil {
		return fmt.Errorf("fetch subscription: %w", err)
	}
	seats := int64(t.MemberCount)
	if seats < 1 {
		seats = 1
	}
	for _, item := range sub.Items.Data {
		if item.Price == nil || item.Price.ID != s.cfg.StripePriceTeamSeat {
			continue
		}
		if item.Quantity == seats {
			return nil
		}
		params := &stripe.SubscriptionItemParams{
			Quantity:          stripe.Int64(seats),
			ProrationBehavior: stripe.String("create_prorations"),
		}
		params.Context = ctx
		if _, err := s.api.SubscriptionItems.Update(item.ID, params); err != nil {
			return fmt.Errorf("update seats: %w", err)
		}
		return nil
	}
	params := &stripe.SubscriptionItemParams{
		Subscription:      stripe.String(sub.ID),
		Price:             stripe.String(s.cfg.StripePriceTeamSeat),
		Quantity:          stripe.Int64(seats),
		ProrationBehavior: stripe.String("create_prorations"),
	}
	params.Context = ctx
	if _, err := s.api.SubscriptionItems.New(params); err != nil {
		return fmt.Errorf("add seats: %w", err)
	}
	return nil
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
		if custID == "" || s.api == nil {
			return nil
		}
		teamID, isTeam := teamRef(cs.ClientReferenceID)
		uid, _ := strconv.ParseInt(cs.ClientReferenceID, 10, 64)
		if !isTeam && uid == 0 {
			return nil
		}
		sub, err := s.api.Subscriptions.Get(cs.Subscription.ID, &stripe.SubscriptionParams{Params: stripe.Params{Context: ctx}})
		if err != nil {
			return fmt.Errorf("fetch subscription: %w", err)
		}
		if isTeam {
			return s.applyTeamSubscription(ctx, sub, teamID)
		}
		return s.applySubscription(ctx, sub, uid)
	case "customer.subscription.created", "customer.subscription.updated", "customer.subscription.deleted":
		var sub stripe.Subscription
		if err := json.Unmarshal(ev.Data.Raw, &sub); err != nil {
			return err
		}
		if handled, err := s.maybeTeamSubscription(ctx, &sub); handled || err != nil {
			return err
		}
		return s.applySubscription(ctx, &sub, 0)
	}
	return nil
}

const teamRefPrefix = "team_"

func teamRef(ref string) (int64, bool) {
	if !strings.HasPrefix(ref, teamRefPrefix) {
		return 0, false
	}
	id, err := strconv.ParseInt(strings.TrimPrefix(ref, teamRefPrefix), 10, 64)
	return id, err == nil && id > 0
}

// maybeTeamSubscription applies sub to a team when it belongs to one, either by
// metadata or by the team's Stripe customer.
func (s *Service) maybeTeamSubscription(ctx context.Context, sub *stripe.Subscription) (bool, error) {
	if id, _ := strconv.ParseInt(sub.Metadata["canvasbin_team_id"], 10, 64); id > 0 {
		return true, s.applyTeamSubscription(ctx, sub, id)
	}
	if sub.Customer == nil {
		return false, nil
	}
	t, err := s.store.GetTeamByStripeCustomer(ctx, sub.Customer.ID)
	if errors.Is(err, store.ErrNotFound) {
		return false, nil
	}
	if err != nil {
		return true, err
	}
	return true, s.applyTeamSubscription(ctx, sub, t.ID)
}

func (s *Service) applyTeamSubscription(ctx context.Context, sub *stripe.Subscription, teamID int64) error {
	if _, err := s.store.GetTeamByID(ctx, teamID); errors.Is(err, store.ErrNotFound) {
		slog.Warn("stripe subscription for unknown team", "subscription", sub.ID)
		return nil
	} else if err != nil {
		return err
	}
	return s.store.UpdateTeamSubscription(ctx, teamID, SubscriptionFrom(sub))
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
