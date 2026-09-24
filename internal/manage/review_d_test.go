package manage

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"rainy/internal/model"
)

// addFile writes a file into the test library and registers it through the (fake)
// scanner when it is audio.
func (e *env) addFile(rel, content string) *model.Track {
	e.t.Helper()
	p := e.abs(rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		e.t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		e.t.Fatal(err)
	}
	if strings.HasSuffix(rel, ".lrc") || strings.HasSuffix(rel, ".jpg") || strings.HasSuffix(rel, ".png") {
		return nil
	}
	if _, err := e.sc.RescanFiles(e.ctx, e.lib.ID, []string{rel}); err != nil {
		e.t.Fatal(err)
	}
	return e.track(rel)
}

func (e *env) trashEntryFor(trackID string) *model.TrashEntry {
	e.t.Helper()
	all, err := e.svc.ListTrash(e.ctx)
	if err != nil {
		e.t.Fatal(err)
	}
	for i := range all {
		if all[i].TrackID == trackID {
			return &all[i]
		}
	}
	e.t.Fatalf("no trash entry for %s", trackID)
	return nil
}

// A purge must work on the trash as it is once the lock is held: an entry restored
// meanwhile (whose trash path was then reused by another deletion) must not take the
// other entry's file with it.
func TestPurgeTrashRereadsEntriesUnderLock(t *testing.T) {
	e := newEnv(t)
	rel := "Static Noise/Unknown Signals/01 Signal 1.ogg"
	a := e.track(rel)
	if _, err := e.svc.Delete(e.ctx, e.user, []string{a.ID}); err != nil {
		t.Fatal(err)
	}
	entryA := e.trashEntryFor(a.ID)
	trashFile := filepath.Join(e.cfg.TrashDir(), filepath.FromSlash(entryA.TrashPath))

	unlock := e.sc.LockLibrary() // a long operation holds the library
	done := make(chan int, 1)
	go func() {
		n, _ := e.svc.PurgeTrash(e.ctx, e.user, []string{entryA.ID})
		done <- n
	}()
	time.Sleep(200 * time.Millisecond) // let the purge load its entries and block

	// Meanwhile (as the lock holder): A is restored, then another file with the same
	// original path is deleted and lands at the same trash path.
	if err := os.Rename(trashFile, e.abs(rel)); err != nil {
		t.Fatal(err)
	}
	if err := e.st.DeleteTrash(e.ctx, entryA.ID); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(trashFile, []byte("other file"), 0o644); err != nil {
		t.Fatal(err)
	}
	entryB := &model.TrashEntry{LibraryID: e.lib.ID, OriginalPath: rel, TrashPath: entryA.TrashPath, Title: "B"}
	if err := e.st.AddTrash(e.ctx, entryB); err != nil {
		t.Fatal(err)
	}
	unlock()

	if n := <-done; n != 0 {
		t.Fatalf("purged %d entries, want 0", n)
	}
	if !exists(trashFile) {
		t.Fatal("the file of another trash entry was purged")
	}
}

// Two audio files with the same stem ("Song.mp3", "Song.flac") must never share a lyrics
// sidecar in the trash: purging one must not delete the other's lyrics and a restore
// must bring back the right one.
func TestTrashKeepsSidecarsPaired(t *testing.T) {
	e := newEnv(t)
	mp3 := e.addFile("X/Y/Song.mp3", "mp3 audio")
	if _, err := e.svc.Delete(e.ctx, e.user, []string{mp3.ID}); err != nil {
		t.Fatal(err)
	}
	flac := e.addFile("X/Y/Song.flac", "flac audio")
	e.addFile("X/Y/Song.lrc", "[00:01.00]flac lyrics")
	if _, err := e.svc.Delete(e.ctx, e.user, []string{flac.ID}); err != nil {
		t.Fatal(err)
	}
	if n, err := e.svc.PurgeTrash(e.ctx, e.user, []string{e.trashEntryFor(mp3.ID).ID}); err != nil || n != 1 {
		t.Fatalf("purge: %d %v", n, err)
	}
	if res, err := e.svc.Restore(e.ctx, e.user, []string{e.trashEntryFor(flac.ID).ID}); err != nil || len(res.Errors) != 0 {
		t.Fatalf("restore: %+v %v", res, err)
	}
	if b, err := os.ReadFile(e.abs("X/Y/Song.lrc")); err != nil || string(b) != "[00:01.00]flac lyrics" {
		t.Fatalf("lyrics of the restored file lost: %q %v", b, err)
	}
}

