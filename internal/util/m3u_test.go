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
