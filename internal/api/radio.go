package api

import (
	"net/http"
	"net/url"
	"strings"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"

	"rainy/internal/auth"
	"rainy/internal/model"
)

// Radio field limits.
const (
	maxRadioNameLen = 200
	maxRadioURLLen  = 2048
)

// routesRadio registers the /radios endpoints (docs/architecture/contract.md §7.4); mounted behind
// auth.RequireUser. Creating, updating and deleting stations requires an administrator.
func (a *API) routesRadio(r chi.Router) {
	r.Get("/radios", a.radioList)
	r.With(auth.RequireAdmin).Post("/radios", a.radioCreate)
	r.With(auth.RequireAdmin).Put("/radios/{id}", a.radioUpdate)
	r.With(auth.RequireAdmin).Delete("/radios/{id}", a.radioDelete)
}

type radioBody struct {
	Name        string `json:"name"`
	StreamURL   string `json:"streamUrl"`
	HomepageURL string `json:"homepageUrl"`
}

// validHTTPURL reports whether s is an absolute http(s) URL with a host.
func validHTTPURL(s string) bool {
	u, err := url.Parse(s)
	return err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
}

func (b *radioBody) validate() error {
	b.Name = strings.TrimSpace(b.Name)
	b.StreamURL = strings.TrimSpace(b.StreamURL)
	b.HomepageURL = strings.TrimSpace(b.HomepageURL)
	switch {
	case b.Name == "":
		return badRequest("name is required")
	case utf8.RuneCountInString(b.Name) > maxRadioNameLen:
		return badRequest("name must be at most %d characters", maxRadioNameLen)
	case len(b.StreamURL) > maxRadioURLLen || !validHTTPURL(b.StreamURL):
		return badRequest("streamUrl must be an http(s) URL")
	case b.HomepageURL != "" && (len(b.HomepageURL) > maxRadioURLLen || !validHTTPURL(b.HomepageURL)):
		return badRequest("homepageUrl must be an http(s) URL")
	}
	return nil
}

func (a *API) radioList(w http.ResponseWriter, r *http.Request) {
	stations, err := a.app.Store.ListRadioStations(r.Context())
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, nonNil(stations))
}

func (a *API) radioCreate(w http.ResponseWriter, r *http.Request) {
	var body radioBody
	if err := decodeJSON(w, r, &body); err != nil {
		writeErr(w, r, err)
		return
	}
	if err := body.validate(); err != nil {
		writeErr(w, r, err)
		return
	}
	st := &model.RadioStation{Name: body.Name, StreamURL: body.StreamURL, HomepageURL: body.HomepageURL}
	if err := a.app.Store.CreateRadioStation(r.Context(), st); err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, st)
}

func (a *API) radioUpdate(w http.ResponseWriter, r *http.Request) {
	var body radioBody
	if err := decodeJSON(w, r, &body); err != nil {
		writeErr(w, r, err)
		return
	}
	if err := body.validate(); err != nil {
		writeErr(w, r, err)
		return
	}
	ctx := r.Context()
	st, err := a.app.Store.GetRadioStation(ctx, chi.URLParam(r, "id"))
	if err != nil {
		writeErr(w, r, err)
		return
	}
	st.Name, st.StreamURL, st.HomepageURL = body.Name, body.StreamURL, body.HomepageURL
	if err := a.app.Store.UpdateRadioStation(ctx, st); err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, st)
}

func (a *API) radioDelete(w http.ResponseWriter, r *http.Request) {
	if err := a.app.Store.DeleteRadioStation(r.Context(), chi.URLParam(r, "id")); err != nil {
		writeErr(w, r, err)
		return
	}
	writeNoContent(w)
}
