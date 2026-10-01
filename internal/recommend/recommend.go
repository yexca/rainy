// Package recommend picks songs for a user from their own library and listening history:
// mixes that continue a queue (the web player's infinite mode) and a daily mix
// (docs/architecture/contract.md §5.18). It only reads local data; nothing leaves the server.
package recommend

import (
	"context"
	"errors"
	"math"
	"math/rand/v2"
	"slices"
	"time"

	"rainy/internal/model"
	"rainy/internal/store"
)

// Limits of the mix and daily mix.
const (
	DefaultMixSize = 10
	MaxMixSize     = 50
	MaxSeeds       = 10
	MaxExclude     = 1000
	DailySize      = 30
	// dailyKeep is how long stored daily mixes are kept.
	dailyKeep = 30 * 24 * time.Hour
)

// ErrInvalid reports a bad request.
var ErrInvalid = errors.New("invalid recommendation request")

// Service picks songs.
type Service struct {
	st   *store.Store
	now  func() time.Time
	rand func() float64 // in [0, 1)
}

// New creates the service.
func New(st *store.Store) *Service { return &Service{st: st, now: time.Now, rand: rand.Float64} }

// MixQuery asks for songs that follow the seed tracks (the songs just played, newest last).
// Exclude lists tracks that must not be picked (the queue); the seeds are never picked.
type MixQuery struct {
	Seeds   []string
	Exclude []string
	Limit   int // DefaultMixSize when 0, at most MaxMixSize
}

// Mix returns songs that suit the seeds: by the same artists, by artists sharing their genres,
// then from the same genres, then anything, favoring songs the user starred, rated highly or
// plays often, avoiding songs played in the last day and songs rated one star, with at most
// two songs per artist when the library allows. Without usable seeds the user's top artists
// of the last 90 days act as seeds.
func (s *Service) Mix(ctx context.Context, userID string, q MixQuery) ([]model.Track, error) {
	limit := q.Limit
	if limit <= 0 {
		limit = DefaultMixSize
	}
	if limit > MaxMixSize {
		return nil, ErrInvalid
	}
	if len(q.Seeds) > MaxSeeds || len(q.Exclude) > MaxExclude {
		return nil, ErrInvalid
	}
	now := s.now()
	seedTracks, err := s.st.GetTracks(ctx, q.Seeds, userID)
	if err != nil {
		return nil, err
	}
	var seedIDs, artists []string
	for _, t := range seedTracks {
		if t.Missing {
			continue
		}
		seedIDs = append(seedIDs, t.ID)
		artists = append(artists, t.ArtistID, t.AlbumArtistID)
	}
	if len(artists) == 0 {
		if artists, err = s.topIDs(ctx, userID, store.TopArtists, now.Add(-90*24*time.Hour), 5); err != nil {
			return nil, err
		}
	}
	genres, err := s.st.TrackGenreIDs(ctx, seedIDs)
	if err != nil {
		return nil, err
	}
	similar, err := s.st.SimilarArtistIDs(ctx, artists, 10)
	if err != nil {
		return nil, err
	}

	exclude := make(map[string]bool, len(q.Exclude)+len(q.Seeds))
	for _, id := range append(slices.Clone(q.Exclude), q.Seeds...) {
		exclude[id] = true
	}
	p := newPicker(s, now, exclude)
	sample := 4 * limit
	sources := []source{
		{weight: 1.0, q: store.CandidateQuery{ArtistIDs: artists, Limit: sample}, narrow: true},
		{weight: 0.8, q: store.CandidateQuery{ArtistIDs: similar, Limit: sample}, narrow: true},
		{weight: 0.55, q: store.CandidateQuery{GenreIDs: genres, Limit: sample}, narrow: true},
		{weight: 0.25, q: store.CandidateQuery{Limit: 2 * limit}},
	}
	if err := p.load(ctx, userID, sources); err != nil {
		return nil, err
	}
	ids := p.spread(p.pick(limit, 2))
	return s.tracks(ctx, userID, ids)
}

// DailyMix is the user's mix of one local day.
type DailyMix struct {
	Date      string        `json:"date"`      // local day, YYYY-MM-DD
	CreatedAt int64         `json:"createdAt"` // unix ms
	Tracks    []model.Track `json:"tracks"`
}

// Daily returns the user's mix of today in loc (nil = UTC), making and storing it on the first
// request of the day so it stays the same all day and on every device. Songs removed since
// are left out. Songs come from four sources: artists similar to the user's top artists of
// the last 30 days, favorites not played for 30 days ("rediscover"), never played songs of
// the user's top genres of the last 90 days, and the top artists and starred songs
// themselves; random songs fill the rest (a new user gets random songs).
func (s *Service) Daily(ctx context.Context, userID string, loc *time.Location) (*DailyMix, error) {
	if loc == nil {
		loc = time.UTC
	}
	now := s.now()
	day := now.In(loc).Format(time.DateOnly)
	row, err := s.st.GetDailyMix(ctx, userID, day)
	if errors.Is(err, store.ErrNotFound) {
		var ids []string
		if ids, err = s.makeDaily(ctx, userID, now); err != nil {
			return nil, err
		}
		row, err = s.st.SaveDailyMix(ctx, userID, store.DailyMixRow{Day: day, TrackIDs: ids, CreatedAt: now.UnixMilli()},
			now.Add(-dailyKeep).UnixMilli())
	}
	if err != nil {
		return nil, err
	}
	tracks, err := s.tracks(ctx, userID, row.TrackIDs)
	if err != nil {
		return nil, err
	}
	return &DailyMix{Date: row.Day, CreatedAt: row.CreatedAt, Tracks: tracks}, nil
}

