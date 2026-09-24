package store

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"

	"rainy/internal/model"
	"rainy/internal/util"
)

// ---- play queue

// GetPlayQueue returns the user's saved queue; an empty queue (not an error) when none
// has been saved.
func (s *Store) GetPlayQueue(ctx context.Context, userID string) (*model.PlayQueue, error) {
	var row struct {
		TrackIDs   string `db:"track_ids"`
		CurrentID  string `db:"current_id"`
		PositionMs int64  `db:"position_ms"`
		ChangedBy  string `db:"changed_by"`
		UpdatedAt  int64  `db:"updated_at"`
		Index      int    `db:"current_index"`
	}
	err := s.db.R.GetContext(ctx, &row, `SELECT track_ids, current_id, position_ms, changed_by, updated_at, current_index
		FROM play_queues WHERE user_id = ?`, userID)
	if err != nil {
		if notFound(err) == ErrNotFound {
			return &model.PlayQueue{TrackIDs: []string{}}, nil
		}
		return nil, err
	}
	q := &model.PlayQueue{CurrentID: row.CurrentID, PositionMs: row.PositionMs, ChangedBy: row.ChangedBy, UpdatedAt: row.UpdatedAt,
		CurrentIndex: row.Index}
	if err := json.Unmarshal([]byte(row.TrackIDs), &q.TrackIDs); err != nil || q.TrackIDs == nil {
		q.TrackIDs = []string{}
	}
	q.CurrentIndex = q.CurrentPos()
	return q, nil
}

// SavePlayQueue replaces the user's queue (UpdatedAt is set to now).
func (s *Store) SavePlayQueue(ctx context.Context, userID string, q *model.PlayQueue) error {
	ids := q.TrackIDs
	if ids == nil {
		ids = []string{}
	}
	b, err := json.Marshal(ids)
	if err != nil {
		return err
	}
	q.UpdatedAt = util.NowMs()
	q.CurrentIndex = q.CurrentPos()
	_, err = s.db.W.ExecContext(ctx, `INSERT INTO play_queues (user_id, track_ids, current_id, position_ms, changed_by, updated_at, current_index)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (user_id) DO UPDATE SET track_ids = excluded.track_ids, current_id = excluded.current_id,
			position_ms = excluded.position_ms, changed_by = excluded.changed_by, updated_at = excluded.updated_at,
			current_index = excluded.current_index`,
		userID, string(b), q.CurrentID, q.PositionMs, q.ChangedBy, q.UpdatedAt, q.CurrentIndex)
	return err
}

// ---- bookmarks

// ListBookmarks returns the user's bookmarks, most recently updated first.
func (s *Store) ListBookmarks(ctx context.Context, userID string) ([]model.Bookmark, error) {
	bms := []model.Bookmark{}
	err := s.db.R.SelectContext(ctx, &bms, `SELECT track_id, position_ms, comment, created_at, updated_at
		FROM bookmarks WHERE user_id = ? ORDER BY updated_at DESC, track_id`, userID)
	return bms, err
}

// UpsertBookmark creates or updates the user's bookmark for b.TrackID.
func (s *Store) UpsertBookmark(ctx context.Context, userID string, b *model.Bookmark) error {
	now := util.NowMs()
	if b.CreatedAt == 0 {
		b.CreatedAt = now
	}
	b.UpdatedAt = now
	_, err := s.db.W.ExecContext(ctx, `INSERT INTO bookmarks (user_id, track_id, position_ms, comment, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT (user_id, track_id) DO UPDATE SET position_ms = excluded.position_ms, comment = excluded.comment,
			updated_at = excluded.updated_at`, userID, b.TrackID, b.PositionMs, b.Comment, b.CreatedAt, b.UpdatedAt)
	return err
}

// DeleteBookmark removes a bookmark (ErrNotFound if none).
func (s *Store) DeleteBookmark(ctx context.Context, userID, trackID string) error {
	return requireAffected(s.db.W.ExecContext(ctx, `DELETE FROM bookmarks WHERE user_id = ? AND track_id = ?`, userID, trackID))
}

