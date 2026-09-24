package api

import (
	"net/http"
	"slices"
	"strings"

	"github.com/go-chi/chi/v5"

	"rainy/internal/model"
	"rainy/internal/store"
)

// List limits (docs/architecture/contract.md §7).
const (
	defaultPageLimit = 50
	maxPageLimit     = 1000
	maxRandomSize    = 500
	maxSearchLimit   = 500
	maxIDsPerRequest = 1000
	topTracksCount   = 10
)

// routesLibrary registers the library browsing endpoints (docs/architecture/contract.md §7.2); mounted
// behind auth.RequireUser.
func (a *API) routesLibrary(r chi.Router) {
	r.Get("/home", a.libHome)
	r.Get("/albums", a.libAlbums)
	r.Get("/albums/{id}", a.libAlbum)
	r.Get("/artists", a.libArtists)
	r.Get("/artists/{id}", a.libArtist)
	r.Get("/tracks", a.libTracks)
	r.Get("/tracks/{id}", a.libTrack)
	r.Get("/genres", a.libGenres)
	r.Get("/search", a.libSearch)
	r.Get("/starred", a.libStarred)
	r.Get("/random", a.libRandom)
	r.Get("/recent-tracks", a.libRecentTracks)
}

// yearRange reads ?fromYear and ?toYear (0 = unbounded).
func yearRange(r *http.Request) (from, to int) {
	return max(queryInt(r, "fromYear", 0), 0), max(queryInt(r, "toYear", 0), 0)
}

// ---- albums

func (a *API) libAlbums(w http.ResponseWriter, r *http.Request) {
	offset, limit, sort, order := pageParams(r, defaultPageLimit, maxPageLimit)
	from, to := yearRange(r)
	q := r.URL.Query()
	albums, total, err := a.app.Store.ListAlbums(r.Context(), store.AlbumQuery{
		UserID:    userFrom(r).ID,
		Q:         strings.TrimSpace(q.Get("q")),
		ArtistID:  q.Get("artistId"),
		Genre:     q.Get("genre"),
		LibraryID: queryInt64(r, "libraryId", 0),
		FromYear:  from,
		ToYear:    to,
		Starred:   queryBool(r, "starred"),
		Sort:      sort,
		Order:     order,
		Offset:    offset,
		Limit:     limit,
	})
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writePage(w, albums, total)
}

// albumDetail is `Album & {tracks, discs}`.
type albumDetail struct {
	model.Album
	Tracks []model.Track `json:"tracks"`
	Discs  []int         `json:"discs"`
}

func (a *API) libAlbum(w http.ResponseWriter, r *http.Request) {
	ctx, uid := r.Context(), userFrom(r).ID
	album, err := a.app.Store.GetAlbum(ctx, chi.URLParam(r, "id"), uid)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	tracks, _, err := a.app.Store.ListTracks(ctx, store.TrackQuery{UserID: uid, AlbumID: album.ID, Sort: "track"})
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, albumDetail{Album: *album, Tracks: nonNil(tracks), Discs: discNumbers(tracks)})
}

// discNumbers returns the distinct disc numbers of tracks in ascending order (never nil).
func discNumbers(tracks []model.Track) []int {
	discs := []int{}
	for _, t := range tracks {
		if !slices.Contains(discs, t.DiscNumber) {
			discs = append(discs, t.DiscNumber)
		}
	}
	slices.Sort(discs)
	return discs
}

// ---- artists

func (a *API) libArtists(w http.ResponseWriter, r *http.Request) {
	offset, limit, sort, order := pageParams(r, defaultPageLimit, maxPageLimit)
	artists, total, err := a.app.Store.ListArtists(r.Context(), store.ArtistQuery{
		UserID:           userFrom(r).ID,
		Q:                strings.TrimSpace(r.URL.Query().Get("q")),
		LibraryID:        queryInt64(r, "libraryId", 0),
		AlbumArtistsOnly: !queryBool(r, "all"),
		Starred:          queryBool(r, "starred"),
		Sort:             sort,
		Order:            order,
		Offset:           offset,
		Limit:            limit,
	})
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writePage(w, artists, total)
}

