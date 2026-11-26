package store_test

import (
	"errors"
	"slices"
	"testing"

	"rainy/internal/model"
	"rainy/internal/store"
)

// Library access is stored per user and normalised: managers are never restricted, and
// deleting a library drops it from every grant.
func TestUserLibraryAccess(t *testing.T) {
	s := newStore(t)
	a, b := newLibrary(t, s, "/path/to/a"), newLibrary(t, s, "/path/to/b")

	u := &model.User{Username: "listener", PasswordEnc: "synthetic", RestrictLibraries: true, LibraryIDs: []int64{b.ID, a.ID, b.ID}}
	if err := s.CreateUser(ctx, u); err != nil {
		t.Fatal(err)
	}
	got, err := s.GetUser(ctx, u.ID)
	if err != nil || !got.RestrictLibraries || !slices.Equal(got.LibraryIDs, []int64{a.ID, b.ID}) {
		t.Fatalf("created = %+v, %v", got, err)
	}
	open := &model.User{Username: "open", PasswordEnc: "synthetic"}
	if err := s.CreateUser(ctx, open); err != nil {
		t.Fatal(err)
	}
	users, err := s.ListUsers(ctx)
	if err != nil || len(users) != 2 || !slices.Equal(users[0].LibraryIDs, []int64{a.ID, b.ID}) ||
		users[1].RestrictLibraries || users[1].LibraryIDs == nil || len(users[1].LibraryIDs) != 0 {
		t.Fatalf("list = %+v, %v", users, err)
	}

	got.LibraryIDs = []int64{a.ID, 9999}
	if err := s.UpdateUser(ctx, got); !errors.Is(err, store.ErrInvalid) {
		t.Errorf("unknown library: %v", err)
	}
	got.LibraryIDs = []int64{b.ID}
	got.CanManage = true
	if err := s.UpdateUser(ctx, got); err != nil {
		t.Fatal(err)
	}
	if got, _ = s.GetUser(ctx, u.ID); got.RestrictLibraries || len(got.LibraryIDs) != 0 {
		t.Errorf("manager = %+v", got)
	}

	got.CanManage, got.RestrictLibraries, got.LibraryIDs = false, true, []int64{a.ID, b.ID}
	if err := s.UpdateUser(ctx, got); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteLibrary(ctx, a.ID); err != nil {
		t.Fatal(err)
	}
	if got, _ = s.GetUser(ctx, u.ID); !got.RestrictLibraries || !slices.Equal(got.LibraryIDs, []int64{b.ID}) {
		t.Errorf("after deleting a library = %+v", got)
	}
}

// A scoped context reads only its libraries; an unscoped one reads all of them.
func TestLibraryScopeReads(t *testing.T) {
	s := newStore(t)
	a, b := newLibrary(t, s, "/path/to/a"), newLibrary(t, s, "/path/to/b")
	ta := upsert(t, s, a.ID, trk{path: "x/1.mp3", title: "Here", artist: "Both", album: "Near", genre: "Folk"})[0]
	tb := upsert(t, s, b.ID, trk{path: "y/1.mp3", title: "There", artist: "Both", album: "Far", genre: "Noise"})[0]
	if err := s.RefreshAll(ctx); err != nil {
		t.Fatal(err)
	}
	scoped := store.WithLibraryScope(ctx, []int64{a.ID})
	if _, restricted := store.LibraryScope(ctx); restricted || !store.CanReadLibrary(scoped, a.ID) || store.CanReadLibrary(scoped, b.ID) {
		t.Fatal("scope reporting")
	}

	if libs, err := s.ListLibraries(scoped); err != nil || len(libs) != 1 || libs[0].ID != a.ID {
		t.Errorf("libraries = %+v, %v", libs, err)
	}
	if _, err := s.GetLibrary(scoped, b.ID); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("other library: %v", err)
	}
	if _, err := s.GetTrack(scoped, tb.ID, ""); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("other track: %v", err)
	}
	if ts, err := s.GetTracks(scoped, []string{tb.ID, ta.ID}, ""); err != nil || len(ts) != 1 || ts[0].ID != ta.ID {
		t.Errorf("tracks = %+v, %v", ts, err)
	}
	if _, err := s.GetAlbum(scoped, tb.AlbumID, ""); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("other album: %v", err)
	}
	ar, err := s.GetArtist(scoped, ta.ArtistID, "")
	if err != nil || ar.AlbumCount != 1 || ar.SongCount != 1 {
		t.Errorf("artist in scope = %+v, %v", ar, err)
	}
	if ar, err = s.GetArtist(ctx, ta.ArtistID, ""); err != nil || ar.AlbumCount != 2 || ar.SongCount != 2 {
		t.Errorf("artist unscoped = %+v, %v", ar, err)
	}
	if gs, err := s.ListGenres(scoped); err != nil || len(gs) != 1 || gs[0].Name != "Folk" {
		t.Errorf("genres = %+v, %v", gs, err)
	}

	none := store.WithLibraryScope(ctx, nil)
	if _, n, err := s.ListTracks(none, store.TrackQuery{}); err != nil || n != 0 {
		t.Errorf("no library: %d tracks, %v", n, err)
	}

	// A play of a track outside the scope cannot be recorded; an earlier one keeps its
	// snapshot and carries no track.
	u := &model.User{Username: "listener", PasswordEnc: "synthetic"}
	if err := s.CreateUser(ctx, u); err != nil {
		t.Fatal(err)
	}
	if err := s.RecordPlay(scoped, u.ID, tb.ID, 1000, "web"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("recording a play outside the scope: %v", err)
	}
	if err := s.RecordPlay(ctx, u.ID, tb.ID, 1000, "web"); err != nil {
		t.Fatal(err)
	}
	plays, _, err := s.ListPlays(scoped, u.ID, 0, 0, 0, 10)
	if err != nil || len(plays) != 1 || plays[0].Track != nil || plays[0].Title != "There" {
		t.Errorf("plays = %+v, %v", plays, err)
	}
}
