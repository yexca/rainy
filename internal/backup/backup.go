// Package backup creates and checks application-state snapshots under the data
// directory. Music, trash files, caches, and yt-dlp files are backed up separately.
package backup

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"time"

	"github.com/jmoiron/sqlx"

	"rainy/internal/auth"
	"rainy/internal/db"
	"rainy/internal/util"
)

const maxDatabaseBytes int64 = 16 << 30
const maxManifestBytes int64 = 64 << 10

var validName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)
var fileNames = []string{"rainy.db", "secret.key"}

type File struct {
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
}

type Manifest struct {
	Format     int             `json:"format"`
	CreatedAt  int64           `json:"createdAt"`
	Version    string          `json:"version"`
	Migrations []string        `json:"migrations"`
	Files      map[string]File `json:"files"`
}

// Create makes an exclusive backups/name directory. VACUUM INTO reads a consistent
// live SQLite snapshot, including committed WAL data, without migrating the source.
// A manifest is written only after the snapshot passes all restore checks.
func Create(ctx context.Context, dataDir, name, version string) (*Manifest, error) {
	target, err := location(dataDir, name)
	if err != nil {
		return nil, err
	}
	source, err := safePath(dataDir, "rainy.db")
	if err != nil {
		return nil, err
	}
	keyPath, err := safePath(dataDir, "secret.key")
	if err != nil {
		return nil, err
	}
	key, err := readSmall(keyPath, auth.KeySize)
	if err != nil || len(key) != auth.KeySize {
		return nil, errors.New("a valid existing secret.key is required")
	}
	if err := regularFile(source, maxDatabaseBytes); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		return nil, err
	}
	if err := os.Mkdir(target, 0o700); err != nil {
		return nil, fmt.Errorf("creating backup (choose an unused name): %w", err)
	}
	complete := false
	defer func() {
		if !complete {
			_ = os.RemoveAll(target)
		}
	}()
	dest, err := safePath(target, "rainy.db")
	if err != nil {
		return nil, err
	}
	if err := writeExclusive(dest, nil); err != nil {
		return nil, err
	}
	conn, err := openReadOnly(ctx, source, false)
	if err != nil {
		return nil, err
	}
	_, snapshotErr := conn.ExecContext(ctx, "VACUUM INTO ?", dest)
	closeErr := conn.Close()
	if err := errors.Join(snapshotErr, closeErr); err != nil {
		return nil, fmt.Errorf("creating database snapshot: %w", err)
	}
	// Refuse a key replaced while the database was being copied.
	after, err := readSmall(keyPath, auth.KeySize)
	if err != nil || !slices.Equal(key, after) {
		return nil, errors.New("secret.key changed during backup; retry")
	}
	destKey, err := safePath(target, "secret.key")
	if err != nil {
		return nil, err
	}
	if err := writeExclusive(destKey, key); err != nil {
		return nil, err
	}
	versions, err := checkDatabase(ctx, dest, key)
	if err != nil {
		return nil, err
	}
	m := &Manifest{Format: 1, CreatedAt: time.Now().UnixMilli(), Version: version, Migrations: versions, Files: map[string]File{}}
	for _, name := range fileNames {
		p, err := safePath(target, name)
		if err != nil {
			return nil, err
		}
		f, err := os.OpenFile(p, os.O_RDWR, 0)
		if err != nil {
			return nil, err
		}
		err = f.Sync()
		err = errors.Join(err, f.Close())
		if err != nil {
			return nil, err
		}
		info, err := checksum(ctx, p)
		if err != nil {
			return nil, err
		}
		m.Files[name] = info
	}
	content, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return nil, err
	}
	manifestPath, err := safePath(target, "manifest.json")
	if err != nil {
		return nil, err
	}
	if err := writeExclusive(manifestPath, append(content, '\n')); err != nil {
		return nil, err
	}
	complete = true
	return m, nil
}

// Verify checks hashes, SQLite integrity, the migration chain and the key against
// stored credentials. It never restores files or migrates the snapshot.
func Verify(ctx context.Context, dataDir, name string) (*Manifest, error) {
	target, err := location(dataDir, name)
	if err != nil {
		return nil, err
	}
	p, err := safePath(target, "manifest.json")
	if err != nil {
		return nil, err
	}
	content, err := readSmall(p, maxManifestBytes)
	if err != nil {
		return nil, err
	}
	var m Manifest
	if err := json.Unmarshal(content, &m); err != nil {
		return nil, errors.New("invalid backup manifest")
	}
	if m.Format != 1 || len(m.Files) != len(fileNames) {
		return nil, errors.New("unsupported backup manifest")
	}
	for _, name := range fileNames {
		expected, ok := m.Files[name]
		if !ok {
			return nil, errors.New("incomplete backup manifest")
		}
		p, err := safePath(target, name)
		if err != nil {
			return nil, err
		}
		actual, err := checksum(ctx, p)
		if err != nil {
			return nil, err
		}
		if actual != expected {
			return nil, fmt.Errorf("backup checksum mismatch: %s", name)
		}
	}
	keyPath, err := safePath(target, "secret.key")
	if err != nil {
		return nil, err
	}
	key, err := readSmall(keyPath, auth.KeySize)
	if err != nil || len(key) != auth.KeySize {
		return nil, errors.New("invalid backup secret.key")
	}
	p, err = safePath(target, "rainy.db")
	if err != nil {
		return nil, err
	}
	for _, suffix := range []string{"-wal", "-journal"} {
		if _, err := os.Lstat(p + suffix); !errors.Is(err, os.ErrNotExist) {
			return nil, errors.New("backup has a journal; use an unmodified snapshot")
		}
	}
	versions, err := checkDatabase(ctx, p, key)
	if err != nil {
		return nil, err
	}
	if !slices.Equal(versions, m.Migrations) {
		return nil, errors.New("backup migration manifest does not match its database")
	}
	return &m, nil
}

