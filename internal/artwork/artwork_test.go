package artwork_test

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"rainy/internal/artwork"
	"rainy/internal/config"
	"rainy/internal/db/dbtest"
	"rainy/internal/events"
	"rainy/internal/model"
	"rainy/internal/scanner"
	"rainy/internal/store"
	"rainy/internal/tags"
	"rainy/internal/util"
)

type fixture struct {
	root string
	st   *store.Store
	art  *artwork.Service
	lib  *model.Library
	ctx  context.Context
}

func setup(t *testing.T) *fixture {
	t.Helper()
	src := filepath.Join("..", "..", "testdata", "music")
	if _, err := os.Stat(src); err != nil {
		t.Skip("testdata/music missing (run: bash scripts/gen-testdata.sh)")
	}
	root := filepath.Join(t.TempDir(), "music")
	err := filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		dst := filepath.Join(root, rel)
		if d.IsDir() {
			return os.MkdirAll(dst, 0o755)
		}
		in, err := os.Open(p)
		if err != nil {
			return err
		}
		defer func() { _ = in.Close() }()
		out, err := os.Create(dst)
		if err != nil {
			return err
		}
		_, err = io.Copy(out, in)
		return errors.Join(err, out.Close())
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	st := store.New(dbtest.New(t))
	lib := &model.Library{Name: "music", Path: root}
	if err := st.CreateLibrary(ctx, lib); err != nil {
		t.Fatal(err)
	}
	cfg := config.Default()
	if err := scanner.New(st, events.NewBus(), cfg).Run(ctx, scanner.Options{}); err != nil {
		t.Fatal(err)
	}
	return &fixture{root: root, st: st, art: artwork.New(st, filepath.Join(t.TempDir(), "cache")), lib: lib, ctx: ctx}
}

func (f *fixture) album(t *testing.T, name string) *model.Album {
	t.Helper()
	as, _, err := f.st.ListAlbums(f.ctx, store.AlbumQuery{Q: name})
	if err != nil {
		t.Fatal(err)
	}
	for i := range as {
		if as[i].Name == name {
			return &as[i]
		}
	}
	t.Fatalf("album %q not found", name)
	return nil
}

func (f *fixture) track(t *testing.T, rel string) *model.Track {
	t.Helper()
	tr, err := f.st.GetTrackByPath(f.ctx, f.lib.ID, rel)
	if err != nil {
		t.Fatal(err)
	}
	return tr
}

func dims(t *testing.T, img *artwork.Image) (int, int, string) {
	t.Helper()
	cfg, format, err := image.DecodeConfig(bytes.NewReader(img.Data))
	if err != nil {
		t.Fatalf("decoding result: %v", err)
	}
	return cfg.Width, cfg.Height, format
}

