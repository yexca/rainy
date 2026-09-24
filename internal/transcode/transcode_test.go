package transcode

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"rainy/internal/model"
)

func TestDecide(t *testing.T) {
	mp3 := &model.Track{Suffix: "mp3", Codec: "MPEG", Bitrate: 320}
	flac := &model.Track{Suffix: "flac", Codec: "FLAC", Bitrate: 900, BitDepth: 16}
	alac := &model.Track{Suffix: "m4a", Codec: "ALAC", Bitrate: 800}
	aacM4a := &model.Track{Suffix: "m4a", Codec: "AAC", Bitrate: 256}
	opus := &model.Track{Suffix: "opus", Codec: "Opus", Bitrate: 128}
	lowMP3 := &model.Track{Suffix: "mp3", Bitrate: 128}
	// As TagLib reports it: AAC in MP4 with a nominal 16-bit depth.
	taglibAAC := &model.Track{Suffix: "m4a", Codec: "mp4/aac", Bitrate: 256, BitDepth: 16}

	cases := []struct {
		name      string
		t         *model.Track
		format    string
		maxBR     int
		defFormat string
		defBR     int
		want      string
		wantBR    int
		transcode bool
	}{
		{"nothing requested", flac, "", 0, "mp3", 192, "raw", 0, false},
		{"raw never", flac, "raw", 128, "mp3", 192, "raw", 0, false},
		{"raw case-insensitive", flac, "RAW", 0, "mp3", 192, "raw", 0, false},
		{"explicit format default bitrate", flac, "mp3", 0, "opus", 192, "mp3", 192, true},
		{"explicit format with max", flac, "opus", 96, "mp3", 192, "opus", 96, true},
		{"explicit same format passthrough", mp3, "mp3", 0, "mp3", 192, "raw", 0, false},
		{"explicit same format over max", mp3, "mp3", 128, "mp3", 192, "mp3", 128, true},
		{"explicit same format under max", lowMP3, "mp3", 192, "mp3", 192, "raw", 0, false},
		{"aac in m4a passthrough", aacM4a, "aac", 0, "mp3", 192, "raw", 0, false},
		{"alac is not aac", alac, "aac", 0, "mp3", 192, "aac", 192, true},
		{"opus passthrough", opus, "opus", 0, "mp3", 192, "raw", 0, false},
		{"max above lossy bitrate", mp3, "", 320, "mp3", 192, "raw", 0, false},
		{"max below lossy bitrate", mp3, "", 128, "opus", 192, "opus", 128, true},
		{"lossless with max", flac, "", 1000, "aac", 192, "aac", 320, true},
		{"alac by codec", alac, "", 256, "mp3", 192, "mp3", 256, true},
		{"unknown format ignored", mp3, "flac", 0, "mp3", 192, "raw", 0, false},
		{"unknown format honours max", mp3, "wma", 96, "mp3", 192, "mp3", 96, true},
		{"invalid default format", flac, "", 128, "vorbis", 192, "mp3", 128, true},
		{"bitrate clamped low", flac, "mp3", 8, "mp3", 192, "mp3", MinBitRate, true},
		{"default bitrate zero", flac, "mp3", 0, "mp3", 0, "mp3", DefaultBitRate, true},
		{"negative max", mp3, "", -5, "mp3", 192, "raw", 0, false},
		{"nil track", nil, "mp3", 0, "mp3", 192, "raw", 0, false},
		{"taglib aac under max stays raw", taglibAAC, "", 320, "mp3", 192, "raw", 0, false},
		{"taglib aac over max", taglibAAC, "", 128, "mp3", 192, "mp3", 128, true},
	}
	for _, c := range cases {
		f, br, tr := Decide(c.t, c.format, c.maxBR, c.defFormat, c.defBR)
		if f != c.want || br != c.wantBR || tr != c.transcode {
			t.Errorf("%s: Decide = (%q, %d, %v), want (%q, %d, %v)", c.name, f, br, tr, c.want, c.wantBR, c.transcode)
		}
	}
}

func TestIsLossless(t *testing.T) {
	cases := map[*model.Track]bool{
		{Suffix: "flac"}:                   true,
		{Suffix: "WAV"}:                    true,
		{Suffix: "m4a", Codec: "ALAC"}:     true,
		{Suffix: "m4a", Codec: "AAC"}:      false,
		{Suffix: "mp3"}:                    false,
		{Suffix: "ogg", Codec: "Vorbis"}:   false,
		{Suffix: "mka", BitDepth: 24}:      true,
		{Suffix: "wv", Codec: "WavPack"}:   true,
		{Suffix: "opus", Codec: "Opus"}:    false,
		{Suffix: "dsf", Codec: "DSD"}:      true,
		{Suffix: "mp4", Codec: "PCM s16"}:  true,
		{Suffix: "wma", Codec: "WMA Pro"}:  false,
		{Suffix: "ape", Codec: "Monkey's"}: true,
		// TagLib reports a nominal 16-bit depth for AAC in MP4.
		{Suffix: "m4a", Codec: "mp4/aac", BitDepth: 16}:  false,
		{Suffix: "m4a", Codec: "mp4/alac", BitDepth: 16}: true,
		{Suffix: "ogg", Codec: "ogg/vorbis"}:             false,
	}
	for tr, want := range cases {
		if got := IsLossless(tr); got != want {
			t.Errorf("IsLossless(%+v) = %v", *tr, got)
		}
	}
}

