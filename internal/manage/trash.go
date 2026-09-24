package manage

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"

	"rainy/internal/model"
	"rainy/internal/store"
	"rainy/internal/tags"
	"rainy/internal/util"
)

// trashRoot returns <data>/trash/<libraryID>.
func (s *Service) trashRoot(libraryID int64) string {
	return filepath.Join(s.cfg.TrashDir(), strconv.FormatInt(libraryID, 10))
}

// moveToTrash moves the library file entry.OriginalPath (and its .lrc sidecar) to
// <trash>/<libraryId>/<original path> (made unique with " (n)"), fills entry.TrashPath /
// Size and records the trash row. Must be called with the library lock held.
func (s *Service) moveToTrash(ctx context.Context, lib *model.Library, entry *model.TrashEntry) error {
	src, err := libPath(lib.Path, entry.OriginalPath)
	if err != nil {
		return err
	}
	fi, err := os.Lstat(src)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("%w: file not found", store.ErrNotFound)
		}
		return err
	}
	if !fi.Mode().IsRegular() {
		return fmt.Errorf("%w: not a regular file", store.ErrInvalid)
	}
	troot := s.trashRoot(lib.ID)
	dst, err := util.SafeJoin(troot, entry.OriginalPath)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	lrc := ""
	if tags.IsAudioFile(src) {
		lrc = ownSidecar(src)
	}
	idx := newNameIndex()
	want := dst
	dst = idx.unique(want)
	// Every trashed file gets a stem of its own ("Song.mp3" and a later "Song.flac" do not
	// share "Song.lrc"): purge and restore address the sidecar by stem.
	for stemTaken(dst) {
		dst = idx.unique(want)
	}
	if err := moveFile(src, dst); err != nil {
		return err
	}
	if lrc != "" {
		if err := moveFile(lrc, stem(dst)+".lrc"); err != nil {
			slog.Warn("manage: moving lyrics sidecar to trash", "path", lrc, "err", err)
		}
	}
	rel, err := filepath.Rel(s.cfg.TrashDir(), dst)
	if err != nil {
		return err
	}
	entry.TrashPath = filepath.ToSlash(rel)
	entry.Size = fi.Size()
	if err := s.st.AddTrash(ctx, entry); err != nil {
		// Put the file back so nothing is lost without a record.
		if moveFile(dst, src) == nil && lrc != "" {
			_ = moveFile(stem(dst)+".lrc", lrc)
		}
		return err
	}
	removeEmptyDirs(lib.Path, filepath.Dir(src))
	return nil
}

// Delete moves the files of tracks to the trash. The track rows are kept (marked
// missing) so that a restore brings back the same id with its annotations and
// playlist entries.
func (s *Service) Delete(ctx context.Context, u *model.User, trackIDs []string) (*BatchResult, error) {
	trackIDs = dedupe(trackIDs)
	if err := checkBatch(len(trackIDs), "trackIds"); err != nil {
		return nil, err
	}
	unlock, err := s.lock(ctx)
	if err != nil {
		return nil, err
	}
	defer unlock()
	ctx = context.WithoutCancel(ctx)
	res := newBatch()
	libs := s.libs()
	var deleted []model.Track
	entries := map[string]*model.TrashEntry{}
	for _, id := range trackIDs {
		t, err := s.st.GetTrack(ctx, id, "")
		if err != nil {
			res.fail(id, "", notFoundTrack(err))
			continue
		}
		lib, err := libs.get(ctx, t.LibraryID)
		if err != nil {
			res.fail(t.ID, t.Path, err)
			continue
		}
		entry := &model.TrashEntry{
			LibraryID: t.LibraryID, OriginalPath: t.Path, Title: t.Title, Artist: t.Artist,
			Album: t.Album, TrackID: t.ID, DeletedBy: u.Username,
		}
		if err := s.moveToTrash(ctx, lib, entry); err != nil {
			res.fail(t.ID, t.Path, fsErr(t.Path, err))
			continue
		}
		if err := s.st.MarkTracksMissing(ctx, []string{t.ID}, true); err != nil {
			slog.Error("manage: marking deleted track missing", "track", t.ID, "err", err)
		}
		deleted = append(deleted, *t)
		entries[t.ID] = entry
	}

	if len(deleted) > 0 {
		ids := trackIDsOf(deleted)
		albums, _ := s.st.AlbumIDsForTracks(ctx, ids)
		artists, _ := s.st.ArtistIDsForTracks(ctx, ids)
		s.refreshAggregates(ctx, albums, artists)
		for _, t := range deleted {
			e := entries[t.ID]
			s.logEdit(ctx, u, "delete", t.ID, t.Path, map[string]any{"trashId": e.ID, "trashPath": e.TrashPath})
		}
		res.Updated = s.tracksByID(ctx, ids, u.ID)
		s.publish("delete")
	}
	if err := res.failure(); err != nil {
		return nil, err
	}
	return res, nil
}

