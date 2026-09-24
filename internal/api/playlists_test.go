package api

import (
	"slices"
	"strings"
	"testing"

	"rainy/internal/model"
)

func TestNativePlaylists(t *testing.T) {
	e := newNativeEnv(t)
	nativeErrorCode(t, e.do("GET", "/playlists", "", nil), 401)

	// Create.
	nativeErrorCode(t, e.do("POST", "/playlists", e.aliceTok, map[string]any{"name": "   "}), 400)
	nativeErrorCode(t, e.do("POST", "/playlists", e.aliceTok, map[string]any{"name": strings.Repeat("x", maxPlaylistNameLen+1)}), 400)
	pl := nativeJSON[model.Playlist](t, e.do("POST", "/playlists", e.aliceTok, map[string]any{
		"name": " Road trip ", "comment": "summer", "trackIds": []string{e.id("one"), "unknown", e.id("tone"), e.id("one")},
	}), 201)
	if pl.ID == "" || pl.Name != "Road trip" || pl.OwnerID != e.alice.ID || pl.OwnerName != "alice" || pl.Public ||
		pl.SongCount != 3 || pl.Duration != 363 || !strings.HasPrefix(pl.CoverArt, "pl-"+pl.ID+"_") {
		t.Fatalf("created %+v", pl)
	}
	pub := nativeJSON[model.Playlist](t, e.do("POST", "/playlists", e.aliceTok, map[string]any{"name": "Shared", "public": true}), 201)

	// Listing: own + public.
	lists := nativeJSON[[]model.Playlist](t, e.do("GET", "/playlists", e.aliceTok, nil), 200)
	if len(lists) != 2 {
		t.Fatalf("alice lists %d", len(lists))
	}
	lists = nativeJSON[[]model.Playlist](t, e.do("GET", "/playlists", e.bobTok, nil), 200)
	if len(lists) != 1 || lists[0].ID != pub.ID {
		t.Fatalf("bob lists %+v", lists)
	}

	// Detail and visibility.
	d := nativeJSON[playlistDetail](t, e.do("GET", "/playlists/"+pl.ID, e.aliceTok, nil), 200)
	if d.Readonly || !slices.Equal(nativeTrackIDs(d.Tracks), []string{e.id("one"), e.id("tone"), e.id("one")}) {
		t.Fatalf("detail %+v", nativeTrackIDs(d.Tracks))
	}
	nativeErrorCode(t, e.do("GET", "/playlists/"+pl.ID, e.bobTok, nil), 404)
	if d = nativeJSON[playlistDetail](t, e.do("GET", "/playlists/"+pl.ID, e.adminTok, nil), 200); d.Readonly {
		t.Fatal("admins may edit any playlist")
	}
	if d = nativeJSON[playlistDetail](t, e.do("GET", "/playlists/"+pub.ID, e.bobTok, nil), 200); !d.Readonly || d.Tracks == nil {
		t.Fatalf("public playlist for bob: %+v", d)
	}
	nativeErrorCode(t, e.do("GET", "/playlists/nope", e.aliceTok, nil), 404)

	// Only the owner or an admin may modify.
	nativeErrorCode(t, e.do("PUT", "/playlists/"+pub.ID, e.bobTok, map[string]any{"name": "Mine now"}), 403)
	nativeErrorCode(t, e.do("DELETE", "/playlists/"+pub.ID, e.bobTok, nil), 403)
	nativeErrorCode(t, e.do("POST", "/playlists/"+pub.ID+"/tracks", e.bobTok, map[string]any{"trackIds": []string{e.id("one")}}), 403)
	nativeErrorCode(t, e.do("PUT", "/playlists/"+pub.ID+"/tracks", e.bobTok, map[string]any{"trackIds": []string{}}), 403)
	nativeErrorCode(t, e.do("DELETE", "/playlists/"+pub.ID+"/tracks", e.bobTok, map[string]any{"positions": []int{0}}), 403)
	nativeErrorCode(t, e.do("PUT", "/playlists/"+pl.ID, e.bobTok, map[string]any{"name": "x"}), 404) // private: not revealed

	// Update (partial).
	up := nativeJSON[model.Playlist](t, e.do("PUT", "/playlists/"+pl.ID, e.aliceTok, map[string]any{"public": true}), 200)
	if up.Name != "Road trip" || up.Comment != "summer" || !up.Public || up.UpdatedAt < pl.UpdatedAt {
		t.Fatalf("updated %+v", up)
	}
	up = nativeJSON[model.Playlist](t, e.do("PUT", "/playlists/"+pl.ID, e.adminTok, map[string]any{"name": "Renamed by admin", "comment": ""}), 200)
	if up.Name != "Renamed by admin" || up.Comment != "" || up.OwnerID != e.alice.ID {
		t.Fatalf("admin update %+v", up)
	}
	nativeErrorCode(t, e.do("PUT", "/playlists/"+pl.ID, e.aliceTok, map[string]any{"name": ""}), 400)

	// Tracks: append, remove positions, replace.
	up = nativeJSON[model.Playlist](t, e.do("POST", "/playlists/"+pl.ID+"/tracks", e.aliceTok, map[string]any{"trackIds": []string{e.id("puddles")}}), 200)
	if up.SongCount != 4 {
		t.Fatalf("append %+v", up)
	}
	up = nativeJSON[model.Playlist](t, e.do("DELETE", "/playlists/"+pl.ID+"/tracks", e.aliceTok, map[string]any{"positions": []int{0, 2, 99}}), 200)
	d = nativeJSON[playlistDetail](t, e.do("GET", "/playlists/"+pl.ID, e.aliceTok, nil), 200)
	if up.SongCount != 2 || !slices.Equal(nativeTrackIDs(d.Tracks), []string{e.id("tone"), e.id("puddles")}) {
		t.Fatalf("remove: %v", nativeTrackIDs(d.Tracks))
	}
	nativeJSON[model.Playlist](t, e.do("PUT", "/playlists/"+pl.ID+"/tracks", e.aliceTok, map[string]any{"trackIds": []string{e.id("feature"), e.id("tone")}}), 200)
	d = nativeJSON[playlistDetail](t, e.do("GET", "/playlists/"+pl.ID, e.aliceTok, nil), 200)
	if !slices.Equal(nativeTrackIDs(d.Tracks), []string{e.id("feature"), e.id("tone")}) {
		t.Fatalf("replace: %v", nativeTrackIDs(d.Tracks))
	}
	up = nativeJSON[model.Playlist](t, e.do("PUT", "/playlists/"+pl.ID+"/tracks", e.aliceTok, map[string]any{"trackIds": nil}), 200)
	if up.SongCount != 0 {
		t.Fatal("clear")
	}
	nativeErrorCode(t, e.do("POST", "/playlists/nope/tracks", e.aliceTok, map[string]any{"trackIds": []string{}}), 404)

	// Delete.
	nativeExpect(t, e.do("DELETE", "/playlists/"+pub.ID, e.adminTok, nil), 204)
	nativeExpect(t, e.do("DELETE", "/playlists/"+pl.ID, e.aliceTok, nil), 204)
	nativeErrorCode(t, e.do("DELETE", "/playlists/"+pl.ID, e.aliceTok, nil), 404)
	if lists = nativeJSON[[]model.Playlist](t, e.do("GET", "/playlists", e.aliceTok, nil), 200); len(lists) != 0 || lists == nil {
		t.Fatalf("after delete %+v", lists)
	}
}
