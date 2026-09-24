package store

import (
	"context"
	"fmt"

	"github.com/jmoiron/sqlx"

	"rainy/internal/model"
	"rainy/internal/util"
)

const libraryCols = `id, name, path, created_at, updated_at, last_scan_at`

// ListLibraries returns all libraries ordered by id.
func (s *Store) ListLibraries(ctx context.Context) ([]model.Library, error) {
	libs := []model.Library{}
	err := s.db.R.SelectContext(ctx, &libs, `SELECT `+libraryCols+` FROM libraries ORDER BY id`)
	return libs, err
}

// GetLibrary returns a library by id.
func (s *Store) GetLibrary(ctx context.Context, id int64) (*model.Library, error) {
	var l model.Library
	if err := s.db.R.GetContext(ctx, &l, `SELECT `+libraryCols+` FROM libraries WHERE id = ?`, id); err != nil {
		return nil, notFound(err)
	}
	return &l, nil
}

// CreateLibrary inserts l and sets l.ID. A duplicate path yields ErrConflict.
func (s *Store) CreateLibrary(ctx context.Context, l *model.Library) error {
	now := util.NowMs()
	if l.CreatedAt == 0 {
		l.CreatedAt = now
	}
	l.UpdatedAt = now
	res, err := s.db.W.ExecContext(ctx, `INSERT INTO libraries (name, path, created_at, updated_at, last_scan_at)
		VALUES (?, ?, ?, ?, ?)`, l.Name, l.Path, l.CreatedAt, l.UpdatedAt, l.LastScanAt)
	if err != nil {
		return fmt.Errorf("creating library: %w", constraint(err, "library path"))
	}
	l.ID, err = res.LastInsertId()
	return err
}

// UpdateLibrary saves name and path.
func (s *Store) UpdateLibrary(ctx context.Context, l *model.Library) error {
	l.UpdatedAt = util.NowMs()
	res, err := s.db.W.ExecContext(ctx, `UPDATE libraries SET name = ?, path = ?, updated_at = ? WHERE id = ?`,
		l.Name, l.Path, l.UpdatedAt, l.ID)
	if err != nil {
		return fmt.Errorf("updating library: %w", constraint(err, "library path"))
	}
	return requireAffected(res, nil)
}

// DeleteLibrary removes a library and all its track rows (never files), including the
// tracks' annotations and bookmarks, then refreshes every aggregate.
func (s *Store) DeleteLibrary(ctx context.Context, id int64) error {
	err := s.db.Tx(ctx, func(tx *sqlx.Tx) error {
		const trackIDs = `SELECT id FROM tracks WHERE library_id = ?`
		if _, err := tx.ExecContext(ctx, `DELETE FROM annotations WHERE item_type = 'track' AND item_id IN (`+trackIDs+`)`, id); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM bookmarks WHERE track_id IN (`+trackIDs+`)`, id); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM tracks WHERE library_id = ?`, id); err != nil {
			return err
		}
		return requireAffected(tx.ExecContext(ctx, `DELETE FROM libraries WHERE id = ?`, id))
	})
	if err != nil {
		return err
	}
	return s.RefreshAll(ctx)
}

// SetLibraryScanned records the completion time of a scan.
func (s *Store) SetLibraryScanned(ctx context.Context, id int64, at int64) error {
	return requireAffected(s.db.W.ExecContext(ctx, `UPDATE libraries SET last_scan_at = ? WHERE id = ?`, at, id))
}

// LibraryTrackCounts returns the number of non-missing tracks per library id.
func (s *Store) LibraryTrackCounts(ctx context.Context) (map[int64]int, error) {
	var rows []struct {
		LibraryID int64 `db:"library_id"`
		N         int   `db:"n"`
	}
	if err := s.db.R.SelectContext(ctx, &rows, `SELECT library_id, COUNT(*) AS n FROM tracks WHERE missing = 0 GROUP BY library_id`); err != nil {
		return nil, err
	}
	out := make(map[int64]int, len(rows))
	for _, r := range rows {
		out[r.LibraryID] = r.N
	}
	return out, nil
}