// artistDetail is `Artist & {albums, appearsOn, topTracks}`.
type artistDetail struct {
	model.Artist
	Albums    []model.Album `json:"albums"`
	AppearsOn []model.Album `json:"appearsOn"`
	TopTracks []model.Track `json:"topTracks"`
}

func (a *API) libArtist(w http.ResponseWriter, r *http.Request) {
	ctx, uid := r.Context(), userFrom(r).ID
	artist, err := a.app.Store.GetArtist(ctx, chi.URLParam(r, "id"), uid)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	albums, _, err := a.app.Store.ListAlbums(ctx, store.AlbumQuery{UserID: uid, ArtistID: artist.ID, Sort: "year", Order: "desc"})
	if err != nil {
		writeErr(w, r, err)
		return
	}
	appearsOn, err := a.app.Store.AlbumsAppearsOn(ctx, artist.ID, uid)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	top, err := a.artistTopTracks(r, artist.ID, topTracksCount)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, artistDetail{Artist: *artist, Albums: nonNil(albums), AppearsOn: nonNil(appearsOn), TopTracks: top})
}

// artistTopTracks returns up to n of the artist's tracks: the user's most played first,
// then random others to fill up.
func (a *API) artistTopTracks(r *http.Request, artistID string, n int) ([]model.Track, error) {
	ctx, uid := r.Context(), userFrom(r).ID
	top, _, err := a.app.Store.ListTracks(ctx, store.TrackQuery{UserID: uid, ArtistID: artistID, Played: true, Sort: "frequent", Limit: n})
	if err != nil {
		return nil, err
	}
	top = nonNil(top)
	if len(top) >= n {
		return top, nil
	}
	fill, _, err := a.app.Store.ListTracks(ctx, store.TrackQuery{UserID: uid, ArtistID: artistID, Sort: "random", Limit: n + len(top)})
	if err != nil {
		return nil, err
	}
	seen := make(map[string]bool, len(top))
	for _, t := range top {
		seen[t.ID] = true
	}
	for _, t := range fill {
		if len(top) >= n {
			break
		}
		if !seen[t.ID] {
			seen[t.ID] = true
			top = append(top, t)
		}
	}
	return top, nil
}

// ---- tracks

func (a *API) libTracks(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r)
	offset, limit, sort, order := pageParams(r, defaultPageLimit, maxPageLimit)
	from, to := yearRange(r)
	q := r.URL.Query()
	tq := store.TrackQuery{
		UserID:    u.ID,
		Q:         strings.TrimSpace(q.Get("q")),
		AlbumID:   q.Get("albumId"),
		ArtistID:  q.Get("artistId"),
		Genre:     q.Get("genre"),
		LibraryID: queryInt64(r, "libraryId", 0),
		Dir:       q.Get("dir"),
		DirPrefix: q.Get("dirPrefix"),
		FromYear:  from,
		ToYear:    to,
		Starred:   queryBool(r, "starred"),
		Sort:      sort,
		Order:     order,
		Offset:    offset,
		Limit:     limit,
	}
	// Missing files are only visible to library managers.
	if m := q.Get("missing"); u.CanEdit() && (m == "include" || m == "only") {
		tq.Missing = m
	}
	ids := queryList(r, "ids")
	if q.Has("ids") {
		if len(ids) > maxIDsPerRequest {
			writeErr(w, r, badRequest("at most %d ids per request", maxIDsPerRequest))
			return
		}
		tq.IDs = nonNil(ids)
	}
	if tq.IDs != nil && sort == "" {
		// No explicit sort: keep the order of the requested ids.
		tq.Offset, tq.Limit = 0, 0
		tracks, _, err := a.app.Store.ListTracks(r.Context(), tq)
		if err != nil {
			writeErr(w, r, err)
			return
		}
		tracks = orderByIDs(tracks, ids)
		total := len(tracks)
		start := min(offset, total)
		writePage(w, tracks[start:min(start+limit, total)], total)
		return
	}
	tracks, total, err := a.app.Store.ListTracks(r.Context(), tq)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writePage(w, tracks, total)
}

