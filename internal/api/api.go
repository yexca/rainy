// Package api is the native JSON API used by the web UI, mounted at /api
// (contract: docs/architecture/contract.md §7).
//
// Layout: this file assembles the router; helpers.go has the shared request/response
// helpers; every other file belongs to one wave-2 owner and defines a
// `func (a *API) routesX(r chi.Router)` for its area.
package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"rainy/internal/app"
	"rainy/internal/auth"
)

// API serves /api.
type API struct {
	app *app.App
}

// New creates the API.
func New(a *app.App) *API { return &API{app: a} }

// Routes returns the /api handler (paths are relative to the /api mount point).
func (a *API) Routes() http.Handler {
	r := chi.NewRouter()
	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		writeError(w, http.StatusNotFound, "not_found", "no such endpoint: "+r.Method+" "+r.URL.Path)
	})
	r.MethodNotAllowed(func(w http.ResponseWriter, r *http.Request) {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method "+r.Method+" not allowed for "+r.URL.Path)
	})
	r.Use(a.app.Auth.Middleware)

	// Public.
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	// The auth area registers its own public routes and wraps its authenticated ones
	// with auth.RequireUser itself.
	a.routesAuth(r)

	// Any authenticated user. All seven routesX functions below share this one group router,
	// so they must NOT call r.Use (chi panics when middleware is added after routes); wrap
	// individual routes with r.With(mw) or open a nested r.Group instead.
	r.Group(func(r chi.Router) {
		r.Use(auth.RequireUser)
		a.routesLibrary(r)
		a.routesAnnotations(r)
		a.routesPlaylists(r)
		a.routesQueue(r)
		a.routesMedia(r)
		a.routesRadio(r)
		a.routesEvents(r)
	})

	// Library managers (canManage or admin). Paths inside are relative: "/tracks/{id}/tags".
	// The guard is attached with With (not Use inside the sub-router) so it runs for every
	// request under the prefix, including unknown paths.
	r.With(auth.RequireManager).Route("/manage", a.routesManage)

	// Administrators. Paths inside are relative: "/users".
	r.With(auth.RequireAdmin).Route("/admin", a.routesAdmin)
	return r
}
