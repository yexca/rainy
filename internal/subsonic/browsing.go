package subsonic

import (
	"context"
	"database/sql"
	"errors"
	"sort"
	"strings"

	"rainy/internal/model"
	"rainy/internal/store"
	"rainy/internal/util"
)

// Item kinds resolved by itemKind.
const (
	kindTrack  = "track"
	kindAlbum  = "album"
	kindArtist = "artist"
)

// itemKind reports whether id is a track, album or artist (ErrNotFound otherwise).
func (a *API) itemKind(ctx context.Context, id string) (string, error) {
	var kind string
	err := a.app.Store.DB().R.GetContext(ctx, &kind, `SELECT kind FROM (
			SELECT 'track' AS kind, 1 AS o FROM tracks WHERE id = ?
			UNION ALL SELECT 'album', 2 FROM albums WHERE id = ?
			UNION ALL SELECT 'artist', 3 FROM artists WHERE id = ?
		) ORDER BY o LIMIT 1`, id, id, id)
	if errors.Is(err, sql.ErrNoRows) {
		return "", store.ErrNotFound
	}
	return kind, err
}

func (a *API) getMusicFolders(q *request) (*Response, error) {
	libs, err := a.app.Store.ListLibraries(q.ctx)
	if err != nil {
		return nil, err
	}
	folders := make([]MusicFolder, 0, len(libs))
	for _, l := range libs {
		folders = append(folders, MusicFolder{ID: l.ID, Name: l.Name})
	}
	resp := newResponse()
	resp.MusicFolders = &MusicFolders{Folders: folders}
	return resp, nil
}

// lastModified is the newest artist change or library scan (ms).
func (a *API) lastModified(ctx context.Context) (int64, error) {
	var ms int64
	err := a.app.Store.DB().R.GetContext(ctx, &ms, `SELECT MAX(
		COALESCE((SELECT MAX(updated_at) FROM artists), 0),
		COALESCE((SELECT MAX(last_scan_at) FROM libraries), 0))`)
	return ms, err
}

// indexArtists returns the album artists (optionally of one library) grouped by index key:
// "A".."Z" in order, then "#".
func (a *API) indexArtists(q *request, libraryID int64) ([]string, map[string][]model.Artist, error) {
	artists, _, err := a.app.Store.ListArtists(q.ctx, store.ArtistQuery{
		UserID: q.user.ID, LibraryID: libraryID, AlbumArtistsOnly: true, Sort: "name",
	})
	if err != nil {
		return nil, nil, err
	}
	groups := map[string][]model.Artist{}
	var keys []string
	for _, ar := range artists {
		k := ar.IndexKey
		if k == "" {
			k = "#"
		}
		if _, ok := groups[k]; !ok {
			keys = append(keys, k)
		}
		groups[k] = append(groups[k], ar)
	}
	sort.Slice(keys, func(i, j int) bool {
		if (keys[i] == "#") != (keys[j] == "#") {
			return keys[j] == "#"
		}
		return keys[i] < keys[j]
	})
	return keys, groups, nil
}

func (a *API) getIndexes(q *request) (*Response, error) {
	lib, err := q.musicFolder()
	if err != nil {
		return nil, err
	}
	since, err := q.int64Param("ifModifiedSince", 0)
	if err != nil {
		return nil, err
	}
	lastMod, err := a.lastModified(q.ctx)
	if err != nil {
		return nil, err
	}
	ind := &Indexes{LastModified: lastMod, IgnoredArticles: q.appSettings().IgnoredArticles, Index: []Index{}}
	if since <= 0 || lastMod > since {
		keys, groups, err := a.indexArtists(q, lib)
		if err != nil {
			return nil, err
		}
		for _, k := range keys {
			ind.Index = append(ind.Index, Index{Name: k, Artists: folderArtists(groups[k])})
		}
	}
	resp := newResponse()
	resp.Indexes = ind
	return resp, nil
}

func (a *API) getArtists(q *request) (*Response, error) {
	lib, err := q.musicFolder()
	if err != nil {
		return nil, err
	}
	keys, groups, err := a.indexArtists(q, lib)
	if err != nil {
		return nil, err
	}
	out := &ArtistsID3{IgnoredArticles: q.appSettings().IgnoredArticles, Index: make([]IndexID3, 0, len(keys))}
	for _, k := range keys {
		out.Index = append(out.Index, IndexID3{Name: k, Artists: artistsID3(groups[k])})
	}
	resp := newResponse()
	resp.Artists = out
	return resp, nil
}

