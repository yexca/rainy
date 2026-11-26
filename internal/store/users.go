package store

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/jmoiron/sqlx"

	"rainy/internal/model"
	"rainy/internal/util"
)

const userCols = `id, username, display_name, email, password_enc, is_admin, can_manage, can_download,
	restrict_libraries, api_key_hash, created_at, updated_at, last_login_at, last_seen_at`

func (s *Store) getUser(ctx context.Context, cond string, arg any) (*model.User, error) {
	var u model.User
	if err := s.db.R.GetContext(ctx, &u, `SELECT `+userCols+` FROM users WHERE `+cond, arg); err != nil {
		return nil, notFound(err)
	}
	if err := s.loadUserLibraries(ctx, &u); err != nil {
		return nil, err
	}
	u.Fill()
	return &u, nil
}

// loadUserLibraries fills LibraryIDs of the restricted users among users.
func (s *Store) loadUserLibraries(ctx context.Context, users ...*model.User) error {
	byID := map[string]*model.User{}
	for _, u := range users {
		u.LibraryIDs = []int64{}
		if u.RestrictLibraries {
			byID[u.ID] = u
		}
	}
	if len(byID) == 0 {
		return nil
	}
	var rows []struct {
		UserID    string `db:"user_id"`
		LibraryID int64  `db:"library_id"`
	}
	q := `SELECT user_id, library_id FROM user_libraries ORDER BY user_id, library_id`
	var args []any
	if len(byID) == 1 {
		for id := range byID {
			q, args = `SELECT user_id, library_id FROM user_libraries WHERE user_id = ? ORDER BY library_id`, []any{id}
		}
	}
	if err := s.db.R.SelectContext(ctx, &rows, q, args...); err != nil {
		return fmt.Errorf("loading user libraries: %w", err)
	}
	for _, r := range rows {
		if u := byID[r.UserID]; u != nil {
			u.LibraryIDs = append(u.LibraryIDs, r.LibraryID)
		}
	}
	return nil
}

// saveUserLibraries replaces the granted libraries of u (after NormalizeLibraries). An
// unknown library id yields ErrInvalid.
func saveUserLibraries(ctx context.Context, tx *sqlx.Tx, u *model.User) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM user_libraries WHERE user_id = ?`, u.ID); err != nil {
		return err
	}
	for _, id := range u.LibraryIDs {
		var n int
		if err := tx.GetContext(ctx, &n, `SELECT COUNT(*) FROM libraries WHERE id = ?`, id); err != nil {
			return err
		}
		if n == 0 {
			return fmt.Errorf("%w: unknown library %d", ErrInvalid, id)
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO user_libraries (user_id, library_id) VALUES (?, ?)`, u.ID, id); err != nil {
			return err
		}
	}
	return nil
}

