package lyrics

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/encoding/simplifiedchinese"

	"rainy/internal/model"
)

func TestParseLRC(t *testing.T) {
	text := string(rune(0xFEFF)) + "[ti:Song]\r\n[ar:Artist]\r\n[la:ja]\r\n[length: 03:20]\r\n" +
		"[00:01.00]first\r\n" +
		"[00:12.34][01:02.5]chorus  line\r\n" +
		"[00:05.123]<00:05.123>word <00:05.600>by <00:06.000>word\r\n" +
		"[00:20:50]colon fraction\r\n" +
		"[00:30]\r\n" +
		"[02:00.00] \r\n" +
		"stray untimed line\r\n"
	l := Parse(text)
	if !l.Synced || l.Source != SourceEmbedded || l.Lang != "ja" || l.Offset != 0 {
		t.Fatalf("meta: %+v", l)
	}
	want := []Line{
		{1000, "first"},
		{5123, "word by word"},
		{12340, "chorus  line"},
		{20500, "colon fraction"},
		{30000, ""},
		{62500, "chorus  line"},
		{120000, ""},
	}
	if !reflect.DeepEqual(l.Lines, want) {
		t.Fatalf("lines:\n got %+v\nwant %+v", l.Lines, want)
	}
	if l.Raw[0] == 0xEF || l.Raw[:4] != "[ti:" {
		t.Error("raw must be kept without BOM")
	}
}

func TestParseOffset(t *testing.T) {
	l := Parse("[offset:+500]\n[00:01.00]a\n[00:00.20]b\n")
	if l.Offset != 500 || !reflect.DeepEqual(l.Lines, []Line{{0, "b"}, {500, "a"}}) {
		t.Fatalf("%+v", l)
	}
	l = Parse("[offset:-250]\n[00:01.00]a")
	if l.Offset != -250 || l.Lines[0].Start != 1250 {
		t.Fatalf("%+v", l)
	}
	l = Parse("[offset:abc]\n[00:01.00]a")
	if l.Offset != 0 || l.Lines[0].Start != 1000 {
		t.Fatalf("%+v", l)
	}
}

func TestParseMillisAndBadTags(t *testing.T) {
	l := Parse("[01:02.345]ms\n[00:61.00]bad seconds\n[1:02.3]tenths\n[100:00.00]long")
	want := []Line{{62300, "tenths"}, {62345, "ms"}, {6000000, "long"}}
	// "[00:61.00]" is not a valid time: the line is dropped from synced output (but keeps no stamp).
	if !l.Synced || !reflect.DeepEqual(l.Lines, want) {
		t.Fatalf("got %+v", l.Lines)
	}
}

func TestParsePlain(t *testing.T) {
	l := Parse("\n\nLine one\r\nLine two\n\n\n[Chorus]\nLa la <not a stamp>\n\n")
	if l.Synced || l.Source != SourceEmbedded {
		t.Fatalf("%+v", l)
	}
	want := []Line{{-1, "Line one"}, {-1, "Line two"}, {-1, ""}, {-1, "[Chorus]"}, {-1, "La la <not a stamp>"}}
	if !reflect.DeepEqual(l.Lines, want) {
		t.Fatalf("got %+v", l.Lines)
	}
}

func TestParseEmpty(t *testing.T) {
	for _, s := range []string{"", "   \n\n", "[ar:Only]\n[ti:Meta]"} {
		l := Parse(s)
		if l.Synced || len(l.Lines) != 0 || l.Lines == nil || l.Source != SourceNone {
			t.Errorf("%q: %+v", s, l)
		}
	}
	b, _ := json.Marshal(Parse(""))
	if string(b) != `{"synced":false,"lines":[],"source":"none","raw":"","offset":0,"lang":""}` {
		t.Errorf("json %s", b)
	}
}

func TestLrcPath(t *testing.T) {
	if got := LrcPath(filepath.Join("a", "b.c", "song.flac")); got != filepath.Join("a", "b.c", "song.lrc") {
		t.Errorf("got %q", got)
	}
	if got := LrcPath("noext"); got != "noext.lrc" {
		t.Errorf("got %q", got)
	}
}

