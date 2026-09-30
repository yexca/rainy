package model

import (
	"encoding/json"
	"sort"
	"strings"
	"testing"
)

// jsonKeys returns the sorted top-level keys of v's JSON object form.
func jsonKeys(t *testing.T, v any) []string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("%T is not a JSON object: %s", v, b)
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// fields splits a whitespace/comma separated key list and sorts it.
func fields(s string) []string {
	out := strings.FieldsFunc(s, func(r rune) bool { return r == ',' || r == ' ' || r == '\n' || r == '\t' })
	sort.Strings(out)
	return out
}

// TestJSONMatchesTSContract locks the JSON field names to the TypeScript contract in
// docs/architecture/contract.md §8 (web/src/lib/api/types.ts). extra lists keys the Go side may send in
// addition (ignored by the web app).
func TestJSONMatchesTSContract(t *testing.T) {
	cases := []struct {
		name  string
		v     any
		want  string
		extra string
	}{
		{"User", User{}, `id username displayName email isAdmin canManage canDownload hasApiKey
			createdAt updatedAt lastLoginAt lastSeenAt`, ""},
		{"Track", Track{}, `id libraryId path dir filename suffix size mtime title album artist albumArtist
			albumId artistId albumArtistId trackNumber trackTotal discNumber discTotal discSubtitle
			year date originalYear genre genres composer comment hasLrc hasLyrics bpm compilation
			duration bitrate sampleRate bitDepth channels codec hasCover
			rgTrackGain rgTrackPeak rgAlbumGain rgAlbumPeak mbzTrackId mbzAlbumId mbzArtistId mbzAlbumArtistId
			sortTitle sortAlbum sortArtist sortAlbumArtist missing createdAt updatedAt
			starred starredAt rating playCount playedAt coverArt contentType`, ""},
		{"Album", Album{}, `id libraryId name sortName artist artistId year genre compilation songCount discCount
			duration size mbzAlbumId createdAt updatedAt starred starredAt rating playCount playedAt coverArt`, ""},
		{"Artist", Artist{}, `id name sortName indexKey albumCount songCount mbzArtistId createdAt updatedAt
			starred starredAt rating playCount playedAt coverArt`, ""},
		{"Genre", Genre{}, `id name songCount albumCount`, ""},
		{"Playlist", Playlist{}, `id name comment ownerId ownerName public songCount duration createdAt updatedAt coverArt`, ""},
		{"PlayQueue", PlayQueue{}, `trackIds currentId positionMs updatedAt`, "changedBy"},
		{"RadioStation", RadioStation{}, `id name streamUrl homepageUrl createdAt updatedAt`, ""},
		{"EditLogEntry", EditLogEntry{}, `id userId username action trackId path details createdAt`, ""},
		{"TrashEntry", TrashEntry{}, `id libraryId originalPath trashPath size title artist album trackId deletedBy deletedAt`, ""},
		{"Library", Library{}, `id name path createdAt updatedAt lastScanAt`, ""},
		{"Settings", Settings{}, `scanInterval genreSeparators ignoredArticles coverArtFiles transcodeFormat
			transcodeBitrate renamePattern fixEncodingOnScan enableDownloads onlineMetadata onlineMetadataChinaIp`, ""},
	}
	for _, c := range cases {
		want := fields(c.want + " " + c.extra)
		got := jsonKeys(t, c.v)
		if strings.Join(got, ",") != strings.Join(want, ",") {
			t.Errorf("%s JSON keys\n got: %v\nwant: %v", c.name, got, want)
		}
	}
}
