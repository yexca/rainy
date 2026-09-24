package api

import (
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"

	"rainy/internal/model"
)

// Playlist field limits.
const (
	maxPlaylistNameLen    = 200
	maxPlaylistCommentLen = 2000
	maxPlaylistTracks     = 10000
)

// routesPlaylists registers the /playlists endpoints (docs/architecture/contract.md §7.4); mounted behind
// auth.RequireUser. Only the owner (or an admin) may modify a playlist; private playlists
// are visible to their owner and admins only.
func (a *API) routesPlaylists(r chi.Router) {
	r.Get("/playlists", a.plList)
	r.Post("/playlists", a.plCreate)
	r.Get("/playlists/{id}", a.plGet)
	r.Put("/playlists/{id}", a.plUpdate)
	r.Delete("/playlists/{id}", a.plDelete)
	r.Post("/playlists/{id}/tracks", a.plAppendTracks)
	r.Put("/playlists/{id}/tracks", a.plSetTracks)
	r.Delete("/playlists/{id}/tracks", a.plRemoveTracks)
}

// playlistDetail is `Playlist & {tracks, readonly}`.
type playlistDetail struct {
	model.Playlist
	Tracks   []model.Track `json:"tracks"`
	Readonly bool          `json:"readonly"`
}

// canEditPlaylist reports whether u may modify p.
func canEditPlaylist(u *model.User, p *model.Playlist) bool {
	return u != nil && (u.IsAdmin || p.OwnerID == u.ID)
}

// canViewPlaylist reports whether u may see p.
func canViewPlaylist(u *model.User, p *model.Playlist) bool {
	return p.Public || canEditPlaylist(u, p)
}

// loadPlaylist fetches the playlist of the {id} URL parameter. Playlists the user may not
// see yield 404 (existence is not revealed); forEdit additionally requires ownership (403).
func (a *API) loadPlaylist(r *http.Request, forEdit bool) (*model.Playlist, error) {
	p, err := a.app.Store.GetPlaylist(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		return nil, err
	}
	u := userFrom(r)
	if !canViewPlaylist(u, p) {
		return nil, notFound("playlist not found")
	}
	if forEdit && !canEditPlaylist(u, p) {
		return nil, forbidden("only the owner of this playlist can change it")
	}
	return p, nil
}

// validatePlaylistFields trims and checks name/comment.
func validatePlaylistFields(name, comment *string) error {
	if name != nil {
		*name = strings.TrimSpace(*name)
		if *name == "" {
			return badRequest("name is required")
		}
		if utf8.RuneCountInString(*name) > maxPlaylistNameLen {
			return badRequest("name must be at most %d characters", maxPlaylistNameLen)
		}
	}
	if comment != nil {
		*comment = strings.TrimSpace(*comment)
		if utf8.RuneCountInString(*comment) > maxPlaylistCommentLen {
			return badRequest("comment must be at most %d characters", maxPlaylistCommentLen)
		}
	}
	return nil
}

func checkPlaylistTrackCount(ids []string) error {
	if len(ids) > maxPlaylistTracks {
		return badRequest("at most %d tracks per request", maxPlaylistTracks)
	}
	return nil
}

func (a *API) plList(w http.ResponseWriter, r *http.Request) {
	pls, err := a.app.Store.ListPlaylists(r.Context(), userFrom(r).ID)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, nonNil(pls))
}

type playlistCreateBody struct {
	Name     string   `json:"name"`
	Comment  string   `json:"comment"`
	Public   bool     `json:"public"`
	TrackIDs []string `json:"trackIds"`
}

func (a *API) plCreate(w http.ResponseWriter, r *http.Request) {
	var body playlistCreateBody
	if err := decodeJSON(w, r, &body); err != nil {
		writeErr(w, r, err)
		return
	}
	if err := validatePlaylistFields(&body.Name, &body.Comment); err != nil {
		writeErr(w, r, err)
		return
	}
	if err := checkPlaylistTrackCount(body.TrackIDs); err != nil {
		writeErr(w, r, err)
		return
	}
	ctx := r.Context()
	p := &model.Playlist{Name: body.Name, Comment: body.Comment, Public: body.Public, OwnerID: userFrom(r).ID}
	if err := a.app.Store.CreatePlaylist(ctx, p, body.TrackIDs); err != nil {
		writeErr(w, r, err)
		return
	}
	a.writePlaylist(w, r, p.ID, http.StatusCreated)
}

