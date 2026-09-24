package scanner

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"rainy/internal/config"
	"rainy/internal/db/dbtest"
	"rainy/internal/events"
	"rainy/internal/model"
	"rainy/internal/store"
	"rainy/internal/tags"
)

// visibleTracks is the number of audio files in testdata/music outside ignored folders.
const visibleTracks = 39

type env struct {
	t    *testing.T
	root string
	st   *store.Store
	bus  *events.Bus
	sc   *Scanner
	lib  *model.Library
	ctx  context.Context
}

// copyDir copies src into dst recursively.
func copyDir(t *testing.T, src, dst string) {
	t.Helper()
	err := filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		in, err := os.Open(p)
		if err != nil {
			return err
		}
		defer func() { _ = in.Close() }()
		out, err := os.Create(target)
		if err != nil {
			return err
		}
		if _, err := io.Copy(out, in); err != nil {
			_ = out.Close()
			return err
		}
		if err := out.Close(); err != nil {
			return err
		}
		fi, err := d.Info()
		if err != nil {
			return err
		}
		return os.Chtimes(target, fi.ModTime(), fi.ModTime())
	})
	if err != nil {
		t.Fatalf("copying test library: %v", err)
	}
}

// newEnv copies testdata/music into a temp dir and returns a scanner over it.
func newEnv(t *testing.T) *env {
	t.Helper()
	src := filepath.Join("..", "..", "testdata", "music")
	if _, err := os.Stat(src); err != nil {
		t.Skip("testdata/music missing (run: bash scripts/gen-testdata.sh)")
	}
	root := filepath.Join(t.TempDir(), "music")
	copyDir(t, src, root)
	return newEnvAt(t, root)
}

func newEnvAt(t *testing.T, root string) *env {
	t.Helper()
	d := dbtest.New(t)
	st := store.New(d)
	bus := events.NewBus()
	cfg := config.Default()
	cfg.ScanInterval = 0
	ctx := context.Background()
	lib := &model.Library{Name: "music", Path: root}
	if err := st.CreateLibrary(ctx, lib); err != nil {
		t.Fatal(err)
	}
	return &env{t: t, root: root, st: st, bus: bus, sc: New(st, bus, cfg), lib: lib, ctx: ctx}
}

func (e *env) run(full bool) Status {
	e.t.Helper()
	if err := e.sc.Run(e.ctx, Options{Full: full}); err != nil {
		e.t.Fatalf("Run(full=%v): %v", full, err)
	}
	return e.sc.Status()
}

func (e *env) tracks(missing string) []model.Track {
	e.t.Helper()
	ts, _, err := e.st.ListTracks(e.ctx, store.TrackQuery{Missing: missing, Sort: "path"})
	if err != nil {
		e.t.Fatal(err)
	}
	return ts
}

func (e *env) byPath(rel string) *model.Track {
	e.t.Helper()
	tr, err := e.st.GetTrackByPath(e.ctx, e.lib.ID, rel)
	if err != nil {
		e.t.Fatalf("GetTrackByPath(%q): %v", rel, err)
	}
	return tr
}

func (e *env) album(name string) *model.Album {
	e.t.Helper()
	as, _, err := e.st.ListAlbums(e.ctx, store.AlbumQuery{Q: name})
	if err != nil {
		e.t.Fatal(err)
	}
	for i := range as {
		if as[i].Name == name {
			return &as[i]
		}
	}
	e.t.Fatalf("album %q not found", name)
	return nil
}

func (e *env) abs(rel string) string { return filepath.Join(e.root, filepath.FromSlash(rel)) }

func counts(st Status) [4]int { return [4]int{st.Added, st.Updated, st.Removed, st.Moved} }

