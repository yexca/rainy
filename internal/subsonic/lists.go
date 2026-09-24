package subsonic

import (
	"errors"
	"strings"
	"time"

	"rainy/internal/model"
	"rainy/internal/store"
)

// albumListQuery builds the store query for getAlbumList(2) (type, size, offset,
// fromYear, toYear, genre, musicFolderId).
func (q *request) albumListQuery() (store.AlbumQuery, error) {
	listType, err := q.requiredStr("type")
	if err != nil {
		return store.AlbumQuery{}, err
	}
	size, offset, err := q.pageParams("size", "offset", 10, 500)
	if err != nil {
		return store.AlbumQuery{}, err
	}
	lib, err := q.musicFolder()
	if err != nil {
		return store.AlbumQuery{}, err
	}
	aq := store.AlbumQuery{UserID: q.user.ID, LibraryID: lib, Offset: offset, Limit: size}
	switch listType {
	case "random":
		aq.Sort, aq.Offset = "random", 0
	case "newest":
		aq.Sort = "recent"
	case "highest":
		aq.Sort = "rating"
	case "frequent":
		aq.Sort, aq.Played = "frequent", true
	case "recent":
		aq.Sort, aq.Played = "played", true
	case "alphabeticalByName":
		aq.Sort = "name"
	case "alphabeticalByArtist":
		aq.Sort = "artist"
	case "starred":
		aq.Sort, aq.Starred = "starred", true
	case "byYear":
		from, err := q.intParam("fromYear", 0)
		if err != nil {
			return aq, err
		}
		to, err := q.intParam("toYear", 0)
		if err != nil {
			return aq, err
		}
		if !q.has("fromYear") {
			return aq, errMissing("fromYear")
		}
		if !q.has("toYear") {
			return aq, errMissing("toYear")
		}
		aq.Sort, aq.FromYear, aq.ToYear = "year", from, to
		if from > to {
			aq.Order = "desc"
		}
	case "byGenre":
		g, err := q.requiredStr("genre")
		if err != nil {
			return aq, err
		}
		aq.Sort, aq.Genre = "name", g
	default:
		return aq, newError(codeGeneric, "Unknown album list type: %s", listType)
	}
	return aq, nil
}

// albumList runs getAlbumList(2). "highest" only returns rated albums.
func (a *API) albumList(q *request) ([]model.Album, error) {
	aq, err := q.albumListQuery()
	if err != nil {
		return nil, err
	}
	if aq.Limit == 0 {
		return []model.Album{}, nil
	}
	if q.str("type") == "highest" {
		return a.ratedAlbums(q, aq)
	}
	albums, _, err := a.app.Store.ListAlbums(q.ctx, aq)
	return albums, err
}

// ratedAlbums lists the user's rated albums (rating desc, then name).
func (a *API) ratedAlbums(q *request, aq store.AlbumQuery) ([]model.Album, error) {
	query := `SELECT al.id FROM albums al JOIN annotations an ON an.user_id = ? AND an.item_type = 'album'
		AND an.item_id = al.id AND an.rating > 0`
	args := []any{q.user.ID}
	if aq.LibraryID > 0 {
		query += ` WHERE al.library_id = ?`
		args = append(args, aq.LibraryID)
	}
	query += ` ORDER BY an.rating DESC, al.sort_name, al.id LIMIT ? OFFSET ?`
	args = append(args, aq.Limit, aq.Offset)
	var ids []string
	if err := a.app.Store.DB().R.SelectContext(q.ctx, &ids, query, args...); err != nil {
		return nil, err
	}
	out := make([]model.Album, 0, len(ids))
	for _, id := range ids {
		al, err := a.app.Store.GetAlbum(q.ctx, id, q.user.ID)
		if errors.Is(err, store.ErrNotFound) {
			continue
		}
		if err != nil {
			return nil, err
		}
		out = append(out, *al)
	}
	return out, nil
}

func (a *API) getAlbumList(q *request) (*Response, error) {
	albums, err := a.albumList(q)
	if err != nil {
		return nil, err
	}
	resp := newResponse()
	resp.AlbumList = &AlbumList{Albums: albumChildren(albums)}
	return resp, nil
}

func (a *API) getAlbumList2(q *request) (*Response, error) {
	albums, err := a.albumList(q)
	if err != nil {
		return nil, err
	}
	resp := newResponse()
	resp.AlbumList2 = &AlbumList2{Albums: albumsID3(albums)}
	return resp, nil
}

