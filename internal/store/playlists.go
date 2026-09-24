package store

import (
	"context"
	"fmt"
	"sort"

	"github.com/jmoiron/sqlx"

	"rainy/internal/model"
	"rainy/internal/util"
)

const playlistSelect = `SELECT p.id, p.name, p.comment, p.owner_id, COALESCE(u.username, '') AS owner_name, p.public,
	COALESCE(agg.song_count, 0) AS song_count, COALESCE(agg.duration, 0) AS duration, p.created_at, p.updated_at,
	COALESCE(agg.albums_updated_at, 0) AS albums_updated_at
	FROM playlists p
	LEFT JOIN users u ON u.id = p.owner_id
	LEFT JOIN (
		SELECT pt.playlist_id, COUNT(*) AS song_count, SUM(t.duration) AS duration,
			MAX(COALESCE(al.updated_at, 0)) AS albums_updated_at
		FROM playlist_tracks pt JOIN tracks t ON t.id = pt.track_id AND t.missing = 0
		LEFT JOIN albums al ON al.id = t.album_id
		GROUP BY pt.playlist_id
	) agg ON agg.playlist_id = p.id`

// ListPlaylists returns the user's own playlists plus everyone's public ones, by name.
func (s *Store) ListPlaylists(ctx context.Context, userID string) ([]model.Playlist, error) {
	pls := []model.Playlist{}
	err := s.db.R.SelectContext(ctx, &pls, playlistSelect+` WHERE p.owner_id = ? OR p.public = 1
		ORDER BY p.name COLLATE NOCASE, p.id`, userID)
	if err != nil {
		return nil, err
	}
	for i := range pls {
		fillPlaylist(&pls[i])
	}
	return pls, nil
}

// GetPlaylist returns a playlist (no visibility check; callers enforce owner/public).
func (s *Store) GetPlaylist(ctx context.Context, id string) (*model.Playlist, error) {
	var p model.Playlist
	if err := s.db.R.GetContext(ctx, &p, playlistSelect+` WHERE p.id = ?`, id); err != nil {
		return nil, notFound(err)
	}
	fillPlaylist(&p)
	return &p, nil
}

// PlaylistTracks returns the playlist's non-missing tracks in playlist order (duplicates
// kept). Indexes into this list are what RemovePlaylistPositions expects.
func (s *Store) PlaylistTracks(ctx context.Context, id, userID string) ([]model.Track, error) {
	var tracks []model.Track
	q := trackSelect(`playlist_tracks pt JOIN tracks t ON t.id = pt.track_id AND t.missing = 0`, false) +
		` WHERE pt.playlist_id = ? ORDER BY pt.position`
	if err := s.db.R.SelectContext(ctx, &tracks, q, userID, id); err != nil {
		return nil, err
	}
	return fillTracks(tracks), nil
}

// existingTrackIDs filters ids down to those present in the tracks table (order and
// duplicates preserved).
func existingTrackIDs(ctx context.Context, tx *sqlx.Tx, ids []string) ([]string, error) {
	known := map[string]bool{}
	for _, chunk := range chunks(dedupe(ids)) {
		var got []string
		if err := tx.SelectContext(ctx, &got, `SELECT id FROM tracks WHERE id IN (`+placeholders(len(chunk))+`)`, anys(chunk)...); err != nil {
			return nil, err
		}
		for _, id := range got {
			known[id] = true
		}
	}
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		if known[id] {
			out = append(out, id)
		}
	}
	return out, nil
}

func insertPlaylistTracks(ctx context.Context, tx *sqlx.Tx, playlistID string, start int, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	stmt, err := tx.PreparexContext(ctx, `INSERT INTO playlist_tracks (playlist_id, position, track_id) VALUES (?, ?, ?)`)
	if err != nil {
		return err
	}
	defer func() { _ = stmt.Close() }()
	for i, id := range ids {
		if _, err := stmt.ExecContext(ctx, playlistID, start+i, id); err != nil {
			return err
		}
	}
	return nil
}

func touchPlaylist(ctx context.Context, tx *sqlx.Tx, id string) error {
	now := util.NowMs()
	return requireAffected(tx.ExecContext(ctx, `UPDATE playlists SET updated_at = `+bumpVersion+` WHERE id = ?`, now, now, id))
}

// CreatePlaylist inserts p (ID and timestamps set when empty) with the given tracks
// (unknown track ids are dropped).
func (s *Store) CreatePlaylist(ctx context.Context, p *model.Playlist, trackIDs []string) error {
	if p.ID == "" {
		p.ID = util.NewID()
	}
	now := util.NowMs()
	if p.CreatedAt == 0 {
		p.CreatedAt = now
	}
	p.UpdatedAt = now
	err := s.db.Tx(ctx, func(tx *sqlx.Tx) error {
		if _, err := tx.ExecContext(ctx, `INSERT INTO playlists (id, name, comment, owner_id, public, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?)`, p.ID, p.Name, p.Comment, p.OwnerID, p.Public, p.CreatedAt, p.UpdatedAt); err != nil {
			return constraint(err, "playlist")
		}
		ids, err := existingTrackIDs(ctx, tx, trackIDs)
		if err != nil {
			return err
		}
		return insertPlaylistTracks(ctx, tx, p.ID, 0, ids)
	})
	if err != nil {
		return fmt.Errorf("creating playlist: %w", err)
	}
	fillPlaylist(p)
	return nil
}

