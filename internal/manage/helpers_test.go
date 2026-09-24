package manage

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"rainy/internal/config"
	"rainy/internal/db/dbtest"
	"rainy/internal/events"
	"rainy/internal/model"
	"rainy/internal/store"
	"rainy/internal/tags"
	"rainy/internal/util"
)

// fakeScanner implements fileScanner on top of the store without reading tags: rescanned
// files keep their row (looked up by path) with refreshed size / mtime; unknown files get
// a row derived from the path; vanished files are marked missing.
type fakeScanner struct {
	mu      sync.Mutex
	st      *store.Store
	calls   [][]string
	dirs    []string
	callsMu sync.Mutex

	checkLock bool   // record in dirLocked whether RescanDir runs under the library lock
	dirLocked []bool // guarded by callsMu
}

func (f *fakeScanner) LockLibrary() func() {
	f.mu.Lock()
	var once sync.Once
	return func() { once.Do(f.mu.Unlock) }
}

func (f *fakeScanner) RescanDir(ctx context.Context, libraryID int64, dir string) error {
	locked := false
	if f.checkLock {
		if f.mu.TryLock() {
			f.mu.Unlock()
		} else {
			locked = true
		}
	}
	f.callsMu.Lock()
	f.dirs = append(f.dirs, dir)
	if f.checkLock {
		f.dirLocked = append(f.dirLocked, locked)
	}
	f.callsMu.Unlock()
	return nil
}

func (f *fakeScanner) RescanFiles(ctx context.Context, libraryID int64, paths []string) ([]model.Track, error) {
	f.callsMu.Lock()
	f.calls = append(f.calls, append([]string{}, paths...))
	f.callsMu.Unlock()
	lib, err := f.st.GetLibrary(ctx, libraryID)
	if err != nil {
		return nil, err
	}
	var out []model.Track
	for _, p := range paths {
		abs, err := util.SafeJoin(lib.Path, p)
		if err != nil {
			return nil, err
		}
		fi, statErr := os.Stat(abs)
		t, err := f.st.GetTrackByPath(ctx, libraryID, p)
		if statErr != nil {
			if err == nil {
				_ = f.st.MarkTracksMissing(ctx, []string{t.ID}, true)
			}
			continue
		}
		if err != nil {
			nt := trackFromPath(libraryID, p)
			t = &nt
		}
		t.Size, t.Mtime, t.Missing = fi.Size(), fi.ModTime().UnixMilli(), false
		t.Genres = nil
		if err := f.st.UpsertTracks(ctx, []model.Track{*t}); err != nil {
			return nil, err
		}
		out = append(out, *t)
	}
	return out, f.st.RefreshAll(ctx)
}

var leadingNum = regexp.MustCompile(`^(?:(\d+)-)?(\d+)\s+`)

// trackFromPath derives a track row from "Artist/Album/[CDn/]NN Title.ext".
func trackFromPath(libraryID int64, rel string) model.Track {
	parts := strings.Split(rel, "/")
	name := parts[len(parts)-1]
	title := stem(name)
	var disc, track int
	if m := leadingNum.FindStringSubmatch(title); m != nil {
		disc, _ = strconv.Atoi(m[1])
		track, _ = strconv.Atoi(m[2])
		title = title[len(m[0]):]
	}
	artist, album := "[Unknown Artist]", "[Unknown Album]"
	if len(parts) >= 3 {
		artist, album = parts[0], parts[1]
	}
	return model.Track{
		LibraryID: libraryID, Path: rel, Title: title, Artist: artist, Album: album, AlbumArtist: artist,
		AlbumID: util.AlbumID(artist, album), ArtistID: util.ArtistID(artist), AlbumArtistID: util.ArtistID(artist),
		TrackNumber: track, DiscNumber: disc, Duration: 60,
	}
}

type env struct {
	t    *testing.T
	ctx  context.Context
	st   *store.Store
	sc   *fakeScanner
	svc  *Service
	cfg  *config.Config
	lib  *model.Library
	user *model.User
	bus  *events.Bus
}

