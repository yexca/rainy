package api

import (
	"context"
	"net/http"
	"slices"
	"strconv"
	"testing"

	"rainy/internal/model"
	"rainy/internal/store"
)

// Only library managers mark program folders. A marked folder's tracks and its album carry
// `program`, stay reachable by id, album and folder, and leave the default lists, random
// picks and the home page.
func TestProgramFolders(t *testing.T) {
	e := newNativeEnv(t)
	ctx := context.Background()
	manager := &model.User{Username: "manager", CanManage: true}
	if err := e.app.Auth.CreateUser(ctx, manager, "secret-manager"); err != nil {
		t.Fatal(err)
	}
	managerTok, err := e.app.Auth.CreateSession(ctx, manager.ID, "test", "127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	mark := map[string]any{"libraryId": e.lib.ID, "path": "Sine Wave/Tones"}

	nativeExpect(t, e.do(http.MethodPut, "/manage/program-folders", e.aliceTok, mark), http.StatusForbidden)
	nativeExpect(t, e.do(http.MethodGet, "/manage/program-folders", e.aliceTok, nil), http.StatusForbidden)
	nativeExpect(t, e.do(http.MethodPut, "/manage/program-folders", managerTok, map[string]any{"libraryId": e.lib.ID, "path": "../x"}), http.StatusBadRequest)
	nativeExpect(t, e.do(http.MethodPut, "/manage/program-folders", managerTok, map[string]any{"libraryId": 9999, "path": "x"}), http.StatusNotFound)
	folders := nativeJSON[store.Programs](t, e.do(http.MethodPut, "/manage/program-folders", managerTok, mark), http.StatusOK).Folders
	if len(folders) != 1 || folders[0].Path != "Sine Wave/Tones" || folders[0].LibraryID != e.lib.ID {
		t.Fatalf("folders = %+v", folders)
	}

	if tr := nativeJSON[model.Track](t, e.do(http.MethodGet, "/tracks/"+e.id("tone"), e.bobTok, nil), http.StatusOK); !tr.Program {
		t.Error("track program = false")
	}
	if al := nativeJSON[albumDetail](t, e.do(http.MethodGet, "/albums/"+e.tones, e.bobTok, nil), http.StatusOK); !al.Program ||
		len(al.Tracks) != 1 || !al.Tracks[0].Program {
		t.Errorf("album = %+v", al)
	}
	if one := nativeJSON[model.Track](t, e.do(http.MethodGet, "/tracks/"+e.id("one"), e.bobTok, nil), http.StatusOK); one.Program {
		t.Error("a music track is a program")
	}

	tracks := func(query string) []string {
		return nativeTrackIDs(nativeJSON[Page[model.Track]](t, e.do(http.MethodGet, "/tracks"+query, e.bobTok, nil), http.StatusOK).Items)
	}
	tone := e.id("tone")
	for query, want := range map[string]bool{
		"": false, "?sort=random": false, "?programs=include": true, "?programs=include&sort=random": false,
		"?programs=only": true, "?ids=" + tone: true, "?albumId=" + e.tones: true, "?dirPrefix=Sine%20Wave": true,
	} {
		if got := slices.Contains(tracks(query), tone); got != want {
			t.Errorf("/tracks%s lists the episode: %v", query, got)
		}
	}
	if ids := tracks("?programs=only"); len(ids) != 1 {
		t.Errorf("programs=only = %v", ids)
	}
	albums := func(query string) []string {
		return nativeAlbumIDs(nativeJSON[Page[model.Album]](t, e.do(http.MethodGet, "/albums"+query, e.bobTok, nil), http.StatusOK).Items)
	}
	if slices.Contains(albums(""), e.tones) || slices.Contains(albums("?sort=random"), e.tones) || !slices.Equal(albums("?programs=only"), []string{e.tones}) {
		t.Errorf("albums: %v / %v", albums(""), albums("?programs=only"))
	}
	for range 5 {
		if random := nativeJSON[[]model.Track](t, e.do(http.MethodGet, "/random?size=50", e.bobTok, nil), http.StatusOK); slices.Contains(nativeTrackIDs(random), tone) {
			t.Fatal("random lists the episode")
		}
	}
	h := nativeJSON[home](t, e.do(http.MethodGet, "/home", e.bobTok, nil), http.StatusOK)
	if slices.Contains(nativeAlbumIDs(h.RecentlyAdded), e.tones) || slices.Contains(nativeAlbumIDs(h.Random), e.tones) ||
		h.Stats.Albums != 2 || h.Stats.Tracks != 5 || h.Stats.Artists != 2 {
		t.Errorf("home = %+v", h)
	}
	artist := nativeJSON[artistDetail](t, e.do(http.MethodGet, "/artists/"+e.sineWave, e.bobTok, nil), http.StatusOK)
	if len(artist.Albums) != 0 || len(artist.TopTracks) != 0 {
		t.Errorf("artist page = %+v / %+v", artist.Albums, artist.TopTracks)
	}

	shows := nativeJSON[[]store.ProgramShow](t, e.do(http.MethodGet, "/programs", e.bobTok, nil), http.StatusOK)
	if len(shows) != 1 || shows[0].ID != e.tones || shows[0].Episodes != 1 || !shows[0].Program {
		t.Errorf("programs = %+v", shows)
	}

	nativeExpect(t, e.do(http.MethodDelete, "/manage/program-folders", e.aliceTok, mark), http.StatusForbidden)
	if folders := nativeJSON[store.Programs](t, e.do(http.MethodDelete, "/manage/program-folders", managerTok, mark), http.StatusOK).Folders; len(folders) != 0 {
		t.Errorf("after unmarking = %+v", folders)
	}
	nativeExpect(t, e.do(http.MethodDelete, "/manage/program-folders", managerTok, mark), http.StatusNotFound)
	if !slices.Contains(tracks(""), tone) {
		t.Error("an unmarked folder's track stays hidden")
	}
}

// Bookmarks are per user and only reach tracks the user may read.
func TestBookmarksScope(t *testing.T) {
	e := newAccessEnv(t)
	one := e.id("one")
	pos := func(ms int64) map[string]any { return map[string]any{"positionMs": ms} }

	b := nativeJSON[model.Bookmark](t, e.do(http.MethodPut, "/bookmarks/"+one, e.aliceTok, pos(61_000)), http.StatusOK)
	if b.TrackID != one || b.PositionMs != 61_000 {
		t.Errorf("saved = %+v", b)
	}
	nativeExpect(t, e.do(http.MethodPut, "/bookmarks/"+one, e.aliceTok, pos(-1)), http.StatusBadRequest)
	nativeExpect(t, e.do(http.MethodPut, "/bookmarks/"+one, e.aliceTok, map[string]any{}), http.StatusBadRequest)

	list := func(tok, ids string) []model.Bookmark {
		return nativeJSON[[]model.Bookmark](t, e.do(http.MethodGet, "/bookmarks?trackIds="+ids, tok, nil), http.StatusOK)
	}
	if got := list(e.aliceTok, one+",trk-secret"); len(got) != 1 || got[0].PositionMs != 61_000 {
		t.Errorf("alice = %+v", got)
	}
	if got := list(e.bobTok, one); len(got) != 0 {
		t.Errorf("bob sees alice's bookmark: %+v", got)
	}

	// Bob reads every library and bookmarks a track of the other one; Alice, limited to the
	// first library, can neither read nor write it.
	nativeJSON[model.Bookmark](t, e.do(http.MethodPut, "/bookmarks/trk-secret", e.bobTok, pos(5_000)), http.StatusOK)
	nativeExpect(t, e.do(http.MethodPut, "/bookmarks/trk-secret", e.aliceTok, pos(5_000)), http.StatusNotFound)
	nativeExpect(t, e.do(http.MethodDelete, "/bookmarks/trk-secret", e.aliceTok, nil), http.StatusNotFound)
	if got := list(e.aliceTok, "trk-secret"); len(got) != 0 {
		t.Errorf("alice reads a hidden bookmark: %+v", got)
	}
	if got := list(e.bobTok, "trk-secret"); len(got) != 1 {
		t.Errorf("bob = %+v", got)
	}

	nativeExpect(t, e.do(http.MethodDelete, "/bookmarks/"+one, e.aliceTok, nil), http.StatusNoContent)
	nativeExpect(t, e.do(http.MethodDelete, "/bookmarks/"+one, e.aliceTok, nil), http.StatusNoContent)
	if got := list(e.aliceTok, one); len(got) != 0 {
		t.Errorf("after delete = %+v", got)
	}
}

// Only administrators make a library a program library; its tracks then leave the music
// lists until it is a music library again.
func TestAdminProgramLibrary(t *testing.T) {
	e := newNativeEnv(t)
	ctx := context.Background()
	manager := &model.User{Username: "manager", CanManage: true}
	if err := e.app.Auth.CreateUser(ctx, manager, "secret-manager"); err != nil {
		t.Fatal(err)
	}
	managerTok, err := e.app.Auth.CreateSession(ctx, manager.ID, "test", "127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	path := "/admin/libraries/" + strconv.FormatInt(e.lib.ID, 10)
	nativeExpect(t, e.do(http.MethodPut, path, managerTok, map[string]any{"program": true}), http.StatusForbidden)
	lib := nativeJSON[LibraryInfo](t, e.do(http.MethodPut, path, e.adminTok, map[string]any{"program": true}), http.StatusOK)
	if !lib.Program || lib.Name != e.lib.Name {
		t.Fatalf("updated = %+v", lib)
	}
	programs := nativeJSON[store.Programs](t, e.do(http.MethodGet, "/manage/program-folders", managerTok, nil), http.StatusOK)
	if !slices.Equal(programs.Libraries, []int64{e.lib.ID}) {
		t.Errorf("programs = %+v", programs)
	}
	if tr := nativeJSON[model.Track](t, e.do(http.MethodGet, "/tracks/"+e.id("one"), e.bobTok, nil), http.StatusOK); !tr.Program {
		t.Error("track program = false")
	}
	if got := nativeJSON[Page[model.Track]](t, e.do(http.MethodGet, "/tracks", e.bobTok, nil), http.StatusOK); got.Total != 0 {
		t.Errorf("default song list = %d", got.Total)
	}
	if random := nativeJSON[[]model.Track](t, e.do(http.MethodGet, "/random", e.bobTok, nil), http.StatusOK); len(random) != 0 {
		t.Errorf("random = %d", len(random))
	}
	if h := nativeJSON[home](t, e.do(http.MethodGet, "/home", e.bobTok, nil), http.StatusOK); h.Stats.Tracks != 0 || h.Stats.Albums != 0 {
		t.Errorf("home stats = %+v", h.Stats)
	}
	if shows := nativeJSON[[]store.ProgramShow](t, e.do(http.MethodGet, "/programs", e.bobTok, nil), http.StatusOK); len(shows) != 3 {
		t.Errorf("programs = %d", len(shows))
	}

	// Leaving out "program" keeps it; false makes the library music again.
	lib = nativeJSON[LibraryInfo](t, e.do(http.MethodPut, path, e.adminTok, map[string]any{"name": "Renamed"}), http.StatusOK)
	if !lib.Program {
		t.Error("renaming dropped the program flag")
	}
	nativeJSON[LibraryInfo](t, e.do(http.MethodPut, path, e.adminTok, map[string]any{"program": false}), http.StatusOK)
	if got := nativeJSON[Page[model.Track]](t, e.do(http.MethodGet, "/tracks", e.bobTok, nil), http.StatusOK); got.Total != 6 {
		t.Errorf("song list after unmarking = %d", got.Total)
	}

	created := nativeJSON[LibraryInfo](t, e.do(http.MethodPost, "/admin/libraries", e.adminTok,
		map[string]any{"path": t.TempDir(), "program": true}), http.StatusCreated)
	if !created.Program {
		t.Errorf("created = %+v", created)
	}
}
