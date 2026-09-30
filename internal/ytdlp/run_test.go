package ytdlp

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"testing"
)

const testLink = "https://www.youtube.com/watch?v=synthetic01"

func TestBuildArgs(t *testing.T) {
	in := argsInput{url: testLink, out: "/work/out", cache: "/data/ytdlp/cache", ffmpeg: "/usr/bin/ffmpeg"}
	a := buildArgs(in)
	for _, want := range []string{"--ignore-config", "--no-plugin-dirs", "--no-playlist", "--extract-audio"} {
		if !slices.Contains(a, want) {
			t.Errorf("missing %s in %v", want, a)
		}
	}
	if i := slices.Index(a, "--use-extractors"); i < 0 || a[i+1] != "default,-generic" {
		t.Errorf("the generic extractor is not disabled: %v", a)
	}
	if a[len(a)-2] != "--" || a[len(a)-1] != testLink {
		t.Errorf("the link must come last, after --: %v", a[len(a)-2:])
	}
	for _, flag := range []string{"--cookies", "--audio-format", "--js-runtimes", "--yes-playlist"} {
		if slices.Contains(a, flag) {
			t.Errorf("unexpected %s without being asked for: %v", flag, a)
		}
	}

	in.cookies, in.format, in.playlist, in.jsName, in.jsPath = "/work/cookies.txt", FormatMP3, true, "quickjs", "/usr/bin/qjs"
	a = buildArgs(in)
	pairs := map[string]string{"--cookies": "/work/cookies.txt", "--audio-format": "mp3", "--js-runtimes": "quickjs:/usr/bin/qjs", "--playlist-items": "1:100"}
	for flag, want := range pairs {
		if i := slices.Index(a, flag); i < 0 || a[i+1] != want {
			t.Errorf("%s = %v, want %q", flag, a, want)
		}
	}
	if slices.Contains(a, "--no-playlist") {
		t.Error("--no-playlist with a playlist request")
	}
	// A link that looks like an option is still passed after "--".
	a = buildArgs(argsInput{url: "--exec=touch /tmp/x"})
	if a[len(a)-2] != "--" {
		t.Error("the link is not protected by --")
	}
}

