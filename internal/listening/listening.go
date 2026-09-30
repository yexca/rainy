// Package listening builds a user's listening report from the play history: totals, a
// timeline, the hour-of-week clock and top artists, albums, tracks, genres and players
// (docs/architecture/contract.md §5.17). Reports are private: a user only sees their own.
package listening

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"rainy/internal/model"
	"rainy/internal/store"
)

// Timeline bucket sizes.
const (
	BucketHour  = "hour"
	BucketDay   = "day"
	BucketWeek  = "week"
	BucketMonth = "month"
)

// Limits of Query.Limit.
const (
	DefaultLimit = 10
	MaxLimit     = 50
)

// Query selects a report period: plays with From <= played_at < To (unix ms). From 0 means
// "since the first play", To 0 means now. Loc is the viewer's time zone for days, weeks and
// hours (nil = UTC).
type Query struct {
	From, To int64
	Loc      *time.Location
	Limit    int // entries per top list (DefaultLimit when 0, at most MaxLimit)
}

// Report is the listening report of one period.
type Report struct {
	From        int64                  `json:"from"`        // effective start (the first play for "all time")
	To          int64                  `json:"to"`          // effective end (at most now)
	TZ          string                 `json:"tz"`          // time zone the timeline and clock use
	Bucket      string                 `json:"bucket"`      // hour | day | week | month
	FirstPlayAt int64                  `json:"firstPlayAt"` // the user's first play ever (0 = none)
	Totals      store.ListeningTotals  `json:"totals"`
	Previous    *store.ListeningTotals `json:"previous"` // the period of the same length just before; nil for all time
	NewTracks   int                    `json:"newTracks"`
	NewArtists  int                    `json:"newArtists"`
	ActiveDays  int                    `json:"activeDays"`    // days with at least one play
	LongestRun  int                    `json:"longestStreak"` // most consecutive active days
	Timeline    []Bucket               `json:"timeline"`
	Clock       [7][24]int             `json:"clock"` // plays by weekday (0 = Monday) and hour, local time
	TopArtists  []TopEntry             `json:"topArtists"`
	TopAlbums   []TopEntry             `json:"topAlbums"`
	TopTracks   []TopTrack             `json:"topTracks"`
	TopGenres   []TopEntry             `json:"topGenres"`
	Clients     []TopEntry             `json:"clients"`
}

// Bucket is one step of the timeline.
type Bucket struct {
	Start    int64   `json:"start"` // unix ms of the local bucket start
	Plays    int     `json:"plays"`
	Duration float64 `json:"duration"`
}

// TopEntry is a row of a top list. Artist is the album artist for albums (empty otherwise);
// CoverArt is empty and Available false when the item no longer exists in the library.
type TopEntry struct {
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	Artist    string  `json:"artist"`
	Plays     int     `json:"plays"`
	Duration  float64 `json:"duration"`
	CoverArt  string  `json:"coverArt"`
	Available bool    `json:"available"`
}

// TopTrack is a row of the top tracks, with the live track (nil when purged or missing).
type TopTrack struct {
	TopEntry
	Track *model.Track `json:"track"`
}

// ErrInvalid reports a bad query.
var ErrInvalid = errors.New("invalid listening query")

// Service builds reports.
type Service struct {
	st  *store.Store
	now func() time.Time
}

// New creates the service.
func New(st *store.Store) *Service { return &Service{st: st, now: time.Now} }

