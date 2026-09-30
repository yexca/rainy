// Package app is the dependency container: it opens the database and wires every
// service. HTTP layers (api, subsonic, server) receive an *App.
package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"rainy/internal/artwork"
	"rainy/internal/auth"
	"rainy/internal/buildinfo"
	"rainy/internal/config"
	"rainy/internal/db"
	"rainy/internal/events"
	"rainy/internal/listening"
	"rainy/internal/lxmusic"
	"rainy/internal/manage"
	"rainy/internal/metasearch"
	"rainy/internal/model"
	"rainy/internal/nowplaying"
	"rainy/internal/scanner"
	"rainy/internal/scrobble"
	"rainy/internal/store"
	"rainy/internal/tags"
	"rainy/internal/transcode"
	"rainy/internal/ytdlp"
)

// App holds the configuration and every service.
type App struct {
	Cfg        *config.Config
	DB         *db.DB
	Store      *store.Store
	Auth       *auth.Service
	Bus        *events.Bus
	NowPlaying *nowplaying.Tracker
	Scanner    *scanner.Scanner
	Artwork    *artwork.Service
	Transcoder *transcode.Service
	Manage     *manage.Service
	Metadata   *metasearch.Service // online metadata lookup; only used when settings.onlineMetadata is on
	Ytdlp      *ytdlp.Service      // downloads from YouTube / bilibili; only used when settings.ytdlpEnabled is on
	Online     *lxmusic.Service    // online music search and lx-music sources; only used when settings.lxSourcesEnabled is on
	Listening  *listening.Service  // listening reports from the play history
	Scrobble   *scrobble.Service   // plays to Last.fm / ListenBrainz; only when settings.lastfmEnabled / listenBrainzEnabled is on
	StartedAt  time.Time
}

// New creates the data directories, opens and migrates the database, loads (or creates)
// the secret key, wires the services and creates the default library from
// RAINY_MUSIC_DIR when no library exists yet.
func New(ctx context.Context, cfg *config.Config) (*App, error) {
	if err := cfg.EnsureDirs(); err != nil {
		return nil, err
	}
	d, err := db.Open(cfg.DBPath())
	if err != nil {
		return nil, err
	}
	a, err := build(ctx, cfg, d)
	if err != nil {
		_ = d.Close()
		return nil, err
	}
	return a, nil
}

func build(ctx context.Context, cfg *config.Config, d *db.DB) (*App, error) {
	if err := d.Migrate(ctx); err != nil {
		return nil, fmt.Errorf("migrating database: %w", err)
	}
	key, err := auth.LoadOrCreateKey(cfg.SecretKeyPath())
	if err != nil {
		return nil, err
	}
	// Files TagLib cannot parse are read with ffprobe/ffmpeg instead (when installed).
	tags.SetFFmpeg(cfg.FFmpegPath)
	st := store.New(d)
	bus := events.NewBus()
	sc := scanner.New(st, bus, cfg)
	art := artwork.New(st, cfg.ArtworkCacheDir())
	crypto := auth.NewCrypto(key)
	a := &App{
		Cfg:        cfg,
		DB:         d,
		Store:      st,
		Auth:       auth.NewService(st, crypto, cfg.SessionTTL),
		Bus:        bus,
		NowPlaying: nowplaying.New(),
		Scanner:    sc,
		Artwork:    art,
		Transcoder: transcode.New(cfg.FFmpegPath),
		Manage:     manage.New(st, sc, art, bus, cfg),
		Metadata:   metasearch.New(nil),
		Ytdlp: ytdlp.New(ytdlp.Options{
			Dir: cfg.YtdlpDir(), TmpDir: cfg.TmpDir(), BinaryPath: cfg.YtdlpPath, FFmpegPath: cfg.FFmpegPath,
			Cipher: crypto, UserAgent: "Rainy/" + buildinfo.Version,
		}),
		StartedAt: time.Now(),
	}
	a.Online = lxmusic.New(lxmusic.Options{Store: st, Metadata: a.Metadata})
	a.Listening = listening.New(st)
	a.Scrobble = scrobble.New(scrobble.Options{
		Store: st, Cipher: crypto, Settings: a.Settings, UserAgent: "Rainy/" + buildinfo.Version,
	})
	a.Manage.SetDownloader(a.Ytdlp)
	a.Manage.SetOnlineSource(a.Online)
	if err := a.ensureDefaultLibrary(ctx); err != nil {
		return nil, err
	}
	if err := st.PurgeExpiredSessions(ctx); err != nil {
		slog.Warn("purging expired sessions", "err", err)
	}
	return a, nil
}

// ensureDefaultLibrary creates library #1 from cfg.MusicDir when no library exists and the
// directory exists.
func (a *App) ensureDefaultLibrary(ctx context.Context) error {
	libs, err := a.Store.ListLibraries(ctx)
	if err != nil {
		return fmt.Errorf("listing libraries: %w", err)
	}
	if len(libs) > 0 {
		return nil
	}
	fi, err := os.Stat(a.Cfg.MusicDir)
	switch {
	case errors.Is(err, os.ErrNotExist):
		slog.Warn("music directory does not exist; no library created (add one in Admin → Libraries)", "path", a.Cfg.MusicDir)
		return nil
	case err != nil:
		slog.Warn("music directory is not accessible; no library created", "path", a.Cfg.MusicDir, "err", err)
		return nil
	case !fi.IsDir():
		slog.Warn("music path is not a directory; no library created", "path", a.Cfg.MusicDir)
		return nil
	}
	name := filepath.Base(a.Cfg.MusicDir)
	if name == "" || name == "." || name == string(filepath.Separator) {
		name = "Music"
	}
	lib := &model.Library{Name: name, Path: a.Cfg.MusicDir}
	if err := a.Store.CreateLibrary(ctx, lib); err != nil {
		return fmt.Errorf("creating default library: %w", err)
	}
	slog.Info("created default library", "id", lib.ID, "name", lib.Name, "path", lib.Path)
	return nil
}

// DefaultSettings returns the settings defaults for this configuration.
func (a *App) DefaultSettings() model.Settings { return model.DefaultSettings(a.Cfg.ScanInterval) }

// Settings returns the current runtime settings (stored values merged over defaults). On
// a read error the defaults are returned and the error is logged.
func (a *App) Settings(ctx context.Context) model.Settings {
	s, err := a.Store.GetSettings(ctx, a.DefaultSettings())
	if err != nil {
		slog.Error("reading settings, using defaults", "err", err)
		return a.DefaultSettings()
	}
	return s
}

// Close stops downloads, source scripts, the scrobble sender and a running yt-dlp install,
// then closes the database.
func (a *App) Close() error {
	a.Manage.CloseDownloads()
	a.Scrobble.Close()
	a.Online.Close()
	a.Ytdlp.Close()
	return a.DB.Close()
}
