package api

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"rainy/internal/artwork"
	"rainy/internal/lyrics"
	"rainy/internal/model"
	"rainy/internal/transcode"
)

// fixtureFile returns the bytes of a fixture track's file.
func (e *nativeEnv) fixtureFile(key string) []byte {
	e.t.Helper()
	b, err := os.ReadFile(filepath.Join(e.music, filepath.FromSlash(e.tr[key].Path)))
	if err != nil {
		e.t.Fatal(err)
	}
	return b
}

func TestNativeStreamRaw(t *testing.T) {
	e := newNativeEnv(t)
	nativeErrorCode(t, e.do("GET", "/stream/"+e.id("one"), "", nil), 401)
	want := e.fixtureFile("one")

	rec := e.do("GET", "/stream/"+e.id("one"), e.bobTok, nil)
	nativeExpect(t, rec, 200)
	h := rec.Header()
	if h.Get("Content-Type") != "audio/mpeg" || h.Get("Accept-Ranges") != "bytes" || h.Get("ETag") == "" ||
		h.Get("Last-Modified") == "" || !bytes.Equal(rec.Body.Bytes(), want) {
		t.Fatalf("raw stream headers %v, %d bytes", h, rec.Body.Len())
	}
	if h.Get("Content-Disposition") != "" {
		t.Error("streams must not be attachments")
	}

	// Range requests.
	req := nativeRequest(t, "GET", "/stream/"+e.id("one"), e.bobTok, nil)
	req.Header.Set("Range", "bytes=100-199")
	rec = nativeServe(t, e.h, req)
	nativeExpect(t, rec, 206)
	if rec.Header().Get("Content-Range") != "bytes 100-199/65536" || !bytes.Equal(rec.Body.Bytes(), want[100:200]) {
		t.Fatalf("range: %v", rec.Header())
	}
	req.Header.Set("Range", "bytes=-10")
	if rec = nativeServe(t, e.h, req); rec.Code != 206 || !bytes.Equal(rec.Body.Bytes(), want[len(want)-10:]) {
		t.Fatalf("suffix range %d", rec.Code)
	}
	req.Header.Set("Range", "bytes=999999-")
	if rec = nativeServe(t, e.h, req); rec.Code != 416 {
		t.Fatalf("unsatisfiable range %d", rec.Code)
	}

	// Conditional requests.
	etag := h.Get("ETag")
	req = nativeRequest(t, "GET", "/stream/"+e.id("one"), e.bobTok, nil)
	req.Header.Set("If-None-Match", etag)
	if rec = nativeServe(t, e.h, req); rec.Code != 304 {
		t.Fatalf("if-none-match %d", rec.Code)
	}

	// HEAD.
	rec = e.do("HEAD", "/stream/"+e.id("one"), e.bobTok, nil)
	if rec.Code != 200 || rec.Body.Len() != 0 || rec.Header().Get("Content-Length") != "65536" {
		t.Fatalf("head %d %v", rec.Code, rec.Header())
	}

	// format=raw with a bitrate never transcodes; WAV content type.
	rec = e.do("GET", "/stream/"+e.id("tone")+"?format=raw&bitrate=64", e.bobTok, nil)
	if rec.Code != 200 || rec.Header().Get("Content-Type") != "audio/wav" || !bytes.Equal(rec.Body.Bytes(), e.fixtureFile("tone")) {
		t.Fatalf("raw wav %d %v", rec.Code, rec.Header())
	}

	// Missing file / unknown track.
	nativeErrorCode(t, e.do("GET", "/stream/"+e.id("gone"), e.bobTok, nil), 404)
	nativeErrorCode(t, e.do("GET", "/stream/unknown", e.bobTok, nil), 404)
}