func TestScanTestLibrary(t *testing.T) {
	e := newEnv(t)
	sub, cancel := e.bus.Subscribe()
	defer cancel()

	st := e.run(false)
	if st.Phase != PhaseDone || st.Scanning || st.Added != visibleTracks || st.FilesSeen != visibleTracks || st.Errors != 0 {
		t.Fatalf("status after first scan: %+v", st)
	}
	ts := e.tracks("")
	if len(ts) != visibleTracks {
		t.Fatalf("got %d tracks, want %d", len(ts), visibleTracks)
	}
	for _, tr := range ts {
		if strings.Contains(tr.Path, "@eaDir") || strings.Contains(tr.Path, ".hidden") {
			t.Errorf("ignored folder scanned: %s", tr.Path)
		}
		if tr.CreatedAt <= 0 || tr.CreatedAt > time.Now().UnixMilli() || tr.CreatedAt > tr.Mtime && tr.Mtime > 0 {
			t.Errorf("%s: created_at %d (mtime %d)", tr.Path, tr.CreatedAt, tr.Mtime)
		}
	}

	// CJK metadata, embedded cover, "N/M" numbers.
	tr := e.byPath("林雨晴/夏日微风 (2021)/01 夏日微风.mp3")
	if tr.Title != "夏日微风" || tr.Artist != "林雨晴" || tr.Album != "夏日微风" || tr.Year != 2021 ||
		tr.TrackNumber != 1 || tr.TrackTotal != 6 || !tr.HasCover || tr.Genre != "Mandopop" || tr.Codec != "mp3" || tr.Duration <= 0 {
		t.Errorf("CJK mp3: %+v", tr)
	}
	// Japanese m4a + genre split on "/".
	tr = e.byPath("星野ミライ/雨上がりの空/01 雨上がりの空.m4a")
	if tr.Title != "雨上がりの空" || !reflect.DeepEqual(tr.Genres, []string{"J-Pop", "Anime"}) || !tr.HasLrc || tr.Codec != "mp4/aac" {
		t.Errorf("Japanese m4a: title=%q genres=%v lrc=%v codec=%q", tr.Title, tr.Genres, tr.HasLrc, tr.Codec)
	}
	// Sidecar and embedded lyrics.
	if tr := e.byPath("林雨晴/城市夜雨 (2019)/01 城市夜雨.flac"); !tr.HasLrc || !reflect.DeepEqual(tr.Genres, []string{"Pop", "Rock"}) || tr.Year != 2019 {
		t.Errorf("flac with lrc: lrc=%v genres=%v year=%d", tr.HasLrc, tr.Genres, tr.Year)
	}
	if tr := e.byPath("林雨晴/城市夜雨 (2019)/03 末班车.flac"); tr.HasLrc || !strings.Contains(tr.Lyrics, "窗外的灯") || !tr.HasLyrics {
		t.Errorf("embedded lyrics: lrc=%v lyrics=%q", tr.HasLrc, tr.Lyrics)
	}
	if tr := e.byPath("林雨晴/城市夜雨 (2019)/04 伞下.flac"); tr.HasLrc || tr.HasLyrics {
		t.Errorf("no lyrics expected: %+v", tr)
	}
	// Untagged fallbacks.
	tr = e.byPath("Unsorted/untagged demo.mp3")
	if tr.Title != "untagged demo" || tr.Artist != UnknownArtist || tr.Album != UnknownAlbum || tr.AlbumArtist != UnknownArtist {
		t.Errorf("untagged: %+v", tr)
	}
	// Multi-disc album with embedded PNG.
	al := e.album("Aurora Lights")
	if al.DiscCount != 2 || al.SongCount != 6 || al.Year != 2020 || al.CoverTrackID == "" || al.CoverPath != "" {
		t.Errorf("multi-disc album: %+v", al)
	}
	if tr := e.byPath("Northern Echo/Aurora Lights/CD2/2-01 Solar Wind.flac"); tr.DiscNumber != 2 || tr.DiscTotal != 2 ||
		tr.DiscSubtitle != "Dawn" || tr.TrackNumber != 1 || tr.TrackTotal != 3 || tr.OriginalYear != 2018 {
		t.Errorf("disc 2 track: %+v", tr)
	}
	// Compilation.
	al = e.album("Summer Hits 2024")
	if !al.Compilation || al.AlbumArtist != "Various Artists" || al.SongCount != 5 {
		t.Errorf("compilation: %+v", al)
	}
	appears, err := e.st.AlbumsAppearsOn(e.ctx, e.byPath("Various Artists/Summer Hits 2024/03 Heatwave.mp3").ArtistID, "")
	if err != nil || len(appears) != 1 || appears[0].ID != al.ID {
		t.Errorf("appears on: %v %v", appears, err)
	}
	// Folder covers, artist image, album without art.
	if al := e.album("城市夜雨"); !strings.HasSuffix(filepath.ToSlash(al.CoverPath), "城市夜雨 (2019)/cover.jpg") {
		t.Errorf("cover.jpg not detected: %q", al.CoverPath)
	}
	if al := e.album("Blue Skies"); !strings.HasSuffix(al.CoverPath, "folder.png") || al.CoverTrackID != "" {
		t.Errorf("folder.png not detected: %+v", al)
	}
	if al := e.album("Unknown Signals"); al.CoverPath != "" || al.CoverTrackID != "" {
		t.Errorf("album without art: %+v", al)
	}
	ar, err := e.st.GetArtist(e.ctx, e.byPath("林雨晴/夏日微风 (2021)/01 夏日微风.mp3").AlbumArtistID, "")
	if err != nil || !strings.HasSuffix(ar.ImagePath, "artist.jpg") || ar.AlbumCount != 2 {
		t.Errorf("artist image: %+v %v", ar, err)
	}
	libs, _ := e.st.ListLibraries(e.ctx)
	if libs[0].LastScanAt == 0 {
		t.Error("library not marked scanned")
	}
	genres, _ := e.st.ListGenres(e.ctx)
	if len(genres) != 9 {
		t.Errorf("genres: %v", genres)
	}

	// Events: scan progress ending with "done", then a library event.
	var sawDone, sawLibrary bool
	for len(sub) > 0 {
		ev := <-sub
		switch ev.Type {
		case events.TypeScan:
			if ev.Data.(Status).Phase == PhaseDone {
				sawDone = true
			}
		case events.TypeLibrary:
			sawLibrary = true
		}
	}
	if !sawDone || !sawLibrary {
		t.Errorf("events: done=%v library=%v", sawDone, sawLibrary)
	}
}

