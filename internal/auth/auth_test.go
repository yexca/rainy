package auth

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"rainy/internal/db/dbtest"
	"rainy/internal/model"
	"rainy/internal/store"
)

var ctx = context.Background()

type clock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *clock) now() time.Time      { c.mu.Lock(); defer c.mu.Unlock(); return c.t }
func (c *clock) add(d time.Duration) { c.mu.Lock(); c.t = c.t.Add(d); c.mu.Unlock() }

func newService(t *testing.T) (*Service, *store.Store, *clock) {
	t.Helper()
	st := store.New(dbtest.New(t))
	key := bytes.Repeat([]byte{7}, KeySize)
	s := NewService(st, NewCrypto(key), time.Hour)
	c := &clock{t: time.Now()}
	s.now = c.now
	return s, st, c
}

func TestLoadOrCreateKey(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "secret.key")
	k1, err := LoadOrCreateKey(path)
	if err != nil || len(k1) != KeySize {
		t.Fatal(k1, err)
	}
	k2, err := LoadOrCreateKey(path)
	if err != nil || !bytes.Equal(k1, k2) {
		t.Fatal("key must be stable across loads")
	}
	if err := os.WriteFile(path, []byte("short"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadOrCreateKey(path); err == nil {
		t.Fatal("malformed key must fail")
	}
}

func TestCrypto(t *testing.T) {
	c := NewCrypto(bytes.Repeat([]byte{1}, KeySize))
	for _, plain := range []string{"", "pässwörd", "密码🎵", strings.Repeat("x", 1000)} {
		enc, err := c.Encrypt(plain)
		if err != nil {
			t.Fatal(err)
		}
		if enc == plain && plain != "" {
			t.Fatal("not encrypted")
		}
		got, err := c.Decrypt(enc)
		if err != nil || got != plain {
			t.Fatalf("round trip %q → %q %v", plain, got, err)
		}
	}
	a, _ := c.Encrypt("same")
	b, _ := c.Encrypt("same")
	if a == b {
		t.Fatal("nonce must be random")
	}
	other := NewCrypto(bytes.Repeat([]byte{2}, KeySize))
	if _, err := other.Decrypt(a); !errors.Is(err, ErrDecrypt) {
		t.Fatalf("wrong key must fail: %v", err)
	}
	for _, bad := range []string{"", "!!!", "YWJj"} {
		if _, err := c.Decrypt(bad); !errors.Is(err, ErrDecrypt) {
			t.Fatalf("Decrypt(%q) = %v", bad, err)
		}
	}
	// Non-32-byte keys are stretched, not rejected.
	short := NewCrypto([]byte("short"))
	enc, _ := short.Encrypt("x")
	if got, err := short.Decrypt(enc); err != nil || got != "x" {
		t.Fatal(err)
	}
}

func TestTokens(t *testing.T) {
	a, b := NewToken(), NewToken()
	if len(a) != 43 || a == b || strings.ContainsAny(a, "+/=") {
		t.Fatalf("token %q", a)
	}
	if h := HashToken("abc"); h != "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad" {
		t.Fatal(h)
	}
}

func TestValidation(t *testing.T) {
	for _, ok := range []string{"alice", "Bob_2", "a.b-c@d+e", "张三", "ユーザー"} {
		if err := ValidateUsername(ok); err != nil {
			t.Errorf("%q: %v", ok, err)
		}
	}
	for _, bad := range []string{"", "has space", "semi;colon", strings.Repeat("a", 65), "tab\t"} {
		if err := ValidateUsername(bad); !errors.Is(err, ErrValidation) {
			t.Errorf("%q should be invalid", bad)
		}
	}
	if err := ValidatePassword("abc"); !errors.Is(err, ErrWeakPassword) {
		t.Fatal(err)
	}
	if err := ValidatePassword("密码密码"); err != nil {
		t.Fatal(err)
	}
}

func TestCreateUserAndLogin(t *testing.T) {
	s, st, _ := newService(t)
	u := &model.User{Username: " alice ", IsAdmin: true}
	if err := s.CreateUser(ctx, u, "secret1"); err != nil {
		t.Fatal(err)
	}
	if u.Username != "alice" || u.PasswordEnc == "" || strings.Contains(u.PasswordEnc, "secret1") {
		t.Fatalf("%+v", u)
	}
	if err := s.CreateUser(ctx, &model.User{Username: "ALICE"}, "secret1"); !errors.Is(err, store.ErrConflict) {
		t.Fatal(err)
	}
	if err := s.CreateUser(ctx, &model.User{Username: "bob"}, "abc"); !errors.Is(err, ErrWeakPassword) {
		t.Fatal(err)
	}
	if !s.CheckPassword(u, "secret1") || s.CheckPassword(u, "secret2") || s.CheckPassword(nil, "x") {
		t.Fatal("CheckPassword")
	}
	if p, err := s.Password(u); err != nil || p != "secret1" {
		t.Fatal(p, err)
	}

	token, got, err := s.Login(ctx, "Alice", "secret1", "ua", "192.0.2.4")
	if err != nil || token == "" || got.ID != u.ID || got.LastLoginAt == 0 {
		t.Fatalf("login: %v %+v", err, got)
	}
	if _, _, err := s.Login(ctx, "alice", "wrong", "ua", "192.0.2.4"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatal(err)
	}
	if _, _, err := s.Login(ctx, "nobody", "secret1", "ua", "192.0.2.4"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatal(err)
	}
	su, err := s.SessionUser(ctx, token)
	if err != nil || su.ID != u.ID {
		t.Fatal(err)
	}
	if err := s.ChangePassword(ctx, u.ID, "newsecret"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SessionUser(ctx, token); err != nil {
		t.Fatal("password change must keep sessions")
	}
	if _, _, err := s.Login(ctx, "alice", "newsecret", "ua", "192.0.2.4"); err != nil {
		t.Fatal(err)
	}
	if err := s.Logout(ctx, token); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SessionUser(ctx, token); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatal("logged out session must be invalid")
	}
	_ = st
}

func TestLoginRateLimit(t *testing.T) {
	s, _, clk := newService(t)
	_ = s.CreateUser(ctx, &model.User{Username: "alice"}, "secret1")
	for i := 0; i < maxFailures; i++ {
		if _, _, err := s.Login(ctx, "alice", "bad", "", "198.51.100.1"); !errors.Is(err, ErrInvalidCredentials) {
			t.Fatalf("attempt %d: %v", i, err)
		}
	}
	// Locked: even the right password is refused, from any IP (username lock) ...
	if _, _, err := s.Login(ctx, "alice", "secret1", "", "198.51.100.2"); !errors.Is(err, ErrRateLimited) {
		t.Fatalf("want rate limited, got %v", err)
	}
	// ... and the IP is locked for other usernames too.
	if _, _, err := s.Login(ctx, "bob", "x", "", "198.51.100.1"); !errors.Is(err, ErrRateLimited) {
		t.Fatalf("ip lock: %v", err)
	}
	clk.add(lockoutPeriod + time.Second)
	if _, _, err := s.Login(ctx, "alice", "secret1", "", "198.51.100.1"); err != nil {
		t.Fatalf("after lockout: %v", err)
	}
	// Failures outside the window do not accumulate.
	for i := 0; i < maxFailures-1; i++ {
		_, _, _ = s.Login(ctx, "alice", "bad", "", "198.51.100.3")
	}
	clk.add(failureWindow + time.Second)
	if _, _, err := s.Login(ctx, "alice", "bad", "", "198.51.100.3"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("window reset: %v", err)
	}
	if _, _, err := s.Login(ctx, "alice", "secret1", "", "198.51.100.3"); err != nil {
		t.Fatalf("should not be locked: %v", err)
	}
}

func TestSessionExpiryAndSlidingTouch(t *testing.T) {
	s, st, clk := newService(t)
	u := &model.User{Username: "alice"}
	_ = s.CreateUser(ctx, u, "secret1")
	token, _, err := s.Login(ctx, "alice", "secret1", "", "203.0.113.1")
	if err != nil {
		t.Fatal(err)
	}
	hash := HashToken(token)
	orig, _ := st.GetSession(ctx, hash)

	// Within a minute: no write.
	clk.add(30 * time.Second)
	if _, touched, err := s.sessionUser(ctx, token); err != nil || touched {
		t.Fatalf("touched too early: %v %v", touched, err)
	}
	if cur, _ := st.GetSession(ctx, hash); cur.ExpiresAt != orig.ExpiresAt {
		t.Fatal("expiry must not change within a minute")
	}
	// After a minute: expiry slides.
	clk.add(31 * time.Second)
	if _, touched, err := s.sessionUser(ctx, token); err != nil || !touched {
		t.Fatalf("not touched: %v %v", touched, err)
	}
	slid, _ := st.GetSession(ctx, hash)
	if slid.ExpiresAt != clk.now().Add(time.Hour).UnixMilli() {
		t.Fatalf("expires %d", slid.ExpiresAt)
	}
	if _, touched, _ := s.sessionUser(ctx, token); touched {
		t.Fatal("second touch within a minute")
	}
	// Keep using it: it never expires while active.
	for i := 0; i < 3; i++ {
		clk.add(50 * time.Minute)
		if _, err := s.SessionUser(ctx, token); err != nil {
			t.Fatalf("active session expired at step %d: %v", i, err)
		}
	}
	// Idle past the TTL: expired and deleted.
	clk.add(time.Hour + time.Second)
	if _, err := s.SessionUser(ctx, token); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatal(err)
	}
	if _, err := st.GetSession(ctx, hash); !errors.Is(err, store.ErrNotFound) {
		t.Fatal("expired session must be deleted")
	}
	if _, err := s.SessionUser(ctx, ""); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatal(err)
	}
}