// ---- radio

const radioCols = `id, name, stream_url, homepage_url, created_at, updated_at`

// ListRadioStations returns all stations by name.
func (s *Store) ListRadioStations(ctx context.Context) ([]model.RadioStation, error) {
	out := []model.RadioStation{}
	err := s.db.R.SelectContext(ctx, &out, `SELECT `+radioCols+` FROM radio_stations ORDER BY name COLLATE NOCASE, id`)
	return out, err
}

// GetRadioStation returns one station.
func (s *Store) GetRadioStation(ctx context.Context, id string) (*model.RadioStation, error) {
	var r model.RadioStation
	if err := s.db.R.GetContext(ctx, &r, `SELECT `+radioCols+` FROM radio_stations WHERE id = ?`, id); err != nil {
		return nil, notFound(err)
	}
	return &r, nil
}

// CreateRadioStation inserts r (ID and timestamps set when empty).
func (s *Store) CreateRadioStation(ctx context.Context, r *model.RadioStation) error {
	if r.ID == "" {
		r.ID = util.NewID()
	}
	now := util.NowMs()
	if r.CreatedAt == 0 {
		r.CreatedAt = now
	}
	r.UpdatedAt = now
	_, err := s.db.W.ExecContext(ctx, `INSERT INTO radio_stations (`+radioCols+`) VALUES (?, ?, ?, ?, ?, ?)`,
		r.ID, r.Name, r.StreamURL, r.HomepageURL, r.CreatedAt, r.UpdatedAt)
	return constraint(err, "radio station")
}

// UpdateRadioStation saves name and URLs.
func (s *Store) UpdateRadioStation(ctx context.Context, r *model.RadioStation) error {
	r.UpdatedAt = util.NowMs()
	return requireAffected(s.db.W.ExecContext(ctx, `UPDATE radio_stations SET name = ?, stream_url = ?, homepage_url = ?, updated_at = ?
		WHERE id = ?`, r.Name, r.StreamURL, r.HomepageURL, r.UpdatedAt, r.ID))
}

// DeleteRadioStation removes a station.
func (s *Store) DeleteRadioStation(ctx context.Context, id string) error {
	return requireAffected(s.db.W.ExecContext(ctx, `DELETE FROM radio_stations WHERE id = ?`, id))
}

// ---- settings

// GetSettings returns the stored settings merged over defaults: each settings row is one
// field (key = JSON field name, value = JSON value); unknown keys and undecodable values
// are ignored.
func (s *Store) GetSettings(ctx context.Context, defaults model.Settings) (model.Settings, error) {
	var rows []struct {
		Key   string `db:"key"`
		Value string `db:"value"`
	}
	if err := s.db.R.SelectContext(ctx, &rows, `SELECT key, value FROM settings`); err != nil {
		return defaults, fmt.Errorf("reading settings: %w", err)
	}
	if len(rows) == 0 {
		return defaults, nil
	}
	base, err := json.Marshal(defaults)
	if err != nil {
		return defaults, err
	}
	merged := map[string]json.RawMessage{}
	if err := json.Unmarshal(base, &merged); err != nil {
		return defaults, err
	}
	for _, r := range rows {
		if _, known := merged[r.Key]; known && json.Valid([]byte(r.Value)) {
			// Validate the single field against the struct so a bad value cannot
			// poison the whole object.
			probe := defaults
			one, _ := json.Marshal(map[string]json.RawMessage{r.Key: json.RawMessage(r.Value)})
			if json.Unmarshal(one, &probe) == nil {
				merged[r.Key] = json.RawMessage(r.Value)
			}
		}
	}
	b, err := json.Marshal(merged)
	if err != nil {
		return defaults, err
	}
	out := defaults
	if err := json.Unmarshal(b, &out); err != nil {
		return defaults, err
	}
	return out, nil
}