// Daily mix sources and how many songs each contributes.
const (
	srcSimilar     = "similar"
	srcRediscover  = "rediscover"
	srcDiscover    = "discover"
	srcFamiliar    = "familiar"
	srcFill        = "fill"
	dailyPerArtist = 2
)

var dailyQuota = []struct {
	src string
	n   int
}{{srcSimilar, 9}, {srcRediscover, 8}, {srcDiscover, 7}, {srcFamiliar, 6}}

func (s *Service) makeDaily(ctx context.Context, userID string, now time.Time) ([]string, error) {
	day := 24 * time.Hour
	top, err := s.topIDs(ctx, userID, store.TopArtists, now.Add(-30*day), 8)
	if err != nil {
		return nil, err
	}
	if len(top) == 0 {
		if top, err = s.topIDs(ctx, userID, store.TopArtists, time.Time{}, 8); err != nil {
			return nil, err
		}
	}
	genres, err := s.topIDs(ctx, userID, store.TopGenres, now.Add(-90*day), 5)
	if err != nil {
		return nil, err
	}
	similar, err := s.st.SimilarArtistIDs(ctx, top, 15)
	if err != nil {
		return nil, err
	}

	p := newPicker(s, now, nil)
	p.recentPenalty = 3 * day // a daily mix should not repeat yesterday
	sources := []source{
		{src: srcSimilar, weight: 0.9, q: store.CandidateQuery{ArtistIDs: similar, Limit: 150}, narrow: true},
		{src: srcRediscover, weight: 0.9, q: store.CandidateQuery{Favorites: true, Played: store.PlayedBefore,
			Before: now.Add(-30 * day).UnixMilli(), Limit: 150}},
		{src: srcDiscover, weight: 0.8, q: store.CandidateQuery{GenreIDs: genres, Played: store.PlayedNever, Limit: 150}, narrow: true},
		{src: srcFamiliar, weight: 0.7, q: store.CandidateQuery{ArtistIDs: top, Limit: 100}, narrow: true},
		{src: srcFamiliar, weight: 0.7, q: store.CandidateQuery{Favorites: true, Limit: 60}},
		{src: srcFill, weight: 0.2, q: store.CandidateQuery{Limit: 120}},
	}
	if err := p.load(ctx, userID, sources); err != nil {
		return nil, err
	}

	// Fill each source's quota, then the rest from every source by score, and interleave
	// the sources so the mix doesn't play in blocks.
	var groups [][]string
	for _, qt := range dailyQuota {
		groups = append(groups, p.pickFrom(qt.src, qt.n, dailyPerArtist))
	}
	picked := interleave(groups)
	picked = append(picked, p.pick(DailySize-len(picked), dailyPerArtist)...)
	return p.spread(picked), nil
}

// topIDs returns the ids of the user's top artists or genres since `since` (zero = all time).
func (s *Service) topIDs(ctx context.Context, userID, kind string, since time.Time, n int) ([]string, error) {
	var from int64
	if !since.IsZero() {
		from = since.UnixMilli()
	}
	items, err := s.st.ListeningTop(ctx, userID, kind, from, 0, n)
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(items))
	for _, it := range items {
		ids = append(ids, it.ID)
	}
	return ids, nil
}

// tracks loads the picked tracks in order, leaving out missing ones.
func (s *Service) tracks(ctx context.Context, userID string, ids []string) ([]model.Track, error) {
	tracks, err := s.st.GetTracks(ctx, ids, userID)
	if err != nil {
		return nil, err
	}
	out := make([]model.Track, 0, len(tracks))
	for _, t := range tracks {
		if !t.Missing {
			out = append(out, t)
		}
	}
	return out, nil
}

// ---- scoring and picking

type scored struct {
	store.Candidate
	src   string
	score float64
}

// artist is the candidate's artist for the per-artist cap: the track artist (the album
// artist of a compilation would group unrelated songs), else the album artist.
func (c *scored) artist() string {
	if c.ArtistID != "" {
		return c.ArtistID
	}
	return c.AlbumArtistID
}

type picker struct {
	s             *Service
	now           time.Time
	exclude       map[string]bool
	recentPenalty time.Duration // songs played this recently are pushed back
	cands         map[string]*scored
	order         []string // candidate ids in arrival order
	taken         map[string]bool
	perArtist     map[string]int
}

