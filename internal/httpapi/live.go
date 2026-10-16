package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"

	"github.com/stewartmckee/canvasbin/internal/live"
	"github.com/stewartmckee/canvasbin/internal/store"
)

const livePingInterval = 25 * time.Second

type liveMessage struct {
	Type      string `json:"type"`
	UpdatedAt string `json:"updated_at,omitempty"`
}

func liveTime(t time.Time) string { return t.UTC().Format(time.RFC3339Nano) }

// RunLive forwards database page change notifications to the live hub until
// ctx is done.
func (s *Server) RunLive(ctx context.Context) {
	if s.Live == nil {
		return
	}
	s.Store.ListenPageChanges(ctx, func(c store.PageChange) {
		if c.Deleted {
			s.Live.Publish(c.ID, live.Event{Type: live.Deleted})
			return
		}
		s.Live.Publish(c.ID, live.Event{Type: live.Updated, UpdatedAt: c.UpdatedAt})
	}, func() { s.Live.PublishAll(live.Event{Type: live.Resync}) })
}

// liveUpdates streams change events for one page over a WebSocket. Any valid
// key may subscribe; events carry no content, only that the page changed.
func (s *Server) liveUpdates(w http.ResponseWriter, r *http.Request) {
	if s.Live == nil {
		writeError(w, http.StatusNotFound, "live updates are not available")
		return
	}
	key, ok := pathKey(w, r)
	if !ok {
		return
	}
	a, err := s.Store.Get(r.Context(), key)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	events, cancel, err := s.Live.Subscribe(a.ID)
	if errors.Is(err, live.ErrFull) {
		writeError(w, http.StatusServiceUnavailable, err.Error())
		return
	}
	defer cancel()

	// The server's read/write timeouts would otherwise cut the socket off.
	rc := http.NewResponseController(w)
	_ = rc.SetReadDeadline(time.Time{})
	_ = rc.SetWriteDeadline(time.Time{})
	// Rendered pages run in an opaque-origin sandbox and send Origin: null, so
	// origins cannot be checked. Safe because no cookies or session are used.
	c, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
	if err != nil {
		return
	}
	defer c.CloseNow()
	c.SetReadLimit(512)
	ctx := c.CloseRead(r.Context())

	send := func(m liveMessage) error {
		wctx, done := context.WithTimeout(ctx, 10*time.Second)
		defer done()
		return wsjson.Write(wctx, c, m)
	}
	last := a.UpdatedAt
	if send(liveMessage{Type: "hello", UpdatedAt: liveTime(last)}) != nil {
		return
	}
	ping := time.NewTicker(livePingInterval)
	defer ping.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ping.C:
			pctx, done := context.WithTimeout(ctx, 10*time.Second)
			err := c.Ping(pctx)
			done()
			if err != nil {
				return
			}
		case ev := <-events:
			if ev.Type == live.Resync {
				t, err := s.Store.PageUpdatedAt(ctx, a.ID)
				switch {
				case errors.Is(err, store.ErrNotFound):
					ev.Type = live.Deleted
				case err != nil:
					slog.Error("live resync", "err", err)
					continue
				case t.Equal(last):
					continue
				default:
					ev = live.Event{Type: live.Updated, UpdatedAt: t}
				}
			}
			if ev.Type == live.Deleted {
				_ = send(liveMessage{Type: live.Deleted})
				c.Close(websocket.StatusNormalClosure, "page deleted")
				return
			}
			last = ev.UpdatedAt
			if send(liveMessage{Type: live.Updated, UpdatedAt: liveTime(last)}) != nil {
				return
			}
		}
	}
}
