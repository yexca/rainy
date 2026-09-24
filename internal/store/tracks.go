package store

import (
	"context"
	"fmt"
	"strings"

	"github.com/jmoiron/sqlx"

	"rainy/internal/model"
	"rainy/internal/util"
)

// trackColumns are the tracks table columns in schema order (lyrics last-but-not-selected in
// list queries is handled separately).
var trackColumns = []string{
	"id", "library_id", "path", "dir", "filename", "suffix", "size", "mtime",
	"title", "album", "artist", "album_artist", "album_id", "artist_id", "album_artist_id",
	"track_number", "track_total", "disc_number", "disc_total", "disc_subtitle",
	"year", "date", "original_year", "genre", "composer", "comment", "lyrics", "has_lrc", "bpm", "compilation",
	"duration", "bitrate", "sample_rate", "bit_depth", "channels", "codec", "has_cover",
	"rg_track_gain", "rg_track_peak", "rg_album_gain", "rg_album_peak",
	"mbz_track_id", "mbz_album_id", "mbz_artist_id", "mbz_album_artist_id",
	"sort_title", "sort_album", "sort_artist", "sort_album_artist",
	"search_text", "missing", "created_at", "updated_at",
}

var (
	// trackListCols selects every column except the (potentially large) lyrics text.
	trackListCols = func() string {
		cols := make([]string, 0, len(trackColumns))
		for _, c := range trackColumns {
			if c != "lyrics" {
				cols = append(cols, "t."+c)
			}
		}
		return strings.Join(cols, ", ")
	}()

	trackUpsertSQL = func() string {
		named := make([]string, len(trackColumns))
		var sets []string
		for i, c := range trackColumns {
			named[i] = ":" + c
			if c != "id" && c != "created_at" {
				sets = append(sets, c+" = excluded."+c)
			}
		}
		return `INSERT INTO tracks (` + strings.Join(trackColumns, ", ") + `) VALUES (` + strings.Join(named, ", ") +
			`) ON CONFLICT (id) DO UPDATE SET ` + strings.Join(sets, ", ")
	}()
)

// trackSelect builds "SELECT <cols> FROM <from> LEFT JOIN albums … LEFT JOIN annotations …".
// from must introduce the tracks table as alias t and contain no bind parameters; the
// returned SQL has exactly one bind parameter (the user id) before any WHERE arguments.
func trackSelect(from string, withLyrics bool) string {
	cols := trackListCols
	if withLyrics {
		cols += ", t.lyrics"
	}
	return `SELECT ` + cols + `, (t.lyrics != '') AS has_embedded_lyrics,
		COALESCE(al.updated_at, t.updated_at) AS album_updated_at, ` + annotationCols + `
		FROM ` + from + `
		LEFT JOIN albums al ON al.id = t.album_id` + annotationJoin("track", "t.id")
}

// TrackQuery filters ListTracks. Zero values mean "no filter".
type TrackQuery struct {
	UserID    string   // per-user annotations (and Starred filter) for this user
	Q         string   // search tokens, all must match search_text
	IDs       []string // restrict to these ids (non-nil empty slice matches nothing)
	AlbumID   string
	ArtistID  string // artist_id OR album_artist_id
	Genre     string // genre name (case-insensitive)
	LibraryID int64
	Dir       string // exact library-relative directory
	DirPrefix string // directory and all its sub-directories
	FromYear  int
	ToYear    int
	Starred   bool   // only tracks starred by UserID
	Played    bool   // only tracks played by UserID (play_count > 0)
	Missing   string // "" exclude missing | "include" | "only"
	Sort      string // see trackSorts; unknown → "title"
	Order     string // "asc" | "desc" | "" (natural for the sort)
	Offset    int
	Limit     int // 0 = unlimited
}

