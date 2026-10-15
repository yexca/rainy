package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"

	"rainy/internal/backup"
	"rainy/internal/buildinfo"
	"rainy/internal/config"
)

func backupCmd(args []string, stdout io.Writer) error {
	if len(args) != 2 || (args[0] != "create" && args[0] != "verify") {
		return errors.New("usage: rainy backup create <name> | rainy backup verify <name>")
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	var manifest *backup.Manifest
	if args[0] == "create" {
		manifest, err = backup.Create(ctx, cfg.DataDir, args[1], buildinfo.Version)
	} else {
		manifest, err = backup.Verify(ctx, cfg.DataDir, args[1])
	}
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(stdout, "Backup %s: %s succeeded (version %s, %d migrations).\nDatabase and secret.key only; music, trash and yt-dlp files require a separate backup.\n", args[1], args[0], manifest.Version, len(manifest.Migrations))
	return err
}