// orderByIDs orders tracks like ids (first occurrence wins; tracks not in ids dropped).
func orderByIDs(tracks []model.Track, ids []string) []model.Track {
	byID := make(map[string]model.Track, len(tracks))
	for _, t := range tracks {
		byID[t.ID] = t
	}
	out := make([]model.Track, 0, len(tracks))
	for _, id := range ids {
		if t, ok := byID[id]; ok {
			out = append(out, t)
			delete(byID, id)
		}
	}
	return out
}

func (a *API) libTrack(w http.ResponseWriter, r *http.Request) {
	t, err := a.app.Store.GetTrack(r.Context(), chi.URLParam(r, "id"), userFrom(r).ID)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, t)
}

// ---- genres, search, starred, random, recent

func (a *API) libGenres(w http.ResponseWriter, r *http.Request) {
	genres, err := a.app.Store.ListGenres(r.Context())
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, nonNil(genres))
}

func (a *API) libSearch(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if q == "" {
		writeJSON(w, http.StatusOK, store.SearchResult{Artists: []model.Artist{}, Albums: []model.Album{}, Tracks: []model.Track{}})
		return
	}
	limit := func(name string, def int) int { return min(max(queryInt(r, name, def), 0), maxSearchLimit) }
	res, err := a.app.Store.Search(r.Context(), store.SearchQuery{
		UserID:      userFrom(r).ID,
		Q:           q,
		LibraryID:   queryInt64(r, "libraryId", 0),
		ArtistLimit: limit("artists", 6),
		AlbumLimit:  limit("albums", 12),
		TrackLimit:  limit("tracks", 30),
	})
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (a *API) libStarred(w http.ResponseWriter, r *http.Request) {
	ctx, uid := r.Context(), userFrom(r).ID
	artists, _, err := a.app.Store.ListArtists(ctx, store.ArtistQuery{UserID: uid, Starred: true, Sort: "starred"})
	if err != nil {
		writeErr(w, r, err)
		return
	}
	albums, _, err := a.app.Store.ListAlbums(ctx, store.AlbumQuery{UserID: uid, Starred: true, Sort: "starred"})
	if err != nil {
		writeErr(w, r, err)
		return
	}
	tracks, _, err := a.app.Store.ListTracks(ctx, store.TrackQuery{UserID: uid, Starred: true, Sort: "starred"})
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, store.SearchResult{Artists: nonNil(artists), Albums: nonNil(albums), Tracks: nonNil(tracks)})
}

func (a *API) libRandom(w http.ResponseWriter, r *http.Request) {
	size := queryInt(r, "size", defaultPageLimit)
	if size <= 0 {
		size = defaultPageLimit
	}
	from, to := yearRange(r)
	tracks, _, err := a.app.Store.ListTracks(r.Context(), store.TrackQuery{
		UserID:   userFrom(r).ID,
		Genre:    r.URL.Query().Get("genre"),
		FromYear: from,
		ToYear:   to,
		Sort:     "random",
		Limit:    min(size, maxRandomSize),
	})
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, nonNil(tracks))
}

func (a *API) libRecentTracks(w http.ResponseWriter, r *http.Request) {
	limit := queryInt(r, "limit", defaultPageLimit)
	if limit <= 0 {
		limit = defaultPageLimit
	}
	tracks, err := a.app.Store.RecentlyPlayedTracks(r.Context(), userFrom(r).ID, min(limit, maxRandomSize))
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, nonNil(tracks))
}
