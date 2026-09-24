package manage

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"rainy/internal/store"
)

func TestDeleteRestoreRoundTripKeepsID(t *testing.T) {
	e := newEnv(t)
	rel := "林雨晴/城市夜雨 (2019)/02 霓虹.flac"
	tr := e.track(rel)
	if err := e.st.SetStarred(e.ctx, e.user.ID, "track", []string{tr.ID}, true); err != nil {
		t.Fatal(err)
	}

	res, err := e.svc.Delete(e.ctx, e.user, []string{tr.ID})
	if err != nil || len(res.Errors) != 0 || len(res.Updated) != 1 || !res.Updated[0].Missing {
		t.Fatalf("delete: %+v %v", res, err)
	}
	if e.exists(rel) || e.exists("林雨晴/城市夜雨 (2019)/02 霓虹.lrc") {
		t.Fatal("file or sidecar still in the library")
	}
	trash, err := e.svc.ListTrash(e.ctx)
	if err != nil || len(trash) != 1 {
		t.Fatalf("trash %+v %v", trash, err)
	}
	entry := trash[0]
	if entry.TrackID != tr.ID || entry.OriginalPath != rel || entry.Title != tr.Title || entry.DeletedBy != "editor" || entry.Size == 0 {
		t.Fatalf("entry %+v", entry)
	}
	trashed := filepath.Join(e.cfg.TrashDir(), filepath.FromSlash(entry.TrashPath))
	if !exists(trashed) || !exists(stem(trashed)+".lrc") {
		t.Fatalf("trashed file missing at %s", trashed)
	}
	if got, _ := e.st.GetTrack(e.ctx, tr.ID, ""); got == nil || !got.Missing {
		t.Fatal("track row must be kept and marked missing")
	}
	// Missing-track purge must not touch tracks whose file is in the trash.
	if n, err := e.svc.PurgeMissing(e.ctx, e.user, nil); err != nil || n != 0 {
		t.Fatalf("purge missing: %d %v", n, err)
	}
	// The doctor does not report trashed files as missing.
	sum, err := e.svc.IssueSummary(e.ctx)
	if err != nil || sum[IssueMissingFiles] != 0 {
		t.Fatalf("summary %v %v", sum, err)
	}

	res, err = e.svc.Restore(e.ctx, e.user, []string{entry.ID})
	if err != nil || len(res.Errors) != 0 {
		t.Fatalf("restore: %+v %v", res, err)
	}
	if !e.exists(rel) || !e.exists("林雨晴/城市夜雨 (2019)/02 霓虹.lrc") {
		t.Fatal("file not restored")
	}
	got := e.track(rel)
	if got.ID != tr.ID || got.Missing {
		t.Fatalf("restored track %+v", got)
	}
	if st, _ := e.st.GetTrack(e.ctx, tr.ID, e.user.ID); st == nil || !st.Starred {
		t.Fatal("annotations lost")
	}
	if trash, _ := e.svc.ListTrash(e.ctx); len(trash) != 0 {
		t.Fatal("trash entry not removed")
	}
	if exists(filepath.Join(e.cfg.TrashDir(), "1")) {
		t.Error("empty trash folders must be removed")
	}
	acts := e.logActions(tr.ID)
	if len(acts) != 2 || acts[0] != "restore" || acts[1] != "delete" {
		t.Fatalf("log %v", acts)
	}
}

