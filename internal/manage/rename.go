package manage

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"rainy/internal/model"
	"rainy/internal/store"
	"rainy/internal/tags"
	"rainy/internal/util"
)

// Rename plan statuses.
const (
	PlanOK        = "ok"
	PlanUnchanged = "unchanged"
	PlanConflict  = "conflict"
	PlanInvalid   = "invalid"
)

// RenamePlan is one row of a rename preview.
type RenamePlan struct {
	TrackID string `json:"trackId"`
	From    string `json:"from"`
	To      string `json:"to"`
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`

	t       *model.Track
	lib     *model.Library
	fromAbs string
	toAbs   string
}

func (p *RenamePlan) set(status, msg string) { p.Status, p.Message = status, msg }

// PreviewRename computes where each track would move with pattern. Nothing is written.
func (s *Service) PreviewRename(ctx context.Context, trackIDs []string, pattern string) ([]RenamePlan, error) {
	p, err := ParsePattern(pattern)
	if err != nil {
		return nil, err
	}
	plans, err := s.planRename(ctx, trackIDs, p)
	if err != nil {
		return nil, err
	}
	out := make([]RenamePlan, len(plans))
	for i, pl := range plans {
		out[i] = *pl
	}
	return out, nil
}

// planRename renders the target path of every track and classifies it (ok, unchanged,
// conflict with an existing file or another item of the batch — compared
// case-insensitively — or invalid).
func (s *Service) planRename(ctx context.Context, trackIDs []string, p *Pattern) ([]*RenamePlan, error) {
	trackIDs = dedupe(trackIDs)
	if err := checkBatch(len(trackIDs), "trackIds"); err != nil {
		return nil, err
	}
	tracks, err := s.st.GetTracks(ctx, trackIDs, "")
	if err != nil {
		return nil, err
	}
	byID := make(map[string]*model.Track, len(tracks))
	for i := range tracks {
		byID[tracks[i].ID] = &tracks[i]
	}
	libs := s.libs()
	multiDisc := map[string]bool{}
	plans := make([]*RenamePlan, 0, len(trackIDs))
	for _, id := range trackIDs {
		pl := &RenamePlan{TrackID: id}
		plans = append(plans, pl)
		t := byID[id]
		if t == nil {
			pl.set(PlanInvalid, "track not found")
			continue
		}
		pl.t, pl.From, pl.To = t, t.Path, t.Path
		if t.Missing {
			pl.set(PlanInvalid, "the file is missing")
			continue
		}
		lib, err := libs.get(ctx, t.LibraryID)
		if err != nil {
			pl.set(PlanInvalid, err.Error())
			continue
		}
		pl.lib = lib
		md, seen := multiDisc[t.AlbumID]
		if !seen {
			if al, err := s.st.GetAlbum(ctx, t.AlbumID, ""); err == nil {
				md = al.DiscCount > 1
			}
			multiDisc[t.AlbumID] = md
		}
		ext := strings.TrimPrefix(filepath.Ext(t.Filename), ".")
		to, err := renderPath(p, trackValues(t, md), ext)
		if err != nil {
			pl.set(PlanInvalid, strings.TrimPrefix(err.Error(), store.ErrInvalid.Error()+": "))
			continue
		}
		pl.To = to
		if pl.fromAbs, err = libPath(lib.Path, t.Path); err != nil {
			pl.set(PlanInvalid, err.Error())
			continue
		}
		if pl.toAbs, err = libPath(lib.Path, to); err != nil {
			pl.set(PlanInvalid, err.Error())
			continue
		}
		if to == t.Path {
			pl.set(PlanUnchanged, "")
			continue
		}
		pl.set(PlanOK, "")
	}

	// Collisions inside the batch.
	count := map[string]int{}
	key := func(pl *RenamePlan) string { return fmt.Sprint(pl.t.LibraryID) + "\x00" + strings.ToLower(pl.To) }
	for _, pl := range plans {
		if pl.t != nil && (pl.Status == PlanOK || pl.Status == PlanUnchanged) {
			count[key(pl)]++
		}
	}
	idx := newNameIndex()
	for _, pl := range plans {
		if pl.Status != PlanOK {
			continue // an unchanged track keeps its name; the others collide with it
		}
		if count[key(pl)] > 1 {
			pl.set(PlanConflict, "several tracks would get this name")
			continue
		}
		if strings.EqualFold(pl.From, pl.To) {
			// Case-only rename: fine unless a distinct file with the new spelling exists
			// (possible on case-sensitive file systems).
			if exists(pl.toAbs) && !sameFile(pl.fromAbs, pl.toAbs) {
				pl.set(PlanConflict, "a file with this name already exists")
			}
			continue
		}
		if idx.taken(pl.toAbs) {
			pl.set(PlanConflict, "a file with this name already exists")
			continue
		}
		if other, err := s.st.GetTrackByPath(ctx, pl.t.LibraryID, pl.To); err == nil && other.ID != pl.t.ID {
			pl.set(PlanConflict, "the path belongs to another (missing) track")
		}
	}
	return plans, nil
}

// Rename moves the files of tracks to the paths produced by pattern (see planRename).
// Sidecar .lrc files move with their track; when every audio file of a directory moves
// to the same new directory, the folder images move too; directories left empty are
// removed. Track ids are kept.
func (s *Service) Rename(ctx context.Context, u *model.User, trackIDs []string, pattern string) (*BatchResult, error) {
	p, err := ParsePattern(pattern)
	if err != nil {
		return nil, err
	}
	if err := checkBatch(len(dedupe(trackIDs)), "trackIds"); err != nil {
		return nil, err
	}
	unlock, err := s.lock(ctx)
	if err != nil {
		return nil, err
	}
	defer unlock()
	ctx = context.WithoutCancel(ctx)
	plans, err := s.planRename(ctx, trackIDs, p)
	if err != nil {
		return nil, err
	}
	res := newBatch()
	var moved []*RenamePlan
	srcDirs := map[string]*model.Library{}
	for _, pl := range plans {
		switch pl.Status {
		case PlanOK:
		case PlanUnchanged:
			continue
		default:
			res.fail(pl.TrackID, pl.From, fmt.Errorf("%s: %s", pl.Status, pl.Message))
			continue
		}
		if err := s.applyRename(ctx, pl); err != nil {
			removeEmptyDirs(pl.lib.Path, filepath.Dir(pl.toAbs)) // folders created for nothing
			res.fail(pl.TrackID, pl.From, fsErr(pl.From, err))
			continue
		}
		moved = append(moved, pl)
		srcDirs[filepath.Dir(pl.fromAbs)] = pl.lib
	}
	coverDirs := map[int64][]string{}
	for srcDir, dstDir := range folderImageMoves(moved) {
		lib := srcDirs[srcDir]
		if n := moveFolderImages(srcDir, dstDir); n > 0 {
			if rel, err := util.ToRel(lib.Path, dstDir); err == nil {
				coverDirs[lib.ID] = append(coverDirs[lib.ID], rel)
			}
		}
	}
	for dir, lib := range srcDirs {
		removeEmptyDirs(lib.Path, dir)
	}

	byLib := map[int64][]string{}
	for _, pl := range moved {
		// Old and new paths: the scanner then also refreshes what depended on the old
		// location (sidecars, folder covers).
		// (Not for case-only renames: on case-insensitive file systems the old spelling
		// still resolves to the file and would be added as a second track.)
		if !strings.EqualFold(pl.From, pl.To) {
			byLib[pl.t.LibraryID] = append(byLib[pl.t.LibraryID], pl.From)
		}
		byLib[pl.t.LibraryID] = append(byLib[pl.t.LibraryID], pl.To)
	}
	for lib, paths := range byLib {
		s.rescan(ctx, lib, paths)
	}
	for lib, dirs := range coverDirs {
		s.refreshDirCovers(ctx, lib, dedupe(dirs))
	}
	for _, pl := range moved {
		s.logEdit(ctx, u, "rename", pl.TrackID, pl.To, map[string]any{"from": pl.From, "to": pl.To})
	}
	ids := make([]string, len(moved))
	for i, pl := range moved {
		ids[i] = pl.TrackID
	}
	res.Updated = s.tracksByID(ctx, ids, u.ID)
	if len(moved) > 0 {
		s.publish("rename")
	}
	if err := res.failure(); err != nil {
		return nil, err
	}
	return res, nil
}

// applyRename moves one file (and its sidecar) and updates the track path, rolling the
// move back when the database update fails.
func (s *Service) applyRename(ctx context.Context, pl *RenamePlan) error {
	if err := os.MkdirAll(filepath.Dir(pl.toAbs), 0o755); err != nil {
		return err
	}
	lrc := ownSidecar(pl.fromAbs)
	caseOnly := strings.EqualFold(pl.fromAbs, pl.toAbs)
	move := moveFile
	if caseOnly {
		move = renameCaseOnly
	} else if newNameIndex().taken(pl.toAbs) {
		return errExists // appeared since planning
	}
	if err := move(pl.fromAbs, pl.toAbs); err != nil {
		return err
	}
	if err := s.st.UpdateTrackPath(ctx, pl.TrackID, pl.To); err != nil {
		if rerr := move(pl.toAbs, pl.fromAbs); rerr != nil {
			slog.Error("manage: rolling back rename failed", "from", pl.To, "to", pl.From, "err", rerr)
		}
		return err
	}
	if lrc != "" {
		dst := stem(pl.toAbs) + ".lrc"
		var err error
		if strings.EqualFold(lrc, dst) {
			err = renameCaseOnly(lrc, dst)
		} else {
			err = moveFile(lrc, dst)
		}
		if err != nil && !errors.Is(err, errExists) {
			slog.Warn("manage: moving lyrics sidecar", "path", lrc, "err", err)
		}
	}
	return nil
}

// folderImageMoves returns source dir → target dir for directories whose audio files
// were all moved (no audio file is left) into one common new directory. The library root
// and directories whose sub-directories still hold audio (an artist folder with
// artist.jpg next to its album folders) never give their images away.
func folderImageMoves(moved []*RenamePlan) map[string]string {
	target := map[string]string{} // src dir → dst dir ("" when ambiguous)
	for _, pl := range moved {
		src, dst := filepath.Dir(pl.fromAbs), filepath.Dir(pl.toAbs)
		if strings.EqualFold(filepath.Clean(src), filepath.Clean(pl.lib.Path)) {
			continue
		}
		if prev, ok := target[src]; ok && prev != dst {
			target[src] = ""
		} else if !ok {
			target[src] = dst
		}
	}
	out := map[string]string{}
	for src, dst := range target {
		if dst == "" || strings.EqualFold(src, dst) {
			continue
		}
		entries, err := os.ReadDir(src)
		if err != nil {
			continue
		}
		left := false
		for _, e := range entries {
			if !e.IsDir() && tags.IsAudioFile(e.Name()) && !util.IsHiddenOrSystem(e.Name()) {
				left = true
				break
			}
		}
		if !left && !subdirsHoldAudio(src) {
			out[src] = dst
		}
	}
	return out
}

// subdirsHoldAudio reports whether any (non-hidden) directory below dir contains an audio
// file. Unreadable directories count as holding audio (keep the images where they are).
func subdirsHoldAudio(dir string) bool {
	found := errors.New("found")
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return found
		}
		if p == dir {
			return nil
		}
		if util.IsHiddenOrSystem(d.Name()) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if !d.IsDir() && filepath.Dir(p) != dir && tags.IsAudioFile(d.Name()) {
			return found
		}
		return nil
	})
	return err != nil
}

// moveFolderImages moves the image files of src into dst (never overwriting) and returns
// how many moved.
func moveFolderImages(src, dst string) int {
	entries, err := os.ReadDir(src)
	if err != nil {
		return 0
	}
	idx := newNameIndex()
	n := 0
	for _, e := range entries {
		name := e.Name()
		ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(name), "."))
		if e.IsDir() || util.IsHiddenOrSystem(name) || !util.IsImageSuffix(ext) {
			continue
		}
		to := filepath.Join(dst, name)
		if idx.taken(to) {
			continue
		}
		if err := moveFile(filepath.Join(src, name), to); err != nil {
			slog.Warn("manage: moving folder image", "path", filepath.Join(src, name), "err", err)
			continue
		}
		idx.reserve(to)
		n++
	}
	return n
}
