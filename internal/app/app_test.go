package app

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"rainy/internal/config"
)

func testConfig(t *testing.T, music string) *config.Config {
	c := config.Default()
	c.DataDir = filepath.Join(t.TempDir(), "data")
	c.MusicDir = music
	return c
}

func TestNewCreatesDefaultLibrary(t *testing.T) {
	ctx := context.Background()
	music := t.TempDir()
	cfg := testConfig(t, music)
	a, err := New(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	libs, err := a.Store.ListLibraries(ctx)
	if err != nil || len(libs) != 1 || libs[0].Path != music {
		t.Fatalf("libraries %+v %v", libs, err)
	}
	for _, p := range []string{cfg.DBPath(), cfg.SecretKeyPath(), cfg.ArtworkCacheDir(), cfg.TrashDir(), cfg.TmpDir()} {
		if _, err := os.Stat(p); err != nil {
			t.Errorf("%s: %v", p, err)
		}
	}
	if s := a.Settings(ctx); s.ScanInterval != "1h" || s.TranscodeBitrate != 192 {
		t.Fatalf("settings %+v", s)
	}
	if a.Scanner.Status().Phase != "idle" {
		t.Fatal("scanner status")
	}
	if err := a.Close(); err != nil {
		t.Fatal(err)
	}

	// Re-opening keeps the key and does not add another library.
	key1, _ := os.ReadFile(cfg.SecretKeyPath())
	cfg.ScanInterval = 30 * time.Minute
	a, err = New(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = a.Close() }()
	key2, _ := os.ReadFile(cfg.SecretKeyPath())
	if string(key1) != string(key2) {
		t.Fatal("secret key changed")
	}
	if libs, _ := a.Store.ListLibraries(ctx); len(libs) != 1 {
		t.Fatal("library duplicated")
	}
	if s := a.Settings(ctx); s.ScanInterval != "30m" {
		t.Fatalf("default scan interval follows config: %q", s.ScanInterval)
	}
}

func TestNewWithoutMusicDir(t *testing.T) {
	ctx := context.Background()
	a, err := New(ctx, testConfig(t, filepath.Join(t.TempDir(), "nope")))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = a.Close() }()
	if libs, _ := a.Store.ListLibraries(ctx); len(libs) != 0 {
		t.Fatal("no library expected")
	}
}
