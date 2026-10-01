package recommend

import (
	"context"
	"fmt"
	"slices"
	"testing"
	"time"

	"rainy/internal/db/dbtest"
	"rainy/internal/model"
	"rainy/internal/store"
	"rainy/internal/util"
)

var ctx = context.Background()

type env struct {
	t   *testing.T
	st  *store.Store
	svc *Service
	now time.Time
	u   *model.User
	ids map[string]string // title → track id
}

// newEnv builds a synthetic library: for each artist, songs titled "<artist> <n>" in the
// given genre. The service's clock is fixed and its random part is zero.
func newEnv(t *testing.T, artists map[string]struct {
	genre string
	songs int
}) *env {
	t.Helper()
	st := store.New(dbtest.New(t))
	lib := &model.Library{Name: "Music", Path: t.TempDir()}
	if err := st.CreateLibrary(ctx, lib); err != nil {
		t.Fatal(err)
	}
	e := &env{t: t, st: st, now: time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC), ids: map[string]string{}}
	var tracks []model.Track
	for name, a := range artists {
		for n := 1; n <= a.songs; n++ {
			title := fmt.Sprintf("%s %d", name, n)
			tr := model.Track{ID: util.NewID(), LibraryID: lib.ID, Path: name + "/" + title + ".flac", Mtime: 1000,
				Title: title, Artist: name, Album: name + " Album", AlbumArtist: name,
				AlbumID: util.AlbumID(name, name+" Album"), ArtistID: util.ArtistID(name), AlbumArtistID: util.ArtistID(name),
				TrackNumber: n, Genre: a.genre, Duration: 180}
			tracks = append(tracks, tr)
			e.ids[title] = tr.ID
		}
	}
	if err := st.UpsertTracks(ctx, tracks); err != nil {
		t.Fatal(err)
	}
	if err := st.RefreshAll(ctx); err != nil {
		t.Fatal(err)
	}
	e.u = &model.User{Username: "listener", PasswordEnc: "synthetic"}
	if err := st.CreateUser(ctx, e.u); err != nil {
		t.Fatal(err)
	}
	e.svc = New(st)
	e.svc.now = func() time.Time { return e.now }
	e.svc.rand = func() float64 { return 0 }
	return e
}

func (e *env) play(title string, ago time.Duration) {
	e.t.Helper()
	if err := e.st.RecordPlay(ctx, e.u.ID, e.ids[title], e.now.Add(-ago).UnixMilli(), "web"); err != nil {
		e.t.Fatal(err)
	}
}

func artistsOf(ts []model.Track) map[string]int {
	out := map[string]int{}
	for _, t := range ts {
		out[t.Artist]++
	}
	return out
}

func trackIDs(ts []model.Track) []string {
	out := make([]string, len(ts))
	for i, t := range ts {
		out[i] = t.ID
	}
	return out
}

var library = map[string]struct {
	genre string
	songs int
}{
	"Alpha": {"Rock", 6}, "Beta": {"Rock", 4}, "Gamma": {"Rock", 4},
	"Delta": {"Jazz", 4}, "Epsilon": {"Jazz", 4}, "Zeta": {"Folk", 4},
}

// A mix follows the seeds: the seed artist and artists sharing its genres come first, at most
// two songs per artist, never a seed, an excluded song or a song rated one star.
func TestMixFollowsSeeds(t *testing.T) {
	e := newEnv(t, library)
	if err := e.st.SetRating(ctx, e.u.ID, "track", e.ids["Alpha 2"], 1); err != nil {
		t.Fatal(err)
	}
	got, err := e.svc.Mix(ctx, e.u.ID, MixQuery{Seeds: []string{e.ids["Alpha 1"]}, Exclude: []string{e.ids["Alpha 3"]}, Limit: 6})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 6 {
		t.Fatalf("got %d songs, want 6", len(got))
	}
	for _, tr := range got {
		if tr.ID == e.ids["Alpha 1"] || tr.ID == e.ids["Alpha 2"] || tr.ID == e.ids["Alpha 3"] {
			t.Fatalf("picked a seed, an excluded or a one-star song: %s", tr.Title)
		}
	}
	by := artistsOf(got)
	if by["Alpha"] != 2 || by["Beta"] != 2 || by["Gamma"] != 2 {
		t.Fatalf("want two each of Alpha, Beta, Gamma (same artist, then same genre), got %v", by)
	}
	for i := 1; i < len(got); i++ {
		if got[i].Artist == got[i-1].Artist {
			t.Fatalf("same artist twice in a row: %v", trackTitles(got))
		}
	}
}

// spread separates songs by the same artist even when they all come last: with nothing
// later to swap in, a trailing repeat moves back between two other artists (a regression:
// "A B A B G G" used to stay as it was).
func TestSpreadMovesTrailingRepeatsBack(t *testing.T) {
	p := &picker{cands: map[string]*scored{}}
	for id, artist := range map[string]string{"a1": "A", "a2": "A", "b1": "B", "b2": "B", "g1": "G", "g2": "G", "g3": "G"} {
		p.cands[id] = &scored{Candidate: store.Candidate{ArtistID: artist}}
	}
	for _, in := range [][]string{
		{"a1", "b1", "a2", "b2", "g1", "g2"},
		{"g1", "g2", "a1", "b1"},
		{"a1", "b1", "a2", "g1", "g2", "g3"},
	} {
		got := p.spread(slices.Clone(in))
		if len(got) != len(in) {
			t.Fatalf("spread(%v) = %v: lost songs", in, got)
		}
		for i := 1; i < len(got); i++ {
			if p.cands[got[i]].artist() == p.cands[got[i-1]].artist() {
				t.Fatalf("spread(%v) = %v: same artist twice in a row", in, got)
			}
		}
	}
	// One artist only: nothing can go in between, and nothing is lost.
	if got := p.spread([]string{"g1", "g2", "g3"}); len(got) != 3 {
		t.Fatalf("spread of one artist = %v", got)
	}
}

