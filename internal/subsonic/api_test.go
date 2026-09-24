package subsonic

import (
	"context"
	"fmt"
	"strconv"
	"testing"

	"rainy/internal/store"
)

func TestErrorCodes(t *testing.T) {
	f := newFixture(t)
	f.seedAnnotations()
	alice, bob, admin := f.aliceParams(), creds("bob", "bobpass"), f.adminParams()
	tests := []struct {
		name   string
		method string
		params map[string][]string
		kv     []string
		code   int
	}{
		{"getAlbum missing id", "getAlbum", alice, nil, codeMissingParam},
		{"getAlbum unknown", "getAlbum", alice, []string{"id", "nope"}, codeNotFound},
		{"getArtist unknown", "getArtist", alice, []string{"id", "nope"}, codeNotFound},
		{"getSong unknown", "getSong", alice, []string{"id", "nope"}, codeNotFound},
		{"getMusicDirectory unknown", "getMusicDirectory", alice, []string{"id", "nope"}, codeNotFound},
		{"bad musicFolderId", "getArtists", alice, []string{"musicFolderId", "99"}, codeNotFound},
		{"bad int", "getAlbumList2", alice, []string{"type", "newest", "size", "ten"}, codeGeneric},
		{"album list no type", "getAlbumList2", alice, nil, codeMissingParam},
		{"album list bad type", "getAlbumList2", alice, []string{"type", "weird"}, codeGeneric},
		{"byYear without years", "getAlbumList2", alice, []string{"type", "byYear"}, codeMissingParam},
		{"byGenre without genre", "getAlbumList2", alice, []string{"type", "byGenre"}, codeMissingParam},
		{"createUser as user", "createUser", alice, []string{"username", "x", "password", "xxxx", "email", "x@y"}, codeNotAuthorized},
		{"getUsers as user", "getUsers", alice, nil, codeNotAuthorized},
		{"getUser other as user", "getUser", alice, []string{"username", "bob"}, codeNotAuthorized},
		{"startScan as user", "startScan", alice, nil, codeNotAuthorized},
		{"radio create as user", "createInternetRadioStation", alice, []string{"streamUrl", "https://radio.example.com/s", "name", "x"}, codeNotAuthorized},
		{"private playlist of other user", "getPlaylist", bob, []string{"id", "playlist1"}, codeNotFound},
		{"delete other user's playlist", "deletePlaylist", bob, []string{"id", "playlist1"}, codeNotFound},
		{"rating out of range", "setRating", alice, []string{"id", "track1", "rating", "6"}, codeGeneric},
		{"star unknown", "star", alice, []string{"id", "nope"}, codeNotFound},
		{"star nothing", "star", alice, nil, codeMissingParam},
		{"scrobble unknown", "scrobble", alice, []string{"id", "nope"}, codeNotFound},
		{"jukebox", "jukeboxControl", alice, []string{"action", "status"}, codeGeneric},
		{"delete self", "deleteUser", admin, []string{"username", "admin"}, codeGeneric},
		{"demote self", "updateUser", admin, []string{"username", "admin", "adminRole", "false"}, codeGeneric},
		{"short password", "createUser", admin, []string{"username", "carol", "password", "x", "email", "c@x"}, codeGeneric},
		{"duplicate user", "createUser", admin, []string{"username", "bob", "password", "xxxxx", "email", "c@x"}, codeGeneric},
		{"queue index out of range", "savePlayQueueByIndex", alice, []string{"id", "track1", "currentIndex", "3"}, codeMissingParam},
		{"queue index missing", "savePlayQueueByIndex", alice, []string{"id", "track1"}, codeMissingParam},
		{"avatar", "getAvatar", alice, []string{"username", "alice"}, codeNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f.t = t
			if got := f.failCode(tt.method, tt.params, tt.kv...); got != tt.code {
				t.Fatalf("code = %d, want %d", got, tt.code)
			}
		})
	}

	// A public playlist becomes readable but stays read-only for others.
	f.t = t
	f.ok("updatePlaylist", alice, "playlistId", "playlist1", "public", "true")
	env := f.ok("getPlaylist", bob, "id", "playlist1")
	if jpath(t, env, "playlist", "readonly") != true {
		t.Fatal("public playlist of another user must be readonly")
	}
	if code := f.failCode("deletePlaylist", bob, "id", "playlist1"); code != codeNotAuthorized {
		t.Fatalf("delete public playlist of other user: %d", code)
	}
	// Admins may modify anyone's playlist.
	f.ok("updatePlaylist", admin, "playlistId", "playlist1", "name", "Renamed by admin")
}

