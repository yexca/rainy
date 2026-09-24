package util

import (
	"crypto/md5"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestNewID(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 2000; i++ {
		id := NewID()
		if len(id) != IDLength {
			t.Fatalf("len(%q) = %d", id, len(id))
		}
		for _, r := range id {
			if !strings.ContainsRune(base62, r) {
				t.Fatalf("invalid rune %q in %q", r, id)
			}
		}
		if seen[id] {
			t.Fatalf("duplicate id %q", id)
		}
		seen[id] = true
	}
}

func TestHashID(t *testing.T) {
	a := HashID("album", "Artist", "Name")
	if len(a) != IDLength || strings.ToLower(a) != a {
		t.Fatalf("bad id %q", a)
	}
	if HashID("album", " artist ", "NAME") != a {
		t.Fatal("HashID must trim and lower-case parts")
	}
	if HashID("album", "ab", "c") == HashID("album", "a", "bc") {
		t.Fatal("parts must be separated")
	}
	sum := md5.Sum([]byte("artist\x00beatles"))
	if got, want := HashID("artist", "Beatles"), hex.EncodeToString(sum[:])[:IDLength]; got != want {
		t.Fatalf("HashID = %q, want %q", got, want)
	}
	if AlbumID("A", "B") != HashID("album", "a", "b") || ArtistID("X") != HashID("artist", "x") || GenreID("Pop") != HashID("genre", "pop") {
		t.Fatal("helpers")
	}
}

func TestTime(t *testing.T) {
	now := NowMs()
	if d := time.Since(time.UnixMilli(now)); d < 0 || d > time.Second {
		t.Fatal(d)
	}
	if RFC3339Ms(0) != "" || !FromMs(0).IsZero() {
		t.Fatal("zero")
	}
	if got := RFC3339Ms(1700000000000); got != "2023-11-14T22:13:20Z" {
		t.Fatal(got)
	}
}

func TestNormalizeSearch(t *testing.T) {
	cases := []struct {
		in   []string
		want string
	}{
		{[]string{"Beyoncé"}, "beyonce"},
		{[]string{"  Hello   World  "}, "hello world"},
		{[]string{"ＡＢＣ　１２３"}, "abc 123"}, // full-width letters, digits and space
		{[]string{"Don't Stop"}, "dont stop"},
		{[]string{"AC/DC", "Back-in Black!"}, "ac dc back in black"},
		{[]string{"周杰伦", "七里香"}, "周杰伦 七里香"},
		{[]string{"ガッチャ"}, "カッチャ"},
		{[]string{"ｶﾞｯﾁｬ"}, "カッチャ"}, // half-width katakana
		{[]string{"Ǆ ﬁ"}, "dz fi"},
		{[]string{"", "  ", ""}, ""},
		{[]string{"100%_done\\"}, "100 done"},
		{[]string{"Tab\tNew\nLine"}, "tab new line"},
		{[]string{"「千本桜」・初音ミク"}, "千本桜 初音ミク"},
	}
	for _, c := range cases {
		if got := NormalizeSearch(c.in...); got != c.want {
			t.Errorf("NormalizeSearch(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestSearchTokens(t *testing.T) {
	if SearchTokens("") != nil || SearchTokens("  !! ") != nil {
		t.Fatal("empty query must give nil")
	}
	got := SearchTokens("The  Beatles the ＬＥＴ")
	if !reflect.DeepEqual(got, []string{"the", "beatles", "let"}) {
		t.Fatal(got)
	}
	if got := SearchTokens("周杰伦 晴天"); !reflect.DeepEqual(got, []string{"周杰伦", "晴天"}) {
		t.Fatal(got)
	}
}

func TestEscapeLike(t *testing.T) {
	for in, want := range map[string]string{"abc": "abc", "50%": `50\%`, "a_b": `a\_b`, `c:\x`: `c:\\x`} {
		if got := EscapeLike(in); got != want {
			t.Errorf("EscapeLike(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSortNameAndIndexKey(t *testing.T) {
	arts := strings.Fields("The El La Los Las Le Les")
	sorts := map[string]string{
		"The Beatles": "beatles", "the  who": "who", "The": "the", "Theatre": "theatre",
		"Les Misérables": "miserables", "Björk": "bjork", "周杰伦": "周杰伦",
	}
	for in, want := range sorts {
		if got := SortName(in, arts); got != want {
			t.Errorf("SortName(%q) = %q, want %q", in, got, want)
		}
	}
	keys := map[string]string{
		"The Beatles": "B", "abba": "A", "Émilie": "E", "Ｚｅｄ": "Z", "周杰伦": "Z", "陈奕迅": "C",
		"王菲": "W", "張學友": "Z", "林俊傑": "L", "宇多田ヒカル": "Y", "あいみょん": "#", "ヨルシカ": "#",
		"아이유": "#", "2Pac": "#", "'Til Tuesday": "T", "(hed) p.e.": "H", "": "#", "   ": "#", "...": "#",
	}
	for in, want := range keys {
		if got := IndexKey(in, arts); got != want {
			t.Errorf("IndexKey(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSplitMulti(t *testing.T) {
	cases := []struct {
		in   []string
		seps string
		want []string
	}{
		{[]string{"Pop; Rock"}, ";/,", []string{"Pop", "Rock"}},
		{[]string{"J-Pop/Anime"}, ";/,", []string{"J-Pop", "Anime"}},
		{[]string{"Rock", "rock", " ROCK ; Jazz"}, ";", []string{"Rock", "Jazz"}},
		{[]string{"", " ; ", ","}, ";,", []string{}},
		{nil, ";", []string{}},
		{[]string{" a;b ", "a;b"}, "", []string{"a;b"}},
	}
	for _, c := range cases {
		if got := SplitMulti(c.in, c.seps); !reflect.DeepEqual(got, c.want) {
			t.Errorf("SplitMulti(%q, %q) = %q, want %q", c.in, c.seps, got, c.want)
		}
	}
}

func TestCoverArtID(t *testing.T) {
	id := CoverArtID("al", "abc123", 1700000000000)
	if id != "al-abc123_loyw3v28" {
		t.Fatal(id)
	}
	if CoverArtID("tr", "", 5) != "" {
		t.Fatal("empty id")
	}
	cases := map[string][3]string{
		id:                        {"al", "abc123", "loyw3v28"},
		"ar-XyZ":                  {"ar", "XyZ", ""},
		"tr-Abc_1":                {"tr", "Abc", "1"},
		"pl-p1_0":                 {"pl", "p1", "0"},
		"abc123":                  {"", "abc123", ""},
		"abc123_9":                {"", "abc123", "9"},
		"xx-abc":                  {"", "xx-abc", ""},
		"al-":                     {"", "al-", ""},
		" al-0123456789abcdef_z ": {"al", "0123456789abcdef", "z"},
	}
	for in, want := range cases {
		k, i, v := SplitCoverArtID(in)
		if k != want[0] || i != want[1] || v != want[2] {
			t.Errorf("SplitCoverArtID(%q) = %q,%q,%q want %q", in, k, i, v, want)
		}
		if k2, i2 := ParseCoverArtID(in); k2 != k || i2 != i {
			t.Errorf("ParseCoverArtID mismatch for %q", in)
		}
	}
}

func TestMimeType(t *testing.T) {
	for in, want := range map[string]string{
		"mp3": "audio/mpeg", ".FLAC": "audio/flac", "m4a": "audio/mp4", "opus": "audio/ogg", "wav": "audio/wav",
		"jpg": "image/jpeg", "PNG": "image/png", "webp": "image/webp", "xyz": "application/octet-stream", "": "application/octet-stream",
	} {
		if got := MimeType(in); got != want {
			t.Errorf("MimeType(%q) = %q, want %q", in, got, want)
		}
	}
	if !IsImageSuffix(".JPG") || IsImageSuffix("gif") {
		t.Fatal("IsImageSuffix")
	}
}

func TestSafeJoin(t *testing.T) {
	root := t.TempDir()
	ok := map[string]string{
		"":                    root,
		".":                   root,
		"a/b.mp3":             filepath.Join(root, "a", "b.mp3"),
		"a\\b.mp3":            filepath.Join(root, "a", "b.mp3"),
		"./a//b/./c.flac":     filepath.Join(root, "a", "b", "c.flac"),
		"周杰伦/七里香/01 七里香.flac": filepath.Join(root, "周杰伦", "七里香", "01 七里香.flac"),
		"..a/b..":             filepath.Join(root, "..a", "b.."),
		"a/...b":              filepath.Join(root, "a", "...b"),
	}
	for rel, want := range ok {
		got, err := SafeJoin(root, rel)
		if err != nil || got != want {
			t.Errorf("SafeJoin(%q) = %q, %v; want %q", rel, got, err, want)
		}
	}
	bad := []string{
		"..", "../x", "a/../../x", "a/..", `..\x`, `a\..\..\x`, "/etc/passwd", `\x`, `\\server\share\x`,
		"C:\\Windows", "C:/Windows", "c:x", "a\x00b",
		"//server/share/x", `a/..\..\x`, `a\b/../../..`, "C:", "/", "./../x", "a/./../..",
	}
	if runtime.GOOS == "windows" {
		bad = append(bad, "a/.. /x", "a/. /x", "a/.../x", "file.mp3:stream", "a/ /b",
			"NUL", "a/con", "a/COM1/b.mp3", "a/CONIN$", `\\?\C:\x`, `\\.\pipe\x`, "a/b\\..\\..\\..\\x")
	}
	for _, rel := range bad {
		if got, err := SafeJoin(root, rel); err == nil || !errors.Is(err, ErrUnsafePath) {
			t.Errorf("SafeJoin(%q) = %q, %v; want ErrUnsafePath", rel, got, err)
		}
	}
	if _, err := SafeJoin("", "a"); err == nil {
		t.Error("empty root must fail")
	}
	// A root with a trailing separator behaves the same.
	if got, err := SafeJoin(root+string(filepath.Separator), "x"); err != nil || got != filepath.Join(root, "x") {
		t.Errorf("trailing separator root: %q %v", got, err)
	}
}

func TestSafeJoinWindowsRoots(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("windows only")
	}
	for _, c := range []struct{ root, rel, want string }{
		{`C:\Music`, "a/b.mp3", `C:\Music\a\b.mp3`},
		{`C:\Music\`, `a\b.mp3`, `C:\Music\a\b.mp3`},
		{`\\nas\share\music`, "x/y.flac", `\\nas\share\music\x\y.flac`},
		{`D:\`, "a", `D:\a`},
	} {
		got, err := SafeJoin(c.root, c.rel)
		if err != nil || got != c.want {
			t.Errorf("SafeJoin(%q, %q) = %q, %v; want %q", c.root, c.rel, got, err, c.want)
		}
	}
	if _, err := SafeJoin(`C:\Music`, `D:\x`); err == nil {
		t.Error("other drive must fail")
	}
}

func TestToRelAndWithin(t *testing.T) {
	root := t.TempDir()
	if rel, err := ToRel(root, filepath.Join(root, "a", "b.mp3")); err != nil || rel != "a/b.mp3" {
		t.Fatal(rel, err)
	}
	if rel, err := ToRel(root, root); err != nil || rel != "" {
		t.Fatal(rel, err)
	}
	if _, err := ToRel(root, filepath.Dir(root)); err == nil {
		t.Fatal("parent must fail")
	}
	if _, err := ToRel(root, root+"x"); err == nil {
		t.Fatal("sibling with common prefix must fail")
	}
	if IsWithin(root, root+"x") || !IsWithin(root, filepath.Join(root, "x")) {
		t.Fatal("IsWithin")
	}
}

func TestEnsureWithinRoot(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "lib")
	outside := filepath.Join(base, "outside")
	for _, d := range []string{filepath.Join(root, "a"), outside} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := EnsureWithinRoot(root, filepath.Join(root, "a", "new", "file.mp3")); err != nil {
		t.Fatalf("non-existing path inside root: %v", err)
	}
	if err := EnsureWithinRoot(root, filepath.Join(root, "a")); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "link")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if err := EnsureWithinRoot(root, filepath.Join(link, "x.mp3")); !errors.Is(err, ErrUnsafePath) {
		t.Fatalf("symlink escape not detected: %v", err)
	}
}

func TestPathParts(t *testing.T) {
	cases := map[string][3]string{
		"A/B/01 x.FLAC": {"A/B", "01 x.FLAC", "flac"},
		"song.mp3":      {"", "song.mp3", "mp3"},
		`a\b\c.Opus`:    {"a/b", "c.Opus", "opus"},
		"/x/noext":      {"x", "noext", ""},
		"dir/.hidden":   {"dir", ".hidden", "hidden"},
	}
	for in, want := range cases {
		d, f, s := PathParts(in)
		if d != want[0] || f != want[1] || s != want[2] {
			t.Errorf("PathParts(%q) = %q,%q,%q want %q", in, d, f, s, want)
		}
	}
}

func TestIsHiddenOrSystem(t *testing.T) {
	for _, n := range []string{".hidden", ".@__thumb", "@eaDir", "@EADIR", "#recycle", "#snapshot", "$RECYCLE.BIN",
		"lost+found", "System Volume Information", ".DS_Store"} {
		if !IsHiddenOrSystem(n) {
			t.Errorf("%q should be hidden", n)
		}
	}
	for _, n := range []string{"", "music", "eaDir", "recycle", "a.b", "周杰伦"} {
		if IsHiddenOrSystem(n) {
			t.Errorf("%q should not be hidden", n)
		}
	}
}

func TestFirstNonEmpty(t *testing.T) {
	if FirstNonEmpty("", " ", "x", "y") != "x" || FirstNonEmpty() != "" {
		t.Fatal()
	}
}
