package listening

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"rainy/internal/db/dbtest"
	"rainy/internal/model"
	"rainy/internal/store"
	"rainy/internal/util"
)

// A fixed UTC+8 zone keeps the tests independent of the host's zoneinfo.
var plus8 = time.FixedZone("UTC+8", 8*3600)

func ms(t time.Time) int64 { return t.UnixMilli() }

func TestBucketFor(t *testing.T) {
	day := int64(24 * time.Hour / time.Millisecond)
	cases := []struct {
		span int64
		want string
	}{
		{day, BucketHour}, {3 * day, BucketHour}, {3*day + 1, BucketDay}, {93 * day, BucketDay},
		{94 * day, BucketWeek}, {731 * day, BucketWeek}, {732 * day, BucketMonth},
	}
	for _, c := range cases {
		if got := BucketFor(0, c.span); got != c.want {
			t.Errorf("span %d days: %s, want %s", c.span/day, got, c.want)
		}
	}
}

// Buckets follow the viewer's calendar: a play at 23:30 UTC on Sunday is Monday morning in
// UTC+8, so it lands in Monday's day and in the week starting that Monday.
func TestTimelineUsesLocalCalendar(t *testing.T) {
	sunLate := time.Date(2026, 9, 6, 23, 30, 0, 0, time.UTC) // Monday 07:30 in UTC+8
	times := []store.PlayTime{
		{At: ms(time.Date(2026, 9, 5, 12, 0, 0, 0, plus8)), Duration: 100},
		{At: ms(sunLate), Duration: 200},
		{At: ms(time.Date(2026, 9, 7, 22, 0, 0, 0, plus8)), Duration: 50},
	}
	from := ms(time.Date(2026, 9, 5, 0, 0, 0, 0, plus8))
	to := ms(time.Date(2026, 9, 8, 0, 0, 0, 0, plus8))

	days := Timeline(times, from, to, BucketDay, plus8)
	if len(days) != 3 {
		t.Fatalf("%d day buckets", len(days))
	}
	got := []int{days[0].Plays, days[1].Plays, days[2].Plays}
	if !reflect.DeepEqual(got, []int{1, 0, 2}) || days[2].Duration != 250 {
		t.Fatalf("days %+v", days)
	}
	if days[2].Start != ms(time.Date(2026, 9, 7, 0, 0, 0, 0, plus8)) {
		t.Fatalf("day start %d", days[2].Start)
	}

	weeks := Timeline(times, from, to, BucketWeek, plus8)
	if len(weeks) != 2 || weeks[0].Plays != 1 || weeks[1].Plays != 2 ||
		weeks[0].Start != ms(time.Date(2026, 8, 31, 0, 0, 0, 0, plus8)) {
		t.Fatalf("weeks %+v", weeks)
	}

	months := Timeline(times, ms(time.Date(2026, 1, 15, 0, 0, 0, 0, plus8)), to, BucketMonth, plus8)
	if len(months) != 9 || months[8].Plays != 3 || months[0].Start != ms(time.Date(2026, 1, 1, 0, 0, 0, 0, plus8)) {
		t.Fatalf("months %d %+v", len(months), months[len(months)-1])
	}

	clock := Clock(times, plus8)
	if clock[0][7] != 1 || clock[0][22] != 1 || clock[5][12] != 1 {
		t.Fatalf("clock monday %v saturday %v", clock[0], clock[5])
	}

	active, longest := ActiveDays(times, plus8)
	if active != 2 || longest != 1 {
		t.Fatalf("active %d longest %d", active, longest)
	}
	times = append(times, store.PlayTime{At: ms(time.Date(2026, 9, 6, 9, 0, 0, 0, plus8))})
	if active, longest := ActiveDays(times, plus8); active != 3 || longest != 3 {
		t.Fatalf("active %d longest %d", active, longest)
	}
}

func TestLoadLocation(t *testing.T) {
	for _, name := range []string{"", "Not/AZone", "../../etc/passwd", "UTC"} {
		if got := LoadLocation(name); got != time.UTC {
			t.Errorf("%q → %v", name, got)
		}
	}
}

