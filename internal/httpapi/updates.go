package httpapi

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/stewartmckee/canvasbin/internal/auth"
	"github.com/stewartmckee/canvasbin/internal/render"
	"github.com/stewartmckee/canvasbin/internal/store"
)

func (s *Server) updatesOn() bool { return s.Config.Updates }

// pageUpdatesAllowed reports whether the page's owner is on a plan with updates.
func (s *Server) pageUpdatesAllowed(ctx context.Context, a *store.Access) bool {
	return s.Config.PageHasUpdates(ctx, s.Store, a.Page)
}

// pageUpdatesLive reports whether the page shows updates and takes subscribers.
func (s *Server) pageUpdatesLive(ctx context.Context, a *store.Access) bool {
	if !s.pageUpdatesAllowed(ctx, a) {
		return false
	}
	on, err := s.Store.UpdatesEnabled(ctx, a.ID)
	return err == nil && on
}

func (s *Server) registerUpdateRoutes(mux *http.ServeMux) {
	if !s.updatesOn() {
		return
	}
	mux.HandleFunc("GET /api/pages/{key}/updates", s.listPageUpdates)
	mux.HandleFunc("POST /api/pages/{key}/subscribe", s.subscribePage)
	mux.HandleFunc("OPTIONS /api/pages/{key}/subscribe", s.subscribePreflight)
	mux.HandleFunc("POST /api/pages/{key}/updates", s.createPageUpdate)
	mux.HandleFunc("PATCH /api/pages/{key}/updates-settings", s.patchUpdatesSettings)
	mux.HandleFunc("GET /api/subscribe/confirm", s.confirmSubscribe)
	mux.HandleFunc("GET /api/unsubscribe", s.unsubscribePage)
}

func (s *Server) lookupPageByKey(ctx context.Context, r *http.Request, key string, needWrite bool) (*store.Access, error) {
	if u := s.SessionUser(r); u != nil {
		a, err := s.Store.GetForUser(ctx, key, u.ID)
		if err != nil {
			return nil, err
		}
		if needWrite && !a.CanWrite {
			return nil, store.ErrReadOnly
		}
		return a, nil
	}
	a, err := s.Store.Get(ctx, key)
	if err != nil {
		return nil, err
	}
	if needWrite && !a.CanWrite {
		return nil, store.ErrReadOnly
	}
	return a, nil
}

func (s *Server) listPageUpdates(w http.ResponseWriter, r *http.Request) {
	if !s.updatesOn() {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	key, ok := pathKey(w, r)
	if !ok {
		return
	}
	a, err := s.lookupPageByKey(r.Context(), r, key, false)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	if !s.pageUpdatesLive(r.Context(), a) {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	list, err := s.Store.ListPageUpdates(r.Context(), a.ID, 50)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"updates": list})
}

type subscribeInput struct {
	Email string `json:"email"`
}

// allowSandboxCaller lets rendered pages call an endpoint. They run in an
// opaque-origin sandbox and send Origin: null, so any origin is allowed; this
// is only for endpoints that use no cookies or session.
func allowSandboxCaller(w http.ResponseWriter) {
	h := w.Header()
	h.Set("Access-Control-Allow-Origin", "*")
	h.Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	h.Set("Access-Control-Allow-Headers", "Content-Type")
	h.Set("Access-Control-Max-Age", "86400")
}