func TestNativeStreamTranscode(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not installed")
	}
	e := newNativeEnv(t)

	rec := e.do("GET", "/stream/"+e.id("tone")+"?format=mp3&bitrate=96", e.bobTok, nil)
	nativeExpect(t, rec, 200)
	h := rec.Header()
	if h.Get("Content-Type") != "audio/mpeg" || h.Get("Accept-Ranges") != "none" || h.Get("X-Content-Duration") != "3.00" || h.Get("Content-Length") != "" {
		t.Fatalf("transcode headers %v", h)
	}
	full := rec.Body.Len()
	if full < 2000 || rec.Body.Bytes()[0] != 0xFF {
		t.Fatalf("mp3 output %d bytes", full)
	}

	// Offset: shorter output and duration header.
	rec = e.do("GET", "/stream/"+e.id("tone")+"?format=mp3&bitrate=96&offset=2", e.bobTok, nil)
	nativeExpect(t, rec, 200)
	if rec.Header().Get("X-Content-Duration") != "1.00" || rec.Body.Len() == 0 || rec.Body.Len() >= full*2/3 {
		t.Fatalf("offset output %d vs %d (%v)", rec.Body.Len(), full, rec.Header())
	}

	// A bitrate cap on a lossless file transcodes to the default format (mp3).
	rec = e.do("GET", "/stream/"+e.id("tone")+"?bitrate=128", e.bobTok, nil)
	if rec.Code != 200 || rec.Header().Get("Content-Type") != "audio/mpeg" {
		t.Fatalf("maxBitRate transcode %d %v", rec.Code, rec.Header())
	}
	// Opus and AAC.
	if rec = e.do("GET", "/stream/"+e.id("tone")+"?format=opus", e.bobTok, nil); rec.Code != 200 ||
		rec.Header().Get("Content-Type") != "audio/ogg" || !bytes.HasPrefix(rec.Body.Bytes(), []byte("OggS")) {
		t.Fatalf("opus %d", rec.Code)
	}
	if rec = e.do("GET", "/stream/"+e.id("tone")+"?format=aac", e.bobTok, nil); rec.Code != 200 || rec.Header().Get("Content-Type") != "audio/aac" {
		t.Fatalf("aac %d", rec.Code)
	}

	// HEAD on a transcode answers headers only.
	if rec = e.do("HEAD", "/stream/"+e.id("tone")+"?format=mp3", e.bobTok, nil); rec.Code != 200 || rec.Body.Len() != 0 {
		t.Fatalf("head transcode %d", rec.Code)
	}

	// ffmpeg failing before any output → JSON error (the fixture mp3 is random bytes).
	rec = e.do("GET", "/stream/"+e.id("one")+"?format=opus", e.bobTok, nil)
	if code := nativeErrorCode(t, rec, 500); code != CodeInternal || !strings.HasPrefix(rec.Header().Get("Content-Type"), "application/json") {
		t.Fatalf("broken input: %s %v", code, rec.Header())
	}

	// Missing files are reported before ffmpeg runs.
	nativeErrorCode(t, e.do("GET", "/stream/"+e.id("gone")+"?format=mp3", e.bobTok, nil), 404)
}

func TestNativeStreamWithoutFFmpegFallsBackToRaw(t *testing.T) {
	e := newNativeEnv(t)
	e.app.Transcoder = transcode.New(filepath.Join(t.TempDir(), "no-ffmpeg"))
	e.h = New(e.app).Routes()
	rec := e.do("GET", "/stream/"+e.id("tone")+"?format=mp3", e.bobTok, nil)
	if rec.Code != 200 || rec.Header().Get("Content-Type") != "audio/wav" || !bytes.Equal(rec.Body.Bytes(), e.fixtureFile("tone")) {
		t.Fatalf("fallback %d %v", rec.Code, rec.Header())
	}
}

func TestNativeDownload(t *testing.T) {
	e := newNativeEnv(t)
	nativeErrorCode(t, e.do("GET", "/download/"+e.id("one"), "", nil), 401)
	if code := nativeErrorCode(t, e.do("GET", "/download/"+e.id("one"), e.bobTok, nil), 403); code != CodeForbidden {
		t.Fatal(code)
	}

	rec := e.do("GET", "/download/"+e.id("one"), e.aliceTok, nil)
	nativeExpect(t, rec, 200)
	if cd := rec.Header().Get("Content-Disposition"); cd != `attachment; filename="01 Rain One.mp3"` {
		t.Fatalf("content-disposition %q", cd)
	}
	if !bytes.Equal(rec.Body.Bytes(), e.fixtureFile("one")) || rec.Header().Get("Content-Type") != "audio/mpeg" {
		t.Fatal("download body")
	}
	// Resumable.
	req := nativeRequest(t, "GET", "/download/"+e.id("one"), e.aliceTok, nil)
	req.Header.Set("Range", "bytes=10-")
	if rec = nativeServe(t, e.h, req); rec.Code != 206 {
		t.Fatalf("range download %d", rec.Code)
	}
	nativeErrorCode(t, e.do("GET", "/download/"+e.id("gone"), e.aliceTok, nil), 404)
	nativeErrorCode(t, e.do("GET", "/download/nope", e.aliceTok, nil), 404)

	// Server-wide switch applies to everyone, admins included.
	s := e.app.Settings(t.Context())
	s.EnableDownloads = false
	if err := e.app.Store.SaveSettings(t.Context(), s); err != nil {
		t.Fatal(err)
	}
	nativeErrorCode(t, e.do("GET", "/download/"+e.id("one"), e.adminTok, nil), 403)
	nativeErrorCode(t, e.do("GET", "/download/album/"+e.blueSkies, e.adminTok, nil), 403)
}

