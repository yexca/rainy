package auth

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"rainy/internal/model"
)

type ctxKey struct{}

// WithUser returns a context carrying u.
func WithUser(ctx context.Context, u *model.User) context.Context {
	return context.WithValue(ctx, ctxKey{}, u)
}

// UserFrom returns the authenticated user of ctx, or nil.
func UserFrom(ctx context.Context) *model.User {
	u, _ := ctx.Value(ctxKey{}).(*model.User)
	return u
}

// RequestToken returns the session token of r: the rainy_session cookie, else an
// "Authorization: Bearer <token>" header. fromCookie reports the source.
func RequestToken(r *http.Request) (token string, fromCookie bool) {
	if c, err := r.Cookie(CookieName); err == nil && c.Value != "" {
		return c.Value, true
	}
	if h := r.Header.Get("Authorization"); len(h) > 7 && strings.EqualFold(h[:7], "bearer ") {
		return strings.TrimSpace(h[7:]), false
	}
	return "", false
}

// Middleware resolves the current user from the session cookie or a bearer token (a
// session token or an API key) and stores it in the request context. It never rejects a
// request; use RequireUser / RequireManager / RequireAdmin for that. When a cookie session's
// sliding expiry is extended the cookie is refreshed too.
func (s *Service) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, fromCookie := RequestToken(r)
		if token == "" {
			next.ServeHTTP(w, r)
			return
		}
		u, touched, err := s.sessionUser(r.Context(), token)
		if err != nil && !fromCookie {
			u, err = s.UserByAPIKey(r.Context(), token)
		}
		if err != nil {
			if !errors.Is(err, ErrInvalidCredentials) {
				slog.Error("auth: resolving session", "err", err)
			}
			next.ServeHTTP(w, r)
			return
		}
		if touched && fromCookie {
			SetSessionCookie(w, r, token, s.ttl)
		}
		next.ServeHTTP(w, r.WithContext(WithUser(r.Context(), u)))
	})
}

// RequireUser answers 401 unless the request has an authenticated user.
func RequireUser(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if UserFrom(r.Context()) == nil {
			writeError(w, http.StatusUnauthorized, "unauthorized", "authentication required")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// RequireManager answers 401 without a user and 403 unless the user may edit the library.
func RequireManager(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u := UserFrom(r.Context())
		switch {
		case u == nil:
			writeError(w, http.StatusUnauthorized, "unauthorized", "authentication required")
		case !u.CanEdit():
			writeError(w, http.StatusForbidden, "forbidden", "library management permission required")
		default:
			next.ServeHTTP(w, r)
		}
	})
}

// RequireAdmin answers 401 without a user and 403 unless the user is an administrator.
func RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u := UserFrom(r.Context())
		switch {
		case u == nil:
			writeError(w, http.StatusUnauthorized, "unauthorized", "authentication required")
		case !u.IsAdmin:
			writeError(w, http.StatusForbidden, "forbidden", "administrator permission required")
		default:
			next.ServeHTTP(w, r)
		}
	})
}

// isSecure reports whether the client connection is HTTPS (directly or via a proxy).
func isSecure(r *http.Request) bool {
	return r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
}

// SetSessionCookie sets the session cookie: HttpOnly, SameSite=Lax, Path=/, Secure when
// the request arrived over HTTPS.
func SetSessionCookie(w http.ResponseWriter, r *http.Request, token string, ttl time.Duration) {
	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    token,
		Path:     "/",
		MaxAge:   int(ttl / time.Second),
		Expires:  time.Now().Add(ttl),
		HttpOnly: true,
		Secure:   isSecure(r),
		SameSite: http.SameSiteLaxMode,
	})
}

// ClearSessionCookie deletes the session cookie.
func ClearSessionCookie(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		Expires:  time.Unix(0, 0),
		HttpOnly: true,
		Secure:   isSecure(r),
		SameSite: http.SameSiteLaxMode,
	})
}

// ClientIP returns the client IP of r (r.RemoteAddr without port; the server's RealIP
// middleware rewrites RemoteAddr from proxy headers when RAINY_TRUST_PROXY is enabled).
func ClientIP(r *http.Request) string {
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}
	return r.RemoteAddr
}

// writeError writes the native API error envelope.
func writeError(w http.ResponseWriter, status int, code, msg string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]string{"code": code, "message": msg}})
}
