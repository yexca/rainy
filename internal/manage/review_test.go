package manage

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"rainy/internal/model"
	"rainy/internal/store"
	"rainy/internal/tags"
)

// An explicit empty list must never mean "everything": a UI bug that sends [] (e.g. every
// fix deselected) must not purge the whole trash or rewrite every file of the library.
func TestEmptyListsAreNoOps(t *testing.T) {
	e := newEnv(t)
	gone := e.track("Static Noise/Unknown Signals/04 Dead Air.ogg")
	if err := e.st.MarkTracksMissing(e.ctx, []string{gone.ID}, true); err != nil {
		t.Fatal(err)
	}
	trashed := e.track("Static Noise/Unknown Signals/01 Signal 1.ogg")
	if _, err := e.svc.Delete(e.ctx, e.user, []string{trashed.ID}); err != nil {
		t.Fatal(err)
	}

	if n, err := e.svc.PurgeTrash(e.ctx, e.user, []string{}); err != nil || n != 0 {
		t.Fatalf("purge trash []: %d %v", n, err)
	}
	if trash, _ := e.svc.ListTrash(e.ctx); len(trash) != 1 {
		t.Fatalf("trash emptied by []: %+v", trash)
	}
	if n, err := e.svc.PurgeMissing(e.ctx, e.user, []string{}); err != nil || n != 0 {
		t.Fatalf("purge missing []: %d %v", n, err)
	}
	if _, err := e.st.GetTrack(e.ctx, gone.ID, ""); err != nil {
		t.Fatalf("missing track purged by []: %v", err)
	}

	rel := "林雨晴/夏日微风 (2021)/02 海边的约定.mp3"
	requireTags(t, e.abs(rel))
	bad := gbkMojibake(t, "海边的约定")
	if err := tags.Write(e.abs(rel), map[string][]string{"TITLE": {bad}}); err != nil {
		t.Fatal(err)
	}
	if raw, _ := tags.ReadRaw(e.abs(rel)); !slices.Equal(raw["TITLE"], []string{bad}) {
		t.Skipf("tag round trip changed the mojibake: %q", raw["TITLE"])
	}
	res, err := e.svc.Encoding(e.ctx, e.user, []string{}, true)
	if err != nil || len(res.Items) != 0 || res.Result == nil || len(res.Result.Updated) != 0 {
		t.Fatalf("encoding []: %+v %v", res, err)
	}
	if raw, _ := tags.ReadRaw(e.abs(rel)); !slices.Equal(raw["TITLE"], []string{bad}) {
		t.Fatalf("encoding [] rewrote a file: %q", raw["TITLE"])
	}
	// nil (omitted) still means every missing track / the whole trash.
	if n, err := e.svc.PurgeMissing(e.ctx, e.user, nil); err != nil || n != 1 {
		t.Fatalf("purge missing nil: %d %v", n, err)
	}
	if n, err := e.svc.PurgeTrash(e.ctx, e.user, nil); err != nil || n != 1 {
		t.Fatalf("purge trash nil: %d %v", n, err)
	}
}

// Generated names must not become hidden or NAS system entries: the scanner skips those,
// so the moved/uploaded files would silently vanish from the library on the next scan.
func TestPatternNeutralizesHiddenAndSystemNames(t *testing.T) {
	p, err := ParsePattern("{albumartist}/{album}/{title}")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		v    Values
		want string
	}{
		{Values{AlbumArtist: ".hack", Album: "@eaDir", Title: ".intro"}, "_.hack/_@eaDir/_.intro.mp3"},
		{Values{AlbumArtist: "#recycle", Album: "lost+found", Title: "x"}, "_#recycle/_lost+found/x.mp3"},
		{Values{AlbumArtist: "A", Album: "B", Title: ".rainyignore"}, "A/B/_.rainyignore.mp3"},
	} {
		got, err := renderPath(p, tc.v, "mp3")
		if err != nil || got != tc.want {
			t.Errorf("%+v: %q %v, want %q", tc.v, got, err, tc.want)
		}
	}
}

