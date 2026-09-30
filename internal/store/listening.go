package store

import (
	"context"
	"fmt"
	"math"

	"rainy/internal/model"
)

// Listening queries read play_history for reports (docs/architecture/contract.md §5.17).
// Every query covers the plays of one user with from <= played_at < to (to <= 0 = no upper
// bound). A play's names, ids and duration come from the live track row when it still
// exists and from the snapshot taken at play time otherwise.

// Effective play_history columns (ph = play_history, t = LEFT JOINed tracks).
const (
	playTitle       = `COALESCE(t.title, ph.title)`
	playArtist      = `COALESCE(t.artist, ph.artist)`
	playAlbum       = `COALESCE(t.album, ph.album)`
	playAlbumArtist = `COALESCE(t.album_artist, ph.album_artist)`
	playArtistID    = `COALESCE(t.artist_id, ph.artist_id)`
	playAlbumID     = `COALESCE(t.album_id, ph.album_id)`
	playDuration    = `COALESCE(t.duration, ph.duration)`
	playFrom        = ` FROM play_history ph LEFT JOIN tracks t ON t.id = ph.track_id
		WHERE ph.user_id = ? AND ph.played_at >= ? AND ph.played_at < ?`
)

// Kinds of ListeningTop.
const (
	TopTracks  = "track"
	TopAlbums  = "album"
	TopArtists = "artist"
	TopGenres  = "genre"
	TopClients = "client"
)

func rangeArgs(userID string, from, to int64) []any {
	if to <= 0 {
		to = math.MaxInt64
	}
	return []any{userID, from, to}
}

// ListeningTotals are the headline numbers of a period.
type ListeningTotals struct {
	Plays    int     `db:"plays" json:"plays"`
	Duration float64 `db:"duration" json:"duration"` // seconds: the sum of the played tracks' durations
	Tracks   int     `db:"tracks" json:"tracks"`     // distinct
	Artists  int     `db:"artists" json:"artists"`
	Albums   int     `db:"albums" json:"albums"`
}

// ListeningTotals counts the plays of a period, their duration and the distinct tracks,
// artists and albums played.
func (s *Store) ListeningTotals(ctx context.Context, userID string, from, to int64) (ListeningTotals, error) {
	var out ListeningTotals
	err := s.db.R.GetContext(ctx, &out, `SELECT COUNT(*) AS plays, TOTAL(`+playDuration+`) AS duration,
			COUNT(DISTINCT ph.track_id) AS tracks,
			COUNT(DISTINCT NULLIF(`+playArtistID+`, '')) AS artists,
			COUNT(DISTINCT NULLIF(`+playAlbumID+`, '')) AS albums`+playFrom, rangeArgs(userID, from, to)...)
	if err != nil {
		return out, fmt.Errorf("listening totals: %w", err)
	}
	return out, nil
}

// PlayTime is when a play happened and how long the track is.
type PlayTime struct {
	At       int64   `db:"at"`
	Duration float64 `db:"duration"`
}

// ListeningTimes returns every play of a period (oldest first) for time-based charts.
func (s *Store) ListeningTimes(ctx context.Context, userID string, from, to int64) ([]PlayTime, error) {
	var out []PlayTime
	err := s.db.R.SelectContext(ctx, &out, `SELECT ph.played_at AS at, `+playDuration+` AS duration`+playFrom+`
		ORDER BY ph.played_at`, rangeArgs(userID, from, to)...)
	if err != nil {
		return nil, fmt.Errorf("listening times: %w", err)
	}
	return out, nil
}

// ListeningTopItem is one row of a top list. Artist is the track artist (tracks) or album
// artist (albums), empty otherwise.
type ListeningTopItem struct {
	ID       string  `db:"id"`
	Name     string  `db:"name"`
	Artist   string  `db:"artist"`
	Plays    int     `db:"plays"`
	Duration float64 `db:"duration"`
}