// artistAlbums returns the albums of an artist (as album artist, by year); for artists
// that only appear on other artists' albums, those albums instead.
func (a *API) artistAlbums(q *request, ar *model.Artist) ([]model.Album, error) {
	albums, _, err := a.app.Store.ListAlbums(q.ctx, store.AlbumQuery{UserID: q.user.ID, ArtistID: ar.ID, Sort: "year"})
	if err != nil || len(albums) > 0 {
		return albums, err
	}
	return a.app.Store.AlbumsAppearsOn(q.ctx, ar.ID, q.user.ID)
}

func (a *API) getMusicDirectory(q *request) (*Response, error) {
	id, err := q.requiredStr("id")
	if err != nil {
		return nil, err
	}
	kind, err := a.itemKind(q.ctx, id)
	if err != nil {
		return nil, errNotFound("Directory")
	}
	var dir *Directory
	switch kind {
	case kindArtist:
		ar, err := a.app.Store.GetArtist(q.ctx, id, q.user.ID)
		if err != nil {
			return nil, err
		}
		albums, err := a.artistAlbums(q, ar)
		if err != nil {
			return nil, err
		}
		dir = &Directory{
			ID: ar.ID, Name: ar.Name, Starred: datePtr(ar.StarredAt), UserRating: ar.Rating,
			PlayCount: ar.PlayCount, Played: date(ar.PlayedAt), Children: albumChildren(albums),
		}
	case kindAlbum:
		al, err := a.app.Store.GetAlbum(q.ctx, id, q.user.ID)
		if err != nil {
			return nil, err
		}
		tracks, _, err := a.app.Store.ListTracks(q.ctx, store.TrackQuery{UserID: q.user.ID, AlbumID: id, Sort: "track"})
		if err != nil {
			return nil, err
		}
		dir = &Directory{
			ID: al.ID, Parent: al.AlbumArtistID, Name: al.Name, Starred: datePtr(al.StarredAt), UserRating: al.Rating,
			PlayCount: al.PlayCount, Played: date(al.PlayedAt), Children: q.children(tracks),
		}
	default:
		return nil, errNotFound("Directory")
	}
	resp := newResponse()
	resp.Directory = dir
	return resp, nil
}

func (a *API) getGenres(q *request) (*Response, error) {
	genres, err := a.app.Store.ListGenres(q.ctx)
	if err != nil {
		return nil, err
	}
	out := make([]Genre, 0, len(genres))
	for _, g := range genres {
		out = append(out, Genre{Name: g.Name, SongCount: g.SongCount, AlbumCount: g.AlbumCount})
	}
	resp := newResponse()
	resp.Genres = &Genres{Genres: out}
	return resp, nil
}

func (a *API) getArtist(q *request) (*Response, error) {
	id, err := q.requiredStr("id")
	if err != nil {
		return nil, err
	}
	ar, err := a.app.Store.GetArtist(q.ctx, id, q.user.ID)
	if err != nil {
		return nil, notFoundAs(err, "Artist")
	}
	albums, err := a.artistAlbums(q, ar)
	if err != nil {
		return nil, err
	}
	resp := newResponse()
	resp.Artist = &ArtistWithAlbumsID3{ArtistID3: artistID3(ar), Albums: albumsID3(albums)}
	return resp, nil
}

func (a *API) getAlbum(q *request) (*Response, error) {
	id, err := q.requiredStr("id")
	if err != nil {
		return nil, err
	}
	al, err := a.app.Store.GetAlbum(q.ctx, id, q.user.ID)
	if err != nil {
		return nil, notFoundAs(err, "Album")
	}
	tracks, _, err := a.app.Store.ListTracks(q.ctx, store.TrackQuery{UserID: q.user.ID, AlbumID: id, Sort: "track"})
	if err != nil {
		return nil, err
	}
	out := &AlbumWithSongsID3{AlbumID3: albumID3(al), Songs: q.children(tracks)}
	out.DiscTitles = discTitles(tracks)
	resp := newResponse()
	resp.Album = out
	return resp, nil
}

