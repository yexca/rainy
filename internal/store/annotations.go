package store

import (
	"context"
	"fmt"

	"github.com/jmoiron/sqlx"

	"rainy/internal/model"
	"rainy/internal/util"
)

// SetStarred stars or unstars items of one type for a user. Re-starring keeps the original
// starred_at.
func (s *Store) SetStarred(ctx context.Context, userID, itemType string, ids []string, starred bool) error {
	if err := validItemType(itemType); err != nil {
		return err
	}
	ids = dedupe(ids)
	if len(ids) == 0 {
		return nil
	}
	var at *int64
	if starred {
		now := util.NowMs()
		at = &now
	}
	return s.db.Tx(ctx, func(tx *sqlx.Tx) error {
		stmt, err := tx.PreparexContext(ctx, `INSERT INTO annotations (user_id, item_type, item_id, starred_at)
			VALUES (?, ?, ?, ?)
			ON CONFLICT (user_id, item_type, item_id) DO UPDATE SET starred_at =
				CASE WHEN excluded.starred_at IS NULL THEN NULL ELSE COALESCE(annotations.starred_at, excluded.starred_at) END`)
		if err != nil {
			return err
		}
		defer func() { _ = stmt.Close() }()
		for _, id := range ids {
			if _, err := stmt.ExecContext(ctx, userID, itemType, id, at); err != nil {
				return fmt.Errorf("starring %s %s: %w", itemType, id, err)
			}
		}
		return nil
	})
}

// SetRating sets a 1–5 rating; 0 clears it.
func (s *Store) SetRating(ctx context.Context, userID, itemType, id string, rating int) error {
	if err := validItemType(itemType); err != nil {
		return err
	}
	if rating < 0 || rating > 5 {
		return fmt.Errorf("%w: rating %d (want 0-5)", ErrInvalid, rating)
	}
	_, err := s.db.W.ExecContext(ctx, `INSERT INTO annotations (user_id, item_type, item_id, rating) VALUES (?, ?, ?, ?)
		ON CONFLICT (user_id, item_type, item_id) DO UPDATE SET rating = excluded.rating`, userID, itemType, id, rating)
	return err
}

// RecordPlay registers a completed play at `at` (unix ms, 0 = now): play_count+1 and
// played_at on the track, its album and its artist (and album artist when different), and
// a play_history row. Unknown tracks yield ErrNotFound.
func (s *Store) RecordPlay(ctx context.Context, userID, trackID string, at int64, client string) error {
	if at <= 0 {
		at = util.NowMs()
	}
	return s.db.Tx(ctx, func(tx *sqlx.Tx) error {
		var t struct {
			AlbumID       string `db:"album_id"`
			ArtistID      string `db:"artist_id"`
			AlbumArtistID string `db:"album_artist_id"`
		}
		if err := tx.GetContext(ctx, &t, `SELECT album_id, artist_id, album_artist_id FROM tracks WHERE id = ?`, trackID); err != nil {
			return notFound(err)
		}
		items := [][2]string{{"track", trackID}, {"album", t.AlbumID}, {"artist", t.ArtistID}}
		if t.AlbumArtistID != "" && t.AlbumArtistID != t.ArtistID {
			items = append(items, [2]string{"artist", t.AlbumArtistID})
		}
		for _, it := range items {
			if it[1] == "" {
				continue
			}
			if _, err := tx.ExecContext(ctx, `INSERT INTO annotations (user_id, item_type, item_id, play_count, played_at)
				VALUES (?, ?, ?, 1, ?)
				ON CONFLICT (user_id, item_type, item_id) DO UPDATE SET play_count = annotations.play_count + 1,
					played_at = MAX(annotations.played_at, excluded.played_at)`, userID, it[0], it[1], at); err != nil {
				return err
			}
		}
		_, err := tx.ExecContext(ctx, `INSERT INTO play_history (user_id, track_id, played_at, client) VALUES (?, ?, ?, ?)`,
			userID, trackID, at, client)
		return err
	})
}

// RecentlyPlayedTracks returns the user's distinct recently played (non-missing) tracks,
// newest first.
func (s *Store) RecentlyPlayedTracks(ctx context.Context, userID string, limit int) ([]model.Track, error) {
	if limit <= 0 {
		limit = 50
	}
	var ids []string
	err := s.db.R.SelectContext(ctx, &ids, `SELECT ph.track_id FROM play_history ph
		JOIN tracks t ON t.id = ph.track_id AND t.missing = 0
		WHERE ph.user_id = ?
		GROUP BY ph.track_id
		ORDER BY MAX(ph.played_at) DESC, MAX(ph.id) DESC
		LIMIT ?`, userID, limit)
	if err != nil {
		return nil, err
	}
	return s.GetTracks(ctx, ids, userID)
}
