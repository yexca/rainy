package api

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"rainy/internal/events"
	"rainy/internal/model"
	"rainy/internal/nowplaying"
	"rainy/internal/store"
	"rainy/internal/util"
)

// accessEnv is the native fixture plus a second library holding an album of its own and a
// track by an artist of the first library. Alice is limited to the first library.
type accessEnv struct {
	*nativeEnv
	other                                   *model.Library
	secretAlbum, hiddenBand, elsewhereAlbum string
}

func newAccessEnv(t *testing.T) *accessEnv {
	t.Helper()
	e := &accessEnv{nativeEnv: newNativeEnv(t)}
	ctx := context.Background()
	dir := filepath.Join(t.TempDir(), "other")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	e.other = &model.Library{Name: "Other", Path: dir}
	if err := e.app.Store.CreateLibrary(ctx, e.other); err != nil {
		t.Fatal(err)
	}
	e.hiddenBand = util.ArtistID("Hidden Band")
	e.secretAlbum = util.AlbumID("Hidden Band", "Secret Album")
	e.elsewhereAlbum = util.AlbumID("The Rainy Days", "Elsewhere")
	mk := func(id, path, title, artist, album, genre string) model.Track {
		return model.Track{
			ID: id, LibraryID: e.other.ID, Path: path, Size: 1, Mtime: 1, Title: title, Album: album,
			Artist: artist, AlbumArtist: artist, AlbumID: util.AlbumID(artist, album),
			ArtistID: util.ArtistID(artist), AlbumArtistID: util.ArtistID(artist), Genre: genre,
			Duration: 60, CreatedAt: 1_800_000_000_000,
		}
	}
	if err := e.app.Store.UpsertTracks(ctx, []model.Track{
		mk("trk-secret", "Hidden Band/Secret Album/01 Secret.mp3", "Secret Song", "Hidden Band", "Secret Album", "Secretcore"),
		mk("trk-elsewhere", "The Rainy Days/Elsewhere/01 Away.mp3", "Distant Lane", "The Rainy Days", "Elsewhere", "Pop"),
	}); err != nil {
		t.Fatal(err)
	}
	if err := e.app.Store.RefreshAll(ctx); err != nil {
		t.Fatal(err)
	}
	rec := e.do(http.MethodPut, "/admin/users/"+e.alice.ID, e.adminTok,
		map[string]any{"restrictLibraries": true, "libraryIds": []int64{e.lib.ID}})
	u := nativeJSON[model.User](t, rec, http.StatusOK)
	if !u.RestrictLibraries || !slices.Equal(u.LibraryIDs, []int64{e.lib.ID}) {
		t.Fatalf("alice access = %v %v", u.RestrictLibraries, u.LibraryIDs)
	}
	return e
}

