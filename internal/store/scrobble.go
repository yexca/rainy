package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jmoiron/sqlx"

	"rainy/internal/model"
	"rainy/internal/util"
)

// Scrobbling accounts and the queue of plays waiting to be sent (docs/architecture/contract.md
// §5.16).

const scrobbleAccountCols = `user_id, service, username, credential_enc, enabled, last_error, last_error_at,
	last_sent_at, created_at, updated_at`

// ListScrobbleAccounts returns the user's linked accounts ordered by service.
func (s *Store) ListScrobbleAccounts(ctx context.Context, userID string) ([]model.ScrobbleAccount, error) {
	out := []model.ScrobbleAccount{}
	err := s.db.R.SelectContext(ctx, &out, `SELECT `+scrobbleAccountCols+` FROM scrobble_accounts WHERE user_id = ? ORDER BY service`, userID)
	return out, err
}

// GetScrobbleAccount returns one linked account or ErrNotFound.
func (s *Store) GetScrobbleAccount(ctx context.Context, userID, service string) (*model.ScrobbleAccount, error) {
	var a model.ScrobbleAccount
	err := s.db.R.GetContext(ctx, &a, `SELECT `+scrobbleAccountCols+` FROM scrobble_accounts WHERE user_id = ? AND service = ?`, userID, service)
	if err != nil {
		return nil, notFound(err)
	}
	return &a, nil
}

// SaveScrobbleAccount links (or re-links) an account: it stores the username and credential,
// enables scrobbling and clears the last error. Plays already queued for the account stay.
func (s *Store) SaveScrobbleAccount(ctx context.Context, a *model.ScrobbleAccount) error {
	now := util.NowMs()
	a.Enabled, a.LastError, a.LastErrorAt, a.UpdatedAt = true, "", 0, now
	if a.CreatedAt == 0 {
		a.CreatedAt = now
	}
	_, err := s.db.W.ExecContext(ctx, `INSERT INTO scrobble_accounts (user_id, service, username, credential_enc, enabled,
			last_error, last_error_at, last_sent_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, 1, '', 0, 0, ?, ?)
		ON CONFLICT (user_id, service) DO UPDATE SET username = excluded.username, credential_enc = excluded.credential_enc,
			enabled = 1, last_error = '', last_error_at = 0, updated_at = excluded.updated_at`,
		a.UserID, a.Service, a.Username, a.CredentialEnc, a.CreatedAt, a.UpdatedAt)
	return err
}

// SetScrobbleAccountEnabled pauses (false) or resumes scrobbling for an account. Pausing
// drops the plays waiting for it, so resuming never sends plays from the pause.
func (s *Store) SetScrobbleAccountEnabled(ctx context.Context, userID, service string, enabled bool) error {
	return s.db.Tx(ctx, func(tx *sqlx.Tx) error {
		res, err := tx.ExecContext(ctx, `UPDATE scrobble_accounts SET enabled = ?, updated_at = ? WHERE user_id = ? AND service = ?`,
			enabled, util.NowMs(), userID, service)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return ErrNotFound
		}
		if !enabled {
			_, err = tx.ExecContext(ctx, `DELETE FROM scrobble_queue WHERE user_id = ? AND service = ?`, userID, service)
		}
		return err
	})
}

// SetScrobbleAccountError records a failure (a revoked credential clears it so the user
// links the account again). An empty msg clears the error.
func (s *Store) SetScrobbleAccountError(ctx context.Context, userID, service, msg string, revoked bool) error {
	at := int64(0)
	if msg != "" {
		at = util.NowMs()
	}
	q := `UPDATE scrobble_accounts SET last_error = ?, last_error_at = ? WHERE user_id = ? AND service = ?`
	if revoked {
		q = `UPDATE scrobble_accounts SET last_error = ?, last_error_at = ?, credential_enc = '' WHERE user_id = ? AND service = ?`
	}
	_, err := s.db.W.ExecContext(ctx, q, msg, at, userID, service)
	return err
}

// MarkScrobbleAccountSent records a successful submission and clears the last error.
func (s *Store) MarkScrobbleAccountSent(ctx context.Context, userID, service string, at int64) error {
	_, err := s.db.W.ExecContext(ctx, `UPDATE scrobble_accounts SET last_sent_at = MAX(last_sent_at, ?), last_error = '', last_error_at = 0
		WHERE user_id = ? AND service = ?`, at, userID, service)
	return err
}

// DeleteScrobbleAccount unlinks an account and drops its queued plays.
func (s *Store) DeleteScrobbleAccount(ctx context.Context, userID, service string) error {
	return s.db.Tx(ctx, func(tx *sqlx.Tx) error {
		if _, err := tx.ExecContext(ctx, `DELETE FROM scrobble_queue WHERE user_id = ? AND service = ?`, userID, service); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, `DELETE FROM scrobble_accounts WHERE user_id = ? AND service = ?`, userID, service)
		return err
	})
}

// CountScrobbleAccounts returns how many users linked the service.
func (s *Store) CountScrobbleAccounts(ctx context.Context, service string) (int, error) {
	var n int
	err := s.db.R.GetContext(ctx, &n, `SELECT COUNT(*) FROM scrobble_accounts WHERE service = ? AND credential_enc != ''`, service)
	return n, err
}