// Report builds the user's report for q.
func (s *Service) Report(ctx context.Context, userID string, q Query) (*Report, error) {
	loc := q.Loc
	if loc == nil {
		loc = time.UTC
	}
	limit := q.Limit
	if limit <= 0 {
		limit = DefaultLimit
	}
	limit = min(limit, MaxLimit)
	now := s.now().UnixMilli()
	if q.From < 0 || q.To < 0 || (q.To > 0 && q.To <= q.From) {
		return nil, fmt.Errorf("%w: from must be before to", ErrInvalid)
	}
	first, err := s.st.FirstPlayAt(ctx, userID)
	if err != nil {
		return nil, err
	}
	from, to := q.From, q.To
	if to == 0 || to > now {
		to = now + 1
	}
	allTime := from == 0
	if allTime {
		from = first
		if from == 0 || from >= to {
			from = to - int64(24*time.Hour/time.Millisecond)
		}
	}
	if from >= to {
		from = to - 1
	}

	r := &Report{From: from, To: to, TZ: loc.String(), FirstPlayAt: first,
		Timeline: []Bucket{}, TopArtists: []TopEntry{}, TopAlbums: []TopEntry{}, TopTracks: []TopTrack{},
		TopGenres: []TopEntry{}, Clients: []TopEntry{}}
	if r.Totals, err = s.st.ListeningTotals(ctx, userID, from, to); err != nil {
		return nil, err
	}
	if !allTime {
		prevFrom := max(from-(to-from), 0)
		prev, err := s.st.ListeningTotals(ctx, userID, prevFrom, from)
		if err != nil {
			return nil, err
		}
		r.Previous = &prev
	}
	if r.NewTracks, r.NewArtists, err = s.st.ListeningFirsts(ctx, userID, from, to); err != nil {
		return nil, err
	}
	times, err := s.st.ListeningTimes(ctx, userID, from, to)
	if err != nil {
		return nil, err
	}
	r.Bucket = BucketFor(from, to)
	r.Timeline = Timeline(times, from, to, r.Bucket, loc)
	r.Clock = Clock(times, loc)
	r.ActiveDays, r.LongestRun = ActiveDays(times, loc)

	if err := s.fillTops(ctx, r, userID, from, to, limit); err != nil {
		return nil, err
	}
	return r, nil
}

func (s *Service) fillTops(ctx context.Context, r *Report, userID string, from, to int64, limit int) error {
	top := func(kind string, n int) ([]store.ListeningTopItem, error) {
		return s.st.ListeningTop(ctx, userID, kind, from, to, n)
	}
	entry := func(it store.ListeningTopItem) TopEntry {
		return TopEntry{ID: it.ID, Name: it.Name, Artist: it.Artist, Plays: it.Plays, Duration: it.Duration}
	}

	artists, err := top(store.TopArtists, limit)
	if err != nil {
		return err
	}
	for _, it := range artists {
		e := entry(it)
		if a, err := s.st.GetArtist(ctx, it.ID, userID); err == nil {
			e.Name, e.CoverArt, e.Available = a.Name, a.CoverArt, true
		} else if !errors.Is(err, store.ErrNotFound) {
			return err
		}
		r.TopArtists = append(r.TopArtists, e)
	}

	albums, err := top(store.TopAlbums, limit)
	if err != nil {
		return err
	}
	for _, it := range albums {
		e := entry(it)
		if a, err := s.st.GetAlbum(ctx, it.ID, userID); err == nil {
			e.Name, e.Artist, e.CoverArt, e.Available = a.Name, a.AlbumArtist, a.CoverArt, true
		} else if !errors.Is(err, store.ErrNotFound) {
			return err
		}
		r.TopAlbums = append(r.TopAlbums, e)
	}

	tracks, err := top(store.TopTracks, limit)
	if err != nil {
		return err
	}
	ids := make([]string, 0, len(tracks))
	for _, it := range tracks {
		ids = append(ids, it.ID)
	}
	live, err := s.st.GetTracks(ctx, ids, userID)
	if err != nil {
		return err
	}
	byID := make(map[string]*model.Track, len(live))
	for i := range live {
		if !live[i].Missing {
			byID[live[i].ID] = &live[i]
		}
	}
	for _, it := range tracks {
		tt := TopTrack{TopEntry: entry(it), Track: byID[it.ID]}
		if tt.Track != nil {
			tt.CoverArt, tt.Available = tt.Track.CoverArt, true
		}
		r.TopTracks = append(r.TopTracks, tt)
	}

	genres, err := top(store.TopGenres, DefaultLimit)
	if err != nil {
		return err
	}
	for _, it := range genres {
		e := entry(it)
		e.Available = true
		r.TopGenres = append(r.TopGenres, e)
	}
	clients, err := top(store.TopClients, DefaultLimit)
	if err != nil {
		return err
	}
	for _, it := range clients {
		r.Clients = append(r.Clients, entry(it))
	}
	return nil
}

