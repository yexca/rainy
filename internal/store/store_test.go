package store_test

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"sort"
	"testing"
	"time"

	"rainy/internal/db/dbtest"
	"rainy/internal/model"
	"rainy/internal/store"
	"rainy/internal/util"
)

var ctx = context.Background()

func newStore(t *testing.T) *store.Store {
	t.Helper()
	return store.New(dbtest.New(t))
}

func newLibrary(t *testing.T, s *store.Store, path string) *model.Library {
	t.Helper()
	l := &model.Library{Name: "Music", Path: path}
	if err := s.CreateLibrary(ctx, l); err != nil {
		t.Fatal(err)
	}
	return l
}

// trk builds a track the way the scanner would (ids derived from names).
type trk struct {
	path, title, artist, album, albumArtist, genre string
	year, disc, track                              int
	duration                                       float64
	size                                           int64
	cover                                          bool
	created                                        int64
}

func (x trk) build(lib int64) model.Track {
	aa := x.albumArtist
	if aa == "" {
		aa = x.artist
	}
	t := model.Track{
		ID: util.NewID(), LibraryID: lib, Path: x.path, Size: x.size, Mtime: 1000,
		Title: x.title, Artist: x.artist, Album: x.album, AlbumArtist: aa,
		AlbumID: util.AlbumID(aa, x.album), ArtistID: util.ArtistID(x.artist), AlbumArtistID: util.ArtistID(aa),
		TrackNumber: x.track, DiscNumber: x.disc, Year: x.year, Genre: x.genre, Duration: x.duration,
		HasCover: x.cover, CreatedAt: x.created, Bitrate: 320,
	}
	return t
}

func upsert(t *testing.T, s *store.Store, lib int64, xs ...trk) []model.Track {
	t.Helper()
	ts := make([]model.Track, len(xs))
	for i, x := range xs {
		ts[i] = x.build(lib)
	}
	if err := s.UpsertTracks(ctx, ts); err != nil {
		t.Fatal(err)
	}
	if err := s.RefreshAll(ctx); err != nil {
		t.Fatal(err)
	}
	return ts
}

func ids[T any](items []T, id func(T) string) []string {
	out := make([]string, len(items))
	for i, it := range items {
		out[i] = id(it)
	}
	return out
}

func trackTitles(ts []model.Track) []string {
	return ids(ts, func(t model.Track) string { return t.Title })
}

func listTracks(t *testing.T, s *store.Store, q store.TrackQuery) ([]model.Track, int) {
	t.Helper()
	ts, total, err := s.ListTracks(ctx, q)
	if err != nil {
		t.Fatal(err)
	}
	return ts, total
}

// sampleLibrary: 2 albums by Alpha (one 2-disc), 1 album by 周杰伦, one compilation.
func sampleLibrary(t *testing.T, s *store.Store) (*model.Library, []model.Track) {
	lib := newLibrary(t, s, t.TempDir())
	ts := upsert(t, s, lib.ID,
		trk{path: "Alpha/First/01 One.mp3", title: "One", artist: "Alpha", album: "First", genre: "Rock", year: 2001, disc: 1, track: 1, duration: 100, size: 1000, created: 5000},
		trk{path: "Alpha/First/02 Two.mp3", title: "Two", artist: "Alpha", album: "First", genre: "Rock; Pop", year: 2002, disc: 1, track: 2, duration: 200, size: 2000, created: 4000, cover: true},
		trk{path: "Alpha/Second/CD2/01 Four.flac", title: "Four", artist: "Alpha", album: "Second", genre: "Pop", year: 2010, disc: 2, track: 1, duration: 50, size: 500, created: 7000, cover: true},
		trk{path: "Alpha/Second/CD1/01 Three.flac", title: "Three", artist: "Alpha", album: "Second", genre: "Pop", year: 2010, disc: 1, track: 1, duration: 50, size: 500, created: 7000, cover: true},
		trk{path: "周杰伦/七里香/01 七里香.flac", title: "七里香", artist: "周杰伦", album: "七里香", genre: "Mandopop", year: 2004, disc: 1, track: 1, duration: 300, size: 3000, created: 6000},
		trk{path: "VA/Hits/01 Guest.ogg", title: "Guest Song", artist: "Alpha", album: "Hits", albumArtist: "Various Artists", genre: "J-Pop", year: 1999, disc: 1, track: 1, duration: 10, size: 10, created: 8000},
		trk{path: "VA/Hits/02 Other.ogg", title: "100% Other_Song", artist: "Beta Band", album: "Hits", albumArtist: "Various Artists", genre: "J-Pop", year: 1999, disc: 1, track: 2, duration: 10, size: 10, created: 8000},
	)
	return lib, ts
}

