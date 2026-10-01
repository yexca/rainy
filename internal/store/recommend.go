package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
)

// Recommendation queries (docs/architecture/contract.md §5.18). They return candidate tracks
// with the user's own signals; package recommend scores and picks them.

// SimilarArtistIDs returns the album artists sharing the most genres with the given artists'
// tracks (most shared genres first, then most matching tracks), never one of the given
// artists. Missing tracks are ignored.
func (s *Store) SimilarArtistIDs(ctx context.Context, artistIDs []string, limit int) ([]string, error) {
	artistIDs = dedupe(artistIDs)
	if limit <= 0 || len(artistIDs) == 0 {
		return nil, nil
	}
	if len(artistIDs) > maxVars/3 {
		artistIDs = artistIDs[:maxVars/3]
	}
	ph := placeholders(len(artistIDs))
	args := make([]any, 0, 3*len(artistIDs)+1)
	for range 3 {
		args = append(args, anys(artistIDs)...)
	}
	var ids []string
	err := s.db.R.SelectContext(ctx, &ids, `
		WITH g AS (
			SELECT DISTINCT tg.genre_id FROM track_genres tg JOIN tracks t ON t.id = tg.track_id
			WHERE t.missing = 0 AND (t.artist_id IN (`+ph+`) OR t.album_artist_id IN (`+ph+`))
		)
		SELECT t.album_artist_id FROM tracks t
		JOIN track_genres tg ON tg.track_id = t.id
		JOIN artists ar ON ar.id = t.album_artist_id AND ar.album_count > 0
		WHERE t.missing = 0 AND tg.genre_id IN (SELECT genre_id FROM g) AND t.album_artist_id NOT IN (`+ph+`)
		GROUP BY t.album_artist_id
		ORDER BY COUNT(DISTINCT tg.genre_id) DESC, COUNT(*) DESC, t.album_artist_id
		LIMIT ?`, append(args, limit)...)
	if err != nil {
		return nil, fmt.Errorf("similar artists: %w", err)
	}
	return ids, nil
}

// TrackGenreIDs returns the distinct genre ids of the given tracks.
func (s *Store) TrackGenreIDs(ctx context.Context, trackIDs []string) ([]string, error) {
	trackIDs = dedupe(trackIDs)
	if len(trackIDs) == 0 {
		return nil, nil
	}
	if len(trackIDs) > maxVars {
		trackIDs = trackIDs[:maxVars]
	}
	var ids []string
	err := s.db.R.SelectContext(ctx, &ids, `SELECT DISTINCT genre_id FROM track_genres
		WHERE track_id IN (`+placeholders(len(trackIDs))+`) ORDER BY genre_id`, anys(trackIDs)...)
	if err != nil {
		return nil, fmt.Errorf("track genres: %w", err)
	}
	return ids, nil
}

// Candidate is a track that could be recommended, with the user's signals for it.
type Candidate struct {
	ID            string  `db:"id"`
	ArtistID      string  `db:"artist_id"`
	AlbumArtistID string  `db:"album_artist_id"`
	AlbumID       string  `db:"album_id"`
	Duration      float64 `db:"duration"`
	Plays         int     `db:"plays"`     // the user's play count
	PlayedAt      int64   `db:"played_at"` // the user's last play (0 = never)
	Starred       bool    `db:"starred"`
	Rating        int     `db:"rating"` // 0 = not rated, 1..5
}

// Values of CandidateQuery.Played.
const (
	PlayedAny    = ""
	PlayedNever  = "never"  // the user never played the track
	PlayedBefore = "before" // played, last time before CandidateQuery.Before
)

// CandidateQuery selects a random sample of tracks that are not missing. ArtistIDs and
// GenreIDs narrow the sample (a track matches when its artist or album artist is listed, or
// when it has one of the genres); both empty means the whole library. Favorites keeps the
// tracks the user starred or played at least twice.
type CandidateQuery struct {
	ArtistIDs []string
	GenreIDs  []string
	Played    string // PlayedAny | PlayedNever | PlayedBefore
	Before    int64  // unix ms, for PlayedBefore
	Favorites bool
	Limit     int // sample size (default 50, at most 1000)
}