func TestAPIKey(t *testing.T) {
	s, st, _ := newService(t)
	u := &model.User{Username: "alice"}
	_ = s.CreateUser(ctx, u, "secret1")
	key, err := s.GenerateAPIKey(ctx, u.ID)
	if err != nil || key == "" {
		t.Fatal(err)
	}
	got, _ := st.GetUser(ctx, u.ID)
	if !got.HasAPIKey || *got.APIKeyHash == key {
		t.Fatal("only the hash may be stored")
	}
	if found, err := s.UserByAPIKey(ctx, key); err != nil || found.ID != u.ID {
		t.Fatal(err)
	}
	key2, _ := s.GenerateAPIKey(ctx, u.ID)
	if _, err := s.UserByAPIKey(ctx, key); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatal("old key must be replaced")
	}
	if err := s.RevokeAPIKey(ctx, u.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.UserByAPIKey(ctx, key2); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatal("revoked key")
	}
	if _, err := s.UserByAPIKey(ctx, ""); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatal("empty key")
	}
}

func TestMiddleware(t *testing.T) {
	s, _, clk := newService(t)
	admin := &model.User{Username: "admin", IsAdmin: true}
	plain := &model.User{Username: "plain"}
	_ = s.CreateUser(ctx, admin, "secret1")
	_ = s.CreateUser(ctx, plain, "secret1")
	adminTok, _, _ := s.Login(ctx, "admin", "secret1", "", "")
	plainTok, _, _ := s.Login(ctx, "plain", "secret1", "", "")
	apiKey, _ := s.GenerateAPIKey(ctx, plain.ID)

	echo := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if u := UserFrom(r.Context()); u != nil {
			_, _ = w.Write([]byte(u.Username))
		}
	})
	mux := http.NewServeMux()
	mux.Handle("/any", s.Middleware(echo))
	mux.Handle("/user", s.Middleware(RequireUser(echo)))
	mux.Handle("/manage", s.Middleware(RequireManager(echo)))
	mux.Handle("/admin", s.Middleware(RequireAdmin(echo)))

	do := func(path string, mod func(*http.Request)) *httptest.ResponseRecorder {
		r := httptest.NewRequest("GET", path, nil)
		if mod != nil {
			mod(r)
		}
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		return w
	}
	cookie := func(tok string) func(*http.Request) {
		return func(r *http.Request) { r.AddCookie(&http.Cookie{Name: CookieName, Value: tok}) }
	}
	bearer := func(tok string) func(*http.Request) {
		return func(r *http.Request) { r.Header.Set("Authorization", "Bearer "+tok) }
	}

	if w := do("/any", nil); w.Code != 200 || w.Body.String() != "" {
		t.Fatal("anonymous must pass through")
	}
	if w := do("/any", cookie("garbage")); w.Code != 200 || w.Body.String() != "" {
		t.Fatal("bad cookie must pass through anonymously")
	}
	if w := do("/user", nil); w.Code != 401 || !strings.Contains(w.Body.String(), `"code":"unauthorized"`) {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
	if w := do("/user", cookie(plainTok)); w.Code != 200 || w.Body.String() != "plain" {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
	if w := do("/user", bearer(adminTok)); w.Code != 200 || w.Body.String() != "admin" {
		t.Fatalf("bearer session: %d %s", w.Code, w.Body)
	}
	if w := do("/user", bearer(apiKey)); w.Code != 200 || w.Body.String() != "plain" {
		t.Fatalf("bearer api key: %d %s", w.Code, w.Body)
	}
	if w := do("/user", cookie(apiKey)); w.Code != 401 {
		t.Fatal("api keys are not accepted as cookies")
	}
	if w := do("/manage", cookie(plainTok)); w.Code != 403 || !strings.Contains(w.Body.String(), `"forbidden"`) {
		t.Fatalf("%d", w.Code)
	}
	if w := do("/manage", cookie(adminTok)); w.Code != 200 {
		t.Fatalf("%d", w.Code)
	}
	if w := do("/admin", cookie(plainTok)); w.Code != 403 {
		t.Fatalf("%d", w.Code)
	}
	if w := do("/admin", nil); w.Code != 401 {
		t.Fatalf("%d", w.Code)
	}
	// Sliding refresh re-sets the cookie.
	clk.add(2 * time.Minute)
	w := do("/user", cookie(plainTok))
	if sc := w.Header().Get("Set-Cookie"); !strings.Contains(sc, CookieName+"="+plainTok) || !strings.Contains(sc, "HttpOnly") ||
		!strings.Contains(sc, "SameSite=Lax") || strings.Contains(sc, "Secure") {
		t.Fatalf("refresh cookie: %q", sc)
	}
}

func TestCookies(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("X-Forwarded-Proto", "https")
	w := httptest.NewRecorder()
	SetSessionCookie(w, r, "tok", time.Hour)
	sc := w.Header().Get("Set-Cookie")
	if !strings.Contains(sc, "Secure") || !strings.Contains(sc, "Max-Age=3600") || !strings.Contains(sc, "Path=/") {
		t.Fatal(sc)
	}
	w = httptest.NewRecorder()
	ClearSessionCookie(w, r)
	if sc := w.Header().Get("Set-Cookie"); !strings.Contains(sc, "Max-Age=0") {
		t.Fatal(sc)
	}
	r.RemoteAddr = "[::1]:1234"
	if ClientIP(r) != "::1" {
		t.Fatal(ClientIP(r))
	}
}