func TestUpsertDerivedFieldsAndGenres(t *testing.T) {
	s := newStore(t)
	lib := newLibrary(t, s, t.TempDir())
	tr := trk{path: "A/B/01 Song.FLAC", title: "Sóng", artist: "Art", album: "Alb", genre: "Pop; Rock;pop"}.build(lib.ID)
	tr.CreatedAt = 0
	if err := s.UpsertTracks(ctx, []model.Track{tr}); err != nil {
		t.Fatal(err)
	}
	got, err := s.GetTrack(ctx, tr.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	if got.Dir != "A/B" || got.Filename != "01 Song.FLAC" || got.Suffix != "flac" || got.ContentType != "audio/flac" {
		t.Fatalf("derived path fields: %+v", got)
	}
	if got.Genre != "Pop; Rock" || !reflect.DeepEqual(got.Genres, []string{"Pop", "Rock"}) {
		t.Fatalf("genres: %q %v", got.Genre, got.Genres)
	}
	if got.SearchText != "song art alb art 01 song flac" || got.CreatedAt == 0 {
		t.Fatalf("search text %q created %d", got.SearchText, got.CreatedAt)
	}
	if got.CoverArt != util.CoverArtID("al", tr.AlbumID, got.AlbumUpdatedAt) || got.AlbumUpdatedAt == 0 {
		t.Fatalf("coverArt %q", got.CoverArt)
	}

	// Re-upsert keeps created_at and rewrites genres; Genres slice wins over Genre.
	created := got.CreatedAt
	tr.CreatedAt = 1
	tr.Genres = []string{"Jazz"}
	tr.Genre = "ignored"
	tr.Lyrics = "la la"
	tr.HasCover = true
	tr.UpdatedAt = 0
	if err := s.UpsertTracks(ctx, []model.Track{tr}); err != nil {
		t.Fatal(err)
	}
	if err := s.RefreshGenres(ctx); err != nil {
		t.Fatal(err)
	}
	got, _ = s.GetTrack(ctx, tr.ID, "")
	if got.CreatedAt != created || got.Genre != "Jazz" || got.Lyrics != "la la" || !got.HasLyrics {
		t.Fatalf("after re-upsert: created=%d genre=%q lyrics=%q", got.CreatedAt, got.Genre, got.Lyrics)
	}
	if got.CoverArt != util.CoverArtID("tr", tr.ID, got.UpdatedAt) {
		t.Fatalf("track cover %q", got.CoverArt)
	}
	genres, _ := s.ListGenres(ctx)
	if len(genres) != 1 || genres[0].Name != "Jazz" || genres[0].SongCount != 1 {
		t.Fatalf("genres %+v", genres)
	}
	// List queries do not load the lyrics text but still report HasLyrics.
	ts, _ := listTracks(t, s, store.TrackQuery{})
	if ts[0].Lyrics != "" || !ts[0].HasLyrics {
		t.Fatalf("list lyrics: %+v", ts[0])
	}
	// Duplicate path with a different id is a conflict.
	dup := tr
	dup.ID = util.NewID()
	if err := s.UpsertTracks(ctx, []model.Track{dup}); !errors.Is(err, store.ErrConflict) {
		t.Fatalf("want ErrConflict, got %v", err)
	}
}

func TestListTracksFilters(t *testing.T) {
	s := newStore(t)
	lib, ts := sampleLibrary(t, s)
	alpha := util.ArtistID("Alpha")
	cases := []struct {
		name string
		q    store.TrackQuery
		want []string
	}{
		{"all by title", store.TrackQuery{}, []string{"100% Other_Song", "Four", "Guest Song", "One", "Three", "Two", "七里香"}},
		{"album order", store.TrackQuery{AlbumID: util.AlbumID("Alpha", "Second"), Sort: "track"}, []string{"Three", "Four"}},
		{"artist or album artist", store.TrackQuery{ArtistID: util.ArtistID("Various Artists"), Sort: "track"}, []string{"Guest Song", "100% Other_Song"}},
		{"artist incl. guest", store.TrackQuery{ArtistID: alpha, Sort: "album"}, []string{"One", "Two", "Guest Song", "Three", "Four"}},
		{"genre", store.TrackQuery{Genre: "pop"}, []string{"Four", "Three", "Two"}},
		{"year range", store.TrackQuery{FromYear: 2002, ToYear: 2004}, []string{"Two", "七里香"}},
		{"year range swapped", store.TrackQuery{FromYear: 2004, ToYear: 2002}, []string{"Two", "七里香"}},
		{"dir", store.TrackQuery{Dir: "Alpha/Second/CD1"}, []string{"Three"}},
		{"dir prefix", store.TrackQuery{DirPrefix: "Alpha/Second/"}, []string{"Four", "Three"}},
		{"dir prefix no sibling", store.TrackQuery{DirPrefix: "Alpha/Sec"}, []string{}},
		{"ids", store.TrackQuery{IDs: []string{ts[0].ID, ts[4].ID, "nope"}}, []string{"One", "七里香"}},
		{"empty ids", store.TrackQuery{IDs: []string{}}, []string{}},
		{"library", store.TrackQuery{LibraryID: lib.ID + 1}, []string{}},
		{"q multi token", store.TrackQuery{Q: "alpha first"}, []string{"One", "Two"}},
		{"q cjk", store.TrackQuery{Q: "七里"}, []string{"七里香"}},
		{"q like escape", store.TrackQuery{Q: "100%"}, []string{"100% Other_Song"}},
		{"sort duration desc", store.TrackQuery{Sort: "duration", Order: "desc", Limit: 2}, []string{"七里香", "Two"}},
		{"sort recent", store.TrackQuery{Sort: "recent", Limit: 1}, []string{"100% Other_Song"}}, // created 8000, tie → id
		{"sort path", store.TrackQuery{Sort: "path", Limit: 2}, []string{"One", "Two"}},
		{"unknown sort → title", store.TrackQuery{Sort: "bogus", Order: "desc", Limit: 1}, []string{"七里香"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, total := listTracks(t, s, c.q)
			titles := trackTitles(got)
			if c.name == "sort recent" { // two tracks share created_at; either may win the id tiebreak
				if total != 7 || (titles[0] != "Guest Song" && titles[0] != "100% Other_Song") {
					t.Fatalf("got %v total %d", titles, total)
				}
				return
			}
			if !reflect.DeepEqual(titles, c.want) {
				t.Fatalf("got %q, want %q", titles, c.want)
			}
			if c.q.Limit == 0 && total != len(c.want) {
				t.Fatalf("total %d, want %d", total, len(c.want))
			}
		})
	}

	// Pagination: total ignores offset/limit, pages are stable and disjoint.
	var seen []string
	for off := 0; off < 7; off += 3 {
		page, total := listTracks(t, s, store.TrackQuery{Sort: "album", Offset: off, Limit: 3})
		if total != 7 {
			t.Fatalf("total %d", total)
		}
		seen = append(seen, trackTitles(page)...)
	}
	all, _ := listTracks(t, s, store.TrackQuery{Sort: "album"})
	if !reflect.DeepEqual(seen, trackTitles(all)) {
		t.Fatalf("paged %v != all %v", seen, trackTitles(all))
	}
	if page, total := listTracks(t, s, store.TrackQuery{Offset: 100, Limit: 5}); len(page) != 0 || total != 7 || page == nil {
		t.Fatalf("beyond end: %v %d", page, total)
	}
	if page, _ := listTracks(t, s, store.TrackQuery{Offset: 5}); len(page) != 2 {
		t.Fatalf("offset without limit: %d", len(page))
	}
}

func TestMissingTracks(t *testing.T) {
	s := newStore(t)
	_, ts := sampleLibrary(t, s)
	if err := s.MarkTracksMissing(ctx, []string{ts[0].ID, ts[1].ID}, true); err != nil {
		t.Fatal(err)
	}
	if _, total := listTracks(t, s, store.TrackQuery{}); total != 5 {
		t.Fatalf("default excludes missing: %d", total)
	}
	if _, total := listTracks(t, s, store.TrackQuery{Missing: "include"}); total != 7 {
		t.Fatalf("include: %d", total)
	}
	if got, _ := listTracks(t, s, store.TrackQuery{Missing: "only"}); !reflect.DeepEqual(trackTitles(got), []string{"One", "Two"}) {
		t.Fatalf("only: %v", trackTitles(got))
	}
	// GetTrack / GetTracks include missing tracks.
	if tr, err := s.GetTrack(ctx, ts[0].ID, ""); err != nil || !tr.Missing {
		t.Fatalf("GetTrack missing: %v %v", tr, err)
	}
	// Aggregates: album "First" has no non-missing tracks left → deleted.
	if err := s.RefreshAll(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := s.GetAlbum(ctx, util.AlbumID("Alpha", "First"), ""); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("album First should be gone: %v", err)
	}
	// Genre "Rock" only had missing tracks.
	genres, _ := s.ListGenres(ctx)
	for _, g := range genres {
		if g.Name == "Rock" {
			t.Fatal("Rock should not be listed")
		}
	}
	// Restore.
	if err := s.MarkTracksMissing(ctx, []string{ts[0].ID}, false); err != nil {
		t.Fatal(err)
	}
	if err := s.RefreshAlbums(ctx, []string{ts[0].AlbumID}); err != nil {
		t.Fatal(err)
	}
	if a, err := s.GetAlbum(ctx, ts[0].AlbumID, ""); err != nil || a.SongCount != 1 {
		t.Fatalf("album restored: %+v %v", a, err)
	}
	st, err := s.TrackFileStates(ctx, ts[0].LibraryID)
	if err != nil || len(st) != 7 || !st["Alpha/First/02 Two.mp3"].Missing || st["Alpha/First/01 One.mp3"].ID != ts[0].ID {
		t.Fatalf("file states %+v %v", st, err)
	}
}

func TestRefreshAggregates(t *testing.T) {
	s := newStore(t)
	_, ts := sampleLibrary(t, s)

	first, err := s.GetAlbum(ctx, util.AlbumID("Alpha", "First"), "")
	if err != nil {
		t.Fatal(err)
	}
	if first.Name != "First" || first.AlbumArtist != "Alpha" || first.AlbumArtistID != util.ArtistID("Alpha") ||
		first.Year != 2002 || first.SongCount != 2 || first.DiscCount != 1 || first.Duration != 300 || first.Size != 3000 ||
		first.CreatedAt != 4000 || first.CoverTrackID != ts[1].ID || first.Genre != "Rock" || first.SortName != "first" {
		t.Fatalf("album First: %+v", first)
	}
	second, _ := s.GetAlbum(ctx, util.AlbumID("Alpha", "Second"), "")
	if second.DiscCount != 2 || second.CoverTrackID != ts[3].ID { // disc 1 track wins over disc 2
		t.Fatalf("album Second: discs=%d cover=%s want %s", second.DiscCount, second.CoverTrackID, ts[3].ID)
	}
	hits, _ := s.GetAlbum(ctx, util.AlbumID("Various Artists", "Hits"), "")
	if hits.AlbumArtist != "Various Artists" || hits.SongCount != 2 || hits.CoverTrackID != "" {
		t.Fatalf("album Hits: %+v", hits)
	}

	alpha, err := s.GetArtist(ctx, util.ArtistID("Alpha"), "")
	if err != nil {
		t.Fatal(err)
	}
	if alpha.AlbumCount != 2 || alpha.SongCount != 5 || alpha.IndexKey != "A" || alpha.SortName != "alpha" {
		t.Fatalf("artist Alpha: %+v", alpha)
	}
	jay, _ := s.GetArtist(ctx, util.ArtistID("周杰伦"), "")
	if jay.IndexKey != "Z" || jay.AlbumCount != 1 || jay.SongCount != 1 {
		t.Fatalf("artist 周杰伦: %+v", jay)
	}
	beta, _ := s.GetArtist(ctx, util.ArtistID("Beta Band"), "")
	if beta.AlbumCount != 0 || beta.SongCount != 1 {
		t.Fatalf("artist Beta: %+v", beta)
	}
	va, _ := s.GetArtist(ctx, util.ArtistID("Various Artists"), "")
	if va.AlbumCount != 1 || va.SongCount != 2 {
		t.Fatalf("artist VA: %+v", va)
	}

	// Appears on: Alpha appears on Hits (album artist Various Artists).
	ap, err := s.AlbumsAppearsOn(ctx, alpha.ID, "")
	if err != nil || len(ap) != 1 || ap[0].Name != "Hits" {
		t.Fatalf("appears on: %+v %v", ap, err)
	}

	// Idempotent refresh leaves updated_at untouched.
	if err := s.RefreshAll(ctx); err != nil {
		t.Fatal(err)
	}
	again, _ := s.GetAlbum(ctx, first.ID, "")
	if again.UpdatedAt != first.UpdatedAt {
		t.Fatalf("updated_at changed without changes: %d → %d", first.UpdatedAt, again.UpdatedAt)
	}

	// cover_path is preserved by refresh; SetAlbumCoverPath bumps updated_at.
	if err := s.SetAlbumCoverPath(ctx, first.ID, "/music/cover.jpg"); err != nil {
		t.Fatal(err)
	}
	withCover, _ := s.GetAlbum(ctx, first.ID, "")
	if withCover.CoverPath != "/music/cover.jpg" || withCover.UpdatedAt <= first.UpdatedAt {
		t.Fatalf("cover path: %+v", withCover)
	}
	if err := s.SetAlbumCoverPath(ctx, "nope", "x"); !errors.Is(err, store.ErrNotFound) {
		t.Fatal(err)
	}

	// Changing a track (new year) bumps the album.
	tr := ts[0]
	tr.Year = 2020
	tr.UpdatedAt = 0
	if err := s.UpsertTracks(ctx, []model.Track{tr}); err != nil {
		t.Fatal(err)
	}
	if err := s.RefreshAlbums(ctx, []string{tr.AlbumID}); err != nil {
		t.Fatal(err)
	}
	changed, _ := s.GetAlbum(ctx, first.ID, "")
	if changed.Year != 2020 || changed.UpdatedAt <= withCover.UpdatedAt || changed.CoverPath != "/music/cover.jpg" {
		t.Fatalf("after change: %+v", changed)
	}
	if err := s.TouchAlbums(ctx, []string{first.ID}); err != nil {
		t.Fatal(err)
	}
	touched, _ := s.GetAlbum(ctx, first.ID, "")
	if touched.UpdatedAt <= changed.UpdatedAt {
		t.Fatal("TouchAlbums must strictly increase updated_at")
	}

	// Moving a track to another album via tag change: old album shrinks, new appears.
	tr.Album = "Renamed"
	tr.AlbumID = util.AlbumID("Alpha", "Renamed")
	if err := s.UpsertTracks(ctx, []model.Track{tr}); err != nil {
		t.Fatal(err)
	}
	if err := s.RefreshAlbums(ctx, []string{first.ID, tr.AlbumID}); err != nil {
		t.Fatal(err)
	}
	if a, _ := s.GetAlbum(ctx, first.ID, ""); a.SongCount != 1 {
		t.Fatalf("old album count %d", a.SongCount)
	}
	if a, err := s.GetAlbum(ctx, tr.AlbumID, ""); err != nil || a.SongCount != 1 {
		t.Fatalf("new album %+v %v", a, err)
	}

	// Deleting all of an artist's tracks removes the artist and album.
	if err := s.DeleteTracks(ctx, []string{ts[4].ID}); err != nil {
		t.Fatal(err)
	}
	if err := s.RefreshAlbums(ctx, []string{ts[4].AlbumID}); err != nil {
		t.Fatal(err)
	}
	if err := s.RefreshArtists(ctx, []string{ts[4].ArtistID}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.GetArtist(ctx, jay.ID, ""); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("artist should be deleted: %v", err)
	}
	if _, err := s.GetAlbum(ctx, ts[4].AlbumID, ""); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("album should be deleted: %v", err)
	}
	if err := s.SetArtistImagePath(ctx, alpha.ID, "/x/artist.jpg"); err != nil {
		t.Fatal(err)
	}
	if err := s.RefreshArtists(ctx, nil); err != nil {
		t.Fatal(err)
	}
	if a, _ := s.GetArtist(ctx, alpha.ID, ""); a.ImagePath != "/x/artist.jpg" {
		t.Fatal("image path must survive refresh")
	}
}

