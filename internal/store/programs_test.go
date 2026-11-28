package store_test

import (
	"errors"
	"slices"
	"testing"

	"rainy/internal/model"
	"rainy/internal/store"
)

// programLibrary indexes a show folder with a subfolder, two sibling folders whose names
// extend the show's, and a music album.
func programLibrary(t *testing.T, s *store.Store) (*model.Library, map[string]model.Track) {
	t.Helper()
	lib := newLibrary(t, s, "/path/to/music")
	ts := upsert(t, s, lib.ID,
		trk{path: "Shows/Example Show/Episode 01.mp3", title: "Episode 01", artist: "Host", album: "Example Show", track: 1, duration: 1680, created: 100},
		trk{path: "Shows/Example Show/2026/Episode 02.mp3", title: "Episode 02", artist: "Host", album: "Example Show", track: 2, duration: 1680, created: 200},
		trk{path: "Shows/Example Show B/Episode 01.mp3", title: "Sibling", artist: "Other Host", album: "Other Show", duration: 1680, created: 300},
		trk{path: "Shows/Example Showcase/Song.mp3", title: "Showcase", artist: "Band", album: "Showcase", duration: 200, created: 400},
		trk{path: "Music/Band/Song.mp3", title: "Song", artist: "Band", album: "Record", duration: 200, created: 500},
	)
	byTitle := map[string]model.Track{}
	for _, tr := range ts {
		byTitle[tr.Title] = tr
	}
	return lib, byTitle
}

// A program folder covers its own tracks and every folder below it, never siblings whose
// names start the same way; marking is idempotent, validated, and removable.
func TestProgramFolderCondition(t *testing.T) {
	s := newStore(t)
	lib, tr := programLibrary(t, s)

	if err := s.AddProgramFolder(ctx, lib.ID, "../x"); !errors.Is(err, store.ErrInvalid) {
		t.Errorf("invalid path: %v", err)
	}
	if err := s.AddProgramFolder(ctx, 9999, "Shows"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("unknown library: %v", err)
	}
	for range 2 {
		if err := s.AddProgramFolder(ctx, lib.ID, "/Shows/Example Show/"); err != nil {
			t.Fatal(err)
		}
	}
	p, err := s.ListPrograms(ctx)
	folders := p.Folders
	if err != nil || len(folders) != 1 || folders[0].Path != "Shows/Example Show" || folders[0].LibraryID != lib.ID {
		t.Fatalf("folders = %+v, %v", folders, err)
	}

	all, _ := listTracks(t, s, store.TrackQuery{Sort: "title"})
	var programs []string
	for _, x := range all {
		if x.Program {
			programs = append(programs, x.Title)
		}
	}
	if !slices.Equal(programs, []string{"Episode 01", "Episode 02"}) {
		t.Errorf("program tracks = %v", programs)
	}
	if got, err := s.GetTrack(ctx, tr["Episode 02"].ID, ""); err != nil || !got.Program {
		t.Errorf("GetTrack = %+v, %v", got, err)
	}
	if got, _ := listTracks(t, s, store.TrackQuery{Programs: store.ProgramsExclude, Sort: "title"}); !slices.Equal(trackTitles(got), []string{"Showcase", "Sibling", "Song"}) {
		t.Errorf("excluded = %v", trackTitles(got))
	}
	if got, _ := listTracks(t, s, store.TrackQuery{Programs: store.ProgramsOnly, Sort: "title"}); !slices.Equal(trackTitles(got), []string{"Episode 01", "Episode 02"}) {
		t.Errorf("only = %v", trackTitles(got))
	}

	show, err := s.GetAlbum(ctx, tr["Episode 01"].AlbumID, "")
	if err != nil || !show.Program {
		t.Fatalf("show album = %+v, %v", show, err)
	}
	albums, _, err := s.ListAlbums(ctx, store.AlbumQuery{Programs: store.ProgramsExclude, Sort: "random"})
	if err != nil || len(albums) != 3 || slices.ContainsFunc(albums, func(a model.Album) bool { return a.ID == show.ID || a.Program }) {
		t.Errorf("albums without programs = %+v, %v", albums, err)
	}
	artists, _, err := s.ListArtists(ctx, store.ArtistQuery{AlbumArtistsOnly: true, ExcludePrograms: true})
	if err != nil || slices.ContainsFunc(artists, func(a model.Artist) bool { return a.Name == "Host" }) || len(artists) != 2 {
		t.Errorf("artists without programs = %+v, %v", artists, err)
	}
	cands, err := s.RecommendCandidates(ctx, "", store.CandidateQuery{Limit: 100})
	if err != nil || len(cands) != 3 || slices.ContainsFunc(cands, func(c store.Candidate) bool {
		return c.ID == tr["Episode 01"].ID || c.ID == tr["Episode 02"].ID
	}) {
		t.Errorf("candidates = %+v, %v", cands, err)
	}

	// An album keeps being music while one of its present tracks is outside program folders.
	if err := s.RemoveProgramFolder(ctx, lib.ID, "Shows/Example Show"); err != nil {
		t.Fatal(err)
	}
	if err := s.AddProgramFolder(ctx, lib.ID, "Shows/Example Show/2026"); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.GetAlbum(ctx, show.ID, ""); got.Program {
		t.Error("an album with a track outside program folders is a program")
	}
	if got, _ := s.GetTrack(ctx, tr["Episode 01"].ID, ""); got.Program {
		t.Error("the parent of a program folder is a program")
	}
	if err := s.RemoveProgramFolder(ctx, lib.ID, "Shows/Example Show"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("removing an unmarked folder: %v", err)
	}

	// Removing the library removes its program folders.
	if err := s.DeleteLibrary(ctx, lib.ID); err != nil {
		t.Fatal(err)
	}
	if p, err := s.ListPrograms(ctx); err != nil || len(p.Folders) != 0 || len(p.Libraries) != 0 {
		t.Errorf("after deleting the library = %+v, %v", p, err)
	}
}