// BucketFor picks the timeline step for a period: hours up to 3 days, days up to about
// three months, weeks up to two years, months beyond.
func BucketFor(from, to int64) string {
	span := time.Duration(to-from) * time.Millisecond
	const day = 24 * time.Hour
	switch {
	case span <= 3*day:
		return BucketHour
	case span <= 93*day:
		return BucketDay
	case span <= 731*day:
		return BucketWeek
	default:
		return BucketMonth
	}
}

// bucketStart returns the local start of the bucket containing t.
func bucketStart(t time.Time, bucket string) time.Time {
	y, m, d := t.Date()
	switch bucket {
	case BucketHour:
		return time.Date(y, m, d, t.Hour(), 0, 0, 0, t.Location())
	case BucketWeek:
		offset := (int(t.Weekday()) + 6) % 7 // Monday = 0
		return time.Date(y, m, d-offset, 0, 0, 0, 0, t.Location())
	case BucketMonth:
		return time.Date(y, m, 1, 0, 0, 0, 0, t.Location())
	default:
		return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
	}
}

func nextBucket(t time.Time, bucket string) time.Time {
	switch bucket {
	case BucketHour:
		return t.Add(time.Hour)
	case BucketWeek:
		y, m, d := t.Date()
		return time.Date(y, m, d+7, 0, 0, 0, 0, t.Location())
	case BucketMonth:
		y, m, _ := t.Date()
		return time.Date(y, m+1, 1, 0, 0, 0, 0, t.Location())
	default:
		y, m, d := t.Date()
		return time.Date(y, m, d+1, 0, 0, 0, 0, t.Location())
	}
}

// Timeline counts plays per bucket from the bucket containing from up to to (exclusive),
// including empty buckets. times must be sorted by At.
func Timeline(times []store.PlayTime, from, to int64, bucket string, loc *time.Location) []Bucket {
	out := []Bucket{}
	start := bucketStart(time.UnixMilli(from).In(loc), bucket)
	end := time.UnixMilli(to).In(loc)
	i := 0
	for b := start; b.Before(end); b = nextBucket(b, bucket) {
		next := nextBucket(b, bucket).UnixMilli()
		cur := Bucket{Start: b.UnixMilli()}
		for i < len(times) && times[i].At < next {
			if times[i].At >= cur.Start {
				cur.Plays++
				cur.Duration += times[i].Duration
			}
			i++
		}
		out = append(out, cur)
		if len(out) > 5000 { // defensive: never build an unbounded timeline
			break
		}
	}
	return out
}

// Clock counts plays by local weekday (0 = Monday) and hour.
func Clock(times []store.PlayTime, loc *time.Location) [7][24]int {
	var c [7][24]int
	for _, p := range times {
		t := time.UnixMilli(p.At).In(loc)
		c[(int(t.Weekday())+6)%7][t.Hour()]++
	}
	return c
}

// ActiveDays returns the number of local days with plays and the longest run of
// consecutive such days.
func ActiveDays(times []store.PlayTime, loc *time.Location) (days, longest int) {
	seen := map[int64]bool{}
	for _, p := range times {
		y, m, d := time.UnixMilli(p.At).In(loc).Date()
		seen[time.Date(y, m, d, 0, 0, 0, 0, time.UTC).Unix()/86400] = true
	}
	keys := make([]int64, 0, len(seen))
	for k := range seen {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
	run := 0
	for i, k := range keys {
		if i > 0 && k == keys[i-1]+1 {
			run++
		} else {
			run = 1
		}
		longest = max(longest, run)
	}
	return len(keys), longest
}

// LoadLocation resolves an IANA time zone name ("" or unknown → UTC).
func LoadLocation(name string) *time.Location {
	if name == "" || len(name) > 64 {
		return time.UTC
	}
	loc, err := time.LoadLocation(name)
	if err != nil {
		return time.UTC
	}
	return loc
}
