package api

import (
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"rainy/internal/manage"
	"rainy/internal/ytdlp"
)

// Downloads from YouTube and bilibili with yt-dlp (contract §7.6 and §7.7). Starting a
// download, checking for a yt-dlp update and installing it send requests to third-party
// services, so they answer 403 unless an administrator turned on settings.ytdlpEnabled.
// Cookies are write-only: they are stored encrypted on the server and no endpoint returns
// them.
//
// Owner: manage agent (D).

// ---- managers (registered by routesManage)

type downloadSite struct {
	ID      string `json:"id"`
	Cookies bool   `json:"cookies"` // the server has sign-in cookies for this site
}

type downloadsStatus struct {
	Enabled bool                 `json:"enabled"`
	Ready   bool                 `json:"ready"` // yt-dlp installed and runnable, ffmpeg present
	Sites   []downloadSite       `json:"sites"`
	Jobs    []manage.DownloadJob `json:"jobs"`
}

func (a *API) manageDownloads(w http.ResponseWriter, r *http.Request) {
	sites := make([]downloadSite, 0, len(ytdlp.Sites))
	for _, id := range ytdlp.Sites {
		sites = append(sites, downloadSite{ID: id, Cookies: a.app.Ytdlp.HasCookies(id)})
	}
	writeJSON(w, http.StatusOK, downloadsStatus{
		Enabled: a.app.Settings(r.Context()).YtdlpEnabled,
		Ready:   a.app.Ytdlp.Ready(r.Context()) == nil,
		Sites:   sites,
		Jobs:    nonNil(a.app.Manage.DownloadJobs()),
	})
}

// requireYtdlp writes the 403 and returns false while downloads are turned off.
func (a *API) requireYtdlp(w http.ResponseWriter, r *http.Request) bool {
	if !a.app.Settings(r.Context()).YtdlpEnabled {
		writeErr(w, r, forbidden("downloads from YouTube and bilibili are turned off; an administrator can enable them in Settings → yt-dlp"))
		return false
	}
	return true
}

func (a *API) manageStartDownload(w http.ResponseWriter, r *http.Request) {
	if !a.requireYtdlp(w, r) {
		return
	}
	var body manage.DownloadRequest
	if err := decodeJSON(w, r, &body); err != nil {
		writeErr(w, r, err)
		return
	}
	job, err := a.app.Manage.StartDownload(r.Context(), userFrom(r), body)
	if err != nil {
		writeManageErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusAccepted, job)
}

func (a *API) manageRemoveDownload(w http.ResponseWriter, r *http.Request) {
	if err := a.app.Manage.RemoveDownload(chi.URLParam(r, "id")); err != nil {
		writeErr(w, r, err)
		return
	}
	writeNoContent(w)
}

// ---- administrators (registered by routesAdmin)

type ytdlpInfo struct {
	Enabled bool `json:"enabled"`
	ytdlp.Status
	Cookies []ytdlp.CookieInfo `json:"cookies"`
}

func (a *API) ytdlpInfo(r *http.Request) ytdlpInfo {
	return ytdlpInfo{
		Enabled: a.app.Settings(r.Context()).YtdlpEnabled,
		Status:  a.app.Ytdlp.Status(r.Context()),
		Cookies: a.app.Ytdlp.Cookies(),
	}
}

func (a *API) adminYtdlp(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, a.ytdlpInfo(r))
}

func (a *API) adminYtdlpCheck(w http.ResponseWriter, r *http.Request) {
	if !a.requireYtdlp(w, r) {
		return
	}
	if _, err := a.app.Ytdlp.CheckLatest(r.Context()); err != nil {
		writeYtdlpErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, a.ytdlpInfo(r))
}

func (a *API) adminYtdlpInstall(w http.ResponseWriter, r *http.Request) {
	if !a.requireYtdlp(w, r) {
		return
	}
	if err := a.app.Ytdlp.StartInstall(); err != nil {
		writeYtdlpErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusAccepted, a.ytdlpInfo(r))
}

func (a *API) adminSetCookies(w http.ResponseWriter, r *http.Request) {
	site := chi.URLParam(r, "site")
	if !ytdlp.ValidSite(site) {
		writeErr(w, r, notFound("unknown site %q", site))
		return
	}
	var body struct {
		Text string `json:"text"`
	}
	if err := decodeJSON(w, r, &body); err != nil {
		writeErr(w, r, err)
		return
	}
	res, err := a.app.Ytdlp.SetCookies(site, body.Text)
	if err != nil {
		writeYtdlpErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (a *API) adminDeleteCookies(w http.ResponseWriter, r *http.Request) {
	site := chi.URLParam(r, "site")
	if !ytdlp.ValidSite(site) {
		writeErr(w, r, notFound("unknown site %q", site))
		return
	}
	if err := a.app.Ytdlp.DeleteCookies(site); err != nil {
		writeYtdlpErr(w, r, err)
		return
	}
	writeNoContent(w)
}

// writeYtdlpErr maps ytdlp errors; GitHub failures become 503 (logged by writeErr).
func writeYtdlpErr(w http.ResponseWriter, r *http.Request, err error) {
	msg := err.Error()
	for _, e := range []error{ytdlp.ErrInvalid, ytdlp.ErrUpstream, ytdlp.ErrUnsupportedPlatform} {
		msg = strings.TrimPrefix(msg, e.Error()+": ")
	}
	switch {
	case errors.Is(err, ytdlp.ErrInvalid):
		writeErr(w, r, badRequest("%s", msg))
	case errors.Is(err, ytdlp.ErrUnmanaged), errors.Is(err, ytdlp.ErrBusy):
		writeErr(w, r, newError(http.StatusConflict, CodeConflict, "%s", err.Error()))
	case errors.Is(err, ytdlp.ErrUnsupportedPlatform):
		writeErr(w, r, newError(http.StatusNotImplemented, CodeNotImplemented, "%s", msg))
	case errors.Is(err, ytdlp.ErrUpstream):
		writeErr(w, r, newError(http.StatusServiceUnavailable, CodeUnavailable, "%s", msg))
	default:
		writeErr(w, r, err)
	}
}