var trackSorts = map[string]sortSpec{
	"title":       {expr: "COALESCE(NULLIF(t.sort_title, ''), t.title) COLLATE NOCASE"},
	"artist":      {expr: "COALESCE(NULLIF(t.sort_artist, ''), t.artist) COLLATE NOCASE", then: []string{"COALESCE(NULLIF(t.sort_album, ''), t.album) COLLATE NOCASE", "t.album_id", "t.disc_number", "t.track_number"}},
	"album":       {expr: "COALESCE(NULLIF(t.sort_album, ''), t.album) COLLATE NOCASE", then: []string{"t.album_id", "t.disc_number", "t.track_number", "t.path"}},
	"albumArtist": {expr: "COALESCE(NULLIF(t.sort_album_artist, ''), t.album_artist) COLLATE NOCASE", then: []string{"COALESCE(NULLIF(t.sort_album, ''), t.album) COLLATE NOCASE", "t.album_id", "t.disc_number", "t.track_number"}},
	"year":        {expr: "t.year", then: []string{"COALESCE(NULLIF(t.sort_album, ''), t.album) COLLATE NOCASE", "t.album_id", "t.disc_number", "t.track_number"}},
	"duration":    {expr: "t.duration"},
	"recent":      {expr: "t.created_at", desc: true},
	"updated":     {expr: "t.updated_at", desc: true},
	"played":      {expr: "COALESCE(an.played_at, 0)", desc: true},
	"frequent":    {expr: "COALESCE(an.play_count, 0)", desc: true},
	"rating":      {expr: "COALESCE(an.rating, 0)", desc: true},
	"starred":     {expr: "an.starred_at", desc: true, nullsLast: true},
	"random":      {expr: "RANDOM()"},
	"path":        {expr: "t.path"},
	"track":       {expr: "t.disc_number", then: []string{"t.track_number", "t.path"}},
	"size":        {expr: "t.size"},
	"bitrate":     {expr: "t.bitrate"},
	"suffix":      {expr: "t.suffix", then: []string{"t.path"}},
}

func (q *TrackQuery) where() *where {
	w := &where{}
	w.tokens("t.search_text", q.Q)
	if q.IDs != nil {
		w.in("t.id", q.IDs)
	}
	if q.AlbumID != "" {
		w.add("t.album_id = ?", q.AlbumID)
	}
	if q.ArtistID != "" {
		w.add("(t.artist_id = ? OR t.album_artist_id = ?)", q.ArtistID, q.ArtistID)
	}
	if q.Genre != "" {
		w.add(`t.id IN (SELECT tg.track_id FROM track_genres tg JOIN genres g ON g.id = tg.genre_id WHERE g.name = ?)`, q.Genre)
	}
	if q.LibraryID > 0 {
		w.add("t.library_id = ?", q.LibraryID)
	}
	if q.Dir != "" {
		w.add("t.dir = ?", strings.Trim(q.Dir, "/"))
	}
	if p := strings.Trim(q.DirPrefix, "/"); p != "" {
		w.add(`(t.dir = ? OR t.dir LIKE ? ESCAPE '\')`, p, util.EscapeLike(p)+"/%")
	}
	from, to := q.FromYear, q.ToYear
	if from > 0 && to > 0 && from > to {
		from, to = to, from
	}
	if from > 0 {
		w.add("t.year >= ?", from)
	}
	if to > 0 {
		w.add("t.year <= ?", to)
	}
	if q.Starred {
		w.add("an.starred_at IS NOT NULL")
	}
	if q.Played {
		w.add("COALESCE(an.play_count, 0) > 0")
	}
	switch q.Missing {
	case "include":
	case "only":
		w.add("t.missing = 1")
	default:
		w.add("t.missing = 0")
	}
	return w
}

// ListTracks returns a page of tracks and the total number of matches (ignoring
// offset/limit).
func (s *Store) ListTracks(ctx context.Context, q TrackQuery) ([]model.Track, int, error) {
	w := q.where()
	args := append([]any{q.UserID}, w.args...)

	var total int
	countSQL := `SELECT COUNT(*) FROM tracks t` + annotationJoin("track", "t.id") + w.sql()
	if err := s.db.R.GetContext(ctx, &total, countSQL, args...); err != nil {
		return nil, 0, fmt.Errorf("counting tracks: %w", err)
	}
	if total == 0 || (q.Limit > 0 && q.Offset >= total) {
		return []model.Track{}, total, nil
	}
	spec, ok := trackSorts[q.Sort]
	if !ok {
		spec = trackSorts["title"]
	}
	query := trackSelect("tracks t", false) + w.sql() + orderBy(spec, q.Order, "t.id") + limitClause(q.Offset, q.Limit)
	var tracks []model.Track
	if err := s.db.R.SelectContext(ctx, &tracks, query, args...); err != nil {
		return nil, 0, fmt.Errorf("listing tracks: %w", err)
	}
	return fillTracks(tracks), total, nil
}

// GetTrack returns one track including its embedded lyrics (missing tracks included).
func (s *Store) GetTrack(ctx context.Context, id, userID string) (*model.Track, error) {
	var t model.Track
	if err := s.db.R.GetContext(ctx, &t, trackSelect("tracks t", true)+` WHERE t.id = ?`, userID, id); err != nil {
		return nil, notFound(err)
	}
	fillTrack(&t)
	return &t, nil
}

