package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"log/slog"
	"net/http"

	"rainy/internal/model"
	"rainy/internal/store"
)

// EnsureDevelopmentUser returns the account RAINY_MODE=development signs requests in as. A
// missing account is created as an administrator with a random password nobody knows; an
// existing one is used as it is, so a listener or manager account previews that role.
func (s *Service) EnsureDevelopmentUser(ctx context.Context, username string) (*model.User, error) {
	u, err := s.st.GetUserByUsername(ctx, username)
	if err == nil {
		return u, nil
	}
	if !errors.Is(err, store.ErrNotFound) {
		return nil, err
	}
	secret := make([]byte, 24)
	if _, err := rand.Read(secret); err != nil {
		return nil, err
	}
	u = &model.User{Username: username, IsAdmin: true, CanManage: true, CanDownload: true}
	if err := s.CreateUser(ctx, u, hex.EncodeToString(secret)); err != nil {
		return nil, err
	}
	return u, nil
}

// DevelopmentMiddleware signs requests that Middleware left anonymous (no valid session or
// API key) in as the user with userID, re-read on every request so role changes apply at
// once. A valid session still wins. Development mode only: it removes authentication.
func (s *Service) DevelopmentMiddleware(userID string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if UserFrom(r.Context()) == nil {
				u, err := s.st.GetUser(r.Context(), userID)
				if err != nil {
					slog.Error("auth: loading the development user", "err", err)
				} else {
					r = r.WithContext(WithUser(r.Context(), u))
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}