func newPicker(s *Service, now time.Time, exclude map[string]bool) *picker {
	return &picker{s: s, now: now, exclude: exclude, recentPenalty: 24 * time.Hour,
		cands: map[string]*scored{}, taken: map[string]bool{}, perArtist: map[string]int{}}
}

// source is one candidate query of a mix. A narrow source is skipped when it has no artists
// or genres to narrow by (an empty filter would sample the whole library).
type source struct {
	src    string
	weight float64
	q      store.CandidateQuery
	narrow bool
}

func (p *picker) load(ctx context.Context, userID string, sources []source) error {
	for _, src := range sources {
		if src.narrow && len(src.q.ArtistIDs) == 0 && len(src.q.GenreIDs) == 0 {
			continue
		}
		cands, err := p.s.st.RecommendCandidates(ctx, userID, src.q)
		if err != nil {
			return err
		}
		p.add(src.src, src.weight, cands)
	}
	return nil
}

// add scores candidates of a source; a track found by several sources keeps its best score.
func (p *picker) add(src string, weight float64, cands []store.Candidate) {
	for _, c := range cands {
		if p.exclude[c.ID] || c.Rating == 1 {
			continue
		}
		sc := p.score(c, weight)
		if old, ok := p.cands[c.ID]; ok {
			if sc > old.score {
				old.score, old.src = sc, src
			}
			continue
		}
		p.cands[c.ID] = &scored{Candidate: c, src: src, score: sc}
		p.order = append(p.order, c.ID)
	}
}

// score: the source weight, the user's signals (star, rating, play count) and a random part
// for variety, minus a penalty for songs played recently.
func (p *picker) score(c store.Candidate, weight float64) float64 {
	sc := weight
	if c.Starred {
		sc += 0.35
	}
	if c.Rating > 0 {
		sc += float64(c.Rating-3) * 0.15
	}
	sc += math.Min(math.Log1p(float64(c.Plays)), 3) * 0.08
	if c.PlayedAt > 0 {
		since := p.now.Sub(time.UnixMilli(c.PlayedAt))
		switch {
		case since < p.recentPenalty:
			sc -= 1
		case since < 7*24*time.Hour:
			sc -= 0.25
		}
	}
	return sc + p.s.rand()*0.4
}

// ranked returns the untaken candidates of src ("" = any source), best first.
func (p *picker) ranked(src string) []*scored {
	var out []*scored
	for _, id := range p.order {
		c := p.cands[id]
		if !p.taken[id] && (src == "" || c.src == src) {
			out = append(out, c)
		}
	}
	slices.SortStableFunc(out, func(a, b *scored) int {
		switch {
		case a.score > b.score:
			return -1
		case a.score < b.score:
			return 1
		}
		return 0
	})
	return out
}

// pickFrom takes up to n songs of src, at most perArtist per artist across everything picked.
func (p *picker) pickFrom(src string, n, perArtist int) []string {
	var out []string
	for _, c := range p.ranked(src) {
		if len(out) >= n {
			break
		}
		if p.perArtist[c.artist()] >= perArtist {
			continue
		}
		p.take(c)
		out = append(out, c.ID)
	}
	return out
}

// pick takes up to n songs from any source; when the per-artist cap leaves it short (a small
// library) the cap is lifted for the rest.
func (p *picker) pick(n, perArtist int) []string {
	if n <= 0 {
		return nil
	}
	out := p.pickFrom("", n, perArtist)
	for _, c := range p.ranked("") {
		if len(out) >= n {
			break
		}
		p.take(c)
		out = append(out, c.ID)
	}
	return out
}

func (p *picker) take(c *scored) {
	p.taken[c.ID] = true
	p.perArtist[c.artist()]++
}

// interleave merges groups round robin.
func interleave(groups [][]string) []string {
	var out []string
	for i := 0; ; i++ {
		added := false
		for _, g := range groups {
			if i < len(g) {
				out = append(out, g[i])
				added = true
			}
		}
		if !added {
			return out
		}
	}
}

// spread reorders picks so the same artist doesn't play twice in a row when another song can
// go in between: a later song by someone else is swapped in, or, when only that artist is
// left (a run at the end), the song moves back to the earliest gap between two other artists.
func (p *picker) spread(ids []string) []string {
	artist := func(id string) string { return p.cands[id].artist() }
	for i := 1; i < len(ids); i++ {
		a := artist(ids[i])
		if a != artist(ids[i-1]) {
			continue
		}
		swapped := false
		for j := i + 1; j < len(ids); j++ {
			if artist(ids[j]) != a {
				ids[i], ids[j] = ids[j], ids[i]
				swapped = true
				break
			}
		}
		if swapped {
			continue
		}
		for k := 0; k < i; k++ {
			if (k == 0 || artist(ids[k-1]) != a) && artist(ids[k]) != a {
				id := ids[i]
				copy(ids[k+1:i+1], ids[k:i])
				ids[k] = id
				break
			}
		}
	}
	return ids
}