func (a *API) getSong(q *request) (*Response, error) {
	id, err := q.requiredStr("id")
	if err != nil {
		return nil, err
	}
	t, err := a.app.Store.GetTrack(q.ctx, id, q.user.ID)
	if err != nil {
		return nil, notFoundAs(err, "Song")
	}
	c := q.child(t)
	resp := newResponse()
	resp.Song = &c
	return resp, nil
}

// notFoundAs turns store.ErrNotFound into a named error 70.
func notFoundAs(err error, what string) error {
	if errors.Is(err, store.ErrNotFound) {
		return errNotFound(what)
	}
	return err
}

// ---- info, similar & top songs (computed locally from the library: shared genres)

// resolveArtist maps an artist, album or song id to an artist id.
func (a *API) resolveArtist(q *request, id string) (string, error) {
	kind, err := a.itemKind(q.ctx, id)
	if err != nil {
		return "", errNotFound("Artist")
	}
	switch kind {
	case kindAlbum:
		al, err := a.app.Store.GetAlbum(q.ctx, id, "")
		if err != nil {
			return "", err
		}
		return al.AlbumArtistID, nil
	case kindTrack:
		t, err := a.app.Store.GetTrack(q.ctx, id, "")
		if err != nil {
			return "", err
		}
		return t.ArtistID, nil
	}
	return id, nil
}

// similarArtistIDs returns album artists sharing the most genres with artistID.
func (a *API) similarArtistIDs(ctx context.Context, artistID string, limit int) ([]string, error) {
	if limit <= 0 {
		return nil, nil
	}
	var ids []string
	err := a.app.Store.DB().R.SelectContext(ctx, &ids, `
		WITH g AS (
			SELECT DISTINCT tg.genre_id FROM track_genres tg JOIN tracks t ON t.id = tg.track_id
			WHERE t.missing = 0 AND (t.artist_id = ? OR t.album_artist_id = ?)
		)
		SELECT t.album_artist_id FROM tracks t
		JOIN track_genres tg ON tg.track_id = t.id
		JOIN artists ar ON ar.id = t.album_artist_id AND ar.album_count > 0
		WHERE t.missing = 0 AND tg.genre_id IN (SELECT genre_id FROM g) AND t.album_artist_id != ?
		GROUP BY t.album_artist_id
		ORDER BY COUNT(DISTINCT tg.genre_id) DESC, COUNT(*) DESC, t.album_artist_id
		LIMIT ?`, artistID, artistID, artistID, limit)
	return ids, err
}

func (a *API) similarArtists(q *request, artistID string, limit int) ([]model.Artist, error) {
	ids, err := a.similarArtistIDs(q.ctx, artistID, limit)
	if err != nil {
		return nil, err
	}
	out := make([]model.Artist, 0, len(ids))
	for _, id := range ids {
		ar, err := a.app.Store.GetArtist(q.ctx, id, q.user.ID)
		if errors.Is(err, store.ErrNotFound) {
			continue
		}
		if err != nil {
			return nil, err
		}
		out = append(out, *ar)
	}
	return out, nil
}

// artistInfo resolves the artist and its similar artists.
func (a *API) artistInfo(q *request) (*model.Artist, []model.Artist, error) {
	id, err := q.requiredStr("id")
	if err != nil {
		return nil, nil, err
	}
	count, _, err := q.pageParams("count", "", 20, 100)
	if err != nil {
		return nil, nil, err
	}
	artistID, err := a.resolveArtist(q, id)
	if err != nil {
		return nil, nil, err
	}
	ar, err := a.app.Store.GetArtist(q.ctx, artistID, q.user.ID)
	if err != nil {
		return nil, nil, notFoundAs(err, "Artist")
	}
	similar, err := a.similarArtists(q, ar.ID, count)
	return ar, similar, err
}

func (a *API) getArtistInfo(q *request) (*Response, error) {
	ar, similar, err := a.artistInfo(q)
	if err != nil {
		return nil, err
	}
	resp := newResponse()
	resp.ArtistInfo = &ArtistInfo{MusicBrainzID: ar.MbzArtistID, SimilarArtists: folderArtists(similar)}
	return resp, nil
}

func (a *API) getArtistInfo2(q *request) (*Response, error) {
	ar, similar, err := a.artistInfo(q)
	if err != nil {
		return nil, err
	}
	resp := newResponse()
	resp.ArtistInfo2 = &ArtistInfo2{MusicBrainzID: ar.MbzArtistID, SimilarArtists: artistsID3(similar)}
	return resp, nil
}

