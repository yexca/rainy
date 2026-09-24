package api

import (
	"sync"
	"time"
)

// failureLimiter locks a key after too many failures within a window (in memory). It
// guards PUT /api/me/password; login has its own limiter in the auth package.
type failureLimiter struct {
	mu      sync.Mutex
	max     int
	window  time.Duration
	lockout time.Duration
	now     func() time.Time
	entries map[string]*failureEntry
}

type failureEntry struct {
	fails       int
	windowStart time.Time
	lockedUntil time.Time
}

// passwordChangeLimiter: 5 wrong current passwords within 15 minutes lock password changes
// of that user for 15 minutes.
var passwordChangeLimiter = newFailureLimiter(5, 15*time.Minute, 15*time.Minute)

func newFailureLimiter(max int, window, lockout time.Duration) *failureLimiter {
	return &failureLimiter{max: max, window: window, lockout: lockout, now: time.Now, entries: map[string]*failureEntry{}}
}

// locked reports whether key is currently locked out.
func (l *failureLimiter) locked(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	e, ok := l.entries[key]
	return ok && l.now().Before(e.lockedUntil)
}

// fail records a failure for key, locking it once max failures fall within the window.
func (l *failureLimiter) fail(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	l.sweep(now)
	e, ok := l.entries[key]
	if !ok || now.Sub(e.windowStart) > l.window {
		e = &failureEntry{windowStart: now}
		l.entries[key] = e
	}
	e.fails++
	if e.fails >= l.max {
		e.lockedUntil = now.Add(l.lockout)
		e.fails, e.windowStart = 0, now
	}
}

// reset forgets key's failures (after a success).
func (l *failureLimiter) reset(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.entries, key)
}

// sweep drops expired entries so the map stays small; called with mu held.
func (l *failureLimiter) sweep(now time.Time) {
	for k, e := range l.entries {
		if now.Sub(e.windowStart) > l.window && !now.Before(e.lockedUntil) {
			delete(l.entries, k)
		}
	}
}
