package api

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"rainy/internal/lxmusic"
	"rainy/internal/manage"
	"rainy/internal/model"
	"rainy/internal/store"
)

// Online music: lx-music source scripts (administrators, contract §7.7 "sources") and search
// + download into a library (managers, §7.6 "online"). Everything that contacts third-party
// services answers 403 unless an administrator turned on settings.lxSourcesEnabled. Source
// scripts are never returned by any endpoint.
//
// Owner: manage agent (D).

// maxScriptBody bounds POST /admin/sources (a script of lxmusic.MaxScriptSize bytes,
// JSON-escaped).
const maxScriptBody = 2*lxmusic.MaxScriptSize + 1<<20

func lxSelection(s model.Settings) lxmusic.Selection {
	return lxmusic.Selection{Mode: s.LxSourceMode, SourceID: s.LxSourceID}
}

// requireLx writes the 403 and returns false while online music is turned off.
func (a *API) requireLx(w http.ResponseWriter, r *http.Request) bool {
	if !a.app.Settings(r.Context()).LxSourcesEnabled {
		writeErr(w, r, forbidden("online music is turned off; an administrator can enable it in Settings → Sources"))
		return false
	}
	return true
}

// writeLxErr maps lxmusic errors: invalid input 400, failed online requests 503.
func writeLxErr(w http.ResponseWriter, r *http.Request, err error) {
	msg := err.Error()
	for _, e := range []error{lxmusic.ErrInvalid, lxmusic.ErrUpstream, lxmusic.ErrScript, lxmusic.ErrUnavailable} {
		msg = strings.TrimPrefix(msg, e.Error()+": ")
	}
	switch {
	case errors.Is(err, lxmusic.ErrInvalid), errors.Is(err, lxmusic.ErrBlocked):
		writeErr(w, r, badRequest("%s", msg))
	case errors.Is(err, lxmusic.ErrNotFound):
		writeErr(w, r, notFound("%s", msg))
	case errors.Is(err, lxmusic.ErrUpstream), errors.Is(err, lxmusic.ErrScript), errors.Is(err, lxmusic.ErrUnavailable):
		writeErr(w, r, newError(http.StatusServiceUnavailable, CodeUnavailable, "%s", msg))
	default:
		writeErr(w, r, err)
	}
}

// ---- managers (registered by routesManage)

type onlinePlatform struct {
	ID        string   `json:"id"`
	Qualities []string `json:"qualities"` // what the selected sources are known to provide
}

type onlineStatus struct {
	Enabled   bool             `json:"enabled"`
	Sources   int              `json:"sources"` // usable sources (enabled; in fixed mode the chosen one)
	Platforms []onlinePlatform `json:"platforms"`
}

func (a *API) manageOnlineStatus(w http.ResponseWriter, r *http.Request) {
	set := a.app.Settings(r.Context())
	sel := lxSelection(set)
	avail, err := a.app.Online.Available(r.Context(), sel)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	usable, err := a.app.Online.Usable(r.Context(), sel)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	st := onlineStatus{Enabled: set.LxSourcesEnabled, Sources: usable, Platforms: []onlinePlatform{}}
	for _, p := range lxmusic.Platforms {
		st.Platforms = append(st.Platforms, onlinePlatform{ID: p, Qualities: nonNil(avail[p])})
	}
	writeJSON(w, http.StatusOK, st)
}

func (a *API) manageOnlineSearch(w http.ResponseWriter, r *http.Request) {
	if !a.requireLx(w, r) {
		return
	}
	q := r.URL.Query()
	res, err := a.app.Online.Search(r.Context(), q.Get("platform"), q.Get("q"), queryInt(r, "page", 1), queryInt(r, "limit", lxmusic.DefaultLimit))
	if err != nil {
		writeLxErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

// manageOnlineCover proxies a search result's cover from the catalogue's image host, so the
// browser does not contact third-party hosts.
func (a *API) manageOnlineCover(w http.ResponseWriter, r *http.Request) {
	if !a.requireLx(w, r) {
		return
	}
	data, ctype, err := a.app.Metadata.Cover(r.Context(), r.URL.Query().Get("url"))
	if err != nil {
		writeMetadataErr(w, r, err)
		return
	}
	h := w.Header()
	h.Set("Content-Type", pictureContentType(ctype))
	h.Set("Content-Length", strconv.Itoa(len(data)))
	h.Set("Cache-Control", "private, max-age=86400")
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Content-Security-Policy", "default-src 'none'; sandbox")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func (a *API) manageOnlineDownload(w http.ResponseWriter, r *http.Request) {
	if !a.requireLx(w, r) {
		return
	}
	var body manage.OnlineDownloadRequest
	if err := decodeJSON(w, r, &body); err != nil {
		writeErr(w, r, err)
		return
	}
	jobs, err := a.app.Manage.StartOnlineDownload(r.Context(), userFrom(r), body)
	if err != nil {
		writeManageErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"jobs": jobs})
}

// ---- administrators (registered by routesAdmin)

type sourcesInfo struct {
	Enabled  bool                 `json:"enabled"`
	Mode     string               `json:"mode"`
	SourceID string               `json:"sourceId"`
	Sources  []lxmusic.SourceInfo `json:"sources"`
}

func (a *API) sourcesInfo(ctx context.Context) (*sourcesInfo, error) {
	set := a.app.Settings(ctx)
	list, err := a.app.Online.List(ctx)
	if err != nil {
		return nil, err
	}
	return &sourcesInfo{Enabled: set.LxSourcesEnabled, Mode: set.LxSourceMode, SourceID: set.LxSourceID, Sources: list}, nil
}

func (a *API) adminSources(w http.ResponseWriter, r *http.Request) {
	info, err := a.sourcesInfo(r.Context())
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, info)
}

