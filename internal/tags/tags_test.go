package tags

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/png"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
)

func ptr(f float64) *float64 { return &f }

func TestParse(t *testing.T) {
	raw := map[string][]string{
		"TITLE":                 {"  Song  "},
		"ARTIST":                {"A", "B", "a"},
		"ALBUM":                 {"Album"},
		"TRACKNUMBER":           {"3/12"},
		"DISCNUMBER":            {"2"},
		"DISCTOTAL":             {"3"},
		"DATE":                  {"2021-06-18"},
		"ORIGINALDATE":          {"1999"},
		"GENRE":                 {"Pop; Rock", " ", "Jazz"},
		"COMPILATION":           {"1"},
		"REPLAYGAIN_TRACK_GAIN": {"-6.50 dB"},
		"REPLAYGAIN_TRACK_PEAK": {"0.98"},
		"R128_ALBUM_GAIN":       {"-512"},
		"UNSYNCEDLYRICS":        {"line1\r\nline2\n"},
		"BPM":                   {"128.4"},
		"MUSICBRAINZ_TRACKID":   {"mbz-1"},
		"ALBUMARTISTSORT":       {"Artist, The"},
	}
	m := parse(raw)
	want := &Metadata{
		Title: "Song", Artist: "A / B", Album: "Album",
		TrackNumber: 3, TrackTotal: 12, DiscNumber: 2, DiscTotal: 3,
		Year: 2021, Date: "2021-06-18", OriginalYear: 1999,
		Genres: []string{"Pop; Rock", "Jazz"}, Compilation: true,
		RGTrackGain: ptr(-6.5), RGTrackPeak: ptr(0.98), RGAlbumGain: ptr(3),
		Lyrics: "line1\nline2", BPM: 128, MbzTrackID: "mbz-1", SortAlbumArtist: "Artist, The",
		Raw: raw,
	}
	if !reflect.DeepEqual(m, want) {
		t.Fatalf("parse:\n got %+v\nwant %+v", m, want)
	}
}

func TestParseHelpers(t *testing.T) {
	pairs := []struct {
		in   string
		n, t int
	}{{"3/12", 3, 12}, {"03", 3, 0}, {"7 of 9", 7, 9}, {"", 0, 0}, {"x", 0, 0}, {"1/", 1, 0}}
	for _, p := range pairs {
		if n, tot := parsePair(p.in); n != p.n || tot != p.t {
			t.Errorf("parsePair(%q) = %d,%d want %d,%d", p.in, n, tot, p.n, p.t)
		}
	}
	years := map[string]int{"2021-06-18": 2021, "20210618": 2021, "1999": 1999, "Spring 85": 0, "0000": 0, "c. 1972": 1972}
	for in, want := range years {
		if got := parseYear(in); got != want {
			t.Errorf("parseYear(%q) = %d want %d", in, got, want)
		}
	}
	if g := parseGain("+1.20dB"); g == nil || *g != 1.2 {
		t.Errorf("parseGain(+1.20dB) = %v", g)
	}
	if g := parseGain("junk"); g != nil {
		t.Errorf("parseGain(junk) = %v", *g)
	}
	for in, want := range map[string]bool{"1": true, "true": true, "TRUE": true, "0": false, "": false} {
		if parseBool(in) != want {
			t.Errorf("parseBool(%q) != %v", in, want)
		}
	}
	if !IsAudioFile("x/Song.FLAC") || IsAudioFile("cover.jpg") || IsAudioFile("noext") {
		t.Error("IsAudioFile")
	}
}

// ---- file based tests (need ffmpeg)

func requireFFmpeg(t *testing.T) string {
	t.Helper()
	p, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg not available")
	}
	return p
}