func TestAlbumListTypes(t *testing.T) {
	f := newFixture(t)
	f.seedAnnotations()
	p := f.aliceParams()
	ids := func(env jsonEnvelope) []string {
		var out []string
		list, _ := jpath(t, env, "albumList2", "album").([]any)
		for _, a := range list {
			out = append(out, a.(map[string]any)["id"].(string))
		}
		return out
	}
	check := func(name string, got []string, want ...string) {
		t.Helper()
		if fmt.Sprint(got) != fmt.Sprint(want) {
			t.Errorf("%s: got %v, want %v", name, got, want)
		}
	}
	check("alphabeticalByName", ids(f.ok("getAlbumList2", p, "type", "alphabeticalByName")), albumFar, albumSkies, albumWet)
	check("alphabeticalByArtist", ids(f.ok("getAlbumList2", p, "type", "alphabeticalByArtist")), albumSkies, albumWet, albumFar)
	check("newest", ids(f.ok("getAlbumList2", p, "type", "newest")), albumFar, albumSkies, albumWet)
	check("starred", ids(f.ok("getAlbumList2", p, "type", "starred")), albumWet)
	check("highest", ids(f.ok("getAlbumList2", p, "type", "highest")), albumSkies)
	check("byYear asc", ids(f.ok("getAlbumList2", p, "type", "byYear", "fromYear", "2018", "toYear", "2021")), albumSkies, albumWet)
	check("byYear desc", ids(f.ok("getAlbumList2", p, "type", "byYear", "fromYear", "2030", "toYear", "2000")), albumFar, albumWet, albumSkies)
	check("byGenre", ids(f.ok("getAlbumList2", p, "type", "byGenre", "genre", "Indie")), albumSkies, albumWet)
	check("musicFolderId", ids(f.ok("getAlbumList2", p, "type", "alphabeticalByName", "musicFolderId", strconv.FormatInt(f.lib2, 10))), albumFar)
	check("size/offset", ids(f.ok("getAlbumList2", p, "type", "alphabeticalByName", "size", "1", "offset", "1")), albumSkies)
	check("recent (none played)", ids(f.ok("getAlbumList2", p, "type", "recent")))
	if n := length(t, jpath(t, f.ok("getAlbumList2", p, "type", "random"), "albumList2", "album")); n != 3 {
		t.Errorf("random: %d albums", n)
	}

	// Scrobbling makes albums show up in recent/frequent.
	f.ok("scrobble", p, "id", "track3", "submission", "true")
	check("recent", ids(f.ok("getAlbumList2", p, "type", "recent")), albumSkies)
	check("frequent", ids(f.ok("getAlbumList2", p, "type", "frequent")), albumSkies)

	// Legacy getAlbumList returns folder-style children.
	env := f.ok("getAlbumList", p, "type", "alphabeticalByName", "size", "1")
	if jpath(t, env, "albumList", "album", 0, "isDir") != true {
		t.Fatal("getAlbumList entries must be directories")
	}
}

func TestSearch(t *testing.T) {
	f := newFixture(t)
	p := f.aliceParams()
	counts := func(env jsonEnvelope, key string) (int, int, int) {
		r := jpath(t, env, key)
		return length(t, jpath(t, r, "artist")), length(t, jpath(t, r, "album")), length(t, jpath(t, r, "song"))
	}
	// Empty query (and "" quoted) returns everything, paged.
	for _, q := range []string{"", `""`} {
		ar, al, so := counts(f.ok("search3", p, "query", q), "searchResult3")
		if ar != 3 || al != 3 || so != 4 {
			t.Fatalf("search3 %q: %d artists %d albums %d songs", q, ar, al, so)
		}
	}
	ar, al, so := counts(f.ok("search3", p, "query", "", "songCount", "2", "songOffset", "3", "artistCount", "0", "albumCount", "1"), "searchResult3")
	if ar != 0 || al != 1 || so != 1 {
		t.Fatalf("paged search3: %d %d %d", ar, al, so)
	}
	ar, al, so = counts(f.ok("search3", p, "query", "rain"), "searchResult3")
	if ar != 1 || al != 1 || so != 2 {
		t.Fatalf("search3 rain: %d %d %d", ar, al, so)
	}
	ar, al, so = counts(f.ok("search3", p, "query", "", "musicFolderId", strconv.FormatInt(f.lib2, 10)), "searchResult3")
	if ar != 1 || al != 1 || so != 1 {
		t.Fatalf("search3 folder: %d %d %d", ar, al, so)
	}
	ar, al, so = counts(f.ok("search2", p, "query", "clouds"), "searchResult2")
	if ar != 0 || al != 0 || so != 1 {
		t.Fatalf("search2: %d %d %d", ar, al, so)
	}
	env := f.ok("search", p, "any", "puddles")
	if jpath(t, env, "searchResult", "totalHits") != float64(1) {
		t.Fatalf("search: %v", env)
	}
}