// A user limited to some libraries never sees, plays or annotates what lies elsewhere, and
// aggregates only count what they can see.
func TestLibraryAccessHidesOtherLibraries(t *testing.T) {
	e := newAccessEnv(t)

	for _, path := range []string{
		"/tracks/trk-secret", "/stream/trk-secret", "/download/trk-secret", "/lyrics/trk-secret",
		"/albums/" + e.secretAlbum, "/artists/" + e.hiddenBand, "/albums/" + e.elsewhereAlbum,
		"/download/album/" + e.secretAlbum,
	} {
		nativeExpect(t, e.do(http.MethodGet, path, e.aliceTok, nil), http.StatusNotFound)
	}
	// Bob reads every library.
	nativeExpect(t, e.do(http.MethodGet, "/tracks/trk-secret", e.bobTok, nil), http.StatusOK)

	tracks := nativeJSON[struct {
		Items []model.Track `json:"items"`
		Total int           `json:"total"`
	}](t, e.do(http.MethodGet, "/tracks?limit=100", e.aliceTok, nil), http.StatusOK)
	if ids := nativeTrackIDs(tracks.Items); slices.Contains(ids, "trk-secret") || slices.Contains(ids, "trk-elsewhere") || tracks.Total != len(ids) {
		t.Errorf("tracks = %v (total %d)", ids, tracks.Total)
	}
	albums := nativeJSON[struct {
		Items []model.Album `json:"items"`
	}](t, e.do(http.MethodGet, "/albums?limit=100", e.aliceTok, nil), http.StatusOK)
	if ids := nativeAlbumIDs(albums.Items); slices.Contains(ids, e.secretAlbum) || slices.Contains(ids, e.elsewhereAlbum) {
		t.Errorf("albums = %v", ids)
	}
	artists := nativeJSON[struct {
		Items []model.Artist `json:"items"`
	}](t, e.do(http.MethodGet, "/artists?all=true&limit=100", e.aliceTok, nil), http.StatusOK)
	for _, ar := range artists.Items {
		if ar.ID == e.hiddenBand {
			t.Errorf("artists include the hidden band")
		}
	}

	// The Rainy Days have Blue Skies here and Elsewhere in the other library: the counts and
	// albums cover the visible one only.
	rd := nativeJSON[struct {
		model.Artist
		Albums []model.Album `json:"albums"`
	}](t, e.do(http.MethodGet, "/artists/"+e.rainyDays, e.aliceTok, nil), http.StatusOK)
	if rd.AlbumCount != 1 || rd.SongCount != 4 || !slices.Equal(nativeAlbumIDs(rd.Albums), []string{e.blueSkies}) {
		t.Errorf("rainy days for alice: albums %d songs %d %v", rd.AlbumCount, rd.SongCount, nativeAlbumIDs(rd.Albums))
	}
	all := nativeJSON[model.Artist](t, e.do(http.MethodGet, "/artists/"+e.rainyDays, e.bobTok, nil), http.StatusOK)
	if all.AlbumCount != 2 || all.SongCount != 5 {
		t.Errorf("rainy days for bob: albums %d songs %d", all.AlbumCount, all.SongCount)
	}

	res := nativeJSON[store.SearchResult](t, e.do(http.MethodGet, "/search?q=secret", e.aliceTok, nil), http.StatusOK)
	if len(res.Artists)+len(res.Albums)+len(res.Tracks) != 0 {
		t.Errorf("search found %+v", res)
	}
	genres := nativeJSON[[]model.Genre](t, e.do(http.MethodGet, "/genres", e.aliceTok, nil), http.StatusOK)
	for _, g := range genres {
		if g.Name == "Secretcore" {
			t.Errorf("genres include one of the other library only")
		}
	}

	nativeExpect(t, e.do(http.MethodPost, "/star", e.aliceTok,
		map[string]any{"type": "track", "ids": []string{"trk-secret"}, "starred": true}), http.StatusNotFound)
	nativeExpect(t, e.do(http.MethodPost, "/rating", e.aliceTok,
		map[string]any{"type": "album", "id": e.secretAlbum, "rating": 5}), http.StatusNotFound)
	nativeExpect(t, e.do(http.MethodPost, "/scrobble", e.aliceTok,
		map[string]any{"trackId": "trk-secret", "submission": true}), http.StatusNotFound)

	// Covers follow the album: Blue Skies has one, which a user of the other library cannot load.
	cover := "/cover/" + util.CoverArtID(util.CoverKindAlbum, e.blueSkies, 0)
	nativeExpect(t, e.do(http.MethodGet, cover, e.bobTok, nil), http.StatusOK)
	nativeExpect(t, e.do(http.MethodPut, "/admin/users/"+e.bob.ID, e.adminTok,
		map[string]any{"restrictLibraries": true, "libraryIds": []int64{e.other.ID}}), http.StatusOK)
	nativeExpect(t, e.do(http.MethodGet, cover, e.bobTok, nil), http.StatusNotFound)
	nativeExpect(t, e.do(http.MethodGet, "/tracks/"+e.id("one"), e.bobTok, nil), http.StatusNotFound)
}

// Playlist entries of other libraries are invisible to a limited user and survive their
// edits, like entries of missing files.
func TestLibraryAccessPlaylistEntries(t *testing.T) {
	e := newAccessEnv(t)
	created := nativeJSON[model.Playlist](t, e.do(http.MethodPost, "/playlists", e.aliceTok,
		map[string]any{"name": "Mix", "trackIds": []string{e.id("one"), "trk-secret"}}), http.StatusCreated)
	if created.SongCount != 1 {
		t.Fatalf("alice could add a track of another library: %+v", created)
	}
	path := "/playlists/" + created.ID
	nativeExpect(t, e.do(http.MethodPost, path+"/tracks", e.adminTok,
		map[string]any{"trackIds": []string{"trk-secret", e.id("tone")}}), http.StatusOK)

	type detail struct {
		model.Playlist
		Tracks []model.Track `json:"tracks"`
	}
	got := nativeJSON[detail](t, e.do(http.MethodGet, path, e.aliceTok, nil), http.StatusOK)
	if ids := nativeTrackIDs(got.Tracks); !slices.Equal(ids, []string{e.id("one"), e.id("tone")}) || got.SongCount != 2 {
		t.Fatalf("alice sees %v (songCount %d)", ids, got.SongCount)
	}
	nativeExpect(t, e.do(http.MethodPut, path+"/tracks", e.aliceTok,
		map[string]any{"trackIds": []string{e.id("tone"), e.id("one")}}), http.StatusOK)
	nativeExpect(t, e.do(http.MethodDelete, path+"/tracks", e.aliceTok,
		map[string]any{"positions": []int{0}}), http.StatusOK)

	admin := nativeJSON[detail](t, e.do(http.MethodGet, path, e.adminTok, nil), http.StatusOK)
	if ids := nativeTrackIDs(admin.Tracks); !slices.Equal(ids, []string{e.id("one"), "trk-secret"}) {
		t.Errorf("admin sees %v after alice's edits", ids)
	}
}