func TestRefreshMostCommonAndSortTags(t *testing.T) {
	s := newStore(t)
	lib := newLibrary(t, s, t.TempDir())
	mk := func(path, album, genre string, year int) model.Track {
		tr := trk{path: path, title: path, artist: "The Beatles", album: album, genre: genre, year: year, disc: 1}.build(lib.ID)
		tr.AlbumID = util.AlbumID("The Beatles", "abbey road")
		return tr
	}
	tracks := []model.Track{
		mk("1.mp3", "abbey road", "Rock", 1969),
		mk("2.mp3", "Abbey Road", "Rock", 1969),
		mk("3.mp3", "Abbey Road", "Pop", 2019),
	}
	tracks[0].SortArtist = "Beatles, The"
	if err := s.UpsertTracks(ctx, tracks); err != nil {
		t.Fatal(err)
	}
	if err := s.RefreshAll(ctx); err != nil {
		t.Fatal(err)
	}
	a, err := s.GetAlbum(ctx, util.AlbumID("The Beatles", "abbey road"), "")
	if err != nil {
		t.Fatal(err)
	}
	if a.Name != "Abbey Road" || a.Genre != "Rock" || a.Year != 2019 {
		t.Fatalf("%+v", a)
	}
	ar, _ := s.GetArtist(ctx, util.ArtistID("The Beatles"), "")
	if ar.SortName != "beatles the" || ar.IndexKey != "B" {
		t.Fatalf("sort tag: %+v", ar)
	}
}

