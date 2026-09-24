package store

import (
	"context"
	"fmt"
	"math"

	"github.com/jmoiron/sqlx"

	"rainy/internal/model"
	"rainy/internal/util"
)

// tally counts string occurrences and returns the most common one; ties go to the value
// seen first (callers feed values in album order, so the first track wins ties).
type tally struct {
	order []string
	n     map[string]int
}

func (c *tally) add(v string) {
	if v == "" {
		return
	}
	if c.n == nil {
		c.n = map[string]int{}
	}
	if _, ok := c.n[v]; !ok {
		c.order = append(c.order, v)
	}
	c.n[v]++
}

func (c *tally) top() string {
	best, bestN := "", 0
	for _, v := range c.order {
		if c.n[v] > bestN {
			best, bestN = v, c.n[v]
		}
	}
	return best
}

// ignoredArticles reads the configured articles (defaults when unset/unreadable).
func (s *Store) ignoredArticles(ctx context.Context) []string {
	set, err := s.GetSettings(ctx, model.DefaultSettings(0))
	if err != nil {
		set = model.DefaultSettings(0)
	}
	return set.IgnoredArticleList()
}

// RefreshAll recomputes every album, artist and genre.
func (s *Store) RefreshAll(ctx context.Context) error {
	if err := s.RefreshAlbums(ctx, nil); err != nil {
		return err
	}
	if err := s.RefreshArtists(ctx, nil); err != nil {
		return err
	}
	return s.RefreshGenres(ctx)
}

// RefreshGenres deletes genres no track references any more.
func (s *Store) RefreshGenres(ctx context.Context) error {
	_, err := s.db.W.ExecContext(ctx, `DELETE FROM genres WHERE id NOT IN (SELECT DISTINCT genre_id FROM track_genres)`)
	return err
}

// ---- albums

type albumTrackRow struct {
	ID            string  `db:"id"`
	AlbumID       string  `db:"album_id"`
	LibraryID     int64   `db:"library_id"`
	Album         string  `db:"album"`
	AlbumArtist   string  `db:"album_artist"`
	AlbumArtistID string  `db:"album_artist_id"`
	SortAlbum     string  `db:"sort_album"`
	Year          int     `db:"year"`
	Genre         string  `db:"genre"`
	Compilation   bool    `db:"compilation"`
	DiscNumber    int     `db:"disc_number"`
	HasCover      bool    `db:"has_cover"`
	Duration      float64 `db:"duration"`
	Size          int64   `db:"size"`
	MbzAlbumID    string  `db:"mbz_album_id"`
	CreatedAt     int64   `db:"created_at"`
	UpdatedAt     int64   `db:"updated_at"`
}

const albumTrackRowSQL = `SELECT id, album_id, library_id, album, album_artist, album_artist_id, sort_album, year, genre,
	compilation, disc_number, has_cover, duration, size, mbz_album_id, created_at, updated_at
	FROM tracks WHERE missing = 0`

const albumOrder = ` ORDER BY album_id, disc_number, track_number, path, id`

type albumAgg struct {
	names, artists, sorts, genres, mbz tally
	artistNames                        map[string]string // album_artist_id → first name seen
	libs                               map[int64]int
	discs                              map[int]bool
	maxDisc                            int
	a                                  model.Album
	maxTrackUpdated                    int64
}

func (g *albumAgg) add(r *albumTrackRow) {
	if g.a.SongCount == 0 {
		g.a.CreatedAt = r.CreatedAt
		g.artistNames = map[string]string{}
		g.libs = map[int64]int{}
		g.discs = map[int]bool{}
	}
	g.a.SongCount++
	g.names.add(r.Album)
	g.artists.add(r.AlbumArtistID)
	if _, ok := g.artistNames[r.AlbumArtistID]; !ok {
		g.artistNames[r.AlbumArtistID] = r.AlbumArtist
	}
	g.sorts.add(r.SortAlbum)
	g.genres.add(r.Genre)
	g.mbz.add(r.MbzAlbumID)
	g.libs[r.LibraryID]++
	disc := r.DiscNumber
	if disc < 1 {
		disc = 1
	}
	g.discs[disc] = true
	g.maxDisc = max(g.maxDisc, disc)
	g.a.Year = max(g.a.Year, r.Year)
	g.a.Compilation = g.a.Compilation || r.Compilation
	g.a.Duration += r.Duration
	g.a.Size += r.Size
	g.a.CreatedAt = min(g.a.CreatedAt, r.CreatedAt)
	g.maxTrackUpdated = max(g.maxTrackUpdated, r.UpdatedAt)
	if r.HasCover && g.a.CoverTrackID == "" {
		g.a.CoverTrackID = r.ID // rows arrive ordered by disc, track
	}
}

