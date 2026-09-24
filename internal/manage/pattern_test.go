package manage

import (
	"errors"
	"strings"
	"testing"

	"rainy/internal/store"
)

func TestParsePatternErrors(t *testing.T) {
	for _, p := range []string{
		"", "   ", "plain text", "{title", "title}", "{nope}", "[{title}", "{title}]",
		"[[{title}]]", "{title:2}", "{track:0}", "{track:x}", "{disc:10}",
	} {
		if _, err := ParsePattern(p); !errors.Is(err, store.ErrInvalid) {
			t.Errorf("ParsePattern(%q) = %v, want ErrInvalid", p, err)
		}
	}
	for _, p := range []string{
		"{title}", "{albumartist}/{album}/[{disc}-]{track:2} {title}", "{Artist} - {TITLE}",
		"[{year} ]{album}/{track:3}", "{genre}/{composer}/{title}", "[]{title}",
	} {
		if _, err := ParsePattern(p); err != nil {
			t.Errorf("ParsePattern(%q): %v", p, err)
		}
	}
}

func TestRender(t *testing.T) {
	base := Values{Title: "Blue Skies", Artist: "The Rainy Days", Album: "Blue Skies", AlbumArtist: "The Rainy Days",
		Track: 1, Disc: 1, Year: 2022, Genre: "Pop", Composer: "Ann"}
	multi := base
	multi.MultiDisc, multi.Disc, multi.Track = true, 2, 3
	long := base
	long.Title = strings.Repeat("长", 120) // 360 bytes
	cases := []struct {
		name, pattern string
		v             Values
		ext, want     string
		wantErr       bool
	}{
		{"default single disc", "{albumartist}/{album}/[{disc}-]{track:2} {title}", base, "flac", "The Rainy Days/Blue Skies/01 Blue Skies.flac", false},
		{"default multi disc", "{albumartist}/{album}/[{disc}-]{track:2} {title}", multi, "flac", "The Rainy Days/Blue Skies/2-03 Blue Skies.flac", false},
		{"disc width", "[{disc:2}.]{track:3} {title}", multi, "mp3", "02.003 Blue Skies.mp3", false},
		{"no track number", "[{track:2} ]{title}", Values{Title: "X"}, "mp3", "X.mp3", false},
		{"track without optional", "{track} {title}", Values{Title: "X"}, "mp3", "X.mp3", false},
		{"optional with year", "{artist}/[{year} - ]{album}/{title}", base, "ogg", "The Rainy Days/2022 - Blue Skies/Blue Skies.ogg", false},
		{"optional without year", "{artist}/[{year} - ]{album}/{title}", Values{Artist: "A", Album: "B", Title: "C"}, "ogg", "A/B/C.ogg", false},
		{"slash in value", "{artist}/{title}", Values{Artist: "AC/DC", Title: "T.N.T."}, "mp3", "AC_DC/T.N.T.mp3", false},
		{"illegal chars", "{title}", Values{Title: `a<b>c:d"e|f?g*h\i`}, "mp3", "a_b_c_d_e_f_g_h_i.mp3", false},
		{"control chars", "{title}", Values{Title: "a\tb\nc"}, "mp3", "a_b_c.mp3", false},
		{"trailing dots and spaces", "{album}/{title}", Values{Album: "Vol. 2...  ", Title: "  x. "}, "mp3", "Vol. 2/x.mp3", false},
		{"dot dot is dropped", "{album}/{title}", Values{Album: "..", Title: "x"}, "mp3", "x.mp3", false},
		{"empty genre level dropped", "{genre}/{title}", Values{Title: "x"}, "mp3", "x.mp3", false},
		{"reserved name", "{title}", Values{Title: "CON"}, "mp3", "CON_.mp3", false},
		{"empty file name", "{title}", Values{}, "mp3", "", true},
		{"cjk", "{albumartist}/{album}/{track:2} {title}", Values{AlbumArtist: "林雨晴", Album: "夏日微风", Track: 6, Title: "晚安，夏天"}, "mp3", "林雨晴/夏日微风/06 晚安，夏天.mp3", false},
		{"case-insensitive tokens", "{Artist} - {TITLE}", base, "mp3", "The Rainy Days - Blue Skies.mp3", false},
		{"long component", "{title}", long, "flac", strings.Repeat("长", 65) + ".flac", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p, err := ParsePattern(c.pattern)
			if err != nil {
				t.Fatal(err)
			}
			got, err := renderPath(p, c.v, c.ext)
			if c.wantErr {
				if !errors.Is(err, store.ErrInvalid) {
					t.Fatalf("got %q, %v; want ErrInvalid", got, err)
				}
				return
			}
			if err != nil || got != c.want {
				t.Fatalf("got %q, %v; want %q", got, err, c.want)
			}
			for _, comp := range strings.Split(got, "/") {
				if len(comp) > maxComponent {
					t.Fatalf("component %q is %d bytes", comp, len(comp))
				}
			}
		})
	}
}

func TestTruncateBytes(t *testing.T) {
	if got := truncateBytes("ab长c", 4); got != "ab" {
		t.Fatalf("got %q", got)
	}
	if got := truncateBytes("abc", 5); got != "abc" {
		t.Fatalf("got %q", got)
	}
}

func TestNaturalLess(t *testing.T) {
	if !naturalLess("2 b", "10 a") || naturalLess("10 a", "2 b") || !naturalLess("a", "B") || !naturalLess("CD1", "cd2") {
		t.Fatal("natural ordering")
	}
}
