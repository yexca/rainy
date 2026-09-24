package scanner

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"sort"
	"strings"

	"golang.org/x/sync/errgroup"

	"rainy/internal/events"
	"rainy/internal/model"
	"rainy/internal/tags"
	"rainy/internal/util"
)

// RescanFiles re-reads the given library-relative paths and brings their rows up to date.
// It is meant to be called after file edits, while the caller still holds LockLibrary
// (RescanFiles never takes that lock itself). For each path:
//   - an existing audio file is re-read and upserted (keeping the id of the row at that
//     path; a file at a path without row takes over the id of a vanished track among the
//     given paths that matches it by size, duration and title — so renames work whether or
//     not the caller already ran store.UpdateTrackPath);
//   - a vanished file marks its row missing (rows are never deleted here);
//   - a ".lrc" / ".LRC" sidecar re-evaluates has_lrc of the audio files next to it with the
//     same base name; an image re-evaluates the covers of the albums in its directory.
//
// Affected albums/artists (old and new ids) and genres are refreshed and album cover /
// artist image paths updated. It returns the current tracks of the audio files that exist,
// in the order given. No events are published (the caller does).
func (s *Scanner) RescanFiles(ctx context.Context, libraryID int64, relPaths []string) ([]model.Track, error) {
	lib, err := s.st.GetLibrary(ctx, libraryID)
	if err != nil {
		return nil, fmt.Errorf("library %d: %w", libraryID, err)
	}
	if err := checkRoot(lib.Path); err != nil {
		return nil, err
	}
	set := s.settings(ctx)

	// Normalise the paths and expand sidecars / images.
	var audio []string
	seenPath := map[string]bool{}
	coverDirs := map[string]bool{}
	addAudio := func(rel string) {
		if !seenPath[rel] {
			seenPath[rel] = true
			audio = append(audio, rel)
		}
	}
	for _, p := range relPaths {
		rel := strings.Trim(strings.ReplaceAll(strings.TrimSpace(p), `\`, "/"), "/")
		if rel == "" {
			continue
		}
		if _, err := util.SafeJoin(lib.Path, rel); err != nil {
			return nil, err
		}
		dir, name, suffix := util.PathParts(rel)
		switch {
		case tags.IsAudioFile(name):
			addAudio(rel)
		case suffix == "lrc":
			base := strings.TrimSuffix(name, path.Ext(name))
			siblings, err := s.st.DB().R.QueryxContext(ctx,
				`SELECT path FROM tracks WHERE library_id = ? AND dir = ?`, lib.ID, dir)
			if err != nil {
				return nil, err
			}
			for siblings.Next() {
				var sp string
				if err := siblings.Scan(&sp); err != nil {
					_ = siblings.Close()
					return nil, err
				}
				if _, sn, _ := util.PathParts(sp); strings.TrimSuffix(sn, path.Ext(sn)) == base {
					addAudio(sp)
				}
			}
			_ = siblings.Close()
		case util.IsImageSuffix(suffix):
			coverDirs[dir] = true
		}
	}

	s.dbMu.Lock()
	defer s.dbMu.Unlock()

	states, err := s.statesForPaths(ctx, lib.ID, audio)
	if err != nil {
		return nil, err
	}
	res := scanResult{albums: map[string]bool{}, artists: map[string]bool{}}
	type item struct {
		f     fileInfo
		state *trackState
		track *model.Track
	}
	var present []*item
	var gone []*trackState
	for _, rel := range audio {
		abs, _ := util.SafeJoin(lib.Path, rel)
		fi, err := os.Stat(abs)
		switch {
		case errors.Is(err, fs.ErrNotExist) || (err == nil && !fi.Mode().IsRegular()):
			if st := states[rel]; st != nil {
				gone = append(gone, st)
			}
			continue
		case err != nil:
			return nil, fmt.Errorf("stat %s: %w", rel, err)
		}
		base := strings.TrimSuffix(abs, path.Ext(abs))
		present = append(present, &item{
			f: fileInfo{rel: rel, abs: abs, size: fi.Size(), mtime: fi.ModTime().UnixMilli(),
				hasLrc: fileExists(base+".lrc") || fileExists(base+".LRC")},
			state: states[rel],
		})
	}

	// Read tags concurrently.
	g, _ := errgroup.WithContext(ctx)
	g.SetLimit(s.workers)
	errs := make([]error, len(present))
	for i, it := range present {
		g.Go(func() error {
			m, err := tags.Read(it.f.abs, tags.ReadOptions{FixEncoding: set.FixEncodingOnScan})
			if err != nil {
				errs[i] = fmt.Errorf("%s: %w", it.f.rel, err)
				return nil
			}
			t := buildTrack(lib.ID, it.f, m, set)
			it.track = &t
			return nil
		})
	}
	_ = g.Wait()
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	// Ids: keep the row at the path; otherwise reuse a vanished track (move).
	usedGone := map[string]bool{}
	for _, sameName := range []bool{true, false} {
		for _, it := range present {
			if it.track == nil || it.state != nil || it.track.ID != "" {
				continue
			}
			if c := pickMove(gone, usedGone, it.track, sameName, false); c != nil {
				usedGone[c.ID] = true
				it.track.ID = c.ID
				res.touchState(c)
			}
		}
	}
	var batch []model.Track
	for _, it := range present {
		if it.track == nil {
			continue
		}
		if it.state != nil {
			it.track.ID = it.state.ID
			res.touchState(it.state)
		}
		if it.track.ID == "" {
			it.track.ID = util.NewID()
		}
		res.touchTrack(it.track)
		batch = append(batch, *it.track)
	}
	var saveErrs []error
	for len(batch) > 0 {
		n := min(len(batch), batchSize)
		if err := upsertResilient(ctx, s.st, lib.ID, batch[:n], func(t *model.Track, err error) {
			saveErrs = append(saveErrs, fmt.Errorf("%s: %w", t.Path, err))
		}); err != nil {
			return nil, fmt.Errorf("saving tracks: %w", err)
		}
		batch = batch[n:]
	}
	var missing []string
	for _, st := range gone {
		if !usedGone[st.ID] && !st.Missing {
			missing = append(missing, st.ID)
			res.touchState(st)
		}
	}
	if err := s.st.MarkTracksMissing(ctx, missing, true); err != nil {
		return nil, fmt.Errorf("marking tracks missing: %w", err)
	}

	// Albums in directories whose images changed.
	for dir := range coverDirs {
		var ids []string
		if err := s.st.DB().R.SelectContext(ctx, &ids,
			`SELECT DISTINCT album_id FROM tracks WHERE library_id = ? AND missing = 0 AND (dir = ? OR dir LIKE ? ESCAPE '\')`,
			lib.ID, dir, util.EscapeLike(dir)+"/%"); err != nil {
			return nil, err
		}
		for _, id := range ids {
			res.albums[id] = true
		}
		// An artist image next to album folders.
		var artistIDs []string
		if err := s.st.DB().R.SelectContext(ctx, &artistIDs,
			`SELECT DISTINCT album_artist_id FROM tracks WHERE library_id = ? AND missing = 0 AND (dir = ? OR dir LIKE ? ESCAPE '\')`,
			lib.ID, dir, util.EscapeLike(dir)+"/%"); err != nil {
			return nil, err
		}
		for _, id := range artistIDs {
			res.artists[id] = true
		}
	}

	if err := s.refreshTouched(ctx, &res, set, newDirCache()); err != nil {
		return nil, err
	}

	var ids []string
	for _, it := range present {
		if it.track != nil {
			ids = append(ids, it.track.ID)
		}
	}
	tracks, err := s.st.GetTracks(ctx, ids, "")
	if err != nil {
		return nil, err
	}
	var readErrs []error
	for _, e := range errs {
		if e != nil {
			readErrs = append(readErrs, e)
		}
	}
	if err := errors.Join(append(readErrs, saveErrs...)...); err != nil {
		return tracks, fmt.Errorf("rescanning files: %w", err)
	}
	return tracks, nil
}

// refreshTouched refreshes the aggregates and covers of the albums/artists in res.
func (s *Scanner) refreshTouched(ctx context.Context, res *scanResult, set model.Settings, cache *dirCache) error {
	albums := keys(res.albums)
	artists := keys(res.artists)
	if len(albums) == 0 && len(artists) == 0 {
		return nil
	}
	if err := s.st.RefreshAlbums(ctx, albums); err != nil {
		return fmt.Errorf("refreshing albums: %w", err)
	}
	if err := s.st.RefreshArtists(ctx, artists); err != nil {
		return fmt.Errorf("refreshing artists: %w", err)
	}
	if err := s.st.RefreshGenres(ctx); err != nil {
		return fmt.Errorf("refreshing genres: %w", err)
	}
	// Covers of the (still existing) touched albums; artist images for their album artists
	// and for touched artists without albums in the set.
	if err := s.updateCovers(ctx, albums, set, cache, nil); err != nil {
		return fmt.Errorf("updating covers: %w", err)
	}
	return nil
}

// RescanDir quick-scans one directory subtree (relDir "" = whole library, but without the
// global refresh a full scan does) and refreshes the affected aggregates and covers. It
// does not take the library lock; it waits for a running scan to finish. A "library" event
// is published when something changed.
func (s *Scanner) RescanDir(ctx context.Context, libraryID int64, relDir string) error {
	lib, err := s.st.GetLibrary(ctx, libraryID)
	if err != nil {
		return fmt.Errorf("library %d: %w", libraryID, err)
	}
	relDir = strings.Trim(strings.ReplaceAll(relDir, `\`, "/"), "/")
	if _, err := util.SafeJoin(lib.Path, relDir); err != nil {
		return err
	}
	set := s.settings(ctx)

	s.dbMu.Lock()
	defer s.dbMu.Unlock()

	cache := newDirCache()
	ls := &libScan{s: s, lib: *lib, set: set, cache: cache}
	res, err := ls.scan(ctx, relDir)
	if err != nil {
		return err
	}
	// Covers may have changed without audio changes: include every album in the subtree.
	q := `SELECT DISTINCT album_id FROM tracks WHERE library_id = ? AND missing = 0`
	args := []any{lib.ID}
	if relDir != "" {
		q += ` AND (dir = ? OR dir LIKE ? ESCAPE '\')`
		args = append(args, relDir, util.EscapeLike(relDir)+"/%")
	}
	var ids []string
	if err := s.st.DB().R.SelectContext(ctx, &ids, q, args...); err != nil {
		return err
	}
	for _, id := range ids {
		res.albums[id] = true
	}
	if err := s.refreshTouched(ctx, res, set, cache); err != nil {
		return err
	}
	if res.changed() && s.bus != nil {
		s.bus.Publish(events.Library("rescan"))
	}
	return nil
}

// statesForPaths loads the stored rows at the given paths.
func (s *Scanner) statesForPaths(ctx context.Context, libraryID int64, paths []string) (map[string]*trackState, error) {
	out := make(map[string]*trackState, len(paths))
	for _, chunk := range chunkStrings(paths, 400) {
		args := make([]any, 0, len(chunk)+1)
		args = append(args, libraryID)
		for _, p := range chunk {
			args = append(args, p)
		}
		var rows []*trackState
		if err := s.st.DB().R.SelectContext(ctx, &rows,
			`SELECT `+trackStateCols+` FROM tracks WHERE library_id = ? AND path IN (`+placeholders(len(chunk))+`)`, args...); err != nil {
			return nil, fmt.Errorf("loading track states: %w", err)
		}
		for _, r := range rows {
			out[r.Path] = r
		}
	}
	return out, nil
}

func fileExists(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.Mode().IsRegular()
}

func keys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