func (g *albumAgg) album(id string, articles []string) model.Album {
	a := g.a
	a.ID = id
	a.Name = g.names.top()
	a.AlbumArtistID = g.artists.top()
	a.AlbumArtist = g.artistNames[a.AlbumArtistID]
	if st := g.sorts.top(); st != "" {
		a.SortName = util.NormalizeSearch(st)
	} else {
		a.SortName = util.SortName(a.Name, articles)
	}
	a.Genre = g.genres.top()
	a.MbzAlbumID = g.mbz.top()
	bestLib, bestN := int64(0), 0
	for lib, n := range g.libs {
		if n > bestN || (n == bestN && lib < bestLib) {
			bestLib, bestN = lib, n
		}
	}
	a.LibraryID = bestLib
	a.DiscCount = max(len(g.discs), g.maxDisc)
	a.SearchText = util.NormalizeSearch(a.Name, a.AlbumArtist)
	return a
}

func albumChanged(old, n *model.Album) bool {
	return old.LibraryID != n.LibraryID || old.Name != n.Name || old.SortName != n.SortName ||
		old.AlbumArtist != n.AlbumArtist || old.AlbumArtistID != n.AlbumArtistID || old.Year != n.Year ||
		old.Genre != n.Genre || old.Compilation != n.Compilation || old.SongCount != n.SongCount ||
		old.DiscCount != n.DiscCount || math.Abs(old.Duration-n.Duration) > 0.0005 || old.Size != n.Size ||
		old.CoverTrackID != n.CoverTrackID || old.MbzAlbumID != n.MbzAlbumID || old.SearchText != n.SearchText ||
		old.CreatedAt != n.CreatedAt
}

const albumStoredCols = `id, library_id, name, sort_name, album_artist, album_artist_id, year, genre, compilation,
	song_count, disc_count, duration, size, cover_path, cover_track_id, mbz_album_id, search_text, created_at, updated_at`

const albumUpsertSQL = `INSERT INTO albums (` + albumStoredCols + `)
	VALUES (:id, :library_id, :name, :sort_name, :album_artist, :album_artist_id, :year, :genre, :compilation,
		:song_count, :disc_count, :duration, :size, :cover_path, :cover_track_id, :mbz_album_id, :search_text, :created_at, :updated_at)
	ON CONFLICT (id) DO UPDATE SET library_id = excluded.library_id, name = excluded.name, sort_name = excluded.sort_name,
		album_artist = excluded.album_artist, album_artist_id = excluded.album_artist_id, year = excluded.year,
		genre = excluded.genre, compilation = excluded.compilation, song_count = excluded.song_count,
		disc_count = excluded.disc_count, duration = excluded.duration, size = excluded.size,
		cover_track_id = excluded.cover_track_id, mbz_album_id = excluded.mbz_album_id,
		search_text = excluded.search_text, created_at = excluded.created_at, updated_at = excluded.updated_at`

