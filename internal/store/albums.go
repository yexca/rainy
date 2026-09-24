package store

import (
	"context"
	"fmt"

	"rainy/internal/model"
)

const albumSelect = `SELECT al.id, al.library_id, al.name, al.sort_name, al.album_artist, al.album_artist_id, al.year,
	al.genre, al.compilation, al.song_count, al.disc_count, al.duration, al.size, al.cover_path, al.cover_track_id,
	al.mbz_album_id, al.search_text, al.created_at, al.updated_at, ` + annotationCols + `
	FROM albums al`

// AlbumQuery filters ListAlbums.
type AlbumQuery struct {
	UserID    string
	Q         string
	ArtistID  string // album artist
	Genre     string // albums containing a non-missing track of this genre
	LibraryID int64
	FromYear  int // if FromYear > ToYear the bounds are swapped (Subsonic byYear)
	ToYear    int
	Starred   bool
	Played    bool   // only albums played by UserID (play_count > 0)
	Sort      string // name, artist, year, recent, random, played, frequent, starred, rating, songCount, duration
	Order     string
	Offset    int
	Limit     int // 0 = unlimited
}

var albumSorts = map[string]sortSpec{
	"name":      {expr: "al.sort_name", then: []string{"al.name COLLATE NOCASE"}},
	"artist":    {expr: "COALESCE((SELECT ar.sort_name FROM artists ar WHERE ar.id = al.album_artist_id), lower(al.album_artist))", then: []string{"al.year", "al.sort_name"}},
	"year":      {expr: "al.year", then: []string{"al.sort_name"}},
	"recent":    {expr: "al.created_at", desc: true},
	"random":    {expr: "RANDOM()"},
	"played":    {expr: "COALESCE(an.played_at, 0)", desc: true},
	"frequent":  {expr: "COALESCE(an.play_count, 0)", desc: true, then: []string{"COALESCE(an.played_at, 0) DESC"}},
	"starred":   {expr: "an.starred_at", desc: true, nullsLast: true},
	"rating":    {expr: "COALESCE(an.rating, 0)", desc: true, then: []string{"al.sort_name"}},
	"songCount": {expr: "al.song_count", desc: true},
	"duration":  {expr: "al.duration", desc: true},
}

func (q *AlbumQuery) where() *where {
	w := &where{}
	w.tokens("al.search_text", q.Q)
	if q.ArtistID != "" {
		w.add("al.album_artist_id = ?", q.ArtistID)
	}
	if q.Genre != "" {
		w.add(`EXISTS (SELECT 1 FROM tracks t JOIN track_genres tg ON tg.track_id = t.id JOIN genres g ON g.id = tg.genre_id
			WHERE t.album_id = al.id AND t.missing = 0 AND g.name = ?)`, q.Genre)
	}
	if q.LibraryID > 0 {
		w.add("al.library_id = ?", q.LibraryID)
	}
	from, to := q.FromYear, q.ToYear
	if from > 0 && to > 0 && from > to {
		from, to = to, from
	}
	if from > 0 {
		w.add("al.year >= ?", from)
	}
	if to > 0 {
		w.add("al.year <= ?", to)
	}
	if q.Starred {
		w.add("an.starred_at IS NOT NULL")
	}
	if q.Played {
		w.add("COALESCE(an.play_count, 0) > 0")
	}
	return w
}

// ListAlbums returns a page of albums and the total number of matches.
func (s *Store) ListAlbums(ctx context.Context, q AlbumQuery) ([]model.Album, int, error) {
	w := q.where()
	args := append([]any{q.UserID}, w.args...)
	join := annotationJoin("album", "al.id")
	var total int
	if err := s.db.R.GetContext(ctx, &total, `SELECT COUNT(*) FROM albums al`+join+w.sql(), args...); err != nil {
		return nil, 0, fmt.Errorf("counting albums: %w", err)
	}
	if total == 0 || (q.Limit > 0 && q.Offset >= total) {
		return []model.Album{}, total, nil
	}
	spec, ok := albumSorts[q.Sort]
	if !ok {
		spec = albumSorts["name"]
	}
	var albums []model.Album
	query := albumSelect + join + w.sql() + orderBy(spec, q.Order, "al.id") + limitClause(q.Offset, q.Limit)
	if err := s.db.R.SelectContext(ctx, &albums, query, args...); err != nil {
		return nil, 0, fmt.Errorf("listing albums: %w", err)
	}
	return fillAlbums(albums), total, nil
}