// ListTrash returns the trash entries, newest first.
func (s *Service) ListTrash(ctx context.Context) ([]model.TrashEntry, error) {
	return s.st.ListTrash(ctx)
}

// Restore moves trashed files back to their original location and rescans them (the
// track keeps its id). An existing file at the original path is a conflict.
func (s *Service) Restore(ctx context.Context, u *model.User, ids []string) (*BatchResult, error) {
	ids = dedupe(ids)
	if err := checkBatch(len(ids), "ids"); err != nil {
		return nil, err
	}
	unlock, err := s.lock(ctx)
	if err != nil {
		return nil, err
	}
	defer unlock()
	ctx = context.WithoutCancel(ctx)
	res := newBatch()
	libs := s.libs()
	restored := map[int64][]string{} // library → restored audio paths
	var logged []*model.TrashEntry
	imageDirs := map[int64][]string{}
	for _, id := range ids {
		e, err := s.st.GetTrash(ctx, id)
		if err != nil {
			res.fail("", "", fmt.Errorf("trash entry %s: %w", id, err))
			continue
		}
		if err := s.restoreOne(ctx, libs, e); err != nil {
			res.fail(e.TrackID, e.OriginalPath, fsErr(e.OriginalPath, err))
			continue
		}
		if tags.IsAudioFile(e.OriginalPath) {
			restored[e.LibraryID] = append(restored[e.LibraryID], e.OriginalPath)
		} else {
			dir, _, _ := util.PathParts(e.OriginalPath)
			imageDirs[e.LibraryID] = append(imageDirs[e.LibraryID], dir)
		}
		logged = append(logged, e)
	}

	for lib, paths := range restored {
		s.rescan(ctx, lib, paths)
	}
	for lib, dirs := range imageDirs {
		s.refreshDirCovers(ctx, lib, dedupe(dirs))
	}
	var trackIDs []string
	for _, e := range logged {
		trackID := e.TrackID
		if t, err := s.st.GetTrackByPath(ctx, e.LibraryID, e.OriginalPath); err == nil {
			trackID = t.ID
			trackIDs = append(trackIDs, t.ID)
		}
		s.logEdit(ctx, u, "restore", trackID, e.OriginalPath, map[string]any{"trashId": e.ID, "from": e.TrashPath, "to": e.OriginalPath})
	}
	res.Updated = s.tracksByID(ctx, trackIDs, u.ID)
	if len(logged) > 0 {
		s.publish("restore")
	}
	if err := res.failure(); err != nil {
		return nil, err
	}
	return res, nil
}

