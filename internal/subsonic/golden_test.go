package subsonic

import "testing"

// TestGoldenResponses snapshots the XML and JSON shapes of the key endpoints
// (go test ./internal/subsonic -update rewrites testdata/*.golden).
func TestGoldenResponses(t *testing.T) {
	f := newFixture(t)
	f.seedAnnotations()
	p := f.aliceParams()

	cases := []struct {
		name   string
		method string
		kv     []string
	}{
		{"ping", "ping", nil},
		{"getMusicFolders", "getMusicFolders", nil},
		{"getArtists", "getArtists", nil},
		{"getIndexes", "getIndexes", nil},
		{"getArtist", "getArtist", []string{"id", artistRainy}},
		{"getAlbum", "getAlbum", []string{"id", albumWet}},
		{"getMusicDirectory", "getMusicDirectory", []string{"id", albumSkies}},
		{"getAlbumList2", "getAlbumList2", []string{"type", "alphabeticalByName"}},
		{"search3_empty", "search3", []string{"query", `""`}},
		{"getPlaylist", "getPlaylist", []string{"id", "playlist1"}},
		{"getStarred2", "getStarred2", nil},
		{"getLyricsBySongId", "getLyricsBySongId", []string{"id", "track1"}},
		{"getGenres", "getGenres", nil},
		{"getUser", "getUser", []string{"username", "alice"}},
		{"error_notfound", "getAlbum", []string{"id", "nope"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f.t = t
			xmlRec := f.get(c.method, merge(p, c.kv...))
			golden(t, c.name+".xml.golden", prettyXML(t, xmlRec.Body.Bytes()))
			jsonRec := f.get(c.method, merge(p, append([]string{"f", "json"}, c.kv...)...))
			golden(t, c.name+".json.golden", prettyJSON(t, jsonRec.Body.Bytes()))
		})
	}
}