// GetTracks returns the tracks for ids in the order given (duplicates repeated, unknown ids
// skipped, missing tracks included, no lyrics text).
func (s *Store) GetTracks(ctx context.Context, ids []string, userID string) ([]model.Track, error) {
	byID := make(map[string]model.Track, len(ids))
	for _, chunk := range chunks(dedupe(ids)) {
		var ts []model.Track
		q := trackSelect("tracks t", false) + ` WHERE t.id IN (` + placeholders(len(chunk)) + `)`
		if err := s.db.R.SelectContext(ctx, &ts, q, append([]any{userID}, anys(chunk)...)...); err != nil {
			return nil, fmt.Errorf("getting tracks: %w", err)
		}
		for _, t := range ts {
			fillTrack(&t)
			byID[t.ID] = t
		}
	}
	out := make([]model.Track, 0, len(ids))
	for _, id := range ids {
		if t, ok := byID[id]; ok {
			out = append(out, t)
		}
	}
	return out, nil
}

// GetTrackByPath returns the track at a library-relative path (lyrics included).
func (s *Store) GetTrackByPath(ctx context.Context, libraryID int64, path string) (*model.Track, error) {
	var t model.Track
	err := s.db.R.GetContext(ctx, &t, trackSelect("tracks t", true)+` WHERE t.library_id = ? AND t.path = ?`, "", libraryID, path)
	if err != nil {
		return nil, notFound(err)
	}
	fillTrack(&t)
	return &t, nil
}

// TrackFileState is the scanner's view of a stored track.
type TrackFileState struct {
	ID      string `db:"id"`
	Size    int64  `db:"size"`
	Mtime   int64  `db:"mtime"`
	Missing bool   `db:"missing"`
}