func (s *Server) subscribePreflight(w http.ResponseWriter, r *http.Request) {
	allowSandboxCaller(w)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) subscribePage(w http.ResponseWriter, r *http.Request) {
	allowSandboxCaller(w)
	if !s.updatesOn() {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	if s.Mailer == nil {
		writeError(w, http.StatusServiceUnavailable, "email is not configured")
		return
	}
	key, ok := pathKey(w, r)
	if !ok {
		return
	}
	a, err := s.lookupPageByKey(r.Context(), r, key, false)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	if !s.pageUpdatesLive(r.Context(), a) {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	var in subscribeInput
	if !decode(w, r, &in) {
		return
	}
	email, err := auth.ValidateEmail(in.Email)
	if err != nil {
		writeError(w, http.StatusBadRequest, "enter a valid email address")
		return
	}
	if !s.allowMail(w, email) {
		return
	}
	tok, err := s.Store.SubscribePage(r.Context(), a.ID, email)
	if err != nil {
		if errors.Is(err, store.ErrAlreadySubscribed) {
			writeJSON(w, http.StatusOK, map[string]string{"message": "You are already subscribed."})
			return
		}
		writeStoreError(w, err)
		return
	}
	link := s.baseURL(r) + "/api/subscribe/confirm?token=" + tok
	if err := s.Mailer.SendUpdateSubscribeConfirm(r.Context(), email, link, a.Title, s.Config.ProductNameOrDefault()); err != nil {
		slog.Error("subscribe confirm email", "err", err)
		writeError(w, http.StatusInternalServerError, "could not send confirmation email")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "Check your email to confirm your subscription."})
}

func (s *Server) confirmSubscribe(w http.ResponseWriter, r *http.Request) {
	if !s.updatesOn() {
		http.NotFound(w, r)
		return
	}
	sub, err := s.Store.ConfirmSubscriber(r.Context(), strings.TrimSpace(r.URL.Query().Get("token")))
	if err != nil {
		s.writeNotice(w, r, http.StatusBadRequest, render.Notice{
			Kind: "error", Title: "Link not valid",
			Heading: "This link isn’t valid",
			Message: "It may have expired or already been used to unsubscribe. Subscribe again from the page to get a new confirmation email.",
		}, store.SubscriptionPage{})
		return
	}
	s.writeNotice(w, r, http.StatusOK, render.Notice{
		Kind: "success", Title: "Subscribed",
		Heading: "You’re subscribed",
		Message: "We’ll email you whenever a new update is posted on",
	}, sub)
}

func (s *Server) unsubscribePage(w http.ResponseWriter, r *http.Request) {
	if !s.updatesOn() {
		http.NotFound(w, r)
		return
	}
	sub, err := s.Store.UnsubscribeByToken(r.Context(), strings.TrimSpace(r.URL.Query().Get("token")))
	if err != nil {
		s.writeNotice(w, r, http.StatusBadRequest, render.Notice{
			Kind: "error", Title: "Link not valid",
			Heading: "This link isn’t valid",
			Message: "We couldn’t find a subscription for this link. If you’re still getting emails, use the unsubscribe link in the latest one.",
		}, store.SubscriptionPage{})
		return
	}
	s.writeNotice(w, r, http.StatusOK, render.Notice{
		Kind: "neutral", Title: "Unsubscribed",
		Heading: "You’re unsubscribed",
		Message: "You won’t get any more update emails for",
	}, sub)
}

// writeNotice renders a standalone notice page, adding the subscription's
// page title, email and a link back to the page when known.
func (s *Server) writeNotice(w http.ResponseWriter, r *http.Request, status int, n render.Notice, sub store.SubscriptionPage) {
	n.Product = s.Config.ProductNameOrDefault()
	n.Email = sub.Email
	if sub.Title != "" {
		n.PageTitle = sub.Title
	} else if sub.Email != "" {
		n.PageTitle = "this page"
	}
	if sub.ReadKey != "" {
		n.LinkURL = s.baseURL(r) + "/" + sub.ReadKey + ".html"
		n.LinkText = "View the page"
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(render.NoticePage(n)))
}

type postUpdateInput struct {
	Title     string     `json:"title"`
	Body      string     `json:"body"`
	PublishAt *time.Time `json:"publish_at"`
}

func (s *Server) createPageUpdate(w http.ResponseWriter, r *http.Request) {
	if !s.updatesOn() || !s.Config.UpdatesFeed() {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	key, ok := pathKey(w, r)
	if !ok {
		return
	}
	a, err := s.lookupPageByKey(r.Context(), r, key, true)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	if !s.pageUpdatesAllowed(r.Context(), a) {
		writeStoreError(w, s.Config.UpdatesPlanError(s.baseURL(r)))
		return
	}
	var in postUpdateInput
	if !decode(w, r, &in) {
		return
	}
	var uid *int64
	if u := s.SessionUser(r); u != nil {
		uid = &u.ID
	}
	var at time.Time
	if in.PublishAt != nil {
		at = *in.PublishAt
	}
	u, err := s.Store.CreatePageUpdate(r.Context(), a.ID, uid, in.Title, in.Body, a.ReadKey, at)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	if !u.Scheduled {
		s.enqueueUpdateEmails(r.Context(), u, a.ReadKey)
	}
	writeJSON(w, http.StatusCreated, map[string]any{"update": u})
}

type updatesSettingsInput struct {
	Enabled *bool `json:"updates_enabled"`
}

func (s *Server) patchUpdatesSettings(w http.ResponseWriter, r *http.Request) {
	if !s.updatesOn() {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	key, ok := pathKey(w, r)
	if !ok {
		return
	}
	a, err := s.lookupPageByKey(r.Context(), r, key, true)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	var in updatesSettingsInput
	if !decode(w, r, &in) || in.Enabled == nil {
		writeError(w, http.StatusBadRequest, "updates_enabled is required")
		return
	}
	if *in.Enabled && !s.pageUpdatesAllowed(r.Context(), a) {
		writeStoreError(w, s.Config.UpdatesPlanError(s.baseURL(r)))
		return
	}
	if err := s.Store.SetUpdatesEnabled(r.Context(), a.ID, *in.Enabled); err != nil {
		writeStoreError(w, err)
		return
	}
	count, _ := s.Store.ActiveSubscriberCount(r.Context(), a.ID)
	writeJSON(w, http.StatusOK, map[string]any{"updates_enabled": *in.Enabled, "subscriber_count": count})
}

func (s *Server) enqueueUpdateEmails(ctx context.Context, u *store.PageUpdate, readKey string) {
	if s.Mailer == nil {
		return
	}
	n, err := s.Store.EnqueueUpdateNotifications(ctx, u.ID, u.PageID, readKey)
	if err != nil {
		slog.Error("enqueue update notifications", "err", err, "update", u.ID)
		return
	}
	if n > 0 {
		s.processOutbox(ctx)
	}
}

// RunUpdateOutboxWorker sends pending notification emails until ctx is cancelled.
// It also publishes scheduled updates once they're due, even without a mailer,
// so they aren't all emailed late if one is configured afterwards.
func (s *Server) RunUpdateOutboxWorker(ctx context.Context) {
	if !s.updatesOn() {
		return
	}
	t := time.NewTicker(15 * time.Second)
	defer t.Stop()
	for {
		s.publishDueUpdates(ctx)
		if s.Mailer != nil {
			s.processOutbox(ctx)
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

// publishDueUpdates queues emails for scheduled updates whose time has come,
// skipping pages that no longer show updates.
func (s *Server) publishDueUpdates(ctx context.Context) {
	for {
		due, err := s.Store.ClaimDueUpdates(ctx, 20)
		if err != nil {
			slog.Error("claim scheduled updates", "err", err)
			return
		}
		if len(due) == 0 {
			return
		}
		for _, d := range due {
			if s.Mailer == nil {
				continue
			}
			a, err := s.Store.GetByID(ctx, d.PageID)
			if err != nil || !s.pageUpdatesLive(ctx, a) {
				continue
			}
			if _, err := s.Store.EnqueueUpdateNotifications(ctx, d.ID, d.PageID, d.ReadKey); err != nil {
				slog.Error("enqueue scheduled update", "err", err, "update", d.ID)
			}
		}
	}
}

func (s *Server) processOutbox(ctx context.Context) {
	for {
		items, err := s.Store.ClaimOutboxBatch(ctx, 20)
		if err != nil || len(items) == 0 {
			return
		}
		base := s.PublicBaseURL
		if base == "" {
			base = "http://localhost:8080"
		}
		product := s.Config.ProductNameOrDefault()
		for _, it := range items {
			readKey := it.ReadKey
			if readKey == "" {
				s.Store.MarkOutboxFailed(ctx, it.ID, "missing read key")
				continue
			}
			viewURL := base + "/" + readKey + ".html#update-" + fmt.Sprint(it.UpdateID)
			unsubURL := base + "/api/unsubscribe?token=" + it.UnsubscribeToken
			summary := excerpt(it.UpdateBody, 280)
			err := s.Mailer.SendUpdateNotification(ctx, it.Email, product, it.PageTitle, it.UpdateTitle, summary, viewURL, unsubURL)
			if err != nil {
				slog.Error("update notification", "err", err, "outbox", it.ID)
				s.Store.MarkOutboxFailed(ctx, it.ID, err.Error())
				continue
			}
			s.Store.MarkOutboxSent(ctx, it.ID)
		}
	}
}

func excerpt(body string, max int) string {
	body = strings.TrimSpace(body)
	if len(body) <= max {
		return body
	}
	return body[:max] + "…"
}

// injectUpdates adds feed HTML to a rendered page when enabled.
func (s *Server) injectUpdates(ctx context.Context, doc string, a *store.Access) string {
	if !s.updatesOn() || !s.pageUpdatesLive(ctx, a) {
		return doc
	}
	list, err := s.Store.ListPageUpdates(ctx, a.ID, 50)
	if err != nil {
		return doc
	}
	block, err := render.UpdatesBlock(a.ReadKey, list, s.Config.ProductNameOrDefault())
	if err != nil {
		slog.Error("render updates", "err", err)
		return doc
	}
	return render.WithUpdates(doc, block)
}

// NotifyPageEditSubscribers is used when UPDATES_MODE=notify_on_page_edit.
func (s *Server) NotifyPageEditSubscribers(ctx context.Context, a *store.Access, notify bool) {
	if !s.updatesOn() || !notify || s.Config.UpdatesFeed() {
		return
	}
	if !s.pageUpdatesLive(ctx, a) {
		return
	}
	title := a.Title
	if title == "" {
		title = "Page updated"
	}
	u, err := s.Store.CreatePageUpdate(ctx, a.ID, nil, title, "The page has been updated.", a.ReadKey, time.Time{})
	if err != nil {
		slog.Error("notify page edit", "err", err)
		return
	}
	s.enqueueUpdateEmails(ctx, u, a.ReadKey)
}