func TestAlbumTrackArtistCovers(t *testing.T) {
	f := setup(t)
	ctx := f.ctx

	// Folder image: original bytes at size 0, resized JPEG otherwise.
	city := f.album(t, "城市夜雨")
	orig, err := os.ReadFile(filepath.Join(f.root, "林雨晴", "城市夜雨 (2019)", "cover.jpg"))
	if err != nil {
		t.Fatal(err)
	}
	img, err := f.art.Get(ctx, city.CoverArt, 0)
	if err != nil || !bytes.Equal(img.Data, orig) || img.ContentType != "image/jpeg" {
		t.Fatalf("original folder cover: %v", err)
	}
	img, err = f.art.Get(ctx, city.CoverArt, 128)
	if err != nil {
		t.Fatal(err)
	}
	if w, h, format := dims(t, img); w != 128 || h != 128 || format != "jpeg" || img.ContentType != "image/jpeg" {
		t.Fatalf("resized: %dx%d %s", w, h, format)
	}
	if size, _ := f.art.CacheSize(); size == 0 {
		t.Fatal("resized image not cached")
	}
	again, err := f.art.Get(ctx, city.CoverArt, 128)
	if err != nil || !bytes.Equal(again.Data, img.Data) {
		t.Fatal("cached result differs")
	}
	// Bigger than the original: original returned unchanged.
	if big, err := f.art.Get(ctx, city.CoverArt, 1024); err != nil || !bytes.Equal(big.Data, orig) {
		t.Fatalf("no upscaling: %v", err)
	}

	// Embedded 1200px PNG (multi-disc album).
	aurora := f.album(t, "Aurora Lights")
	img, err = f.art.Get(ctx, aurora.CoverArt, 0)
	if err != nil || img.ContentType != "image/png" {
		t.Fatalf("embedded original: %v %v", err, img)
	}
	img, err = f.art.Get(ctx, "al-"+aurora.ID, 300) // no version suffix
	if err != nil {
		t.Fatal(err)
	}
	if w, h, _ := dims(t, img); w != 300 || h != 300 {
		t.Fatalf("embedded resized: %dx%d", w, h)
	}

	// Track with embedded art; track without → album folder cover; bare ids.
	mp3 := f.track(t, "林雨晴/夏日微风 (2021)/01 夏日微风.mp3")
	if _, err := f.art.Get(ctx, mp3.CoverArt, 64); err != nil || mp3.CoverArt[:3] != "tr-" {
		t.Fatalf("track cover %q: %v", mp3.CoverArt, err)
	}
	flac := f.track(t, "林雨晴/城市夜雨 (2019)/02 霓虹.flac")
	if img, err := f.art.Get(ctx, util.CoverArtID("tr", flac.ID, 1), 0); err != nil || !bytes.Equal(img.Data, orig) {
		t.Fatalf("track → album fallback: %v", err)
	}
	if img, err := f.art.Get(ctx, flac.ID, 0); err != nil || !bytes.Equal(img.Data, orig) {
		t.Fatalf("bare track id: %v", err)
	}
	if _, err := f.art.Get(ctx, city.ID, 64); err != nil {
		t.Fatalf("bare album id: %v", err)
	}

	// Artists: artist.jpg; fallback to an album cover; none.
	lin, _ := f.st.GetArtist(ctx, mp3.AlbumArtistID, "")
	artistJPG, _ := os.ReadFile(filepath.Join(f.root, "林雨晴", "artist.jpg"))
	if img, err := f.art.Get(ctx, lin.CoverArt, 0); err != nil || !bytes.Equal(img.Data, artistJPG) {
		t.Fatalf("artist image: %v", err)
	}
	echo := f.track(t, "Northern Echo/Aurora Lights/CD1/1-01 Polar Night.flac")
	if _, err := f.art.Get(ctx, util.CoverArtID("ar", echo.AlbumArtistID, 1), 64); err != nil {
		t.Fatalf("artist → album fallback: %v", err)
	}
	noise := f.track(t, "Static Noise/Unknown Signals/01 Signal 1.ogg")
	if _, err := f.art.Get(ctx, util.CoverArtID("ar", noise.AlbumArtistID, 1), 64); !errors.Is(err, artwork.ErrNotFound) {
		t.Fatalf("artist without art: %v", err)
	}
	if _, err := f.art.Get(ctx, util.CoverArtID("al", noise.AlbumID, 1), 64); !errors.Is(err, artwork.ErrNotFound) {
		t.Fatalf("album without art: %v", err)
	}
	for _, id := range []string{"", "al-nope_1", "nope", "pl-nope"} {
		if _, err := f.art.Get(ctx, id, 64); !errors.Is(err, artwork.ErrNotFound) {
			t.Fatalf("Get(%q): %v", id, err)
		}
	}

	// Editing the folder image changes the result (cache keyed by file identity).
	coverPath := filepath.Join(f.root, "林雨晴", "城市夜雨 (2019)", "cover.jpg")
	var buf bytes.Buffer
	red := image.NewRGBA(image.Rect(0, 0, 400, 200))
	for i := range red.Pix {
		red.Pix[i] = 0xff
	}
	if err := jpeg.Encode(&buf, red, nil); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(coverPath, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	later := time.Now().Add(time.Hour)
	_ = os.Chtimes(coverPath, later, later)
	img, err = f.art.Get(ctx, city.CoverArt, 128)
	if err != nil {
		t.Fatal(err)
	}
	if w, h, _ := dims(t, img); w != 128 || h != 64 {
		t.Fatalf("edited cover not picked up / aspect not kept: %dx%d", w, h)
	}

	// Cache management.
	size, err := f.art.CacheSize()
	if err != nil || size == 0 {
		t.Fatalf("CacheSize: %d %v", size, err)
	}
	freed, err := f.art.ClearCache()
	if err != nil || freed != size {
		t.Fatalf("ClearCache: freed %d of %d: %v", freed, size, err)
	}
	if size, _ := f.art.CacheSize(); size != 0 {
		t.Fatalf("cache not empty: %d", size)
	}
}

// A corrupt folder image falls back to the embedded picture; with nothing else to fall
// back to the result is ErrNotFound (404 / placeholder), never an internal error or garbage.
func TestCorruptImagesFallBack(t *testing.T) {
	f := setup(t)
	ctx := f.ctx
	garbage := []byte("this is not an image, just a truncated download")

	aurora := f.album(t, "Aurora Lights") // embedded PNG, no folder image
	bad := filepath.Join(f.root, "Northern Echo", "Aurora Lights", "cover.jpg")
	if err := os.WriteFile(bad, garbage, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := f.st.SetAlbumCoverPath(ctx, aurora.ID, bad); err != nil {
		t.Fatal(err)
	}
	aurora = f.album(t, "Aurora Lights")
	for _, size := range []int{0, 128} {
		img, err := f.art.Get(ctx, aurora.CoverArt, size)
		if err != nil {
			t.Fatalf("size %d: %v", size, err)
		}
		if _, _, format := dims(t, img); format != "png" && format != "jpeg" {
			t.Fatalf("size %d: format %s", size, format)
		}
	}

	blue := f.album(t, "Blue Skies") // folder.png only
	if err := os.WriteFile(filepath.Join(f.root, "The Rainy Days", "Blue Skies", "folder.png"), garbage, 0o644); err != nil {
		t.Fatal(err)
	}
	track := f.track(t, "The Rainy Days/Blue Skies/01 Blue Skies.opus")
	for _, id := range []string{blue.CoverArt, track.CoverArt, blue.ID} {
		for _, size := range []int{0, 64} {
			if img, err := f.art.Get(ctx, id, size); !errors.Is(err, artwork.ErrNotFound) {
				t.Fatalf("Get(%s, %d) = %v, %v; want ErrNotFound", id, size, img != nil, err)
			}
		}
	}
}

func TestPlaylistMosaic(t *testing.T) {
	f := setup(t)
	ctx := f.ctx
	u := &model.User{Username: "u", PasswordEnc: "synthetic"}
	if err := f.st.CreateUser(ctx, u); err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, rel := range []string{
		"林雨晴/夏日微风 (2021)/01 夏日微风.mp3",
		"林雨晴/夏日微风 (2021)/02 海边的约定.mp3", // same album: counted once
		"林雨晴/城市夜雨 (2019)/01 城市夜雨.flac",
		"Static Noise/Unknown Signals/01 Signal 1.ogg", // no art: skipped
		"星野ミライ/雨上がりの空/01 雨上がりの空.m4a",
		"The Rainy Days/Blue Skies/01 Blue Skies.opus",
	} {
		ids = append(ids, f.track(t, rel).ID)
	}
	pl := &model.Playlist{Name: "mix", OwnerID: u.ID}
	if err := f.st.CreatePlaylist(ctx, pl, ids); err != nil {
		t.Fatal(err)
	}
	pl, _ = f.st.GetPlaylist(ctx, pl.ID)
	img, err := f.art.Get(ctx, pl.CoverArt, 300)
	if err != nil {
		t.Fatal(err)
	}
	if w, h, format := dims(t, img); w != 300 || h != 300 || format != "jpeg" {
		t.Fatalf("mosaic: %dx%d %s", w, h, format)
	}
	decoded, _ := jpeg.Decode(bytes.NewReader(img.Data))
	// Quadrants come from different covers.
	c1 := color.RGBAModel.Convert(decoded.At(75, 75)).(color.RGBA)
	c4 := color.RGBAModel.Convert(decoded.At(225, 225)).(color.RGBA)
	if c1 == c4 {
		t.Fatalf("mosaic quadrants identical: %v", c1)
	}
	if img, err := f.art.Get(ctx, pl.CoverArt, 0); err != nil || img.ContentType != "image/jpeg" {
		t.Fatalf("mosaic size 0: %v", err)
	}

	// Fewer than four albums: the first cover.
	pl2 := &model.Playlist{Name: "one", OwnerID: u.ID}
	if err := f.st.CreatePlaylist(ctx, pl2, ids[:2]); err != nil {
		t.Fatal(err)
	}
	single, err := f.art.Get(ctx, util.CoverArtID("pl", pl2.ID, 1), 0)
	if err != nil {
		t.Fatal(err)
	}
	first, _ := f.art.Get(ctx, f.track(t, "林雨晴/夏日微风 (2021)/01 夏日微风.mp3").CoverArt, 0)
	if !bytes.Equal(single.Data, first.Data) {
		t.Fatal("single-album playlist should use that album's cover")
	}
	// Empty playlist.
	pl3 := &model.Playlist{Name: "empty", OwnerID: u.ID}
	if err := f.st.CreatePlaylist(ctx, pl3, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := f.art.Get(ctx, "pl-"+pl3.ID, 64); !errors.Is(err, artwork.ErrNotFound) {
		t.Fatalf("empty playlist: %v", err)
	}
}

func TestConcurrentGets(t *testing.T) {
	f := setup(t)
	city := f.album(t, "城市夜雨")
	var wg sync.WaitGroup
	results := make([][]byte, 16)
	for i := range results {
		wg.Add(1)
		go func() {
			defer wg.Done()
			img, err := f.art.Get(f.ctx, city.CoverArt, 200)
			if err != nil {
				t.Error(err)
				return
			}
			results[i] = img.Data
		}()
	}
	wg.Wait()
	for _, r := range results[1:] {
		if !bytes.Equal(r, results[0]) {
			t.Fatal("concurrent results differ")
		}
	}
}

func TestPlaceholder(t *testing.T) {
	art := artwork.New(nil, "")
	for _, tc := range []struct{ in, want int }{{0, 512}, {-5, 512}, {64, 64}, {5000, 1024}, {1, 16}} {
		img := art.Placeholder(tc.in)
		if img == nil || img.ContentType != "image/jpeg" {
			t.Fatalf("Placeholder(%d) = %v", tc.in, img)
		}
		if w, h, _ := dims(t, img); w != tc.want || h != tc.want {
			t.Fatalf("Placeholder(%d): %dx%d", tc.in, w, h)
		}
	}
	if first, again := art.Placeholder(64), art.Placeholder(64); first != again {
		t.Fatal("placeholder not memoised")
	}
}

func TestFindFolderImage(t *testing.T) {
	dir := t.TempDir()
	var buf bytes.Buffer
	_ = png.Encode(&buf, image.NewGray(image.Rect(0, 0, 2, 2)))
	for _, n := range []string{"folder.png", "Cover.JPG", "front.webp", "cover.txt", "albumart_small.jpg", ".cover.jpg"} {
		if err := os.WriteFile(filepath.Join(dir, n), buf.Bytes(), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	patterns := model.DefaultSettings(0).CoverArtPatterns()
	if got := artwork.FindFolderImage(dir, patterns); filepath.Base(got) != "Cover.JPG" {
		t.Fatalf("FindFolderImage = %q", got)
	}
	if got := artwork.FindFolderImage(dir, []string{"albumart*.*"}); filepath.Base(got) != "albumart_small.jpg" {
		t.Fatalf("FindFolderImage(albumart) = %q", got)
	}
	if got := artwork.FindFolderImage(dir, []string{"nothing.*"}); got != "" {
		t.Fatalf("no match = %q", got)
	}
	if got := artwork.FindFolderImage(filepath.Join(dir, "missing"), patterns); got != "" {
		t.Fatalf("missing dir = %q", got)
	}
	if got := artwork.MatchFolderImage([]string{"b.png", "FOLDER.PNG", "cover.gif"}, patterns); got != "FOLDER.PNG" {
		t.Fatalf("MatchFolderImage = %q", got)
	}
}

// An artist without an image falls back to the cover of its most recent album that has a
// usable one: an unusable cover of the newest album (corrupt file, nothing embedded) must
// not hide the older albums' covers.
func TestArtistFallbackSkipsUnusableAlbumCovers(t *testing.T) {
	f := setup(t)
	ctx := f.ctx
	mp3 := f.track(t, "林雨晴/夏日微风 (2021)/01 夏日微风.mp3")
	if err := f.st.SetArtistImagePath(ctx, mp3.AlbumArtistID, ""); err != nil {
		t.Fatal(err)
	}
	bad := filepath.Join(f.root, "林雨晴", "夏日微风 (2021)", "cover.jpg")
	if err := os.WriteFile(bad, []byte("not an image"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := f.st.SetAlbumCoverPath(ctx, mp3.AlbumID, bad); err != nil {
		t.Fatal(err)
	}
	if _, err := f.st.DB().W.ExecContext(ctx, `UPDATE albums SET cover_track_id = '' WHERE id = ?`, mp3.AlbumID); err != nil {
		t.Fatal(err)
	}
	orig, err := os.ReadFile(filepath.Join(f.root, "林雨晴", "城市夜雨 (2019)", "cover.jpg"))
	if err != nil {
		t.Fatal(err)
	}
	for _, size := range []int{0, 64} {
		img, err := f.art.Get(ctx, util.CoverArtID("ar", mp3.AlbumArtistID, 1), size)
		if err != nil {
			t.Fatalf("size %d: %v", size, err)
		}
		if size == 0 && !bytes.Equal(img.Data, orig) {
			t.Fatal("artist fallback did not use the older album's cover")
		}
	}
}

// A playlist mosaic must follow a changed fallback cover (the embedded picture used when
// the album's folder image is unusable): the disk cache key and the cover-art id version
// both have to change, otherwise clients and the cache keep serving the old mosaic.
func TestPlaylistMosaicFollowsFallbackCovers(t *testing.T) {
	f := setup(t)
	ctx := f.ctx
	u := &model.User{Username: "u", PasswordEnc: "synthetic"}
	if err := f.st.CreateUser(ctx, u); err != nil {
		t.Fatal(err)
	}
	cityDir := "林雨晴/城市夜雨 (2019)"
	var ids, flacs []string
	for _, rel := range []string{
		"林雨晴/夏日微风 (2021)/01 夏日微风.mp3",
		cityDir + "/01 城市夜雨.flac",
		"星野ミライ/雨上がりの空/01 雨上がりの空.m4a",
		"The Rainy Days/Blue Skies/01 Blue Skies.opus",
	} {
		ids = append(ids, f.track(t, rel).ID)
	}
	for _, n := range []string{"01 城市夜雨.flac", "02 霓虹.flac", "03 末班车.flac", "04 伞下.flac", "05 天亮以前.flac"} {
		flacs = append(flacs, cityDir+"/"+n)
	}
	pl := &model.Playlist{Name: "mix", OwnerID: u.ID}
	if err := f.st.CreatePlaylist(ctx, pl, ids); err != nil {
		t.Fatal(err)
	}
	// An unusable folder cover: the album falls back to its cover track's embedded picture.
	if err := os.WriteFile(filepath.Join(f.root, filepath.FromSlash(cityDir), "cover.jpg"), []byte("not an image"), 0o644); err != nil {
		t.Fatal(err)
	}
	sc := scanner.New(f.st, events.NewBus(), config.Default())
	embed := func(c color.RGBA) {
		t.Helper()
		img := image.NewRGBA(image.Rect(0, 0, 64, 64))
		for i := 0; i < len(img.Pix); i += 4 {
			img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = c.R, c.G, c.B, 0xff
		}
		var buf bytes.Buffer
		if err := png.Encode(&buf, img); err != nil {
			t.Fatal(err)
		}
		for _, rel := range flacs {
			if err := tags.WritePicture(filepath.Join(f.root, filepath.FromSlash(rel)), buf.Bytes()); err != nil {
				t.Fatal(err)
			}
		}
		time.Sleep(5 * time.Millisecond) // distinct updated_at
		if _, err := sc.RescanFiles(ctx, f.lib.ID, flacs); err != nil {
			t.Fatal(err)
		}
	}
	tile := func() (color.RGBA, string) {
		t.Helper()
		p, err := f.st.GetPlaylist(ctx, pl.ID)
		if err != nil {
			t.Fatal(err)
		}
		img, err := f.art.Get(ctx, p.CoverArt, 300)
		if err != nil {
			t.Fatal(err)
		}
		decoded, err := jpeg.Decode(bytes.NewReader(img.Data))
		if err != nil {
			t.Fatal(err)
		}
		return color.RGBAModel.Convert(decoded.At(225, 75)).(color.RGBA), p.CoverArt
	}
	embed(color.RGBA{R: 0xff})
	c1, v1 := tile()
	if c1.R < 0xc0 || c1.B > 0x40 {
		t.Fatalf("tile of the fallback cover should be red, got %v", c1)
	}
	embed(color.RGBA{B: 0xff})
	c2, v2 := tile()
	if c2.B < 0xc0 || c2.R > 0x40 {
		t.Fatalf("mosaic still shows the old fallback cover: %v", c2)
	}
	if v1 == v2 {
		t.Fatalf("playlist cover-art id did not change (%s) although a cover in its mosaic did", v1)
	}
}
