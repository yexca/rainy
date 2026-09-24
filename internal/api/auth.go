package api

import (
	"errors"
	"net/http"
	"net/mail"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"

	"rainy/internal/auth"
	"rainy/internal/buildinfo"
	"rainy/internal/model"
)

// Limits for profile fields.
const (
	maxDisplayNameLen = 100
	maxEmailLen       = 254
)

// setupMu serialises first-run setup so two concurrent requests cannot both create an
// administrator.
var setupMu sync.Mutex

// routesAuth registers the auth & me endpoints (docs/architecture/contract.md §7.1): public
// /auth/status, /auth/setup, /auth/login and /auth/logout; authenticated /me/*.
func (a *API) routesAuth(r chi.Router) {
	r.Get("/auth/status", a.authStatus)
	r.Post("/auth/setup", a.authSetup)
	r.Post("/auth/login", a.authLogin)
	// Logout is public so an expired session can still clear its cookie.
	r.Post("/auth/logout", a.authLogout)

	r.Group(func(r chi.Router) {
		r.Use(auth.RequireUser)
		r.Get("/me", a.meGet)
		r.Put("/me", a.meUpdate)
		r.Put("/me/password", a.meChangePassword)
		r.Post("/me/apikey", a.meCreateAPIKey)
		r.Delete("/me/apikey", a.meDeleteAPIKey)
	})
}

type authStatusResponse struct {
	Initialized bool        `json:"initialized"`
	User        *model.User `json:"user"`
	Version     string      `json:"version"`
}

func (a *API) authStatus(w http.ResponseWriter, r *http.Request) {
	n, err := a.app.Store.CountUsers(r.Context())
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, authStatusResponse{Initialized: n > 0, User: userFrom(r), Version: buildinfo.Version})
}

type loginBody struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (c *loginBody) validate() error {
	c.Username = strings.TrimSpace(c.Username)
	if c.Username == "" || c.Password == "" {
		return badRequest("username and password are required")
	}
	return nil
}

// authSetup creates the first (administrator) account and logs it in. It is only
// available while no user exists.
func (a *API) authSetup(w http.ResponseWriter, r *http.Request) {
	var body loginBody
	if err := decodeJSON(w, r, &body); err != nil {
		writeErr(w, r, err)
		return
	}
	if err := body.validate(); err != nil {
		writeErr(w, r, err)
		return
	}
	ctx := r.Context()

	setupMu.Lock()
	defer setupMu.Unlock()
	n, err := a.app.Store.CountUsers(ctx)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	if n > 0 {
		writeErr(w, r, forbidden("the server is already set up; sign in instead"))
		return
	}
	u := &model.User{Username: body.Username, IsAdmin: true, CanManage: true, CanDownload: true}
	if err := a.app.Auth.CreateUser(ctx, u, body.Password); err != nil {
		writeErr(w, r, err)
		return
	}
	token, err := a.app.Auth.CreateSession(ctx, u.ID, r.UserAgent(), auth.ClientIP(r))
	if err != nil {
		writeErr(w, r, err)
		return
	}
	_ = a.app.Store.TouchUserLogin(ctx, u.ID)
	if fresh, err := a.app.Store.GetUser(ctx, u.ID); err == nil {
		u = fresh
	}
	auth.SetSessionCookie(w, r, token, a.app.Auth.SessionTTL())
	writeJSON(w, http.StatusCreated, u)
}

func (a *API) authLogin(w http.ResponseWriter, r *http.Request) {
	var body loginBody
	if err := decodeJSON(w, r, &body); err != nil {
		writeErr(w, r, err)
		return
	}
	if err := body.validate(); err != nil {
		writeErr(w, r, err)
		return
	}
	token, u, err := a.app.Auth.Login(r.Context(), body.Username, body.Password, r.UserAgent(), auth.ClientIP(r))
	if err != nil {
		if errors.Is(err, auth.ErrInvalidCredentials) {
			err = newError(http.StatusUnauthorized, CodeUnauthorized, "wrong username or password")
		}
		writeErr(w, r, err)
		return
	}
	auth.SetSessionCookie(w, r, token, a.app.Auth.SessionTTL())
	writeJSON(w, http.StatusOK, u)
}

