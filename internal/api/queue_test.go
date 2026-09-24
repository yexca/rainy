package api

import (
	"slices"
	"strings"
	"testing"
)

func TestNativeQueue(t *testing.T) {
	e := newNativeEnv(t)
	nativeErrorCode(t, e.do("GET", "/queue", "", nil), 401)

	q := nativeJSON[queueResponse](t, e.do("GET", "/queue", e.aliceTok, nil), 200)
	if q.TrackIDs == nil || q.Tracks == nil || len(q.TrackIDs) != 0 || q.CurrentID != "" {
		t.Fatalf("empty queue %+v", q)
	}
	body := e.do("GET", "/queue", e.aliceTok, nil).Body.String()
	for _, k := range []string{`"trackIds":[]`, `"currentId":""`, `"positionMs":0`, `"updatedAt":0`, `"tracks":[]`} {
		if !strings.Contains(body, k) {
			t.Errorf("empty queue JSON %s lacks %s", body, k)
		}
	}

	ids := []string{e.id("tone"), e.id("gone"), "deleted", e.id("one"), ""}
	nativeExpect(t, e.do("PUT", "/queue", e.aliceTok, map[string]any{"trackIds": ids, "currentId": e.id("one"), "positionMs": 12345}), 204)
	q = nativeJSON[queueResponse](t, e.do("GET", "/queue", e.aliceTok, nil), 200)
	if !slices.Equal(q.TrackIDs, ids[:4]) || q.CurrentID != e.id("one") || q.PositionMs != 12345 || q.UpdatedAt == 0 || q.ChangedBy != "web" {
		t.Fatalf("queue %+v", q.PlayQueue)
	}
	// Missing and unknown tracks are not returned as playable tracks.
	if got := nativeTrackIDs(q.Tracks); !slices.Equal(got, []string{e.id("tone"), e.id("one")}) {
		t.Fatalf("queue tracks %v", got)
	}
	// Queues are per user.
	if q = nativeJSON[queueResponse](t, e.do("GET", "/queue", e.bobTok, nil), 200); len(q.TrackIDs) != 0 {
		t.Fatal("queue leaked")
	}
	nativeExpect(t, e.do("PUT", "/queue", e.aliceTok, map[string]any{"trackIds": []string{}, "positionMs": -5}), 204)
	if q = nativeJSON[queueResponse](t, e.do("GET", "/queue", e.aliceTok, nil), 200); len(q.TrackIDs) != 0 || q.PositionMs != 0 {
		t.Fatalf("cleared %+v", q)
	}
	nativeErrorCode(t, e.do("PUT", "/queue", e.aliceTok, `{"trackIds": "nope"}`), 400)
	big := make([]string, maxQueueTracks+1)
	nativeErrorCode(t, e.do("PUT", "/queue", e.aliceTok, map[string]any{"trackIds": big}), 400)
}

// currentIndex (optional) picks which copy of a song queued twice is current; it is kept
// for Subsonic's getPlayQueueByIndex. An index that does not point at currentId is ignored.
func TestNativeQueueCurrentIndex(t *testing.T) {
	e := newNativeEnv(t)
	one, tone := e.id("one"), e.id("tone")
	put := func(idx any) int {
		t.Helper()
		nativeExpect(t, e.do("PUT", "/queue", e.aliceTok, map[string]any{
			"trackIds": []string{one, "", tone, one}, "currentId": one, "positionMs": 1, "currentIndex": idx}), 204)
		q, err := e.app.Store.GetPlayQueue(t.Context(), e.alice.ID)
		if err != nil {
			t.Fatal(err)
		}
		return q.CurrentIndex
	}
	if got := put(3); got != 2 { // the empty id is dropped: index 3 → 2
		t.Fatalf("currentIndex 3 stored as %d, want 2", got)
	}
	if got := put(2); got != 0 { // points at tone: first occurrence of currentId
		t.Fatalf("mismatched index stored as %d, want 0", got)
	}
	if got := put(nil); got != 0 {
		t.Fatalf("no index stored as %d, want 0", got)
	}
}
