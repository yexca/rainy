package util

import (
	"strings"
	"testing"
)

func TestM3USpecialNames(t *testing.T) {
	text, err := FormatM3U([]M3UTrack{{ID: "example-id", Path: "#Example/示例 100%.flac", Artist: "Example\n#injection", Title: "示例"}})
	if err != nil {
		t.Fatal(err)
	}
	entries, err := ParseM3U(text, "")
	if err != nil || len(entries) != 1 || entries[0].Path != "#Example/示例 100%.flac" || entries[0].TrackID != "example-id" {
		t.Fatalf("%+v: %v", entries, err)
	}
	if strings.Contains(text, "\n#injection") {
		t.Fatal("metadata injected a line")
	}
	if _, err := FormatM3U([]M3UTrack{{Path: "bad\nname.mp3"}}); err == nil {
		t.Fatal("unsafe path exported")
	}
	if _, err := ParseM3U(string([]byte{0xff}), ""); err == nil {
		t.Fatal("invalid UTF-8 accepted")
	}
}

func TestM3UColonPaths(t *testing.T) {
	paths := []string{"Re:ZERO/01 Example.flac", "Example: Vol. 1/02.mp3", "A:B.flac", "http:example.flac"}
	tracks := make([]M3UTrack, len(paths))
	for i, p := range paths {
		tracks[i] = M3UTrack{ID: "example-id", Path: p}
	}
	text, err := FormatM3U(tracks)
	if err != nil {
		t.Fatal(err)
	}
	entries, err := ParseM3U(text, "")
	if err != nil || len(entries) != len(paths) {
		t.Fatalf("%+v: %v", entries, err)
	}
	for i, e := range entries {
		if !e.Valid || e.Path != paths[i] {
			t.Fatalf("entry %d = %+v, want %q", i, e, paths[i])
		}
	}
	if entries, err := ParseM3U("Song.flac\n", "Re:ZERO"); err != nil || entries[0].Path != "Re:ZERO/Song.flac" {
		t.Fatalf("%+v: %v", entries, err)
	}
}

func TestM3URejectsNonRelativeEntries(t *testing.T) {
	lines := []string{
		"http://example.com/a.mp3", "HTTPS://example.com/a.mp3", "file:///path/to/music/a.mp3", "file:/a.mp3",
		"smb://192.0.2.1/music/a.mp3", "C:\\Music\\a.mp3", "c:/music/a.mp3", "C:a.mp3",
		"/path/to/music/a.mp3", "\\\\server\\share\\a.mp3", "../a.mp3", "Example/../../a.mp3",
	}
	entries, err := ParseM3U(strings.Join(lines, "\n"), "")
	if err != nil || len(entries) != len(lines) {
		t.Fatalf("%+v: %v", entries, err)
	}
	for i, e := range entries {
		if e.Valid {
			t.Errorf("%q accepted as %q", lines[i], e.Path)
		}
	}
	if _, err := ParseM3U("a.mp3", "C:/Music"); err == nil {
		t.Fatal("drive-letter base folder accepted")
	}
	for _, p := range []string{"/a.mp3", "a//b.mp3", "./a.mp3", "../a.mp3", " a.mp3"} {
		if _, err := FormatM3U([]M3UTrack{{Path: p}}); err == nil {
			t.Errorf("%q exported", p)
		}
	}
}
