package subsonic

import (
	"fmt"
	"slices"
	"testing"
)

// folderIDsOf returns the numeric folder ids of a decoded JSON array.
func folderIDsOf(t *testing.T, v any) []int64 {
	t.Helper()
	var out []int64
	if v == nil {
		return out
	}
	for _, x := range v.([]any) {
		switch n := x.(type) {
		case float64:
			out = append(out, int64(n))
		case map[string]any:
			out = append(out, int64(n["id"].(float64)))
		}
	}
	return out
}

// musicFolderId on updateUser limits a user to those libraries; Subsonic clients of that
// user then see nothing of the other libraries.
func TestLibraryAccessSubsonic(t *testing.T) {
	f := newFixture(t)
	lib1, lib2 := fmt.Sprint(f.lib1), fmt.Sprint(f.lib2)
	f.ok("updateUser", f.adminParams(), "username", "alice", "musicFolderId", lib1)
	alice := f.aliceParams()

	if got := folderIDsOf(t, jpath(t, f.ok("getMusicFolders", alice), "musicFolders", "musicFolder")); !slices.Equal(got, []int64{f.lib1}) {
		t.Errorf("folders = %v", got)
	}
	if got := folderIDsOf(t, jpath(t, f.ok("getUser", alice, "username", "alice"), "user", "folder")); !slices.Equal(got, []int64{f.lib1}) {
		t.Errorf("getUser folders = %v", got)
	}
	for _, c := range [][]string{
		{"getSong", "id", "track4"}, {"getAlbum", "id", albumFar}, {"getArtist", "id", artistStatic},
		{"getMusicDirectory", "id", albumFar}, {"getIndexes", "musicFolderId", lib2},
		{"stream", "id", "track4"}, {"download", "id", "track4"},
		{"star", "id", "track4"}, {"setRating", "id", albumFar, "rating", "3"},
	} {
		if code := f.failCode(c[0], alice, c[1:]...); code != codeNotFound {
			t.Errorf("%v: code %d, want %d", c, code, codeNotFound)
		}
	}
	songs := jpath(t, f.ok("search3", alice, "query", "", "songCount", "50"), "searchResult3", "song")
	if n := length(t, songs); n != 3 {
		t.Errorf("search3 found %d songs, want 3", n)
	}
	for _, idx := range jpath(t, f.ok("getArtists", alice), "artists", "index").([]any) {
		for _, ar := range idx.(map[string]any)["artist"].([]any) {
			if ar.(map[string]any)["id"] == artistStatic {
				t.Errorf("getArtists lists an artist of the other library")
			}
		}
	}
	if n := jpath(t, f.ok("getScanStatus", alice), "scanStatus", "count"); n != float64(3) {
		t.Errorf("scan status count = %v", n)
	}

	// Bob plays a song of the other library: alice does not see it as now playing.
	f.ok("scrobble", creds("bob", "bobpass"), "id", "track4", "submission", "false")
	if n := length(t, jpath(t, f.ok("getNowPlaying", alice), "nowPlaying", "entry")); n != 0 {
		t.Errorf("alice sees %d now playing entries", n)
	}

	// Administrators see each user's folders; naming every library lifts the restriction.
	users := jpath(t, f.ok("getUsers", f.adminParams()), "users", "user").([]any)
	for _, u := range users {
		u := u.(map[string]any)
		want := []int64{f.lib1, f.lib2}
		if u["username"] == "alice" {
			want = []int64{f.lib1}
		}
		if got := folderIDsOf(t, u["folder"]); !slices.Equal(got, want) {
			t.Errorf("%v folders = %v, want %v", u["username"], got, want)
		}
	}
	f.ok("updateUser", f.adminParams(), "username", "alice", "musicFolderId", lib1, "musicFolderId", lib2)
	f.ok("getSong", alice, "id", "track4")
	if u, err := f.app.Store.GetUser(t.Context(), f.alice.ID); err != nil || u.RestrictLibraries {
		t.Errorf("alice after naming every folder: %+v, %v", u, err)
	}
}