func TestBrowsing(t *testing.T) {
	f := newFixture(t)
	p := f.aliceParams()

	env := f.ok("getIndexes", p)
	lastMod := jpath(t, env, "indexes", "lastModified").(float64)
	// "Northern Echo" → N, "The Rainy Days" → R (article stripped), "Static Noise" → S
	if n := length(t, jpath(t, env, "indexes", "index")); n != 3 {
		t.Fatalf("index groups: %d", n)
	}
	env = f.ok("getIndexes", p, "ifModifiedSince", strconv.FormatInt(int64(lastMod), 10))
	if n := length(t, jpath(t, env, "indexes", "index")); n != 0 {
		t.Fatalf("ifModifiedSince should return no index, got %d", n)
	}
	env = f.ok("getIndexes", p, "musicFolderId", strconv.FormatInt(f.lib2, 10))
	if n := length(t, jpath(t, env, "indexes", "index")); n != 1 {
		t.Fatalf("musicFolderId index groups: %d", n)
	}

	// artist directory → albums → songs
	env = f.ok("getMusicDirectory", p, "id", artistRainy)
	if jpath(t, env, "directory", "child", 0, "id") != albumWet || jpath(t, env, "directory", "child", 0, "isDir") != true {
		t.Fatalf("artist directory: %v", env)
	}
	env = f.ok("getMusicDirectory", p, "id", albumWet)
	if n := length(t, jpath(t, env, "directory", "child")); n != 2 || jpath(t, env, "directory", "parent") != artistRainy {
		t.Fatalf("album directory: %v", env)
	}

	env = f.ok("getSong", p, "id", "track3")
	if jpath(t, env, "song", "bitDepth") != float64(24) || jpath(t, env, "song", "samplingRate") != float64(96000) {
		t.Fatalf("getSong: %v", env)
	}

	env = f.ok("getRandomSongs", p, "size", "2")
	if n := length(t, jpath(t, env, "randomSongs", "song")); n != 2 {
		t.Fatalf("random songs %d", n)
	}
	env = f.ok("getRandomSongs", p, "genre", "Ambient")
	if n := length(t, jpath(t, env, "randomSongs", "song")); n != 1 {
		t.Fatalf("random songs by genre %d", n)
	}
	env = f.ok("getSongsByGenre", p, "genre", "Rock")
	if n := length(t, jpath(t, env, "songsByGenre", "song")); n != 2 {
		t.Fatalf("songs by genre %d", n)
	}

	// Similar artists / songs share genres ("Indie" links The Rainy Days and Northern Echo).
	env = f.ok("getArtistInfo2", p, "id", artistRainy)
	if jpath(t, env, "artistInfo2", "similarArtist", 0, "id") != artistNorthern {
		t.Fatalf("artistInfo2: %v", env)
	}
	env = f.ok("getSimilarSongs2", p, "id", artistNorthern, "count", "10")
	if n := length(t, jpath(t, env, "similarSongs2", "song")); n != 3 {
		t.Fatalf("similar songs %d", n)
	}
	env = f.ok("getTopSongs", p, "artist", "The Rainy Days")
	if n := length(t, jpath(t, env, "topSongs", "song")); n != 2 {
		t.Fatalf("top songs %d", n)
	}
	env = f.ok("getTopSongs", p, "artist", "Unknown Person")
	if n := length(t, jpath(t, env, "topSongs", "song")); n != 0 {
		t.Fatalf("top songs of unknown artist %d", n)
	}
	f.ok("getAlbumInfo2", p, "id", albumWet)
	f.ok("getAlbumInfo", p, "id", "track1")
}