// EnqueueScrobble queues a play for every enabled, linked account of the user in services.
// It returns the services it queued for.
func (s *Store) EnqueueScrobble(ctx context.Context, e model.QueuedScrobble, services []string) ([]string, error) {
	if len(services) == 0 {
		return nil, nil
	}
	var queued []string
	err := s.db.Tx(ctx, func(tx *sqlx.Tx) error {
		queued = queued[:0]
		var linked []string
		q, args, err := sqlx.In(`SELECT service FROM scrobble_accounts
			WHERE user_id = ? AND enabled = 1 AND credential_enc != '' AND service IN (?) ORDER BY service`, e.UserID, services)
		if err != nil {
			return err
		}
		if err := tx.SelectContext(ctx, &linked, tx.Rebind(q), args...); err != nil {
			return err
		}
		now := util.NowMs()
		for _, svc := range linked {
			if _, err := tx.ExecContext(ctx, `INSERT INTO scrobble_queue (user_id, service, track_id, title, artist, album,
					album_artist, track_number, duration, mbz_track_id, played_at, attempts, next_attempt_at, last_error, created_at)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 0, 0, '', ?)`,
				e.UserID, svc, e.TrackID, e.Title, e.Artist, e.Album, e.AlbumArtist, e.TrackNumber, e.Duration,
				e.MbzTrackID, e.PlayedAt, now); err != nil {
				return err
			}
			queued = append(queued, svc)
		}
		return nil
	})
	return queued, err
}

// ScrobbleDueUsers returns the users with queued plays for the service that are due at now,
// oldest waiting first.
func (s *Store) ScrobbleDueUsers(ctx context.Context, service string, now int64) ([]string, error) {
	var ids []string
	err := s.db.R.SelectContext(ctx, &ids, `SELECT user_id FROM scrobble_queue WHERE service = ? AND next_attempt_at <= ?
		GROUP BY user_id ORDER BY MIN(id)`, service, now)
	return ids, err
}

// DueScrobbles returns up to limit of the user's due plays for the service, oldest first.
func (s *Store) DueScrobbles(ctx context.Context, userID, service string, now int64, limit int) ([]model.QueuedScrobble, error) {
	out := []model.QueuedScrobble{}
	err := s.db.R.SelectContext(ctx, &out, `SELECT id, user_id, service, track_id, title, artist, album, album_artist,
			track_number, duration, mbz_track_id, played_at, attempts, next_attempt_at, last_error, created_at
		FROM scrobble_queue WHERE user_id = ? AND service = ? AND next_attempt_at <= ?
		ORDER BY played_at, id LIMIT ?`, userID, service, now, limit)
	return out, err
}

// DeleteScrobbles removes queued plays (sent, refused or expired).
func (s *Store) DeleteScrobbles(ctx context.Context, ids []int64) error {
	return s.db.Tx(ctx, func(tx *sqlx.Tx) error {
		for _, chunk := range chunks(ids) {
			if _, err := tx.ExecContext(ctx, `DELETE FROM scrobble_queue WHERE id IN (`+placeholders(len(chunk))+`)`, anys(chunk)...); err != nil {
				return err
			}
		}
		return nil
	})
}

// DeferScrobbles counts a failed attempt for queued plays and schedules the next one.
func (s *Store) DeferScrobbles(ctx context.Context, ids []int64, next int64, msg string) error {
	return s.db.Tx(ctx, func(tx *sqlx.Tx) error {
		for _, chunk := range chunks(ids) {
			args := append([]any{next, msg}, anys(chunk)...)
			if _, err := tx.ExecContext(ctx, `UPDATE scrobble_queue SET attempts = attempts + 1, next_attempt_at = ?, last_error = ?
				WHERE id IN (`+placeholders(len(chunk))+`)`, args...); err != nil {
				return err
			}
		}
		return nil
	})
}

// PurgeScrobblesBefore drops queued plays of the service played before `before` (too old
// for the service to accept) and returns how many were dropped.
func (s *Store) PurgeScrobblesBefore(ctx context.Context, service string, before int64) (int64, error) {
	res, err := s.db.W.ExecContext(ctx, `DELETE FROM scrobble_queue WHERE service = ? AND played_at < ?`, service, before)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// CountQueuedScrobbles returns how many of the user's plays wait for the service.
func (s *Store) CountQueuedScrobbles(ctx context.Context, userID, service string) (int, error) {
	var n int
	err := s.db.R.GetContext(ctx, &n, `SELECT COUNT(*) FROM scrobble_queue WHERE user_id = ? AND service = ?`, userID, service)
	return n, err
}

// ---- server-side values outside model.Settings

// Keys of GetValue / SetValue. They live in the settings table but are not part of
// model.Settings, so GET /api/admin/settings never returns them.
const (
	ValueLastfmAppKey     = "lastfm.apiKey"
	ValueLastfmSigningEnc = "lastfm.secretEnc" // encrypted with secret.key
)

// GetValue returns a stored string value ("" when unset).
func (s *Store) GetValue(ctx context.Context, key string) (string, error) {
	var raw string
	err := s.db.R.GetContext(ctx, &raw, `SELECT value FROM settings WHERE key = ?`, key)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("reading %s: %w", key, err)
	}
	var v string
	if json.Unmarshal([]byte(raw), &v) != nil {
		return "", nil
	}
	return v, nil
}

// SetValue stores a string value; "" deletes it.
func (s *Store) SetValue(ctx context.Context, key, value string) error {
	if value == "" {
		_, err := s.db.W.ExecContext(ctx, `DELETE FROM settings WHERE key = ?`, key)
		return err
	}
	b, err := json.Marshal(value)
	if err != nil {
		return err
	}
	_, err = s.db.W.ExecContext(ctx, `INSERT INTO settings (key, value) VALUES (?, ?)
		ON CONFLICT (key) DO UPDATE SET value = excluded.value`, key, string(b))
	return err
}
