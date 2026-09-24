package manage

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"rainy/internal/model"
	"rainy/internal/store"
)

func planStatus(plans []RenamePlan) map[string]RenamePlan {
	out := map[string]RenamePlan{}
	for _, p := range plans {
		out[p.TrackID] = p
	}
	return out
}

func TestPreviewRename(t *testing.T) {
	e := newEnv(t)
	t1 := e.track("The Rainy Days/Blue Skies/01 Blue Skies.opus")
	t2 := e.track("The Rainy Days/Blue Skies/02 Umbrella.opus")
	missing := e.track("Static Noise/Unknown Signals/04 Dead Air.ogg")
	if err := e.st.MarkTracksMissing(e.ctx, []string{missing.ID}, true); err != nil {
		t.Fatal(err)
	}
	// An existing file that differs only in case from t1's target.
	if err := os.WriteFile(e.abs("The Rainy Days/Blue Skies/blue skies.opus"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	plans, err := e.svc.PreviewRename(e.ctx, []string{t1.ID, t2.ID, missing.ID, "nope"}, "{artist}/{album}/{title}")
	if err != nil {
		t.Fatal(err)
	}
	got := planStatus(plans)
	if p := got[t1.ID]; p.Status != PlanConflict || p.To != "The Rainy Days/Blue Skies/Blue Skies.opus" {
		t.Errorf("t1 %+v", p)
	}
	if p := got[t2.ID]; p.Status != PlanOK || p.From != t2.Path || p.To != "The Rainy Days/Blue Skies/Umbrella.opus" {
		t.Errorf("t2 %+v", p)
	}
	if p := got[missing.ID]; p.Status != PlanInvalid {
		t.Errorf("missing %+v", p)
	}
	if p := got["nope"]; p.Status != PlanInvalid {
		t.Errorf("unknown %+v", p)
	}
	if !e.exists(t2.Path) || e.exists("The Rainy Days/Blue Skies/Umbrella.opus") {
		t.Fatal("preview must not touch files")
	}

	// Unchanged + collisions inside the batch.
	plans, err = e.svc.PreviewRename(e.ctx, []string{t1.ID}, "{albumartist}/{album}/[{disc}-]{track:2} {title}")
	if err != nil || plans[0].Status != PlanUnchanged {
		t.Fatalf("unchanged: %+v %v", plans, err)
	}
	plans, err = e.svc.PreviewRename(e.ctx, []string{t1.ID, t2.ID}, "{artist}/{album}")
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range plans {
		if p.Status != PlanConflict {
			t.Errorf("batch collision: %+v", p)
		}
	}

	if _, err := e.svc.PreviewRename(e.ctx, []string{t1.ID}, "{bogus}"); err == nil {
		t.Fatal("invalid pattern accepted")
	}
}

func TestRenameMovesFilesImagesAndKeepsIDs(t *testing.T) {
	e := newEnv(t)
	t1 := e.track("The Rainy Days/Blue Skies/01 Blue Skies.opus")
	album, _, err := e.st.ListTracks(e.ctx, store.TrackQuery{DirPrefix: "The Rainy Days"})
	if err != nil || len(album) != 4 {
		t.Fatalf("fixture: %d tracks, %v", len(album), err)
	}
	if err := os.WriteFile(e.abs("The Rainy Days/Blue Skies/.DS_Store"), []byte("junk"), 0o644); err != nil {
		t.Fatal(err)
	}

	res, err := e.svc.Rename(e.ctx, e.user, trackIDsOf(album), "Moved/{album}/{track:2} - {title}")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Errors) != 0 || len(res.Updated) != 4 {
		t.Fatalf("result %+v", res)
	}
	for _, want := range []string{"Moved/Blue Skies/01 - Blue Skies.opus", "Moved/Blue Skies/02 - Umbrella.opus", "Moved/Blue Skies/folder.png"} {
		if !e.exists(want) {
			t.Errorf("%s not created", want)
		}
	}
	if e.exists("The Rainy Days") {
		t.Error("emptied directories must be removed")
	}
	if _, err := os.Stat(e.lib.Path); err != nil {
		t.Fatal("library root removed")
	}
	if got := e.track("Moved/Blue Skies/01 - Blue Skies.opus"); got.ID != t1.ID {
		t.Errorf("id changed: %s != %s", got.ID, t1.ID)
	}
	if acts := e.logActions(t1.ID); !slices.Equal(acts, []string{"rename"}) {
		t.Errorf("log %v", acts)
	}
	if len(e.sc.calls) != 1 || len(e.sc.calls[0]) != 8 { // old + new paths
		t.Errorf("rescan calls %v", e.sc.calls)
	}
}

func TestRenamePartialDirKeepsImagesAndMovesSidecar(t *testing.T) {
	e := newEnv(t)
	dir := "林雨晴/城市夜雨 (2019)"
	t1 := e.track(dir + "/01 城市夜雨.flac")
	res, err := e.svc.Rename(e.ctx, e.user, []string{t1.ID}, "{albumartist}/Singles/{title}")
	if err != nil || len(res.Errors) != 0 {
		t.Fatalf("%+v %v", res, err)
	}
	if !e.exists("林雨晴/Singles/城市夜雨.flac") || !e.exists("林雨晴/Singles/城市夜雨.lrc") {
		t.Fatal("track or sidecar not moved")
	}
	if e.exists(dir+"/01 城市夜雨.lrc") || !e.exists(dir+"/cover.jpg") || e.exists("林雨晴/Singles/cover.jpg") {
		t.Fatal("images must stay when not every audio file of the folder moves")
	}
}

func TestRenameCaseOnly(t *testing.T) {
	e := newEnv(t)
	tr := e.track("Static Noise/Unknown Signals/01 Signal 1.ogg")
	tr.Title = "SIGNAL 1"
	if err := e.st.UpsertTracks(e.ctx, []model.Track{*tr}); err != nil {
		t.Fatal(err)
	}
	res, err := e.svc.Rename(e.ctx, e.user, []string{tr.ID}, "{artist}/{album}/{track:2} {title}")
	if err != nil || len(res.Errors) != 0 || len(res.Updated) != 1 {
		t.Fatalf("%+v %v", res, err)
	}
	entries, _ := os.ReadDir(e.abs("Static Noise/Unknown Signals"))
	found := false
	for _, en := range entries {
		if en.Name() == "01 SIGNAL 1.ogg" {
			found = true
		}
		if en.Name() == "01 Signal 1.ogg" {
			t.Fatal("old spelling still present")
		}
	}
	if !found || res.Updated[0].Path != "Static Noise/Unknown Signals/01 SIGNAL 1.ogg" {
		t.Fatalf("case-only rename not applied: %+v", res.Updated[0].Path)
	}
}

func TestRenameConflictReported(t *testing.T) {
	e := newEnv(t)
	t1 := e.track("The Rainy Days/Blue Skies/01 Blue Skies.opus")
	if err := os.MkdirAll(e.abs("X"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(e.abs("X"), "BLUE SKIES.opus"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err := e.svc.Rename(e.ctx, e.user, []string{t1.ID}, "X/{title}")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Errors) != 1 || len(res.Updated) != 0 || !e.exists(t1.Path) {
		t.Fatalf("conflict must not move the file: %+v", res)
	}
}

// Folder images only follow the audio of a directory that is a leaf album folder: never
// images of the library root, nor of a folder whose sub-folders still hold audio (an
// artist folder with artist.jpg and loose tracks next to its album folders).
func TestRenameKeepsRootAndArtistFolderImages(t *testing.T) {
	e := newEnv(t)
	loose := e.addFile("Solo Artist/loose.mp3", "loose")
	e.addFile("Solo Artist/artist.jpg", "img")
	e.addFile("Solo Artist/Album/01 song.mp3", "song")
	root := e.addFile("root track.mp3", "root")
	e.addFile("root.jpg", "img")

	res, err := e.svc.Rename(e.ctx, e.user, []string{loose.ID, root.ID}, "Elsewhere/{title}")
	if err != nil || len(res.Errors) != 0 || len(res.Updated) != 2 {
		t.Fatalf("%+v %v", res, err)
	}
	if !e.exists("Solo Artist/artist.jpg") || e.exists("Elsewhere/artist.jpg") {
		t.Error("artist.jpg must stay in a folder whose sub-folders still hold audio")
	}
	if !e.exists("root.jpg") || e.exists("Elsewhere/root.jpg") {
		t.Error("images of the library root must never move")
	}

	// A leaf folder emptied of audio still takes its images along.
	leaf := e.track("Solo Artist/Album/01 song.mp3")
	e.addFile("Solo Artist/Album/cover.jpg", "img")
	if res, err := e.svc.Rename(e.ctx, e.user, []string{leaf.ID}, "Moved Album/{title}"); err != nil || len(res.Errors) != 0 {
		t.Fatalf("%+v %v", res, err)
	}
	if !e.exists("Moved Album/cover.jpg") || e.exists("Solo Artist/Album/cover.jpg") {
		t.Error("leaf album folder images should follow their audio")
	}
	if !e.exists("Solo Artist/artist.jpg") {
		t.Error("artist.jpg must stay")
	}
}
