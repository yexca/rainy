package api

import (
	"slices"
	"testing"
	"time"

	"rainy/internal/model"
	"rainy/internal/recommend"
)

// The mix, the daily and weekly mix and the song picks need a user; the mix validates its
// input and never returns the seeds or excluded songs; stored mixes stay the same.
func TestNativeRecommend(t *testing.T) {
	e := newNativeEnv(t)
	nativeErrorCode(t, e.do("POST", "/recommend/mix", "", map[string]any{}), 401)
	nativeErrorCode(t, e.do("GET", "/recommend/daily", "", nil), 401)
	nativeErrorCode(t, e.do("GET", "/recommend/weekly", "", nil), 401)
	nativeErrorCode(t, e.do("GET", "/recommend/picks", "", nil), 401)

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

	weekly := nativeJSON[recommend.WeeklyMix](t, e.do("GET", "/recommend/weekly?tz=Asia%2FShanghai", e.aliceTok, nil), 200)
	monday, err := time.Parse(time.DateOnly, weekly.Week)
	if err != nil || monday.Weekday() != time.Monday || weekly.CreatedAt == 0 || len(weekly.Tracks) == 0 {
		t.Fatalf("weekly mix %+v %v", weekly, err)
	}
	sameWeek := nativeJSON[recommend.WeeklyMix](t, e.do("GET", "/recommend/weekly?tz=Asia%2FShanghai", e.aliceTok, nil), 200)
	if sameWeek.Week != weekly.Week || !slices.Equal(ids(sameWeek.Tracks), ids(weekly.Tracks)) {
		t.Fatal("the weekly mix changed between two requests")
	}

	picks := nativeJSON[recommend.SongPicks](t, e.do("GET", "/recommend/picks", e.aliceTok, nil), 200)
	if len(picks.Quick) == 0 || picks.OnRepeat == nil || picks.Rediscover == nil {
		t.Fatalf("song picks %+v", picks)
	}
}