func testPNG(t *testing.T, c color.Color) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 32, 32))
	for y := 0; y < 32; y++ {
		for x := 0; x < 32; x++ {
			img.Set(x, y, c)
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// genAudio creates a short tagged audio file with ffmpeg (optionally with a cover).
func genAudio(t *testing.T, dir, name string, cover []byte, meta ...string) string {
	t.Helper()
	ff := requireFFmpeg(t)
	out := filepath.Join(dir, name)
	args := []string{"-hide_banner", "-loglevel", "error", "-nostdin", "-y",
		"-f", "lavfi", "-i", "sine=frequency=440:sample_rate=44100:duration=2"}
	maps := []string{"-map", "0:a"}
	if cover != nil {
		cp := filepath.Join(dir, name+".cover.png")
		if err := os.WriteFile(cp, cover, 0o644); err != nil {
			t.Fatal(err)
		}
		args = append(args, "-i", cp)
		maps = append(maps, "-map", "1:v", "-c:v", "copy", "-disposition:v:0", "attached_pic")
	}
	args = append(args, maps...)
	switch filepath.Ext(name) {
	case ".mp3":
		args = append(args, "-c:a", "libmp3lame", "-b:a", "64k", "-id3v2_version", "4")
	case ".flac":
		args = append(args, "-c:a", "flac")
	case ".m4a":
		args = append(args, "-c:a", "aac", "-b:a", "64k")
	case ".ogg":
		args = append(args, "-c:a", "libvorbis")
	case ".opus":
		args = append(args, "-c:a", "libopus", "-b:a", "32k")
	case ".aiff":
		args = append(args, "-write_id3v2", "1")
	}
	for i := 0; i+1 < len(meta); i += 2 {
		args = append(args, "-metadata", meta[i]+"="+meta[i+1])
	}
	args = append(args, out)
	cmd := exec.Command(ff, args...)
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Skipf("ffmpeg cannot create %s: %v\n%s", name, err, b)
	}
	return out
}

func TestReadWriteRoundTrip(t *testing.T) {
	cover := testPNG(t, color.RGBA{200, 40, 40, 255})
	for _, name := range []string{"a.mp3", "b.flac", "c.m4a", "d.ogg", "e.opus"} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			var pic []byte
			if name == "a.mp3" || name == "b.flac" || name == "c.m4a" {
				pic = cover
			}
			p := genAudio(t, dir, name, pic,
				"title", "夏日微风", "artist", "林雨晴", "album", "Album", "genre", "Pop", "track", "3/10")

			m, err := Read(p, ReadOptions{FixEncoding: true})
			if err != nil {
				t.Fatalf("Read: %v", err)
			}
			if m.Title != "夏日微风" || m.Artist != "林雨晴" || m.Album != "Album" || m.TrackNumber != 3 {
				t.Fatalf("Read tags: %+v", m)
			}
			if m.Duration < 1.5 || m.Duration > 2.5 || m.SampleRate == 0 || m.Channels == 0 || m.Codec == "" {
				t.Fatalf("Read properties: dur=%v sr=%d ch=%d codec=%q", m.Duration, m.SampleRate, m.Channels, m.Codec)
			}
			if m.HasPicture != (pic != nil) {
				t.Fatalf("HasPicture = %v", m.HasPicture)
			}

			// Merge semantics: replace, add, delete; others untouched.
			if err := Write(p, map[string][]string{
				"title":       {"New Title"},
				"GENRE":       {"Rock", "Jazz"},
				"ALBUM":       {},
				"COMPOSER":    {"Composer"},
				"LYRICS":      {"[00:01.00]hello\n[00:02.00]world"},
				"COMPILATION": {"1"},
			}); err != nil {
				t.Fatalf("Write: %v", err)
			}
			m, err = Read(p, ReadOptions{})
			if err != nil {
				t.Fatalf("Read after write: %v", err)
			}
			if m.Title != "New Title" || m.Album != "" || m.Artist != "林雨晴" || m.Composer != "Composer" {
				t.Fatalf("after write: %+v", m)
			}
			if !reflect.DeepEqual(m.Genres, []string{"Rock", "Jazz"}) {
				t.Fatalf("genres after write: %v", m.Genres)
			}
			if m.Lyrics != "[00:01.00]hello\n[00:02.00]world" {
				t.Fatalf("lyrics after write: %q", m.Lyrics)
			}
			if !m.Compilation {
				t.Fatalf("compilation after write: %v", m.Raw["COMPILATION"])
			}

			// Pictures.
			newCover := testPNG(t, color.RGBA{10, 200, 10, 255})
			if err := WritePicture(p, newCover); err != nil {
				t.Fatalf("WritePicture: %v", err)
			}
			got, err := ReadPicture(p)
			if err != nil || !bytes.Equal(got, newCover) {
				t.Fatalf("ReadPicture after write: %d bytes, %v", len(got), err)
			}
			if err := WritePicture(p, nil); err != nil {
				t.Fatalf("WritePicture(nil): %v", err)
			}
			if got, err := ReadPicture(p); err != nil || got != nil {
				t.Fatalf("ReadPicture after removal: %d bytes, %v", len(got), err)
			}
			if m, err := Read(p, ReadOptions{}); err != nil || m.HasPicture || m.Title != "New Title" {
				t.Fatalf("Read after picture removal: %+v %v", m, err)
			}
		})
	}
}

