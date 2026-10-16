// Package live fans page change notifications out to connected browsers.
package live

import (
	"errors"
	"sync"
	"time"
)

// Event kinds sent to subscribers.
const (
	Updated = "updated"
	Deleted = "deleted"
	// Resync means changes may have been missed; subscribers should recheck.
	Resync = "resync"
)

type Event struct {
	Type      string
	UpdatedAt time.Time
}

var ErrFull = errors.New("too many live connections")

// Hub tracks subscribers per page id.
type Hub struct {
	mu   sync.Mutex
	subs map[int64]map[chan Event]struct{}
	n    int
	max  int
}

// New returns a hub that accepts at most max subscribers in total.
func New(max int) *Hub {
	return &Hub{subs: map[int64]map[chan Event]struct{}{}, max: max}
}

// Subscribe registers interest in a page. Call cancel when done.
func (h *Hub) Subscribe(pageID int64) (events <-chan Event, cancel func(), err error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.n >= h.max {
		return nil, nil, ErrFull
	}
	ch := make(chan Event, 4)
	if h.subs[pageID] == nil {
		h.subs[pageID] = map[chan Event]struct{}{}
	}
	h.subs[pageID][ch] = struct{}{}
	h.n++
	var once sync.Once
	return ch, func() {
		once.Do(func() {
			h.mu.Lock()
			defer h.mu.Unlock()
			delete(h.subs[pageID], ch)
			if len(h.subs[pageID]) == 0 {
				delete(h.subs, pageID)
			}
			h.n--
		})
	}, nil
}

// Publish sends ev to every subscriber of pageID without blocking. A
// subscriber with a full buffer already has an event pending, which is enough
// for it to refresh.
func (h *Hub) Publish(pageID int64, ev Event) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for ch := range h.subs[pageID] {
		select {
		case ch <- ev:
		default:
		}
	}
}

// PublishAll sends ev to every subscriber of every page.
func (h *Hub) PublishAll(ev Event) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, set := range h.subs {
		for ch := range set {
			select {
			case ch <- ev:
			default:
			}
		}
	}
}

// Count reports the number of current subscribers.
func (h *Hub) Count() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.n
}