func TestListAlbumsAndArtists(t *testing.T) {
	s := newStore(t)
	sampleLibrary(t, s)
	u := &model.User{Username: "u", PasswordEnc: "synthetic"}
	if err := s.CreateUser(ctx, u); err != nil {
		t.Fatal(err)
	}
	names := func(as []model.Album) []string { return ids(as, func(a model.Album) string { return a.Name }) }

	as, total, err := s.ListAlbums(ctx, store.AlbumQuery{})
	if err != nil || total != 4 || !reflect.DeepEqual(names(as), []string{"First", "Hits", "Second", "七里香"}) {
		t.Fatalf("albums %v %d %v", names(as), total, err)
	}
	as, _, _ = s.ListAlbums(ctx, store.AlbumQuery{Sort: "year", Order: "desc"})
	if !reflect.DeepEqual(names(as), []string{"Second", "七里香", "First", "Hits"}) {
		t.Fatalf("by year desc %v", names(as))
	}
	as, total, _ = s.ListAlbums(ctx, store.AlbumQuery{ArtistID: util.ArtistID("Alpha")})
	if total != 2 {
		t.Fatalf("artist albums %v", names(as))
	}
	as, _, _ = s.ListAlbums(ctx, store.AlbumQuery{Genre: "J-POP"})
	if !reflect.DeepEqual(names(as), []string{"Hits"}) {
		t.Fatalf("genre %v", names(as))
	}
	as, _, _ = s.ListAlbums(ctx, store.AlbumQuery{FromYear: 2010, ToYear: 2000})
	if !reflect.DeepEqual(names(as), []string{"First", "Second", "七里香"}) {
		t.Fatalf("years %v", names(as))
	}
	as, total, _ = s.ListAlbums(ctx, store.AlbumQuery{Sort: "recent", Limit: 1})
	if total != 4 || names(as)[0] != "Hits" {
		t.Fatalf("recent %v", names(as))
	}
	as, _, _ = s.ListAlbums(ctx, store.AlbumQuery{Q: "七里"})
	if !reflect.DeepEqual(names(as), []string{"七里香"}) {
		t.Fatalf("q %v", names(as))
	}
	as, _, _ = s.ListAlbums(ctx, store.AlbumQuery{Sort: "artist"})
	if !reflect.DeepEqual(names(as), []string{"First", "Second", "Hits", "七里香"}) {
		t.Fatalf("by artist %v", names(as))
	}

	// Starred + per-user fields.
	second := util.AlbumID("Alpha", "Second")
	if err := s.SetStarred(ctx, u.ID, "album", []string{second}, true); err != nil {
		t.Fatal(err)
	}
	as, total, _ = s.ListAlbums(ctx, store.AlbumQuery{UserID: u.ID, Starred: true})
	if total != 1 || !as[0].Starred || as[0].StarredAt == nil {
		t.Fatalf("starred %+v", as)
	}
	as, _, _ = s.ListAlbums(ctx, store.AlbumQuery{Starred: true}) // no user → nothing starred
	if len(as) != 0 {
		t.Fatal("starred without user")
	}
	as, _, _ = s.ListAlbums(ctx, store.AlbumQuery{UserID: u.ID, Sort: "starred"})
	if names(as)[0] != "Second" {
		t.Fatalf("sort starred %v", names(as))
	}

	artists, total, err := s.ListArtists(ctx, store.ArtistQuery{})
	an := ids(artists, func(a model.Artist) string { return a.Name })
	if err != nil || total != 4 || !reflect.DeepEqual(an, []string{"Alpha", "Beta Band", "Various Artists", "周杰伦"}) {
		t.Fatalf("artists %v %d %v", an, total, err)
	}
	artists, total, _ = s.ListArtists(ctx, store.ArtistQuery{AlbumArtistsOnly: true, Sort: "albumCount"})
	an = ids(artists, func(a model.Artist) string { return a.Name })
	if total != 3 || an[0] != "Alpha" {
		t.Fatalf("album artists %v", an)
	}
	if artists[0].CoverArt != util.CoverArtID("ar", artists[0].ID, artists[0].UpdatedAt) {
		t.Fatal("artist cover art")
	}
}

