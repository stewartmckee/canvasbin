package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/stewartmckee/canvasbin/internal/config"
	"github.com/stewartmckee/canvasbin/internal/live"
	"github.com/stewartmckee/canvasbin/internal/store"
)

func (f *fakeMailer) SendUpdateSubscribeConfirm(_ context.Context, e, l, _, _ string) error {
	return f.put("subconfirm", e, l)
}
func (f *fakeMailer) SendUpdateNotification(_ context.Context, e, _, _, _, _, view, unsub string) error {
	return f.put("notify", e, view+"|"+unsub)
}

func TestUpdatesDisabled(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	st, err := store.Open(context.Background(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(st.Close)
	s := &Server{
		Store: st, Config: config.Config{Updates: false},
		Web:  fstest.MapFS{"index.html": {Data: []byte("<html></html>")}},
		Live: live.New(10),
	}
	srv := httptest.NewServer(s.Handler())
	t.Cleanup(srv.Close)
	res, _ := http.Get(srv.URL + "/api/pages/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa/subscribe")
	if res.StatusCode != http.StatusNotFound {
		t.Fatalf("status %d", res.StatusCode)
	}
}

func TestUpdatesSubscribeAndPost(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	st, err := store.Open(context.Background(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(st.Close)
	m := &fakeMailer{links: map[string]string{}}
	cfg := config.Config{Updates: true, UpdatesMode: config.UpdatesModeFeed, MailgunKey: "x", MailgunDomain: "t", MailFrom: "t <t@t>",
		StripePricePro: "price_pro", StripePriceSignal: "price_signal"}
	s := &Server{
		Store: st, Config: cfg, Mailer: m,
		Web:  fstest.MapFS{"index.html": {Data: []byte("<html></html>")}},
		Live: live.New(10),
	}
	srv := httptest.NewServer(s.Handler())
	t.Cleanup(srv.Close)
	ctx := context.Background()

	owner := func(price string) int64 {
		u, err := st.CreateUser(ctx, fmt.Sprintf("%s-%d@example.com", price, time.Now().UnixNano()), "hash", "Owner")
		if err != nil {
			t.Fatal(err)
		}
		if err := st.UpdateSubscription(ctx, u.ID, store.Subscription{CustomerID: "cus_" + price + fmt.Sprint(u.ID), SubscriptionID: "sub_" + fmt.Sprint(u.ID), PriceID: price, Status: "active"}); err != nil {
			t.Fatal(err)
		}
		return u.ID
	}
	enable := func(writeKey string) int {
		b, _ := json.Marshal(map[string]bool{"updates_enabled": true})
		req, _ := http.NewRequest(http.MethodPatch, srv.URL+"/api/pages/"+writeKey+"/updates-settings", bytes.NewReader(b))
		req.Header.Set("Content-Type", "application/json")
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		return res.StatusCode
	}

	pro, err := st.CreateOwned(ctx, owner("price_pro"), "Pro", "html", "<p>x</p>", false)
	if err != nil {
		t.Fatal(err)
	}
	if code := enable(pro.WriteKey); code != http.StatusPaymentRequired {
		t.Fatalf("individual owner enabling updates: status %d, want 402", code)
	}
	anon, err := st.Create(ctx, "Anon", "html", "<p>x</p>", false)
	if err != nil {
		t.Fatal(err)
	}
	if code := enable(anon.WriteKey); code != http.StatusPaymentRequired {
		t.Fatalf("anonymous page enabling updates: status %d, want 402", code)
	}

	a, err := st.CreateOwned(ctx, owner("price_signal"), "Site", "html", "<p>x</p>", false)
	if err != nil {
		t.Fatal(err)
	}
	if code := enable(a.WriteKey); code != http.StatusOK {
		t.Fatalf("signal owner enabling updates: status %d", code)
	}
	pre, _ := http.NewRequest(http.MethodOptions, srv.URL+"/api/pages/"+a.ReadKey+"/subscribe", nil)
	pre.Header.Set("Origin", "null")
	pre.Header.Set("Access-Control-Request-Method", "POST")
	pre.Header.Set("Access-Control-Request-Headers", "content-type")
	pres, err := http.DefaultClient.Do(pre)
	if err != nil || pres.StatusCode != http.StatusNoContent || pres.Header.Get("Access-Control-Allow-Origin") != "*" {
		t.Fatalf("subscribe preflight from a sandboxed page should be allowed: %v %v", pres, err)
	}
	bad, _ := json.Marshal(map[string]string{"email": "not-an-email"})
	if res, err := http.Post(srv.URL+"/api/pages/"+a.ReadKey+"/subscribe", "application/json", bytes.NewReader(bad)); err != nil || res.StatusCode != http.StatusBadRequest {
		t.Fatalf("invalid email should be rejected: %v %v", res, err)
	}
	body, _ := json.Marshal(map[string]string{"email": "fan@example.com"})
	res, err := http.Post(srv.URL+"/api/pages/"+a.ReadKey+"/subscribe", "application/json", bytes.NewReader(body))
	if err != nil || res.StatusCode != 200 || res.Header.Get("Access-Control-Allow-Origin") != "*" {
		t.Fatalf("subscribe %d %v", res.StatusCode, err)
	}
	link := m.get("subconfirm", "fan@example.com")
	if link == "" {
		t.Fatal("no confirm email")
	}
	page := func(url string, status int, want ...string) {
		t.Helper()
		res, err := http.Get(url)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		b, _ := io.ReadAll(res.Body)
		if res.StatusCode != status {
			t.Fatalf("GET %s: status %d, want %d", url, res.StatusCode, status)
		}
		for _, w := range want {
			if !strings.Contains(string(b), w) {
				t.Errorf("GET %s: page missing %q", url, w)
			}
		}
	}
	page(link, 200, "You’re subscribed", "<strong>Site</strong>", "fan@example.com", "/"+a.ReadKey+".html")
	page(link, 200, "You’re subscribed")
	page(srv.URL+"/api/subscribe/confirm?token=nope", 400, "isn’t valid")
	postBody, _ := json.Marshal(map[string]string{"title": "News", "body": "Details"})
	res3, err := http.Post(srv.URL+"/api/pages/"+a.WriteKey+"/updates", "application/json", bytes.NewReader(postBody))
	if err != nil || res3.StatusCode != 201 {
		t.Fatalf("post update %d", res3.StatusCode)
	}
	s.processOutbox(context.Background())
	sent := m.get("notify", "fan@example.com")
	if sent == "" {
		t.Fatal("expected notification email")
	}
	unsub := sent[strings.Index(sent, "/api/unsubscribe"):]
	page(srv.URL+unsub, 200, "You’re unsubscribed", "<strong>Site</strong>")
}
