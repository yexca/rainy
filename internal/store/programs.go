package store

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"rainy/internal/model"
	"rainy/internal/util"
)

// Program folders and libraries (docs/architecture/contract.md §5.4): a track of a program
// library, or whose library-relative dir is a program folder or lies below one, is a program
// episode (Track.Program), and an album whose present tracks all are is a program show
// (Album.Program). Queries ask for the condition through programCond, which inlines the
// libraries and folders like the library scope does; both lists are cached here and reloaded
// after every change of a program folder or a library.

// Values of TrackQuery.Programs, AlbumQuery.Programs and ArtistQuery.Programs.
const (
	ProgramsAll     = ""        // programs and everything else (internal callers)
	ProgramsExclude = "exclude" // leave programs out
	ProgramsOnly    = "only"    // only programs
)

// Programs is what makes tracks program episodes: whole program libraries and program
// folders of other libraries (a folder of a program library may stay marked; it changes
// nothing). Slices are never nil.
type Programs struct {
	Libraries []int64               `json:"libraries"` // ids, ascending
	Folders   []model.ProgramFolder `json:"folders"`   // by library and path
}

func (p Programs) clone() Programs {
	return Programs{Libraries: append([]int64{}, p.Libraries...), Folders: append([]model.ProgramFolder{}, p.Folders...)}
}

// programCache holds the programs. A change bumps gen and drops them; a load stores its
// result only when no change happened while it read, so a slow reader never puts back a
// list older than the last change.
type programCache struct {
	mu     sync.RWMutex
	gen    uint64
	loaded bool
	p      Programs
}

// get returns the cached programs and whether they are loaded, with the generation a load
// must pass to set.
func (c *programCache) get() (Programs, bool, uint64) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.p, c.loaded, c.gen
}

func (c *programCache) set(p Programs, gen uint64) {
	c.mu.Lock()
	if c.gen == gen {
		c.p, c.loaded = p, true
	}
	c.mu.Unlock()
}

func (c *programCache) invalidate() {
	c.mu.Lock()
	c.gen++
	c.p, c.loaded = Programs{}, false
	c.mu.Unlock()
}

// ListPrograms returns the program libraries and program folders.
func (s *Store) ListPrograms(ctx context.Context) (Programs, error) {
	p, ok, gen := s.programs.get()
	if ok {
		return p.clone(), nil
	}
	p = Programs{Libraries: []int64{}, Folders: []model.ProgramFolder{}}
	if err := s.db.R.SelectContext(ctx, &p.Libraries, `SELECT id FROM libraries WHERE program = 1 ORDER BY id`); err != nil {
		return Programs{}, fmt.Errorf("listing program libraries: %w", err)
	}
	if err := s.db.R.SelectContext(ctx, &p.Folders, `SELECT library_id, path, created_at FROM program_folders
		ORDER BY library_id, path`); err != nil {
		return Programs{}, fmt.Errorf("listing program folders: %w", err)
	}
	s.programs.set(p, gen)
	return p.clone(), nil
}

