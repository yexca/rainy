package db_test

import (
	"context"
	"errors"
	"path/filepath"
	"sync"
	"testing"

	"github.com/jmoiron/sqlx"

	"rainy/internal/db"
	"rainy/internal/db/dbtest"
)

func TestOpenMigrate(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "x.db")
	d, err := db.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = d.Close() }()
	if err := d.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	if err := d.Migrate(ctx); err != nil { // idempotent
		t.Fatalf("second migrate: %v", err)
	}
	v, err := d.Versions(ctx)
	if err != nil || len(v) != 4 || v[0] != "0001_init" || v[1] != "0002_play_queue_index" || v[2] != "0003_lx_sources" ||
		v[3] != "0004_listening" {
		t.Fatalf("versions = %v, %v", v, err)
	}
	var mode string
	if err := d.W.Get(&mode, "PRAGMA journal_mode"); err != nil || mode != "wal" {
		t.Fatalf("journal_mode = %q, %v", mode, err)
	}
	var fk int
	if err := d.R.Get(&fk, "PRAGMA foreign_keys"); err != nil || fk != 1 {
		t.Fatalf("reader foreign_keys = %d, %v", fk, err)
	}
	// Reader is query-only.
	if _, err := d.R.Exec(`INSERT INTO settings (key, value) VALUES ('a', '1')`); err == nil {
		t.Fatal("reader must be read-only")
	}
}

func TestTx(t *testing.T) {
	ctx := context.Background()
	d := dbtest.New(t)
	sentinel := errors.New("boom")
	err := d.Tx(ctx, func(tx *sqlx.Tx) error {
		if _, err := tx.Exec(`INSERT INTO settings (key, value) VALUES ('k', 'v')`); err != nil {
			return err
		}
		return sentinel
	})
	if !errors.Is(err, sentinel) {
		t.Fatalf("got %v", err)
	}
	var n int
	_ = d.R.Get(&n, `SELECT COUNT(*) FROM settings`)
	if n != 0 {
		t.Fatal("rollback failed")
	}

	func() {
		defer func() {
			if recover() == nil {
				t.Fatal("panic not re-raised")
			}
		}()
		_ = d.Tx(ctx, func(tx *sqlx.Tx) error {
			_, _ = tx.Exec(`INSERT INTO settings (key, value) VALUES ('k', 'v')`)
			panic("x")
		})
	}()
	_ = d.R.Get(&n, `SELECT COUNT(*) FROM settings`)
	if n != 0 {
		t.Fatal("rollback after panic failed")
	}

	if err := d.Tx(ctx, func(tx *sqlx.Tx) error {
		_, err := tx.Exec(`INSERT INTO settings (key, value) VALUES ('k', 'v')`)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	_ = d.R.Get(&n, `SELECT COUNT(*) FROM settings`)
	if n != 1 {
		t.Fatal("commit failed")
	}
}

func TestConcurrentWrites(t *testing.T) {
	ctx := context.Background()
	d := dbtest.New(t)
	var wg sync.WaitGroup
	errs := make(chan error, 50)
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			errs <- d.Tx(ctx, func(tx *sqlx.Tx) error {
				_, err := tx.Exec(`INSERT INTO settings (key, value) VALUES (?, 'v')`, i)
				return err
			})
		}(i)
		wg.Add(1)
		go func() {
			defer wg.Done()
			var n int
			_ = d.R.Get(&n, `SELECT COUNT(*) FROM settings`)
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
}

// Upgrading to 0004_listening backfills the play-history snapshot from the tracks that still
// exist; plays of tracks purged before the upgrade keep blank snapshots.
func TestListeningMigrationBackfill(t *testing.T) {
	ctx := context.Background()
	d := dbtest.New(t)
	// Turn the migrated database back into a 0003 one.
	for _, stmt := range []string{
		`DROP TABLE scrobble_queue`, `DROP TABLE scrobble_accounts`, `DROP INDEX idx_play_history_user_track`,
		`ALTER TABLE play_history DROP COLUMN title`, `ALTER TABLE play_history DROP COLUMN artist`,
		`ALTER TABLE play_history DROP COLUMN album`, `ALTER TABLE play_history DROP COLUMN album_artist`,
		`ALTER TABLE play_history DROP COLUMN artist_id`, `ALTER TABLE play_history DROP COLUMN album_id`,
		`ALTER TABLE play_history DROP COLUMN duration`,
		`DELETE FROM schema_migrations WHERE version = '0004_listening'`,
		`INSERT INTO libraries (id, name, path, created_at, updated_at) VALUES (1, 'Music', '/path/to/music', 1, 1)`,
		`INSERT INTO users (id, username, password_enc, created_at, updated_at) VALUES ('u', 'u', 'x', 1, 1)`,
		`INSERT INTO tracks (id, library_id, path, dir, filename, suffix, size, mtime, title, album, artist, album_artist,
			album_id, artist_id, album_artist_id, duration, created_at, updated_at)
			VALUES ('t1', 1, 'a/1.flac', 'a', '1.flac', 'flac', 1, 1, 'One', 'First', 'Alpha', 'Alpha', 'al', 'ar', 'ar', 201.5, 1, 1)`,
		`INSERT INTO play_history (user_id, track_id, played_at, client) VALUES ('u', 't1', 1000, 'web'), ('u', 'gone', 2000, 'web')`,
	} {
		if _, err := d.W.ExecContext(ctx, stmt); err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}
	if err := d.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	var rows []struct {
		TrackID  string  `db:"track_id"`
		Title    string  `db:"title"`
		Artist   string  `db:"artist"`
		Album    string  `db:"album"`
		ArtistID string  `db:"artist_id"`
		AlbumID  string  `db:"album_id"`
		Duration float64 `db:"duration"`
	}
	if err := d.R.SelectContext(ctx, &rows, `SELECT track_id, title, artist, album, artist_id, album_id, duration
		FROM play_history ORDER BY played_at`); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || rows[0].Title != "One" || rows[0].Artist != "Alpha" || rows[0].Album != "First" ||
		rows[0].ArtistID != "ar" || rows[0].AlbumID != "al" || rows[0].Duration != 201.5 {
		t.Fatalf("backfilled %+v", rows)
	}
	if rows[1].Title != "" || rows[1].Duration != 0 {
		t.Fatalf("purged track got a snapshot: %+v", rows[1])
	}
}
