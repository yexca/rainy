// Package subsonic implements the Subsonic / OpenSubsonic API under /rest
// (docs/architecture/contract.md §6).
//
// Every method is reachable as /rest/<method> and /rest/<method>.view, via GET, POST
// (application/x-www-form-urlencoded, merged with the query string — OpenSubsonic
// formPost) or HEAD. Responses are XML (default), JSON (f=json) or JSONP (f=jsonp with
// callback=<fn>). Folder browsing is simulated from tags: getIndexes lists album artists,
// an artist "directory" holds its albums and an album "directory" holds its songs.
package subsonic

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"

	"rainy/internal/app"
	"rainy/internal/model"
)

// maxFormBytes bounds a POSTed form body (large queues/playlists fit comfortably).
const maxFormBytes = 4 << 20

// API serves /rest.
type API struct {
	app       *app.App
	endpoints map[string]endpoint
	limiter   *limiter

	seenMu sync.Mutex
	seen   map[string]time.Time // user id → last TouchUserSeen
}

// handlerFunc handles one method. It returns the response to serialise, or (nil, nil)
// when it already wrote a (binary) response itself.
type handlerFunc func(*request) (*Response, error)

type endpoint struct {
	h      handlerFunc
	public bool // no authentication required
}

// New creates the Subsonic API.
func New(a *app.App) *API {
	api := &API{app: a, limiter: newLimiter(), seen: map[string]time.Time{}}
	api.endpoints = api.routes()
	return api
}

// routes is the method table.
func (a *API) routes() map[string]endpoint {
	h := func(f handlerFunc) endpoint { return endpoint{h: f} }
	return map[string]endpoint{
		// system
		"ping":                      h(a.ping),
		"getLicense":                h(a.getLicense),
		"getOpenSubsonicExtensions": {h: a.getOpenSubsonicExtensions, public: true},
		"tokenInfo":                 h(a.tokenInfo),
		"getScanStatus":             h(a.getScanStatus),
		"startScan":                 h(a.startScan),

		// browsing
		"getMusicFolders":   h(a.getMusicFolders),
		"getIndexes":        h(a.getIndexes),
		"getMusicDirectory": h(a.getMusicDirectory),
		"getGenres":         h(a.getGenres),
		"getArtists":        h(a.getArtists),
		"getArtist":         h(a.getArtist),
		"getAlbum":          h(a.getAlbum),
		"getSong":           h(a.getSong),
		"getArtistInfo":     h(a.getArtistInfo),
		"getArtistInfo2":    h(a.getArtistInfo2),
		"getAlbumInfo":      h(a.getAlbumInfo),
		"getAlbumInfo2":     h(a.getAlbumInfo),
		"getSimilarSongs":   h(a.getSimilarSongs),
		"getSimilarSongs2":  h(a.getSimilarSongs2),
		"getTopSongs":       h(a.getTopSongs),

		// lists
		"getAlbumList":    h(a.getAlbumList),
		"getAlbumList2":   h(a.getAlbumList2),
		"getRandomSongs":  h(a.getRandomSongs),
		"getSongsByGenre": h(a.getSongsByGenre),
		"getNowPlaying":   h(a.getNowPlaying),
		"getStarred":      h(a.getStarred),
		"getStarred2":     h(a.getStarred2),

		// search
		"search":  h(a.search),
		"search2": h(a.search2),
		"search3": h(a.search3),

		// playlists
		"getPlaylists":   h(a.getPlaylists),
		"getPlaylist":    h(a.getPlaylist),
		"createPlaylist": h(a.createPlaylist),
		"updatePlaylist": h(a.updatePlaylist),
		"deletePlaylist": h(a.deletePlaylist),

		// media
		"stream":            h(a.stream),
		"download":          h(a.download),
		"getCoverArt":       h(a.getCoverArt),
		"getLyrics":         h(a.getLyrics),
		"getLyricsBySongId": h(a.getLyricsBySongID),
		"getAvatar":         h(a.getAvatar),

		// annotation
		"star":      h(a.star),
		"unstar":    h(a.unstar),
		"setRating": h(a.setRating),
		"scrobble":  h(a.scrobble),

		// bookmarks & play queue
		"getBookmarks":         h(a.getBookmarks),
		"createBookmark":       h(a.createBookmark),
		"deleteBookmark":       h(a.deleteBookmark),
		"getPlayQueue":         h(a.getPlayQueue),
		"savePlayQueue":        h(a.savePlayQueue),
		"getPlayQueueByIndex":  h(a.getPlayQueueByIndex),
		"savePlayQueueByIndex": h(a.savePlayQueueByIndex),

		// users
		"getUser":        h(a.getUser),
		"getUsers":       h(a.getUsers),
		"createUser":     h(a.createUser),
		"updateUser":     h(a.updateUser),
		"deleteUser":     h(a.deleteUser),
		"changePassword": h(a.changePassword),

		// internet radio
		"getInternetRadioStations":   h(a.getInternetRadioStations),
		"createInternetRadioStation": h(a.createInternetRadioStation),
		"updateInternetRadioStation": h(a.updateInternetRadioStation),
		"deleteInternetRadioStation": h(a.deleteInternetRadioStation),

		// unsupported features: empty lists / "not supported"
		"getPodcasts":            h(a.getPodcasts),
		"getNewestPodcasts":      h(a.getNewestPodcasts),
		"getPodcastEpisode":      h(notFoundStub("Podcast episode")),
		"refreshPodcasts":        h(unsupported("Podcasts")),
		"createPodcastChannel":   h(unsupported("Podcasts")),
		"deletePodcastChannel":   h(unsupported("Podcasts")),
		"deletePodcastEpisode":   h(unsupported("Podcasts")),
		"downloadPodcastEpisode": h(unsupported("Podcasts")),
		"getShares":              h(a.getShares),
		"createShare":            h(unsupported("Sharing")),
		"updateShare":            h(unsupported("Sharing")),
		"deleteShare":            h(unsupported("Sharing")),
		"getChatMessages":        h(a.getChatMessages),
		"addChatMessage":         h(unsupported("Chat")),
		"jukeboxControl":         h(unsupported("Jukebox")),
		"getVideos":              h(a.getVideos),
		"getVideoInfo":           h(notFoundStub("Video")),
	}
}