// RefreshAlbums recomputes the given albums (nil = all) from their non-missing tracks:
// name/artist = most common values, year = max, genre = most common, counts, duration,
// size, created_at = min(tracks.created_at), cover_track_id = first track with an embedded
// picture ordered by disc/track. updated_at = max(previous, tracks.updated_at), raised to
// now when anything changed. cover_path is preserved. Albums without tracks are deleted.
func (s *Store) RefreshAlbums(ctx context.Context, ids []string) error {
	all := ids == nil
	ids = dedupe(ids)
	if !all && len(ids) == 0 {
		return nil
	}
	articles := s.ignoredArticles(ctx)
	now := util.NowMs()

	return s.db.Tx(ctx, func(tx *sqlx.Tx) error {
		aggs := map[string]*albumAgg{}
		existing := map[string]model.Album{}
		collect := func(query string, args ...any) error {
			rows, err := tx.QueryxContext(ctx, query, args...)
			if err != nil {
				return err
			}
			defer func() { _ = rows.Close() }()
			for rows.Next() {
				var r albumTrackRow
				if err := rows.StructScan(&r); err != nil {
					return err
				}
				g := aggs[r.AlbumID]
				if g == nil {
					g = &albumAgg{}
					aggs[r.AlbumID] = g
				}
				g.add(&r)
			}
			return rows.Err()
		}
		loadExisting := func(query string, args ...any) error {
			var as []model.Album
			if err := tx.SelectContext(ctx, &as, query, args...); err != nil {
				return err
			}
			for _, a := range as {
				existing[a.ID] = a
			}
			return nil
		}

		targets := ids
		if all {
			if err := collect(albumTrackRowSQL + albumOrder); err != nil {
				return fmt.Errorf("loading album tracks: %w", err)
			}
			if err := loadExisting(`SELECT ` + albumStoredCols + ` FROM albums`); err != nil {
				return err
			}
			set := map[string]bool{}
			for id := range aggs {
				set[id] = true
			}
			for id := range existing {
				set[id] = true
			}
			targets = make([]string, 0, len(set))
			for id := range set {
				targets = append(targets, id)
			}
		} else {
			for _, chunk := range chunks(ids) {
				ph, args := placeholders(len(chunk)), anys(chunk)
				if err := collect(albumTrackRowSQL+` AND album_id IN (`+ph+`)`+albumOrder, args...); err != nil {
					return fmt.Errorf("loading album tracks: %w", err)
				}
				if err := loadExisting(`SELECT `+albumStoredCols+` FROM albums WHERE id IN (`+ph+`)`, args...); err != nil {
					return err
				}
			}
		}

		upsert, err := tx.PrepareNamedContext(ctx, albumUpsertSQL)
		if err != nil {
			return err
		}
		defer func() { _ = upsert.Close() }()
		var toDelete []string
		for _, id := range targets {
			if id == "" {
				continue
			}
			g := aggs[id]
			old, exists := existing[id]
			if g == nil {
				if exists {
					toDelete = append(toDelete, id)
				}
				continue
			}
			n := g.album(id, articles)
			n.CoverPath = old.CoverPath
			updated := max(old.UpdatedAt, g.maxTrackUpdated)
			changed := !exists || albumChanged(&old, &n)
			if changed {
				updated = max(updated, now)
				if exists && updated <= old.UpdatedAt {
					updated = old.UpdatedAt + 1
				}
			}
			n.UpdatedAt = updated
			if !changed && exists && n.UpdatedAt == old.UpdatedAt {
				continue
			}
			if _, err := upsert.ExecContext(ctx, &n); err != nil {
				return fmt.Errorf("saving album %s: %w", id, err)
			}
		}
		for _, chunk := range chunks(toDelete) {
			if _, err := tx.ExecContext(ctx, `DELETE FROM albums WHERE id IN (`+placeholders(len(chunk))+`)`, anys(chunk)...); err != nil {
				return err
			}
		}
		return nil
	})
}

// ---- artists

type artistTrackRow struct {
	ID               string `db:"id"`
	AlbumID          string `db:"album_id"`
	ArtistID         string `db:"artist_id"`
	Artist           string `db:"artist"`
	SortArtist       string `db:"sort_artist"`
	MbzArtistID      string `db:"mbz_artist_id"`
	AlbumArtistID    string `db:"album_artist_id"`
	AlbumArtist      string `db:"album_artist"`
	SortAlbumArtist  string `db:"sort_album_artist"`
	MbzAlbumArtistID string `db:"mbz_album_artist_id"`
	CreatedAt        int64  `db:"created_at"`
	UpdatedAt        int64  `db:"updated_at"`
}

const artistTrackRowSQL = `SELECT id, album_id, artist_id, artist, sort_artist, mbz_artist_id, album_artist_id,
	album_artist, sort_album_artist, mbz_album_artist_id, created_at, updated_at FROM tracks WHERE missing = 0`

type artistAgg struct {
	names, sorts, mbz tally
	albums            map[string]bool
	songs             map[string]bool
	createdAt         int64
	maxTrackUpdated   int64
}

func (g *artistAgg) add(trackID, name, sortName, mbz string, albumID string, isAlbumArtist bool, created, updated int64) {
	if g.songs == nil {
		g.songs = map[string]bool{}
		g.albums = map[string]bool{}
		g.createdAt = created
	}
	g.names.add(name)
	g.sorts.add(sortName)
	g.mbz.add(mbz)
	g.songs[trackID] = true
	if isAlbumArtist {
		g.albums[albumID] = true
	}
	g.createdAt = min(g.createdAt, created)
	g.maxTrackUpdated = max(g.maxTrackUpdated, updated)
}