func TestSearch(t *testing.T) {
	s := newStore(t)
	sampleLibrary(t, s)
	all := store.SearchQuery{ArtistLimit: 100, AlbumLimit: 100, TrackLimit: 100}

	res, err := s.Search(ctx, all)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Artists) != 4 || len(res.Albums) != 4 || len(res.Tracks) != 7 {
		t.Fatalf("empty query must match everything: %d %d %d", len(res.Artists), len(res.Albums), len(res.Tracks))
	}
	q := all
	q.Q = "周杰伦"
	res, _ = s.Search(ctx, q)
	if len(res.Artists) != 1 || len(res.Albums) != 1 || len(res.Tracks) != 1 {
		t.Fatalf("cjk: %+v", res)
	}
	q.Q = "alpha two"
	res, _ = s.Search(ctx, q)
	if len(res.Artists) != 0 || len(res.Albums) != 0 || len(res.Tracks) != 1 || res.Tracks[0].Title != "Two" {
		t.Fatalf("multi token: %+v", res)
	}
	q.Q = "ALPHA  "
	res, _ = s.Search(ctx, q)
	if len(res.Artists) != 1 || len(res.Albums) != 2 || len(res.Tracks) != 5 {
		t.Fatalf("case-insensitive: %d %d %d", len(res.Artists), len(res.Albums), len(res.Tracks))
	}
	q.Q = "zzz"
	res, _ = s.Search(ctx, q)
	if res.Artists == nil || res.Albums == nil || res.Tracks == nil || len(res.Tracks) != 0 {
		t.Fatalf("no match must give empty slices: %+v", res)
	}
	// Paging and zero limits.
	res, _ = s.Search(ctx, store.SearchQuery{TrackOffset: 5, TrackLimit: 10})
	if len(res.Artists) != 0 || len(res.Albums) != 0 || len(res.Tracks) != 2 {
		t.Fatalf("paging: %d %d %d", len(res.Artists), len(res.Albums), len(res.Tracks))
	}
	b, _ := json.Marshal(res)
	if string(b[:13]) != `{"artists":[]` {
		t.Fatalf("json %s", b)
	}
}

func TestGetTracksOrderAndPaths(t *testing.T) {
	s := newStore(t)
	lib, ts := sampleLibrary(t, s)
	got, err := s.GetTracks(ctx, []string{ts[2].ID, "unknown", ts[0].ID, ts[2].ID}, "")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(trackTitles(got), []string{"Four", "One", "Four"}) {
		t.Fatalf("order %v", trackTitles(got))
	}
	if got, err := s.GetTracks(ctx, nil, ""); err != nil || got == nil || len(got) != 0 {
		t.Fatal("empty")
	}
	byPath, err := s.GetTrackByPath(ctx, lib.ID, "Alpha/First/01 One.mp3")
	if err != nil || byPath.ID != ts[0].ID {
		t.Fatal(err)
	}
	if _, err := s.GetTrack(ctx, "nope", ""); !errors.Is(err, store.ErrNotFound) {
		t.Fatal(err)
	}
	if err := s.UpdateTrackPath(ctx, ts[0].ID, "New/Dir/uno.OGG"); err != nil {
		t.Fatal(err)
	}
	moved, _ := s.GetTrack(ctx, ts[0].ID, "")
	if moved.Path != "New/Dir/uno.OGG" || moved.Dir != "New/Dir" || moved.Filename != "uno.OGG" || moved.Suffix != "ogg" ||
		moved.SearchText != util.NormalizeSearch("One", "Alpha", "First", "Alpha", "", "uno.OGG") {
		t.Fatalf("moved %+v", moved)
	}
	if err := s.UpdateTrackPath(ctx, ts[0].ID, ts[1].Path); !errors.Is(err, store.ErrConflict) {
		t.Fatalf("path conflict: %v", err)
	}
	if err := s.UpdateTrackPath(ctx, "nope", "x"); !errors.Is(err, store.ErrNotFound) {
		t.Fatal(err)
	}
	albumIDs, _ := s.AlbumIDsForTracks(ctx, []string{ts[0].ID, ts[1].ID, ts[5].ID})
	sort.Strings(albumIDs)
	want := []string{ts[0].AlbumID, ts[5].AlbumID}
	sort.Strings(want)
	if !reflect.DeepEqual(albumIDs, want) {
		t.Fatalf("album ids %v", albumIDs)
	}
	artistIDs, _ := s.ArtistIDsForTracks(ctx, []string{ts[5].ID})
	sort.Strings(artistIDs)
	want = []string{util.ArtistID("Alpha"), util.ArtistID("Various Artists")}
	sort.Strings(want)
	if !reflect.DeepEqual(artistIDs, want) {
		t.Fatalf("artist ids %v", artistIDs)
	}
}