// Routes returns the /rest handler.
func (a *API) Routes() http.Handler {
	r := chi.NewRouter()
	r.MethodFunc(http.MethodGet, "/{method}", a.serve)
	r.MethodFunc(http.MethodPost, "/{method}", a.serve)
	r.MethodFunc(http.MethodHead, "/{method}", a.serve)
	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		writeResponse(w, r.Form.Get("f"), r.Form.Get("callback"), http.StatusNotFound,
			failedResponse(newError(codeNotFound, "Unknown API endpoint")))
	})
	r.MethodNotAllowed(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Allow", "GET, POST, HEAD")
		writeResponse(w, "", "", http.StatusMethodNotAllowed,
			failedResponse(newError(codeGeneric, "Method not allowed")))
	})
	return r
}

// request is the per-call state passed to handlers.
type request struct {
	api      *API
	w        http.ResponseWriter
	r        *http.Request
	ctx      context.Context
	params   url.Values
	user     *model.User
	client   string
	format   string
	callback string

	settingsOnce sync.Once
	settings     model.Settings
}

func (a *API) serve(w http.ResponseWriter, r *http.Request) {
	if r.Body != nil {
		r.Body = http.MaxBytesReader(w, r.Body, maxFormBytes)
	}
	parseErr := r.ParseForm()
	req := &request{
		api:      a,
		w:        w,
		r:        r,
		ctx:      r.Context(),
		params:   r.Form,
		client:   strings.TrimSpace(r.Form.Get("c")),
		format:   strings.ToLower(r.Form.Get("f")),
		callback: r.Form.Get("callback"),
	}
	if req.client == "" {
		req.client = "subsonic"
	}
	if len(req.client) > 128 {
		req.client = req.client[:128]
	}
	if parseErr != nil {
		req.fail(newError(codeGeneric, "Invalid request parameters"))
		return
	}

	method := strings.TrimSuffix(chi.URLParam(r, "method"), ".view")
	ep, ok := a.endpoints[method]
	if !ok {
		writeResponse(w, req.format, req.callback, http.StatusNotFound,
			failedResponse(newError(codeNotFound, "Unknown API method: %s", method)))
		return
	}
	if !ep.public || hasCredentials(req.params) {
		u, aerr := a.authenticate(req)
		if aerr != nil {
			if !ep.public {
				req.fail(aerr)
				return
			}
		} else {
			req.user = u
			a.touchSeen(req.ctx, u.ID)
		}
	}

	resp, err := ep.h(req)
	if err != nil {
		req.fail(toAPIError(r, err))
		return
	}
	if resp == nil {
		return // handler wrote the response itself
	}
	writeResponse(w, req.format, req.callback, http.StatusOK, resp)
}