const artistStoredCols = `id, name, sort_name, index_key, album_count, song_count, mbz_artist_id, image_path,
	search_text, created_at, updated_at`

const artistUpsertSQL = `INSERT INTO artists (` + artistStoredCols + `)
	VALUES (:id, :name, :sort_name, :index_key, :album_count, :song_count, :mbz_artist_id, :image_path,
		:search_text, :created_at, :updated_at)
	ON CONFLICT (id) DO UPDATE SET name = excluded.name, sort_name = excluded.sort_name, index_key = excluded.index_key,
		album_count = excluded.album_count, song_count = excluded.song_count, mbz_artist_id = excluded.mbz_artist_id,
		search_text = excluded.search_text, created_at = excluded.created_at, updated_at = excluded.updated_at`

// RefreshArtists recomputes the given artists (nil = all) from non-missing tracks where
// they are the artist or the album artist: album_count = distinct albums as album artist,
// song_count = distinct tracks in either role, name = most common spelling, sort name from
// sort tags or the name, index key, created_at = min. image_path is preserved. Artists
// without tracks are deleted.
func (s *Store) RefreshArtists(ctx context.Context, ids []string) error {
	all := ids == nil
	ids = dedupe(ids)
	if !all && len(ids) == 0 {
		return nil
	}
	articles := s.ignoredArticles(ctx)
	now := util.NowMs()

	return s.db.Tx(ctx, func(tx *sqlx.Tx) error {
		aggs := map[string]*artistAgg{}
		existing := map[string]model.Artist{}
		collect := func(accept func(id string) bool, query string, args ...any) error {
			rows, err := tx.QueryxContext(ctx, query, args...)
			if err != nil {
				return err
			}
			defer func() { _ = rows.Close() }()
			for rows.Next() {
				var r artistTrackRow
				if err := rows.StructScan(&r); err != nil {
					return err
				}
				get := func(id string) *artistAgg {
					g := aggs[id]
					if g == nil {
						g = &artistAgg{}
						aggs[id] = g
					}
					return g
				}
				if r.ArtistID != "" && accept(r.ArtistID) {
					get(r.ArtistID).add(r.ID, r.Artist, r.SortArtist, r.MbzArtistID, r.AlbumID, r.ArtistID == r.AlbumArtistID, r.CreatedAt, r.UpdatedAt)
				}
				if r.AlbumArtistID != "" && r.AlbumArtistID != r.ArtistID && accept(r.AlbumArtistID) {
					get(r.AlbumArtistID).add(r.ID, r.AlbumArtist, r.SortAlbumArtist, r.MbzAlbumArtistID, r.AlbumID, true, r.CreatedAt, r.UpdatedAt)
				}
			}
			return rows.Err()
		}
		loadExisting := func(query string, args ...any) error {
			var as []model.Artist
			if err := tx.SelectContext(ctx, &as, query, args...); err != nil {
				return err
			}
			for _, a := range as {
				existing[a.ID] = a
			}
			return nil
		}

		targets := ids
		if all {
			if err := collect(func(string) bool { return true }, artistTrackRowSQL); err != nil {
				return fmt.Errorf("loading artist tracks: %w", err)
			}
			if err := loadExisting(`SELECT ` + artistStoredCols + ` FROM artists`); err != nil {
				return err
			}
			set := map[string]bool{}
			for id := range aggs {
				set[id] = true
			}
			for id := range existing {
				set[id] = true
			}
			targets = make([]string, 0, len(set))
			for id := range set {
				targets = append(targets, id)
			}
		} else {
			for _, chunk := range chunks(ids) {
				inChunk := make(map[string]bool, len(chunk))
				for _, id := range chunk {
					inChunk[id] = true
				}
				ph, args := placeholders(len(chunk)), anys(chunk)
				q := artistTrackRowSQL + ` AND (artist_id IN (` + ph + `) OR album_artist_id IN (` + ph + `))`
				if err := collect(func(id string) bool { return inChunk[id] }, q, append(args, args...)...); err != nil {
					return fmt.Errorf("loading artist tracks: %w", err)
				}
				if err := loadExisting(`SELECT `+artistStoredCols+` FROM artists WHERE id IN (`+ph+`)`, args...); err != nil {
					return err
				}
			}
		}

		upsert, err := tx.PrepareNamedContext(ctx, artistUpsertSQL)
		if err != nil {
			return err
		}
		defer func() { _ = upsert.Close() }()
		var toDelete []string
		for _, id := range targets {
			if id == "" {
				continue
			}
			g := aggs[id]
			old, exists := existing[id]
			if g == nil {
				if exists {
					toDelete = append(toDelete, id)
				}
				continue
			}
			n := model.Artist{ID: id, ImagePath: old.ImagePath, MbzArtistID: g.mbz.top(),
				AlbumCount: len(g.albums), SongCount: len(g.songs), CreatedAt: g.createdAt}
			n.Name = g.names.top()
			if st := g.sorts.top(); st != "" {
				n.SortName = util.NormalizeSearch(st)
				n.IndexKey = util.IndexKey(st, articles)
			} else {
				n.SortName = util.SortName(n.Name, articles)
				n.IndexKey = util.IndexKey(n.Name, articles)
			}
			n.SearchText = util.NormalizeSearch(n.Name)
			changed := !exists || old.Name != n.Name || old.SortName != n.SortName || old.IndexKey != n.IndexKey ||
				old.AlbumCount != n.AlbumCount || old.SongCount != n.SongCount || old.MbzArtistID != n.MbzArtistID ||
				old.SearchText != n.SearchText || old.CreatedAt != n.CreatedAt
			updated := max(old.UpdatedAt, g.maxTrackUpdated)
			if changed {
				updated = max(updated, now)
				if exists && updated <= old.UpdatedAt {
					updated = old.UpdatedAt + 1
				}
			}
			n.UpdatedAt = updated
			if !changed && exists && n.UpdatedAt == old.UpdatedAt {
				continue
			}
			if _, err := upsert.ExecContext(ctx, &n); err != nil {
				return fmt.Errorf("saving artist %s: %w", id, err)
			}
		}
		for _, chunk := range chunks(toDelete) {
			if _, err := tx.ExecContext(ctx, `DELETE FROM artists WHERE id IN (`+placeholders(len(chunk))+`)`, anys(chunk)...); err != nil {
				return err
			}
		}
		return nil
	})
}

