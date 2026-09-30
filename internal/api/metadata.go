package api

import (
	"errors"
	"net/http"
	"strconv"

	"rainy/internal/metasearch"
)

// Online metadata lookup for the tag editor (contract §7.6), registered by routesManage.
// Every endpoint refuses with 403 unless an administrator enabled settings.onlineMetadata,
// because these are the only requests Rainy sends to third-party services. Results only
// fill the editor's draft; files change through the normal save endpoints.
//
// Owner: manage agent (D).

type metadataStatus struct {
	Enabled   bool                      `json:"enabled"`
	Providers []metasearch.ProviderInfo `json:"providers"`
}

func (a *API) manageMetadataStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, metadataStatus{
		Enabled:   a.app.Settings(r.Context()).OnlineMetadata,
		Providers: a.app.Metadata.Providers(),
	})
}

// metadataOptions returns the lookup options from the settings, or writes the 403 and
// returns false when the feature is off.
func (a *API) metadataOptions(w http.ResponseWriter, r *http.Request) (metasearch.Options, bool) {
	s := a.app.Settings(r.Context())
	if !s.OnlineMetadata {
		writeErr(w, r, forbidden("online metadata lookup is disabled; an administrator can enable it in Settings"))
		return metasearch.Options{}, false
	}
	return metasearch.Options{ChinaIP: s.OnlineMetadataChinaIP}, true
}

func (a *API) manageMetadataSearch(w http.ResponseWriter, r *http.Request) {
	opts, ok := a.metadataOptions(w, r)
	if !ok {
		return
	}
	q := r.URL.Query()
	items, err := a.app.Metadata.Search(r.Context(), q.Get("provider"), q.Get("q"),
		queryInt(r, "limit", metasearch.DefaultLimit), q.Get("region"), opts)
	if err != nil {
		writeMetadataErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": nonNil(items)})
}

func (a *API) manageMetadataLyrics(w http.ResponseWriter, r *http.Request) {
	opts, ok := a.metadataOptions(w, r)
	if !ok {
		return
	}
	q := r.URL.Query()
	l, err := a.app.Metadata.Lyrics(r.Context(), q.Get("provider"), q.Get("id"), opts)
	if err != nil {
		writeMetadataErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, l)
}

// manageMetadataCover proxies a result's cover from the provider's image host, so the
// browser neither contacts third-party hosts nor needs CORS to turn it into an upload.
func (a *API) manageMetadataCover(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.metadataOptions(w, r); !ok {
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
	h.Set("Cache-Control", "private, max-age=3600")
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Content-Security-Policy", "default-src 'none'; sandbox")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

// writeMetadataErr maps metasearch errors; upstream failures become 503 (logged by writeErr)
// with a message that names the provider host but never the query.
func writeMetadataErr(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, metasearch.ErrUnknownProvider), errors.Is(err, metasearch.ErrInvalid):
		writeErr(w, r, badRequest("%s", err.Error()))
	case errors.Is(err, metasearch.ErrNotFound):
		writeErr(w, r, notFound("%s", err.Error()))
	case errors.Is(err, metasearch.ErrUpstream):
		writeErr(w, r, newError(http.StatusServiceUnavailable, CodeUnavailable, "%s", err.Error()))
	default:
		writeErr(w, r, err)
	}
}