func TestAnnotationsAndScrobble(t *testing.T) {
	f := newFixture(t)
	p := f.aliceParams()
	ctx := context.Background()

	f.ok("star", p, "id", "track2", "albumId", albumSkies, "artistId", artistRainy)
	f.ok("star", p, "id", albumFar) // untyped album id
	env := f.ok("getStarred2", p)
	if length(t, jpath(t, env, "starred2", "song")) != 1 || length(t, jpath(t, env, "starred2", "album")) != 2 ||
		length(t, jpath(t, env, "starred2", "artist")) != 1 {
		t.Fatalf("starred2: %v", env)
	}
	env = f.ok("getStarred", p)
	if jpath(t, env, "starred", "album", 0, "isDir") != true {
		t.Fatalf("getStarred albums must be directories: %v", env)
	}
	f.ok("unstar", p, "id", "track2", "albumId", albumSkies)
	env = f.ok("getStarred2", p)
	if length(t, jpath(t, env, "starred2", "song")) != 0 || length(t, jpath(t, env, "starred2", "album")) != 1 {
		t.Fatalf("after unstar: %v", env)
	}

	f.ok("setRating", p, "id", "track1", "rating", "3")
	if tr, _ := f.app.Store.GetTrack(ctx, "track1", f.alice.ID); tr.Rating != 3 {
		t.Fatalf("rating %d", tr.Rating)
	}
	f.ok("setRating", p, "id", "track1", "rating", "0")

	// now playing
	f.ok("scrobble", p, "id", "track1", "submission", "false")
	env = f.ok("getNowPlaying", p)
	if jpath(t, env, "nowPlaying", "entry", 0, "username") != "alice" || jpath(t, env, "nowPlaying", "entry", 0, "playerName") != "test" {
		t.Fatalf("now playing: %v", env)
	}
	// submissions with explicit times (ms)
	f.ok("scrobble", p, "id", "track1", "id", "track2", "time", "1690000000000", "time", "1690000100000")
	f.ok("scrobble", p, "id", "track1")
	tr, err := f.app.Store.GetTrack(ctx, "track1", f.alice.ID)
	if err != nil || tr.PlayCount != 2 {
		t.Fatalf("play count %d %v", tr.PlayCount, err)
	}
	tr, _ = f.app.Store.GetTrack(ctx, "track2", f.alice.ID)
	if tr.PlayCount != 1 || tr.PlayedAt != 1690000100000 {
		t.Fatalf("track2 play %d at %d", tr.PlayCount, tr.PlayedAt)
	}
	env = f.ok("getSong", p, "id", "track1")
	if jpath(t, env, "song", "playCount") != float64(2) || jpath(t, env, "song", "played") == nil {
		t.Fatalf("song play fields: %v", env)
	}
}