func (a *API) authLogout(w http.ResponseWriter, r *http.Request) {
	if token, _ := auth.RequestToken(r); token != "" {
		if err := a.app.Auth.Logout(r.Context(), token); err != nil {
			writeErr(w, r, err)
			return
		}
	}
	auth.ClearSessionCookie(w, r)
	writeNoContent(w)
}

// freshMe re-reads the authenticated user from the store (fresh flags and timestamps).
func (a *API) freshMe(r *http.Request) (*model.User, error) {
	return a.app.Store.GetUser(r.Context(), userFrom(r).ID)
}

func (a *API) meGet(w http.ResponseWriter, r *http.Request) {
	u, err := a.freshMe(r)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, u)
}

type meUpdateBody struct {
	DisplayName *string `json:"displayName"`
	Email       *string `json:"email"`
}

func (a *API) meUpdate(w http.ResponseWriter, r *http.Request) {
	var body meUpdateBody
	if err := decodeJSON(w, r, &body); err != nil {
		writeErr(w, r, err)
		return
	}
	u, err := a.freshMe(r)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	if body.DisplayName != nil {
		name := strings.TrimSpace(*body.DisplayName)
		if utf8.RuneCountInString(name) > maxDisplayNameLen {
			writeErr(w, r, badRequest("display name must be at most %d characters", maxDisplayNameLen))
			return
		}
		u.DisplayName = name
	}
	if body.Email != nil {
		email := strings.TrimSpace(*body.Email)
		if email != "" {
			if len(email) > maxEmailLen {
				writeErr(w, r, badRequest("email must be at most %d characters", maxEmailLen))
				return
			}
			if addr, err := mail.ParseAddress(email); err != nil || addr.Address != email {
				writeErr(w, r, badRequest("invalid email address"))
				return
			}
		}
		u.Email = email
	}
	ctx := r.Context()
	if err := a.app.Store.UpdateUser(ctx, u); err != nil {
		writeErr(w, r, err)
		return
	}
	if u, err = a.app.Store.GetUser(ctx, u.ID); err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, u)
}

type changePasswordBody struct {
	CurrentPassword string `json:"currentPassword"`
	NewPassword     string `json:"newPassword"`
}

func (a *API) meChangePassword(w http.ResponseWriter, r *http.Request) {
	var body changePasswordBody
	if err := decodeJSON(w, r, &body); err != nil {
		writeErr(w, r, err)
		return
	}
	u, err := a.freshMe(r)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	// A stolen session must not be usable to brute-force the current password.
	if passwordChangeLimiter.locked(u.ID) {
		writeErr(w, r, newError(http.StatusTooManyRequests, CodeRateLimited, "too many wrong passwords; try again later"))
		return
	}
	if !a.app.Auth.CheckPassword(u, body.CurrentPassword) {
		passwordChangeLimiter.fail(u.ID)
		writeErr(w, r, forbidden("the current password is incorrect"))
		return
	}
	passwordChangeLimiter.reset(u.ID)
	if err := a.app.Auth.ChangePassword(r.Context(), u.ID, body.NewPassword); err != nil {
		writeErr(w, r, err)
		return
	}
	writeNoContent(w)
}

func (a *API) meCreateAPIKey(w http.ResponseWriter, r *http.Request) {
	key, err := a.app.Auth.GenerateAPIKey(r.Context(), userFrom(r).ID)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"apiKey": key})
}

func (a *API) meDeleteAPIKey(w http.ResponseWriter, r *http.Request) {
	if err := a.app.Auth.RevokeAPIKey(r.Context(), userFrom(r).ID); err != nil {
		writeErr(w, r, err)
		return
	}
	writeNoContent(w)
}
