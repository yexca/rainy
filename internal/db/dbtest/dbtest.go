// Package dbtest provides a migrated, file-backed SQLite database for tests.
package dbtest

import (
	"context"
	"path/filepath"
	"testing"

	"rainy/internal/db"
)

// New returns a freshly migrated database in a temporary directory. It is closed
// automatically when the test finishes.
func New(t testing.TB) *db.DB {
	t.Helper()
	d, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("dbtest: open: %v", err)
	}
	t.Cleanup(func() { _ = d.Close() })
	if err := d.Migrate(context.Background()); err != nil {
		t.Fatalf("dbtest: migrate: %v", err)
	}
	return d
}