// TrackFileStates returns every track of a library keyed by relative path.
func (s *Store) TrackFileStates(ctx context.Context, libraryID int64) (map[string]TrackFileState, error) {
	rows, err := s.db.R.QueryxContext(ctx, `SELECT id, path, size, mtime, missing FROM tracks WHERE library_id = ?`, libraryID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := map[string]TrackFileState{}
	for rows.Next() {
		var st TrackFileState
		var path string
		if err := rows.Scan(&st.ID, &path, &st.Size, &st.Mtime, &st.Missing); err != nil {
			return nil, err
		}
		out[path] = st
	}
	return out, rows.Err()
}

// normalizeTrack derives the columns the store owns so every writer stays consistent:
// dir/filename/suffix from Path, Genre/Genres, search_text and timestamps.
func normalizeTrack(t *model.Track, now int64) {
	t.Path = strings.Trim(strings.ReplaceAll(t.Path, `\`, "/"), "/")
	t.Dir, t.Filename, t.Suffix = util.PathParts(t.Path)
	if len(t.Genres) > 0 {
		t.Genres = util.SplitMulti(t.Genres, "")
	} else {
		t.Genres = util.SplitMulti([]string{t.Genre}, ";")
	}
	t.Genre = strings.Join(t.Genres, model.GenreJoiner)
	t.SearchText = util.NormalizeSearch(t.Title, t.Artist, t.Album, t.AlbumArtist, t.Composer, t.Filename)
	if t.CreatedAt == 0 {
		t.CreatedAt = now
	}
	if t.UpdatedAt == 0 {
		t.UpdatedAt = now
	}
}

// UpsertTracks inserts or updates tracks in one transaction (ON CONFLICT(id) DO UPDATE;
// an existing created_at is kept). IDs are generated when empty. Dir, Filename, Suffix,
// Genre and SearchText are derived from the other fields; track_genres and genres are
// rewritten from t.Genres (or Genre split on ";"). Aggregates are NOT refreshed.
func (s *Store) UpsertTracks(ctx context.Context, tracks []model.Track) error {
	if len(tracks) == 0 {
		return nil
	}
	now := util.NowMs()
	for i := range tracks {
		if tracks[i].ID == "" {
			tracks[i].ID = util.NewID()
		}
		normalizeTrack(&tracks[i], now)
	}
	return s.db.Tx(ctx, func(tx *sqlx.Tx) error {
		upsert, err := tx.PrepareNamedContext(ctx, trackUpsertSQL)
		if err != nil {
			return fmt.Errorf("preparing track upsert: %w", err)
		}
		defer func() { _ = upsert.Close() }()
		delGenres, err := tx.PreparexContext(ctx, `DELETE FROM track_genres WHERE track_id = ?`)
		if err != nil {
			return err
		}
		defer func() { _ = delGenres.Close() }()
		addGenre, err := tx.PreparexContext(ctx, `INSERT INTO genres (id, name) VALUES (?, ?) ON CONFLICT DO NOTHING`)
		if err != nil {
			return err
		}
		defer func() { _ = addGenre.Close() }()
		link, err := tx.PreparexContext(ctx, `INSERT OR IGNORE INTO track_genres (track_id, genre_id) VALUES (?, ?)`)
		if err != nil {
			return err
		}
		defer func() { _ = link.Close() }()

		for i := range tracks {
			t := &tracks[i]
			if _, err := upsert.ExecContext(ctx, t); err != nil {
				return fmt.Errorf("upserting track %s (%s): %w", t.ID, t.Path, constraint(err, "track path"))
			}
			if _, err := delGenres.ExecContext(ctx, t.ID); err != nil {
				return err
			}
			for _, g := range t.Genres {
				gid := util.GenreID(g)
				if _, err := addGenre.ExecContext(ctx, gid, g); err != nil {
					return fmt.Errorf("adding genre %q: %w", g, err)
				}
				if _, err := link.ExecContext(ctx, t.ID, gid); err != nil {
					return err
				}
			}
		}
		return nil
	})
}

// MarkTracksMissing sets or clears the missing flag (bumping updated_at).
func (s *Store) MarkTracksMissing(ctx context.Context, ids []string, missing bool) error {
	ids = dedupe(ids)
	if len(ids) == 0 {
		return nil
	}
	now := util.NowMs()
	return s.db.Tx(ctx, func(tx *sqlx.Tx) error {
		for _, chunk := range chunks(ids) {
			args := append([]any{missing, now}, anys(chunk)...)
			if _, err := tx.ExecContext(ctx, `UPDATE tracks SET missing = ?, updated_at = ? WHERE id IN (`+placeholders(len(chunk))+`)`, args...); err != nil {
				return err
			}
		}
		return nil
	})
}

// DeleteTracks purges track rows together with their annotations and bookmarks
// (track_genres cascade). Playlist entries are kept (filtered out by joins).
func (s *Store) DeleteTracks(ctx context.Context, ids []string) error {
	ids = dedupe(ids)
	if len(ids) == 0 {
		return nil
	}
	return s.db.Tx(ctx, func(tx *sqlx.Tx) error {
		for _, chunk := range chunks(ids) {
			ph, args := placeholders(len(chunk)), anys(chunk)
			if _, err := tx.ExecContext(ctx, `DELETE FROM annotations WHERE item_type = 'track' AND item_id IN (`+ph+`)`, args...); err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `DELETE FROM bookmarks WHERE track_id IN (`+ph+`)`, args...); err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `DELETE FROM tracks WHERE id IN (`+ph+`)`, args...); err != nil {
				return err
			}
		}
		return nil
	})
}

// UpdateTrackPath moves a track to a new library-relative path (updating dir, filename,
// suffix, search_text and updated_at). A path already used by another track yields
// ErrConflict.
func (s *Store) UpdateTrackPath(ctx context.Context, id string, path string) error {
	return s.db.Tx(ctx, func(tx *sqlx.Tx) error {
		var t model.Track
		err := tx.GetContext(ctx, &t, `SELECT title, artist, album, album_artist, composer FROM tracks WHERE id = ?`, id)
		if err != nil {
			return notFound(err)
		}
		t.Path = strings.Trim(strings.ReplaceAll(path, `\`, "/"), "/")
		t.Dir, t.Filename, t.Suffix = util.PathParts(t.Path)
		search := util.NormalizeSearch(t.Title, t.Artist, t.Album, t.AlbumArtist, t.Composer, t.Filename)
		_, err = tx.ExecContext(ctx, `UPDATE tracks SET path = ?, dir = ?, filename = ?, suffix = ?, search_text = ?, updated_at = ? WHERE id = ?`,
			t.Path, t.Dir, t.Filename, t.Suffix, search, util.NowMs(), id)
		return constraint(err, "track path")
	})
}

// AlbumIDsForTracks returns the distinct album ids of the given tracks.
func (s *Store) AlbumIDsForTracks(ctx context.Context, ids []string) ([]string, error) {
	return s.distinctForTracks(ctx, ids, `SELECT DISTINCT album_id FROM tracks WHERE id IN (%s)`)
}

// ArtistIDsForTracks returns the distinct artist and album-artist ids of the given tracks.
func (s *Store) ArtistIDsForTracks(ctx context.Context, ids []string) ([]string, error) {
	return s.distinctForTracks(ctx, ids,
		`SELECT artist_id FROM tracks WHERE id IN (%[1]s) UNION SELECT album_artist_id FROM tracks WHERE id IN (%[1]s)`)
}

func (s *Store) distinctForTracks(ctx context.Context, ids []string, query string) ([]string, error) {
	var out []string
	for _, chunk := range chunks(dedupe(ids)) {
		var got []string
		ph := placeholders(len(chunk))
		q := fmt.Sprintf(query, ph)
		args := anys(chunk)
		if strings.Count(q, "?") == 2*len(chunk) {
			args = append(args, anys(chunk)...)
		}
		if err := s.db.R.SelectContext(ctx, &got, q, args...); err != nil {
			return nil, err
		}
		out = append(out, got...)
	}
	return dedupe(out), nil
}
