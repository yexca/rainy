package subsonic

import (
	"crypto/md5"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"log/slog"
	"net/url"
	"strings"
	"sync"
	"time"

	"rainy/internal/auth"
	"rainy/internal/model"
	"rainy/internal/store"
)

// hasCredentials reports whether any authentication parameter is present.
func hasCredentials(p url.Values) bool {
	for _, k := range []string{"u", "p", "t", "s", "apiKey"} {
		if p.Get(k) != "" {
			return true
		}
	}
	return false
}

// authenticate resolves the user from u+p (plain or "enc:<hex>"), u+t+s (token =
// md5(password+salt)) or apiKey (OpenSubsonic apiKeyAuthentication).
func (a *API) authenticate(q *request) (*model.User, *apiError) {
	p := q.params
	username, password, token, salt, apiKey := p.Get("u"), p.Get("p"), p.Get("t"), p.Get("s"), p.Get("apiKey")
	ip := auth.ClientIP(q.r)

	if apiKey != "" {
		if username != "" || password != "" || token != "" || salt != "" {
			return nil, newError(codeConflictingAuth, "Multiple conflicting authentication mechanisms provided")
		}
		if a.limiter.locked("ip:" + ip) {
			return nil, newError(codeInvalidAPIKey, "Too many failed attempts, try again later")
		}
		u, err := a.app.Auth.UserByAPIKey(q.ctx, apiKey)
		if err != nil {
			if !errors.Is(err, auth.ErrInvalidCredentials) {
				slog.Error("subsonic: api key lookup", "err", err)
				return nil, newError(codeGeneric, "Internal server error")
			}
			a.limiter.fail("ip:" + ip)
			return nil, newError(codeInvalidAPIKey, "Invalid API key")
		}
		return u, nil
	}

	if username == "" {
		return nil, errMissing("u")
	}
	if password != "" && token != "" {
		return nil, newError(codeConflictingAuth, "Multiple conflicting authentication mechanisms provided")
	}
	if password == "" && token == "" {
		return nil, errMissing("p or t")
	}
	if token != "" && salt == "" {
		return nil, errMissing("s")
	}
	if password != "" {
		dec, ok := decodePassword(password)
		if !ok {
			return nil, newError(codeWrongCredentials, "Wrong username or password")
		}
		password = dec
	}

	userKey, ipKey := "user:"+strings.ToLower(username), "ip:"+ip
	if a.limiter.locked(userKey) || a.limiter.locked(ipKey) {
		return nil, newError(codeWrongCredentials, "Too many failed login attempts, try again later")
	}
	u, err := a.app.Store.GetUserByUsername(q.ctx, username)
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		slog.Error("subsonic: user lookup", "err", err)
		return nil, newError(codeGeneric, "Internal server error")
	}
	ok := false
	if u != nil {
		if password != "" {
			ok = a.app.Auth.CheckPassword(u, password)
		} else if plain, err := a.app.Auth.Password(u); err == nil {
			ok = checkToken(plain, salt, token)
		}
	}
	if !ok {
		a.limiter.fail(userKey)
		a.limiter.fail(ipKey)
		return nil, newError(codeWrongCredentials, "Wrong username or password")
	}
	return u, nil
}

// decodePassword decodes a Subsonic "p" value: plain text or "enc:" + hex.
func decodePassword(p string) (string, bool) {
	if hexed, ok := strings.CutPrefix(p, "enc:"); ok {
		b, err := hex.DecodeString(hexed)
		if err != nil {
			return "", false
		}
		return string(b), true
	}
	return p, true
}

// checkToken verifies token == md5(password + salt) in constant time.
func checkToken(password, salt, token string) bool {
	sum := md5.Sum([]byte(password + salt))
	want := hex.EncodeToString(sum[:])
	return subtle.ConstantTimeCompare([]byte(want), []byte(strings.ToLower(token))) == 1
}

// ---- failure rate limiting

// Failed authentications: 20 failures within 10 minutes per IP or per username lock that
// key for 5 minutes (Subsonic clients retry stored credentials, so the threshold is higher
// than the web login's).
const (
	limitMaxFailures = 20
	limitWindow      = 10 * time.Minute
	limitLockout     = 5 * time.Minute
)

type limitEntry struct {
	failures    int
	windowStart time.Time
	lockedUntil time.Time
}

type limiter struct {
	mu        sync.Mutex
	now       func() time.Time
	entries   map[string]*limitEntry
	lastSweep time.Time
}

func newLimiter() *limiter {
	return &limiter{now: time.Now, entries: map[string]*limitEntry{}}
}

func (l *limiter) locked(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	e := l.entries[key]
	return e != nil && l.now().Before(e.lockedUntil)
}

func (l *limiter) fail(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	if now.Sub(l.lastSweep) > limitWindow {
		l.lastSweep = now
		for k, e := range l.entries {
			if now.After(e.lockedUntil) && now.Sub(e.windowStart) > limitWindow {
				delete(l.entries, k)
			}
		}
	}
	e := l.entries[key]
	if e == nil {
		e = &limitEntry{windowStart: now}
		l.entries[key] = e
	}
	if now.Before(e.lockedUntil) {
		return
	}
	if now.Sub(e.windowStart) > limitWindow {
		e.failures, e.windowStart = 0, now
	}
	e.failures++
	if e.failures >= limitMaxFailures {
		e.lockedUntil = now.Add(limitLockout)
		e.failures, e.windowStart = 0, now
	}
}