func TestNativeDownloadUnicodeName(t *testing.T) {
	e := newNativeEnv(t)
	tr := e.tr["feature"]
	dir := filepath.Join(e.music, "Various Artists", "Hits")
	if err := os.WriteFile(filepath.Join(dir, "夏祭り.mp3"), []byte("abc"), 0o644); err != nil {
		t.Fatal(err)
	}
	tr.Path = "Various Artists/Hits/夏祭り.mp3"
	if err := e.app.Store.UpsertTracks(t.Context(), []model.Track{tr}); err != nil {
		t.Fatal(err)
	}
	rec := e.do("GET", "/download/"+tr.ID, e.aliceTok, nil)
	nativeExpect(t, rec, 200)
	if cd := rec.Header().Get("Content-Disposition"); cd != "attachment; filename*=utf-8''%E5%A4%8F%E7%A5%AD%E3%82%8A.mp3" {
		t.Fatalf("content-disposition %q", cd)
	}
}

func TestNativeAlbumZip(t *testing.T) {
	e := newNativeEnv(t)
	nativeErrorCode(t, e.do("GET", "/download/album/"+e.blueSkies, e.bobTok, nil), 403)
	nativeErrorCode(t, e.do("GET", "/download/album/nope", e.aliceTok, nil), 404)

	rec := e.do("GET", "/download/album/"+e.blueSkies, e.aliceTok, nil)
	nativeExpect(t, rec, 200)
	if rec.Header().Get("Content-Type") != "application/zip" ||
		rec.Header().Get("Content-Disposition") != `attachment; filename="The Rainy Days - Blue Skies.zip"` {
		t.Fatalf("zip headers %v", rec.Header())
	}
	zr, err := zip.NewReader(bytes.NewReader(rec.Body.Bytes()), int64(rec.Body.Len()))
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, f := range zr.File {
		names = append(names, f.Name)
		if f.Method != zip.Store {
			t.Errorf("%s: method %d", f.Name, f.Method)
		}
	}
	want := []string{
		"The Rainy Days - Blue Skies/CD1/01 Rain One.mp3",
		"The Rainy Days - Blue Skies/CD1/02 Umbrella.mp3",
		"The Rainy Days - Blue Skies/CD2/01 Puddles.mp3",
		"The Rainy Days - Blue Skies/cover.png",
	}
	if !slices.Equal(names, want) {
		t.Fatalf("zip entries %q", names)
	}
	rc, err := zr.File[1].Open()
	if err != nil {
		t.Fatal(err)
	}
	got, _ := io.ReadAll(rc)
	_ = rc.Close()
	if !bytes.Equal(got, e.fixtureFile("umbrella")) {
		t.Fatal("zip content")
	}

	// A single-folder album with a sidecar .lrc.
	rec = e.do("GET", "/download/album/"+e.tones, e.adminTok, nil)
	nativeExpect(t, rec, 200)
	zr, err = zip.NewReader(bytes.NewReader(rec.Body.Bytes()), int64(rec.Body.Len()))
	if err != nil {
		t.Fatal(err)
	}
	names = names[:0]
	for _, f := range zr.File {
		names = append(names, f.Name)
	}
	if !slices.Equal(names, []string{"Sine Wave - Tones/01 Tone.wav", "Sine Wave - Tones/01 Tone.lrc"}) {
		t.Fatalf("tones entries %q", names)
	}

	// No file on disk at all → 404 before any bytes are sent.
	for _, k := range []string{"feature", "guestTrack"} {
		if err := os.Remove(filepath.Join(e.music, filepath.FromSlash(e.tr[k].Path))); err != nil {
			t.Fatal(err)
		}
	}
	nativeErrorCode(t, e.do("GET", "/download/album/"+e.hits, e.aliceTok, nil), 404)
}