// RecommendCandidates returns a random sample of tracks matching q with the user's signals.
func (s *Store) RecommendCandidates(ctx context.Context, userID string, q CandidateQuery) ([]Candidate, error) {
	limit := q.Limit
	if limit <= 0 {
		limit = 50
	}
	limit = min(limit, 1000)
	w := &where{}
	w.add("t.missing = 0")
	artists, genres := dedupe(q.ArtistIDs), dedupe(q.GenreIDs)
	if len(artists) > maxVars/2 {
		artists = artists[:maxVars/2]
	}
	if len(genres) > maxVars/2 {
		genres = genres[:maxVars/2]
	}
	var match []string
	var matchArgs []any
	if len(artists) > 0 {
		ph := placeholders(len(artists))
		match = append(match, "t.artist_id IN ("+ph+") OR t.album_artist_id IN ("+ph+")")
		matchArgs = append(append(matchArgs, anys(artists)...), anys(artists)...)
	}
	if len(genres) > 0 {
		match = append(match, "t.id IN (SELECT track_id FROM track_genres WHERE genre_id IN ("+placeholders(len(genres))+"))")
		matchArgs = append(matchArgs, anys(genres)...)
	}
	if len(match) == 1 {
		w.add("("+match[0]+")", matchArgs...)
	} else if len(match) == 2 {
		w.add("("+match[0]+" OR "+match[1]+")", matchArgs...)
	}
	switch q.Played {
	case PlayedAny:
	case PlayedNever:
		w.add("COALESCE(an.play_count, 0) = 0")
	case PlayedBefore:
		w.add("COALESCE(an.play_count, 0) > 0 AND COALESCE(an.played_at, 0) < ?", q.Before)
	default:
		return nil, fmt.Errorf("%w: played filter %q", ErrInvalid, q.Played)
	}
	if q.Favorites {
		w.add("(an.starred_at IS NOT NULL OR COALESCE(an.play_count, 0) >= 2)")
	}
	var out []Candidate
	err := s.db.R.SelectContext(ctx, &out, `SELECT t.id, t.artist_id, t.album_artist_id, t.album_id, t.duration,
			COALESCE(an.play_count, 0) AS plays, COALESCE(an.played_at, 0) AS played_at,
			an.starred_at IS NOT NULL AS starred, COALESCE(an.rating, 0) AS rating
		FROM tracks t`+annotationJoin("track", "t.id")+w.sql()+`
		ORDER BY RANDOM() LIMIT ?`, append(append([]any{userID}, w.args...), limit)...)
	if err != nil {
		return nil, fmt.Errorf("recommendation candidates: %w", err)
	}
	return out, nil
}

// DailyMixRow is a stored daily mix.
type DailyMixRow struct {
	Day       string
	TrackIDs  []string
	CreatedAt int64
}

// GetDailyMix returns the user's mix of a local day (ErrNotFound when none was made).
func (s *Store) GetDailyMix(ctx context.Context, userID, day string) (*DailyMixRow, error) {
	var row struct {
		TrackIDs  string `db:"track_ids"`
		CreatedAt int64  `db:"created_at"`
	}
	err := s.db.R.GetContext(ctx, &row, `SELECT track_ids, created_at FROM daily_mixes WHERE user_id = ? AND day = ?`, userID, day)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("getting daily mix: %w", err)
	}
	out := &DailyMixRow{Day: day, CreatedAt: row.CreatedAt}
	if err := json.Unmarshal([]byte(row.TrackIDs), &out.TrackIDs); err != nil {
		return nil, fmt.Errorf("decoding daily mix: %w", err)
	}
	return out, nil
}

// SaveDailyMix stores the user's mix of a day unless one already exists (the first one wins),
// removes the user's mixes created before pruneBefore (unix ms), and returns the stored mix.
func (s *Store) SaveDailyMix(ctx context.Context, userID string, mix DailyMixRow, pruneBefore int64) (*DailyMixRow, error) {
	ids := mix.TrackIDs
	if ids == nil {
		ids = []string{}
	}
	b, err := json.Marshal(ids)
	if err != nil {
		return nil, err
	}
	if _, err := s.db.W.ExecContext(ctx, `INSERT INTO daily_mixes (user_id, day, track_ids, created_at) VALUES (?, ?, ?, ?)
		ON CONFLICT (user_id, day) DO NOTHING`, userID, mix.Day, string(b), mix.CreatedAt); err != nil {
		return nil, fmt.Errorf("saving daily mix: %w", err)
	}
	if _, err := s.db.W.ExecContext(ctx, `DELETE FROM daily_mixes WHERE user_id = ? AND created_at < ?`, userID, pruneBefore); err != nil {
		return nil, fmt.Errorf("pruning daily mixes: %w", err)
	}
	return s.GetDailyMix(ctx, userID, mix.Day)
}
