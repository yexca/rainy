// Package config loads Rainy's process configuration from RAINY_* environment variables.
//
// Runtime-editable settings (scan interval, transcoding defaults, …) live in the database
// (see model.Settings); this package only covers what must be known before the database is
// opened, plus defaults for those settings.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Config is the process configuration. All paths are absolute after Load.
type Config struct {
	Address      string        // listen address (RAINY_ADDRESS)
	Port         int           // listen port (RAINY_PORT)
	DataDir      string        // DB, secret key, caches, trash (RAINY_DATA_DIR)
	MusicDir     string        // default library root (RAINY_MUSIC_DIR)
	ScanInterval time.Duration // default for settings.scanInterval; 0 disables (RAINY_SCAN_INTERVAL)
	ScanOnStart  bool          // quick scan at startup (RAINY_SCAN_ON_START)
	FFmpegPath   string        // ffmpeg binary (RAINY_FFMPEG_PATH)
	LogLevel     string        // debug|info|warn|error (RAINY_LOG_LEVEL)
	LogFormat    string        // text|json (RAINY_LOG_FORMAT)
	SessionTTL   time.Duration // sliding web session lifetime (RAINY_SESSION_TTL)
	TrustProxy   bool          // honour X-Forwarded-For / X-Real-IP (RAINY_TRUST_PROXY)
	DevCORS      string        // extra allowed origin for /api in development (RAINY_DEV_CORS)
}

// Default returns the configuration used when no environment variable is set
// (paths are still relative; Load makes them absolute).
func Default() *Config {
	return &Config{
		Address:      "0.0.0.0",
		Port:         7650,
		DataDir:      "./data",
		MusicDir:     "./music",
		ScanInterval: time.Hour,
		ScanOnStart:  true,
		FFmpegPath:   "ffmpeg",
		LogLevel:     "info",
		LogFormat:    "text",
		SessionTTL:   720 * time.Hour,
	}
}

// Load reads the configuration from the process environment.
func Load() (*Config, error) { return LoadFrom(os.LookupEnv) }

// LoadFrom reads the configuration using lookup (os.LookupEnv-compatible); useful for tests.
func LoadFrom(lookup func(string) (string, bool)) (*Config, error) {
	c := Default()
	var errs []error
	get := func(key string) (string, bool) {
		v, ok := lookup(key)
		if !ok {
			return "", false
		}
		v = strings.TrimSpace(v)
		return v, v != ""
	}

	if v, ok := get("RAINY_ADDRESS"); ok {
		c.Address = v
	}
	if v, ok := get("RAINY_PORT"); ok {
		p, err := strconv.Atoi(v)
		if err != nil || p < 1 || p > 65535 {
			errs = append(errs, fmt.Errorf("RAINY_PORT: invalid port %q", v))
		} else {
			c.Port = p
		}
	}
	if v, ok := get("RAINY_DATA_DIR"); ok {
		c.DataDir = v
	}
	if v, ok := get("RAINY_MUSIC_DIR"); ok {
		c.MusicDir = v
	}
	if v, ok := get("RAINY_SCAN_INTERVAL"); ok {
		d, err := ParseDuration(v)
		if err != nil || d < 0 {
			errs = append(errs, fmt.Errorf("RAINY_SCAN_INTERVAL: invalid duration %q", v))
		} else {
			c.ScanInterval = d
		}
	}
	if v, ok := get("RAINY_SCAN_ON_START"); ok {
		b, err := ParseBool(v)
		if err != nil {
			errs = append(errs, fmt.Errorf("RAINY_SCAN_ON_START: %w", err))
		} else {
			c.ScanOnStart = b
		}
	}
	if v, ok := get("RAINY_FFMPEG_PATH"); ok {
		c.FFmpegPath = v
	}
	if v, ok := get("RAINY_LOG_LEVEL"); ok {
		v = strings.ToLower(v)
		switch v {
		case "debug", "info", "warn", "error":
			c.LogLevel = v
		case "warning":
			c.LogLevel = "warn"
		default:
			errs = append(errs, fmt.Errorf("RAINY_LOG_LEVEL: invalid level %q (debug|info|warn|error)", v))
		}
	}
	if v, ok := get("RAINY_LOG_FORMAT"); ok {
		v = strings.ToLower(v)
		switch v {
		case "text", "json":
			c.LogFormat = v
		default:
			errs = append(errs, fmt.Errorf("RAINY_LOG_FORMAT: invalid format %q (text|json)", v))
		}
	}
	if v, ok := get("RAINY_SESSION_TTL"); ok {
		d, err := ParseDuration(v)
		if err != nil || d < time.Minute {
			errs = append(errs, fmt.Errorf("RAINY_SESSION_TTL: invalid duration %q (minimum 1m)", v))
		} else {
			c.SessionTTL = d
		}
	}
	if v, ok := get("RAINY_TRUST_PROXY"); ok {
		b, err := ParseBool(v)
		if err != nil {
			errs = append(errs, fmt.Errorf("RAINY_TRUST_PROXY: %w", err))
		} else {
			c.TrustProxy = b
		}
	}
	if v, ok := get("RAINY_DEV_CORS"); ok {
		c.DevCORS = strings.TrimRight(v, "/")
	}

	if err := errors.Join(errs...); err != nil {
		return nil, err
	}
	var err error
	if c.DataDir, err = filepath.Abs(c.DataDir); err != nil {
		return nil, fmt.Errorf("RAINY_DATA_DIR: %w", err)
	}
	if c.MusicDir, err = filepath.Abs(c.MusicDir); err != nil {
		return nil, fmt.Errorf("RAINY_MUSIC_DIR: %w", err)
	}
	return c, nil
}