// writePlaylist re-reads a playlist (fresh counts and cover version) and writes it.
func (a *API) writePlaylist(w http.ResponseWriter, r *http.Request, id string, status int) {
	p, err := a.app.Store.GetPlaylist(r.Context(), id)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, status, p)
}

func (a *API) plGet(w http.ResponseWriter, r *http.Request) {
	p, err := a.loadPlaylist(r, false)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	u := userFrom(r)
	tracks, err := a.app.Store.PlaylistTracks(r.Context(), p.ID, u.ID)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, playlistDetail{Playlist: *p, Tracks: nonNil(tracks), Readonly: !canEditPlaylist(u, p)})
}

type playlistUpdateBody struct {
	Name    *string `json:"name"`
	Comment *string `json:"comment"`
	Public  *bool   `json:"public"`
}

func (a *API) plUpdate(w http.ResponseWriter, r *http.Request) {
	var body playlistUpdateBody
	if err := decodeJSON(w, r, &body); err != nil {
		writeErr(w, r, err)
		return
	}
	if err := validatePlaylistFields(body.Name, body.Comment); err != nil {
		writeErr(w, r, err)
		return
	}
	p, err := a.loadPlaylist(r, true)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	if body.Name != nil {
		p.Name = *body.Name
	}
	if body.Comment != nil {
		p.Comment = *body.Comment
	}
	if body.Public != nil {
		p.Public = *body.Public
	}
	if err := a.app.Store.UpdatePlaylist(r.Context(), p); err != nil {
		writeErr(w, r, err)
		return
	}
	a.writePlaylist(w, r, p.ID, http.StatusOK)
}

func (a *API) plDelete(w http.ResponseWriter, r *http.Request) {
	p, err := a.loadPlaylist(r, true)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	if err := a.app.Store.DeletePlaylist(r.Context(), p.ID); err != nil {
		writeErr(w, r, err)
		return
	}
	writeNoContent(w)
}

type playlistTracksBody struct {
	TrackIDs []string `json:"trackIds"`
}

func (a *API) plAppendTracks(w http.ResponseWriter, r *http.Request) {
	a.plChangeTracks(w, r, false)
}

func (a *API) plSetTracks(w http.ResponseWriter, r *http.Request) {
	a.plChangeTracks(w, r, true)
}

// plChangeTracks appends (replace=false) or replaces the playlist's tracks.
func (a *API) plChangeTracks(w http.ResponseWriter, r *http.Request, replace bool) {
	var body playlistTracksBody
	if err := decodeJSON(w, r, &body); err != nil {
		writeErr(w, r, err)
		return
	}
	if err := checkPlaylistTrackCount(body.TrackIDs); err != nil {
		writeErr(w, r, err)
		return
	}
	p, err := a.loadPlaylist(r, true)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	if replace {
		err = a.app.Store.SetPlaylistTracks(r.Context(), p.ID, body.TrackIDs)
	} else {
		err = a.app.Store.AppendPlaylistTracks(r.Context(), p.ID, body.TrackIDs)
	}
	if err != nil {
		writeErr(w, r, err)
		return
	}
	a.writePlaylist(w, r, p.ID, http.StatusOK)
}

type playlistPositionsBody struct {
	Positions []int `json:"positions"`
}

func (a *API) plRemoveTracks(w http.ResponseWriter, r *http.Request) {
	var body playlistPositionsBody
	if err := decodeJSON(w, r, &body); err != nil {
		writeErr(w, r, err)
		return
	}
	if len(body.Positions) > maxPlaylistTracks {
		writeErr(w, r, badRequest("at most %d positions per request", maxPlaylistTracks))
		return
	}
	p, err := a.loadPlaylist(r, true)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	if err := a.app.Store.RemovePlaylistPositions(r.Context(), p.ID, body.Positions); err != nil {
		writeErr(w, r, err)
		return
	}
	a.writePlaylist(w, r, p.ID, http.StatusOK)
}