// SaveSettings stores every field of set.
func (s *Store) SaveSettings(ctx context.Context, set model.Settings) error {
	b, err := json.Marshal(set)
	if err != nil {
		return err
	}
	fields := map[string]json.RawMessage{}
	if err := json.Unmarshal(b, &fields); err != nil {
		return err
	}
	return s.db.Tx(ctx, func(tx *sqlx.Tx) error {
		for k, v := range fields {
			if _, err := tx.ExecContext(ctx, `INSERT INTO settings (key, value) VALUES (?, ?)
				ON CONFLICT (key) DO UPDATE SET value = excluded.value`, k, string(v)); err != nil {
				return err
			}
		}
		return nil
	})
}

// ---- edit log

// AddEditLog appends an entry (ID and CreatedAt are set; Details defaults to {}).
func (s *Store) AddEditLog(ctx context.Context, e *model.EditLogEntry) error {
	if e.CreatedAt == 0 {
		e.CreatedAt = util.NowMs()
	}
	details := "{}"
	if len(e.Details) > 0 && json.Valid(e.Details) {
		details = string(e.Details)
	}
	res, err := s.db.W.ExecContext(ctx, `INSERT INTO edit_log (user_id, username, action, track_id, path, details, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`, e.UserID, e.Username, e.Action, e.TrackID, e.Path, details, e.CreatedAt)
	if err != nil {
		return fmt.Errorf("adding edit log: %w", err)
	}
	e.ID, err = res.LastInsertId()
	e.Details = json.RawMessage(details)
	return err
}

// ListEditLog returns entries newest first, optionally only for one track, plus the total.
func (s *Store) ListEditLog(ctx context.Context, trackID string, offset, limit int) ([]model.EditLogEntry, int, error) {
	w := &where{}
	if trackID != "" {
		w.add("track_id = ?", trackID)
	}
	var total int
	if err := s.db.R.GetContext(ctx, &total, `SELECT COUNT(*) FROM edit_log`+w.sql(), w.args...); err != nil {
		return nil, 0, err
	}
	var rows []struct {
		model.EditLogEntry
		DetailsText string `db:"details_text"`
	}
	err := s.db.R.SelectContext(ctx, &rows, `SELECT id, user_id, username, action, track_id, path, details AS details_text, created_at
		FROM edit_log`+w.sql()+` ORDER BY created_at DESC, id DESC`+limitClause(offset, limit), w.args...)
	if err != nil {
		return nil, 0, err
	}
	out := make([]model.EditLogEntry, len(rows))
	for i, r := range rows {
		out[i] = r.EditLogEntry
		out[i].Details = json.RawMessage(r.DetailsText)
	}
	return out, total, nil
}

// ---- trash

const trashCols = `id, library_id, original_path, trash_path, size, title, artist, album, track_id, deleted_by, deleted_at`