// ParseDuration parses a Go duration, additionally accepting "0" and a "d" (day) unit
// such as "7d" or "1d12h".
func ParseDuration(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if s == "0" {
		return 0, nil
	}
	if i := strings.IndexByte(s, 'd'); i > 0 {
		days, err := strconv.Atoi(s[:i])
		if err != nil {
			return 0, fmt.Errorf("invalid duration %q", s)
		}
		d := time.Duration(days) * 24 * time.Hour
		if rest := s[i+1:]; rest != "" {
			r, err := time.ParseDuration(rest)
			if err != nil {
				return 0, fmt.Errorf("invalid duration %q", s)
			}
			d += r
		}
		return d, nil
	}
	return time.ParseDuration(s)
}

// ParseBool accepts 1/0, true/false, yes/no, on/off (case-insensitive).
func ParseBool(s string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "1", "true", "yes", "on", "y", "t":
		return true, nil
	case "0", "false", "no", "off", "n", "f", "":
		return false, nil
	}
	return false, fmt.Errorf("invalid boolean %q", s)
}

// ListenAddr returns "address:port" suitable for http.Server.Addr.
func (c *Config) ListenAddr() string {
	addr := c.Address
	if strings.Contains(addr, ":") && !strings.HasPrefix(addr, "[") {
		addr = "[" + addr + "]" // IPv6 literal
	}
	return addr + ":" + strconv.Itoa(c.Port)
}

// DBPath is the SQLite database file.
func (c *Config) DBPath() string { return filepath.Join(c.DataDir, "rainy.db") }

// CacheDir is the root of all caches.
func (c *Config) CacheDir() string { return filepath.Join(c.DataDir, "cache") }

// ArtworkCacheDir holds resized cover images.
func (c *Config) ArtworkCacheDir() string { return filepath.Join(c.CacheDir(), "artwork") }

// TrashDir holds deleted files as <TrashDir>/<libraryId>/<original relative path>.
func (c *Config) TrashDir() string { return filepath.Join(c.DataDir, "trash") }

// TmpDir is the upload staging directory.
func (c *Config) TmpDir() string { return filepath.Join(c.DataDir, "tmp") }

// SecretKeyPath is the 32-byte secret key file (AES key for stored passwords).
func (c *Config) SecretKeyPath() string { return filepath.Join(c.DataDir, "secret.key") }

// EnsureDirs creates the data directory layout.
func (c *Config) EnsureDirs() error {
	for _, d := range []string{c.DataDir, c.CacheDir(), c.ArtworkCacheDir(), c.TrashDir(), c.TmpDir()} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return fmt.Errorf("creating %s: %w", d, err)
		}
	}
	return nil
}
