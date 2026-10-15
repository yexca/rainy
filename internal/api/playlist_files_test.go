package api

import (
	"slices"
	"strings"
	"testing"

	"rainy/internal/model"
	"rainy/internal/store"
	"rainy/internal/util"
)

func TestPlaylistFilesRoundTripAndVisibility(t *testing.T) {
	e := newNativeEnv(t)
	ids := []string{e.id("one"), e.id("tone"), e.id("one")}
	pl := nativeJSON[model.Playlist](t, e.do("POST", "/playlists", e.aliceTok, map[string]any{"name": "Example playlist", "trackIds": ids}), 201)
	url := "/playlists/" + pl.ID + "/export"
	nativeErrorCode(t, e.do("GET", url, "", nil), 401)
	nativeErrorCode(t, e.do("GET", url, e.bobTok, nil), 404)
	response := e.do("GET", url, e.aliceTok, nil)
	nativeExpect(t, response, 200)
	if response.Header().Get("Cache-Control") != "no-store" || !strings.Contains(response.Header().Get("Content-Disposition"), ".m3u8") {
		t.Fatal("export must be a private downloadable file")
	}
	body := map[string]any{"text": response.Body.String()}
	nativeErrorCode(t, e.do("POST", "/playlists/import/preview", "", body), 401)
	preview := nativeJSON[store.PlaylistImportPreview](t, e.do("POST", "/playlists/import/preview", e.bobTok, body), 200)
	var got []string
	for _, entry := range preview.Entries {
		if entry.Status != "matched" {
			t.Fatalf("unmatched export: %+v", entry)
		}
		got = append(got, entry.TrackID)
	}
	if !slices.Equal(got, ids) {
		t.Fatalf("round trip %v", got)
	}
	if lists := nativeJSON[[]model.Playlist](t, e.do("GET", "/playlists", e.bobTok, nil), 200); len(lists) != 0 {
		t.Fatal("preview must not create a playlist")
	}
}

func TestPlaylistImportPathsAndAmbiguity(t *testing.T) {
	e := newNativeEnv(t)
	lib := &model.Library{Name: "Example second library", Path: t.TempDir()}
	if err := e.app.Store.CreateLibrary(t.Context(), lib); err != nil {
		t.Fatal(err)
	}
	copy := e.tr["one"]
	copy.ID, copy.LibraryID = "example-second-copy", lib.ID
	if err := e.app.Store.UpsertTracks(t.Context(), []model.Track{copy}); err != nil {
		t.Fatal(err)
	}
	body := map[string]any{"text": strings.Join([]string{
		"\ufeff#EXTM3U", copy.Path, "#RAINY-TRACK:" + copy.ID, "./" + copy.Path,
		e.tr["gone"].Path, "unknown.mp3", "../outside.mp3", "/path/to/music/song.mp3",
		"https://example.com/song.mp3", "C:\\music\\song.mp3",
		strings.ReplaceAll(e.tr["tone"].Path, "/", "\\"),
	}, "\r\n")}
	preview := nativeJSON[store.PlaylistImportPreview](t, e.do("POST", "/playlists/import/preview", e.aliceTok, body), 200)
	var statuses []string
	for _, row := range preview.Entries {
		statuses = append(statuses, row.Status)
	}
	want := []string{"ambiguous", "matched", "missing", "missing", "invalid", "invalid", "invalid", "invalid", "matched"}
	if !slices.Equal(statuses, want) || preview.Entries[1].TrackID != copy.ID {
		t.Fatalf("%+v", preview)
	}
	body["libraryId"] = e.lib.ID
	preview = nativeJSON[store.PlaylistImportPreview](t, e.do("POST", "/playlists/import/preview", e.aliceTok, body), 200)
	if preview.Entries[0].TrackID != e.id("one") || preview.Entries[1].TrackID != e.id("one") {
		t.Fatalf("library scope: %+v", preview)
	}
	body = map[string]any{"text": "01 Tone.wav", "baseDir": "Sine Wave/Tones"}
	preview = nativeJSON[store.PlaylistImportPreview](t, e.do("POST", "/playlists/import/preview", e.aliceTok, body), 200)
	if preview.Entries[0].TrackID != e.id("tone") {
		t.Fatal("relative folder match")
	}
	for _, invalid := range []map[string]any{
		{"text": strings.Repeat("x", util.MaxPlaylistBytes+1)},
		{"text": strings.Repeat("a\n", util.MaxPlaylistEntries+1)},
		{"text": "a", "baseDir": "../outside"},
		{"text": "a", "libraryId": -1},
	} {
		nativeErrorCode(t, e.do("POST", "/playlists/import/preview", e.aliceTok, invalid), 400)
	}
}