// A small library still fills the mix by lifting the per-artist cap; without seeds or plays
// a mix is random songs.
func TestMixFallbacks(t *testing.T) {
	e := newEnv(t, map[string]struct {
		genre string
		songs int
	}{"Solo": {"Rock", 5}})
	got, err := e.svc.Mix(ctx, e.u.ID, MixQuery{Seeds: []string{e.ids["Solo 1"]}, Limit: 10})
	if err != nil || len(got) != 4 {
		t.Fatalf("one-artist library: %d songs, err %v; want the 4 other songs", len(got), err)
	}
	got, err = e.svc.Mix(ctx, e.u.ID, MixQuery{Limit: 3})
	if err != nil || len(got) != 3 {
		t.Fatalf("no seeds: %d songs, err %v", len(got), err)
	}
	if _, err := e.svc.Mix(ctx, e.u.ID, MixQuery{Limit: MaxMixSize + 1}); err == nil {
		t.Fatal("limit above the maximum accepted")
	}
	if _, err := e.svc.Mix(ctx, e.u.ID, MixQuery{Seeds: make([]string, MaxSeeds+1)}); err == nil {
		t.Fatal("too many seeds accepted")
	}
}

// Songs played in the last day go to the back of a mix.
func TestMixAvoidsRecentPlays(t *testing.T) {
	e := newEnv(t, library)
	e.play("Beta 1", time.Hour)
	e.play("Beta 2", time.Hour)
	got, err := e.svc.Mix(ctx, e.u.ID, MixQuery{Seeds: []string{e.ids["Alpha 1"]}, Limit: 6})
	if err != nil {
		t.Fatal(err)
	}
	for _, tr := range got {
		if tr.ID == e.ids["Beta 1"] || tr.ID == e.ids["Beta 2"] {
			t.Fatalf("picked %s, played an hour ago, while other songs were available", tr.Title)
		}
	}
}

// The daily mix stays the same all day, changes the next day, leaves out removed songs and
// mixes favorites to rediscover with songs never played.
func TestDailyMix(t *testing.T) {
	big := map[string]struct {
		genre string
		songs int
	}{}
	for name, a := range library {
		big[name] = struct {
			genre string
			songs int
		}{a.genre, 8} // 48 songs: more than a mix holds
	}
	e := newEnv(t, big)
	day := 24 * time.Hour
	for range 3 {
		e.play("Alpha 1", 60*day) // a favorite not heard for two months
	}
	e.play("Alpha 2", 2*day)
	e.play("Delta 1", 2*day)

	first, err := e.svc.Daily(ctx, e.u.ID, time.UTC)
	if err != nil {
		t.Fatal(err)
	}
	if first.Date != "2026-10-01" || len(first.Tracks) != DailySize {
		t.Fatalf("first mix %s with %d songs", first.Date, len(first.Tracks))
	}
	ids := trackIDs(first.Tracks)
	if !slices.Contains(ids, e.ids["Alpha 1"]) {
		t.Fatal("the favorite to rediscover is missing")
	}
	if slices.Contains(ids, e.ids["Alpha 2"]) || slices.Contains(ids, e.ids["Delta 1"]) {
		t.Fatal("a song played two days ago is in the mix")
	}
	if by := artistsOf(first.Tracks); len(by) != 6 {
		t.Fatalf("artists %v", by)
	}

	e.play("Gamma 1", time.Hour)
	if err := e.st.MarkTracksMissing(ctx, []string{first.Tracks[0].ID}, true); err != nil {
		t.Fatal(err)
	}
	e.now = e.now.Add(6 * time.Hour)
	again, err := e.svc.Daily(ctx, e.u.ID, time.UTC)
	if err != nil {
		t.Fatal(err)
	}
	if again.CreatedAt != first.CreatedAt || !slices.Equal(trackIDs(again.Tracks), trackIDs(first.Tracks)[1:]) {
		t.Fatal("the mix changed during the day (beyond leaving out the removed song)")
	}

	// Asia/Tokyo is already on the next day.
	tokyo := time.FixedZone("UTC+9", 9*60*60)
	next, err := e.svc.Daily(ctx, e.u.ID, tokyo)
	if err != nil || next.Date != "2026-10-02" || next.CreatedAt == first.CreatedAt {
		t.Fatalf("next day's mix %+v %v", next, err)
	}
}

// A new user with no plays still gets a mix; an empty library gives an empty one.
func TestDailyMixColdStart(t *testing.T) {
	e := newEnv(t, library)
	mix, err := e.svc.Daily(ctx, e.u.ID, nil)
	if err != nil || len(mix.Tracks) != 26 { // the whole library
		t.Fatalf("new user: %d songs, err %v", len(mix.Tracks), err)
	}
	if by := artistsOf(mix.Tracks); len(by) != 6 {
		t.Fatalf("artists %v", by)
	}

	empty := newEnv(t, nil)
	mix, err = empty.svc.Daily(ctx, empty.u.ID, nil)
	if err != nil || mix.Tracks == nil || len(mix.Tracks) != 0 {
		t.Fatalf("empty library: %+v %v", mix, err)
	}
}

func trackTitles(ts []model.Track) []string {
	out := make([]string, len(ts))
	for i, t := range ts {
		out[i] = t.Title
	}
	return out
}