func location(dataDir, name string) (string, error) {
	if !validName.MatchString(name) {
		return "", errors.New("backup name must be 1–64 letters, digits, dots, dashes or underscores, starting with a letter or digit")
	}
	return safePath(dataDir, "backups/"+name)
}

func safePath(root, rel string) (string, error) {
	p, err := util.SafeJoin(root, rel)
	if err != nil {
		return "", err
	}
	if err := util.EnsureWithinRoot(root, p); err != nil {
		return "", err
	}
	return p, nil
}

func regularFile(p string, limit int64) error {
	info, err := os.Lstat(p)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || info.Size() > limit {
		return errors.New("backup input must be a regular file within the size limit")
	}
	return nil
}

func readSmall(p string, limit int64) ([]byte, error) {
	if err := regularFile(p, limit); err != nil {
		return nil, err
	}
	f, err := os.Open(p)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	b, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err == nil && int64(len(b)) > limit {
		err = errors.New("backup input exceeds its size limit")
	}
	return b, err
}

func writeExclusive(p string, content []byte) error {
	f, err := os.OpenFile(p, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	_, err = f.Write(content)
	if err == nil {
		err = f.Sync()
	}
	return errors.Join(err, f.Close())
}

type contextReader struct {
	ctx context.Context
	io.Reader
}

func (r contextReader) Read(b []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.Reader.Read(b)
}

func checksum(ctx context.Context, p string) (File, error) {
	if err := regularFile(p, maxDatabaseBytes); err != nil {
		return File{}, err
	}
	f, err := os.Open(p)
	if err != nil {
		return File{}, err
	}
	defer func() { _ = f.Close() }()
	hash := sha256.New()
	n, err := io.Copy(hash, contextReader{ctx, io.LimitReader(f, maxDatabaseBytes+1)})
	if err != nil {
		return File{}, err
	}
	if n > maxDatabaseBytes {
		return File{}, errors.New("backup file exceeds 16 GiB")
	}
	return File{Size: n, SHA256: hex.EncodeToString(hash.Sum(nil))}, nil
}

func openReadOnly(ctx context.Context, p string, immutable bool) (*sqlx.DB, error) {
	u := &url.URL{Path: filepath.ToSlash(p)}
	dsn := "file:" + u.EscapedPath() + "?mode=ro&_pragma=busy_timeout(10000)&_pragma=trusted_schema(OFF)"
	if immutable {
		dsn += "&immutable=1"
	}
	return sqlx.ConnectContext(ctx, "sqlite", dsn)
}

func checkDatabase(ctx context.Context, p string, key []byte) ([]string, error) {
	conn, err := openReadOnly(ctx, p, true)
	if err != nil {
		return nil, err
	}
	defer func() { _ = conn.Close() }()
	var checks []string
	if err := conn.SelectContext(ctx, &checks, "PRAGMA integrity_check"); err != nil {
		return nil, err
	}
	if !slices.Equal(checks, []string{"ok"}) {
		return nil, errors.New("backup database integrity check failed")
	}
	rows, err := conn.QueryContext(ctx, "PRAGMA foreign_key_check")
	if err != nil {
		return nil, err
	}
	hasViolation := rows.Next()
	readErr := rows.Err()
	_ = rows.Close()
	if readErr != nil {
		return nil, readErr
	}
	if hasViolation {
		return nil, errors.New("backup database foreign key check failed")
	}
	var versions []string
	if err := conn.SelectContext(ctx, &versions, "SELECT version FROM schema_migrations ORDER BY version"); err != nil {
		return nil, err
	}
	known, err := db.MigrationVersions()
	if err != nil {
		return nil, err
	}
	if len(versions) == 0 || len(versions) > len(known) || !slices.Equal(versions, known[:len(versions)]) {
		return nil, errors.New("backup schema is incomplete or newer than this binary; use a compatible Rainy version")
	}
	queries := []string{"SELECT password_enc FROM users", "SELECT value FROM settings WHERE key = 'lastfm.secretEnc'"}
	if slices.Contains(versions, "0004_listening") {
		queries = append(queries, "SELECT credential_enc FROM scrobble_accounts WHERE credential_enc != ''")
	}
	cipher := auth.NewCrypto(key)
	for _, query := range queries {
		rows, err := conn.QueryContext(ctx, query)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var encrypted string
			if err := rows.Scan(&encrypted); err != nil {
				_ = rows.Close()
				return nil, err
			}
			if encrypted != "" {
				if _, err := cipher.Decrypt(encrypted); err != nil {
					_ = rows.Close()
					return nil, errors.New("backup key does not match stored credentials")
				}
			}
		}
		err = rows.Err()
		_ = rows.Close()
		if err != nil {
			return nil, err
		}
	}
	return versions, nil
}
