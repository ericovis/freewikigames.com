package sse

import "sync"

// Hub manages SSE subscriber channels per game session.
// All methods are safe for concurrent use.
type Hub struct {
	mu          sync.Mutex
	subscribers map[int64]map[chan Event]struct{}
}

// NewHub creates a ready-to-use Hub.
func NewHub() *Hub {
	return &Hub{subscribers: make(map[int64]map[chan Event]struct{})}
}

// Subscribe registers a channel to receive events for the given session.
// The returned channel is buffered; callers must call Unsubscribe when done.
func (h *Hub) Subscribe(sessionID int64) chan Event {
	ch := make(chan Event, 16)
	h.mu.Lock()
	if h.subscribers[sessionID] == nil {
		h.subscribers[sessionID] = make(map[chan Event]struct{})
	}
	h.subscribers[sessionID][ch] = struct{}{}
	h.mu.Unlock()
	return ch
}

// Unsubscribe removes and closes the channel for the given session.
func (h *Hub) Unsubscribe(sessionID int64, ch chan Event) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if subs, ok := h.subscribers[sessionID]; ok {
		delete(subs, ch)
		if len(subs) == 0 {
			delete(h.subscribers, sessionID)
		}
	}
	close(ch)
}

// Broadcast sends an event to all current subscribers of the session.
// Slow subscribers are skipped (non-blocking send).
func (h *Hub) Broadcast(sessionID int64, event Event) {
	h.mu.Lock()
	subs := h.subscribers[sessionID]
	h.mu.Unlock()

	for ch := range subs {
		select {
		case ch <- event:
		default:
		}
	}
}
