package store

import "sync"

// Broadcaster fans out ToolCall events to any number of dashboard SSE
// subscribers. Sends are non-blocking — a slow subscriber drops events
// rather than back-pressuring the proxy hot path.
//
// One Broadcaster is owned by *Store and notified from the batched writer
// after every successful flush; the dashboard's SSE handler calls Subscribe
// per connection.
type Broadcaster struct {
	mu   sync.RWMutex
	subs map[chan ToolCall]struct{}
}

// NewBroadcaster returns an empty broadcaster.
func NewBroadcaster() *Broadcaster {
	return &Broadcaster{subs: make(map[chan ToolCall]struct{})}
}

// Subscribe returns a channel that receives every subsequent ToolCall
// insert. Buffer size controls the slow-subscriber tolerance — events that
// arrive while the channel is full are dropped.
//
// Callers MUST eventually call Unsubscribe; the returned chan is closed
// inside Unsubscribe.
func (b *Broadcaster) Subscribe(buf int) chan ToolCall {
	if buf <= 0 {
		buf = 64
	}
	ch := make(chan ToolCall, buf)
	b.mu.Lock()
	b.subs[ch] = struct{}{}
	b.mu.Unlock()
	return ch
}

// Unsubscribe removes ch and closes it. Safe to call multiple times.
func (b *Broadcaster) Unsubscribe(ch chan ToolCall) {
	b.mu.Lock()
	if _, ok := b.subs[ch]; ok {
		delete(b.subs, ch)
		close(ch)
	}
	b.mu.Unlock()
}

// Publish fans out one ToolCall to every subscriber. Non-blocking — a
// subscriber whose buffer is full simply misses the event.
func (b *Broadcaster) Publish(tc ToolCall) {
	b.mu.RLock()
	for ch := range b.subs {
		select {
		case ch <- tc:
		default:
		}
	}
	b.mu.RUnlock()
}

// Count returns the number of active subscribers (useful for diagnostics
// and the dashboard's own /api/stats handler).
func (b *Broadcaster) Count() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.subs)
}
