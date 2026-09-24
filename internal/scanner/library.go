package scanner

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"math"
	"path"
	"sort"
	"strings"

	"golang.org/x/sync/errgroup"

	"rainy/internal/model"
	"rainy/internal/store"
	"rainy/internal/tags"
	"rainy/internal/util"
)

// trackState is the scanner's view of a stored track.
type trackState struct {
	ID            string  `db:"id"`
	Path          string  `db:"path"`
	Size          int64   `db:"size"`
	Mtime         int64   `db:"mtime"`
	Missing       bool    `db:"missing"`
	HasLrc        bool    `db:"has_lrc"`
	Title         string  `db:"title"`
	Duration      float64 `db:"duration"`
	AlbumID       string  `db:"album_id"`
	ArtistID      string  `db:"artist_id"`
	AlbumArtistID string  `db:"album_artist_id"`
}

const trackStateCols = `id, path, size, mtime, missing, has_lrc, title, duration, album_id, artist_id, album_artist_id`

// loadStates returns the stored tracks of a library (optionally only those under dir),
// keyed by path.
func (s *Scanner) loadStates(ctx context.Context, libraryID int64, dir string) (map[string]*trackState, error) {
	q := `SELECT ` + trackStateCols + ` FROM tracks WHERE library_id = ?`
	args := []any{libraryID}
	if dir = strings.Trim(dir, "/"); dir != "" {
		q += ` AND path LIKE ? ESCAPE '\'`
		args = append(args, util.EscapeLike(dir)+"/%")
	}
	var rows []*trackState
	if err := s.st.DB().R.SelectContext(ctx, &rows, q, args...); err != nil {
		return nil, fmt.Errorf("loading track states: %w", err)
	}
	out := make(map[string]*trackState, len(rows))
	for _, r := range rows {
		// LIKE is case-insensitive for ASCII: "Rock/%" also matches "rock/…", a different
		// folder on case-sensitive file systems whose tracks must not be marked missing.
		if dir != "" && !strings.HasPrefix(r.Path, dir+"/") {
			continue
		}
		out[r.Path] = r
	}
	return out, nil
}

// scanResult counts what a library scan changed and which aggregates it touched.
type scanResult struct {
	added, updated, removed, moved, errors int
	albums, artists                        map[string]bool // old and new ids of changed tracks
}

func (r *scanResult) changed() bool {
	return r.added+r.updated+r.removed+r.moved > 0
}

func (r *scanResult) touch(albumID string, artistIDs ...string) {
	if albumID != "" {
		r.albums[albumID] = true
	}
	for _, id := range artistIDs {
		if id != "" {
			r.artists[id] = true
		}
	}
}

func (r *scanResult) touchState(st *trackState) {
	r.touch(st.AlbumID, st.ArtistID, st.AlbumArtistID)
}

func (r *scanResult) touchTrack(t *model.Track) {
	r.touch(t.AlbumID, t.ArtistID, t.AlbumArtistID)
}

// libScan scans one library (or a subtree of it).
type libScan struct {
	s        *Scanner
	lib      model.Library
	full     bool
	set      model.Settings
	cache    *dirCache
	progress bool // report progress in the scanner status
	res      scanResult
}

// readJob / readResult flow through the tag-reading worker pool.
type readJob struct {
	f     fileInfo
	state *trackState // nil for new files
}

type readResult struct {
	track     model.Track
	state     *trackState
	err       error
	path      string
	unchanged bool // full scan: identical to the stored row
}