// Program shows report the user's own progress and follow the library scope.
func TestProgramShowsAndBookmarks(t *testing.T) {
	s := newStore(t)
	lib, tr := programLibrary(t, s)
	other := newLibrary(t, s, "/path/to/other")
	if err := s.AddProgramFolder(ctx, lib.ID, "Shows"); err != nil {
		t.Fatal(err)
	}
	listener, other2 := createUser(t, s, "listener"), createUser(t, s, "other")
	ep1, ep2 := tr["Episode 01"].ID, tr["Episode 02"].ID
	if err := s.RecordPlay(ctx, listener, ep1, 1000, "web"); err != nil {
		t.Fatal(err)
	}
	if err := s.UpsertBookmark(ctx, listener, &model.Bookmark{TrackID: ep2, PositionMs: 60000, Comment: "synthetic note"}); err != nil {
		t.Fatal(err)
	}

	shows, err := s.ListProgramShows(ctx, listener)
	if err != nil || len(shows) != 3 {
		t.Fatalf("shows = %+v, %v", shows, err)
	}
	byName := map[string]store.ProgramShow{}
	for _, sh := range shows {
		byName[sh.Name] = sh
	}
	ex := byName["Example Show"]
	if !ex.Program || ex.Episodes != 2 || ex.Played != 1 || ex.InProgress != 1 || ex.LatestEpisode != 200 {
		t.Errorf("Example Show = %+v", ex)
	}
	if shows[0].Name != "Example Show" {
		t.Errorf("most recent activity first: %v", shows[0].Name)
	}
	if theirs, _ := s.ListProgramShows(ctx, other2); theirs[0].Played != 0 || theirs[0].InProgress != 0 {
		t.Errorf("another user's progress = %+v", theirs[0])
	}

	// Moving the position keeps a comment set through Subsonic.
	b, err := s.SetBookmarkPosition(ctx, listener, ep2, 90000)
	if err != nil || b.PositionMs != 90000 || b.Comment != "synthetic note" {
		t.Errorf("moved bookmark = %+v, %v", b, err)
	}
	if bms, err := s.TrackBookmarks(ctx, listener, []string{ep1, ep2}); err != nil || len(bms) != 1 || bms[0].TrackID != ep2 {
		t.Errorf("bookmarks = %+v, %v", bms, err)
	}

	scoped := store.WithLibraryScope(ctx, []int64{other.ID})
	if bms, err := s.TrackBookmarks(scoped, listener, []string{ep2}); err != nil || len(bms) != 0 {
		t.Errorf("bookmark outside the scope = %+v, %v", bms, err)
	}
	if shows, err := s.ListProgramShows(scoped, listener); err != nil || len(shows) != 0 {
		t.Errorf("shows outside the scope = %+v, %v", shows, err)
	}
}