func TestRescanIsNoop(t *testing.T) {
	e := newEnv(t)
	e.run(false)
	before := e.tracks("")
	albumsBefore, _, _ := e.st.ListAlbums(e.ctx, store.AlbumQuery{Sort: "name"})

	for _, full := range []bool{false, true} {
		st := e.run(full)
		if counts(st) != [4]int{} || st.Errors != 0 {
			t.Fatalf("rescan (full=%v) changed things: %+v", full, st)
		}
	}
	after := e.tracks("")
	for i := range before {
		if before[i].ID != after[i].ID || before[i].UpdatedAt != after[i].UpdatedAt {
			t.Fatalf("track %s changed: %d → %d", before[i].Path, before[i].UpdatedAt, after[i].UpdatedAt)
		}
	}
	albumsAfter, _, _ := e.st.ListAlbums(e.ctx, store.AlbumQuery{Sort: "name"})
	for i := range albumsBefore {
		if albumsBefore[i].UpdatedAt != albumsAfter[i].UpdatedAt {
			t.Fatalf("album %s updated_at changed", albumsBefore[i].Name)
		}
	}
}

func TestTouchMoveDeleteRestore(t *testing.T) {
	e := newEnv(t)
	e.run(false)
	ctx := e.ctx

	// Touching a file re-reads it.
	rel := "Static Noise/Unknown Signals/02 Signal 2.ogg"
	orig := e.byPath(rel)
	later := time.Now().Add(time.Minute)
	if err := os.Chtimes(e.abs(rel), later, later); err != nil {
		t.Fatal(err)
	}
	if st := e.run(false); counts(st) != [4]int{0, 1, 0, 0} {
		t.Fatalf("touch: %+v", st)
	}
	if tr := e.byPath(rel); tr.ID != orig.ID || tr.Mtime == orig.Mtime || tr.CreatedAt != orig.CreatedAt {
		t.Fatalf("touched track: %+v", tr)
	}

	// Moving a file keeps its id (and annotations).
	u := &model.User{Username: "u", PasswordEnc: "synthetic"}
	if err := e.st.CreateUser(ctx, u); err != nil {
		t.Fatal(err)
	}
	if err := e.st.SetStarred(ctx, u.ID, "track", []string{orig.ID}, true); err != nil {
		t.Fatal(err)
	}
	newRel := "Static Noise/Moved Here/renamed.ogg"
	if err := os.MkdirAll(filepath.Dir(e.abs(newRel)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(e.abs(rel), e.abs(newRel)); err != nil {
		t.Fatal(err)
	}
	if st := e.run(false); counts(st) != [4]int{0, 0, 0, 1} {
		t.Fatalf("move: %+v", st)
	}
	moved, err := e.st.GetTrack(ctx, orig.ID, u.ID)
	if err != nil || moved.Path != newRel || moved.Missing || !moved.Starred || moved.CreatedAt != orig.CreatedAt {
		t.Fatalf("moved track: %+v %v", moved, err)
	}
	if n := len(e.tracks("")); n != visibleTracks {
		t.Fatalf("after move: %d tracks", n)
	}

	// Deleting marks missing; restoring brings the same id back.
	del := "Nature Sounds/Field Recordings/01 Forest Rain.wav"
	delTrack := e.byPath(del)
	tmp := filepath.Join(t.TempDir(), "keep.wav")
	if err := os.Rename(e.abs(del), tmp); err != nil {
		t.Fatal(err)
	}
	if st := e.run(false); counts(st) != [4]int{0, 0, 1, 0} {
		t.Fatalf("delete: %+v", st)
	}
	if tr := e.byPath(del); !tr.Missing {
		t.Fatal("deleted track not missing")
	}
	if n := len(e.tracks("")); n != visibleTracks-1 {
		t.Fatalf("after delete: %d tracks", n)
	}
	if err := os.Rename(tmp, e.abs(del)); err != nil {
		t.Fatal(err)
	}
	if st := e.run(false); counts(st) != [4]int{0, 1, 0, 0} {
		t.Fatalf("restore: %+v", st)
	}
	if tr := e.byPath(del); tr.Missing || tr.ID != delTrack.ID {
		t.Fatalf("restored track: %+v", tr)
	}

	// Swapping two files keeps one row per path.
	a, b := "Nature Sounds/Field Recordings/02 Mountain Stream.wav", "Nature Sounds/Field Recordings/03 Night Crickets.wav"
	tmp2 := e.abs(a) + ".tmp"
	for _, mv := range [][2]string{{e.abs(a), tmp2}, {e.abs(b), e.abs(a)}, {tmp2, e.abs(b)}} {
		if err := os.Rename(mv[0], mv[1]); err != nil {
			t.Fatal(err)
		}
	}
	e.run(false)
	if ta, tb := e.byPath(a), e.byPath(b); ta.Title != "Night Crickets" || tb.Title != "Mountain Stream" {
		t.Fatalf("swap: %q %q", ta.Title, tb.Title)
	}
	if n := len(e.tracks("")); n != visibleTracks {
		t.Fatalf("after swap: %d tracks", n)
	}

	// Sidecar lyrics added/removed without touching the audio file.
	lrc := strings.TrimSuffix(e.abs(a), ".wav") + ".lrc"
	if err := os.WriteFile(lrc, []byte("[00:01.00]hi"), 0o644); err != nil {
		t.Fatal(err)
	}
	e.run(false)
	if !e.byPath(a).HasLrc {
		t.Fatal("new .lrc not detected")
	}

	// .rainyignore excludes a folder.
	if err := os.WriteFile(e.abs("Nature Sounds/.rainyignore"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if st := e.run(false); st.Removed != 3 {
		t.Fatalf(".rainyignore: %+v", st)
	}
}

func TestLibraryUnavailable(t *testing.T) {
	e := newEnv(t)
	e.run(false)

	// Share offline: root missing.
	offline := e.root + "-offline"
	if err := os.Rename(e.root, offline); err != nil {
		t.Fatal(err)
	}
	if err := e.sc.Run(e.ctx, Options{}); !errors.Is(err, ErrLibraryUnavailable) {
		t.Fatalf("missing root: %v", err)
	}
	if st := e.sc.Status(); st.Phase != PhaseError || st.LastError == "" {
		t.Fatalf("status: %+v", st)
	}
	// Mount point present but empty.
	if err := os.MkdirAll(e.root, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := e.sc.Run(e.ctx, Options{}); !errors.Is(err, ErrLibraryUnavailable) {
		t.Fatalf("empty root: %v", err)
	}
	if n := len(e.tracks("")); n != visibleTracks {
		t.Fatalf("tracks marked missing while offline: %d left", n)
	}
	if al := e.album("城市夜雨"); al.CoverPath == "" {
		t.Fatal("cover path cleared while the library was offline")
	}
	if ar, _ := e.st.GetArtist(e.ctx, e.album("城市夜雨").AlbumArtistID, ""); ar == nil || ar.ImagePath == "" {
		t.Fatal("artist image cleared while the library was offline")
	}
	// Back online: nothing changed.
	if err := os.Remove(e.root); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(offline, e.root); err != nil {
		t.Fatal(err)
	}
	if st := e.run(false); counts(st) != [4]int{} {
		t.Fatalf("back online: %+v", st)
	}
}

func TestCancelledScanChangesNothing(t *testing.T) {
	e := newEnv(t)
	e.run(false)
	if err := os.Remove(e.abs("Unsorted/untagged demo.mp3")); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(e.ctx)
	cancel()
	if err := e.sc.Run(ctx, Options{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled run: %v", err)
	}
	if tr := e.byPath("Unsorted/untagged demo.mp3"); tr.Missing {
		t.Fatal("cancelled scan marked a track missing")
	}
	if e.sc.Status().Scanning {
		t.Fatal("still scanning")
	}
}

func TestStartInProgressAndLock(t *testing.T) {
	e := newEnv(t)
	unlock := e.sc.LockLibrary() // the scan will block on the lock
	if err := e.sc.Start(e.ctx, Options{}); err != nil {
		t.Fatal(err)
	}
	if err := e.sc.Start(e.ctx, Options{}); !errors.Is(err, ErrScanInProgress) {
		t.Fatalf("second Start: %v", err)
	}
	if err := e.sc.Run(e.ctx, Options{}); !errors.Is(err, ErrScanInProgress) {
		t.Fatalf("Run while running: %v", err)
	}
	ctx, cancel := context.WithTimeout(e.ctx, 50*time.Millisecond)
	defer cancel()
	unlock()
	unlock() // idempotent
	deadline := time.Now().Add(30 * time.Second)
	for e.sc.Status().Scanning || e.sc.Status().FinishedAt == 0 {
		if time.Now().After(deadline) {
			t.Fatal("scan did not finish")
		}
		time.Sleep(20 * time.Millisecond)
	}
	if st := e.sc.Status(); st.Added != visibleTracks {
		t.Fatalf("async scan: %+v", st)
	}
	// TryLockLibrary gives up while the lock is held.
	u2 := e.sc.LockLibrary()
	if _, err := e.sc.TryLockLibrary(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("TryLockLibrary: %v", err)
	}
	u2()
}

func TestSchedule(t *testing.T) {
	e := newEnv(t)
	set := model.DefaultSettings(0)
	set.ScanInterval = "1s"
	if err := e.st.SaveSettings(e.ctx, set); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(e.ctx)
	done := make(chan struct{})
	go func() { e.sc.Schedule(ctx); close(done) }()
	deadline := time.Now().Add(20 * time.Second)
	for e.sc.Status().FinishedAt == 0 {
		if time.Now().After(deadline) {
			t.Fatal("scheduled scan did not run")
		}
		time.Sleep(50 * time.Millisecond)
	}
	cancel()
	select {
	case <-done:
	case <-time.After(15 * time.Second):
		t.Fatal("Schedule did not return")
	}
	if err := e.sc.Start(e.ctx, Options{}); !errors.Is(err, ErrClosed) {
		t.Fatalf("Start after shutdown: %v", err)
	}
}

func TestRescanFiles(t *testing.T) {
	e := newEnv(t)
	e.run(false)
	ctx := e.ctx

	// File changed (tag edit) → new title, album refreshed.
	rel := "The Rainy Days/Blue Skies/02 Umbrella.opus"
	orig := e.byPath(rel)
	if err := tags.Write(e.abs(rel), map[string][]string{"TITLE": {"Umbrella (Edit)"}, "ALBUM": {"Blue Skies Deluxe"}}); err != nil {
		t.Fatal(err)
	}
	got, err := e.sc.RescanFiles(ctx, e.lib.ID, []string{rel})
	if err != nil || len(got) != 1 || got[0].ID != orig.ID || got[0].Title != "Umbrella (Edit)" || got[0].Album != "Blue Skies Deluxe" {
		t.Fatalf("RescanFiles(changed): %+v %v", got, err)
	}
	deluxe := e.album("Blue Skies Deluxe")
	if deluxe.SongCount != 1 || !strings.HasSuffix(deluxe.CoverPath, "folder.png") {
		t.Fatalf("new album: %+v", deluxe)
	}
	if al := e.album("Blue Skies"); al.SongCount != 3 {
		t.Fatalf("old album not refreshed: %+v", al)
	}

	// Renamed, caller already updated the path.
	newRel := "The Rainy Days/Blue Skies/Umbrella renamed.opus"
	if err := os.Rename(e.abs(rel), e.abs(newRel)); err != nil {
		t.Fatal(err)
	}
	if err := e.st.UpdateTrackPath(ctx, orig.ID, newRel); err != nil {
		t.Fatal(err)
	}
	got, err = e.sc.RescanFiles(ctx, e.lib.ID, []string{rel, newRel})
	if err != nil || len(got) != 1 || got[0].ID != orig.ID || got[0].Path != newRel {
		t.Fatalf("RescanFiles(renamed+updated): %+v %v", got, err)
	}

	// Renamed without UpdateTrackPath → move detection among the given paths.
	third := "The Rainy Days/Blue Skies/sub/third name.opus"
	if err := os.MkdirAll(filepath.Dir(e.abs(third)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(e.abs(newRel), e.abs(third)); err != nil {
		t.Fatal(err)
	}
	got, err = e.sc.RescanFiles(ctx, e.lib.ID, []string{newRel, third})
	if err != nil || len(got) != 1 || got[0].ID != orig.ID || got[0].Path != third || got[0].Missing {
		t.Fatalf("RescanFiles(renamed): %+v %v", got, err)
	}

	// Deleted → missing; new file → added.
	got, err = e.sc.RescanFiles(ctx, e.lib.ID, []string{"The Rainy Days/Blue Skies/01 Blue Skies.opus"})
	if err != nil || len(got) != 1 {
		t.Fatal(err)
	}
	blue := got[0]
	if err := os.Remove(e.abs(blue.Path)); err != nil {
		t.Fatal(err)
	}
	copyRel := "The Rainy Days/Blue Skies/copy.opus"
	data, _ := os.ReadFile(e.abs("The Rainy Days/Blue Skies/04 Don't Stop the Rain.opus"))
	if err := os.WriteFile(e.abs(copyRel), data, 0o644); err != nil {
		t.Fatal(err)
	}
	got, err = e.sc.RescanFiles(ctx, e.lib.ID, []string{blue.Path, copyRel})
	if err != nil || len(got) != 1 || got[0].Path != copyRel || got[0].ID == blue.ID {
		t.Fatalf("RescanFiles(delete+new): %+v %v", got, err)
	}
	if tr, _ := e.st.GetTrack(ctx, blue.ID, ""); tr == nil || !tr.Missing {
		t.Fatal("deleted file not marked missing")
	}

	// Sidecar lyrics path re-evaluates has_lrc of its audio file.
	lrcRel := "The Rainy Days/Blue Skies/copy.lrc"
	if err := os.WriteFile(e.abs(lrcRel), []byte("[00:00.00]x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := e.sc.RescanFiles(ctx, e.lib.ID, []string{lrcRel}); err != nil {
		t.Fatal(err)
	}
	if !e.byPath(copyRel).HasLrc {
		t.Fatal("has_lrc not updated from sidecar path")
	}

	// Unsafe paths are rejected.
	if _, err := e.sc.RescanFiles(ctx, e.lib.ID, []string{"../outside.mp3"}); err == nil {
		t.Fatal("unsafe path accepted")
	}
}

func TestRescanDir(t *testing.T) {
	e := newEnv(t)
	e.run(false)
	dir := "Static Noise/Unknown Signals"
	cover, err := os.ReadFile(e.abs("林雨晴/城市夜雨 (2019)/cover.jpg"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(e.abs(dir+"/Front.JPG"), cover, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(e.abs(dir + "/04 Dead Air.ogg")); err != nil {
		t.Fatal(err)
	}
	sub, cancel := e.bus.Subscribe()
	defer cancel()
	if err := e.sc.RescanDir(e.ctx, e.lib.ID, dir); err != nil {
		t.Fatal(err)
	}
	al := e.album("Unknown Signals")
	if !strings.HasSuffix(al.CoverPath, "Front.JPG") || al.SongCount != 3 {
		t.Fatalf("RescanDir: %+v", al)
	}
	if len(sub) == 0 {
		t.Fatal("no library event")
	}
	// Other folders are untouched.
	if n := len(e.tracks("")); n != visibleTracks-1 {
		t.Fatalf("tracks: %d", n)
	}
	// A vanished sub-directory marks its tracks missing (root still fine).
	if err := os.RemoveAll(e.abs("Nature Sounds")); err != nil {
		t.Fatal(err)
	}
	if err := e.sc.RescanDir(e.ctx, e.lib.ID, "Nature Sounds"); err != nil {
		t.Fatal(err)
	}
	if n := len(e.tracks("")); n != visibleTracks-4 {
		t.Fatalf("tracks after removing a folder: %d", n)
	}
}

// Identical copies of a song (same size, duration and title) that are moved together must
// each keep their own id: the file that kept its name takes over the id of that name.
func TestMoveDuplicatesKeepTheirIDs(t *testing.T) {
	e := newEnv(t)
	data, err := os.ReadFile(e.abs("Static Noise/Unknown Signals/01 Signal 1.ogg"))
	if err != nil {
		t.Fatal(err)
	}
	names := []string{"a.ogg", "b.ogg", "c.ogg", "d.ogg", "e.ogg"}
	if err := os.MkdirAll(e.abs("Dups"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, n := range names {
		if err := os.WriteFile(e.abs("Dups/"+n), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	e.run(false)
	ids := map[string]string{}
	for _, n := range names {
		ids[n] = e.byPath("Dups/" + n).ID
	}
	if err := os.Rename(e.abs("Dups"), e.abs("Moved Dups")); err != nil {
		t.Fatal(err)
	}
	if st := e.run(false); st.Moved != len(names) || st.Added != 0 || st.Removed != 0 {
		t.Fatalf("move: %+v", st)
	}
	for _, n := range names {
		if got := e.byPath("Moved Dups/" + n).ID; got != ids[n] {
			t.Errorf("%s: id %s, want %s", n, got, ids[n])
		}
	}
	// The same through RescanFiles (caller did not update the paths).
	if err := os.Rename(e.abs("Moved Dups"), e.abs("Dups")); err != nil {
		t.Fatal(err)
	}
	var paths []string
	for i := len(names) - 1; i >= 0; i-- {
		paths = append(paths, "Dups/"+names[i], "Moved Dups/"+names[i])
	}
	if _, err := e.sc.RescanFiles(e.ctx, e.lib.ID, paths); err != nil {
		t.Fatal(err)
	}
	for _, n := range names {
		if got := e.byPath("Dups/" + n); got.ID != ids[n] || got.Missing {
			t.Errorf("RescanFiles %s: id %s missing=%v, want %s", n, got.ID, got.Missing, ids[n])
		}
	}
}

// RescanDir must only consider tracks inside the exact directory: SQLite's LIKE is
// case-insensitive, and on Linux "Nature Sounds" and "nature sounds" are different folders.
func TestRescanDirIsCaseExact(t *testing.T) {
	e := newEnv(t)
	e.run(false)
	other := model.Track{LibraryID: e.lib.ID, Path: "nature sounds/elsewhere.wav", Title: "Elsewhere",
		Album: "Other", Artist: "Other", AlbumArtist: "Other", Size: 1, Mtime: 1}
	if err := e.st.UpsertTracks(e.ctx, []model.Track{other}); err != nil {
		t.Fatal(err)
	}
	if err := e.sc.RescanDir(e.ctx, e.lib.ID, "Nature Sounds"); err != nil {
		t.Fatal(err)
	}
	if tr := e.byPath("nature sounds/elsewhere.wav"); tr.Missing {
		t.Fatal("RescanDir marked a track of a differently-cased folder missing")
	}
	// Tracks of a directory whose name merely starts with the scanned one are untouched too.
	if err := e.sc.RescanDir(e.ctx, e.lib.ID, "nature"); err != nil && !errors.Is(err, fs.ErrNotExist) {
		t.Fatal(err)
	}
	if n := len(e.tracks("")); n != visibleTracks+1 {
		t.Fatalf("tracks: %d", n)
	}
}

func TestBackslashNamesSkipped(t *testing.T) {
	if addressable(`a\b.mp3`) || !addressable("AC-DC.mp3") {
		t.Fatal("addressable")
	}
	if runtime.GOOS == "windows" {
		t.Skip("file names cannot contain a backslash on Windows")
	}
	e := newEnv(t)
	data, err := os.ReadFile(e.abs("Unsorted/untagged demo.mp3"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(e.root, "Unsorted", `AC\DC.mp3`), data, 0o644); err != nil {
		t.Fatal(err)
	}
	if st := e.run(false); st.Added != visibleTracks || st.Errors != 0 {
		t.Fatalf("scan: %+v", st)
	}
}

func TestMatchHelpers(t *testing.T) {
	for dir, want := range map[string]bool{"CD1": true, "Disc 2": true, "disk-3": true, "CD 1 - Live": true, "21": false, "Aurora Lights": false, "CDs": false} {
		if discDirRe.MatchString(dir) != want {
			t.Errorf("discDirRe(%q) != %v", dir, want)
		}
	}
	w := walkResult{failed: []string{"a/b"}}
	if !w.underFailed("a/b/c.mp3") || w.underFailed("a/bc.mp3") || w.underFailed("x.mp3") {
		t.Error("underFailed")
	}
}

// linkDir links link → target: a symlink, or on Windows without the symlink privilege a
// directory junction (which behaves the same way when its target is gone).
func linkDir(t *testing.T, target, link string) {
	t.Helper()
	err := os.Symlink(target, link)
	if err == nil {
		return
	}
	if runtime.GOOS == "windows" {
		if out, jerr := exec.Command("cmd", "/c", "mklink", "/J", link, target).CombinedOutput(); jerr == nil {
			return
		} else {
			t.Logf("mklink /J: %v %s", jerr, out)
		}
	}
	t.Skipf("links unavailable: %v", err)
}

// A linked folder whose target is unreachable (a NAS share linked into the library that
// is offline) must be treated like an unreadable directory: its tracks are left alone
// instead of being marked missing, and its albums keep their covers and artist images.
func TestOfflineLinkedShareKeepsTracksAndCovers(t *testing.T) {
	src := filepath.Join("..", "..", "testdata", "music")
	if _, err := os.Stat(src); err != nil {
		t.Skip("testdata/music missing")
	}
	base := t.TempDir()
	root := filepath.Join(base, "music")
	copyDir(t, filepath.Join(src, "Static Noise"), filepath.Join(root, "Static Noise"))
	share := filepath.Join(base, "nas", "林雨晴")
	copyDir(t, filepath.Join(src, "林雨晴"), share)
	linkDir(t, share, filepath.Join(root, "林雨晴"))
	e := newEnvAt(t, root)
	e.run(false)
	if n := len(e.tracks("")); n != 15 {
		t.Fatalf("initial scan: %d tracks, want 15", n)
	}
	city := e.album("城市夜雨")
	artist, err := e.st.GetArtist(e.ctx, city.AlbumArtistID, "")
	if err != nil || city.CoverPath == "" || artist.ImagePath == "" {
		t.Fatalf("initial covers: album %q artist %q %v", city.CoverPath, artist.ImagePath, err)
	}

	offline := filepath.Join(base, "nas-offline")
	if err := os.Rename(filepath.Join(base, "nas"), offline); err != nil {
		t.Fatal(err)
	}
	for _, full := range []bool{false, true} {
		st := e.run(full)
		if st.Removed != 0 || len(e.tracks("only")) != 0 {
			t.Fatalf("share offline (full=%v): removed=%d missing=%d; want tracks left alone", full, st.Removed, len(e.tracks("only")))
		}
		a := e.album("城市夜雨")
		ar, _ := e.st.GetArtist(e.ctx, city.AlbumArtistID, "")
		if a.CoverPath != city.CoverPath || a.UpdatedAt != city.UpdatedAt || ar.ImagePath != artist.ImagePath {
			t.Fatalf("share offline (full=%v): cover %q → %q, artist image %q → %q", full, city.CoverPath, a.CoverPath, artist.ImagePath, ar.ImagePath)
		}
	}

	if err := os.Rename(offline, filepath.Join(base, "nas")); err != nil {
		t.Fatal(err)
	}
	if st := e.run(false); counts(st) != [4]int{} || len(e.tracks("")) != 15 {
		t.Fatalf("share back: %+v", st)
	}
}

// A moved file must take over the id of the track that just vanished, not that of an old,
// already-missing copy of the same song (whose row carries stale annotations): otherwise
// the live track's plays, stars and playlist entries end up on a missing row.
func TestMovePrefersJustVanishedOverMissing(t *testing.T) {
	e := newEnv(t)
	data, err := os.ReadFile(e.abs("Static Noise/Unknown Signals/01 Signal 1.ogg"))
	if err != nil {
		t.Fatal(err)
	}
	for _, dir := range []string{"A Old", "X Live"} {
		if err := os.MkdirAll(e.abs(dir), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(e.abs(dir+"/song.ogg"), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	e.run(false)
	old, live := e.byPath("A Old/song.ogg").ID, e.byPath("X Live/song.ogg").ID
	if err := os.RemoveAll(e.abs("A Old")); err != nil {
		t.Fatal(err)
	}
	if st := e.run(false); st.Removed != 1 {
		t.Fatalf("delete old copy: %+v", st)
	}
	if err := os.Rename(e.abs("X Live"), e.abs("Z Moved")); err != nil {
		t.Fatal(err)
	}
	if st := e.run(false); st.Moved != 1 || st.Removed != 0 || st.Added != 0 {
		t.Fatalf("move: %+v", st)
	}
	if got := e.byPath("Z Moved/song.ogg").ID; got != live {
		t.Fatalf("moved file got id %s (old missing copy %s), want %s", got, old, live)
	}
	if !e.byPath("A Old/song.ogg").Missing {
		t.Fatal("old copy should still be missing")
	}
}

// A live track moved onto the path of an old, already-missing row must keep its own id:
// the stale row at that path must not be revived while the live track's row (with its
// plays, stars and playlist entries) is marked missing.
func TestMoveOntoMissingRowPathKeepsLiveID(t *testing.T) {
	e := newEnv(t)
	data, err := os.ReadFile(e.abs("Static Noise/Unknown Signals/01 Signal 1.ogg"))
	if err != nil {
		t.Fatal(err)
	}
	for _, dir := range []string{"Z Target", "X Live"} {
		if err := os.MkdirAll(e.abs(dir), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(e.abs(dir+"/song.ogg"), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	e.run(false)
	stale, live := e.byPath("Z Target/song.ogg").ID, e.byPath("X Live/song.ogg").ID
	if err := os.RemoveAll(e.abs("Z Target")); err != nil {
		t.Fatal(err)
	}
	if st := e.run(false); st.Removed != 1 {
		t.Fatalf("delete old copy: %+v", st)
	}
	u := &model.User{Username: "listener", PasswordEnc: "synthetic"}
	if err := e.st.CreateUser(e.ctx, u); err != nil {
		t.Fatal(err)
	}
	if err := e.st.SetStarred(e.ctx, u.ID, "track", []string{live}, true); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(e.abs("X Live"), e.abs("Z Target")); err != nil {
		t.Fatal(err)
	}
	if st := e.run(false); st.Moved != 1 || st.Removed != 0 || st.Added != 0 || st.Errors != 0 {
		t.Fatalf("move: %+v", st)
	}
	got := e.byPath("Z Target/song.ogg")
	if got.ID != live || got.Missing {
		t.Fatalf("moved file got id %s missing=%v (stale row %s), want %s", got.ID, got.Missing, stale, live)
	}
	if _, err := e.st.GetTrack(e.ctx, stale, ""); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("stale row at the reused path should be gone, got err=%v", err)
	}
	if tr, err := e.st.GetTrack(e.ctx, live, u.ID); err != nil || !tr.Starred {
		t.Fatalf("live annotations lost: %+v %v", tr, err)
	}
	// Without a matching vanished track the file at a missing row's path revives that row.
	if err := os.WriteFile(e.abs("Z Target/other.ogg"), data, 0o644); err != nil {
		t.Fatal(err)
	}
	e.run(false)
	other := e.byPath("Z Target/other.ogg").ID
	if err := os.Remove(e.abs("Z Target/other.ogg")); err != nil {
		t.Fatal(err)
	}
	e.run(false)
	if err := os.WriteFile(e.abs("Z Target/other.ogg"), data, 0o644); err != nil {
		t.Fatal(err)
	}
	if st := e.run(false); st.Updated != 1 || st.Moved != 0 {
		t.Fatalf("revive: %+v", st)
	}
	if got := e.byPath("Z Target/other.ogg"); got.ID != other || got.Missing {
		t.Fatalf("revived row %s missing=%v, want %s", got.ID, got.Missing, other)
	}
}

// A directory that held many tracks and is now completely empty is most likely an offline
// mount point (a docker bind mount of a NAS share inside the library): its tracks are left
// alone instead of being marked missing. Small folders emptied by the user, and folders
// that are removed, are handled normally.
func TestEmptiedLargeDirIsTreatedAsOffline(t *testing.T) {
	e := newEnv(t)
	data, err := os.ReadFile(e.abs("Static Noise/Unknown Signals/01 Signal 1.ogg"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(e.abs("Mount/Album"), 0o755); err != nil {
		t.Fatal(err)
	}
	for i := range offlineDirMinTracks {
		if err := os.WriteFile(e.abs(fmt.Sprintf("Mount/Album/%02d.ogg", i)), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if st := e.run(false); st.Added != visibleTracks+offlineDirMinTracks {
		t.Fatalf("initial scan: %+v", st)
	}
	// The mount goes offline: an empty mount point remains.
	if err := os.RemoveAll(e.abs("Mount/Album")); err != nil {
		t.Fatal(err)
	}
	// A small folder emptied at the same time is a normal deletion.
	for _, n := range []string{"01 Signal 1.ogg", "02 Signal 2.ogg", "03 Interference.ogg", "04 Dead Air.ogg"} {
		if err := os.Remove(e.abs("Static Noise/Unknown Signals/" + n)); err != nil {
			t.Fatal(err)
		}
	}
	for _, full := range []bool{false, true} {
		if st := e.run(full); st.Removed != 4 && (!full || st.Removed != 0) {
			t.Fatalf("offline mount (full=%v): %+v", full, st)
		}
		if n := len(e.tracks("only")); n != 4 {
			t.Fatalf("offline mount (full=%v): %d missing tracks, want only the 4 deleted ones", full, n)
		}
	}
	// Once the mount point itself is gone, its tracks are missing.
	if err := os.Remove(e.abs("Mount")); err != nil {
		t.Fatal(err)
	}
	if st := e.run(false); st.Removed != offlineDirMinTracks {
		t.Fatalf("mount point removed: %+v", st)
	}
}