func TestContentTypeSuffix(t *testing.T) {
	for f, ct := range map[string]string{"mp3": "audio/mpeg", "opus": "audio/ogg", "aac": "audio/aac", "x": "application/octet-stream"} {
		if got := ContentType(f); got != ct {
			t.Errorf("ContentType(%q) = %q", f, got)
		}
	}
	for f, s := range map[string]string{"mp3": "mp3", "opus": "opus", "aac": "aac", "raw": "", "flac": ""} {
		if got := Suffix(f); got != s {
			t.Errorf("Suffix(%q) = %q", f, got)
		}
	}
}

func TestArgs(t *testing.T) {
	args, err := Args("in put.flac", Options{Format: "mp3", BitRate: 999, Offset: 12.5})
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(args, " ")
	for _, want := range []string{"-ss 12.500 -i file:" + mustAbs(t, "in put.flac"), "-c:a libmp3lame", "-b:a 320k", "-f mp3", "-map 0:a:0", "-vn"} {
		if !strings.Contains(joined, want) {
			t.Errorf("args %q missing %q", joined, want)
		}
	}
	if args[len(args)-1] != "pipe:1" {
		t.Error("output must go to stdout")
	}
	if idx := slices.Index(args, "-ss"); idx < 0 || idx > slices.Index(args, "-i") {
		t.Error("-ss must come before -i for fast seeking")
	}

	args, _ = Args("x", Options{Format: "opus", BitRate: 96})
	joined = strings.Join(args, " ")
	if strings.Contains(joined, "-ss") || !strings.Contains(joined, "-c:a libopus -b:a 96k") || !strings.Contains(joined, "-f ogg") {
		t.Errorf("opus args: %s", joined)
	}
	args, _ = Args("x", Options{Format: "aac"})
	joined = strings.Join(args, " ")
	if !strings.Contains(joined, "-c:a aac -b:a 192k") || !strings.Contains(joined, "-f adts") {
		t.Errorf("aac args: %s", joined)
	}
	if _, err := Args("x", Options{Format: "flac"}); !errors.Is(err, ErrUnsupportedFormat) {
		t.Errorf("flac: %v", err)
	}
}

func TestParseVersion(t *testing.T) {
	if v := parseVersion("ffmpeg version 7.1-full_build-www.gyan.dev Copyright (c) 2000-2024\nbuilt with"); v != "7.1-full_build-www.gyan.dev" {
		t.Errorf("got %q", v)
	}
	if v := parseVersion("weird"); v != "weird" {
		t.Errorf("got %q", v)
	}
}

func TestTailBuffer(t *testing.T) {
	b := &tailBuffer{limit: 8}
	_, _ = b.Write([]byte("hello "))
	_, _ = b.Write([]byte("world"))
	if b.String() != "lo world" {
		t.Errorf("got %q", b.String())
	}
	n, _ := b.Write([]byte("0123456789abc"))
	if n != 13 || b.String() != "56789abc" {
		t.Errorf("got %q (%d)", b.String(), n)
	}
}

func TestUnavailable(t *testing.T) {
	s := New(filepath.Join(t.TempDir(), "no-such-ffmpeg"))
	if s.Available() || s.Version() != "" {
		t.Fatal("must be unavailable")
	}
	err := s.Stream(context.Background(), io.Discard, "x.flac", Options{Format: "mp3"})
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("got %v", err)
	}
	if New("").Path() != "ffmpeg" {
		t.Error("default path")
	}
}

// writeWAV writes a mono 16-bit PCM sine wave of the given length.
func writeWAV(t *testing.T, path string, seconds float64) {
	t.Helper()
	const rate = 44100
	n := int(seconds * rate)
	var buf bytes.Buffer
	le := binary.LittleEndian
	buf.WriteString("RIFF")
	_ = binary.Write(&buf, le, uint32(36+n*2))
	buf.WriteString("WAVEfmt ")
	_ = binary.Write(&buf, le, uint32(16))
	_ = binary.Write(&buf, le, uint16(1)) // PCM
	_ = binary.Write(&buf, le, uint16(1)) // mono
	_ = binary.Write(&buf, le, uint32(rate))
	_ = binary.Write(&buf, le, uint32(rate*2))
	_ = binary.Write(&buf, le, uint16(2))
	_ = binary.Write(&buf, le, uint16(16))
	buf.WriteString("data")
	_ = binary.Write(&buf, le, uint32(n*2))
	for i := range n {
		_ = binary.Write(&buf, le, int16(8000*math.Sin(2*math.Pi*440*float64(i)/rate)))
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
}

func requireFFmpeg(t *testing.T) *Service {
	t.Helper()
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not installed")
	}
	s := New("ffmpeg")
	if !s.Available() {
		t.Skip("ffmpeg not available")
	}
	return s
}