// fail writes a failed envelope (HTTP 200, as Subsonic clients expect).
func (q *request) fail(e *apiError) {
	writeResponse(q.w, q.format, q.callback, http.StatusOK, failedResponse(e))
}

// touchSeen updates the user's last-seen time at most once per minute.
func (a *API) touchSeen(ctx context.Context, userID string) {
	now := time.Now()
	a.seenMu.Lock()
	last, ok := a.seen[userID]
	if ok && now.Sub(last) < time.Minute {
		a.seenMu.Unlock()
		return
	}
	a.seen[userID] = now
	a.seenMu.Unlock()
	_ = a.app.Store.TouchUserSeen(ctx, userID)
}

// appSettings returns the runtime settings (read once per request).
func (q *request) appSettings() model.Settings {
	q.settingsOnce.Do(func() { q.settings = q.api.app.Settings(q.ctx) })
	return q.settings
}

// ---- parameters

// has reports whether a parameter is present (possibly empty).
func (q *request) has(name string) bool {
	_, ok := q.params[name]
	return ok
}

// str returns a parameter value ("" when absent).
func (q *request) str(name string) string { return strings.TrimSpace(q.params.Get(name)) }

// requiredStr returns a non-empty parameter or error 10.
func (q *request) requiredStr(name string) (string, error) {
	v := q.str(name)
	if v == "" {
		return "", errMissing(name)
	}
	return v, nil
}

// strs returns all non-empty values of a repeated parameter.
func (q *request) strs(name string) []string {
	var out []string
	for _, v := range q.params[name] {
		if v = strings.TrimSpace(v); v != "" {
			out = append(out, v)
		}
	}
	return out
}

// intParam parses an integer parameter, returning def when absent or empty.
func (q *request) intParam(name string, def int) (int, error) {
	v := q.str(name)
	if v == "" {
		return def, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0, newError(codeGeneric, "Invalid value for parameter %s: %q", name, v)
	}
	return n, nil
}

// int64Param parses an int64 parameter, returning def when absent or empty.
func (q *request) int64Param(name string, def int64) (int64, error) {
	v := q.str(name)
	if v == "" {
		return def, nil
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return 0, newError(codeGeneric, "Invalid value for parameter %s: %q", name, v)
	}
	return n, nil
}

// floatParam parses a float parameter, returning def when absent or empty.
func (q *request) floatParam(name string, def float64) (float64, error) {
	v := q.str(name)
	if v == "" {
		return def, nil
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil || f != f { // reject NaN
		return 0, newError(codeGeneric, "Invalid value for parameter %s: %q", name, v)
	}
	return f, nil
}

// boolParam parses a boolean parameter ("true"/"false"/"1"/"0"), def when absent.
func (q *request) boolParam(name string, def bool) (bool, error) {
	v := strings.ToLower(q.str(name))
	switch v {
	case "":
		return def, nil
	case "true", "1", "yes", "on":
		return true, nil
	case "false", "0", "no", "off":
		return false, nil
	}
	return false, newError(codeGeneric, "Invalid value for parameter %s: %q", name, v)
}

// optBool returns a pointer to the parsed boolean, or nil when absent.
func (q *request) optBool(name string) (*bool, error) {
	if q.str(name) == "" {
		return nil, nil
	}
	b, err := q.boolParam(name, false)
	if err != nil {
		return nil, err
	}
	return &b, nil
}

// pageParams parses a count/offset pair: count defaults to def and is clamped to
// [0, max]; offset must be ≥ 0.
func (q *request) pageParams(countName, offsetName string, def, max int) (count, offset int, err error) {
	if count, err = q.intParam(countName, def); err != nil {
		return 0, 0, err
	}
	if offset, err = q.intParam(offsetName, 0); err != nil {
		return 0, 0, err
	}
	if count < 0 {
		count = 0
	}
	if count > max {
		count = max
	}
	if offset < 0 {
		offset = 0
	}
	return count, offset, nil
}

// musicFolder returns the musicFolderId filter (0 = all libraries); an unknown id yields
// error 70.
func (q *request) musicFolder() (int64, error) {
	v := q.str("musicFolderId")
	if v == "" {
		return 0, nil
	}
	id, err := strconv.ParseInt(v, 10, 64)
	if err != nil || id <= 0 {
		return 0, errNotFound("Music folder")
	}
	if _, err := q.api.app.Store.GetLibrary(q.ctx, id); err != nil {
		return 0, err
	}
	return id, nil
}