func TestNativeCover(t *testing.T) {
	e := newNativeEnv(t)
	nativeErrorCode(t, e.do("GET", "/cover/al-"+e.tones+"_1", "", nil), 401)
	if code := nativeErrorCode(t, e.do("GET", "/cover/al-nope_1?size=128", e.aliceTok, nil), 404); code != CodeNotFound {
		t.Fatal(code)
	}

	al := nativeJSON[albumDetail](t, e.do("GET", "/albums/"+e.blueSkies, e.aliceTok, nil), 200)
	if _, err := e.app.Artwork.Get(t.Context(), al.CoverArt, 0); errors.Is(err, artwork.ErrNotFound) {
		t.Skip("artwork service does not resolve album folder images yet")
	}
	rec := e.do("GET", "/cover/"+al.CoverArt, e.aliceTok, nil)
	nativeExpect(t, rec, 200)
	h := rec.Header()
	if !strings.HasPrefix(h.Get("Content-Type"), "image/") || h.Get("Cache-Control") != "public, max-age=31536000, immutable" || h.Get("ETag") == "" {
		t.Fatalf("cover headers %v", h)
	}
	req := nativeRequest(t, "GET", "/cover/"+al.CoverArt, e.aliceTok, nil)
	req.Header.Set("If-None-Match", h.Get("ETag"))
	if rec = nativeServe(t, e.h, req); rec.Code != 304 {
		t.Fatalf("conditional cover %d", rec.Code)
	}
	// Unversioned ids must be revalidated.
	rec = e.do("GET", "/cover/al-"+e.blueSkies+"?size=100", e.aliceTok, nil)
	nativeExpect(t, rec, 200)
	if rec.Header().Get("Cache-Control") != "private, no-cache" {
		t.Fatalf("unversioned cache-control %q", rec.Header().Get("Cache-Control"))
	}
}

func TestNativeLyrics(t *testing.T) {
	e := newNativeEnv(t)
	nativeErrorCode(t, e.do("GET", "/lyrics/"+e.id("tone"), "", nil), 401)
	l := nativeJSON[lyrics.Lyrics](t, e.do("GET", "/lyrics/"+e.id("tone"), e.aliceTok, nil), 200)
	if l.Source != "lrc" || !l.Synced || len(l.Lines) != 3 || l.Lines[0] != (lyrics.Line{Start: 500, Text: "first line"}) || l.Raw != nativeToneLRC {
		t.Fatalf("lrc lyrics %+v", l)
	}
	l = nativeJSON[lyrics.Lyrics](t, e.do("GET", "/lyrics/"+e.id("one"), e.aliceTok, nil), 200)
	if l.Source != "embedded" || l.Synced || len(l.Lines) != 2 || l.Lines[1].Start != -1 {
		t.Fatalf("embedded lyrics %+v", l)
	}
	rec := e.do("GET", "/lyrics/"+e.id("umbrella"), e.aliceTok, nil)
	if l = nativeJSON[lyrics.Lyrics](t, rec, 200); l.Source != "none" || !strings.Contains(rec.Body.String(), `"lines":[]`) {
		t.Fatalf("no lyrics %s", rec.Body.String())
	}
	// Embedded lyrics are still served when the file itself is gone.
	nativeExpect(t, e.do("GET", "/lyrics/"+e.id("gone"), e.aliceTok, nil), 200)
	nativeErrorCode(t, e.do("GET", "/lyrics/nope", e.aliceTok, nil), 404)
}

func TestSanitizeFileName(t *testing.T) {
	cases := map[string]string{
		"AC/DC: Live?":        "AC_DC_ Live_",
		"  .hidden. ":         "hidden",
		"":                    "_",
		"...":                 "_",
		"tab\there":           "tab_here",
		"夏祭り":                 "夏祭り",
		`a<b>c"d\e|f*g`:       "a_b_c_d_e_f_g",
		"normal - name.flac":  "normal - name.flac",
		"bad\xffutf8":         "bad_utf8",
		"trailing dots...mp3": "trailing dots...mp3",
	}
	for in, want := range cases {
		if got := sanitizeFileName(in); got != want {
			t.Errorf("sanitizeFileName(%q) = %q, want %q", in, got, want)
		}
	}
	long := strings.Repeat("雨", 100) + ".flac" // 305 bytes
	got := sanitizeFileName(long)
	if len(got) > maxZipNameBytes || !strings.HasSuffix(got, ".flac") || !strings.HasPrefix(got, "雨") {
		t.Fatalf("long name %q (%d bytes)", got, len(got))
	}
}