func TestAnnotationsAndPlays(t *testing.T) {
	s := newStore(t)
	_, ts := sampleLibrary(t, s)
	u := &model.User{Username: "u", PasswordEnc: "synthetic"}
	_ = s.CreateUser(ctx, u)

	if err := s.SetStarred(ctx, u.ID, "track", []string{ts[0].ID, ts[1].ID}, true); err != nil {
		t.Fatal(err)
	}
	first, _ := s.GetTrack(ctx, ts[0].ID, u.ID)
	if !first.Starred || first.StarredAt == nil {
		t.Fatal("not starred")
	}
	time.Sleep(3 * time.Millisecond)
	_ = s.SetStarred(ctx, u.ID, "track", []string{ts[0].ID}, true)
	again, _ := s.GetTrack(ctx, ts[0].ID, u.ID)
	if *again.StarredAt != *first.StarredAt {
		t.Fatal("re-star must keep starred_at")
	}
	_ = s.SetStarred(ctx, u.ID, "track", []string{ts[1].ID}, false)
	got, _ := listTracks(t, s, store.TrackQuery{UserID: u.ID, Starred: true})
	if !reflect.DeepEqual(trackTitles(got), []string{"One"}) {
		t.Fatalf("starred %v", trackTitles(got))
	}
	if err := s.SetStarred(ctx, u.ID, "song", []string{"x"}, true); !errors.Is(err, store.ErrInvalid) {
		t.Fatal(err)
	}

	if err := s.SetRating(ctx, u.ID, "track", ts[0].ID, 4); err != nil {
		t.Fatal(err)
	}
	if err := s.SetRating(ctx, u.ID, "track", ts[0].ID, 6); !errors.Is(err, store.ErrInvalid) {
		t.Fatal(err)
	}
	if tr, _ := s.GetTrack(ctx, ts[0].ID, u.ID); tr.Rating != 4 || !tr.Starred {
		t.Fatalf("rating %d", tr.Rating)
	}
	_ = s.SetRating(ctx, u.ID, "track", ts[0].ID, 0)
	if tr, _ := s.GetTrack(ctx, ts[0].ID, u.ID); tr.Rating != 0 || !tr.Starred {
		t.Fatal("clearing rating must keep star")
	}

	// Plays: Guest Song (artist Alpha, album artist VA) twice, One once.
	for i, at := range []int64{1000, 3000} {
		if err := s.RecordPlay(ctx, u.ID, ts[5].ID, at, "test"); err != nil {
			t.Fatal(i, err)
		}
	}
	_ = s.RecordPlay(ctx, u.ID, ts[0].ID, 2000, "test")
	if err := s.RecordPlay(ctx, u.ID, "nope", 0, ""); !errors.Is(err, store.ErrNotFound) {
		t.Fatal(err)
	}
	guest, _ := s.GetTrack(ctx, ts[5].ID, u.ID)
	if guest.PlayCount != 2 || guest.PlayedAt != 3000 {
		t.Fatalf("track plays %d %d", guest.PlayCount, guest.PlayedAt)
	}
	hits, _ := s.GetAlbum(ctx, ts[5].AlbumID, u.ID)
	if hits.PlayCount != 2 || hits.PlayedAt != 3000 {
		t.Fatalf("album plays %+v", hits)
	}
	alpha, _ := s.GetArtist(ctx, util.ArtistID("Alpha"), u.ID)
	va, _ := s.GetArtist(ctx, util.ArtistID("Various Artists"), u.ID)
	if alpha.PlayCount != 3 || va.PlayCount != 2 {
		t.Fatalf("artist plays alpha=%d va=%d", alpha.PlayCount, va.PlayCount)
	}
	// Other users see nothing.
	if tr, _ := s.GetTrack(ctx, ts[5].ID, ""); tr.PlayCount != 0 {
		t.Fatal("annotation leaked")
	}
	recent, err := s.RecentlyPlayedTracks(ctx, u.ID, 10)
	if err != nil || !reflect.DeepEqual(trackTitles(recent), []string{"Guest Song", "One"}) {
		t.Fatalf("recent %v %v", trackTitles(recent), err)
	}
	played, _ := listTracks(t, s, store.TrackQuery{UserID: u.ID, Played: true, Sort: "frequent"})
	if !reflect.DeepEqual(trackTitles(played), []string{"Guest Song", "One"}) {
		t.Fatalf("frequent %v", trackTitles(played))
	}
	as, _, _ := s.ListAlbums(ctx, store.AlbumQuery{UserID: u.ID, Played: true, Sort: "played"})
	if len(as) != 2 || as[0].ID != ts[5].AlbumID {
		t.Fatalf("recently played albums %+v", as)
	}
	st, _ := s.Stats(ctx)
	if st.PlaysLast30Days != 0 { // plays at t=1000ms are in 1970
		t.Fatalf("plays30 %d", st.PlaysLast30Days)
	}
	_ = s.RecordPlay(ctx, u.ID, ts[0].ID, 0, "")
	if st, _ = s.Stats(ctx); st.PlaysLast30Days != 1 {
		t.Fatalf("plays30 %d", st.PlaysLast30Days)
	}
}