// A track that stays where it is must be reported "unchanged", not "conflict", when
// another track of the batch would take its path.
func TestRenameUnchangedIsNotAConflict(t *testing.T) {
	e := newEnv(t)
	keep := e.track("Static Noise/Unknown Signals/01 Signal 1.ogg")
	other := e.track("Static Noise/Unknown Signals/02 Signal 2.ogg")
	other.Title, other.TrackNumber = keep.Title, keep.TrackNumber
	if err := e.st.UpsertTracks(e.ctx, []model.Track{*other}); err != nil {
		t.Fatal(err)
	}
	plans, err := e.svc.PreviewRename(e.ctx, []string{keep.ID, other.ID}, "{artist}/{album}/{track:2} {title}")
	if err != nil {
		t.Fatal(err)
	}
	got := planStatus(plans)
	if got[keep.ID].Status != PlanUnchanged {
		t.Errorf("kept track: %+v", got[keep.ID])
	}
	if got[other.ID].Status != PlanConflict {
		t.Errorf("colliding track: %+v", got[other.ID])
	}
}

// Only OS metadata files (and NAS thumbnail folders) may be deleted with an emptied
// folder; a folder that merely has a junk-looking name must never be removed with its
// content.
func TestRemoveEmptyDirsKeepsRealContent(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "A", "B")
	keep := filepath.Join(dir, "._stuff", "real.flac")
	if err := os.MkdirAll(filepath.Dir(keep), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keep, []byte("audio"), 0o644); err != nil {
		t.Fatal(err)
	}
	removeEmptyDirs(root, dir)
	if !exists(keep) {
		t.Fatal("a directory with real content was deleted")
	}

	junk := filepath.Join(root, "C", "D")
	for _, p := range []string{".DS_Store", "Thumbs.db", "._01 x.mp3", "@eaDir/01 x.mp3/SYNOPHOTO_THUMB_S.jpg"} {
		f := filepath.Join(junk, filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(f), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(f, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	removeEmptyDirs(root, junk)
	if exists(filepath.Join(root, "C")) {
		t.Fatal("junk-only folders must be removed")
	}
	if !exists(root) {
		t.Fatal("root removed")
	}
}

// Saving a folder image removes every other cover.* variant whatever its letter case
// (on case-sensitive file systems "Cover.JPG" is a different file than "cover.jpg").
func TestSaveFolderImageReplacesCaseVariants(t *testing.T) {
	e := newEnvAt(t, t.TempDir(), false)
	root := e.lib.Path
	for _, n := range []string{"Cover.PNG", "COVER.png", "cover.webp", "folder.jpg"} {
		// Some names collide on case-insensitive file systems; that is fine.
		if err := os.WriteFile(filepath.Join(root, n), []byte("old"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	img := pngImage(t, 20, 20)
	pc, err := prepareCover(img)
	if err != nil {
		t.Fatal(err)
	}
	p, err := e.svc.saveFolderImage(e.ctx, e.user, e.lib, root, pc)
	if err != nil {
		t.Fatal(err)
	}
	entries, _ := os.ReadDir(root)
	var covers []string
	for _, en := range entries {
		if strings.EqualFold(stem(en.Name()), "cover") {
			covers = append(covers, en.Name())
		}
	}
	if len(covers) != 1 || covers[0] != "cover.png" {
		t.Fatalf("cover variants left: %v", covers)
	}
	if b := mustRead(t, p); string(b) != string(img) {
		t.Fatal("new image not written")
	}
	if !exists(filepath.Join(root, "folder.jpg")) {
		t.Fatal("other folder images must be kept")
	}
}

// A sidecar .lrc that is a symlink must never be read or written through: it could point
// anywhere outside the library.
func TestSidecarSymlinkNotFollowed(t *testing.T) {
	e := newEnv(t)
	rel := "Static Noise/Unknown Signals/01 Signal 1.ogg"
	tr := e.track(rel)
	outside := filepath.Join(t.TempDir(), "secret.txt")
	if err := os.WriteFile(outside, []byte("secret"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, e.abs("Static Noise/Unknown Signals/01 Signal 1.lrc")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if requireTagsOK(e.abs(rel)) {
		tt, err := e.svc.TrackTags(e.ctx, tr.ID, e.user.ID)
		if err != nil {
			t.Fatal(err)
		}
		if tt.Lrc != nil {
			t.Fatalf("symlinked sidecar content exposed: %q", *tt.Lrc)
		}
	}
	if _, err := e.svc.SetLyrics(e.ctx, e.user, tr.ID, "[00:01.00]pwned", "lrc"); err == nil {
		t.Fatal("writing through a symlinked sidecar must fail")
	}
	if b := mustRead(t, outside); string(b) != "secret" {
		t.Fatalf("file outside the library modified: %q", b)
	}
	// A dangling symlink must not be used to create a file outside the library either.
	target := filepath.Join(t.TempDir(), "created.lrc")
	rel2 := "Static Noise/Unknown Signals/02 Signal 2.ogg"
	if err := os.Symlink(target, e.abs("Static Noise/Unknown Signals/02 Signal 2.lrc")); err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.SetLyrics(e.ctx, e.user, e.track(rel2).ID, "hello", "lrc"); err == nil {
		t.Fatal("writing through a dangling symlink must fail")
	}
	if exists(target) {
		t.Fatal("file created outside the library")
	}
}

func requireTagsOK(file string) bool {
	_, err := tags.ReadRaw(file)
	return err == nil
}

// Lyrics for a track whose file is gone must not create an orphaned sidecar.
func TestSetLyricsMissingFile(t *testing.T) {
	e := newEnv(t)
	rel := "Static Noise/Unknown Signals/03 Interference.ogg"
	tr := e.track(rel)
	if err := os.Remove(e.abs(rel)); err != nil {
		t.Fatal(err)
	}
	_, err := e.svc.SetLyrics(e.ctx, e.user, tr.ID, "text", "lrc")
	if !errors.Is(err, store.ErrConflict) && !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("got %v", err)
	}
	if e.exists("Static Noise/Unknown Signals/03 Interference.lrc") {
		t.Fatal("orphaned sidecar created")
	}
}

// The folder rescan is a scan: it must hold the library lock so it never observes a
// half-finished file operation.
func TestRescanFolderHoldsLibraryLock(t *testing.T) {
	e := newEnv(t)
	e.sc.checkLock = true
	if err := e.svc.RescanFolder(e.ctx, e.lib.ID, "林雨晴"); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		e.sc.callsMu.Lock()
		n := len(e.sc.dirLocked)
		e.sc.callsMu.Unlock()
		if n > 0 || time.Now().After(deadline) {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	e.sc.callsMu.Lock()
	defer e.sc.callsMu.Unlock()
	if len(e.sc.dirLocked) != 1 || !e.sc.dirLocked[0] {
		t.Fatalf("RescanDir ran without the library lock: %v", e.sc.dirLocked)
	}
}

// AppleDouble files ("._x.mp3", common in folders copied from macOS) are rejected as
// unsupported; hidden directories in upload names are neutralized.
func TestUploadHiddenNames(t *testing.T) {
	e := newEnv(t)
	mp3, err := os.ReadFile(e.abs("林雨晴/夏日微风 (2021)/01 夏日微风.mp3"))
	if err != nil {
		t.Fatal(err)
	}
	fields := map[string]string{"libraryId": fmt.Sprint(e.lib.ID)}
	res, err := e.svc.Upload(e.ctx, e.user, multipartBody(t, []upFile{{"Album/._01.mp3", mp3}, {".git/01.mp3", mp3}}, fields))
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Errors) != 1 || !strings.Contains(res.Errors[0].Error, "unsupported") {
		t.Fatalf("errors %+v", res.Errors)
	}
	if len(res.Updated) != 1 || res.Updated[0].Path != "_.git/01.mp3" {
		t.Fatalf("updated %+v", res.Updated)
	}
}

// Client-supplied folders may not point into hidden or NAS system folders (e.g. a data
// directory that lives inside the music folder).
func TestHiddenFolderParamsRejected(t *testing.T) {
	e := newEnv(t)
	if err := os.MkdirAll(e.abs(".rainy/trash"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, dir := range []string{".rainy", ".rainy/trash", "林雨晴/@eaDir"} {
		if _, err := e.svc.ListFolder(e.ctx, e.lib.ID, dir); !errors.Is(err, store.ErrInvalid) {
			t.Errorf("list %q: %v", dir, err)
		}
		if err := e.svc.RescanFolder(e.ctx, e.lib.ID, dir); !errors.Is(err, store.ErrInvalid) {
			t.Errorf("rescan %q: %v", dir, err)
		}
	}
	mp3, err := os.ReadFile(e.abs("林雨晴/夏日微风 (2021)/01 夏日微风.mp3"))
	if err != nil {
		t.Fatal(err)
	}
	fields := map[string]string{"libraryId": fmt.Sprint(e.lib.ID), "dir": ".rainy/trash"}
	if _, err := e.svc.Upload(e.ctx, e.user, multipartBody(t, []upFile{{"x.mp3", mp3}}, fields)); !errors.Is(err, store.ErrInvalid) {
		t.Fatalf("upload into hidden dir: %v", err)
	}
	if entries, _ := os.ReadDir(e.abs(".rainy/trash")); len(entries) != 0 {
		t.Fatal("file placed in a hidden folder")
	}
}

// A library whose folder vanished (unmounted volume) is a conflict, not a server error.
func TestLibraryRootMissingIsConflict(t *testing.T) {
	e := newEnv(t)
	tr := e.track("Static Noise/Unknown Signals/01 Signal 1.ogg")
	lib := *e.lib
	lib.Path = filepath.Join(t.TempDir(), "unmounted")
	if err := e.st.UpdateLibrary(e.ctx, &lib); err != nil {
		t.Fatal(err)
	}
	res, err := e.svc.Delete(e.ctx, e.user, []string{tr.ID})
	if err != nil || len(res.Errors) != 1 || !strings.Contains(res.Errors[0].Error, "not accessible") {
		t.Fatalf("%+v %v", res, err)
	}
	if _, err := e.svc.ListFolder(e.ctx, e.lib.ID, ""); !errors.Is(err, store.ErrConflict) {
		t.Fatalf("list: %v", err)
	}
}

// A symlinked folder inside the library that points outside it must never be written to
// or listed: rename targets, upload folders, the folder browser and restores all refuse.
func TestSymlinkedDirEscapes(t *testing.T) {
	e := newEnv(t)
	outside := t.TempDir()
	if err := os.Symlink(outside, e.abs("Escape")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	tr := e.track("Static Noise/Unknown Signals/01 Signal 1.ogg")
	plans, err := e.svc.PreviewRename(e.ctx, []string{tr.ID}, "Escape/{title}")
	if err != nil || plans[0].Status != PlanInvalid {
		t.Fatalf("rename into symlink: %+v %v", plans, err)
	}
	res, err := e.svc.Rename(e.ctx, e.user, []string{tr.ID}, "Escape/{title}")
	if err != nil || len(res.Errors) != 1 || !e.exists(tr.Path) {
		t.Fatalf("rename: %+v %v", res, err)
	}
	if _, err := e.svc.ListFolder(e.ctx, e.lib.ID, "Escape"); err == nil {
		t.Fatal("listing a symlink escape")
	}
	mp3, _ := os.ReadFile(e.abs("林雨晴/夏日微风 (2021)/01 夏日微风.mp3"))
	fields := map[string]string{"libraryId": fmt.Sprint(e.lib.ID), "dir": "Escape/sub"}
	if _, err := e.svc.Upload(e.ctx, e.user, multipartBody(t, []upFile{{"x.mp3", mp3}}, fields)); err == nil {
		t.Fatal("upload through a symlink escape")
	}
	// Restore: delete a track, then replace its folder by an escaping symlink.
	del, err := e.svc.Delete(e.ctx, e.user, []string{tr.ID})
	if err != nil || len(del.Errors) != 0 {
		t.Fatalf("delete %+v %v", del, err)
	}
	dir := e.abs("Static Noise/Unknown Signals")
	if err := os.RemoveAll(dir); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, dir); err != nil {
		t.Fatal(err)
	}
	trash, _ := e.svc.ListTrash(e.ctx)
	res, err = e.svc.Restore(e.ctx, e.user, []string{trash[0].ID})
	if err != nil || len(res.Errors) != 1 {
		t.Fatalf("restore through symlink: %+v %v", res, err)
	}
	if entries, _ := os.ReadDir(outside); len(entries) != 0 {
		t.Fatalf("files written outside the library: %v", entries)
	}
}
