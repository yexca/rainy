package backup

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"rainy/internal/auth"
	"rainy/internal/db"
)

func fixture(t *testing.T) (string, *db.DB) {
	t.Helper()
	dir := t.TempDir()
	d, err := db.Open(filepath.Join(dir, "rainy.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = d.Close() })
	if err := d.Migrate(t.Context()); err != nil {
		t.Fatal(err)
	}
	key, err := auth.LoadOrCreateKey(filepath.Join(dir, "secret.key"))
	if err != nil {
		t.Fatal(err)
	}
	encrypted, err := auth.NewCrypto(key).Encrypt("synthetic-password")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.W.ExecContext(t.Context(), "INSERT INTO users (id, username, password_enc, created_at, updated_at) VALUES (?, ?, ?, 0, 0)", "example-user", "example-user", encrypted); err != nil {
		t.Fatal(err)
	}
	return dir, d
}

func TestSnapshotCapturesWALAndVerifiesWithoutChangingFiles(t *testing.T) {
	dir, d := fixture(t)
	if _, err := d.W.ExecContext(t.Context(), "INSERT INTO playlists (id, name, owner_id, created_at, updated_at) VALUES ('example-playlist', 'Example Playlist', 'example-user', 0, 0)"); err != nil {
		t.Fatal(err)
	}
	manifest, err := Create(t.Context(), dir, "example-backup", "test")
	if err != nil {
		t.Fatal(err)
	}
	if manifest.Files["secret.key"].Size != 32 || manifest.Files["rainy.db"].Size == 0 {
		t.Fatalf("%+v", manifest)
	}
	if _, err := d.W.ExecContext(t.Context(), "UPDATE playlists SET name = 'Later edit'"); err != nil {
		t.Fatal(err)
	}
	snapshot := filepath.Join(dir, "backups", "example-backup", "rainy.db")
	conn, err := openReadOnly(t.Context(), snapshot, true)
	if err != nil {
		t.Fatal(err)
	}
	var name string
	err = conn.GetContext(t.Context(), &name, "SELECT name FROM playlists WHERE id = 'example-playlist'")
	_ = conn.Close()
	if err != nil || name != "Example Playlist" {
		t.Fatalf("snapshot name %q: %v", name, err)
	}
	if _, err := Verify(t.Context(), dir, "example-backup"); err != nil {
		t.Fatal(err)
	}
	if _, err := Create(t.Context(), dir, "example-backup", "test"); err == nil {
		t.Fatal("overwrote backup")
	}
	after, err := checksum(t.Context(), snapshot)
	if err != nil || after != manifest.Files["rainy.db"] {
		t.Fatal("verification changed snapshot")
	}
	if err := os.WriteFile(filepath.Join(dir, "backups", "example-backup", "secret.key"), make([]byte, 32), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Verify(t.Context(), dir, "example-backup"); err == nil || !strings.Contains(err.Error(), "checksum") {
		t.Fatalf("tamper accepted: %v", err)
	}
}

func TestSnapshotRejectsUnsafeOrIncompleteInputs(t *testing.T) {
	dir, _ := fixture(t)
	for _, name := range []string{"", "..", "../escape", "/absolute", "C:\\absolute", "a/b"} {
		if _, err := Create(t.Context(), dir, name, "test"); err == nil {
			t.Errorf("accepted %q", name)
		}
	}
	if err := os.Remove(filepath.Join(dir, "secret.key")); err != nil {
		t.Fatal(err)
	}
	if _, err := Create(t.Context(), dir, "missing-key", "test"); err == nil {
		t.Fatal("missing key accepted")
	}
	if _, err := os.Stat(filepath.Join(dir, "secret.key")); !os.IsNotExist(err) {
		t.Fatal("backup created a replacement key")
	}
	if err := os.WriteFile(filepath.Join(dir, "secret.key"), make([]byte, 32), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Create(t.Context(), dir, "wrong-key", "test"); err == nil || !strings.Contains(err.Error(), "credentials") {
		t.Fatalf("wrong key: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "backups", "wrong-key")); !os.IsNotExist(err) {
		t.Fatal("failed backup left behind")
	}
}

func TestSnapshotRejectsFutureSchemaAndCancellation(t *testing.T) {
	dir, d := fixture(t)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := Create(ctx, dir, "canceled", "test"); err == nil {
		t.Fatal("cancellation ignored")
	}
	if _, err := os.Stat(filepath.Join(dir, "backups", "canceled")); !os.IsNotExist(err) {
		t.Fatal("canceled backup left behind")
	}
	if _, err := d.W.ExecContext(t.Context(), "INSERT INTO schema_migrations VALUES ('9999_future', 0)"); err != nil {
		t.Fatal(err)
	}
	if _, err := Create(t.Context(), dir, "future", "test"); err == nil || !strings.Contains(err.Error(), "schema") {
		t.Fatalf("future schema: %v", err)
	}
}

func TestBackupSymlinkCannotEscapeDataDirectory(t *testing.T) {
	dir, _ := fixture(t)
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(dir, "backups")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if _, err := Create(t.Context(), dir, "example", "test"); err == nil {
		t.Fatal("escaped through symlink")
	}
	if entries, err := os.ReadDir(outside); err != nil || len(entries) != 0 {
		t.Fatal("wrote outside data directory")
	}
}