func TestPlaylists(t *testing.T) {
	s := newStore(t)
	_, ts := sampleLibrary(t, s)
	owner := &model.User{Username: "owner", PasswordEnc: "synthetic"}
	other := &model.User{Username: "other", PasswordEnc: "synthetic"}
	_ = s.CreateUser(ctx, owner)
	_ = s.CreateUser(ctx, other)

	p := &model.Playlist{Name: "Mix", OwnerID: owner.ID}
	if err := s.CreatePlaylist(ctx, p, []string{ts[0].ID, "unknown", ts[1].ID, ts[0].ID}); err != nil {
		t.Fatal(err)
	}
	got, err := s.GetPlaylist(ctx, p.ID)
	if err != nil || got.SongCount != 3 || got.Duration != 400 || got.OwnerName != "owner" || got.CoverArt == "" {
		t.Fatalf("playlist %+v %v", got, err)
	}
	tracks, _ := s.PlaylistTracks(ctx, p.ID, owner.ID)
	if !reflect.DeepEqual(trackTitles(tracks), []string{"One", "Two", "One"}) {
		t.Fatalf("tracks %v", trackTitles(tracks))
	}
	if err := s.AppendPlaylistTracks(ctx, p.ID, []string{ts[4].ID}); err != nil {
		t.Fatal(err)
	}
	// Hide "Two" (missing): visible = One, One, 七里香; remove visible index 1 (second One).
	_ = s.MarkTracksMissing(ctx, []string{ts[1].ID}, true)
	tracks, _ = s.PlaylistTracks(ctx, p.ID, owner.ID)
	if !reflect.DeepEqual(trackTitles(tracks), []string{"One", "One", "七里香"}) {
		t.Fatalf("visible %v", trackTitles(tracks))
	}
	if err := s.RemovePlaylistPositions(ctx, p.ID, []int{1, 99, -1}); err != nil {
		t.Fatal(err)
	}
	_ = s.MarkTracksMissing(ctx, []string{ts[1].ID}, false)
	tracks, _ = s.PlaylistTracks(ctx, p.ID, owner.ID)
	if !reflect.DeepEqual(trackTitles(tracks), []string{"One", "Two", "七里香"}) {
		t.Fatalf("after removal %v", trackTitles(tracks))
	}
	var positions []int
	_ = s.DB().R.Select(&positions, `SELECT position FROM playlist_tracks WHERE playlist_id = ? ORDER BY position`, p.ID)
	if !reflect.DeepEqual(positions, []int{0, 1, 2}) {
		t.Fatalf("positions not contiguous: %v", positions)
	}
	if err := s.SetPlaylistTracks(ctx, p.ID, []string{ts[6].ID, ts[5].ID}); err != nil {
		t.Fatal(err)
	}
	tracks, _ = s.PlaylistTracks(ctx, p.ID, owner.ID)
	if !reflect.DeepEqual(trackTitles(tracks), []string{"100% Other_Song", "Guest Song"}) {
		t.Fatalf("set %v", trackTitles(tracks))
	}
	before, _ := s.GetPlaylist(ctx, p.ID)
	p.Name, p.Public = "Public Mix", true
	if err := s.UpdatePlaylist(ctx, p); err != nil {
		t.Fatal(err)
	}
	after, _ := s.GetPlaylist(ctx, p.ID)
	if after.Name != "Public Mix" || !after.Public || after.UpdatedAt <= before.UpdatedAt {
		t.Fatalf("update %+v", after)
	}

	private := &model.Playlist{Name: "Private", OwnerID: owner.ID}
	_ = s.CreatePlaylist(ctx, private, nil)
	if l, _ := s.ListPlaylists(ctx, other.ID); len(l) != 1 || l[0].ID != p.ID {
		t.Fatalf("other sees %+v", l)
	}
	if l, _ := s.ListPlaylists(ctx, owner.ID); len(l) != 2 {
		t.Fatalf("owner sees %d", len(l))
	}
	if err := s.DeletePlaylist(ctx, p.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.GetPlaylist(ctx, p.ID); !errors.Is(err, store.ErrNotFound) {
		t.Fatal(err)
	}
	if err := s.AppendPlaylistTracks(ctx, p.ID, []string{ts[0].ID}); !errors.Is(err, store.ErrNotFound) {
		t.Fatal(err)
	}
}

func TestUsersAndSessions(t *testing.T) {
	s := newStore(t)
	u := &model.User{Username: "Alice", PasswordEnc: "synthetic-enc", IsAdmin: true, CanDownload: true}
	if err := s.CreateUser(ctx, u); err != nil {
		t.Fatal(err)
	}
	if u.ID == "" || u.CreatedAt == 0 {
		t.Fatal("id/timestamps not set")
	}
	if err := s.CreateUser(ctx, &model.User{Username: "alice", PasswordEnc: "synthetic"}); !errors.Is(err, store.ErrConflict) {
		t.Fatalf("duplicate username: %v", err)
	}
	got, err := s.GetUserByUsername(ctx, "ALICE")
	if err != nil || got.ID != u.ID || !got.IsAdmin || got.HasAPIKey {
		t.Fatalf("%+v %v", got, err)
	}
	if n, _ := s.CountUsers(ctx); n != 1 {
		t.Fatal(n)
	}
	h := "hash1"
	if err := s.SetUserAPIKeyHash(ctx, u.ID, &h); err != nil {
		t.Fatal(err)
	}
	if got, err := s.GetUserByAPIKeyHash(ctx, "hash1"); err != nil || got.ID != u.ID || !got.HasAPIKey {
		t.Fatal(err)
	}
	if _, err := s.GetUserByAPIKeyHash(ctx, ""); !errors.Is(err, store.ErrNotFound) {
		t.Fatal(err)
	}
	_ = s.SetUserAPIKeyHash(ctx, u.ID, nil)
	if got, _ := s.GetUser(ctx, u.ID); got.HasAPIKey {
		t.Fatal("api key not cleared")
	}
	u.DisplayName, u.CanManage = "Alice A.", true
	if err := s.UpdateUser(ctx, u); err != nil {
		t.Fatal(err)
	}
	if err := s.SetUserPassword(ctx, u.ID, "enc2"); err != nil {
		t.Fatal(err)
	}
	_ = s.TouchUserLogin(ctx, u.ID)
	got, _ = s.GetUser(ctx, u.ID)
	if got.DisplayName != "Alice A." || !got.CanManage || got.PasswordEnc != "enc2" || got.LastLoginAt == 0 || got.LastSeenAt == 0 {
		t.Fatalf("%+v", got)
	}
	if err := s.UpdateUser(ctx, &model.User{ID: "nope"}); !errors.Is(err, store.ErrNotFound) {
		t.Fatal(err)
	}
	if users, _ := s.ListUsers(ctx); len(users) != 1 {
		t.Fatal("list")
	}

	now := util.NowMs()
	live := &model.Session{TokenHash: "synthetic-live", UserID: u.ID, ExpiresAt: now + 60000}
	dead := &model.Session{TokenHash: "synthetic-dead", UserID: u.ID, ExpiresAt: now - 1}
	for _, ss := range []*model.Session{live, dead} {
		if err := s.CreateSession(ctx, ss); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.TouchSession(ctx, "synthetic-live", now+120000); err != nil {
		t.Fatal(err)
	}
	if got, err := s.GetSession(ctx, "synthetic-live"); err != nil || got.ExpiresAt != now+120000 || got.UserID != u.ID {
		t.Fatalf("%+v %v", got, err)
	}
	if err := s.TouchSession(ctx, "nope", 1); !errors.Is(err, store.ErrNotFound) {
		t.Fatal(err)
	}
	_ = s.PurgeExpiredSessions(ctx)
	if _, err := s.GetSession(ctx, "synthetic-dead"); !errors.Is(err, store.ErrNotFound) {
		t.Fatal("expired session not purged")
	}
	_ = s.DeleteSession(ctx, "synthetic-live")
	if _, err := s.GetSession(ctx, "synthetic-live"); !errors.Is(err, store.ErrNotFound) {
		t.Fatal("session not deleted")
	}
	_ = s.CreateSession(ctx, &model.Session{TokenHash: "synthetic-a", UserID: u.ID, ExpiresAt: now + 1000})
	if err := s.DeleteUser(ctx, u.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.GetSession(ctx, "synthetic-a"); !errors.Is(err, store.ErrNotFound) {
		t.Fatal("sessions must cascade")
	}
	if err := s.DeleteUser(ctx, u.ID); !errors.Is(err, store.ErrNotFound) {
		t.Fatal(err)
	}
}

func TestLibraries(t *testing.T) {
	s := newStore(t)
	lib, _ := sampleLibrary(t, s)
	if err := s.CreateLibrary(ctx, &model.Library{Name: "dup", Path: lib.Path}); !errors.Is(err, store.ErrConflict) {
		t.Fatalf("dup path: %v", err)
	}
	other := newLibrary(t, s, t.TempDir())
	upsert(t, s, other.ID, trk{path: "x.mp3", title: "X", artist: "Alpha", album: "Else"})
	counts, _ := s.LibraryTrackCounts(ctx)
	if counts[lib.ID] != 7 || counts[other.ID] != 1 {
		t.Fatalf("counts %v", counts)
	}
	libs, _ := s.ListLibraries(ctx)
	if len(libs) != 2 {
		t.Fatal(len(libs))
	}
	other.Name = "Other"
	if err := s.UpdateLibrary(ctx, other); err != nil {
		t.Fatal(err)
	}
	if err := s.SetLibraryScanned(ctx, other.ID, 42); err != nil {
		t.Fatal(err)
	}
	if l, _ := s.GetLibrary(ctx, other.ID); l.Name != "Other" || l.LastScanAt != 42 {
		t.Fatalf("%+v", l)
	}
	if err := s.DeleteLibrary(ctx, lib.ID); err != nil {
		t.Fatal(err)
	}
	if _, total := listTracks(t, s, store.TrackQuery{Missing: "include"}); total != 1 {
		t.Fatalf("tracks after library delete: %d", total)
	}
	if _, total, _ := s.ListAlbums(ctx, store.AlbumQuery{}); total != 1 {
		t.Fatalf("albums after library delete: %d", total)
	}
	if _, total, _ := s.ListArtists(ctx, store.ArtistQuery{}); total != 1 {
		t.Fatalf("artists after library delete: %d", total)
	}
	if err := s.DeleteLibrary(ctx, lib.ID); !errors.Is(err, store.ErrNotFound) {
		t.Fatal(err)
	}
	if _, err := s.GetLibrary(ctx, lib.ID); !errors.Is(err, store.ErrNotFound) {
		t.Fatal(err)
	}
}

func TestSettings(t *testing.T) {
	s := newStore(t)
	def := model.DefaultSettings(time.Hour)
	got, err := s.GetSettings(ctx, def)
	if err != nil || got != def {
		t.Fatalf("defaults: %+v %v", got, err)
	}
	changed := def
	changed.TranscodeBitrate = 128
	changed.EnableDownloads = false
	if err := s.SaveSettings(ctx, changed); err != nil {
		t.Fatal(err)
	}
	if got, _ = s.GetSettings(ctx, def); got != changed {
		t.Fatalf("saved: %+v", got)
	}
	// A partial store (only some keys) merges over new defaults; bad values are ignored.
	_, _ = s.DB().W.Exec(`DELETE FROM settings WHERE key != 'transcodeBitrate'`)
	_, _ = s.DB().W.Exec(`INSERT INTO settings (key, value) VALUES ('transcodeFormat', '42'), ('unknown', '"x"'), ('renamePattern', 'not json')`)
	def2 := model.DefaultSettings(0)
	got, _ = s.GetSettings(ctx, def2)
	want := def2
	want.TranscodeBitrate = 128
	if got != want {
		t.Fatalf("merge: %+v\nwant %+v", got, want)
	}
}

func TestQueueBookmarksRadioTrashLog(t *testing.T) {
	s := newStore(t)
	u := &model.User{Username: "u", PasswordEnc: "synthetic"}
	_ = s.CreateUser(ctx, u)

	q, err := s.GetPlayQueue(ctx, u.ID)
	if err != nil || q.TrackIDs == nil || len(q.TrackIDs) != 0 {
		t.Fatalf("empty queue %+v %v", q, err)
	}
	if err := s.SavePlayQueue(ctx, u.ID, &model.PlayQueue{TrackIDs: []string{"a", "b", "a"}, CurrentID: "b", PositionMs: 1234, ChangedBy: "web"}); err != nil {
		t.Fatal(err)
	}
	q, _ = s.GetPlayQueue(ctx, u.ID)
	if !reflect.DeepEqual(q.TrackIDs, []string{"a", "b", "a"}) || q.CurrentID != "b" || q.PositionMs != 1234 || q.UpdatedAt == 0 {
		t.Fatalf("%+v", q)
	}

	if err := s.UpsertBookmark(ctx, u.ID, &model.Bookmark{TrackID: "t1", PositionMs: 10}); err != nil {
		t.Fatal(err)
	}
	_ = s.UpsertBookmark(ctx, u.ID, &model.Bookmark{TrackID: "t1", PositionMs: 20, Comment: "c"})
	bms, _ := s.ListBookmarks(ctx, u.ID)
	if len(bms) != 1 || bms[0].PositionMs != 20 || bms[0].Comment != "c" {
		t.Fatalf("%+v", bms)
	}
	if err := s.DeleteBookmark(ctx, u.ID, "t1"); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteBookmark(ctx, u.ID, "t1"); !errors.Is(err, store.ErrNotFound) {
		t.Fatal(err)
	}

	r := &model.RadioStation{Name: "Radio", StreamURL: "https://radio.example.com/stream"}
	if err := s.CreateRadioStation(ctx, r); err != nil || r.ID == "" {
		t.Fatal(err)
	}
	r.HomepageURL = "https://radio.example.com"
	_ = s.UpdateRadioStation(ctx, r)
	if got, _ := s.GetRadioStation(ctx, r.ID); got.HomepageURL != "https://radio.example.com" {
		t.Fatal("radio update")
	}
	if l, _ := s.ListRadioStations(ctx); len(l) != 1 {
		t.Fatal("radio list")
	}
	_ = s.DeleteRadioStation(ctx, r.ID)
	if _, err := s.GetRadioStation(ctx, r.ID); !errors.Is(err, store.ErrNotFound) {
		t.Fatal(err)
	}

	e := &model.TrashEntry{LibraryID: 1, OriginalPath: "a/b.mp3", TrashPath: "1/a/b.mp3", Title: "B"}
	if err := s.AddTrash(ctx, e); err != nil || e.ID == "" || e.DeletedAt == 0 {
		t.Fatal(err)
	}
	if l, _ := s.ListTrash(ctx); len(l) != 1 || l[0].Title != "B" {
		t.Fatal("trash list")
	}
	_ = s.DeleteTrash(ctx, e.ID)
	if _, err := s.GetTrash(ctx, e.ID); !errors.Is(err, store.ErrNotFound) {
		t.Fatal(err)
	}

	for i, tid := range []string{"t1", "t2", "t1"} {
		entry := &model.EditLogEntry{UserID: u.ID, Username: "u", Action: "tags", TrackID: tid, CreatedAt: int64(i + 1)}
		if i == 0 {
			entry.Details = json.RawMessage(`{"changes":{"TITLE":{"old":["a"],"new":["b"]}}}`)
		}
		if err := s.AddEditLog(ctx, entry); err != nil || entry.ID == 0 {
			t.Fatal(err)
		}
	}
	entries, total, err := s.ListEditLog(ctx, "t1", 0, 10)
	if err != nil || total != 2 || len(entries) != 2 || entries[0].CreatedAt != 3 || string(entries[0].Details) != "{}" ||
		string(entries[1].Details) != `{"changes":{"TITLE":{"old":["a"],"new":["b"]}}}` {
		t.Fatalf("%+v %d %v", entries, total, err)
	}
	if entries, total, _ = s.ListEditLog(ctx, "", 1, 1); total != 3 || len(entries) != 1 || entries[0].CreatedAt != 2 {
		t.Fatalf("%+v %d", entries, total)
	}
}

func TestStats(t *testing.T) {
	s := newStore(t)
	_, ts := sampleLibrary(t, s)
	_ = s.MarkTracksMissing(ctx, []string{ts[6].ID}, true)
	st, err := s.Stats(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if st.Tracks != 6 || st.MissingTracks != 1 || st.Albums != 4 || st.Artists != 4 || st.Genres != 4 ||
		st.TotalDuration != 710 || st.TotalSize != 7010 || len(st.Formats) != 3 || st.Formats[0].Suffix != "flac" || st.Formats[0].Count != 3 {
		t.Fatalf("%+v", st)
	}
}