func TestStreamFormats(t *testing.T) {
	s := requireFFmpeg(t)
	if s.Version() == "" {
		t.Error("expected a version")
	}
	in := filepath.Join(t.TempDir(), "tone.wav")
	writeWAV(t, in, 2)
	magic := map[string]func([]byte) bool{
		"mp3":  func(b []byte) bool { return len(b) > 2 && b[0] == 0xFF && b[1]&0xE0 == 0xE0 },
		"opus": func(b []byte) bool { return bytes.HasPrefix(b, []byte("OggS")) },
		"aac":  func(b []byte) bool { return len(b) > 2 && b[0] == 0xFF && b[1]&0xF0 == 0xF0 },
	}
	for f, ok := range magic {
		var out bytes.Buffer
		if err := s.Stream(context.Background(), &out, in, Options{Format: f, BitRate: 96}); err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		if out.Len() < 1000 || !ok(out.Bytes()) {
			t.Errorf("%s: unexpected output (%d bytes, % x)", f, out.Len(), out.Bytes()[:min(8, out.Len())])
		}
	}

	// An offset shortens the output.
	var full, cut bytes.Buffer
	_ = s.Stream(context.Background(), &full, in, Options{Format: "mp3", BitRate: 128})
	if err := s.Stream(context.Background(), &cut, in, Options{Format: "mp3", BitRate: 128, Offset: 1.5}); err != nil {
		t.Fatal(err)
	}
	if cut.Len() == 0 || cut.Len() >= full.Len()*2/3 {
		t.Errorf("offset output %d vs full %d", cut.Len(), full.Len())
	}
}

func TestStreamErrorIncludesStderr(t *testing.T) {
	s := requireFFmpeg(t)
	bad := filepath.Join(t.TempDir(), "bad.flac")
	if err := os.WriteFile(bad, []byte("definitely not audio"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := s.Stream(context.Background(), io.Discard, bad, Options{Format: "mp3"})
	if err == nil || !strings.Contains(err.Error(), "ffmpeg") || len(err.Error()) < 30 {
		t.Fatalf("expected an ffmpeg error with stderr, got %v", err)
	}
}

// failingWriter fails after n bytes, like a disconnected HTTP client.
type failingWriter struct{ n int }

func (f *failingWriter) Write(p []byte) (int, error) {
	if f.n <= 0 {
		return 0, errors.New("client gone")
	}
	f.n -= len(p)
	return len(p), nil
}

func TestStreamStopsOnWriteErrorAndCancel(t *testing.T) {
	s := requireFFmpeg(t)
	in := filepath.Join(t.TempDir(), "long.wav")
	writeWAV(t, in, 30)

	start := time.Now()
	err := s.Stream(context.Background(), &failingWriter{n: 1000}, in, Options{Format: "mp3"})
	if err == nil || !strings.Contains(err.Error(), "client gone") {
		t.Fatalf("got %v", err)
	}
	if time.Since(start) > 10*time.Second {
		t.Error("ffmpeg was not stopped promptly")
	}

	ctx, cancel := context.WithCancel(context.Background())
	pr, pw := io.Pipe()
	done := make(chan error, 1)
	go func() { done <- s.Stream(ctx, pw, in, Options{Format: "mp3"}) }()
	// Read a little, then cancel while ffmpeg is blocked writing.
	if _, err := io.ReadFull(pr, make([]byte, 512)); err != nil {
		t.Fatal(err)
	}
	cancel()
	go func() { _, _ = io.Copy(io.Discard, pr) }()
	select {
	case err := <-done:
		if err == nil {
			t.Error("expected an error after cancellation")
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Stream did not return after cancel")
	}
	_ = pw.Close()
}

func mustAbs(t *testing.T, p string) string {
	t.Helper()
	abs, err := filepath.Abs(p)
	if err != nil {
		t.Fatal(err)
	}
	return abs
}

func TestInputURL(t *testing.T) {
	// Paths that ffmpeg would otherwise read as options or protocols stay plain files.
	for _, p := range []string{"-y.flac", "concat:a.flac|b.flac", "http:x.mp3"} {
		got := inputURL(p)
		if got != "file:"+mustAbs(t, p) {
			t.Errorf("inputURL(%q) = %q", p, got)
		}
	}
}
