// Package nowplaying tracks what users are currently playing (in memory).
package nowplaying

import (
	"sort"
	"sync"
	"time"
)

// TTL is how long an entry stays listed after its last update.
const TTL = 15 * time.Minute

// Entry is one user's current track on one player.
type Entry struct {
	UserID   string `json:"userId"`
	Username string `json:"username"`
	TrackID  string `json:"trackId"`
	Player   string `json:"player"` // client name, e.g. "web" or a Subsonic client id
	Since    int64  `json:"since"`  // unix ms when playback started / was reported
}

// Tracker holds the current entries, keyed by user and player.
type Tracker struct {
	mu      sync.Mutex
	entries map[string]Entry
	now     func() time.Time
}

// New creates an empty tracker.
func New() *Tracker { return &Tracker{entries: map[string]Entry{}, now: time.Now} }

func key(e Entry) string { return e.UserID + "\x00" + e.Player }

// Set records e, replacing the previous entry of the same user and player. Since defaults
// to now.
func (t *Tracker) Set(e Entry) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if e.Since == 0 {
		e.Since = t.now().UnixMilli()
	}
	t.entries[key(e)] = e
}

// Remove drops the entry of a user and player (e.g. when playback stops).
func (t *Tracker) Remove(userID, player string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.entries, key(Entry{UserID: userID, Player: player}))
}

// List returns the entries updated within TTL, newest first (never nil). Stale entries
// are dropped.
func (t *Tracker) List() []Entry {
	t.mu.Lock()
	defer t.mu.Unlock()
	cutoff := t.now().Add(-TTL).UnixMilli()
	out := make([]Entry, 0, len(t.entries))
	for k, e := range t.entries {
		if e.Since < cutoff {
			delete(t.entries, k)
			continue
		}
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Since != out[j].Since {
			return out[i].Since > out[j].Since
		}
		return key(out[i]) < key(out[j])
	})
	return out
}