// getAlbumInfo serves getAlbumInfo and getAlbumInfo2 (album or song id).
func (a *API) getAlbumInfo(q *request) (*Response, error) {
	id, err := q.requiredStr("id")
	if err != nil {
		return nil, err
	}
	kind, err := a.itemKind(q.ctx, id)
	if err != nil {
		return nil, errNotFound("Album")
	}
	albumID := id
	switch kind {
	case kindTrack:
		t, err := a.app.Store.GetTrack(q.ctx, id, "")
		if err != nil {
			return nil, err
		}
		albumID = t.AlbumID
	case kindArtist:
		return nil, errNotFound("Album")
	}
	al, err := a.app.Store.GetAlbum(q.ctx, albumID, q.user.ID)
	if err != nil {
		return nil, notFoundAs(err, "Album")
	}
	resp := newResponse()
	resp.AlbumInfo = &AlbumInfo{MusicBrainzID: al.MbzAlbumID}
	return resp, nil
}

// similarSongs returns random songs by the seed's artist and by similar artists.
func (a *API) similarSongs(q *request) ([]Child, error) {
	id, err := q.requiredStr("id")
	if err != nil {
		return nil, err
	}
	count, _, err := q.pageParams("count", "", 50, 500)
	if err != nil {
		return nil, err
	}
	artistID, err := a.resolveArtist(q, id)
	if err != nil {
		return nil, err
	}
	similar, err := a.similarArtistIDs(q.ctx, artistID, 10)
	if err != nil {
		return nil, err
	}
	artistIDs := append([]string{artistID}, similar...)
	ph := strings.TrimSuffix(strings.Repeat("?, ", len(artistIDs)), ", ")
	args := make([]any, 0, 2*len(artistIDs)+2)
	for range 2 {
		for _, v := range artistIDs {
			args = append(args, v)
		}
	}
	args = append(args, id, count)
	var ids []string
	err = a.app.Store.DB().R.SelectContext(q.ctx, &ids, `SELECT t.id FROM tracks t
		WHERE t.missing = 0 AND (t.artist_id IN (`+ph+`) OR t.album_artist_id IN (`+ph+`)) AND t.id != ?
		ORDER BY RANDOM() LIMIT ?`, args...)
	if err != nil {
		return nil, err
	}
	tracks, err := a.app.Store.GetTracks(q.ctx, ids, q.user.ID)
	if err != nil {
		return nil, err
	}
	return q.children(tracks), nil
}

func (a *API) getSimilarSongs(q *request) (*Response, error) {
	songs, err := a.similarSongs(q)
	if err != nil {
		return nil, err
	}
	resp := newResponse()
	resp.SimilarSongs = &Songs{Songs: songs}
	return resp, nil
}

func (a *API) getSimilarSongs2(q *request) (*Response, error) {
	songs, err := a.similarSongs(q)
	if err != nil {
		return nil, err
	}
	resp := newResponse()
	resp.SimilarSongs2 = &Songs{Songs: songs}
	return resp, nil
}

// getTopSongs returns an artist's (by name) songs ordered by total plays of all users.
func (a *API) getTopSongs(q *request) (*Response, error) {
	name, err := q.requiredStr("artist")
	if err != nil {
		return nil, err
	}
	count, _, err := q.pageParams("count", "", 50, 500)
	if err != nil {
		return nil, err
	}
	resp := newResponse()
	resp.TopSongs = &Songs{Songs: []Child{}}
	artistID := util.ArtistID(name)
	var ids []string
	err = a.app.Store.DB().R.SelectContext(q.ctx, &ids, `SELECT t.id FROM tracks t
		LEFT JOIN (SELECT item_id, SUM(play_count) AS plays FROM annotations WHERE item_type = 'track' GROUP BY item_id) p
			ON p.item_id = t.id
		WHERE t.missing = 0 AND (t.artist_id = ? OR t.album_artist_id = ?)
		ORDER BY COALESCE(p.plays, 0) DESC, t.year, t.album_id, t.disc_number, t.track_number, t.id
		LIMIT ?`, artistID, artistID, count)
	if err != nil {
		return nil, err
	}
	tracks, err := a.app.Store.GetTracks(q.ctx, ids, q.user.ID)
	if err != nil {
		return nil, err
	}
	resp.TopSongs.Songs = q.children(tracks)
	return resp, nil
}
