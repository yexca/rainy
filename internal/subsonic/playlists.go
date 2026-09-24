package subsonic

import (
	"strconv"

	"rainy/internal/model"
)

// getPlaylists lists the user's own and all public playlists. Admins may pass username
// to list another user's playlists.
func (a *API) getPlaylists(q *request) (*Response, error) {
	userID := q.user.ID
	if name := q.str("username"); name != "" && name != q.user.Username {
		if !q.user.IsAdmin {
			return nil, errForbidden()
		}
		other, err := a.app.Store.GetUserByUsername(q.ctx, name)
		if err != nil {
			return nil, notFoundAs(err, "User")
		}
		userID = other.ID
	}
	pls, err := a.app.Store.ListPlaylists(q.ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]Playlist, 0, len(pls))
	for i := range pls {
		if userID != q.user.ID && pls[i].OwnerID != userID {
			continue // another user's view: only their own playlists
		}
		out = append(out, playlistOf(&pls[i], q.user))
	}
	resp := newResponse()
	resp.Playlists = &Playlists{Playlists: out}
	return resp, nil
}

// playlistResponse loads a playlist with its songs.
func (a *API) playlistResponse(q *request, id string) (*Response, error) {
	p, err := a.app.Store.GetPlaylist(q.ctx, id)
	if err != nil {
		return nil, notFoundAs(err, "Playlist")
	}
	if !canReadPlaylist(p, q.user) {
		return nil, errNotFound("Playlist")
	}
	tracks, err := a.app.Store.PlaylistTracks(q.ctx, id, q.user.ID)
	if err != nil {
		return nil, err
	}
	resp := newResponse()
	resp.Playlist = &PlaylistWithSongs{Playlist: playlistOf(p, q.user), Entries: q.children(tracks)}
	return resp, nil
}

func (a *API) getPlaylist(q *request) (*Response, error) {
	id, err := q.requiredStr("id")
	if err != nil {
		return nil, err
	}
	return a.playlistResponse(q, id)
}

// writablePlaylist loads a playlist the user may modify (70 if invisible, 50 if read-only).
func (a *API) writablePlaylist(q *request, id string) (*model.Playlist, error) {
	p, err := a.app.Store.GetPlaylist(q.ctx, id)
	if err != nil {
		return nil, notFoundAs(err, "Playlist")
	}
	if !canReadPlaylist(p, q.user) {
		return nil, errNotFound("Playlist")
	}
	if !canModifyPlaylist(p, q.user) {
		return nil, errForbidden()
	}
	return p, nil
}

// createPlaylist creates a playlist (name, songId…) or, with playlistId, replaces the
// songs (and optionally the name) of an existing one. Returns the playlist.
func (a *API) createPlaylist(q *request) (*Response, error) {
	songIDs := q.strs("songId")
	if id := q.str("playlistId"); id != "" {
		p, err := a.writablePlaylist(q, id)
		if err != nil {
			return nil, err
		}
		if name := q.str("name"); name != "" && name != p.Name {
			p.Name = name
			if err := a.app.Store.UpdatePlaylist(q.ctx, p); err != nil {
				return nil, err
			}
		}
		if err := a.app.Store.SetPlaylistTracks(q.ctx, id, songIDs); err != nil {
			return nil, err
		}
		return a.playlistResponse(q, id)
	}
	name, err := q.requiredStr("name")
	if err != nil {
		return nil, err
	}
	p := &model.Playlist{Name: name, OwnerID: q.user.ID}
	if err := a.app.Store.CreatePlaylist(q.ctx, p, songIDs); err != nil {
		return nil, err
	}
	return a.playlistResponse(q, p.ID)
}

// updatePlaylist changes metadata (name, comment, public), then removes songIndexToRemove
// (indexes into the current song list) and appends songIdToAdd.
func (a *API) updatePlaylist(q *request) (*Response, error) {
	id, err := q.requiredStr("playlistId")
	if err != nil {
		return nil, err
	}
	p, err := a.writablePlaylist(q, id)
	if err != nil {
		return nil, err
	}
	public, err := q.optBool("public")
	if err != nil {
		return nil, err
	}
	var remove []int
	for _, v := range q.strs("songIndexToRemove") {
		n, err := strconv.Atoi(v)
		if err != nil {
			return nil, newError(codeGeneric, "Invalid value for parameter songIndexToRemove: %q", v)
		}
		remove = append(remove, n)
	}
	changed := false
	if q.has("name") && q.str("name") != "" {
		p.Name, changed = q.str("name"), true
	}
	if q.has("comment") {
		p.Comment, changed = q.params.Get("comment"), true
	}
	if public != nil {
		p.Public, changed = *public, true
	}
	if changed {
		if err := a.app.Store.UpdatePlaylist(q.ctx, p); err != nil {
			return nil, err
		}
	}
	if len(remove) > 0 {
		if err := a.app.Store.RemovePlaylistPositions(q.ctx, id, remove); err != nil {
			return nil, err
		}
	}
	if add := q.strs("songIdToAdd"); len(add) > 0 {
		if err := a.app.Store.AppendPlaylistTracks(q.ctx, id, add); err != nil {
			return nil, err
		}
	}
	return newResponse(), nil
}

func (a *API) deletePlaylist(q *request) (*Response, error) {
	id, err := q.requiredStr("id")
	if err != nil {
		return nil, err
	}
	if _, err := a.writablePlaylist(q, id); err != nil {
		return nil, err
	}
	if err := a.app.Store.DeletePlaylist(q.ctx, id); err != nil {
		return nil, notFoundAs(err, "Playlist")
	}
	return newResponse(), nil
}