func TestPlayQueueAndBookmarks(t *testing.T) {
	f := newFixture(t)
	p := f.aliceParams()

	// Nothing saved yet → empty response.
	env := f.ok("getPlayQueue", p)
	if _, ok := env["playQueue"]; ok {
		t.Fatalf("expected no queue: %v", env)
	}
	f.ok("savePlayQueue", p, "id", "track1", "id", "track2", "id", "track3", "current", "track2", "position", "12345")
	env = f.ok("getPlayQueue", p)
	if jpath(t, env, "playQueue", "current") != "track2" || jpath(t, env, "playQueue", "position") != float64(12345) ||
		length(t, jpath(t, env, "playQueue", "entry")) != 3 || jpath(t, env, "playQueue", "changedBy") != "test" ||
		jpath(t, env, "playQueue", "username") != "alice" {
		t.Fatalf("queue: %v", env)
	}
	env = f.ok("getPlayQueueByIndex", p)
	if jpath(t, env, "playQueueByIndex", "currentIndex") != float64(1) {
		t.Fatalf("queue by index: %v", env)
	}
	f.ok("savePlayQueueByIndex", p, "id", "track3", "id", "track1", "currentIndex", "1", "position", "500")
	env = f.ok("getPlayQueue", p)
	if jpath(t, env, "playQueue", "current") != "track1" || jpath(t, env, "playQueue", "position") != float64(500) {
		t.Fatalf("queue after by-index save: %v", env)
	}
	// The queue is shared with the web UI (store).
	q, err := f.app.Store.GetPlayQueue(context.Background(), f.alice.ID)
	if err != nil || len(q.TrackIDs) != 2 || q.CurrentID != "track1" {
		t.Fatalf("stored queue %+v %v", q, err)
	}
	// Clearing
	f.ok("savePlayQueueByIndex", p)
	env = f.ok("getPlayQueueByIndex", p)
	if n := length(t, jpath(t, env, "playQueueByIndex", "entry")); n != 0 {
		t.Fatalf("cleared queue has %d entries", n)
	}

	// bookmarks
	f.ok("createBookmark", p, "id", "track3", "position", "60000", "comment", "halfway")
	env = f.ok("getBookmarks", p)
	if jpath(t, env, "bookmarks", "bookmark", 0, "position") != float64(60000) ||
		jpath(t, env, "bookmarks", "bookmark", 0, "entry", "id") != "track3" ||
		jpath(t, env, "bookmarks", "bookmark", 0, "comment") != "halfway" {
		t.Fatalf("bookmarks: %v", env)
	}
	if code := f.failCode("createBookmark", p, "id", "track3"); code != codeMissingParam {
		t.Fatalf("bookmark without position: %d", code)
	}
	f.ok("deleteBookmark", p, "id", "track3")
	f.ok("deleteBookmark", p, "id", "track3") // idempotent
	if n := length(t, jpath(t, f.ok("getBookmarks", p), "bookmarks", "bookmark")); n != 0 {
		t.Fatalf("bookmarks after delete: %d", n)
	}
}

func TestPlaylists(t *testing.T) {
	f := newFixture(t)
	p := f.aliceParams()

	env := f.ok("createPlaylist", p, "name", "Road trip", "songId", "track1", "songId", "track3", "songId", "nope")
	id := jpath(t, env, "playlist", "id").(string)
	if jpath(t, env, "playlist", "songCount") != float64(2) || jpath(t, env, "playlist", "owner") != "alice" ||
		jpath(t, env, "playlist", "readonly") != false {
		t.Fatalf("created playlist: %v", env)
	}
	// remove index 0 (track1), add track2 → [track3, track2]
	f.ok("updatePlaylist", p, "playlistId", id, "songIndexToRemove", "0", "songIdToAdd", "track2",
		"comment", "summer", "public", "true", "name", "Road trip 2")
	env = f.ok("getPlaylist", p, "id", id)
	entries := jpath(t, env, "playlist", "entry").([]any)
	if len(entries) != 2 || entries[0].(map[string]any)["id"] != "track3" || entries[1].(map[string]any)["id"] != "track2" ||
		jpath(t, env, "playlist", "comment") != "summer" || jpath(t, env, "playlist", "public") != true ||
		jpath(t, env, "playlist", "name") != "Road trip 2" {
		t.Fatalf("updated playlist: %v", env)
	}
	// createPlaylist with playlistId replaces the songs.
	f.ok("createPlaylist", p, "playlistId", id, "songId", "track2")
	env = f.ok("getPlaylists", p)
	if n := length(t, jpath(t, env, "playlists", "playlist")); n != 1 || jpath(t, env, "playlists", "playlist", 0, "songCount") != float64(1) {
		t.Fatalf("playlists: %v", env)
	}
	// Public playlists are listed for others; admins can list a user's playlists.
	if n := length(t, jpath(t, f.ok("getPlaylists", creds("bob", "bobpass")), "playlists", "playlist")); n != 1 {
		t.Fatalf("bob sees %d playlists", n)
	}
	if n := length(t, jpath(t, f.ok("getPlaylists", f.adminParams(), "username", "alice"), "playlists", "playlist")); n != 1 {
		t.Fatalf("admin view of alice: %d", n)
	}
	if code := f.failCode("getPlaylists", creds("bob", "bobpass"), "username", "alice"); code != codeNotAuthorized {
		t.Fatalf("bob listing alice: %d", code)
	}
	f.ok("deletePlaylist", p, "id", id)
	if _, err := f.app.Store.GetPlaylist(context.Background(), id); err != store.ErrNotFound {
		t.Fatalf("playlist not deleted: %v", err)
	}
}