// scan synchronises the tracks under sub ("" = whole library) with the file system.
func (ls *libScan) scan(ctx context.Context, sub string) (*scanResult, error) {
	s := ls.s
	ls.res = scanResult{albums: map[string]bool{}, artists: map[string]bool{}}
	res := &ls.res
	sub = strings.Trim(sub, "/")
	if err := checkRoot(ls.lib.Path); err != nil {
		return nil, err
	}
	stored, err := s.loadStates(ctx, ls.lib.ID, sub)
	if err != nil {
		return nil, err
	}

	// 1. Walk.
	w := &walker{root: ls.lib.Path, cache: ls.cache}
	if ls.progress {
		w.onDir = func(n int) {
			if n > 0 {
				s.update(false, func(st *Status) { st.FilesSeen += n })
			}
		}
	}
	if err := w.walk(ctx, sub); err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if sub == "" || !errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("%w: reading %s: %v", ErrLibraryUnavailable, ls.lib.Path, err)
		}
		// The sub-directory is gone: its tracks vanished.
	}
	walked := &w.res
	if sub == "" && len(walked.files) == 0 {
		live := 0
		for _, st := range stored {
			if !st.Missing {
				live++
			}
		}
		if live > 0 {
			return nil, fmt.Errorf("%w: %s contains no audio files but the library has %d tracks; refusing to mark them missing (is the share mounted?)",
				ErrLibraryUnavailable, ls.lib.Path, live)
		}
	}

	// 2. Classify.
	guardEmptyDirs(walked, stored)
	seen := make(map[string]bool, len(walked.files))
	var jobs []readJob
	var lrcChanged []*trackState
	for _, f := range walked.files {
		seen[f.rel] = true
		st := stored[f.rel]
		switch {
		case st == nil, ls.full, st.Missing, st.Size != f.size, st.Mtime != f.mtime:
			jobs = append(jobs, readJob{f: f, state: st})
		case st.HasLrc != f.hasLrc:
			st.HasLrc = f.hasLrc
			lrcChanged = append(lrcChanged, st)
		}
	}
	// Tracks whose file is gone (moved-file candidates) and those that must be marked missing.
	candidates := map[int64][]*trackState{}
	var vanished []*trackState
	for p, st := range stored {
		if seen[p] || walked.underFailed(p) {
			continue
		}
		candidates[st.Size] = append(candidates[st.Size], st)
		if !st.Missing {
			vanished = append(vanished, st)
		}
	}
	if ls.progress {
		s.update(true, func(st *Status) { st.Phase = PhaseReading })
	}

	// 3. Read tags (worker pool) and write (single writer, batches of batchSize).
	used := map[string]bool{} // candidate ids reused by moves
	if err := ls.readAndWrite(ctx, jobs, candidates, used); err != nil {
		return nil, err
	}

	// 4. Sidecar .lrc changes (no tag re-read needed).
	if err := ls.updateLrc(ctx, lrcChanged); err != nil {
		return nil, err
	}

	// 5. Mark vanished tracks missing.
	var missing []string
	for _, st := range vanished {
		if !used[st.ID] {
			missing = append(missing, st.ID)
			res.touchState(st)
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := s.st.MarkTracksMissing(ctx, missing, true); err != nil {
		return nil, fmt.Errorf("marking tracks missing: %w", err)
	}
	res.removed = len(missing)
	if ls.progress && len(missing) > 0 {
		s.update(false, func(st *Status) { st.Removed += len(missing) })
	}
	return res, nil
}

// guardEmptyDirs treats a directory that is now completely empty but held at least
// offlineDirMinTracks live tracks like an unreadable one (its tracks are left alone): an
// unmounted docker bind mount or NAS share inside the library shows up as an empty
// directory, and marking a whole share missing would needlessly churn albums and covers.
// Deleting the directory itself (or leaving any entry in it) marks the tracks missing.
func guardEmptyDirs(walked *walkResult, stored map[string]*trackState) {
	if len(walked.empty) == 0 {
		return
	}
	empty := make(map[string]bool, len(walked.empty))
	for _, d := range walked.empty {
		empty[d] = true
	}
	count := map[string]int{}
	for p, st := range stored {
		if st.Missing {
			continue
		}
		for d := path.Dir(p); d != "." && d != "/" && d != ""; d = path.Dir(d) {
			if empty[d] {
				count[d]++
				break
			}
		}
	}
	for _, d := range walked.empty {
		if n := count[d]; n >= offlineDirMinTracks {
			slog.Warn("scanner: directory is empty but held tracks; assuming an offline mount and leaving them unchanged (delete the directory to remove them)",
				"dir", d, "tracks", n)
			walked.failed = append(walked.failed, d)
		}
	}
}

// readAndWrite reads the tags of jobs with a worker pool and upserts the tracks in
// batches. New files that could be moved tracks (same size as a vanished track) are held
// back until every file was read, then matched against the candidates.
func (ls *libScan) readAndWrite(ctx context.Context, jobs []readJob, candidates map[int64][]*trackState, used map[string]bool) error {
	if len(jobs) == 0 {
		return nil
	}
	s := ls.s
	g, gctx := errgroup.WithContext(ctx)
	in := make(chan readJob)
	out := make(chan readResult, 64)

	g.Go(func() error {
		defer close(in)
		for _, j := range jobs {
			select {
			case in <- j:
			case <-gctx.Done():
				return gctx.Err()
			}
		}
		return nil
	})
	workers := min(s.workers, len(jobs))
	var wg errgroup.Group
	for range workers {
		wg.Go(func() error {
			for j := range in {
				r := ls.read(gctx, j)
				select {
				case out <- r:
				case <-gctx.Done():
					return gctx.Err()
				}
			}
			return nil
		})
	}
	g.Go(func() error {
		err := wg.Wait()
		close(out)
		return err
	})

	var batch []model.Track
	var pending []readResult // possible moves
	flush := func(ctx context.Context) error {
		if len(batch) == 0 {
			return nil
		}
		err := ls.upsert(ctx, batch)
		batch = batch[:0]
		return err
	}
	g.Go(func() error {
		for r := range out {
			if r.err != nil {
				ls.res.errors++
				if ls.progress {
					s.update(false, func(st *Status) { st.Errors++ })
				}
				slog.Warn("scanner: cannot read tags", "path", r.path, "err", r.err)
				continue
			}
			if r.unchanged {
				continue
			}
			// A new file, or one at the path of an already-missing row (which a live track
			// moved onto that path must not revive), may be a moved track.
			if (r.state == nil || r.state.Missing) && len(candidates[r.track.Size]) > 0 {
				pending = append(pending, r)
				continue
			}
			ls.count(r, false)
			batch = append(batch, r.track)
			if len(batch) >= batchSize {
				if err := flush(gctx); err != nil {
					return err
				}
			}
		}
		return flush(gctx)
	})
	if err := g.Wait(); err != nil {
		return err
	}

	// Moved-file detection: a new file matching a vanished track by size, duration (±1 s)
	// and title takes over that track's id (and with it annotations, playlists, created_at).
	// Files keeping their name (folder renames) are matched first, so that duplicate copies
	// of a song keep their own ids; the order is deterministic.
	sort.Slice(pending, func(i, j int) bool { return pending[i].path < pending[j].path })
	matched := make([]*trackState, len(pending))
	for _, sameName := range []bool{true, false} {
		for i := range pending {
			if matched[i] == nil {
				// A file at a missing row's path only takes over a track that vanished just now.
				liveOnly := pending[i].state != nil
				if c := pickMove(candidates[pending[i].track.Size], used, &pending[i].track, sameName, liveOnly); c != nil {
					used[c.ID] = true
					matched[i] = c
				}
			}
		}
	}
	// Stale missing rows whose path a moved track takes over are purged first (the path is
	// unique; such a row could never come back anyway).
	var stale []string
	for i, r := range pending {
		if matched[i] != nil && r.state != nil {
			stale = append(stale, r.state.ID)
			ls.res.touchState(r.state)
		}
	}
	if len(stale) > 0 {
		if err := flush(ctx); err != nil {
			return err
		}
		if err := s.st.DeleteTracks(ctx, stale); err != nil {
			return fmt.Errorf("purging stale rows of moved paths: %w", err)
		}
		slog.Info("scanner: moved tracks replaced missing rows at their new paths", "count", len(stale))
	}
	for i, r := range pending {
		c := matched[i]
		if c != nil {
			r.track.ID = c.ID
			r.state = c
			ls.res.touchState(c)
		}
		ls.count(r, c != nil)
		batch = append(batch, r.track)
		if len(batch) >= batchSize {
			if err := flush(ctx); err != nil {
				return err
			}
		}
	}
	return flush(ctx)
}

// pickMove returns the vanished track a new file t most likely is (docs/architecture/contract.md §5.8: same
// size, duration ±1 s and title), or nil. With sameName only candidates that had the same
// file name qualify. Among several matches a track that vanished just now beats one that
// was already missing (an old copy must not steal the live track's id), then the one in
// the same directory wins, then the smallest path (deterministic).
// With liveOnly, tracks that were already missing do not qualify.
func pickMove(cands []*trackState, used map[string]bool, t *model.Track, sameName, liveOnly bool) *trackState {
	dir, name, _ := util.PathParts(t.Path)
	var best *trackState
	rank := func(c *trackState, sameDir bool) int {
		r := 0
		if !c.Missing {
			r += 2
		}
		if sameDir {
			r++
		}
		return r
	}
	bestRank := -1
	for _, c := range cands {
		if used[c.ID] || (liveOnly && c.Missing) || c.Size != t.Size || math.Abs(c.Duration-t.Duration) > 1 ||
			!strings.EqualFold(strings.TrimSpace(c.Title), strings.TrimSpace(t.Title)) {
			continue
		}
		cdir, cname, _ := util.PathParts(c.Path)
		if sameName && cname != name {
			continue
		}
		r := rank(c, cdir == dir)
		if best == nil || r > bestRank || (r == bestRank && c.Path < best.Path) {
			best, bestRank = c, r
		}
	}
	return best
}

// count records an added / updated / moved track.
func (ls *libScan) count(r readResult, moved bool) {
	s := ls.s
	ls.res.touchTrack(&r.track)
	switch {
	case moved:
		ls.res.moved++
	case r.state == nil:
		ls.res.added++
	default:
		ls.res.updated++
		ls.res.touchState(r.state)
	}
	if ls.progress {
		s.update(false, func(st *Status) {
			switch {
			case moved:
				st.Moved++
			case r.state == nil:
				st.Added++
			default:
				st.Updated++
			}
		})
	}
}

// read reads one file and builds its track.
func (ls *libScan) read(ctx context.Context, j readJob) readResult {
	meta, err := tags.Read(j.f.abs, tags.ReadOptions{FixEncoding: ls.set.FixEncodingOnScan})
	if err != nil {
		return readResult{err: err, path: j.f.rel, state: j.state}
	}
	t := buildTrack(ls.lib.ID, j.f, meta, ls.set)
	r := readResult{track: t, state: j.state, path: j.f.rel}
	if j.state != nil {
		r.track.ID = j.state.ID
		if ls.full && !j.state.Missing {
			var old model.Track
			err := ls.s.st.DB().R.GetContext(ctx, &old, `SELECT * FROM tracks WHERE id = ?`, j.state.ID)
			r.unchanged = err == nil && sameContent(&old, &r.track)
		}
	}
	return r
}

// upsert writes a batch; on a path conflict (another row already owns a path — only
// possible when the database changed behind the scanner's back) it retries track by track,
// adopting the id of the row that owns the path.
func (ls *libScan) upsert(ctx context.Context, batch []model.Track) error {
	return upsertResilient(ctx, ls.s.st, ls.lib.ID, batch, func(t *model.Track, err error) {
		ls.res.errors++
		slog.Warn("scanner: cannot save track", "path", t.Path, "err", err)
	})
}

func upsertResilient(ctx context.Context, st *store.Store, libraryID int64, batch []model.Track, onErr func(*model.Track, error)) error {
	tracks := append([]model.Track(nil), batch...)
	err := st.UpsertTracks(ctx, tracks)
	if err == nil || !errors.Is(err, store.ErrConflict) {
		return err
	}
	for i := range batch {
		t := batch[i]
		err := st.UpsertTracks(ctx, []model.Track{t})
		if errors.Is(err, store.ErrConflict) {
			if owner, gerr := st.GetTrackByPath(ctx, libraryID, t.Path); gerr == nil && owner.ID != t.ID {
				t.ID = owner.ID
				err = st.UpsertTracks(ctx, []model.Track{t})
			}
		}
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			onErr(&t, err)
		}
	}
	return nil
}

// updateLrc stores changed sidecar-lyrics flags.
func (ls *libScan) updateLrc(ctx context.Context, states []*trackState) error {
	if len(states) == 0 {
		return nil
	}
	now := util.NowMs()
	tx, err := ls.s.st.DB().W.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	for _, st := range states {
		if _, err := tx.ExecContext(ctx, `UPDATE tracks SET has_lrc = ?, updated_at = ? WHERE id = ?`, st.HasLrc, now, st.ID); err != nil {
			return fmt.Errorf("updating lyrics flag: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	ls.res.updated += len(states)
	for _, st := range states {
		ls.res.touchState(st)
	}
	if ls.progress {
		ls.s.update(false, func(st *Status) { st.Updated += len(states) })
	}
	return nil
}
