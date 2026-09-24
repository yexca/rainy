package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"rainy/internal/auth"
	"rainy/internal/model"
)

// sessionCookie returns the rainy_session cookie set by rec (nil if none).
func nativeSessionCookie(rec *httptest.ResponseRecorder) *http.Cookie {
	for _, c := range rec.Result().Cookies() {
		if c.Name == auth.CookieName {
			return c
		}
	}
	return nil
}

func nativeWithCookie(req *http.Request, c *http.Cookie) *http.Request {
	req.AddCookie(&http.Cookie{Name: c.Name, Value: c.Value})
	return req
}

func TestNativeAuthSetupAndStatus(t *testing.T) {
	a, _ := newNativeApp(t)
	h := New(a).Routes()
	serve := func(req *http.Request) *httptest.ResponseRecorder { return nativeServe(t, h, req) }

	st := nativeJSON[authStatusResponse](t, serve(nativeRequest(t, "GET", "/auth/status", "", nil)), 200)
	if st.Initialized || st.User != nil || st.Version == "" {
		t.Fatalf("status before setup: %+v", st)
	}
	// Protected routes need a user.
	nativeErrorCode(t, serve(nativeRequest(t, "GET", "/me", "", nil)), 401)

	// Validation.
	nativeErrorCode(t, serve(nativeRequest(t, "POST", "/auth/setup", "", map[string]string{"username": "root"})), 400)
	nativeErrorCode(t, serve(nativeRequest(t, "POST", "/auth/setup", "", map[string]string{"username": "root", "password": "123"})), 400)
	nativeErrorCode(t, serve(nativeRequest(t, "POST", "/auth/setup", "", map[string]string{"username": "bad name!", "password": "1234"})), 400)
	nativeErrorCode(t, serve(nativeRequest(t, "POST", "/auth/setup", "", "{nope")), 400)

	rec := serve(nativeRequest(t, "POST", "/auth/setup", "", map[string]string{"username": " root ", "password": "s3cret!"}))
	u := nativeJSON[model.User](t, rec, 201)
	if u.Username != "root" || !u.IsAdmin || !u.CanManage || !u.CanDownload || u.LastLoginAt == 0 {
		t.Fatalf("setup user %+v", u)
	}
	c := nativeSessionCookie(rec)
	if c == nil || c.Value == "" || !c.HttpOnly || c.SameSite != http.SameSiteLaxMode || c.Path != "/" {
		t.Fatalf("session cookie %+v", c)
	}
	if n, _ := a.Store.CountUsers(t.Context()); n != 1 {
		t.Fatal("exactly one user expected")
	}

	// The cookie authenticates.
	st = nativeJSON[authStatusResponse](t, serve(nativeWithCookie(nativeRequest(t, "GET", "/auth/status", "", nil), c)), 200)
	if !st.Initialized || st.User == nil || st.User.ID != u.ID {
		t.Fatalf("status after setup: %+v", st)
	}
	me := nativeJSON[model.User](t, serve(nativeWithCookie(nativeRequest(t, "GET", "/me", "", nil), c)), 200)
	if me.ID != u.ID {
		t.Fatal("me")
	}

	// Setup is closed once a user exists.
	code := nativeErrorCode(t, serve(nativeRequest(t, "POST", "/auth/setup", "", map[string]string{"username": "evil", "password": "whatever"})), 403)
	if code != CodeForbidden {
		t.Fatalf("code %s", code)
	}
	// Anonymous status still works.
	st = nativeJSON[authStatusResponse](t, serve(nativeRequest(t, "GET", "/auth/status", "", nil)), 200)
	if !st.Initialized || st.User != nil {
		t.Fatalf("anonymous status: %+v", st)
	}
}

func TestNativeAuthLoginLogout(t *testing.T) {
	e := newNativeEnv(t)
	login := func(user, pass string) *httptest.ResponseRecorder {
		return e.do("POST", "/auth/login", "", map[string]string{"username": user, "password": pass})
	}
	nativeErrorCode(t, login("alice", ""), 400)
	if code := nativeErrorCode(t, login("alice", "wrong"), 401); code != CodeUnauthorized {
		t.Fatal(code)
	}
	nativeErrorCode(t, login("nobody", "wrong"), 401)

	rec := login("alice", "secret-alice")
	u := nativeJSON[model.User](t, rec, 200)
	c := nativeSessionCookie(rec)
	if u.ID != e.alice.ID || c == nil || u.LastLoginAt == 0 {
		t.Fatalf("login: %+v %+v", u, c)
	}
	nativeExpect(t, nativeServe(t, e.h, nativeWithCookie(nativeRequest(t, "GET", "/me", "", nil), c)), 200)

	// Logout deletes the session and clears the cookie.
	rec = nativeServe(t, e.h, nativeWithCookie(nativeRequest(t, "POST", "/auth/logout", "", nil), c))
	nativeExpect(t, rec, 204)
	if cleared := nativeSessionCookie(rec); cleared == nil || cleared.MaxAge >= 0 {
		t.Fatalf("cookie not cleared: %+v", cleared)
	}
	nativeErrorCode(t, nativeServe(t, e.h, nativeWithCookie(nativeRequest(t, "GET", "/me", "", nil), c)), 401)
	// Logging out without a session is harmless.
	nativeExpect(t, e.do("POST", "/auth/logout", "", nil), 204)
}