func TestUsersAndSystem(t *testing.T) {
	f := newFixture(t)
	admin := f.adminParams()

	f.ok("createUser", admin, "username", "carol", "password", "enc:"+fmt.Sprintf("%x", "carolpw"), "email", "c@x.org",
		"uploadRole", "true", "downloadRole", "false")
	env := f.ok("getUser", creds("carol", "carolpw"), "username", "carol")
	u := jpath(t, env, "user").(map[string]any)
	if u["uploadRole"] != true || u["coverArtRole"] != true || u["downloadRole"] != false || u["adminRole"] != false ||
		u["streamRole"] != true || u["settingsRole"] != true || u["playlistRole"] != true || u["email"] != "c@x.org" {
		t.Fatalf("carol: %v", u)
	}
	f.ok("updateUser", admin, "username", "carol", "adminRole", "true", "password", "newpass")
	env = f.ok("getUser", creds("carol", "newpass"), "username", "carol")
	if jpath(t, env, "user", "adminRole") != true {
		t.Fatalf("carol not admin: %v", env)
	}
	env = f.ok("getUsers", admin)
	if n := length(t, jpath(t, env, "users", "user")); n != 4 {
		t.Fatalf("users: %d", n)
	}
	// Self password change, then admin changes it back.
	f.ok("changePassword", creds("bob", "bobpass"), "username", "bob", "password", "bobpass2")
	f.ok("ping", creds("bob", "bobpass2"))
	if code := f.failCode("changePassword", creds("bob", "bobpass2"), "username", "alice", "password", "hacked"); code != codeNotAuthorized {
		t.Fatalf("bob changing alice's password: %d", code)
	}
	f.ok("changePassword", admin, "username", "bob", "password", "bobpass")
	f.ok("deleteUser", admin, "username", "carol")
	if code := f.failCode("getUser", admin, "username", "carol"); code != codeNotFound {
		t.Fatalf("deleted user: %d", code)
	}

	// Scan status.
	env = f.ok("getScanStatus", f.aliceParams())
	if jpath(t, env, "scanStatus", "scanning") != false || jpath(t, env, "scanStatus", "count") != float64(4) {
		t.Fatalf("scan status: %v", env)
	}

	// Internet radio.
	f.ok("createInternetRadioStation", admin, "streamUrl", "https://radio.example/stream", "name", "Example FM", "homepageUrl", "https://radio.example")
	env = f.ok("getInternetRadioStations", f.aliceParams())
	st := jpath(t, env, "internetRadioStations", "internetRadioStation", 0).(map[string]any)
	if st["name"] != "Example FM" || st["streamUrl"] != "https://radio.example/stream" || st["homePageUrl"] != "https://radio.example" {
		t.Fatalf("radio: %v", st)
	}
	id := st["id"].(string)
	if code := f.failCode("createInternetRadioStation", admin, "streamUrl", "javascript:alert(1)", "name", "x"); code != codeGeneric {
		t.Fatalf("bad radio url: %d", code)
	}
	f.ok("updateInternetRadioStation", admin, "id", id, "streamUrl", "https://radio.example/v2", "name", "Example 2")
	f.ok("deleteInternetRadioStation", admin, "id", id)
	if n := length(t, jpath(t, f.ok("getInternetRadioStations", admin), "internetRadioStations", "internetRadioStation")); n != 0 {
		t.Fatalf("stations left: %d", n)
	}

	// Stubs return empty lists.
	for method, key := range map[string]string{"getPodcasts": "podcasts", "getNewestPodcasts": "newestPodcasts",
		"getShares": "shares", "getChatMessages": "chatMessages", "getVideos": "videos"} {
		if _, ok := f.ok(method, admin)[key]; !ok {
			t.Errorf("%s: missing %s", method, key)
		}
	}
	f.ok("getLicense", admin)
}

// Typed star ids must name an existing item of that type (no orphan annotations).
func TestStarTypedIDsValidated(t *testing.T) {
	f := newFixture(t)
	p := f.aliceParams()
	for _, kv := range [][]string{
		{"albumId", "no-such-album"},
		{"artistId", "no-such-artist"},
		{"albumId", artistRainy}, // an artist id passed as album
		{"artistId", "track1"},
	} {
		if code := f.failCode("star", p, kv...); code != codeNotFound {
			t.Errorf("star %v: code %d, want %d", kv, code, codeNotFound)
		}
	}
	var n int
	if err := f.app.DB.R.Get(&n, `SELECT COUNT(*) FROM annotations`); err != nil || n != 0 {
		t.Fatalf("annotations written: %d %v", n, err)
	}
}