func (a *API) getRandomSongs(q *request) (*Response, error) {
	size, _, err := q.pageParams("size", "", 10, 500)
	if err != nil {
		return nil, err
	}
	lib, err := q.musicFolder()
	if err != nil {
		return nil, err
	}
	from, err := q.intParam("fromYear", 0)
	if err != nil {
		return nil, err
	}
	to, err := q.intParam("toYear", 0)
	if err != nil {
		return nil, err
	}
	resp := newResponse()
	resp.RandomSongs = &Songs{Songs: []Child{}}
	if size == 0 {
		return resp, nil
	}
	tracks, _, err := a.app.Store.ListTracks(q.ctx, store.TrackQuery{
		UserID: q.user.ID, Genre: q.str("genre"), LibraryID: lib, FromYear: from, ToYear: to,
		Sort: "random", Limit: size,
	})
	if err != nil {
		return nil, err
	}
	resp.RandomSongs.Songs = q.children(tracks)
	return resp, nil
}

func (a *API) getSongsByGenre(q *request) (*Response, error) {
	genre, err := q.requiredStr("genre")
	if err != nil {
		return nil, err
	}
	count, offset, err := q.pageParams("count", "offset", 10, 500)
	if err != nil {
		return nil, err
	}
	lib, err := q.musicFolder()
	if err != nil {
		return nil, err
	}
	resp := newResponse()
	resp.SongsByGenre = &Songs{Songs: []Child{}}
	if count == 0 {
		return resp, nil
	}
	tracks, _, err := a.app.Store.ListTracks(q.ctx, store.TrackQuery{
		UserID: q.user.ID, Genre: genre, LibraryID: lib, Sort: "albumArtist", Offset: offset, Limit: count,
	})
	if err != nil {
		return nil, err
	}
	resp.SongsByGenre.Songs = q.children(tracks)
	return resp, nil
}

func (a *API) getNowPlaying(q *request) (*Response, error) {
	entries := a.app.NowPlaying.List()
	out := make([]NowPlayingEntry, 0, len(entries))
	now := time.Now().UnixMilli()
	for _, e := range entries {
		t, err := a.app.Store.GetTrack(q.ctx, e.TrackID, e.UserID)
		if errors.Is(err, store.ErrNotFound) {
			continue
		}
		if err != nil {
			return nil, err
		}
		out = append(out, NowPlayingEntry{
			Child:      q.child(t),
			Username:   e.Username,
			MinutesAgo: int(max(0, now-e.Since) / 60000),
			PlayerName: e.Player,
		})
	}
	resp := newResponse()
	resp.NowPlaying = &NowPlaying{Entries: out}
	return resp, nil
}

// starredItems loads the user's starred artists, albums and songs.
func (a *API) starredItems(q *request) ([]model.Artist, []model.Album, []model.Track, error) {
	lib, err := q.musicFolder()
	if err != nil {
		return nil, nil, nil, err
	}
	artists, _, err := a.app.Store.ListArtists(q.ctx, store.ArtistQuery{UserID: q.user.ID, LibraryID: lib, Starred: true, Sort: "starred"})
	if err != nil {
		return nil, nil, nil, err
	}
	albums, _, err := a.app.Store.ListAlbums(q.ctx, store.AlbumQuery{UserID: q.user.ID, LibraryID: lib, Starred: true, Sort: "starred"})
	if err != nil {
		return nil, nil, nil, err
	}
	tracks, _, err := a.app.Store.ListTracks(q.ctx, store.TrackQuery{UserID: q.user.ID, LibraryID: lib, Starred: true, Sort: "starred"})
	if err != nil {
		return nil, nil, nil, err
	}
	return artists, albums, tracks, nil
}

func (a *API) getStarred(q *request) (*Response, error) {
	artists, albums, tracks, err := a.starredItems(q)
	if err != nil {
		return nil, err
	}
	resp := newResponse()
	resp.Starred = &Starred{Artists: folderArtists(artists), Albums: albumChildren(albums), Songs: q.children(tracks)}
	return resp, nil
}

func (a *API) getStarred2(q *request) (*Response, error) {
	artists, albums, tracks, err := a.starredItems(q)
	if err != nil {
		return nil, err
	}
	resp := newResponse()
	resp.Starred2 = &Starred2{Artists: artistsID3(artists), Albums: albumsID3(albums), Songs: q.children(tracks)}
	return resp, nil
}

// searchQuery normalises a search string: surrounding quotes and a trailing "*" (Lucene
// syntax sent by some clients) are dropped; `""` means "everything".
func searchQuery(s string) string {
	s = strings.TrimSpace(s)
	s = strings.Trim(s, `"`)
	s = strings.TrimSuffix(s, "*")
	return strings.TrimSpace(s)
}