func TestParserOutput(t *testing.T) {
	out := t.TempDir()
	audio := filepath.Join(out, "synthetic01.opus")
	for _, p := range []string{audio, filepath.Join(out, "synthetic01.jpg")} {
		if err := os.WriteFile(p, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	outside := filepath.Join(t.TempDir(), "outside.opus")
	_ = os.WriteFile(outside, []byte("x"), 0o600)

	var got []Progress
	p := &parser{out: out, progress: func(pr Progress) { got = append(got, pr) }}
	lines := []string{
		`[rainy-start]{"id":"synthetic01","title":"Synthetic Song","playlist_index":2,"n_entries":4}`,
		`[rainy-progress]500/1000/NA/250.5/2`,
		`[rainy-progress]NA/NA/NA/NA/NA`,
		`[rainy-postprocess]ExtractAudio`,
		`[rainy-item]` + mustJSON(t, map[string]any{
			"filepath": audio, "id": "synthetic01", "title": "Synthetic Song (Official Video)", "track": "Synthetic Song",
			"artists": []string{"Synthetic Artist", "Guest"}, "uploader": "Synthetic Artist - Topic", "album": "Synthetic Album",
			"track_number": 3, "release_year": 2024, "upload_date": "20240102", "webpage_url": testLink,
		}),
		`[rainy-item]` + mustJSON(t, map[string]any{"filepath": outside, "id": "escape"}),
		`[rainy-item]` + mustJSON(t, map[string]any{"filepath": filepath.Join(out, "missing.opus"), "id": "missing"}),
		`[rainy-item]not json`,
		`WARNING: something harmless`,
		`ERROR: [youtube] synthetic02: Video unavailable`,
	}
	for _, l := range lines {
		p.line(l)
	}
	if len(got) != 4 {
		t.Fatalf("progress updates %+v", got)
	}
	if got[0].Item != 2 || got[0].Items != 4 || got[0].Title != "Synthetic Song" {
		t.Errorf("start %+v", got[0])
	}
	if got[1].Fraction != 0.5 || got[1].Speed != 250.5 || got[1].ETA != 2 {
		t.Errorf("progress %+v", got[1])
	}
	if got[2].Fraction != -1 || got[3].Phase != PhaseProcessing {
		t.Errorf("unknown / processing %+v %+v", got[2], got[3])
	}
	items := p.result()
	if len(items) != 1 {
		t.Fatalf("items %+v (files outside the work dir and missing files must be ignored)", items)
	}
	it := items[0]
	if it.Title != "Synthetic Song" || it.Date != "2024" || it.TrackNumber != 3 || it.Thumbnail != filepath.Join(out, "synthetic01.jpg") {
		t.Errorf("item %+v", it)
	}
	wantTags := map[string][]string{
		"TITLE": {"Synthetic Song"}, "ARTIST": {"Synthetic Artist", "Guest"}, "ALBUM": {"Synthetic Album"},
		"DATE": {"2024"}, "TRACKNUMBER": {"3"}, "COMMENT": {testLink},
	}
	if tags := it.Tags(); !reflect.DeepEqual(tags, wantTags) {
		t.Errorf("tags %v, want %v", tags, wantTags)
	}
	if msg := p.errorMessage(); msg != "[youtube] synthetic02: Video unavailable" {
		t.Errorf("error message %q", msg)
	}
}

func TestItemFallbacks(t *testing.T) {
	out := t.TempDir()
	audio := filepath.Join(out, "BV1Synthetic.m4a")
	_ = os.WriteFile(audio, []byte("x"), 0o600)
	p := &parser{out: out, progress: func(Progress) {}}
	p.line(`[rainy-item]` + mustJSON(t, map[string]any{"filepath": audio, "id": "BV1Synthetic", "title": "Synthetic",
		"uploader": "Synthetic Uploader - Topic", "upload_date": "20250101", "webpage_url": "javascript:alert(1)"}))
	it := p.result()[0]
	if !reflect.DeepEqual(it.Artists, []string{"Synthetic Uploader"}) || it.Date != "2025-01-01" || it.WebpageURL != "" {
		t.Errorf("item %+v", it)
	}
}

// TestDownloadRunsYtdlp drives Download with a fake yt-dlp that checks its arguments and
// prints what the real one would.
func TestDownloadRunsYtdlp(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the fake yt-dlp is a shell script")
	}
	sh, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	s := newTestService(t)
	s.opt.FFmpegPath = sh // only looked up; the fake never calls it
	if err := os.MkdirAll(s.opt.Dir, 0o755); err != nil {
		t.Fatal(err)
	}
	fake := `#!/bin/sh
case "$*" in *--version*) echo 2026.08.19; exit 0;; esac
out=""; last=""; prev=""; cookies=""
for a in "$@"; do
  [ "$prev" = "--paths" ] && out="$a"
  [ "$prev" = "--cookies" ] && cookies="$a"
  prev="$a"; last="$a"
done
[ "$last" = "` + testLink + `" ] || { echo "ERROR: wrong link $last" >&2; exit 2; }
[ -z "$cookies" ] || { echo "ERROR: cookies passed without any stored" >&2; exit 2; }
echo '[rainy-start]{"id":"synthetic01","title":"Synthetic Song"}'
echo '[rainy-progress]10/20/NA/5/2' >&2
printf 'audio' > "$out/synthetic01.opus"
echo "[rainy-item]{\"filepath\": \"$out/synthetic01.opus\", \"id\": \"synthetic01\", \"title\": \"Synthetic Song\"}"
`
	if err := os.WriteFile(s.binaryPath(), []byte(fake), 0o755); err != nil {
		t.Fatal(err)
	}
	target, _ := ParseURL(testLink)
	var updates int
	items, err := s.Download(t.Context(), Request{Target: target, Dir: t.TempDir()}, func(Progress) { updates++ })
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Title != "Synthetic Song" || updates < 2 {
		t.Fatalf("items %+v, %d progress updates", items, updates)
	}

	// A failing run reports yt-dlp's error line.
	bad, _ := ParseURL("https://www.youtube.com/watch?v=synthetic02")
	_, err = s.Download(t.Context(), Request{Target: bad, Dir: t.TempDir()}, nil)
	if !errors.Is(err, ErrUpstream) || !strings.Contains(err.Error(), "wrong link") {
		t.Fatalf("err = %v", err)
	}
}

func TestDownloadNeedsYtdlp(t *testing.T) {
	s := newTestService(t)
	target, _ := ParseURL(testLink)
	if _, err := s.Download(t.Context(), Request{Target: target, Dir: t.TempDir()}, nil); !errors.Is(err, ErrNotInstalled) {
		t.Fatalf("err = %v, want ErrNotInstalled", err)
	}
	if err := s.Ready(t.Context()); !errors.Is(err, ErrNotInstalled) {
		t.Fatalf("Ready = %v", err)
	}
}

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
