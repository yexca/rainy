package config

import (
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func env(m map[string]string) func(string) (string, bool) {
	return func(k string) (string, bool) { v, ok := m[k]; return v, ok }
}

func TestLoadDefaults(t *testing.T) {
	c, err := LoadFrom(env(nil))
	if err != nil {
		t.Fatal(err)
	}
	if c.Port != 7650 || c.Address != "0.0.0.0" || c.ScanInterval != time.Hour || !c.ScanOnStart ||
		c.FFmpegPath != "ffmpeg" || c.LogLevel != "info" || c.LogFormat != "text" ||
		c.SessionTTL != 720*time.Hour || c.TrustProxy || c.DevCORS != "" {
		t.Fatalf("unexpected defaults: %+v", c)
	}
	if !filepath.IsAbs(c.DataDir) || !filepath.IsAbs(c.MusicDir) {
		t.Fatalf("paths must be absolute: %q %q", c.DataDir, c.MusicDir)
	}
	configDir, _ := filepath.Abs("./config")
	musicDir, _ := filepath.Abs("./data")
	if c.DataDir != configDir || c.MusicDir != musicDir {
		t.Fatalf("state and music defaults must be separate: %q %q", c.DataDir, c.MusicDir)
	}
	if c.ListenAddr() != "0.0.0.0:7650" {
		t.Fatalf("ListenAddr = %q", c.ListenAddr())
	}
	if filepath.Base(c.DBPath()) != "rainy.db" || filepath.Dir(c.ArtworkCacheDir()) != c.CacheDir() {
		t.Fatal("bad derived paths")
	}
}

func TestLoadOverrides(t *testing.T) {
	dir := t.TempDir()
	c, err := LoadFrom(env(map[string]string{
		"RAINY_ADDRESS": "::1", "RAINY_PORT": "8080", "RAINY_DATA_DIR": dir, "RAINY_MUSIC_DIR": dir,
		"RAINY_SCAN_INTERVAL": "0", "RAINY_SCAN_ON_START": "false", "RAINY_FFMPEG_PATH": "/usr/bin/ffmpeg",
		"RAINY_LOG_LEVEL": "DEBUG", "RAINY_LOG_FORMAT": "json", "RAINY_SESSION_TTL": "7d",
		"RAINY_TRUST_PROXY": "1", "RAINY_DEV_CORS": "http://localhost:5173/",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if c.Port != 8080 || c.ScanInterval != 0 || c.ScanOnStart || c.LogLevel != "debug" || c.LogFormat != "json" ||
		c.SessionTTL != 7*24*time.Hour || !c.TrustProxy || c.DevCORS != "http://localhost:5173" || c.DataDir != dir {
		t.Fatalf("unexpected: %+v", c)
	}
	if c.ListenAddr() != "[::1]:8080" {
		t.Fatalf("ListenAddr = %q", c.ListenAddr())
	}
}

func TestLoadErrors(t *testing.T) {
	_, err := LoadFrom(env(map[string]string{
		"RAINY_PORT": "99999", "RAINY_SCAN_INTERVAL": "soon", "RAINY_LOG_LEVEL": "loud", "RAINY_TRUST_PROXY": "maybe",
	}))
	if err == nil {
		t.Fatal("expected error")
	}
	for _, want := range []string{"RAINY_PORT", "RAINY_SCAN_INTERVAL", "RAINY_LOG_LEVEL", "RAINY_TRUST_PROXY"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not mention %s", err, want)
		}
	}
}

func TestParseDuration(t *testing.T) {
	for in, want := range map[string]time.Duration{
		"0": 0, "1h": time.Hour, "90m": 90 * time.Minute, "2d": 48 * time.Hour, "1d12h": 36 * time.Hour,
	} {
		got, err := ParseDuration(in)
		if err != nil || got != want {
			t.Errorf("ParseDuration(%q) = %v, %v; want %v", in, got, err, want)
		}
	}
	for _, in := range []string{"x", "1x", "d", "1dq"} {
		if _, err := ParseDuration(in); err == nil {
			t.Errorf("ParseDuration(%q) should fail", in)
		}
	}
}