func TestErrors(t *testing.T) {
	dir := t.TempDir()
	if _, err := Read(filepath.Join(dir, "missing.mp3"), ReadOptions{}); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("missing file: %v", err)
	}
	junk := filepath.Join(dir, "junk.mp3")
	if err := os.WriteFile(junk, []byte("this is not audio"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Read(junk, ReadOptions{}); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("junk file: %v", err)
	}
	if err := Write(junk, map[string][]string{"A\tB": {"x"}}); !errors.Is(err, ErrInvalidKey) {
		t.Fatalf("invalid key: %v", err)
	}
	if _, _, err := embeddable([]byte("not an image")); !errors.Is(err, ErrInvalidImage) {
		t.Fatalf("invalid image: %v", err)
	}
}

// WAV and AIFF carry two tag containers (RIFF INFO / AIFF text chunks and ID3v2). Embedding
// a picture creates an ID3v2 tag; TagLib then reads the text tags from it, so every text
// tag must be carried over, or the file would suddenly have no title / artist.
func TestPictureKeepsTagsInDualContainerFormats(t *testing.T) {
	cover := testPNG(t, color.RGBA{10, 120, 200, 255})
	for _, name := range []string{"a.wav", "b.aiff"} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			p := genAudio(t, dir, name, nil, "title", "Forest Rain", "artist", "Nature Sounds",
				"album", "Field Recordings", "genre", "Ambient", "track", "1")
			before, err := Read(p, ReadOptions{})
			if err != nil {
				t.Fatalf("Read: %v", err)
			}
			if before.Title != "Forest Rain" || before.Artist != "Nature Sounds" {
				t.Skipf("ffmpeg wrote no readable tags into %s: %+v", name, before.Raw)
			}
			check := func(step string, wantPic bool, title string) {
				t.Helper()
				m, err := Read(p, ReadOptions{})
				if err != nil {
					t.Fatalf("%s: Read: %v", step, err)
				}
				if m.Title != title || m.Artist != before.Artist || m.Album != before.Album ||
					!reflect.DeepEqual(m.Genres, before.Genres) || m.HasPicture != wantPic {
					t.Fatalf("%s: title=%q artist=%q album=%q genres=%v pic=%v; raw=%v",
						step, m.Title, m.Artist, m.Album, m.Genres, m.HasPicture, m.Raw)
				}
			}
			if err := WritePicture(p, cover); err != nil {
				t.Fatalf("WritePicture: %v", err)
			}
			check("after WritePicture", true, "Forest Rain")
			if got, err := ReadPicture(p); err != nil || !bytes.Equal(got, cover) {
				t.Fatalf("ReadPicture: %d bytes, %v", len(got), err)
			}
			if err := Write(p, map[string][]string{"TITLE": {"Forest Rain (Edit)"}}); err != nil {
				t.Fatalf("Write: %v", err)
			}
			check("after Write", true, "Forest Rain (Edit)")
			if err := WritePicture(p, nil); err != nil {
				t.Fatalf("WritePicture(nil): %v", err)
			}
			check("after picture removal", false, "Forest Rain (Edit)")
		})
	}
}
