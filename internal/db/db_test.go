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
	if err != nil || len(v) != 3 || v[0] != "0001_init" || v[1] != "0002_play_queue_index" || v[2] != "0003_lx_sources" {
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
