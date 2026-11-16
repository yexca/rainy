package manage

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"golang.org/x/text/encoding/simplifiedchinese"

	"rainy/internal/model"
	"rainy/internal/store"
)

func tinyPNG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.RGBA{R: 32, G: 96, B: 180, A: 255})
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestListFolderEnrichesTracksAndSubfolderStats(t *testing.T) {
	e := newEnvAt(t, t.TempDir(), false)
	mustWrite := func(rel string, data []byte) {
		t.Helper()
		abs := filepath.Join(e.lib.Path, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(abs, data, 0o600); err != nil {
			t.Fatal(err)
		}
	}

	mustWrite("Artist 10/Album/01.wav", silentWAV())
	mustWrite("Artist 2/Disc 1/01.wav", silentWAV())
	mustWrite("Artist 2/Disc 1/02.wav", silentWAV())
	mustWrite("Artist 2/Disc 1/unindexed.wav", silentWAV())
	mustWrite("Artist 2/Disc 1/cover.png", tinyPNG(t))
	mustWrite("Artist 2/Disc 1/01.lrc", []byte("[00:01.00]Synthetic line\n"))
	mustWrite("Artist 2/Disc 1/.DS_Store", []byte("ignored"))
	mustWrite("EmptyLeaf/notes.txt", []byte("plain note"))
	mustWrite("UnscannedTree/Child/notes.txt", []byte("nested note"))

	tracks := []model.Track{
		{
			ID: "trk-artist10", LibraryID: e.lib.ID, Path: "Artist 10/Album/01.wav",
			Dir: "Artist 10/Album", Filename: "01.wav", Suffix: "wav",
			Title: "Ten", Artist: "Artist 10", Album: "Album", Duration: 42,
		},
		{
			ID: "trk-01", LibraryID: e.lib.ID, Path: "Artist 2/Disc 1/01.wav",
			Dir: "Artist 2/Disc 1", Filename: "01.wav", Suffix: "wav",
			Title: "First Song", Artist: "Artist 2", Album: "Disc 1", Duration: 120, HasLrc: true,
		},
		{
			ID: "trk-02", LibraryID: e.lib.ID, Path: "Artist 2/Disc 1/02.wav",
			Dir: "Artist 2/Disc 1", Filename: "02.wav", Suffix: "wav",
			Title: "Second Song", Artist: "Artist 2", Album: "Disc 1", Duration: 95,
		},
	}
	if err := e.st.UpsertTracks(e.ctx, tracks); err != nil {
		t.Fatal(err)
	}
	if err := e.st.SetStarred(e.ctx, e.user.ID, "track", []string{"trk-01"}, true); err != nil {
		t.Fatal(err)
	}

	root, err := e.svc.ListFolder(e.ctx, e.lib.ID, "", e.user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if root.Parent != nil || len(root.Folders) != 4 {
		t.Fatalf("unexpected root listing: %+v", root)
	}
	// Natural sort puts "Artist 2" before "Artist 10".
	if root.Folders[0].Name != "Artist 2" || root.Folders[1].Name != "Artist 10" {
		t.Fatalf("unexpected folder order: %+v", root.Folders)
	}
	if root.Folders[0].TrackCount != 2 || !root.Folders[0].HasSubfolders {
		t.Fatalf("Artist 2 stats: %+v", root.Folders[0])
	}
	if root.Folders[1].TrackCount != 1 || !root.Folders[1].HasSubfolders {
		t.Fatalf("Artist 10 stats: %+v", root.Folders[1])
	}
	for _, f := range root.Folders {
		if f.Name == "EmptyLeaf" && (f.TrackCount != 0 || f.HasSubfolders) {
			t.Fatalf("EmptyLeaf should have no tracks or subfolders: %+v", f)
		}
		if f.Name == "UnscannedTree" && (f.TrackCount != 0 || !f.HasSubfolders) {
			t.Fatalf("UnscannedTree should report disk subfolders: %+v", f)
		}
	}

	leaf, err := e.svc.ListFolder(e.ctx, e.lib.ID, "Artist 2/Disc 1", e.user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if leaf.Parent == nil || *leaf.Parent != "Artist 2" || len(leaf.Folders) != 0 {
		t.Fatalf("unexpected leaf header: %+v", leaf)
	}
	if len(leaf.Files) != 5 {
		t.Fatalf("expected 5 visible files (excluding .DS_Store), got %+v", leaf.Files)
	}
	byName := map[string]FileEntry{}
	for _, f := range leaf.Files {
		byName[f.Name] = f
	}
	first := byName["01.wav"]
	if first.TrackID == nil || *first.TrackID != "trk-01" || first.Track == nil {
		t.Fatalf("01.wav missing track payload: %+v", first)
	}
	if first.Track.Title != "First Song" || !first.Track.Starred || first.Track.Duration != 120 {
		t.Fatalf("01.wav track annotations/metadata wrong: %+v", first.Track)
	}
	unindexed := byName["unindexed.wav"]
	if !unindexed.IsAudio || unindexed.TrackID != nil || unindexed.Track != nil {
		t.Fatalf("unindexed.wav should not have track metadata: %+v", unindexed)
	}
	if !byName["cover.png"].IsImage {
		t.Fatalf("cover.png should be marked IsImage: %+v", byName["cover.png"])
	}
}

func TestFolderImageAndTextPreviewsAndGuards(t *testing.T) {
	e := newEnvAt(t, t.TempDir(), false)
	write := func(rel string, data []byte) {
		t.Helper()
		abs := filepath.Join(e.lib.Path, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(abs, data, 0o600); err != nil {
			t.Fatal(err)
		}
	}

	pngBytes := tinyPNG(t)
	write("Album/cover.png", pngBytes)
	write("Album/fake.png", []byte("<html>not an image</html>"))
	write("Album/.hidden.png", pngBytes)
	write("Album/song.wav", silentWAV())
	write("Album/binary.txt", []byte{'a', 0, 'b'})

	gbkLyrics, err := simplifiedchinese.GB18030.NewEncoder().Bytes([]byte("[00:02.00]中文歌词测试\n"))
	if err != nil {
		t.Fatal(err)
	}
	write("Album/song.lrc", gbkLyrics)
	write("Album/huge.log", bytes.Repeat([]byte("line0123\n"), 40000))

	data, ctype, size, mtime, err := e.svc.FolderImage(e.ctx, e.lib.ID, "Album/cover.png")
	if err != nil {
		t.Fatal(err)
	}
	if ctype != "image/png" || size != int64(len(pngBytes)) || mtime <= 0 || !bytes.Equal(data, pngBytes) {
		t.Fatalf("unexpected image preview: ctype=%s size=%d mtime=%d", ctype, size, mtime)
	}

	for _, bad := range []string{"", "../cover.png", "Album/.hidden.png", "Album/song.wav", "Album/fake.png", "Album"} {
		if _, _, _, _, err := e.svc.FolderImage(e.ctx, e.lib.ID, bad); !errors.Is(err, store.ErrInvalid) {
			t.Errorf("FolderImage(%q) expected ErrInvalid, got %v", bad, err)
		}
	}
	if _, _, _, _, err := e.svc.FolderImage(e.ctx, e.lib.ID, "Album/missing.png"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}

	txt, err := e.svc.FolderText(e.ctx, e.lib.ID, "Album/song.lrc")
	if err != nil {
		t.Fatal(err)
	}
	if txt.Truncated || !strings.Contains(txt.Content, "中文歌词测试") {
		t.Fatalf("unexpected decoded LRC: %+v", txt)
	}

	huge, err := e.svc.FolderText(e.ctx, e.lib.ID, "Album/huge.log")
	if err != nil {
		t.Fatal(err)
	}
	if !huge.Truncated || len(huge.Content) > maxFolderTextSize {
		t.Fatalf("expected truncated log preview, got len=%d truncated=%v", len(huge.Content), huge.Truncated)
	}

	for _, bad := range []string{"", "../notes.txt", "Album/.hidden.txt", "Album/song.wav", "Album/binary.txt", "Album"} {
		if _, err := e.svc.FolderText(e.ctx, e.lib.ID, bad); !errors.Is(err, store.ErrInvalid) {
			t.Errorf("FolderText(%q) expected ErrInvalid, got %v", bad, err)
		}
	}

	if runtime.GOOS != "windows" {
		outside := filepath.Join(t.TempDir(), "outside.txt")
		if err := os.WriteFile(outside, []byte("secret"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(outside, filepath.Join(e.lib.Path, "Album", "escape.txt")); err == nil {
			if _, err := e.svc.FolderText(e.ctx, e.lib.ID, "Album/escape.txt"); err == nil {
				t.Fatal("symlink escape must be rejected")
			}
		}
	}
}
