package api

import (
	"testing"
	"time"

	"rainy/internal/events"
	"rainy/internal/model"
	"rainy/internal/nowplaying"
)

func TestNativeStarAndRating(t *testing.T) {
	e := newNativeEnv(t)
	nativeErrorCode(t, e.do("POST", "/star", "", map[string]any{"type": "track", "ids": []string{"x"}, "starred": true}), 401)
	nativeErrorCode(t, e.do("POST", "/star", e.aliceTok, map[string]any{"type": "song", "ids": []string{"x"}, "starred": true}), 400)
	nativeErrorCode(t, e.do("POST", "/star", e.aliceTok, "[]"), 400)

	nativeExpect(t, e.do("POST", "/star", e.aliceTok, map[string]any{"type": "track", "ids": []string{e.id("one"), " ", e.id("tone")}, "starred": true}), 204)
	tr := nativeJSON[model.Track](t, e.do("GET", "/tracks/"+e.id("one"), e.aliceTok, nil), 200)
	if !tr.Starred || tr.StarredAt == nil {
		t.Fatalf("not starred: %+v", tr)
	}
	if tr = nativeJSON[model.Track](t, e.do("GET", "/tracks/"+e.id("one"), e.bobTok, nil), 200); tr.Starred {
		t.Fatal("stars are per user")
	}
	nativeExpect(t, e.do("POST", "/star", e.aliceTok, map[string]any{"type": "album", "ids": []string{e.blueSkies}, "starred": true}), 204)
	nativeExpect(t, e.do("POST", "/star", e.aliceTok, map[string]any{"type": "artist", "ids": []string{e.sineWave}, "starred": true}), 204)
	nativeExpect(t, e.do("POST", "/star", e.aliceTok, map[string]any{"type": "track", "ids": []string{e.id("one")}, "starred": false}), 204)
	if tr = nativeJSON[model.Track](t, e.do("GET", "/tracks/"+e.id("one"), e.aliceTok, nil), 200); tr.Starred {
		t.Fatal("still starred")
	}
	if al := nativeJSON[model.Album](t, e.do("GET", "/albums/"+e.blueSkies, e.aliceTok, nil), 200); !al.Starred {
		t.Fatal("album not starred")
	}
	// Empty id list is a no-op.
	nativeExpect(t, e.do("POST", "/star", e.aliceTok, map[string]any{"type": "track", "ids": []string{}, "starred": true}), 204)
	ids := make([]string, maxIDsPerRequest+1)
	for i := range ids {
		ids[i] = "x"
	}
	nativeErrorCode(t, e.do("POST", "/star", e.aliceTok, map[string]any{"type": "track", "ids": ids, "starred": true}), 400)

	// Ratings.
	nativeExpect(t, e.do("POST", "/rating", e.aliceTok, map[string]any{"type": "track", "id": e.id("tone"), "rating": 4}), 204)
	if tr = nativeJSON[model.Track](t, e.do("GET", "/tracks/"+e.id("tone"), e.aliceTok, nil), 200); tr.Rating != 4 {
		t.Fatalf("rating %d", tr.Rating)
	}
	nativeExpect(t, e.do("POST", "/rating", e.aliceTok, map[string]any{"type": "track", "id": e.id("tone"), "rating": 0}), 204)
	if tr = nativeJSON[model.Track](t, e.do("GET", "/tracks/"+e.id("tone"), e.aliceTok, nil), 200); tr.Rating != 0 {
		t.Fatal("rating not cleared")
	}
	nativeErrorCode(t, e.do("POST", "/rating", e.aliceTok, map[string]any{"type": "track", "id": e.id("tone"), "rating": 6}), 400)
	nativeErrorCode(t, e.do("POST", "/rating", e.aliceTok, map[string]any{"type": "track", "id": e.id("tone"), "rating": -1}), 400)
	nativeErrorCode(t, e.do("POST", "/rating", e.aliceTok, map[string]any{"type": "genre", "id": "x", "rating": 1}), 400)
	nativeErrorCode(t, e.do("POST", "/rating", e.aliceTok, map[string]any{"type": "album", "rating": 1}), 400)
}

func TestNativeScrobble(t *testing.T) {
	e := newNativeEnv(t)
	ch, cancel := e.app.Bus.Subscribe()
	defer cancel()

	nativeErrorCode(t, e.do("POST", "/scrobble", e.aliceTok, map[string]any{"submission": false}), 400)
	nativeErrorCode(t, e.do("POST", "/scrobble", e.aliceTok, map[string]any{"trackId": "nope", "submission": false}), 404)
	nativeErrorCode(t, e.do("POST", "/scrobble", e.aliceTok, map[string]any{"trackId": "nope", "submission": true}), 404)

	// Now playing.
	nativeExpect(t, e.do("POST", "/scrobble", e.aliceTok, map[string]any{"trackId": e.id("tone"), "submission": false}), 204)
	list := e.app.NowPlaying.List()
	if len(list) != 1 || list[0].TrackID != e.id("tone") || list[0].Username != "alice" || list[0].Player != "web" {
		t.Fatalf("now playing %+v", list)
	}
	select {
	case ev := <-ch:
		entries, ok := ev.Data.([]nowplaying.Entry)
		if ev.Type != events.TypeNowPlaying || !ok || len(entries) != 1 {
			t.Fatalf("event %+v", ev)
		}
	case <-time.After(time.Second):
		t.Fatal("no nowPlaying event")
	}
	if tr := nativeJSON[model.Track](t, e.do("GET", "/tracks/"+e.id("tone"), e.aliceTok, nil), 200); tr.PlayCount != 0 {
		t.Fatal("now playing must not count as a play")
	}
	// A second player of the same user is listed separately.
	nativeExpect(t, e.do("POST", "/scrobble", e.aliceTok, map[string]any{"trackId": e.id("one"), "player": "phone"}), 204)
	if n := len(e.app.NowPlaying.List()); n != 2 {
		t.Fatalf("players %d", n)
	}

	// Submissions.
	at := time.Now().Add(-time.Hour).UnixMilli()
	nativeExpect(t, e.do("POST", "/scrobble", e.aliceTok, map[string]any{"trackId": e.id("tone"), "submission": true, "time": at}), 204)
	nativeExpect(t, e.do("POST", "/scrobble", e.aliceTok, map[string]any{"trackId": e.id("tone"), "submission": true}), 204)
	tr := nativeJSON[model.Track](t, e.do("GET", "/tracks/"+e.id("tone"), e.aliceTok, nil), 200)
	if tr.PlayCount != 2 || tr.PlayedAt <= at {
		t.Fatalf("plays %d at %d", tr.PlayCount, tr.PlayedAt)
	}
	// Future timestamps are clamped to now.
	future := time.Now().Add(24 * time.Hour).UnixMilli()
	nativeExpect(t, e.do("POST", "/scrobble", e.aliceTok, map[string]any{"trackId": e.id("one"), "submission": true, "time": future}), 204)
	if tr = nativeJSON[model.Track](t, e.do("GET", "/tracks/"+e.id("one"), e.aliceTok, nil), 200); tr.PlayedAt >= future || tr.PlayCount != 1 {
		t.Fatalf("future play %+v", tr.PlayedAt)
	}
	if al := nativeJSON[model.Album](t, e.do("GET", "/albums/"+e.tones, e.aliceTok, nil), 200); al.PlayCount != 2 {
		t.Fatalf("album plays %d", al.PlayCount)
	}
}
