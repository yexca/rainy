// Command rainy is the Rainy music server.
//
// Usage:
//
//	rainy [serve]                                   start the server (default)
//	rainy version                                   print version information
//	rainy user list                                 list users
//	rainy user reset-password <username> <password> set a new password (signs the user out everywhere)
//
// Configuration comes from RAINY_* environment variables (see docs/architecture/contract.md §4).
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"text/tabwriter"
	"time"

	"rainy/internal/app"
	"rainy/internal/auth"
	"rainy/internal/buildinfo"
	"rainy/internal/config"
	"rainy/internal/db"
	"rainy/internal/scanner"
	"rainy/internal/server"
	"rainy/internal/store"
)

const shutdownTimeout = 10 * time.Second

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "rainy:", err)
		os.Exit(1)
	}
}

func run(args []string, stdout, stderr io.Writer) error {
	cmd := "serve"
	if len(args) > 0 {
		cmd, args = args[0], args[1:]
	}
	switch cmd {
	case "serve":
		return serve()
	case "version", "--version", "-v":
		_, _ = fmt.Fprintln(stdout, buildinfo.String())
		return nil
	case "user":
		return userCmd(args, stdout)
	case "help", "--help", "-h":
		usage(stdout)
		return nil
	default:
		usage(stderr)
		return fmt.Errorf("unknown command %q", cmd)
	}
}

func usage(w io.Writer) {
	_, _ = fmt.Fprint(w, `Usage:
  rainy [serve]                                   start the server
  rainy version                                   print version information
  rainy user list                                 list users
  rainy user reset-password <username> <password> set a new password

Configuration: RAINY_ADDRESS, RAINY_PORT, RAINY_DATA_DIR, RAINY_MUSIC_DIR, RAINY_SCAN_INTERVAL,
RAINY_SCAN_ON_START, RAINY_FFMPEG_PATH, RAINY_LOG_LEVEL, RAINY_LOG_FORMAT, RAINY_SESSION_TTL,
RAINY_TRUST_PROXY, RAINY_DEV_CORS
`)
}

func setupLogging(cfg *config.Config) {
	var level slog.Level
	switch cfg.LogLevel {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}
	opts := &slog.HandlerOptions{Level: level}
	var h slog.Handler
	if cfg.LogFormat == "json" {
		h = slog.NewJSONHandler(os.Stderr, opts)
	} else {
		h = slog.NewTextHandler(os.Stderr, opts)
	}
	slog.SetDefault(slog.New(h))
}

func serve() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	setupLogging(cfg)
	slog.Info("starting Rainy", "version", buildinfo.Version, "commit", buildinfo.Commit,
		"dataDir", cfg.DataDir, "musicDir", cfg.MusicDir)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	a, err := app.New(ctx, cfg)
	if err != nil {
		return err
	}
	defer func() {
		if err := a.Close(); err != nil {
			slog.Error("closing database", "err", err)
		}
	}()

	if !a.Transcoder.Available() {
		slog.Warn("ffmpeg not found; transcoding is disabled", "path", cfg.FFmpegPath)
	}

	// Schedule stops (and cancels a running scan) when ctx ends; wait for it before the
	// deferred a.Close so no scan writes to a closed database.
	scheduleDone := make(chan struct{})
	go func() {
		defer close(scheduleDone)
		a.Scanner.Schedule(ctx)
	}()
	defer func() {
		stop() // cancels ctx on early-return paths too
		<-scheduleDone
	}()
	if cfg.ScanOnStart {
		if err := a.Scanner.Start(ctx, scanner.Options{}); err != nil {
			slog.Error("starting initial scan", "err", err)
		}
	}

	srv := server.New(a)
	ln, err := net.Listen("tcp", srv.Addr)
	if err != nil {
		return fmt.Errorf("listening on %s: %w", srv.Addr, err)
	}
	slog.Info("Rainy is listening", "url", displayURL(cfg, ln.Addr()))
	if err := server.ServeListener(ctx, srv, ln, shutdownTimeout); err != nil {
		return err
	}
	slog.Info("bye")
	return nil
}

// displayURL returns a clickable URL for the listen address (localhost for wildcard binds).
func displayURL(cfg *config.Config, addr net.Addr) string {
	host := cfg.Address
	if tcp, ok := addr.(*net.TCPAddr); ok && (tcp.IP.IsUnspecified() || host == "") {
		host = "localhost"
	}
	port := strconv.Itoa(cfg.Port)
	if tcp, ok := addr.(*net.TCPAddr); ok {
		port = strconv.Itoa(tcp.Port)
	}
	return "http://" + net.JoinHostPort(host, port)
}

// openStore opens and migrates the database without starting services (for CLI commands).
func openStore(ctx context.Context) (*store.Store, *auth.Service, func(), error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, nil, nil, err
	}
	setupLogging(cfg)
	if _, err := os.Stat(cfg.DBPath()); err != nil {
		return nil, nil, nil, fmt.Errorf("no database at %s (is RAINY_DATA_DIR set correctly?)", cfg.DBPath())
	}
	d, err := db.Open(cfg.DBPath())
	if err != nil {
		return nil, nil, nil, err
	}
	if err := d.Migrate(ctx); err != nil {
		_ = d.Close()
		return nil, nil, nil, err
	}
	key, err := auth.LoadOrCreateKey(cfg.SecretKeyPath())
	if err != nil {
		_ = d.Close()
		return nil, nil, nil, err
	}
	st := store.New(d)
	return st, auth.NewService(st, auth.NewCrypto(key), cfg.SessionTTL), func() { _ = d.Close() }, nil
}

func userCmd(args []string, stdout io.Writer) error {
	if len(args) == 0 {
		return errors.New("usage: rainy user list | rainy user reset-password <username> <password>")
	}
	ctx := context.Background()
	switch args[0] {
	case "list":
		st, _, closeDB, err := openStore(ctx)
		if err != nil {
			return err
		}
		defer closeDB()
		users, err := st.ListUsers(ctx)
		if err != nil {
			return err
		}
		tw := tabwriter.NewWriter(stdout, 0, 4, 2, ' ', 0)
		_, _ = fmt.Fprintln(tw, "USERNAME\tDISPLAY NAME\tROLE\tDOWNLOAD\tLAST SEEN")
		for _, u := range users {
			role := "user"
			switch {
			case u.IsAdmin:
				role = "admin"
			case u.CanManage:
				role = "manager"
			}
			seen := "never"
			if u.LastSeenAt > 0 {
				seen = time.UnixMilli(u.LastSeenAt).Format("2006-01-02 15:04")
			}
			_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\t%t\t%s\n", u.Username, u.DisplayName, role, u.CanDownload, seen)
		}
		return tw.Flush()
	case "reset-password":
		if len(args) != 3 {
			return errors.New("usage: rainy user reset-password <username> <password>")
		}
		st, as, closeDB, err := openStore(ctx)
		if err != nil {
			return err
		}
		defer closeDB()
		u, err := st.GetUserByUsername(ctx, args[1])
		if errors.Is(err, store.ErrNotFound) {
			return fmt.Errorf("user %q not found", args[1])
		}
		if err != nil {
			return err
		}
		if err := as.ChangePassword(ctx, u.ID, args[2]); err != nil {
			return err
		}
		if err := st.DeleteUserSessions(ctx, u.ID); err != nil {
			return err
		}
		_, _ = fmt.Fprintf(stdout, "Password for %q updated; existing web sessions were signed out.\n", u.Username)
		return nil
	default:
		return fmt.Errorf("unknown user command %q", args[0])
	}
}