// A sidecar shared by several audio files with the same stem stays with the files that
// remain: deleting or renaming one of them must not take the lyrics away from the others.
func TestSharedSidecarStaysWithRemainingFile(t *testing.T) {
	e := newEnv(t)
	mp3 := e.addFile("X/Y/Song.mp3", "mp3 audio")
	e.addFile("X/Y/Song.flac", "flac audio")
	e.addFile("X/Y/Song.lrc", "lyrics")
	if _, err := e.svc.Delete(e.ctx, e.user, []string{mp3.ID}); err != nil {
		t.Fatal(err)
	}
	if !e.exists("X/Y/Song.lrc") {
		t.Fatal("deleting Song.mp3 took the lyrics of Song.flac to the trash")
	}

	e.addFile("X/Y/Other.mp3", "mp3 audio")
	other := e.track("X/Y/Other.mp3")
	e.addFile("X/Y/Other.ogg", "ogg audio")
	e.addFile("X/Y/Other.lrc", "other lyrics")
	res, err := e.svc.Rename(e.ctx, e.user, []string{other.ID}, "Z/{title}")
	if err != nil || len(res.Errors) != 0 {
		t.Fatalf("rename: %+v %v", res, err)
	}
	if !e.exists("X/Y/Other.lrc") {
		t.Fatal("renaming Other.mp3 took the lyrics of Other.ogg")
	}
}

// Saving a folder image never destroys the images it replaces: they go to the trash
// (restorable, logged).
func TestSaveFolderImageTrashesReplacedImages(t *testing.T) {
	e := newEnv(t)
	dir := "Static Noise/Unknown Signals"
	tr := e.track(dir + "/01 Signal 1.ogg")
	e.addFile(dir+"/cover.png", "old png cover")
	e.addFile(dir+"/Cover.jpg", "old jpg cover")
	img := pngImage(t, 20, 20)
	res, err := e.svc.SetCover(e.ctx, e.user, CoverRequest{TrackIDs: []string{tr.ID}, SaveToFolder: true, Image: img})
	if err != nil || len(res.Errors) != 0 {
		t.Fatalf("set cover: %+v %v", res, err)
	}
	if b := mustRead(t, e.abs(dir+"/cover.png")); string(b) != string(img) {
		t.Fatal("new cover not written")
	}
	trash, _ := e.svc.ListTrash(e.ctx)
	var saved []string
	for _, en := range trash {
		b, err := os.ReadFile(filepath.Join(e.cfg.TrashDir(), filepath.FromSlash(en.TrashPath)))
		if err != nil {
			t.Fatal(err)
		}
		saved = append(saved, string(b))
	}
	slices.Sort(saved)
	if !slices.Equal(saved, []string{"old jpg cover", "old png cover"}) {
		t.Fatalf("replaced images not kept in the trash: %q", saved)
	}
	entries, _, _ := e.st.ListEditLog(e.ctx, "", 0, 0)
	deletes := 0
	for _, en := range entries {
		if en.Action == "delete" {
			deletes++
		}
	}
	if deletes != 2 {
		t.Fatalf("replaced images not logged: %d delete entries", deletes)
	}
}

// Renames hand both the old and the new paths to the scanner.
func TestRenameRescansOldAndNewPaths(t *testing.T) {
	e := newEnv(t)
	rel := "Static Noise/Unknown Signals/01 Signal 1.ogg"
	tr := e.track(rel)
	e.sc.calls = nil
	res, err := e.svc.Rename(e.ctx, e.user, []string{tr.ID}, "Moved/{title}")
	if err != nil || len(res.Updated) != 1 {
		t.Fatalf("rename: %+v %v", res, err)
	}
	var all []string
	for _, c := range e.sc.calls {
		all = append(all, c...)
	}
	if !slices.Contains(all, rel) || !slices.Contains(all, res.Updated[0].Path) {
		t.Fatalf("rescanned %v, want old %q and new %q", all, rel, res.Updated[0].Path)
	}
}
