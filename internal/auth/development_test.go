package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"rainy/internal/model"
)

func TestEnsureDevelopmentUser(t *testing.T) {
	s, st, _ := newService(t)

	u, err := s.EnsureDevelopmentUser(ctx, "dev")
	if err != nil {
		t.Fatal(err)
	}
	if !u.IsAdmin || !u.CanManage || !u.CanDownload {
		t.Fatalf("a created development user must be an administrator: %+v", u)
	}
	again, err := s.EnsureDevelopmentUser(ctx, "dev")
	if err != nil || again.ID != u.ID {
		t.Fatalf("second call must return the same account: %v %+v", err, again)
	}
	if n, _ := st.CountUsers(ctx); n != 1 {
		t.Fatalf("users = %d, want 1", n)
	}

	listener := &model.User{Username: "listener"}
	if err := s.CreateUser(ctx, listener, "secret1"); err != nil {
		t.Fatal(err)
	}
	got, err := s.EnsureDevelopmentUser(ctx, "listener")
	if err != nil || got.ID != listener.ID || got.IsAdmin || got.CanManage {
		t.Fatalf("an existing account must be used unchanged: %v %+v", err, got)
	}

	if _, err := s.EnsureDevelopmentUser(ctx, "bad name"); err == nil {
		t.Fatal("an invalid username must fail")
	}
}

func TestDevelopmentMiddleware(t *testing.T) {
	s, _, _ := newService(t)
	dev, err := s.EnsureDevelopmentUser(ctx, "dev")
	if err != nil {
		t.Fatal(err)
	}
	other := &model.User{Username: "other"}
	_ = s.CreateUser(ctx, other, "secret1")
	otherTok, _, _ := s.Login(ctx, "other", "secret1", "", "")

	echo := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(UserFrom(r.Context()).Username))
	})
	h := s.Middleware(s.DevelopmentMiddleware(dev.ID)(RequireAdmin(echo)))
	do := func(mod func(*http.Request)) *httptest.ResponseRecorder {
		r := httptest.NewRequest("GET", "/admin", nil)
		if mod != nil {
			mod(r)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}

	if w := do(nil); w.Code != 200 || w.Body.String() != "dev" {
		t.Fatalf("anonymous request: %d %q, want the development user", w.Code, w.Body.String())
	}
	if w := do(func(r *http.Request) { r.AddCookie(&http.Cookie{Name: CookieName, Value: "garbage"}) }); w.Body.String() != "dev" {
		t.Fatalf("invalid session: %q, want the development user", w.Body.String())
	}
	// A valid session still decides who the user is.
	if w := do(func(r *http.Request) { r.AddCookie(&http.Cookie{Name: CookieName, Value: otherTok}) }); w.Code != 403 {
		t.Fatalf("session of a non-admin: %d, want 403", w.Code)
	}
}