// AddTrash records a trashed file (ID and DeletedAt set when empty).
func (s *Store) AddTrash(ctx context.Context, e *model.TrashEntry) error {
	if e.ID == "" {
		e.ID = util.NewID()
	}
	if e.DeletedAt == 0 {
		e.DeletedAt = util.NowMs()
	}
	_, err := s.db.W.ExecContext(ctx, `INSERT INTO trash (`+trashCols+`) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		e.ID, e.LibraryID, e.OriginalPath, e.TrashPath, e.Size, e.Title, e.Artist, e.Album, e.TrackID, e.DeletedBy, e.DeletedAt)
	return constraint(err, "trash entry")
}

// ListTrash returns trash entries, newest first.
func (s *Store) ListTrash(ctx context.Context) ([]model.TrashEntry, error) {
	out := []model.TrashEntry{}
	err := s.db.R.SelectContext(ctx, &out, `SELECT `+trashCols+` FROM trash ORDER BY deleted_at DESC, id`)
	return out, err
}

// GetTrash returns one trash entry.
func (s *Store) GetTrash(ctx context.Context, id string) (*model.TrashEntry, error) {
	var e model.TrashEntry
	if err := s.db.R.GetContext(ctx, &e, `SELECT `+trashCols+` FROM trash WHERE id = ?`, id); err != nil {
		return nil, notFound(err)
	}
	return &e, nil
}

// DeleteTrash removes a trash entry row (not the file).
func (s *Store) DeleteTrash(ctx context.Context, id string) error {
	return requireAffected(s.db.W.ExecContext(ctx, `DELETE FROM trash WHERE id = ?`, id))
}

// ---- stats

// FormatStat counts non-missing tracks per file suffix.
type FormatStat struct {
	Suffix string `db:"suffix" json:"suffix"`
	Count  int    `db:"count" json:"count"`
	Size   int64  `db:"size" json:"size"`
}

// LibraryStats summarises the library (the JSON shape of GET /api/admin/stats).
type LibraryStats struct {
	Tracks          int          `json:"tracks"`
	Albums          int          `json:"albums"`
	Artists         int          `json:"artists"`
	Genres          int          `json:"genres"`
	Playlists       int          `json:"playlists"`
	Users           int          `json:"users"`
	MissingTracks   int          `json:"missingTracks"`
	TotalDuration   float64      `json:"totalDuration"`
	TotalSize       int64        `json:"totalSize"`
	Formats         []FormatStat `json:"formats"`
	PlaysLast30Days int          `json:"playsLast30Days"`
}

// Stats computes library statistics.
func (s *Store) Stats(ctx context.Context) (*LibraryStats, error) {
	st := &LibraryStats{Formats: []FormatStat{}}
	var agg struct {
		Tracks   int     `db:"tracks"`
		Missing  int     `db:"missing"`
		Duration float64 `db:"duration"`
		Size     int64   `db:"size"`
	}
	if err := s.db.R.GetContext(ctx, &agg, `SELECT
			COALESCE(SUM(missing = 0), 0) AS tracks, COALESCE(SUM(missing = 1), 0) AS missing,
			COALESCE(SUM(CASE WHEN missing = 0 THEN duration END), 0) AS duration,
			COALESCE(SUM(CASE WHEN missing = 0 THEN size END), 0) AS size
		FROM tracks`); err != nil {
		return nil, err
	}
	st.Tracks, st.MissingTracks, st.TotalDuration, st.TotalSize = agg.Tracks, agg.Missing, agg.Duration, agg.Size
	counts := []struct {
		dst *int
		sql string
		arg []any
	}{
		{&st.Albums, `SELECT COUNT(*) FROM albums`, nil},
		{&st.Artists, `SELECT COUNT(*) FROM artists WHERE album_count > 0 OR song_count > 0`, nil},
		{&st.Genres, `SELECT COUNT(DISTINCT tg.genre_id) FROM track_genres tg JOIN tracks t ON t.id = tg.track_id AND t.missing = 0`, nil},
		{&st.Playlists, `SELECT COUNT(*) FROM playlists`, nil},
		{&st.Users, `SELECT COUNT(*) FROM users`, nil},
		{&st.PlaysLast30Days, `SELECT COUNT(*) FROM play_history WHERE played_at >= ?`, []any{time.Now().Add(-30 * 24 * time.Hour).UnixMilli()}},
	}
	for _, c := range counts {
		if err := s.db.R.GetContext(ctx, c.dst, c.sql, c.arg...); err != nil {
			return nil, err
		}
	}
	if err := s.db.R.SelectContext(ctx, &st.Formats, `SELECT suffix, COUNT(*) AS count, COALESCE(SUM(size), 0) AS size
		FROM tracks WHERE missing = 0 GROUP BY suffix ORDER BY count DESC, suffix`); err != nil {
		return nil, err
	}
	if st.Formats == nil {
		st.Formats = []FormatStat{}
	}
	return st, nil
}
