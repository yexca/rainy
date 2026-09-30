package store

import (
	"context"
	"fmt"

	"github.com/jmoiron/sqlx"

	"rainy/internal/model"
	"rainy/internal/util"
)

// lx-music custom source scripts (table lx_sources, contract §5.15).

// lxSourceCols are the columns of model.LxSource without the script.
const lxSourceCols = `id, name, description, version, author, homepage, source_url, script_hash,
	length(CAST(script AS BLOB)) AS script_size, enabled, position, allow_update_alert, platforms,
	last_error, loaded_at, update_log, update_url, update_at, created_at, updated_at`

// ListLxSources returns every source by priority (without scripts).
func (s *Store) ListLxSources(ctx context.Context) ([]model.LxSource, error) {
	out := []model.LxSource{}
	err := s.db.R.SelectContext(ctx, &out, `SELECT `+lxSourceCols+` FROM lx_sources ORDER BY position, created_at, id`)
	return out, err
}

// GetLxSource returns one source including its script.
func (s *Store) GetLxSource(ctx context.Context, id string) (*model.LxSource, error) {
	var src model.LxSource
	if err := s.db.R.GetContext(ctx, &src, `SELECT `+lxSourceCols+`, script FROM lx_sources WHERE id = ?`, id); err != nil {
		return nil, notFound(err)
	}
	return &src, nil
}

// CreateLxSource inserts src at the end of the priority order (ID and timestamps set). A
// script that is already imported is ErrConflict.
func (s *Store) CreateLxSource(ctx context.Context, src *model.LxSource) error {
	if src.ID == "" {
		src.ID = util.NewID()
	}
	now := util.NowMs()
	src.CreatedAt, src.UpdatedAt = now, now
	if src.Platforms == "" {
		src.Platforms = "{}"
	}
	return s.db.Tx(ctx, func(tx *sqlx.Tx) error {
		if err := tx.GetContext(ctx, &src.Position, `SELECT COALESCE(MAX(position) + 1, 0) FROM lx_sources`); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, `INSERT INTO lx_sources (id, name, description, version, author, homepage,
			source_url, script, script_hash, enabled, position, allow_update_alert, platforms, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			src.ID, src.Name, src.Description, src.Version, src.Author, src.Homepage, src.SourceURL, src.Script,
			src.ScriptHash, src.Enabled, src.Position, src.AllowUpdateAlert, src.Platforms, src.CreatedAt, src.UpdatedAt)
		return constraint(err, "this script")
	})
}

// ReplaceLxSourceScript stores a new version of a source's script and header; what the old
// script reported (platforms, errors, update notice) is cleared.
func (s *Store) ReplaceLxSourceScript(ctx context.Context, src *model.LxSource) error {
	src.UpdatedAt = util.NowMs()
	err := requireAffected(s.db.W.ExecContext(ctx, `UPDATE lx_sources SET name = ?, description = ?, version = ?,
		author = ?, homepage = ?, script = ?, script_hash = ?, platforms = '{}', last_error = '', loaded_at = 0,
		update_log = '', update_url = '', update_at = 0, updated_at = ? WHERE id = ?`,
		src.Name, src.Description, src.Version, src.Author, src.Homepage, src.Script, src.ScriptHash, src.UpdatedAt, src.ID))
	return constraint(err, "this script")
}

// SetLxSourceFlags changes the enabled and update-notice switches (nil = unchanged).
func (s *Store) SetLxSourceFlags(ctx context.Context, id string, enabled, allowUpdateAlert *bool) error {
	return requireAffected(s.db.W.ExecContext(ctx, `UPDATE lx_sources SET
		enabled = COALESCE(?, enabled), allow_update_alert = COALESCE(?, allow_update_alert), updated_at = ?
		WHERE id = ?`, enabled, allowUpdateAlert, util.NowMs(), id))
}

// SetLxSourceLoaded records the outcome of starting a source: the platforms it provides
// (JSON) or the error.
func (s *Store) SetLxSourceLoaded(ctx context.Context, id, platforms, lastError string, at int64) error {
	if platforms == "" {
		platforms = "{}"
	}
	return requireAffected(s.db.W.ExecContext(ctx, `UPDATE lx_sources SET
		platforms = CASE WHEN ? = '' THEN ? ELSE platforms END, last_error = ?, loaded_at = ? WHERE id = ?`,
		lastError, platforms, lastError, at, id))
}

// SetLxSourceUpdateAlert stores the script's update notice.
func (s *Store) SetLxSourceUpdateAlert(ctx context.Context, id, log, url string, at int64) error {
	return requireAffected(s.db.W.ExecContext(ctx, `UPDATE lx_sources SET update_log = ?, update_url = ?, update_at = ?
		WHERE id = ?`, log, url, at, id))
}

// DeleteLxSource removes a source.
func (s *Store) DeleteLxSource(ctx context.Context, id string) error {
	return requireAffected(s.db.W.ExecContext(ctx, `DELETE FROM lx_sources WHERE id = ?`, id))
}

// ReorderLxSources sets the priority order; ids must list every source exactly once.
func (s *Store) ReorderLxSources(ctx context.Context, ids []string) error {
	return s.db.Tx(ctx, func(tx *sqlx.Tx) error {
		var existing []string
		if err := tx.SelectContext(ctx, &existing, `SELECT id FROM lx_sources`); err != nil {
			return err
		}
		known := map[string]bool{}
		for _, id := range existing {
			known[id] = true
		}
		seen := map[string]bool{}
		for _, id := range ids {
			if !known[id] || seen[id] {
				return fmt.Errorf("%w: unknown or repeated source id %q", ErrInvalid, id)
			}
			seen[id] = true
		}
		if len(ids) != len(existing) {
			return fmt.Errorf("%w: the order must list all %d sources", ErrInvalid, len(existing))
		}
		for i, id := range ids {
			if _, err := tx.ExecContext(ctx, `UPDATE lx_sources SET position = ? WHERE id = ?`, i, id); err != nil {
				return err
			}
		}
		return nil
	})
}