// GetAlbum returns one album.
func (s *Store) GetAlbum(ctx context.Context, id, userID string) (*model.Album, error) {
	var a model.Album
	if err := s.db.R.GetContext(ctx, &a, albumSelect+annotationJoin("album", "al.id")+` WHERE al.id = ?`, userID, id); err != nil {
		return nil, notFound(err)
	}
	fillAlbum(&a)
	return &a, nil
}

// AlbumsAppearsOn returns albums containing non-missing tracks by artistID whose album
// artist is someone else (compilations, features), ordered by year and name.
func (s *Store) AlbumsAppearsOn(ctx context.Context, artistID, userID string) ([]model.Album, error) {
	var albums []model.Album
	err := s.db.R.SelectContext(ctx, &albums, albumSelect+annotationJoin("album", "al.id")+`
		WHERE al.album_artist_id != ? AND EXISTS (SELECT 1 FROM tracks t WHERE t.album_id = al.id AND t.missing = 0 AND t.artist_id = ?)
		ORDER BY al.year, al.sort_name, al.id`, userID, artistID, artistID)
	if err != nil {
		return nil, err
	}
	return fillAlbums(albums), nil
}

// ---- artists

const artistSelect = `SELECT ar.id, ar.name, ar.sort_name, ar.index_key, ar.album_count, ar.song_count, ar.mbz_artist_id,
	ar.image_path, ar.search_text, ar.created_at, ar.updated_at, ` + annotationCols + `
	FROM artists ar`

// ArtistQuery filters ListArtists.
type ArtistQuery struct {
	UserID           string
	Q                string
	LibraryID        int64 // artists with non-missing tracks in this library
	AlbumArtistsOnly bool  // album_count > 0
	Starred          bool
	Sort             string // name, albumCount, songCount, played, frequent, random, recent
	Order            string
	Offset           int
	Limit            int // 0 = unlimited
}

var artistSorts = map[string]sortSpec{
	"name":       {expr: "ar.sort_name", then: []string{"ar.name COLLATE NOCASE"}},
	"albumCount": {expr: "ar.album_count", desc: true, then: []string{"ar.sort_name"}},
	"songCount":  {expr: "ar.song_count", desc: true, then: []string{"ar.sort_name"}},
	"played":     {expr: "COALESCE(an.played_at, 0)", desc: true},
	"frequent":   {expr: "COALESCE(an.play_count, 0)", desc: true, then: []string{"ar.sort_name"}},
	"random":     {expr: "RANDOM()"},
	"recent":     {expr: "ar.created_at", desc: true},
	"starred":    {expr: "an.starred_at", desc: true, nullsLast: true},
	"rating":     {expr: "COALESCE(an.rating, 0)", desc: true, then: []string{"ar.sort_name"}},
}

func (q *ArtistQuery) where() *where {
	w := &where{}
	w.tokens("ar.search_text", q.Q)
	if q.LibraryID > 0 {
		w.add(`EXISTS (SELECT 1 FROM tracks t WHERE t.library_id = ? AND t.missing = 0 AND (t.artist_id = ar.id OR t.album_artist_id = ar.id))`, q.LibraryID)
	}
	if q.AlbumArtistsOnly {
		w.add("ar.album_count > 0")
	}
	if q.Starred {
		w.add("an.starred_at IS NOT NULL")
	}
	return w
}

// ListArtists returns a page of artists and the total number of matches.
func (s *Store) ListArtists(ctx context.Context, q ArtistQuery) ([]model.Artist, int, error) {
	w := q.where()
	args := append([]any{q.UserID}, w.args...)
	join := annotationJoin("artist", "ar.id")
	var total int
	if err := s.db.R.GetContext(ctx, &total, `SELECT COUNT(*) FROM artists ar`+join+w.sql(), args...); err != nil {
		return nil, 0, fmt.Errorf("counting artists: %w", err)
	}
	if total == 0 || (q.Limit > 0 && q.Offset >= total) {
		return []model.Artist{}, total, nil
	}
	spec, ok := artistSorts[q.Sort]
	if !ok {
		spec = artistSorts["name"]
	}
	var artists []model.Artist
	query := artistSelect + join + w.sql() + orderBy(spec, q.Order, "ar.id") + limitClause(q.Offset, q.Limit)
	if err := s.db.R.SelectContext(ctx, &artists, query, args...); err != nil {
		return nil, 0, fmt.Errorf("listing artists: %w", err)
	}
	return fillArtists(artists), total, nil
}

