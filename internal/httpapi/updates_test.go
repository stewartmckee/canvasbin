package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"testing/fstest"

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
	cfg := config.Config{Updates: true, UpdatesMode: config.UpdatesModeFeed, MailgunKey: "x", MailgunDomain: "t", MailFrom: "t <t@t>"}
	s := &Server{
		Store: st, Config: cfg, Mailer: m,
		Web:  fstest.MapFS{"index.html": {Data: []byte("<html></html>")}},
		Live: live.New(10),
	}
	srv := httptest.NewServer(s.Handler())
	t.Cleanup(srv.Close)

	a, err := st.Create(context.Background(), "Site", "html", "<p>x</p>", false)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.SetUpdatesEnabled(context.Background(), a.ID, true); err != nil {
		t.Fatal(err)
	}
	body, _ := json.Marshal(map[string]string{"email": "fan@example.com"})
	res, err := http.Post(srv.URL+"/api/pages/"+a.ReadKey+"/subscribe", "application/json", bytes.NewReader(body))
	if err != nil || res.StatusCode != 200 {
		t.Fatalf("subscribe %d %v", res.StatusCode, err)
	}
	link := m.get("subconfirm", "fan@example.com")
	if link == "" {
		t.Fatal("no confirm email")
	}
	res2, err := http.Get(link)
	if err != nil || res2.StatusCode != 200 {
		t.Fatalf("confirm %d", res2.StatusCode)
	}
	postBody, _ := json.Marshal(map[string]string{"title": "News", "body": "Details"})
	res3, err := http.Post(srv.URL+"/api/pages/"+a.WriteKey+"/updates", "application/json", bytes.NewReader(postBody))
	if err != nil || res3.StatusCode != 201 {
		t.Fatalf("post update %d", res3.StatusCode)
	}
	s.processOutbox(context.Background())
	if m.get("notify", "fan@example.com") == "" {
		t.Fatal("expected notification email")
	}
}