func TestReport(t *testing.T) {
	ctx := context.Background()
	st := store.New(dbtest.New(t))
	lib := &model.Library{Name: "Music", Path: t.TempDir()}
	if err := st.CreateLibrary(ctx, lib); err != nil {
		t.Fatal(err)
	}
	mk := func(title, artist string, dur float64) model.Track {
		return model.Track{ID: util.NewID(), LibraryID: lib.ID, Path: title + ".flac", Title: title, Artist: artist,
			Album: "Album", AlbumArtist: artist, AlbumID: util.AlbumID(artist, "Album"), ArtistID: util.ArtistID(artist),
			AlbumArtistID: util.ArtistID(artist), Duration: dur}
	}
	ts := []model.Track{mk("One", "Alpha", 100), mk("Two", "Beta", 200)}
	if err := st.UpsertTracks(ctx, ts); err != nil {
		t.Fatal(err)
	}
	if err := st.RefreshAll(ctx); err != nil {
		t.Fatal(err)
	}
	u := &model.User{Username: "u", PasswordEnc: "synthetic"}
	if err := st.CreateUser(ctx, u); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 9, 30, 12, 0, 0, 0, plus8)
	day := 24 * time.Hour
	for _, p := range []struct {
		track int
		at    time.Time
	}{{0, now.Add(-40 * day)}, {0, now.Add(-2 * day)}, {1, now.Add(-2 * day)}, {0, now.Add(-time.Hour)}} {
		if err := st.RecordPlay(ctx, u.ID, ts[p.track].ID, ms(p.at), "web"); err != nil {
			t.Fatal(err)
		}
	}
	s := New(st)
	s.now = func() time.Time { return now }

	week, err := s.Report(ctx, u.ID, Query{From: ms(now.Add(-7 * day)), Loc: plus8})
	if err != nil {
		t.Fatal(err)
	}
	if week.Totals.Plays != 3 || week.Totals.Duration != 400 || week.Previous == nil || week.Previous.Plays != 0 {
		t.Fatalf("week totals %+v previous %+v", week.Totals, week.Previous)
	}
	if week.To != ms(now)+1 || week.Bucket != BucketDay || len(week.Timeline) != 8 || week.TZ != "UTC+8" {
		t.Fatalf("week range to=%d bucket=%s timeline=%d tz=%s", week.To, week.Bucket, len(week.Timeline), week.TZ)
	}
	if week.NewTracks != 1 || week.NewArtists != 1 || week.ActiveDays != 2 {
		t.Fatalf("week firsts %d %d active %d", week.NewTracks, week.NewArtists, week.ActiveDays)
	}
	if len(week.TopTracks) != 2 || week.TopTracks[0].Name != "One" || week.TopTracks[0].Track == nil || !week.TopTracks[0].Available {
		t.Fatalf("top tracks %+v", week.TopTracks)
	}
	if len(week.TopArtists) != 2 || week.TopArtists[0].CoverArt == "" || !week.TopArtists[0].Available {
		t.Fatalf("top artists %+v", week.TopArtists)
	}
	if len(week.TopAlbums) != 2 || week.TopAlbums[0].Artist != "Alpha" || len(week.Clients) != 1 {
		t.Fatalf("top albums %+v clients %+v", week.TopAlbums, week.Clients)
	}

	all, err := s.Report(ctx, u.ID, Query{Loc: plus8, Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	if all.From != ms(now.Add(-40*day)) || all.Previous != nil || all.Totals.Plays != 4 || len(all.TopTracks) != 1 {
		t.Fatalf("all time from=%d previous=%v plays=%d tops=%d", all.From, all.Previous, all.Totals.Plays, len(all.TopTracks))
	}

	// Purged tracks stay in the report, without links.
	if err := st.DeleteTracks(ctx, []string{ts[1].ID}); err != nil {
		t.Fatal(err)
	}
	week, _ = s.Report(ctx, u.ID, Query{From: ms(now.Add(-7 * day)), Loc: plus8})
	if len(week.TopTracks) != 2 || week.TopTracks[1].Name != "Two" || week.TopTracks[1].Track != nil || week.TopTracks[1].Available {
		t.Fatalf("purged top track %+v", week.TopTracks[1])
	}

	empty, err := s.Report(ctx, "nobody", Query{})
	if err != nil || empty.Totals.Plays != 0 || empty.FirstPlayAt != 0 || empty.TopTracks == nil {
		t.Fatalf("empty report %+v %v", empty, err)
	}
	if _, err := s.Report(ctx, u.ID, Query{From: 10, To: 5}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("inverted range: %v", err)
	}
}
