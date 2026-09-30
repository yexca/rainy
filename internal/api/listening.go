package api

import (
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"rainy/internal/listening"
	"rainy/internal/model"
	"rainy/internal/scrobble"
)

// routesListening registers the listening report, the play history and the user's
// scrobbling accounts (docs/architecture/contract.md §7.8); mounted behind auth.RequireUser.
// Everything here only ever reads or changes the signed-in user's own data.
func (a *API) routesListening(r chi.Router) {
	r.Get("/listening/report", a.listeningReport)
	r.Get("/listening/history", a.listeningHistory)
	r.Get("/me/scrobbling", a.scrobblingStatus)
	r.Post("/me/scrobbling/lastfm/auth", a.scrobblingLastfmAuth)
	r.Post("/me/scrobbling/lastfm", a.scrobblingLastfmLink)
	r.Post("/me/scrobbling/listenbrainz", a.scrobblingListenBrainzLink)
	r.Put("/me/scrobbling/{service}", a.scrobblingUpdate)
	r.Delete("/me/scrobbling/{service}", a.scrobblingUnlink)
}

func (a *API) listeningReport(w http.ResponseWriter, r *http.Request) {
	q := listening.Query{
		From:  queryInt64(r, "from", 0),
		To:    queryInt64(r, "to", 0),
		Loc:   listening.LoadLocation(strings.TrimSpace(r.URL.Query().Get("tz"))),
		Limit: queryInt(r, "limit", listening.DefaultLimit),
	}
	rep, err := a.app.Listening.Report(r.Context(), userFrom(r).ID, q)
	if errors.Is(err, listening.ErrInvalid) {
		err = badRequest("%s", err.Error())
	}
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, rep)
}

func (a *API) listeningHistory(w http.ResponseWriter, r *http.Request) {
	offset, limit, _, _ := pageParams(r, 50, 500)
	from, to := queryInt64(r, "from", 0), queryInt64(r, "to", 0)
	if from < 0 || to < 0 || (to > 0 && to <= from) {
		writeErr(w, r, badRequest("from must be before to"))
		return
	}
	plays, total, err := a.app.Store.ListPlays(r.Context(), userFrom(r).ID, from, to, offset, limit)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writePage[model.Play](w, plays, total)
}

// scrobbleError maps scrobble errors to API errors. Messages from the services are their
// own text (never credentials).
func scrobbleError(err error) error {
	var se *scrobble.Error
	switch {
	case errors.As(err, &se):
		if se.Retryable() {
			return newError(http.StatusServiceUnavailable, CodeUnavailable, "%s", se.Error())
		}
		return badRequest("%s", se.Error())
	case errors.Is(err, scrobble.ErrDisabled):
		return forbidden("%s", err.Error())
	case errors.Is(err, scrobble.ErrNotConfigured):
		return newError(http.StatusConflict, CodeConflict, "%s", err.Error())
	case errors.Is(err, scrobble.ErrNotLinked):
		return notFound("%s", err.Error())
	case errors.Is(err, scrobble.ErrInvalid), errors.Is(err, scrobble.ErrExpired):
		return badRequest("%s", err.Error())
	}
	return err
}

func (a *API) scrobblingStatus(w http.ResponseWriter, r *http.Request) {
	st, err := a.app.Scrobble.Status(r.Context(), userFrom(r).ID)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, st)
}

func (a *API) scrobblingLastfmAuth(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Callback string `json:"callback"`
	}
	if err := decodeJSON(w, r, &body); err != nil {
		writeErr(w, r, err)
		return
	}
	u, err := a.app.Scrobble.LastfmAuthURL(r.Context(), userFrom(r).ID, body.Callback)
	if err != nil {
		writeErr(w, r, scrobbleError(err))
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"url": u})
}

func (a *API) scrobblingLastfmLink(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Token string `json:"token"`
		State string `json:"state"`
	}
	if err := decodeJSON(w, r, &body); err != nil {
		writeErr(w, r, err)
		return
	}
	st, err := a.app.Scrobble.LinkLastfm(r.Context(), userFrom(r).ID, strings.TrimSpace(body.Token), strings.TrimSpace(body.State))
	if err != nil {
		writeErr(w, r, scrobbleError(err))
		return
	}
	writeJSON(w, http.StatusOK, st)
}

func (a *API) scrobblingListenBrainzLink(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Token string `json:"token"`
	}
	if err := decodeJSON(w, r, &body); err != nil {
		writeErr(w, r, err)
		return
	}
	st, err := a.app.Scrobble.LinkListenBrainz(r.Context(), userFrom(r).ID, body.Token)
	if err != nil {
		writeErr(w, r, scrobbleError(err))
		return
	}
	writeJSON(w, http.StatusOK, st)
}

func (a *API) scrobblingUpdate(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Enabled *bool `json:"enabled"`
	}
	if err := decodeJSON(w, r, &body); err != nil {
		writeErr(w, r, err)
		return
	}
	if body.Enabled == nil {
		writeErr(w, r, badRequest("enabled is required"))
		return
	}
	st, err := a.app.Scrobble.SetEnabled(r.Context(), userFrom(r).ID, chi.URLParam(r, "service"), *body.Enabled)
	if err != nil {
		writeErr(w, r, scrobbleError(err))
		return
	}
	writeJSON(w, http.StatusOK, st)
}

func (a *API) scrobblingUnlink(w http.ResponseWriter, r *http.Request) {
	if err := a.app.Scrobble.Unlink(r.Context(), userFrom(r).ID, chi.URLParam(r, "service")); err != nil {
		writeErr(w, r, scrobbleError(err))
		return
	}
	writeNoContent(w)
}

// ---- administration (mounted under /admin by routesAdmin)

func (a *API) adminScrobbling(w http.ResponseWriter, r *http.Request) {
	info, err := a.app.Scrobble.AdminInfo(r.Context())
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, info)
}

// adminScrobblingLastfm stores the Last.fm API key and/or shared secret (omitted =
// unchanged, "" = remove). The secret is never returned.
func (a *API) adminScrobblingLastfm(w http.ResponseWriter, r *http.Request) {
	var body struct {
		APIKey *string `json:"apiKey"`
		Secret *string `json:"secret"`
	}
	if err := decodeJSON(w, r, &body); err != nil {
		writeErr(w, r, err)
		return
	}
	if err := a.app.Scrobble.SetLastfmCredentials(r.Context(), body.APIKey, body.Secret); err != nil {
		writeErr(w, r, scrobbleError(err))
		return
	}
	a.adminScrobbling(w, r)
}