func TestCommonDirAndZipNames(t *testing.T) {
	cases := []struct {
		dirs []string
		want string
	}{
		{nil, ""},
		{[]string{"a/b/CD1", "a/b/CD2"}, "a/b"},
		{[]string{"a/b", "a/b"}, "a/b"},
		{[]string{"a/bc", "a/bd"}, "a"},
		{[]string{"", "a"}, ""},
		{[]string{"x/y"}, "x/y"},
	}
	for _, c := range cases {
		if got := commonDir(c.dirs); got != c.want {
			t.Errorf("commonDir(%q) = %q, want %q", c.dirs, got, c.want)
		}
	}
	if got := zipRelName("a/b", "a/b/CD1/01: x.mp3"); got != "CD1/01_ x.mp3" {
		t.Errorf("zipRelName = %q", got)
	}
	n := zipNames{}
	if a, b, c := n.unique("x/a.mp3"), n.unique("x/A.mp3"), n.unique("x/a.mp3"); a != "x/a.mp3" || b != "x/A (2).mp3" || c != "x/a (3).mp3" {
		t.Errorf("unique: %q %q %q", a, b, c)
	}
}

func TestSnapCoverSize(t *testing.T) {
	for in, want := range map[int]int{-1: 0, 0: 0, 1: 64, 64: 64, 65: 128, 300: 512, 1024: 1024, 5000: 2048} {
		if got := snapCoverSize(in); got != want {
			t.Errorf("snapCoverSize(%d) = %d, want %d", in, got, want)
		}
	}
}

// TestNativeStreamOffsetForcesTranscode: a file already in the requested format is served
// raw, but a request with ?offset can only be honoured by a transcode.
func TestNativeStreamOffsetForcesTranscode(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not installed")
	}
	e := newNativeEnv(t)
	// Turn the "feature" track into a real (3 s) mp3.
	var mp3 bytes.Buffer
	toneAbs := filepath.Join(e.music, filepath.FromSlash(e.tr["tone"].Path))
	if err := e.app.Transcoder.Stream(t.Context(), &mp3, toneAbs, transcode.Options{Format: "mp3", BitRate: 128}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(e.music, filepath.FromSlash(e.tr["feature"].Path)), mp3.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	id := e.id("feature")

	rec := e.do("GET", "/stream/"+id+"?format=mp3", e.bobTok, nil)
	if rec.Code != 200 || rec.Header().Get("Accept-Ranges") != "bytes" || !bytes.Equal(rec.Body.Bytes(), mp3.Bytes()) {
		t.Fatalf("same format must be served raw: %d %v", rec.Code, rec.Header())
	}
	rec = e.do("GET", "/stream/"+id+"?format=mp3&offset=2", e.bobTok, nil)
	nativeExpect(t, rec, 200)
	if rec.Header().Get("Accept-Ranges") != "none" || rec.Header().Get("Content-Type") != "audio/mpeg" ||
		rec.Body.Len() == 0 || rec.Body.Len() >= mp3.Len() {
		t.Fatalf("offset must transcode: %v, %d bytes (full %d)", rec.Header(), rec.Body.Len(), mp3.Len())
	}
	// Without a requested format an offset does not force anything (raw files seek by Range).
	rec = e.do("GET", "/stream/"+id+"?offset=2", e.bobTok, nil)
	if rec.Code != 200 || rec.Header().Get("Accept-Ranges") != "bytes" {
		t.Fatalf("raw with offset: %d %v", rec.Code, rec.Header())
	}
}

// TestNativeStreamTranscodeClientDisconnect: when the client goes away mid-stream the
// handler returns promptly, which means ffmpeg was killed and reaped (no zombie).
func TestNativeStreamTranscodeClientDisconnect(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not installed")
	}
	e := newNativeEnv(t)
	// Five minutes of audio: far more output than socket buffers hold.
	if err := os.WriteFile(filepath.Join(e.music, filepath.FromSlash(e.tr["tone"].Path)), nativeWAV(300), 0o644); err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		e.h.ServeHTTP(w, r)
		close(done)
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(t.Context())
	req, _ := http.NewRequestWithContext(ctx, "GET", srv.URL+"/stream/"+e.id("tone")+"?format=mp3&bitrate=320", nil)
	req.Header.Set("Authorization", "Bearer "+e.bobTok)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("status %d", resp.StatusCode)
	}
	if _, err := io.ReadFull(resp.Body, make([]byte, 4096)); err != nil {
		t.Fatal(err)
	}
	cancel()
	_ = resp.Body.Close()
	select {
	case <-done:
	case <-time.After(15 * time.Second):
		t.Fatal("stream handler (and ffmpeg) still running after the client disconnected")
	}
}