// musicSource returns the shared test library (skips when it has not been generated).
func musicSource(t *testing.T) string {
	t.Helper()
	_, file, _, _ := runtime.Caller(0)
	src := filepath.Join(filepath.Dir(file), "..", "..", "testdata", "music")
	if _, err := os.Stat(src); err != nil {
		t.Skip("testdata/music missing: run bash scripts/gen-testdata.sh")
	}
	return src
}

// copyTree copies src into dst (never modifying src).
func copyTree(t *testing.T, src, dst string) {
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
		return out.Close()
	})
	if err != nil {
		t.Fatalf("copying test library: %v", err)
	}
}

// newEnv creates a service over a fresh DB and a copy of testdata/music (seeded as
// tracks derived from the paths).
func newEnv(t *testing.T) *env {
	t.Helper()
	music := filepath.Join(t.TempDir(), "music")
	copyTree(t, musicSource(t), music)
	return newEnvAt(t, music, true)
}

func newEnvAt(t *testing.T, music string, seed bool) *env {
	t.Helper()
	ctx := context.Background()
	st := store.New(dbtest.New(t))
	cfg := config.Default()
	cfg.DataDir = filepath.Join(t.TempDir(), "data")
	cfg.MusicDir = music
	if err := cfg.EnsureDirs(); err != nil {
		t.Fatal(err)
	}
	lib := &model.Library{Name: "music", Path: music}
	if err := st.CreateLibrary(ctx, lib); err != nil {
		t.Fatal(err)
	}
	u := &model.User{Username: "editor", PasswordEnc: "synthetic", CanManage: true}
	if err := st.CreateUser(ctx, u); err != nil {
		t.Fatal(err)
	}
	sc := &fakeScanner{st: st}
	bus := events.NewBus()
	e := &env{t: t, ctx: ctx, st: st, sc: sc, svc: newService(st, sc, nil, bus, cfg), cfg: cfg, lib: lib, user: u, bus: bus}
	if seed {
		e.seed()
	}
	return e
}

// seed inserts a track row for every audio file of the library.
func (e *env) seed() {
	e.t.Helper()
	var tracks []model.Track
	err := filepath.WalkDir(e.lib.Path, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && util.IsHiddenOrSystem(d.Name()) {
			return filepath.SkipDir
		}
		if d.IsDir() || !tags.IsAudioFile(d.Name()) {
			return nil
		}
		rel, _ := util.ToRel(e.lib.Path, p)
		fi, _ := d.Info()
		tr := trackFromPath(e.lib.ID, rel)
		tr.Size, tr.Mtime = fi.Size(), fi.ModTime().UnixMilli()
		tracks = append(tracks, tr)
		return nil
	})
	if err != nil {
		e.t.Fatal(err)
	}
	if err := e.st.UpsertTracks(e.ctx, tracks); err != nil {
		e.t.Fatal(err)
	}
	if err := e.st.RefreshAll(e.ctx); err != nil {
		e.t.Fatal(err)
	}
}

// track returns the row at a library-relative path.
func (e *env) track(rel string) *model.Track {
	e.t.Helper()
	t, err := e.st.GetTrackByPath(e.ctx, e.lib.ID, rel)
	if err != nil {
		e.t.Fatalf("track %q: %v", rel, err)
	}
	return t
}

func (e *env) abs(rel string) string {
	return filepath.Join(e.lib.Path, filepath.FromSlash(rel))
}

func (e *env) exists(rel string) bool { return exists(e.abs(rel)) }

// logActions returns the actions of the edit log, newest first.
func (e *env) logActions(trackID string) []string {
	e.t.Helper()
	entries, _, err := e.st.ListEditLog(e.ctx, trackID, 0, 0)
	if err != nil {
		e.t.Fatal(err)
	}
	out := make([]string, len(entries))
	for i, en := range entries {
		out[i] = en.Action
	}
	return out
}

// requireTags skips the test when the tags package cannot read/write files yet.
func requireTags(t *testing.T, file string) {
	t.Helper()
	if _, err := tags.ReadRaw(file); err != nil {
		if errors.Is(err, errors.ErrUnsupported) {
			t.Skip("tags package not implemented yet")
		}
		t.Fatalf("reading tags of %s: %v", file, err)
	}
}

func init() { lockTimeout = 5 * time.Second }

func mustRead(t *testing.T, p string) []byte {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