// UpdatePlaylist saves name, comment and public.
func (s *Store) UpdatePlaylist(ctx context.Context, p *model.Playlist) error {
	now := util.NowMs()
	return requireAffected(s.db.W.ExecContext(ctx, `UPDATE playlists SET name = ?, comment = ?, public = ?,
		updated_at = `+bumpVersion+` WHERE id = ?`, p.Name, p.Comment, p.Public, now, now, p.ID))
}

// DeletePlaylist removes a playlist and its entries.
func (s *Store) DeletePlaylist(ctx context.Context, id string) error {
	return requireAffected(s.db.W.ExecContext(ctx, `DELETE FROM playlists WHERE id = ?`, id))
}

// SetPlaylistTracks replaces the playlist's visible order with trackIDs (unknown ids
// dropped). Entries whose track is currently missing are invisible to clients, so they
// are preserved: each stays right after the visible entry it followed before (or at the
// start); if that entry was removed, it moves to the end. Entries of purged tracks are dropped.
func (s *Store) SetPlaylistTracks(ctx context.Context, id string, trackIDs []string) error {
	return s.db.Tx(ctx, func(tx *sqlx.Tx) error {
		if err := touchPlaylist(ctx, tx, id); err != nil {
			return err
		}
		var old []playlistEntry
		if err := tx.SelectContext(ctx, &old, `SELECT pt.track_id, t.missing
			FROM playlist_tracks pt JOIN tracks t ON t.id = pt.track_id
			WHERE pt.playlist_id = ? ORDER BY pt.position`, id); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM playlist_tracks WHERE playlist_id = ?`, id); err != nil {
			return err
		}
		ids, err := existingTrackIDs(ctx, tx, trackIDs)
		if err != nil {
			return err
		}
		return insertPlaylistTracks(ctx, tx, id, 0, mergeHiddenEntries(old, ids))
	})
}

type playlistEntry struct {
	TrackID string `db:"track_id"`
	Missing bool   `db:"missing"`
}

// mergeHiddenEntries re-inserts the hidden (missing) entries of old into the new visible
// order. Hidden entries are anchored to the visible entry preceding them in old (the n-th
// occurrence of a track id anchors to the n-th occurrence in visible).
func mergeHiddenEntries(old []playlistEntry, visible []string) []string {
	type anchor struct {
		id  string
		occ int
	}
	var lead []string
	after := map[anchor][]string{}
	var order []anchor
	seen := map[string]int{}
	var cur *anchor
	for _, e := range old {
		if !e.Missing {
			seen[e.TrackID]++
			cur = &anchor{e.TrackID, seen[e.TrackID]}
			continue
		}
		if cur == nil {
			lead = append(lead, e.TrackID)
			continue
		}
		if _, ok := after[*cur]; !ok {
			order = append(order, *cur)
		}
		after[*cur] = append(after[*cur], e.TrackID)
	}
	out := make([]string, 0, len(visible)+len(old))
	out = append(out, lead...)
	seen = map[string]int{}
	for _, id := range visible {
		out = append(out, id)
		seen[id]++
		k := anchor{id, seen[id]}
		if h, ok := after[k]; ok {
			out = append(out, h...)
			delete(after, k)
		}
	}
	for _, k := range order {
		out = append(out, after[k]...)
	}
	return out
}

// AppendPlaylistTracks appends tracks at the end (unknown ids dropped).
func (s *Store) AppendPlaylistTracks(ctx context.Context, id string, trackIDs []string) error {
	return s.db.Tx(ctx, func(tx *sqlx.Tx) error {
		if err := touchPlaylist(ctx, tx, id); err != nil {
			return err
		}
		var next int
		if err := tx.GetContext(ctx, &next, `SELECT COALESCE(MAX(position) + 1, 0) FROM playlist_tracks WHERE playlist_id = ?`, id); err != nil {
			return err
		}
		ids, err := existingTrackIDs(ctx, tx, trackIDs)
		if err != nil {
			return err
		}
		return insertPlaylistTracks(ctx, tx, id, next, ids)
	})
}

// RemovePlaylistPositions removes entries by their index in the visible track list (as
// returned by PlaylistTracks / Subsonic getPlaylist, i.e. skipping missing tracks), then
// renumbers all positions contiguously. Out-of-range indexes are ignored.
func (s *Store) RemovePlaylistPositions(ctx context.Context, id string, positions []int) error {
	return s.db.Tx(ctx, func(tx *sqlx.Tx) error {
		if err := touchPlaylist(ctx, tx, id); err != nil {
			return err
		}
		var visible []int
		if err := tx.SelectContext(ctx, &visible, `SELECT pt.position FROM playlist_tracks pt
			JOIN tracks t ON t.id = pt.track_id AND t.missing = 0
			WHERE pt.playlist_id = ? ORDER BY pt.position`, id); err != nil {
			return err
		}
		remove := map[int]bool{}
		for _, idx := range positions {
			if idx >= 0 && idx < len(visible) {
				remove[visible[idx]] = true
			}
		}
		if len(remove) == 0 {
			return nil
		}
		var all []struct {
			Position int    `db:"position"`
			TrackID  string `db:"track_id"`
		}
		if err := tx.SelectContext(ctx, &all, `SELECT position, track_id FROM playlist_tracks WHERE playlist_id = ? ORDER BY position`, id); err != nil {
			return err
		}
		sort.SliceStable(all, func(i, j int) bool { return all[i].Position < all[j].Position })
		keep := make([]string, 0, len(all))
		for _, e := range all {
			if !remove[e.Position] {
				keep = append(keep, e.TrackID)
			}
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM playlist_tracks WHERE playlist_id = ?`, id); err != nil {
			return err
		}
		return insertPlaylistTracks(ctx, tx, id, 0, keep)
	})
}
