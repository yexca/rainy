package store_test

import (
	"errors"
	"reflect"
	"testing"

	"rainy/internal/model"
	"rainy/internal/store"
	"rainy/internal/util"
)

func listener(t *testing.T, s *store.Store, name string) *model.User {
	t.Helper()
	u := &model.User{Username: name, PasswordEnc: "synthetic"}
	if err := s.CreateUser(ctx, u); err != nil {
		t.Fatal(err)
	}
	return u
}

func play(t *testing.T, s *store.Store, userID, trackID string, at int64, client string) {
	t.Helper()
	if err := s.RecordPlay(ctx, userID, trackID, at, client); err != nil {
		t.Fatal(err)
	}
}

// The listening queries must keep reporting plays of tracks that were purged, from the
// snapshot taken at play time, and must never mix users.
func TestListeningQueries(t *testing.T) {
	s := newStore(t)
	lib := newLibrary(t, s, t.TempDir())
	ts := upsert(t, s, lib.ID,
		trk{path: "a/1.flac", title: "One", artist: "Alpha", album: "First", genre: "Rock", duration: 200},
		trk{path: "a/2.flac", title: "Two", artist: "Alpha", album: "First", genre: "Rock", duration: 100},
		trk{path: "b/1.flac", title: "Three", artist: "Beta", album: "Second", genre: "Jazz", duration: 300},
	)
	u, other := listener(t, s, "u"), listener(t, s, "other")

	play(t, s, u.ID, ts[0].ID, 1000, "web")
	play(t, s, u.ID, ts[0].ID, 2000, "web")
	play(t, s, u.ID, ts[1].ID, 3000, "Symfonium")
	play(t, s, u.ID, ts[2].ID, 10_000, "web")
	play(t, s, other.ID, ts[2].ID, 1500, "web")

	// Purge "Three": its play stays with the snapshot.
	if err := s.DeleteTracks(ctx, []string{ts[2].ID}); err != nil {
		t.Fatal(err)
	}

	tot, err := s.ListeningTotals(ctx, u.ID, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	want := store.ListeningTotals{Plays: 4, Duration: 800, Tracks: 3, Artists: 2, Albums: 2}
	if tot != want {
		t.Fatalf("totals %+v, want %+v", tot, want)
	}
	if tot, _ := s.ListeningTotals(ctx, u.ID, 1500, 5000); tot.Plays != 2 || tot.Duration != 300 || tot.Tracks != 2 {
		t.Fatalf("range totals %+v", tot)
	}

	tracks, err := s.ListeningTop(ctx, u.ID, store.TopTracks, 0, 0, 10)
	if err != nil {
		t.Fatal(err)
	}
	if got := ids(tracks, func(it store.ListeningTopItem) string { return it.Name }); !reflect.DeepEqual(got, []string{"One", "Three", "Two"}) {
		t.Fatalf("top tracks %v", got)
	}
	if tracks[1].Artist != "Beta" || tracks[1].Duration != 300 {
		t.Fatalf("purged track snapshot %+v", tracks[1])
	}
	artists, _ := s.ListeningTop(ctx, u.ID, store.TopArtists, 0, 0, 10)
	if len(artists) != 2 || artists[0].ID != util.ArtistID("Alpha") || artists[0].Plays != 3 || artists[1].Name != "Beta" {
		t.Fatalf("top artists %+v", artists)
	}
	genres, _ := s.ListeningTop(ctx, u.ID, store.TopGenres, 0, 0, 10)
	if len(genres) != 1 || genres[0].Name != "Rock" || genres[0].Plays != 3 {
		t.Fatalf("top genres %+v (the purged track lost its genre)", genres)
	}
	clients, _ := s.ListeningTop(ctx, u.ID, store.TopClients, 0, 0, 10)
	if len(clients) != 2 || clients[0].Name != "web" || clients[0].Plays != 3 {
		t.Fatalf("clients %+v", clients)
	}
	if _, err := s.ListeningTop(ctx, u.ID, "nope", 0, 0, 10); !errors.Is(err, store.ErrInvalid) {
		t.Fatalf("unknown kind: %v", err)
	}

	// First plays: "Two" (3000) and "Three" + artist Beta (10000) are new after 2500.
	newTracks, newArtists, err := s.ListeningFirsts(ctx, u.ID, 2500, 0)
	if err != nil || newTracks != 2 || newArtists != 1 {
		t.Fatalf("firsts %d %d %v", newTracks, newArtists, err)
	}
	if at, _ := s.FirstPlayAt(ctx, u.ID); at != 1000 {
		t.Fatalf("first play %d", at)
	}

	times, _ := s.ListeningTimes(ctx, u.ID, 0, 0)
	if len(times) != 4 || times[0].At != 1000 || times[3].Duration != 300 {
		t.Fatalf("times %+v", times)
	}

	plays, total, err := s.ListPlays(ctx, u.ID, 0, 0, 0, 3)
	if err != nil || total != 4 || len(plays) != 3 {
		t.Fatalf("plays %d/%d %v", len(plays), total, err)
	}
	if plays[0].Title != "Three" || plays[0].Track != nil || plays[0].Client != "web" {
		t.Fatalf("purged play %+v", plays[0])
	}
	if plays[1].Track == nil || plays[1].Track.ID != ts[1].ID || plays[1].Track.PlayCount != 1 || plays[1].Client != "Symfonium" {
		t.Fatalf("live play %+v", plays[1])
	}
	if other, _, _ := s.ListPlays(ctx, other.ID, 0, 0, 0, 10); len(other) != 1 {
		t.Fatalf("other user's history %+v", other)
	}
}

func TestScrobbleQueue(t *testing.T) {
	s := newStore(t)
	u := listener(t, s, "u")
	e := model.QueuedScrobble{UserID: u.ID, TrackID: "t1", Title: "One", Artist: "Alpha", PlayedAt: 1000}

	// Nothing is linked yet.
	if got, err := s.EnqueueScrobble(ctx, e, []string{model.ScrobbleLastfm}); err != nil || len(got) != 0 {
		t.Fatalf("queued without an account: %v %v", got, err)
	}
	for _, svc := range []string{model.ScrobbleLastfm, model.ScrobbleListenBrainz} {
		if err := s.SaveScrobbleAccount(ctx, &model.ScrobbleAccount{UserID: u.ID, Service: svc, Username: "me", CredentialEnc: "enc"}); err != nil {
			t.Fatal(err)
		}
	}
	got, err := s.EnqueueScrobble(ctx, e, []string{model.ScrobbleLastfm, model.ScrobbleListenBrainz})
	if err != nil || !reflect.DeepEqual(got, []string{model.ScrobbleLastfm, model.ScrobbleListenBrainz}) {
		t.Fatalf("queued %v %v", got, err)
	}
	// Only the services asked for.
	if got, _ := s.EnqueueScrobble(ctx, model.QueuedScrobble{UserID: u.ID, TrackID: "t2", Title: "Two", Artist: "A", PlayedAt: 2000},
		[]string{model.ScrobbleListenBrainz}); !reflect.DeepEqual(got, []string{model.ScrobbleListenBrainz}) {
		t.Fatalf("queued %v", got)
	}

	users, _ := s.ScrobbleDueUsers(ctx, model.ScrobbleListenBrainz, 5000)
	due, _ := s.DueScrobbles(ctx, u.ID, model.ScrobbleListenBrainz, 5000, 10)
	if !reflect.DeepEqual(users, []string{u.ID}) || len(due) != 2 || due[0].Title != "One" {
		t.Fatalf("due %v %+v", users, due)
	}
	if err := s.DeferScrobbles(ctx, []int64{due[0].ID}, 9000, "offline"); err != nil {
		t.Fatal(err)
	}
	if due, _ := s.DueScrobbles(ctx, u.ID, model.ScrobbleListenBrainz, 5000, 10); len(due) != 1 || due[0].Title != "Two" {
		t.Fatalf("deferred play still due: %+v", due)
	}
	if due, _ := s.DueScrobbles(ctx, u.ID, model.ScrobbleListenBrainz, 9000, 10); len(due) != 2 || due[0].Attempts != 1 || due[0].LastError != "offline" {
		t.Fatalf("after the backoff %+v", due)
	}
	if n, _ := s.PurgeScrobblesBefore(ctx, model.ScrobbleListenBrainz, 1500); n != 1 {
		t.Fatalf("purged %d", n)
	}

	// Pausing drops the account's queue; the other service keeps its own.
	if err := s.SetScrobbleAccountEnabled(ctx, u.ID, model.ScrobbleListenBrainz, false); err != nil {
		t.Fatal(err)
	}
	if n, _ := s.CountQueuedScrobbles(ctx, u.ID, model.ScrobbleListenBrainz); n != 0 {
		t.Fatalf("paused account kept %d plays", n)
	}
	if got, _ := s.EnqueueScrobble(ctx, e, []string{model.ScrobbleListenBrainz}); len(got) != 0 {
		t.Fatal("queued for a paused account")
	}
	if n, _ := s.CountQueuedScrobbles(ctx, u.ID, model.ScrobbleLastfm); n != 1 {
		t.Fatalf("lastfm queue %d", n)
	}

	// A revoked credential stops queueing until the account is linked again.
	if err := s.SetScrobbleAccountError(ctx, u.ID, model.ScrobbleLastfm, "revoked", true); err != nil {
		t.Fatal(err)
	}
	a, _ := s.GetScrobbleAccount(ctx, u.ID, model.ScrobbleLastfm)
	if a.CredentialEnc != "" || a.LastError != "revoked" || a.LastErrorAt == 0 {
		t.Fatalf("revoked account %+v", a)
	}
	if n, _ := s.CountScrobbleAccounts(ctx, model.ScrobbleLastfm); n != 0 {
		t.Fatalf("revoked account counted: %d", n)
	}
	if err := s.DeleteScrobbleAccount(ctx, u.ID, model.ScrobbleLastfm); err != nil {
		t.Fatal(err)
	}
	if n, _ := s.CountQueuedScrobbles(ctx, u.ID, model.ScrobbleLastfm); n != 0 {
		t.Fatalf("unlinked account kept %d plays", n)
	}
	if _, err := s.GetScrobbleAccount(ctx, u.ID, model.ScrobbleLastfm); !errors.Is(err, store.ErrNotFound) {
		t.Fatal(err)
	}
	if err := s.SetScrobbleAccountEnabled(ctx, u.ID, model.ScrobbleLastfm, true); !errors.Is(err, store.ErrNotFound) {
		t.Fatal(err)
	}
}

// Values stored beside the settings never appear in model.Settings.
func TestStoredValues(t *testing.T) {
	s := newStore(t)
	if v, err := s.GetValue(ctx, store.ValueLastfmSigningEnc); err != nil || v != "" {
		t.Fatalf("unset value %q %v", v, err)
	}
	if err := s.SetValue(ctx, store.ValueLastfmSigningEnc, "sealed"); err != nil {
		t.Fatal(err)
	}
	if v, _ := s.GetValue(ctx, store.ValueLastfmSigningEnc); v != "sealed" {
		t.Fatalf("value %q", v)
	}
	defaults := model.DefaultSettings(0)
	if got, err := s.GetSettings(ctx, defaults); err != nil || got != defaults {
		t.Fatalf("settings changed by a stored value: %+v %v", got, err)
	}
	if err := s.SetValue(ctx, store.ValueLastfmSigningEnc, ""); err != nil {
		t.Fatal(err)
	}
	if v, _ := s.GetValue(ctx, store.ValueLastfmSigningEnc); v != "" {
		t.Fatalf("deleted value %q", v)
	}
}
