package handlers

import (
	"sync"
	"time"
)

// NotificationEvent is the single wire shape used by the SSE stream, the web
// push payload and the FCM data block.
type NotificationEvent struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"`
	Title     string    `json:"title"`
	Body      string    `json:"body"`
	URL       string    `json:"url"`
	PostID    int64     `json:"post_id,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// hubBufferSize is how many undelivered events one subscriber may accumulate
// before new events are dropped. A slow or stalled browser connection must
// never grow memory without bound or block a publisher.
const hubBufferSize = 8

// NotificationHub fans events out to live SSE subscribers keyed by user ID.
// Publishing is non-blocking: a subscriber whose buffer is full simply misses
// the event (the post is still visible in the app).
type NotificationHub struct {
	mu      sync.RWMutex
	clients map[int64]map[chan NotificationEvent]struct{}
}

var (
	globalHub     *NotificationHub
	globalHubOnce sync.Once
)

// GlobalNotificationHub returns the process-wide hub used by the HTTP layer.
func GlobalNotificationHub() *NotificationHub {
	globalHubOnce.Do(func() {
		globalHub = NewNotificationHub()
	})
	return globalHub
}

// NewNotificationHub builds an empty hub. Tests use this to stay isolated
// from the global instance.
func NewNotificationHub() *NotificationHub {
	return &NotificationHub{clients: make(map[int64]map[chan NotificationEvent]struct{})}
}

// Subscribe registers a listener for userID. The returned cancel function must
// be called when the listener goes away; it is safe to call more than once.
func (h *NotificationHub) Subscribe(userID int64) (<-chan NotificationEvent, func()) {
	ch := make(chan NotificationEvent, hubBufferSize)

	h.mu.Lock()
	if h.clients[userID] == nil {
		h.clients[userID] = make(map[chan NotificationEvent]struct{})
	}
	h.clients[userID][ch] = struct{}{}
	h.mu.Unlock()

	var once sync.Once
	cancel := func() {
		once.Do(func() {
			h.mu.Lock()
			if subs, ok := h.clients[userID]; ok {
				delete(subs, ch)
				if len(subs) == 0 {
					delete(h.clients, userID)
				}
			}
			h.mu.Unlock()
			// The channel is not closed: a publisher that already holds the
			// channel might race with a close. Dropping the reference is
			// enough because the subscriber stops reading and the garbage
			// collector reclaims it.
		})
	}
	return ch, cancel
}

// Publish delivers ev to every live subscriber of userID without blocking.
// When a subscriber's buffer is full the event is dropped for that
// subscriber only.
func (h *NotificationHub) Publish(userID int64, ev NotificationEvent) {
	h.mu.RLock()
	subs := h.clients[userID]
	channels := make([]chan NotificationEvent, 0, len(subs))
	for ch := range subs {
		channels = append(channels, ch)
	}
	h.mu.RUnlock()

	for _, ch := range channels {
		select {
		case ch <- ev:
		default:
		}
	}
}

// SubscriberCount reports how many live listeners a user has. Used by the
// admin test endpoint to be explicit about realtime delivery.
func (h *NotificationHub) SubscriberCount(userID int64) int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients[userID])
}