// GetArtist returns one artist.
func (s *Store) GetArtist(ctx context.Context, id, userID string) (*model.Artist, error) {
	var a model.Artist
	if err := s.db.R.GetContext(ctx, &a, artistSelect+annotationJoin("artist", "ar.id")+` WHERE ar.id = ?`, userID, id); err != nil {
		return nil, notFound(err)
	}
	fillArtist(&a)
	return &a, nil
}

// ListGenres returns genres that have non-missing tracks, with song/album counts, sorted
// by name.
func (s *Store) ListGenres(ctx context.Context) ([]model.Genre, error) {
	genres := []model.Genre{}
	err := s.db.R.SelectContext(ctx, &genres, `SELECT g.id, g.name, COUNT(DISTINCT t.id) AS song_count,
			COUNT(DISTINCT t.album_id) AS album_count
		FROM genres g
		JOIN track_genres tg ON tg.genre_id = g.id
		JOIN tracks t ON t.id = tg.track_id AND t.missing = 0
		GROUP BY g.id, g.name
		ORDER BY g.name COLLATE NOCASE, g.id`)
	return genres, err
}

// ---- search

// SearchQuery is a combined artist/album/track search. A limit <= 0 returns no items of
// that kind (Subsonic semantics: artistCount=0 means none).
type SearchQuery struct {
	UserID       string
	Q            string // "" matches everything (Subsonic search3 full sync)
	LibraryID    int64
	ArtistOffset int
	ArtistLimit  int
	AlbumOffset  int
	AlbumLimit   int
	TrackOffset  int
	TrackLimit   int
}

// SearchResult holds the matches of Search; slices are never nil.
type SearchResult struct {
	Artists []model.Artist `json:"artists"`
	Albums  []model.Album  `json:"albums"`
	Tracks  []model.Track  `json:"tracks"`
}

// Search matches every token of q (NormalizeSearch, LIKE %tok%) against search_text of
// artists (only those with albums or songs), albums and non-missing tracks.
func (s *Store) Search(ctx context.Context, q SearchQuery) (*SearchResult, error) {
	res := &SearchResult{Artists: []model.Artist{}, Albums: []model.Album{}, Tracks: []model.Track{}}
	if q.ArtistLimit > 0 {
		w := &where{}
		w.tokens("ar.search_text", q.Q)
		w.add("(ar.album_count > 0 OR ar.song_count > 0)")
		if q.LibraryID > 0 {
			w.add(`EXISTS (SELECT 1 FROM tracks t WHERE t.library_id = ? AND t.missing = 0 AND (t.artist_id = ar.id OR t.album_artist_id = ar.id))`, q.LibraryID)
		}
		var artists []model.Artist
		query := artistSelect + annotationJoin("artist", "ar.id") + w.sql() + ` ORDER BY ar.sort_name, ar.id` + limitClause(q.ArtistOffset, q.ArtistLimit)
		if err := s.db.R.SelectContext(ctx, &artists, query, append([]any{q.UserID}, w.args...)...); err != nil {
			return nil, fmt.Errorf("searching artists: %w", err)
		}
		res.Artists = fillArtists(artists)
	}
	if q.AlbumLimit > 0 {
		aq := AlbumQuery{UserID: q.UserID, Q: q.Q, LibraryID: q.LibraryID}
		w := aq.where()
		var albums []model.Album
		query := albumSelect + annotationJoin("album", "al.id") + w.sql() + ` ORDER BY al.sort_name, al.id` + limitClause(q.AlbumOffset, q.AlbumLimit)
		if err := s.db.R.SelectContext(ctx, &albums, query, append([]any{q.UserID}, w.args...)...); err != nil {
			return nil, fmt.Errorf("searching albums: %w", err)
		}
		res.Albums = fillAlbums(albums)
	}
	if q.TrackLimit > 0 {
		tq := TrackQuery{UserID: q.UserID, Q: q.Q, LibraryID: q.LibraryID}
		w := tq.where()
		var tracks []model.Track
		query := trackSelect("tracks t", false) + w.sql() +
			` ORDER BY COALESCE(NULLIF(t.sort_title, ''), t.title) COLLATE NOCASE, t.id` + limitClause(q.TrackOffset, q.TrackLimit)
		if err := s.db.R.SelectContext(ctx, &tracks, query, append([]any{q.UserID}, w.args...)...); err != nil {
			return nil, fmt.Errorf("searching tracks: %w", err)
		}
		res.Tracks = fillTracks(tracks)
	}
	return res, nil
}
