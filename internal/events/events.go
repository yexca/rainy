// Package events is a small in-process publish/subscribe bus used to push server events
// (scan progress, library changes, now playing) to web clients over SSE.
package events

import "sync"

// Event types.
const (
	TypeScan       = "scan"       // Data: scanner.Status
	TypeLibrary    = "library"    // Data: LibraryData
	TypeNowPlaying = "nowPlaying" // Data: []nowplaying.Entry
)

// Event is one message; its JSON form is sent as the SSE data.
type Event struct {
	Type string `json:"type"`
	Data any    `json:"data"`
}

// LibraryData is the payload of a "library" event.
type LibraryData struct {
	Reason string `json:"reason"`
}

// Library returns a "library" event with the given reason (e.g. "scan", "tags", "upload").
func Library(reason string) Event { return Event{Type: TypeLibrary, Data: LibraryData{Reason: reason}} }

// SubscriberBuffer is the per-subscriber channel capacity; when it is full new events for
// that subscriber are dropped (slow consumers never block publishers).
const SubscriberBuffer = 32

// Bus fans events out to subscribers. The zero value is not usable; use NewBus.
type Bus struct {
	mu     sync.RWMutex
	nextID int
	subs   map[int]chan Event
}

// NewBus creates an empty bus.
func NewBus() *Bus { return &Bus{subs: map[int]chan Event{}} }

// Publish delivers e to every subscriber without blocking.
func (b *Bus) Publish(e Event) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	for _, ch := range b.subs {
		select {
		case ch <- e:
		default: // subscriber is slow: drop
		}
	}
}

// Subscribe returns a buffered channel of events and a cancel function that unsubscribes
// and closes the channel. cancel is idempotent.
func (b *Bus) Subscribe() (<-chan Event, func()) {
	ch := make(chan Event, SubscriberBuffer)
	b.mu.Lock()
	id := b.nextID
	b.nextID++
	b.subs[id] = ch
	b.mu.Unlock()
	var once sync.Once
	return ch, func() {
		once.Do(func() {
			b.mu.Lock()
			delete(b.subs, id)
			b.mu.Unlock()
			close(ch)
		})
	}
}

// Subscribers returns the number of active subscribers.
func (b *Bus) Subscribers() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.subs)
}