// CreateUser inserts u with its library access. ID (NewID) and timestamps are set when
// empty. A duplicate username (case-insensitive) yields ErrConflict, an unknown granted
// library ErrInvalid.
func (s *Store) CreateUser(ctx context.Context, u *model.User) error {
	if u.ID == "" {
		u.ID = util.NewID()
	}
	now := util.NowMs()
	if u.CreatedAt == 0 {
		u.CreatedAt = now
	}
	u.UpdatedAt = now
	u.LibraryIDs = normalizedLibraryIDs(u)
	err := s.db.Tx(ctx, func(tx *sqlx.Tx) error {
		if _, err := tx.ExecContext(ctx, `INSERT INTO users (`+userCols+`)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			u.ID, u.Username, u.DisplayName, u.Email, u.PasswordEnc, u.IsAdmin, u.CanManage, u.CanDownload,
			u.RestrictLibraries, u.APIKeyHash, u.CreatedAt, u.UpdatedAt, u.LastLoginAt, u.LastSeenAt); err != nil {
			return constraint(err, "username")
		}
		return saveUserLibraries(ctx, tx, u)
	})
	if err != nil {
		return fmt.Errorf("creating user: %w", err)
	}
	u.Fill()
	return nil
}

// normalizedLibraryIDs applies NormalizeLibraries to u and returns its sorted, distinct
// library ids.
func normalizedLibraryIDs(u *model.User) []int64 {
	u.NormalizeLibraries()
	ids := slices.Clone(u.LibraryIDs)
	slices.Sort(ids)
	return slices.Compact(ids)
}

// GetUser returns a user by id.
func (s *Store) GetUser(ctx context.Context, id string) (*model.User, error) {
	return s.getUser(ctx, "id = ?", id)
}

// GetUserByUsername returns a user by username (case-insensitive).
func (s *Store) GetUserByUsername(ctx context.Context, username string) (*model.User, error) {
	return s.getUser(ctx, "username = ?", username)
}

// GetUserByAPIKeyHash returns the user owning an API key hash.
func (s *Store) GetUserByAPIKeyHash(ctx context.Context, hash string) (*model.User, error) {
	if hash == "" {
		return nil, ErrNotFound
	}
	return s.getUser(ctx, "api_key_hash = ?", hash)
}

// ListUsers returns all users ordered by username.
func (s *Store) ListUsers(ctx context.Context) ([]model.User, error) {
	users := []model.User{}
	if err := s.db.R.SelectContext(ctx, &users, `SELECT `+userCols+` FROM users ORDER BY username COLLATE NOCASE, id`); err != nil {
		return nil, err
	}
	ptrs := make([]*model.User, len(users))
	for i := range users {
		ptrs[i] = &users[i]
	}
	if err := s.loadUserLibraries(ctx, ptrs...); err != nil {
		return nil, err
	}
	for i := range users {
		users[i].Fill()
	}
	return users, nil
}

// UpdateUser saves username, display name, email, the permission flags and the library
// access of u (not the password, API key or login timestamps). An unknown granted library
// yields ErrInvalid.
func (s *Store) UpdateUser(ctx context.Context, u *model.User) error {
	u.UpdatedAt = util.NowMs()
	u.LibraryIDs = normalizedLibraryIDs(u)
	err := s.db.Tx(ctx, func(tx *sqlx.Tx) error {
		res, err := tx.ExecContext(ctx, `UPDATE users SET username = ?, display_name = ?, email = ?,
			is_admin = ?, can_manage = ?, can_download = ?, restrict_libraries = ?, updated_at = ? WHERE id = ?`,
			u.Username, u.DisplayName, u.Email, u.IsAdmin, u.CanManage, u.CanDownload, u.RestrictLibraries, u.UpdatedAt, u.ID)
		if err != nil {
			return constraint(err, "username")
		}
		if err := requireAffected(res, nil); err != nil {
			return err
		}
		return saveUserLibraries(ctx, tx, u)
	})
	if err != nil && !errors.Is(err, ErrNotFound) {
		return fmt.Errorf("updating user: %w", err)
	}
	return err
}

// DeleteUser removes a user (sessions, annotations, playlists, queue and bookmarks cascade).
func (s *Store) DeleteUser(ctx context.Context, id string) error {
	return requireAffected(s.db.W.ExecContext(ctx, `DELETE FROM users WHERE id = ?`, id))
}

// CountUsers returns the number of users.
func (s *Store) CountUsers(ctx context.Context) (int, error) {
	var n int
	err := s.db.R.GetContext(ctx, &n, `SELECT COUNT(*) FROM users`)
	return n, err
}

// SetUserPassword stores a new encrypted password.
func (s *Store) SetUserPassword(ctx context.Context, id, passwordEnc string) error {
	return requireAffected(s.db.W.ExecContext(ctx,
		`UPDATE users SET password_enc = ?, updated_at = ? WHERE id = ?`, passwordEnc, util.NowMs(), id))
}

// SetUserAPIKeyHash sets (or clears, with nil) the user's API key hash.
func (s *Store) SetUserAPIKeyHash(ctx context.Context, id string, hash *string) error {
	res, err := s.db.W.ExecContext(ctx, `UPDATE users SET api_key_hash = ?, updated_at = ? WHERE id = ?`, hash, util.NowMs(), id)
	if err != nil {
		return constraint(err, "api key")
	}
	return requireAffected(res, nil)
}

// TouchUserLogin records a successful login (last_login_at and last_seen_at = now).
func (s *Store) TouchUserLogin(ctx context.Context, id string) error {
	now := util.NowMs()
	_, err := s.db.W.ExecContext(ctx, `UPDATE users SET last_login_at = ?, last_seen_at = ? WHERE id = ?`, now, now, id)
	return err
}

// TouchUserSeen sets last_seen_at = now.
func (s *Store) TouchUserSeen(ctx context.Context, id string) error {
	_, err := s.db.W.ExecContext(ctx, `UPDATE users SET last_seen_at = ? WHERE id = ?`, util.NowMs(), id)
	return err
}

// ---- sessions

const sessionCols = `token_hash, user_id, user_agent, ip, created_at, last_seen_at, expires_at`

// CreateSession inserts a session (CreatedAt/LastSeenAt default to now).
func (s *Store) CreateSession(ctx context.Context, sess *model.Session) error {
	now := util.NowMs()
	if sess.CreatedAt == 0 {
		sess.CreatedAt = now
	}
	if sess.LastSeenAt == 0 {
		sess.LastSeenAt = now
	}
	_, err := s.db.W.ExecContext(ctx, `INSERT INTO sessions (`+sessionCols+`) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		sess.TokenHash, sess.UserID, sess.UserAgent, sess.IP, sess.CreatedAt, sess.LastSeenAt, sess.ExpiresAt)
	if err != nil {
		return fmt.Errorf("creating session: %w", constraint(err, "session"))
	}
	return nil
}

// GetSession returns a session by token hash (expired sessions are returned too; the
// caller checks ExpiresAt).
func (s *Store) GetSession(ctx context.Context, tokenHash string) (*model.Session, error) {
	var sess model.Session
	if err := s.db.R.GetContext(ctx, &sess, `SELECT `+sessionCols+` FROM sessions WHERE token_hash = ?`, tokenHash); err != nil {
		return nil, notFound(err)
	}
	return &sess, nil
}

// TouchSession sets last_seen_at = now and a new expiry.
func (s *Store) TouchSession(ctx context.Context, tokenHash string, expiresAt int64) error {
	return s.TouchSessionAt(ctx, tokenHash, util.NowMs(), expiresAt)
}

// TouchSessionAt is TouchSession with an explicit last-seen time.
func (s *Store) TouchSessionAt(ctx context.Context, tokenHash string, seenAt, expiresAt int64) error {
	return requireAffected(s.db.W.ExecContext(ctx,
		`UPDATE sessions SET last_seen_at = ?, expires_at = ? WHERE token_hash = ?`, seenAt, expiresAt, tokenHash))
}

// DeleteSession removes a session (no error if it does not exist).
func (s *Store) DeleteSession(ctx context.Context, tokenHash string) error {
	_, err := s.db.W.ExecContext(ctx, `DELETE FROM sessions WHERE token_hash = ?`, tokenHash)
	return err
}

// DeleteUserSessions removes every session of a user.
func (s *Store) DeleteUserSessions(ctx context.Context, userID string) error {
	_, err := s.db.W.ExecContext(ctx, `DELETE FROM sessions WHERE user_id = ?`, userID)
	return err
}

// PurgeExpiredSessions deletes sessions whose expiry has passed.
func (s *Store) PurgeExpiredSessions(ctx context.Context) error {
	_, err := s.db.W.ExecContext(ctx, `DELETE FROM sessions WHERE expires_at < ?`, util.NowMs())
	return err
}