func TestNativeAuthRateLimit(t *testing.T) {
	e := newNativeEnv(t)
	for i := range 10 {
		rec := e.do("POST", "/auth/login", "", map[string]string{"username": "bob", "password": "nope"})
		if rec.Code != 401 {
			t.Fatalf("attempt %d: %d", i, rec.Code)
		}
	}
	rec := e.do("POST", "/auth/login", "", map[string]string{"username": "bob", "password": "secret-bob"})
	if code := nativeErrorCode(t, rec, 429); code != CodeRateLimited {
		t.Fatal(code)
	}
}

func TestNativeMeProfileAndPassword(t *testing.T) {
	e := newNativeEnv(t)
	u := nativeJSON[model.User](t, e.do("PUT", "/me", e.aliceTok, map[string]string{"displayName": "  Alice A. ", "email": "alice@example.com"}), 200)
	if u.DisplayName != "Alice A." || u.Email != "alice@example.com" || u.CanManage {
		t.Fatalf("profile %+v", u)
	}
	// Omitted fields are kept; flags cannot be changed through /me.
	u = nativeJSON[model.User](t, e.do("PUT", "/me", e.aliceTok, `{"isAdmin":true,"canManage":true}`), 200)
	if u.DisplayName != "Alice A." || u.IsAdmin || u.CanManage {
		t.Fatalf("profile after no-op %+v", u)
	}
	nativeErrorCode(t, e.do("PUT", "/me", e.aliceTok, map[string]string{"email": "not an email"}), 400)
	nativeErrorCode(t, e.do("PUT", "/me", e.aliceTok, map[string]string{"email": "Alice <a@b.c>"}), 400)
	u = nativeJSON[model.User](t, e.do("PUT", "/me", e.aliceTok, map[string]string{"email": ""}), 200)
	if u.Email != "" {
		t.Fatal("email not cleared")
	}

	// Password change.
	pw := func(cur, next string) *httptest.ResponseRecorder {
		return e.do("PUT", "/me/password", e.aliceTok, map[string]string{"currentPassword": cur, "newPassword": next})
	}
	nativeErrorCode(t, pw("wrong", "new-password"), 403)
	nativeErrorCode(t, pw("secret-alice", "abc"), 400)
	nativeExpect(t, pw("secret-alice", "new-password"), 204)
	// The current session survives; the new password works.
	nativeExpect(t, e.do("GET", "/me", e.aliceTok, nil), 200)
	nativeExpect(t, e.do("POST", "/auth/login", "", map[string]string{"username": "alice", "password": "new-password"}), 200)
	nativeErrorCode(t, e.do("POST", "/auth/login", "", map[string]string{"username": "alice", "password": "secret-alice"}), 401)
}

func TestNativeMeAPIKey(t *testing.T) {
	e := newNativeEnv(t)
	nativeErrorCode(t, e.do("POST", "/me/apikey", "", nil), 401)
	key := nativeJSON[map[string]string](t, e.do("POST", "/me/apikey", e.bobTok, nil), 200)["apiKey"]
	if len(key) < 20 {
		t.Fatalf("key %q", key)
	}
	me := nativeJSON[model.User](t, e.do("GET", "/me", key, nil), 200)
	if me.ID != e.bob.ID || !me.HasAPIKey {
		t.Fatalf("api key user %+v", me)
	}
	nativeExpect(t, e.do("DELETE", "/me/apikey", e.bobTok, nil), 204)
	nativeErrorCode(t, e.do("GET", "/me", key, nil), 401)
	if me = nativeJSON[model.User](t, e.do("GET", "/me", e.bobTok, nil), 200); me.HasAPIKey {
		t.Fatal("key still reported")
	}
}

func TestNativeMePasswordRateLimit(t *testing.T) {
	e := newNativeEnv(t)
	pw := func(tok, cur string) *httptest.ResponseRecorder {
		return e.do("PUT", "/me/password", tok, map[string]string{"currentPassword": cur, "newPassword": "new-password"})
	}
	for i := 0; i < 5; i++ {
		nativeErrorCode(t, pw(e.bobTok, "wrong"), 403)
	}
	// Locked now, even with the right password; other users are unaffected.
	if code := nativeErrorCode(t, pw(e.bobTok, "secret-bob"), 429); code != CodeRateLimited {
		t.Fatal(code)
	}
	nativeExpect(t, pw(e.aliceTok, "secret-alice"), 204)
}

func TestFailureLimiter(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	l := newFailureLimiter(3, time.Minute, 5*time.Minute)
	l.now = func() time.Time { return now }

	l.fail("k")
	l.fail("k")
	if l.locked("k") {
		t.Fatal("locked too early")
	}
	// Failures outside the window start a new count.
	now = now.Add(2 * time.Minute)
	l.fail("k")
	l.fail("k")
	if l.locked("k") {
		t.Fatal("old failures must expire")
	}
	l.fail("k")
	if !l.locked("k") || l.locked("other") {
		t.Fatal("lockout")
	}
	now = now.Add(5*time.Minute + time.Second)
	if l.locked("k") {
		t.Fatal("lockout must expire")
	}
	l.fail("k")
	l.reset("k")
	l.fail("k")
	l.fail("k")
	if l.locked("k") {
		t.Fatal("reset must clear failures")
	}
	// Expired entries are swept.
	now = now.Add(time.Hour)
	l.fail("x")
	if len(l.entries) != 1 {
		t.Fatalf("entries not swept: %d", len(l.entries))
	}
}