// ListeningTop returns the most played tracks, albums, artists, genres or clients (players)
// of a period, most plays first (ties: most recently played first). Plays without the id
// (purged before snapshots existed) are left out.
func (s *Store) ListeningTop(ctx context.Context, userID, kind string, from, to int64, limit int) ([]ListeningTopItem, error) {
	if limit <= 0 {
		limit = 10
	}
	var sel, from2, group string
	switch kind {
	case TopTracks:
		sel, group = `ph.track_id AS id, `+playTitle+` AS name, `+playArtist+` AS artist`, `ph.track_id`
	case TopAlbums:
		sel, group = playAlbumID+` AS id, `+playAlbum+` AS name, `+playAlbumArtist+` AS artist`, playAlbumID
	case TopArtists:
		sel, group = playArtistID+` AS id, `+playArtist+` AS name, '' AS artist`, playArtistID
	case TopGenres:
		sel, group = `g.id AS id, g.name AS name, '' AS artist`, `g.id`
		from2 = ` JOIN track_genres tg ON tg.track_id = ph.track_id JOIN genres g ON g.id = tg.genre_id`
	case TopClients:
		sel, group = `ph.client AS id, ph.client AS name, '' AS artist`, `ph.client`
	default:
		return nil, fmt.Errorf("%w: top list kind %q", ErrInvalid, kind)
	}
	q := `SELECT id, name, artist, plays, duration FROM (
			SELECT ` + sel + `, COUNT(*) AS plays, TOTAL(` + playDuration + `) AS duration, MAX(ph.played_at) AS last
			FROM play_history ph LEFT JOIN tracks t ON t.id = ph.track_id` + from2 + `
			WHERE ph.user_id = ? AND ph.played_at >= ? AND ph.played_at < ?
			GROUP BY ` + group + `
		) WHERE id != ''
		ORDER BY plays DESC, last DESC, id
		LIMIT ?`
	var out []ListeningTopItem
	if err := s.db.R.SelectContext(ctx, &out, q, append(rangeArgs(userID, from, to), limit)...); err != nil {
		return nil, fmt.Errorf("listening top %s: %w", kind, err)
	}
	return out, nil
}

// ListeningFirsts counts the tracks and artists the user played for the very first time in
// the period.
func (s *Store) ListeningFirsts(ctx context.Context, userID string, from, to int64) (tracks, artists int, err error) {
	args := rangeArgs(userID, from, to)
	err = s.db.R.GetContext(ctx, &tracks, `SELECT COUNT(*) FROM (
			SELECT MIN(played_at) AS first FROM play_history WHERE user_id = ? GROUP BY track_id
		) WHERE first >= ? AND first < ?`, args...)
	if err != nil {
		return 0, 0, fmt.Errorf("first plays: %w", err)
	}
	err = s.db.R.GetContext(ctx, &artists, `SELECT COUNT(*) FROM (
			SELECT MIN(ph.played_at) AS first FROM play_history ph LEFT JOIN tracks t ON t.id = ph.track_id
			WHERE ph.user_id = ? AND `+playArtistID+` != ''
			GROUP BY `+playArtistID+`
		) WHERE first >= ? AND first < ?`, args...)
	if err != nil {
		return 0, 0, fmt.Errorf("first artist plays: %w", err)
	}
	return tracks, artists, nil
}

// FirstPlayAt returns the time of the user's first recorded play (0 = none).
func (s *Store) FirstPlayAt(ctx context.Context, userID string) (int64, error) {
	var at int64
	err := s.db.R.GetContext(ctx, &at, `SELECT COALESCE(MIN(played_at), 0) FROM play_history WHERE user_id = ?`, userID)
	return at, err
}

// ListPlays returns the user's plays of a period, newest first, with the total count. Each
// play carries its live track (with annotations) unless the track was purged or is missing.
func (s *Store) ListPlays(ctx context.Context, userID string, from, to int64, offset, limit int) ([]model.Play, int, error) {
	if limit <= 0 {
		limit = 50
	}
	args := rangeArgs(userID, from, to)
	var total int
	if err := s.db.R.GetContext(ctx, &total, `SELECT COUNT(*) FROM play_history ph
		WHERE ph.user_id = ? AND ph.played_at >= ? AND ph.played_at < ?`, args...); err != nil {
		return nil, 0, fmt.Errorf("counting plays: %w", err)
	}
	plays := []model.Play{}
	err := s.db.R.SelectContext(ctx, &plays, `SELECT ph.id, ph.track_id, ph.played_at, ph.client,
			`+playTitle+` AS title, `+playArtist+` AS artist, `+playAlbum+` AS album,
			`+playAlbumArtist+` AS album_artist, `+playArtistID+` AS artist_id, `+playAlbumID+` AS album_id,
			`+playDuration+` AS duration`+playFrom+`
		ORDER BY ph.played_at DESC, ph.id DESC LIMIT ? OFFSET ?`, append(args, limit, max(offset, 0))...)
	if err != nil {
		return nil, 0, fmt.Errorf("listing plays: %w", err)
	}
	ids := make([]string, 0, len(plays))
	for _, p := range plays {
		ids = append(ids, p.TrackID)
	}
	tracks, err := s.GetTracks(ctx, ids, userID)
	if err != nil {
		return nil, 0, err
	}
	byID := make(map[string]*model.Track, len(tracks))
	for i := range tracks {
		if !tracks[i].Missing {
			byID[tracks[i].ID] = &tracks[i]
		}
	}
	for i := range plays {
		plays[i].Track = byID[plays[i].TrackID]
	}
	return plays, total, nil
}