func TestDeleteUniqueTrashNamesAndRestoreConflict(t *testing.T) {
	e := newEnv(t)
	rel := "Static Noise/Unknown Signals/01 Signal 1.ogg"
	tr := e.track(rel)
	orig, _ := os.ReadFile(e.abs(rel))
	if _, err := e.svc.Delete(e.ctx, e.user, []string{tr.ID}); err != nil {
		t.Fatal(err)
	}
	trash, _ := e.svc.ListTrash(e.ctx)
	if len(trash) != 1 {
		t.Fatalf("trash %+v", trash)
	}
	first := trash[0].ID
	// A new file appears at the same path and is deleted too.
	if err := os.WriteFile(e.abs(rel), []byte("second"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := e.sc.RescanFiles(e.ctx, e.lib.ID, []string{rel}); err != nil {
		t.Fatal(err)
	}
	if res, err := e.svc.Delete(e.ctx, e.user, []string{e.track(rel).ID}); err != nil || len(res.Errors) != 0 {
		t.Fatalf("%+v %v", res, err)
	}
	trash, _ = e.svc.ListTrash(e.ctx)
	if len(trash) != 2 || trash[0].TrashPath == trash[1].TrashPath {
		t.Fatalf("trash names must be unique: %+v", trash)
	}
	// Occupy the original path: restoring is a conflict and keeps the trash entry.
	if err := os.WriteFile(e.abs(rel), []byte("third"), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err := e.svc.Restore(e.ctx, e.user, []string{first})
	if err != nil || len(res.Errors) != 1 {
		t.Fatalf("expected conflict: %+v %v", res, err)
	}
	if b, _ := os.ReadFile(e.abs(rel)); string(b) != "third" {
		t.Fatal("restore overwrote a file")
	}
	if err := os.Remove(e.abs(rel)); err != nil {
		t.Fatal(err)
	}
	if res, err := e.svc.Restore(e.ctx, e.user, []string{first}); err != nil || len(res.Errors) != 0 {
		t.Fatalf("%+v %v", res, err)
	}
	if b, _ := os.ReadFile(e.abs(rel)); string(b) != string(orig) {
		t.Fatal("wrong file restored")
	}
}

func TestPurgeTrashAndMissing(t *testing.T) {
	e := newEnv(t)
	a := e.track("Static Noise/Unknown Signals/01 Signal 1.ogg")
	b := e.track("Static Noise/Unknown Signals/02 Signal 2.ogg")
	c := e.track("Static Noise/Unknown Signals/03 Interference.ogg")
	if _, err := e.svc.Delete(e.ctx, e.user, []string{a.ID, b.ID}); err != nil {
		t.Fatal(err)
	}
	trash, _ := e.svc.ListTrash(e.ctx)
	var aEntry string
	for _, en := range trash {
		if en.TrackID == a.ID {
			aEntry = en.ID
		}
	}
	n, err := e.svc.PurgeTrash(e.ctx, e.user, []string{aEntry})
	if err != nil || n != 1 {
		t.Fatalf("purge %d %v", n, err)
	}
	if _, err := e.st.GetTrack(e.ctx, a.ID, ""); !errors.Is(err, store.ErrNotFound) {
		t.Fatal("purged track row must be deleted")
	}
	if _, err := e.st.GetTrack(e.ctx, b.ID, ""); err != nil {
		t.Fatal("other trashed track must be kept")
	}
	// Empty the whole trash.
	if n, err := e.svc.PurgeTrash(e.ctx, e.user, nil); err != nil || n != 1 {
		t.Fatalf("empty trash %d %v", n, err)
	}
	if exists(filepath.Join(e.cfg.TrashDir(), "1")) {
		t.Error("trash dirs left behind")
	}

	// A track whose file vanished outside Rainy.
	if err := os.Remove(e.abs(c.Path)); err != nil {
		t.Fatal(err)
	}
	if err := e.st.MarkTracksMissing(e.ctx, []string{c.ID}, true); err != nil {
		t.Fatal(err)
	}
	sum, _ := e.svc.IssueSummary(e.ctx)
	if sum[IssueMissingFiles] != 1 {
		t.Fatalf("summary %v", sum)
	}
	if n, err := e.svc.PurgeMissing(e.ctx, e.user, []string{c.ID}); err != nil || n != 1 {
		t.Fatalf("purge missing %d %v", n, err)
	}
	if _, err := e.st.GetTrack(e.ctx, c.ID, ""); !errors.Is(err, store.ErrNotFound) {
		t.Fatal("missing track not purged")
	}
}
