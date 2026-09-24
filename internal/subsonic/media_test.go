package subsonic

import (
	"archive/zip"
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"rainy/internal/transcode"
)

func TestStreamRange(t *testing.T) {
	f := newFixture(t)
	req := httptest.NewRequest(http.MethodGet, "/stream?"+merge(f.aliceParams(), "id", "track1").Encode(), nil)
	req.Header.Set("Range", "bytes=100-199")
	rec := httptest.NewRecorder()
	f.handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusPartialContent {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	if got := rec.Header().Get("Content-Range"); got != "bytes 100-199/4096" {
		t.Fatalf("Content-Range %q", got)
	}
	if got := rec.Header().Get("Content-Type"); got != "audio/mpeg" {
		t.Fatalf("Content-Type %q", got)
	}
	if !bytes.Equal(rec.Body.Bytes(), f.fileData[100:200]) {
		t.Fatal("range body mismatch")
	}

	// Full request, format=raw, and a maxBitRate above the file's bit rate: original file.
	for _, kv := range [][]string{{"id", "track1"}, {"id", "track1", "format", "raw"}, {"id", "track1", "maxBitRate", "320"}} {
		rec = f.get("stream", merge(f.aliceParams(), kv...))
		if rec.Code != http.StatusOK || !bytes.Equal(rec.Body.Bytes(), f.fileData) {
			t.Fatalf("%v: status %d, %d bytes", kv, rec.Code, rec.Body.Len())
		}
		if rec.Header().Get("Accept-Ranges") != "bytes" {
			t.Fatalf("%v: missing Accept-Ranges", kv)
		}
	}

	// HEAD
	req = httptest.NewRequest(http.MethodHead, "/stream?"+merge(f.aliceParams(), "id", "track1").Encode(), nil)
	rec = httptest.NewRecorder()
	f.handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || rec.Header().Get("Content-Length") != "4096" || rec.Body.Len() != 0 {
		t.Fatalf("HEAD: %d %q %d", rec.Code, rec.Header().Get("Content-Length"), rec.Body.Len())
	}
}

func TestStreamErrors(t *testing.T) {
	f := newFixture(t)
	p := f.aliceParams()
	// track2 has no file on disk.
	if code := f.failCode("stream", p, "id", "track2"); code != codeNotFound {
		t.Fatalf("missing file: %d", code)
	}
	if code := f.failCode("stream", p, "id", "nope"); code != codeNotFound {
		t.Fatalf("unknown id: %d", code)
	}
	if code := f.failCode("stream", p); code != codeMissingParam {
		t.Fatalf("no id: %d", code)
	}
	f.exec(`UPDATE tracks SET missing = 1 WHERE id = 'track1'`)
	if code := f.failCode("stream", p, "id", "track1"); code != codeNotFound {
		t.Fatalf("missing track: %d", code)
	}
}

func TestStreamPathTraversalRejected(t *testing.T) {
	f := newFixture(t)
	outside := filepath.Join(t.TempDir(), "secret.mp3")
	if err := os.WriteFile(outside, []byte("secret"), 0o644); err != nil {
		t.Fatal(err)
	}
	f.exec(`UPDATE tracks SET path = '../../secret.mp3' WHERE id = 'track1'`)
	if code := f.failCode("stream", f.aliceParams(), "id", "track1"); code != codeNotFound {
		t.Fatalf("traversal: %d", code)
	}
}

func TestDownload(t *testing.T) {
	f := newFixture(t)
	p := f.aliceParams()
	rec := f.get("download", merge(p, "id", "track1"))
	if rec.Code != http.StatusOK || !bytes.Equal(rec.Body.Bytes(), f.fileData) {
		t.Fatalf("download: %d", rec.Code)
	}
	if cd := rec.Header().Get("Content-Disposition"); !strings.HasPrefix(cd, "attachment") || !strings.Contains(cd, "01 Rain Song.mp3") {
		t.Fatalf("Content-Disposition %q", cd)
	}

	// Album zip (track2's file is absent: the zip stops after the entries it could add).
	rec = f.get("download", merge(p, "id", albumSkies))
	if rec.Header().Get("Content-Type") != "application/zip" {
		t.Fatalf("album zip content type %q", rec.Header().Get("Content-Type"))
	}
	// A complete album zip: remove the missing track from the album first.
	f.exec(`UPDATE tracks SET album_id = 'other' WHERE id = 'track2'`)
	rec = f.get("download", merge(p, "id", albumWet))
	zr, err := zip.NewReader(bytes.NewReader(rec.Body.Bytes()), int64(rec.Body.Len()))
	if err != nil {
		t.Fatalf("zip: %v", err)
	}
	if len(zr.File) != 1 || zr.File[0].Name != "01 Rain Song.mp3" {
		t.Fatalf("zip entries %v", zr.File)
	}
	rc, _ := zr.File[0].Open()
	data, _ := io.ReadAll(rc)
	_ = rc.Close()
	if !bytes.Equal(data, f.fileData) {
		t.Fatal("zip content mismatch")
	}

	// Permission and setting.
	f.exec(`UPDATE users SET can_download = 0 WHERE username = 'bob'`)
	if code := f.failCode("download", creds("bob", "bobpass"), "id", "track1"); code != codeNotAuthorized {
		t.Fatalf("no download permission: %d", code)
	}
	f.exec(`INSERT INTO settings (key, value) VALUES ('enableDownloads', 'false')`)
	if code := f.failCode("download", p, "id", "track1"); code != codeNotAuthorized {
		t.Fatalf("downloads disabled: %d", code)
	}
	// Streaming is unaffected.
	if rec := f.get("stream", merge(p, "id", "track1")); rec.Code != http.StatusOK {
		t.Fatalf("stream with downloads disabled: %d", rec.Code)
	}
}

func TestCoverArt(t *testing.T) {
	f := newFixture(t)
	rec := f.get("getCoverArt", merge(f.aliceParams(), "id", "al-"+albumWet+"_1", "size", "64"))
	if rec.Code != http.StatusOK || !strings.HasPrefix(rec.Header().Get("Content-Type"), "image/") || rec.Body.Len() == 0 {
		t.Fatalf("cover: %d %q", rec.Code, rec.Header().Get("Content-Type"))
	}
	rec = f.get("getCoverArt", merge(f.aliceParams(), "id", "does-not-exist"))
	if rec.Code != http.StatusOK || !strings.HasPrefix(rec.Header().Get("Content-Type"), "image/") {
		t.Fatalf("placeholder: %d %q", rec.Code, rec.Header().Get("Content-Type"))
	}
	if code := f.failCode("getCoverArt", f.aliceParams()); code != codeMissingParam {
		t.Fatalf("no id: %d", code)
	}
}

func TestLyrics(t *testing.T) {
	f := newFixture(t)
	p := f.aliceParams()
	env := f.ok("getLyrics", p, "artist", "the rainy days", "title", "rain song")
	if v := jpath(t, env, "lyrics", "value"); v != "Hello rain\nGoodbye sun" {
		t.Fatalf("lyrics %q", v)
	}
	env = f.ok("getLyrics", p, "artist", "Nobody", "title", "Nothing")
	if v := jpath(t, env, "lyrics", "value"); v != "" {
		t.Fatalf("no lyrics %q", v)
	}
	env = f.ok("getLyricsBySongId", p, "id", "track2")
	if n := length(t, jpath(t, env, "lyricsList", "structuredLyrics")); n != 0 {
		t.Fatalf("track2 lyrics %d", n)
	}
	// A sidecar .lrc wins over embedded lyrics.
	lrc := filepath.Join(f.music, "The Rainy Days", "Wet Streets", "01 Rain Song.lrc")
	if err := os.WriteFile(lrc, []byte("[00:02.00]From sidecar\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	env = f.ok("getLyricsBySongId", p, "id", "track1")
	if jpath(t, env, "lyricsList", "structuredLyrics", 0, "line", 0, "value") != "From sidecar" ||
		jpath(t, env, "lyricsList", "structuredLyrics", 0, "line", 0, "start") != float64(2000) {
		t.Fatalf("sidecar lyrics: %v", env)
	}
}

// wavSilence returns a 16-bit mono PCM WAV of the given length.
func wavSilence(seconds int) []byte {
	const rate = 8000
	n := rate * 2 * seconds
	var b bytes.Buffer
	le := func(v uint32, size int) {
		for i := 0; i < size; i++ {
			b.WriteByte(byte(v >> (8 * i)))
		}
	}
	b.WriteString("RIFF")
	le(uint32(36+n), 4)
	b.WriteString("WAVEfmt ")
	le(16, 4)
	le(1, 2) // PCM
	le(1, 2) // mono
	le(rate, 4)
	le(rate*2, 4)
	le(2, 2)
	le(16, 2)
	b.WriteString("data")
	le(uint32(n), 4)
	b.Write(make([]byte, n))
	return b.Bytes()
}

func TestStreamTranscode(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg not available")
	}
	f := newFixture(t)
	f.app.Transcoder = transcode.New(ffmpeg)
	rel := "The Rainy Days/Wet Streets/01 Rain Song.wav"
	if err := os.WriteFile(filepath.Join(f.music, filepath.FromSlash(rel)), wavSilence(3), 0o644); err != nil {
		t.Fatal(err)
	}
	f.exec(`UPDATE tracks SET path = ?, suffix = 'wav', duration = 3, bitrate = 128 WHERE id = 'track1'`, rel)

	rec := f.get("stream", merge(f.aliceParams(), "id", "track1", "format", "mp3", "maxBitRate", "64", "estimateContentLength", "true"))
	if rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != "audio/mpeg" {
		t.Fatalf("transcode: %d %q %s", rec.Code, rec.Header().Get("Content-Type"), rec.Body.String())
	}
	if cl := rec.Header().Get("Content-Length"); cl != strconv.Itoa(rec.Body.Len()) {
		t.Fatalf("Content-Length %s but %d bytes", cl, rec.Body.Len())
	}
	// timeOffset shortens the output.
	full := f.get("stream", merge(f.aliceParams(), "id", "track1", "format", "mp3", "maxBitRate", "64"))
	part := f.get("stream", merge(f.aliceParams(), "id", "track1", "format", "mp3", "maxBitRate", "64", "timeOffset", "2"))
	if part.Body.Len() == 0 || part.Body.Len() >= full.Body.Len() {
		t.Fatalf("timeOffset: %d vs %d bytes", part.Body.Len(), full.Body.Len())
	}
}

// A mono source cannot be encoded to Opus above 256 kbps; the request must still succeed.
func TestStreamTranscodeOpusHighBitRate(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg not available")
	}
	f := newFixture(t)
	f.app.Transcoder = transcode.New(ffmpeg)
	rel := "The Rainy Days/Wet Streets/01 Rain Song.wav"
	if err := os.WriteFile(filepath.Join(f.music, filepath.FromSlash(rel)), wavSilence(2), 0o644); err != nil {
		t.Fatal(err)
	}
	f.exec(`UPDATE tracks SET path = ?, suffix = 'wav', duration = 2, bitrate = 128, channels = 2 WHERE id = 'track1'`, rel)
	rec := f.get("stream", merge(f.aliceParams(), "id", "track1", "format", "opus", "maxBitRate", "320", "estimateContentLength", "true"))
	if rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != "audio/ogg" {
		t.Fatalf("opus transcode: %d %q %s", rec.Code, rec.Header().Get("Content-Type"), rec.Body.String())
	}
	if cl := rec.Header().Get("Content-Length"); cl != strconv.Itoa(rec.Body.Len()) {
		t.Fatalf("Content-Length %s but %d bytes", cl, rec.Body.Len())
	}
	if !bytes.HasPrefix(rec.Body.Bytes(), []byte("OggS")) {
		t.Fatalf("not an Ogg stream: %q", rec.Body.Bytes()[:min(16, rec.Body.Len())])
	}
}

func TestEncoderBitRate(t *testing.T) {
	for _, tc := range []struct {
		format string
		in     int
		want   int
	}{
		{transcode.FormatOpus, 320, 256},
		{transcode.FormatOpus, 510, 256},
		{transcode.FormatOpus, 96, 96},
		{transcode.FormatMP3, 320, 320},
		{transcode.FormatAAC, 256, 256},
	} {
		if got := encoderBitRate(tc.format, tc.in); got != tc.want {
			t.Errorf("encoderBitRate(%s, %d) = %d, want %d", tc.format, tc.in, got, tc.want)
		}
	}
}

// An encoder overshooting the estimated Content-Length is cut at exactly that length.
func TestCappedWriter(t *testing.T) {
	var buf bytes.Buffer
	cw := &cappedWriter{w: &buf, limit: 10}
	if n, err := cw.Write([]byte("0123")); n != 4 || err != nil {
		t.Fatalf("first write: %d %v", n, err)
	}
	n, err := cw.Write([]byte("456789abcd"))
	if n != 6 || !errors.Is(err, errLimitReached) {
		t.Fatalf("overflowing write: %d %v", n, err)
	}
	if n, err := cw.Write([]byte("x")); n != 0 || !errors.Is(err, errLimitReached) {
		t.Fatalf("write after limit: %d %v", n, err)
	}
	if buf.String() != "0123456789" {
		t.Fatalf("body %q", buf.String())
	}
	// io.Copy (as used by the transcoder) stops with the sentinel error.
	buf.Reset()
	_, err = io.Copy(&cappedWriter{w: &buf, limit: 5}, strings.NewReader(strings.Repeat("z", 100)))
	if !errors.Is(err, errLimitReached) || buf.Len() != 5 {
		t.Fatalf("io.Copy: %v, %d bytes", err, buf.Len())
	}
}
