package api

import (
	"slices"
	"testing"

	"rainy/internal/model"
	"rainy/internal/recommend"
)

// The mix and the daily mix need a user, validate their input and never return the seeds or
// excluded songs.
func TestNativeRecommend(t *testing.T) {
	e := newNativeEnv(t)
	nativeErrorCode(t, e.do("POST", "/recommend/mix", "", map[string]any{}), 401)
	nativeErrorCode(t, e.do("GET", "/recommend/daily", "", nil), 401)

	seed, excluded := e.id("one"), e.id("umbrella")
	mix := nativeJSON[[]model.Track](t, e.do("POST", "/recommend/mix", e.aliceTok,
		map[string]any{"seeds": []string{seed}, "exclude": []string{excluded}, "limit": 3}), 200)
	if len(mix) == 0 || len(mix) > 3 {
		t.Fatalf("mix of %d songs", len(mix))
	}
	for _, tr := range mix {
		if tr.ID == seed || tr.ID == excluded || tr.Missing {
			t.Fatalf("mix returned %s", tr.Title)
		}
	}
	nativeErrorCode(t, e.do("POST", "/recommend/mix", e.aliceTok, map[string]any{"limit": recommend.MaxMixSize + 1}), 400)
	nativeErrorCode(t, e.do("POST", "/recommend/mix", e.aliceTok, map[string]any{"seeds": make([]string, recommend.MaxSeeds+1)}), 400)

	daily := nativeJSON[recommend.DailyMix](t, e.do("GET", "/recommend/daily?tz=Asia%2FShanghai", e.aliceTok, nil), 200)
	if daily.Date == "" || daily.CreatedAt == 0 || len(daily.Tracks) == 0 {
		t.Fatalf("daily mix %+v", daily)
	}
	again := nativeJSON[recommend.DailyMix](t, e.do("GET", "/recommend/daily?tz=Asia%2FShanghai", e.aliceTok, nil), 200)
	ids := func(ts []model.Track) []string {
		out := make([]string, len(ts))
		for i, tr := range ts {
			out[i] = tr.ID
		}
		return out
	}
	if !slices.Equal(ids(again.Tracks), ids(daily.Tracks)) {
		t.Fatal("the daily mix changed between two requests")
	}
}
