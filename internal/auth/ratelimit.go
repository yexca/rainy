package auth

import (
	"sync"
	"time"
)

// limiter is an in-memory failure counter: after max failures within window a key is
// locked for lockout. A successful login resets the key.
type limiter struct {
	mu        sync.Mutex
	now       func() time.Time
	max       int
	window    time.Duration
	lockout   time.Duration
	entries   map[string]*limitEntry
	lastSweep time.Time
}

type limitEntry struct {
	failures    int
	windowStart time.Time
	lockedUntil time.Time
}

func newLimiter(max int, window, lockout time.Duration, now func() time.Time) *limiter {
	return &limiter{max: max, window: window, lockout: lockout, now: now, entries: map[string]*limitEntry{}}
}

// locked reports whether key is currently locked out.
func (l *limiter) locked(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	e := l.entries[key]
	return e != nil && l.now().Before(e.lockedUntil)
}

// fail records a failure for key, locking it when the threshold is reached.
func (l *limiter) fail(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	l.sweep(now)
	e := l.entries[key]
	if e == nil {
		e = &limitEntry{windowStart: now}
		l.entries[key] = e
	}
	if now.Before(e.lockedUntil) {
		return
	}
	if now.Sub(e.windowStart) > l.window {
		e.failures, e.windowStart = 0, now
	}
	e.failures++
	if e.failures >= l.max {
		e.lockedUntil = now.Add(l.lockout)
		e.failures, e.windowStart = 0, now
	}
}

// reset forgets key.
func (l *limiter) reset(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.entries, key)
}

// sweep drops stale entries at most once per window (caller holds mu).
func (l *limiter) sweep(now time.Time) {
	if now.Sub(l.lastSweep) < l.window {
		return
	}
	l.lastSweep = now
	for k, e := range l.entries {
		if now.After(e.lockedUntil) && now.Sub(e.windowStart) > l.window {
			delete(l.entries, k)
		}
	}
}
