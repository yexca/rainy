package db_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"rainy/internal/db"
)

// TestOpenUnusualPaths checks that characters meaningful in SQLite URIs ("#", "%", "?")
// and spaces in the data directory still open the intended file.
func TestOpenUnusualPaths(t *testing.T) {
	for _, name := range []string{"with space", "hash#dir", "pct%20dir", "q?dir", "周 杰"} {
		dir := filepath.Join(t.TempDir(), name)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			continue // "?" is not a valid file name character on Windows
		}
		p := filepath.Join(dir, "rainy.db")
		d, err := db.Open(p)
		if err != nil {
			t.Errorf("%q: open: %v", name, err)
			continue
		}
		if err := d.Migrate(context.Background()); err != nil {
			t.Errorf("%q: migrate: %v", name, err)
		}
		_ = d.Close()
		if _, err := os.Stat(p); err != nil {
			t.Errorf("%q: database not created at %s: %v", name, p, err)
		}
	}
}