func TestLoad(t *testing.T) {
	dir := t.TempDir()
	audio := filepath.Join(dir, "song.flac")
	tr := &model.Track{Lyrics: "embedded plain"}

	// Embedded only.
	l, err := Load(tr, audio)
	if err != nil || l.Source != SourceEmbedded || l.Synced || l.Lines[0].Text != "embedded plain" {
		t.Fatalf("embedded: %+v %v", l, err)
	}
	// Sidecar wins.
	if err := os.WriteFile(filepath.Join(dir, "song.lrc"), []byte("[00:01.00]from lrc"), 0o644); err != nil {
		t.Fatal(err)
	}
	l, err = Load(tr, audio)
	if err != nil || l.Source != SourceLRC || !l.Synced || l.Lines[0].Text != "from lrc" {
		t.Fatalf("sidecar: %+v %v", l, err)
	}
	// Empty sidecar falls back to embedded.
	if err := os.WriteFile(filepath.Join(dir, "song.lrc"), []byte("  \n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if l, _ = Load(tr, audio); l.Source != SourceEmbedded {
		t.Fatalf("empty sidecar: %+v", l)
	}
	// Nothing.
	l, err = Load(&model.Track{}, filepath.Join(dir, "other.mp3"))
	if err != nil || l.Source != SourceNone || l.Lines == nil || len(l.Lines) != 0 {
		t.Fatalf("none: %+v %v", l, err)
	}
	if l, _ = Load(nil, ""); l.Source != SourceNone {
		t.Fatal("nil track")
	}
	// A directory named like the sidecar is ignored.
	if err := os.Mkdir(filepath.Join(dir, "dir.lrc"), 0o755); err != nil {
		t.Fatal(err)
	}
	if l, _ = Load(&model.Track{}, filepath.Join(dir, "dir.mp3")); l.Source != SourceNone {
		t.Fatal("directory sidecar")
	}
	// Oversized sidecars are skipped.
	big := make([]byte, maxSidecarSize+10)
	for i := range big {
		big[i] = 'a'
	}
	if err := os.WriteFile(filepath.Join(dir, "big.lrc"), big, 0o644); err != nil {
		t.Fatal(err)
	}
	if l, _ = Load(&model.Track{Lyrics: "x"}, filepath.Join(dir, "big.mp3")); l.Source != SourceEmbedded {
		t.Fatal("oversized sidecar")
	}
}

func TestDecodeText(t *testing.T) {
	if got := DecodeText([]byte("\xEF\xBB\xBFhello")); got != "hello" {
		t.Errorf("utf8 bom: %q", got)
	}
	if got := DecodeText([]byte{0xFF, 0xFE, 'h', 0, 0x28, 0x96}); got != "h"+string(rune(0x9628)) {
		t.Errorf("utf16le: %q", got)
	}
	if got := DecodeText([]byte{0xFE, 0xFF, 0, 'h', 0, 'i'}); got != "hi" {
		t.Errorf("utf16be: %q", got)
	}
	gbk, _ := simplifiedchinese.GBK.NewEncoder().String("[00:01.00]下雨天的歌词")
	if got := DecodeText([]byte(gbk)); got != "[00:01.00]下雨天的歌词" {
		t.Errorf("gbk: %q", got)
	}
	sjis, _ := japanese.ShiftJIS.NewEncoder().String("雨上がりの空")
	if got := DecodeText([]byte(sjis)); got != "雨上がりの空" {
		t.Errorf("shift-jis: %q", got)
	}
	if got := DecodeText([]byte("plain ascii")); got != "plain ascii" {
		t.Errorf("ascii: %q", got)
	}
}

func TestParseUntimedContinuationLines(t *testing.T) {
	// Untimed lines after a timed line (translations, wrapped text) keep its time; untimed
	// text before the first timed line and blank lines are dropped.
	l := Parse("Lyrics by someone\n[00:01.00]one\n一\n\n[00:02.00][00:04.00]chorus\n副歌\n[00:03.00]three")
	want := []Line{{1000, "one"}, {1000, "一"}, {2000, "chorus"}, {2000, "副歌"}, {3000, "three"}, {4000, "chorus"}, {4000, "副歌"}}
	if !l.Synced || !reflect.DeepEqual(l.Lines, want) {
		t.Fatalf("got %+v", l.Lines)
	}
}