// A rejected password must not leave the other updateUser changes applied.
func TestUpdateUserAtomicValidation(t *testing.T) {
	f := newFixture(t)
	if code := f.failCode("updateUser", f.adminParams(), "username", "bob", "adminRole", "true", "password", "x"); code != codeGeneric {
		t.Fatalf("weak password: code %d", code)
	}
	u, err := f.app.Store.GetUserByUsername(context.Background(), "bob")
	if err != nil || u.IsAdmin {
		t.Fatalf("bob promoted despite the failed update: %+v %v", u, err)
	}
	f.ok("ping", creds("bob", "bobpass"))
}

// getIndexes always carries an index array in JSON, even when nothing changed.
func TestIndexesNotModified(t *testing.T) {
	f := newFixture(t)
	env := f.ok("getIndexes", f.aliceParams(), "ifModifiedSince", "99999999999999")
	idx, ok := jpath(t, env, "indexes").(map[string]any)["index"]
	if !ok || length(t, idx) != 0 {
		t.Fatalf("indexes: %v", env)
	}
}

// A queue whose current song went missing never reports a current id outside its entries.
func TestPlayQueueMissingCurrent(t *testing.T) {
	f := newFixture(t)
	p := f.aliceParams()
	f.ok("savePlayQueue", p, "id", "track1", "id", "track2", "id", "track3", "current", "track2", "position", "4000")
	f.exec(`UPDATE tracks SET missing = 1 WHERE id = 'track2'`)
	env := f.ok("getPlayQueue", p)
	if jpath(t, env, "playQueue", "current") != "track1" || jpath(t, env, "playQueue", "position") != float64(0) ||
		length(t, jpath(t, env, "playQueue", "entry")) != 2 {
		t.Fatalf("playQueue: %v", env)
	}
	env = f.ok("getPlayQueueByIndex", p)
	if jpath(t, env, "playQueueByIndex", "currentIndex") != float64(0) || jpath(t, env, "playQueueByIndex", "position") != float64(0) {
		t.Fatalf("playQueueByIndex: %v", env)
	}
	// A present current song keeps its position.
	f.ok("savePlayQueue", p, "id", "track1", "id", "track3", "current", "track3", "position", "4000")
	env = f.ok("getPlayQueueByIndex", p)
	if jpath(t, env, "playQueueByIndex", "currentIndex") != float64(1) || jpath(t, env, "playQueueByIndex", "position") != float64(4000) {
		t.Fatalf("playQueueByIndex: %v", env)
	}
}

// A queue holding the same song twice must report the saved entry as current, not the
// first copy (the index is stored with the queue; missing songs before it are skipped).
func TestPlayQueueByIndexDuplicates(t *testing.T) {
	f := newFixture(t)
	p := f.aliceParams()
	f.ok("savePlayQueueByIndex", p, "id", "track1", "id", "track2", "id", "track1", "currentIndex", "2", "position", "700")
	env := f.ok("getPlayQueueByIndex", p)
	if jpath(t, env, "playQueueByIndex", "currentIndex") != float64(2) || jpath(t, env, "playQueueByIndex", "position") != float64(700) {
		t.Fatalf("duplicate current: %v", env)
	}
	// A missing song before the current entry shifts the visible index.
	f.exec(`UPDATE tracks SET missing = 1 WHERE id = 'track2'`)
	env = f.ok("getPlayQueueByIndex", p)
	if jpath(t, env, "playQueueByIndex", "currentIndex") != float64(1) || length(t, jpath(t, env, "playQueueByIndex", "entry")) != 2 {
		t.Fatalf("after missing: %v", env)
	}
	f.exec(`UPDATE tracks SET missing = 0 WHERE id = 'track2'`)
	// An id-based save (web UI, savePlayQueue) points at the first copy.
	f.ok("savePlayQueue", p, "id", "track1", "id", "track2", "id", "track1", "current", "track1")
	env = f.ok("getPlayQueueByIndex", p)
	if jpath(t, env, "playQueueByIndex", "currentIndex") != float64(0) {
		t.Fatalf("id-based save: %v", env)
	}
}