func (s *Service) restoreOne(ctx context.Context, libs *libraries, e *model.TrashEntry) error {
	lib, err := libs.get(ctx, e.LibraryID)
	if err != nil {
		return fmt.Errorf("%w: the library of this file no longer exists", store.ErrConflict)
	}
	src, err := util.SafeJoin(s.cfg.TrashDir(), e.TrashPath)
	if err != nil {
		return err
	}
	if !exists(src) {
		return fmt.Errorf("%w: the trashed file is gone", store.ErrNotFound)
	}
	dst, err := libPath(lib.Path, e.OriginalPath)
	if err != nil {
		return err
	}
	if newNameIndex().taken(dst) {
		return fmt.Errorf("%w: a file already exists at %s", store.ErrConflict, e.OriginalPath)
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	if err := moveFile(src, dst); err != nil {
		return err
	}
	if lrc := stem(src) + ".lrc"; exists(lrc) {
		if err := moveFile(lrc, stem(dst)+".lrc"); err != nil {
			slog.Warn("manage: restoring lyrics sidecar", "path", lrc, "err", err)
		}
	}
	if err := s.st.DeleteTrash(ctx, e.ID); err != nil && !errors.Is(err, store.ErrNotFound) {
		return err
	}
	removeEmptyDirs(s.cfg.TrashDir(), filepath.Dir(src))
	return nil
}

// PurgeTrash permanently deletes trashed files and the rows of their (missing) tracks.
// ids == nil (omitted in the request) empties the whole trash; an empty non-nil list
// purges nothing.
func (s *Service) PurgeTrash(ctx context.Context, u *model.User, ids []string) (int, error) {
	var entries []model.TrashEntry
	if ids == nil {
		all, err := s.st.ListTrash(ctx)
		if err != nil {
			return 0, err
		}
		entries = all
	} else {
		for _, id := range dedupe(ids) {
			e, err := s.st.GetTrash(ctx, id)
			if errors.Is(err, store.ErrNotFound) {
				continue
			}
			if err != nil {
				return 0, err
			}
			entries = append(entries, *e)
		}
	}
	if len(entries) == 0 {
		return 0, nil
	}
	unlock, err := s.lock(ctx)
	if err != nil {
		return 0, err
	}
	defer unlock()
	ctx = context.WithoutCancel(ctx)
	// Re-read the entries under the lock: while waiting, some may have been restored or
	// purged and their trash paths reused by newer deletions.
	current, err := s.st.ListTrash(ctx)
	if err != nil {
		return 0, err
	}
	still := make(map[string]model.TrashEntry, len(current))
	for _, e := range current {
		still[e.ID] = e
	}
	kept := entries[:0]
	for _, e := range entries {
		if cur, ok := still[e.ID]; ok {
			kept = append(kept, cur)
		}
	}
	entries = kept
	purged := 0
	var trackIDs []string
	var firstErr error
	for _, e := range entries {
		p, err := util.SafeJoin(s.cfg.TrashDir(), e.TrashPath)
		if err != nil {
			firstErr = cmpErr(firstErr, err)
			continue
		}
		for _, f := range []string{p, stem(p) + ".lrc"} {
			if err := os.Remove(f); err != nil && !errors.Is(err, fs.ErrNotExist) {
				firstErr = cmpErr(firstErr, fsErr(e.TrashPath, err))
			}
		}
		if exists(p) {
			continue
		}
		if err := s.st.DeleteTrash(ctx, e.ID); err != nil && !errors.Is(err, store.ErrNotFound) {
			firstErr = cmpErr(firstErr, err)
			continue
		}
		removeEmptyDirs(s.cfg.TrashDir(), filepath.Dir(p))
		if e.TrackID != "" {
			trackIDs = append(trackIDs, e.TrackID)
		}
		purged++
		s.logEdit(ctx, u, "purge", e.TrackID, e.OriginalPath, map[string]any{"trashId": e.ID})
	}

	if err := s.deleteMissingRows(ctx, trackIDs); err != nil {
		return purged, err
	}
	if purged > 0 {
		s.publish("purge")
	}
	if purged == 0 && firstErr != nil {
		return 0, firstErr
	}
	return purged, nil
}

func cmpErr(first, err error) error {
	if first != nil {
		return first
	}
	return err
}

// deleteMissingRows purges the rows of tracks that are (still) missing.
func (s *Service) deleteMissingRows(ctx context.Context, ids []string) error {
	ids = dedupe(ids)
	if len(ids) == 0 {
		return nil
	}
	ts, err := s.st.GetTracks(ctx, ids, "")
	if err != nil {
		return err
	}
	var gone []string
	for _, t := range ts {
		if t.Missing {
			gone = append(gone, t.ID)
		}
	}
	if len(gone) == 0 {
		return nil
	}
	albums, _ := s.st.AlbumIDsForTracks(ctx, gone)
	artists, _ := s.st.ArtistIDsForTracks(ctx, gone)
	if err := s.st.DeleteTracks(ctx, gone); err != nil {
		return err
	}
	s.refreshAggregates(ctx, albums, artists)
	return nil
}

// PurgeMissing removes the rows of missing tracks: all of them when trackIDs == nil
// (omitted in the request), none for an empty non-nil list. Tracks whose file is in the
// trash are kept so they can still be restored.
func (s *Service) PurgeMissing(ctx context.Context, u *model.User, trackIDs []string) (int, error) {
	if trackIDs != nil && len(dedupe(trackIDs)) == 0 {
		return 0, nil
	}
	// Hold the library lock: a running scan may be re-attaching a moved file to one of
	// these rows right now.
	unlock, err := s.lock(ctx)
	if err != nil {
		return 0, err
	}
	defer unlock()
	candidates, err := s.missingNotTrashed(ctx, dedupe(trackIDs))
	if err != nil {
		return 0, err
	}
	if len(candidates) == 0 {
		return 0, nil
	}
	ids := make([]string, len(candidates))
	for i, c := range candidates {
		ids[i] = c.ID
	}
	ctx = context.WithoutCancel(ctx)
	albums, _ := s.st.AlbumIDsForTracks(ctx, ids)
	artists, _ := s.st.ArtistIDsForTracks(ctx, ids)
	if err := s.st.DeleteTracks(ctx, ids); err != nil {
		return 0, err
	}
	s.refreshAggregates(ctx, albums, artists)
	for _, c := range candidates {
		s.logEdit(ctx, u, "purge", c.ID, c.Path, map[string]any{"missing": true})
	}
	s.publish("purge")
	return len(candidates), nil
}

type idPath struct {
	ID   string `db:"id"`
	Path string `db:"path"`
}

// missingNotTrashed returns missing tracks (restricted to ids when given) that have no
// trash entry.
func (s *Service) missingNotTrashed(ctx context.Context, ids []string) ([]idPath, error) {
	const q = `SELECT id, path FROM tracks WHERE missing = 1
		AND id NOT IN (SELECT track_id FROM trash WHERE track_id != '')`
	var rows []idPath
	if err := s.st.DB().R.SelectContext(ctx, &rows, q+` ORDER BY path`); err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return rows, nil
	}
	want := map[string]bool{}
	for _, id := range ids {
		want[id] = true
	}
	out := rows[:0]
	for _, r := range rows {
		if want[r.ID] {
			out = append(out, r)
		}
	}
	return out, nil
}
