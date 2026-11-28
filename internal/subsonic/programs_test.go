package subsonic

import (
	"context"
	"testing"
)

// Program episodes leave the random, album-list, similar and top-song lists, while search,
// folder browsing, starred items and lookups by id still reach them.
func TestProgramsLeaveLists(t *testing.T) {
	f := newFixture(t)
	if err := f.app.Store.AddProgramFolder(context.Background(), f.lib1, "Northern Echo/Skies"); err != nil {
		t.Fatal(err)
	}
	p := f.aliceParams()
	f.ok("star", p, "id", "track3")
	f.ok("star", p, "albumId", albumSkies)
	f.ok("setRating", p, "id", albumSkies, "rating", "5")

	songIDs := func(v any) []string {
		var out []string
		for i := range length(t, v) {
			out = append(out, jpath(t, v, i, "id").(string))
		}
		return out
	}
	has := func(ids []string, id string) bool {
		for _, x := range ids {
			if x == id {
				return true
			}
		}
		return false
	}

	for range 5 {
		env := f.ok("getRandomSongs", p, "size", "500")
		if ids := songIDs(jpath(t, env, "randomSongs", "song")); len(ids) != 3 || has(ids, "track3") {
			t.Fatalf("random songs = %v", ids)
		}
	}
	for _, typ := range []string{"random", "newest", "alphabeticalByName", "alphabeticalByArtist", "highest", "starred"} {
		for _, method := range []string{"getAlbumList", "getAlbumList2"} {
			env := f.ok(method, p, "type", typ, "size", "500")
			key := map[string]string{"getAlbumList": "albumList", "getAlbumList2": "albumList2"}[method]
			if list := jpath(t, env, key); list != nil {
				if albums, ok := list.(map[string]any)["album"]; ok && has(songIDs(albums), albumSkies) {
					t.Errorf("%s type=%s lists the program show", method, typ)
				}
			}
		}
	}
	env := f.ok("getSimilarSongs2", p, "id", artistRainy, "count", "50")
	if ids := songIDs(jpath(t, env, "similarSongs2", "song")); len(ids) != 2 || has(ids, "track3") {
		t.Errorf("similar songs = %v", ids)
	}
	env = f.ok("getSimilarSongs", p, "id", artistRainy, "count", "50")
	if ids := songIDs(jpath(t, env, "similarSongs", "song")); has(ids, "track3") {
		t.Errorf("similar songs (folder) = %v", ids)
	}
	env = f.ok("getTopSongs", p, "artist", "Northern Echo")
	if n := length(t, jpath(t, env, "topSongs", "song")); n != 0 {
		t.Errorf("top songs of the show's artist = %d", n)
	}

	f.ok("getSong", p, "id", "track3")
	env = f.ok("search3", p, "query", "Clouds")
	if n := length(t, jpath(t, env, "searchResult3", "song")); n != 1 {
		t.Errorf("search = %v", env)
	}
	env = f.ok("getMusicDirectory", p, "id", albumSkies)
	if n := length(t, jpath(t, env, "directory", "child")); n != 1 {
		t.Errorf("folder browsing = %v", env)
	}
	env = f.ok("getStarred2", p)
	if ids := songIDs(jpath(t, env, "starred2", "song")); !has(ids, "track3") {
		t.Errorf("starred = %v", ids)
	}
}