// AddProgramFolder marks a folder of a library as a program folder (already marked: no
// change). The path must be valid (model.CleanFolderPath, ErrInvalid otherwise); an
// unknown library yields ErrNotFound.
func (s *Store) AddProgramFolder(ctx context.Context, libraryID int64, path string) error {
	p, ok := model.CleanFolderPath(path)
	if !ok {
		return fmt.Errorf("%w: folder path", ErrInvalid)
	}
	defer s.programs.invalidate()
	res, err := s.db.W.ExecContext(ctx, `INSERT INTO program_folders (library_id, path, created_at)
		SELECT id, ?, ? FROM libraries WHERE id = ?
		ON CONFLICT (library_id, path) DO NOTHING`, p, util.NowMs(), libraryID)
	if err != nil {
		return fmt.Errorf("adding program folder: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		var exists bool
		if err := s.db.R.GetContext(ctx, &exists, `SELECT EXISTS (SELECT 1 FROM libraries WHERE id = ?)`, libraryID); err != nil {
			return err
		}
		if !exists {
			return ErrNotFound
		}
	}
	return nil
}

// RemoveProgramFolder unmarks a program folder (ErrNotFound when it is not marked). Folders
// below or above it keep their own marks.
func (s *Store) RemoveProgramFolder(ctx context.Context, libraryID int64, path string) error {
	p, ok := model.CleanFolderPath(path)
	if !ok {
		return fmt.Errorf("%w: folder path", ErrInvalid)
	}
	defer s.programs.invalidate()
	return requireAffected(s.db.W.ExecContext(ctx, `DELETE FROM program_folders WHERE library_id = ? AND path = ?`, libraryID, p))
}

// programCond returns the condition that the tracks columns <prefix>library_id and
// <prefix>dir lie in a program library or a program folder ("0" when there is none).
// Should the lists fail to load, the condition reads the tables itself.
func (s *Store) programCond(ctx context.Context, prefix string) string {
	p, err := s.ListPrograms(ctx)
	lib, dir := prefix+"library_id", prefix+"dir"
	if err != nil {
		slog.Warn("loading programs", "err", err)
		return `(` + lib + ` IN (SELECT id FROM libraries WHERE program = 1)
			OR EXISTS (SELECT 1 FROM program_folders pf WHERE pf.library_id = ` + lib + ` AND (` + dir + ` = pf.path
			OR (` + dir + ` >= pf.path || '/' AND ` + dir + ` < pf.path || '0'))))`
	}
	return programsCond(prefix, p)
}

// andNotProgram returns " AND NOT (programCond)", or "" when there is no program folder.
func (s *Store) andNotProgram(ctx context.Context, prefix string) string {
	c := s.programCond(ctx, prefix)
	if c == "0" {
		return ""
	}
	return " AND NOT (" + c + ")"
}

// AndNotProgram is andNotProgram for queries written outside the store (Subsonic lists).
func (s *Store) AndNotProgram(ctx context.Context, prefix string) string {
	return s.andNotProgram(ctx, prefix)
}

// albumProgram returns the condition that the album at alias.id is a program show: it has
// present tracks and all of them lie in program folders ("0" when there is no program
// folder). Every track of the album counts, whatever the library scope.
func (s *Store) albumProgram(ctx context.Context, alias string) string {
	c := s.programCond(ctx, "pt.")
	if c == "0" {
		return "0"
	}
	return `(EXISTS (SELECT 1 FROM tracks pt WHERE pt.album_id = ` + alias + `.id AND pt.missing = 0)
		AND NOT EXISTS (SELECT 1 FROM tracks pt WHERE pt.album_id = ` + alias + `.id AND pt.missing = 0 AND NOT (` + c + `)))`
}

// AndNotProgramAlbum returns " AND NOT (albumProgram)" for the albums alias, or "" when
// there is no program folder.
func (s *Store) AndNotProgramAlbum(ctx context.Context, alias string) string {
	c := s.albumProgram(ctx, alias)
	if c == "0" {
		return ""
	}
	return " AND NOT " + c
}

// ArtistMusicCounts returns the artist's album_count and song_count (the definitions
// RefreshArtists stores) over the present tracks readable by ctx that are not program
// episodes: the albums it is the album artist of with such a track, and those tracks in
// either role.
func (s *Store) ArtistMusicCounts(ctx context.Context, artistID string) (albums, songs int, err error) {
	var n struct {
		Albums int `db:"albums"`
		Songs  int `db:"songs"`
	}
	err = s.db.R.GetContext(ctx, &n, `SELECT COUNT(DISTINCT CASE WHEN t.album_artist_id = ? THEN t.album_id END) AS albums,
			COUNT(*) AS songs
		FROM `+TracksFrom(ctx)+` t
		WHERE t.missing = 0 AND (t.artist_id = ? OR t.album_artist_id = ?)`+s.andNotProgram(ctx, "t."),
		artistID, artistID, artistID)
	if err != nil {
		return 0, 0, fmt.Errorf("counting artist music: %w", err)
	}
	return n.Albums, n.Songs, nil
}

// programsFilter adds the condition for a Programs filter value; isProgram is the
// condition that a row is a program.
func programsFilter(w *where, mode, isProgram string) {
	switch mode {
	case ProgramsExclude:
		if isProgram != "0" {
			w.add("NOT (" + isProgram + ")")
		}
	case ProgramsOnly:
		w.add("(" + isProgram + ")")
	}
}

// ProgramShow is a program album with the user's progress through its episodes, the
// readable present tracks of the album.
type ProgramShow struct {
	model.Album
	Episodes      int   `db:"episodes" json:"episodes"`
	Played        int   `db:"played_episodes" json:"played"`          // played and not bookmarked
	InProgress    int   `db:"in_progress_episodes" json:"inProgress"` // bookmarked
	LatestEpisode int64 `db:"latest_episode" json:"latestEpisodeAt"`  // newest episode's created_at
	ActivityAt    int64 `db:"activity_at" json:"activityAt"`          // latest of a new episode, a play, a bookmark
}

// ListProgramShows returns the program albums readable by ctx with the user's progress,
// most recent activity first.
func (s *Store) ListProgramShows(ctx context.Context, userID string) ([]ProgramShow, error) {
	shows := []ProgramShow{}
	isProgram := s.albumProgram(ctx, "al")
	if isProgram == "0" {
		return shows, nil
	}
	err := s.db.R.SelectContext(ctx, &shows, `SELECT `+albumCols+`, 1 AS program,
			COUNT(t.id) AS episodes,
			SUM(COALESCE(tan.play_count, 0) > 0 AND b.track_id IS NULL) AS played_episodes,
			SUM(b.track_id IS NOT NULL) AS in_progress_episodes,
			MAX(t.created_at) AS latest_episode,
			MAX(MAX(t.created_at), MAX(COALESCE(tan.played_at, 0)), MAX(COALESCE(b.updated_at, 0))) AS activity_at
		FROM `+AlbumsFrom(ctx)+` al`+annotationJoin("album", "al.id")+`
		JOIN `+TracksFrom(ctx)+` t ON t.album_id = al.id AND t.missing = 0
		LEFT JOIN annotations tan ON tan.user_id = ? AND tan.item_type = 'track' AND tan.item_id = t.id
		LEFT JOIN bookmarks b ON b.user_id = ? AND b.track_id = t.id
		WHERE `+isProgram+`
		GROUP BY al.id
		ORDER BY activity_at DESC, al.sort_name, al.id`, userID, userID, userID)
	if err != nil {
		return nil, fmt.Errorf("listing program shows: %w", err)
	}
	for i := range shows {
		fillAlbum(&shows[i].Album)
	}
	return shows, nil
}

// TrackBookmarks returns the user's bookmarks of the given tracks that ctx may read, in no
// particular order.
func (s *Store) TrackBookmarks(ctx context.Context, userID string, trackIDs []string) ([]model.Bookmark, error) {
	out := []model.Bookmark{}
	for _, chunk := range chunks(dedupe(trackIDs)) {
		var got []model.Bookmark
		err := s.db.R.SelectContext(ctx, &got, `SELECT b.track_id, b.position_ms, b.comment, b.created_at, b.updated_at
			FROM bookmarks b JOIN `+TracksFrom(ctx)+` t ON t.id = b.track_id
			WHERE b.user_id = ? AND b.track_id IN (`+placeholders(len(chunk))+`)`, append([]any{userID}, anys(chunk)...)...)
		if err != nil {
			return nil, fmt.Errorf("listing bookmarks: %w", err)
		}
		out = append(out, got...)
	}
	return out, nil
}

// SetBookmarkPosition creates or moves the user's bookmark of a track, keeping the comment
// of an existing one.
func (s *Store) SetBookmarkPosition(ctx context.Context, userID, trackID string, positionMs int64) (*model.Bookmark, error) {
	now := util.NowMs()
	if _, err := s.db.W.ExecContext(ctx, `INSERT INTO bookmarks (user_id, track_id, position_ms, comment, created_at, updated_at)
		VALUES (?, ?, ?, '', ?, ?)
		ON CONFLICT (user_id, track_id) DO UPDATE SET position_ms = excluded.position_ms, updated_at = excluded.updated_at`,
		userID, trackID, positionMs, now, now); err != nil {
		return nil, fmt.Errorf("saving bookmark: %w", err)
	}
	var b model.Bookmark
	err := s.db.R.GetContext(ctx, &b, `SELECT track_id, position_ms, comment, created_at, updated_at
		FROM bookmarks WHERE user_id = ? AND track_id = ?`, userID, trackID)
	return &b, notFound(err)
}