func createUser(t *testing.T, s *store.Store, name string) string {
	t.Helper()
	u := &model.User{Username: name, PasswordEnc: "synthetic"}
	if err := s.CreateUser(ctx, u); err != nil {
		t.Fatal(err)
	}
	return u.ID
}

// A program library makes every track of it a program episode, like a program folder at its
// root; turning it back into a music library restores the music behavior.
func TestProgramLibrary(t *testing.T) {
	s := newStore(t)
	music := newLibrary(t, s, "/path/to/music")
	shows := newLibrary(t, s, "/path/to/shows")
	upsert(t, s, music.ID, trk{path: "Band/Record/01.mp3", title: "Song", artist: "Band", album: "Record", duration: 200})
	ep := upsert(t, s, shows.ID, trk{path: "Example Show/Episode 01.mp3", title: "Episode 01", artist: "Host", album: "Example Show", duration: 1680})[0]

	shows.Program = true
	if err := s.UpdateLibrary(ctx, shows); err != nil {
		t.Fatal(err)
	}
	// A folder marked inside a program library changes nothing.
	if err := s.AddProgramFolder(ctx, shows.ID, "Example Show"); err != nil {
		t.Fatal(err)
	}
	p, err := s.ListPrograms(ctx)
	if err != nil || !slices.Equal(p.Libraries, []int64{shows.ID}) || len(p.Folders) != 1 {
		t.Fatalf("programs = %+v, %v", p, err)
	}
	if got, _ := s.GetLibrary(ctx, shows.ID); !got.Program {
		t.Error("library program = false")
	}
	if got, _ := s.GetTrack(ctx, ep.ID, ""); !got.Program {
		t.Error("track of a program library is not a program")
	}
	if al, _ := s.GetAlbum(ctx, ep.AlbumID, ""); !al.Program {
		t.Error("album of a program library is not a program")
	}
	if got, _ := listTracks(t, s, store.TrackQuery{Programs: store.ProgramsExclude}); !slices.Equal(trackTitles(got), []string{"Song"}) {
		t.Errorf("excluded = %v", trackTitles(got))
	}
	if cands, _ := s.RecommendCandidates(ctx, "", store.CandidateQuery{Limit: 10}); len(cands) != 1 || cands[0].ID == ep.ID {
		t.Errorf("candidates = %+v", cands)
	}

	shows.Program = false
	if err := s.UpdateLibrary(ctx, shows); err != nil {
		t.Fatal(err)
	}
	if err := s.RemoveProgramFolder(ctx, shows.ID, "Example Show"); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.GetTrack(ctx, ep.ID, ""); got.Program {
		t.Error("track stays a program after the library became music again")
	}
	if cands, _ := s.RecommendCandidates(ctx, "", store.CandidateQuery{Limit: 10}); len(cands) != 2 {
		t.Errorf("candidates after unmarking = %+v", cands)
	}
}