// adminImportSource imports a script sent as text, or downloaded from a link (which
// contacts that server, so it needs the setting). While the setting is on, the script is
// started right away and the result shows whether it works.
func (a *API) adminImportSource(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Script string `json:"script"`
		URL    string `json:"url"`
	}
	if err := decodeJSONLimit(w, r, &body, maxScriptBody); err != nil {
		writeErr(w, r, err)
		return
	}
	enabled := a.app.Settings(r.Context()).LxSourcesEnabled
	script, sourceURL := body.Script, ""
	switch {
	case strings.TrimSpace(body.URL) != "" && body.Script != "":
		writeErr(w, r, badRequest("send either a script or a link"))
		return
	case strings.TrimSpace(body.URL) != "":
		if !enabled {
			writeErr(w, r, forbidden("online music is turned off; enable it before importing from a link"))
			return
		}
		sourceURL = strings.TrimSpace(body.URL)
		text, err := a.app.Online.FetchScript(r.Context(), sourceURL)
		if err != nil {
			writeLxErr(w, r, err)
			return
		}
		script = text
	case script == "":
		writeErr(w, r, badRequest("send a script or a link"))
		return
	}
	info, err := a.app.Online.Import(r.Context(), script, sourceURL, enabled)
	if err != nil {
		if errors.Is(err, store.ErrConflict) {
			writeErr(w, r, newError(http.StatusConflict, CodeConflict, "this script is already imported"))
			return
		}
		writeLxErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, info)
}

func (a *API) adminUpdateSource(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Enabled          *bool `json:"enabled"`
		AllowUpdateAlert *bool `json:"allowUpdateAlert"`
	}
	if err := decodeJSON(w, r, &body); err != nil {
		writeErr(w, r, err)
		return
	}
	info, err := a.app.Online.Update(r.Context(), chi.URLParam(r, "id"), body.Enabled, body.AllowUpdateAlert)
	if err != nil {
		writeLxErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, info)
}

func (a *API) adminDeleteSource(w http.ResponseWriter, r *http.Request) {
	if err := a.app.Online.Delete(r.Context(), chi.URLParam(r, "id")); err != nil {
		writeLxErr(w, r, err)
		return
	}
	writeNoContent(w)
}

func (a *API) adminReorderSources(w http.ResponseWriter, r *http.Request) {
	var body struct {
		IDs []string `json:"ids"`
	}
	if err := decodeJSON(w, r, &body); err != nil {
		writeErr(w, r, err)
		return
	}
	if err := a.app.Online.Reorder(r.Context(), body.IDs); err != nil {
		writeErr(w, r, err)
		return
	}
	a.adminSources(w, r)
}

// adminReloadSource (re)starts a script (it may contact its servers).
func (a *API) adminReloadSource(w http.ResponseWriter, r *http.Request) {
	if !a.requireLx(w, r) {
		return
	}
	info, err := a.app.Online.Reload(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		writeLxErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, info)
}

// adminRefreshSource downloads the script again from the link it was imported from.
func (a *API) adminRefreshSource(w http.ResponseWriter, r *http.Request) {
	if !a.requireLx(w, r) {
		return
	}
	info, err := a.app.Online.Refresh(r.Context(), chi.URLParam(r, "id"), true)
	if err != nil {
		if errors.Is(err, store.ErrConflict) {
			writeErr(w, r, newError(http.StatusConflict, CodeConflict, "the new version is already imported as another source"))
			return
		}
		writeLxErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, info)
}