// Administrators set library access; managers and administrators always read every library.
func TestAdminLibraryAccess(t *testing.T) {
	e := newAccessEnv(t)
	path := "/admin/users/" + e.alice.ID

	rec := e.do(http.MethodPut, path, e.adminTok, map[string]any{"libraryIds": []int64{e.lib.ID, 9999}})
	if code := nativeErrorCode(t, rec, http.StatusBadRequest); code != "bad_request" {
		t.Errorf("unknown library: %s", code)
	}

	// The current user reports the access too.
	me := nativeJSON[model.User](t, e.do(http.MethodGet, "/me", e.aliceTok, nil), http.StatusOK)
	if !me.RestrictLibraries || !slices.Equal(me.LibraryIDs, []int64{e.lib.ID}) {
		t.Errorf("me = %v %v", me.RestrictLibraries, me.LibraryIDs)
	}

	u := nativeJSON[model.User](t, e.do(http.MethodPut, path, e.adminTok, map[string]any{"canManage": true}), http.StatusOK)
	if u.RestrictLibraries || len(u.LibraryIDs) != 0 {
		t.Errorf("a manager keeps a restriction: %v %v", u.RestrictLibraries, u.LibraryIDs)
	}
	nativeExpect(t, e.do(http.MethodGet, "/tracks/trk-secret", e.aliceTok, nil), http.StatusOK)

	created := nativeJSON[model.User](t, e.do(http.MethodPost, "/admin/users", e.adminTok, map[string]any{
		"username": "carol", "password": "secret-carol", "restrictLibraries": true, "libraryIds": []int64{e.other.ID},
	}), http.StatusCreated)
	if !created.RestrictLibraries || !slices.Equal(created.LibraryIDs, []int64{e.other.ID}) {
		t.Errorf("carol = %v %v", created.RestrictLibraries, created.LibraryIDs)
	}

	// Deleting a library removes it from every grant.
	rec = e.do(http.MethodDelete, fmt.Sprintf("/admin/libraries/%d", e.other.ID), e.adminTok, nil)
	if rec.Code != http.StatusNoContent && rec.Code != http.StatusAccepted && rec.Code != http.StatusOK {
		t.Fatalf("delete library: %d %s", rec.Code, rec.Body.String())
	}
	carol, err := e.app.Store.GetUser(context.Background(), created.ID)
	if err != nil || !carol.RestrictLibraries || len(carol.LibraryIDs) != 0 {
		t.Errorf("carol after delete = %+v, %v", carol, err)
	}
}

// The now-playing list a limited user receives over /events leaves out songs of other
// libraries.
func TestLibraryAccessNowPlayingEvents(t *testing.T) {
	e := newAccessEnv(t)
	entries := []nowplaying.Entry{{UserID: e.bob.ID, TrackID: "trk-secret"}, {UserID: e.admin.ID, TrackID: e.id("one")}}
	ev := events.Event{Type: events.TypeNowPlaying, Data: entries}
	a := New(e.app)

	scoped := store.WithLibraryScope(context.Background(), []int64{e.lib.ID})
	got, _ := a.visibleEvent(scoped, ev).Data.([]nowplaying.Entry)
	if len(got) != 1 || got[0].TrackID != e.id("one") {
		t.Errorf("limited user sees %+v", got)
	}
	if all, _ := a.visibleEvent(context.Background(), ev).Data.([]nowplaying.Entry); len(all) != 2 {
		t.Errorf("unrestricted user sees %+v", all)
	}
}