// SetAlbumCoverPath sets an album's folder image (absolute path, "" for none) and bumps
// updated_at (and the album artist's) when it changed.
func (s *Store) SetAlbumCoverPath(ctx context.Context, albumID, coverPath string) error {
	now := util.NowMs()
	return s.db.Tx(ctx, func(tx *sqlx.Tx) error {
		var cur struct {
			CoverPath     string `db:"cover_path"`
			AlbumArtistID string `db:"album_artist_id"`
		}
		if err := tx.GetContext(ctx, &cur, `SELECT cover_path, album_artist_id FROM albums WHERE id = ?`, albumID); err != nil {
			return notFound(err)
		}
		if cur.CoverPath == coverPath {
			return nil
		}
		if _, err := tx.ExecContext(ctx, `UPDATE albums SET cover_path = ?, updated_at = `+bumpVersion+` WHERE id = ?`,
			coverPath, now, now, albumID); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, `UPDATE artists SET updated_at = `+bumpVersion+` WHERE id = ?`, now, now, cur.AlbumArtistID)
		return err
	})
}

// SetArtistImagePath sets an artist's image (absolute path, "" for none) and bumps
// updated_at when it changed.
func (s *Store) SetArtistImagePath(ctx context.Context, artistID, path string) error {
	now := util.NowMs()
	return s.db.Tx(ctx, func(tx *sqlx.Tx) error {
		var cur string
		if err := tx.GetContext(ctx, &cur, `SELECT image_path FROM artists WHERE id = ?`, artistID); err != nil {
			return notFound(err)
		}
		if cur == path {
			return nil
		}
		_, err := tx.ExecContext(ctx, `UPDATE artists SET image_path = ?, updated_at = `+bumpVersion+` WHERE id = ?`, path, now, now, artistID)
		return err
	})
}

// TouchAlbums bumps updated_at of the given albums (cover cache busting); the new value is
// always strictly greater than the old one.
func (s *Store) TouchAlbums(ctx context.Context, ids []string) error {
	ids = dedupe(ids)
	if len(ids) == 0 {
		return nil
	}
	now := util.NowMs()
	return s.db.Tx(ctx, func(tx *sqlx.Tx) error {
		for _, chunk := range chunks(ids) {
			args := append([]any{now, now}, anys(chunk)...)
			if _, err := tx.ExecContext(ctx, `UPDATE albums SET updated_at = `+bumpVersion+` WHERE id IN (`+placeholders(len(chunk))+`)`, args...); err != nil {
				return err
			}
		}
		return nil
	})
}
