package store_test

import (
	"errors"
	"slices"
	"testing"

	"rainy/internal/store"
	"rainy/internal/util"
)

// Similar artists share genres with any of the given artists and are never one of them.
func TestSimilarArtistIDs(t *testing.T) {
	s := newStore(t)
	lib := newLibrary(t, s, t.TempDir())
	upsert(t, s, lib.ID,
		trk{path: "a/1.flac", title: "A1", artist: "Alpha", album: "A", genre: "Rock; Pop"},
		trk{path: "b/1.flac", title: "B1", artist: "Beta", album: "B", genre: "Jazz"},
		trk{path: "c/1.flac", title: "C1", artist: "Gamma", album: "C", genre: "Rock; Pop"},
		trk{path: "d/1.flac", title: "D1", artist: "Delta", album: "D", genre: "Pop"},
		trk{path: "e/1.flac", title: "E1", artist: "Epsilon", album: "E", genre: "Jazz"},
		trk{path: "f/1.flac", title: "F1", artist: "Phi", album: "F", genre: "Folk"},
	)
	alpha, beta := util.ArtistID("Alpha"), util.ArtistID("Beta")

	got, err := s.SimilarArtistIDs(ctx, []string{alpha}, 10)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{util.ArtistID("Gamma"), util.ArtistID("Delta")}; !slices.Equal(got, want) {
		t.Fatalf("similar to Alpha %v, want Gamma (2 genres) then Delta (1)", got)
	}
	got, err = s.SimilarArtistIDs(ctx, []string{alpha, beta, ""}, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 || slices.Contains(got, alpha) || slices.Contains(got, beta) || slices.Contains(got, util.ArtistID("Phi")) {
		t.Fatalf("similar to Alpha and Beta %v", got)
	}
	if got, err := s.SimilarArtistIDs(ctx, nil, 10); err != nil || len(got) != 0 {
		t.Fatalf("no artists: %v %v", got, err)
	}
}

// Candidates carry the user's own signals and honour the filters; missing tracks never show.
func TestRecommendCandidates(t *testing.T) {
	s := newStore(t)
	lib := newLibrary(t, s, t.TempDir())
	ts := upsert(t, s, lib.ID,
		trk{path: "a/1.flac", title: "A1", artist: "Alpha", album: "A", genre: "Rock"},
		trk{path: "a/2.flac", title: "A2", artist: "Alpha", album: "A", genre: "Rock"},
		trk{path: "b/1.flac", title: "B1", artist: "Beta", album: "B", genre: "Jazz"},
		trk{path: "c/1.flac", title: "C1", artist: "Gamma", album: "C", genre: "Folk"},
		trk{path: "c/2.flac", title: "C2", artist: "Gamma", album: "C", genre: "Folk"},
	)
	u, other := listener(t, s, "u"), listener(t, s, "other")
	play(t, s, u.ID, ts[0].ID, 1000, "web")
	play(t, s, u.ID, ts[0].ID, 2000, "web")
	play(t, s, u.ID, ts[2].ID, 9000, "web")
	play(t, s, other.ID, ts[3].ID, 1000, "web")
	if err := s.SetStarred(ctx, u.ID, "track", []string{ts[3].ID}, true); err != nil {
		t.Fatal(err)
	}
	if err := s.SetRating(ctx, u.ID, "track", ts[1].ID, 4); err != nil {
		t.Fatal(err)
	}
	if err := s.MarkTracksMissing(ctx, []string{ts[4].ID}, true); err != nil {
		t.Fatal(err)
	}

	cands := func(q store.CandidateQuery) map[string]store.Candidate {
		t.Helper()
		cs, err := s.RecommendCandidates(ctx, u.ID, q)
		if err != nil {
			t.Fatal(err)
		}
		out := map[string]store.Candidate{}
		for _, c := range cs {
			out[c.ID] = c
		}
		return out
	}
	all := cands(store.CandidateQuery{})
	if len(all) != 4 {
		t.Fatalf("whole library %d candidates, want 4 (the missing track left out)", len(all))
	}
	if c := all[ts[0].ID]; c.Plays != 2 || c.PlayedAt != 2000 || c.Starred || c.ArtistID != util.ArtistID("Alpha") {
		t.Fatalf("A1 signals %+v", c)
	}
	if c := all[ts[3].ID]; !c.Starred || c.Plays != 0 {
		t.Fatalf("C1 signals %+v (the other user's play must not count)", c)
	}
	if all[ts[1].ID].Rating != 4 {
		t.Fatalf("A2 rating %+v", all[ts[1].ID])
	}

	genres, err := s.TrackGenreIDs(ctx, []string{ts[2].ID})
	if err != nil || len(genres) != 1 {
		t.Fatalf("genres of B1 %v %v", genres, err)
	}
	byArtistOrGenre := cands(store.CandidateQuery{ArtistIDs: []string{util.ArtistID("Alpha")}, GenreIDs: genres})
	if len(byArtistOrGenre) != 3 || byArtistOrGenre[ts[3].ID].ID != "" {
		t.Fatalf("Alpha or Jazz: %v", byArtistOrGenre)
	}
	never := cands(store.CandidateQuery{Played: store.PlayedNever})
	if len(never) != 2 || never[ts[1].ID].ID == "" || never[ts[3].ID].ID == "" {
		t.Fatalf("never played: %v", never)
	}
	before := cands(store.CandidateQuery{Played: store.PlayedBefore, Before: 5000})
	if len(before) != 1 || before[ts[0].ID].ID == "" {
		t.Fatalf("played before 5000: %v", before)
	}
	favorites := cands(store.CandidateQuery{Favorites: true})
	if len(favorites) != 2 || favorites[ts[0].ID].ID == "" || favorites[ts[3].ID].ID == "" {
		t.Fatalf("favorites (starred or played twice): %v", favorites)
	}
	if len(cands(store.CandidateQuery{Limit: 2})) != 2 {
		t.Fatal("limit not applied")
	}
	if _, err := s.RecommendCandidates(ctx, u.ID, store.CandidateQuery{Played: "sometimes"}); !errors.Is(err, store.ErrInvalid) {
		t.Fatalf("unknown played filter: %v", err)
	}
}

// The first daily mix of a day wins; old mixes are pruned; users never share mixes.
func TestDailyMixes(t *testing.T) {
	s := newStore(t)
	u, other := listener(t, s, "u"), listener(t, s, "other")
	if _, err := s.GetDailyMix(ctx, u.ID, "2026-10-01"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("no mix yet: %v", err)
	}
	if _, err := s.SaveDailyMix(ctx, u.ID, store.DailyMixRow{Day: "2026-09-01", TrackIDs: []string{"old"}, CreatedAt: 100}, 0); err != nil {
		t.Fatal(err)
	}
	got, err := s.SaveDailyMix(ctx, u.ID, store.DailyMixRow{Day: "2026-10-01", TrackIDs: []string{"a", "b"}, CreatedAt: 5000}, 0)
	if err != nil || !slices.Equal(got.TrackIDs, []string{"a", "b"}) || got.CreatedAt != 5000 {
		t.Fatalf("saved %+v %v", got, err)
	}
	got, err = s.SaveDailyMix(ctx, u.ID, store.DailyMixRow{Day: "2026-10-01", TrackIDs: []string{"c"}, CreatedAt: 6000}, 1000)
	if err != nil || !slices.Equal(got.TrackIDs, []string{"a", "b"}) || got.CreatedAt != 5000 {
		t.Fatalf("second save of the day must keep the first: %+v %v", got, err)
	}
	if _, err := s.GetDailyMix(ctx, u.ID, "2026-09-01"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("old mix not pruned: %v", err)
	}
	if _, err := s.GetDailyMix(ctx, other.ID, "2026-10-01"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("another user's mix: %v", err)
	}
	empty, err := s.SaveDailyMix(ctx, other.ID, store.DailyMixRow{Day: "2026-10-01", CreatedAt: 5000}, 0)
	if err != nil || empty.TrackIDs == nil || len(empty.TrackIDs) != 0 {
		t.Fatalf("empty mix %+v %v", empty, err)
	}
}
