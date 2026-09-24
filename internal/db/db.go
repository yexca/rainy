// Package db opens Rainy's SQLite database (pure-Go modernc.org/sqlite) and applies the
// embedded migrations.
//
// A DB has two pools on the same file: R, a reader pool for queries, and W, a single
// connection for all writes (SQLite allows one writer at a time; funnelling writes through
// one connection avoids SQLITE_BUSY storms). WAL mode lets readers proceed while writing.
package db

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
	_ "modernc.org/sqlite" // registers the "sqlite" driver
)

//go:embed migrations/*.sql
var migrationFS embed.FS

// DB bundles the reader pool and the single-connection writer.
type DB struct {
	R *sqlx.DB // reader pool (max open = 2*NumCPU), query_only
	W *sqlx.DB // writer, MaxOpenConns(1)
}

var pragmas = []string{
	"journal_mode(WAL)",
	"busy_timeout(10000)",
	"foreign_keys(ON)",
	"synchronous(NORMAL)",
	"cache_size(-20000)",
}

func dsn(path string, extra ...string) string {
	q := url.Values{}
	for _, p := range append(append([]string{}, pragmas...), extra...) {
		q.Add("_pragma", p)
	}
	return "file:" + uriEscaper.Replace(filepath.ToSlash(path)) + "?" + q.Encode()
}

// uriEscaper escapes the characters that SQLite's URI filename parser would otherwise
// interpret ("%XX" decoding, "#" fragment, "?" query) so data dirs such as
// "/volume1/my#music" or "C:\100%\data" open the right file.
var uriEscaper = strings.NewReplacer("%", "%25", "#", "%23", "?", "%3f")

// Open opens (creating if needed) the database at path. The parent directory must exist.
func Open(path string) (*DB, error) {
	if dir := filepath.Dir(path); dir != "" {
		if _, err := os.Stat(dir); err != nil {
			return nil, fmt.Errorf("database directory: %w", err)
		}
	}
	// The writer uses BEGIN IMMEDIATE so write transactions take the lock up front instead
	// of failing with SQLITE_BUSY when upgrading from a read lock.
	w, err := sqlx.Open("sqlite", dsn(path)+"&_txlock=immediate")
	if err != nil {
		return nil, fmt.Errorf("opening writer: %w", err)
	}
	w.SetMaxOpenConns(1)
	w.SetMaxIdleConns(1)
	w.SetConnMaxLifetime(0)
	w.SetConnMaxIdleTime(0)
	if err := w.Ping(); err != nil {
		_ = w.Close()
		return nil, fmt.Errorf("opening %s: %w", path, err)
	}

	r, err := sqlx.Open("sqlite", dsn(path, "query_only(1)"))
	if err != nil {
		_ = w.Close()
		return nil, fmt.Errorf("opening reader: %w", err)
	}
	n := 2 * runtime.NumCPU()
	if n < 4 {
		n = 4
	}
	r.SetMaxOpenConns(n)
	r.SetMaxIdleConns(n)
	r.SetConnMaxIdleTime(5 * time.Minute)
	if err := r.Ping(); err != nil {
		_ = w.Close()
		_ = r.Close()
		return nil, fmt.Errorf("opening %s (reader): %w", path, err)
	}
	return &DB{R: r, W: w}, nil
}

// Close closes both pools. The writer is closed last so the WAL is checkpointed.
func (d *DB) Close() error {
	return errors.Join(d.R.Close(), d.W.Close())
}

// Tx runs fn in a write transaction on W. The transaction is rolled back when fn returns an
// error or panics (the panic is re-raised), committed otherwise.
func (d *DB) Tx(ctx context.Context, fn func(tx *sqlx.Tx) error) (err error) {
	tx, err := d.W.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback()
			panic(p)
		}
		if err != nil {
			_ = tx.Rollback()
		}
	}()
	if err = fn(tx); err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}

// Migrate applies every embedded migration (migrations/*.sql, lexical order) that is not yet
// recorded in schema_migrations. Each migration runs in its own transaction.
func (d *DB) Migrate(ctx context.Context) error {
	if _, err := d.W.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		version    TEXT    PRIMARY KEY,
		applied_at INTEGER NOT NULL
	)`); err != nil {
		return fmt.Errorf("creating schema_migrations: %w", err)
	}
	names, err := fs.Glob(migrationFS, "migrations/*.sql")
	if err != nil {
		return err
	}
	sort.Strings(names)

	var applied []string
	if err := d.W.SelectContext(ctx, &applied, `SELECT version FROM schema_migrations`); err != nil {
		return fmt.Errorf("reading schema_migrations: %w", err)
	}
	done := make(map[string]bool, len(applied))
	for _, v := range applied {
		done[v] = true
	}

	for _, name := range names {
		version := strings.TrimSuffix(filepath.Base(name), ".sql")
		if done[version] {
			continue
		}
		body, err := migrationFS.ReadFile(name)
		if err != nil {
			return err
		}
		err = d.Tx(ctx, func(tx *sqlx.Tx) error {
			if _, err := tx.ExecContext(ctx, string(body)); err != nil {
				return err
			}
			_, err := tx.ExecContext(ctx, `INSERT INTO schema_migrations (version, applied_at) VALUES (?, ?)`,
				version, time.Now().UnixMilli())
			return err
		})
		if err != nil {
			return fmt.Errorf("applying migration %s: %w", version, err)
		}
	}
	return nil
}

// Versions returns the applied migration versions in order.
func (d *DB) Versions(ctx context.Context) ([]string, error) {
	var v []string
	err := d.R.SelectContext(ctx, &v, `SELECT version FROM schema_migrations ORDER BY version`)
	return v, err
}
